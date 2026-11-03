package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"mem/application/ports"
	"mem/domain"
)

func StatsQuery(db *sql.DB, dbPath, project string) (projectCount, totalCount int64, err error) {
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE project = ?`, project).Scan(&projectCount); err != nil {
		return 0, 0, fmt.Errorf("count project memories: %w", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&totalCount); err != nil {
		return 0, 0, fmt.Errorf("count total memories: %w", err)
	}
	return projectCount, totalCount, nil
}

func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	return info.Size(), nil
}

// esperaLimpiezaMs acota lo que una limpieza posterior al borrado espera a un lector.
const esperaLimpiezaMs = 200

// alcanceBorrado fija qué hace la política única de borrado además de borrar.
type alcanceBorrado struct {
	project string // Proyecto de una purga total; vacío abarca todos.
	total   bool   // Vacía los originales del alcance en vez de buscar copias.
	fisico  bool   // Reescribe la base con VACUUM: retira restos antiguos.
}

// memoriaBorrada es el texto exacto que salió de memories en la transacción.
type memoriaBorrada struct {
	project, title, content string
}

// borrarMemorias es la política única de borrado: forget, purge, gc y la TUI
// llegan todas aquí. En una sola transacción retira las memorias que cumplen
// where, sus relaciones, su texto del índice FTS5 y sus copias comprimidas; no
// hay ON DELETE CASCADE ni triggers.
func borrarMemorias(ctx context.Context, conn *sql.Conn, where string, args []any, alcance alcanceBorrado) (int64, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var deleted int64
	var borradas []memoriaBorrada
	if alcance.total {
		// Una purga total no necesita el contenido: vacía el alcance entero.
		deleted, err = borrarSinContenido(ctx, tx, where, args)
	} else {
		borradas, err = borrarDevolviendoContenido(ctx, tx, where, args)
		deleted = int64(len(borradas))
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_relations
		WHERE memory_id_a NOT IN (SELECT id FROM memories)
		   OR memory_id_b NOT IN (SELECT id FROM memories)`); err != nil {
		return 0, fmt.Errorf("clean orphan relations: %w", err)
	}
	if err := limpiarIndiceDeMemorias(tx); err != nil {
		return 0, err
	}
	if alcance.total {
		err = vaciarOriginales(ctx, tx, alcance.project)
	} else {
		err = retirarCopiasExactas(ctx, tx, borradas)
	}
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return deleted, nil
}

func borrarSinContenido(ctx context.Context, tx *sql.Tx, where string, args []any) (int64, error) {
	res, err := tx.ExecContext(ctx, `DELETE FROM memories WHERE `+where, args...)
	if err != nil {
		return 0, fmt.Errorf("delete memories: %w", err)
	}
	return res.RowsAffected()
}

func borrarDevolviendoContenido(ctx context.Context, tx *sql.Tx, where string, args []any) ([]memoriaBorrada, error) {
	rows, err := tx.QueryContext(ctx, `DELETE FROM memories WHERE `+where+` RETURNING project, title, content`, args...)
	if err != nil {
		return nil, fmt.Errorf("delete memories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var borradas []memoriaBorrada
	for rows.Next() {
		var b memoriaBorrada
		if err := rows.Scan(&b.project, &b.title, &b.content); err != nil {
			return nil, fmt.Errorf("delete memories: %w", err)
		}
		borradas = append(borradas, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("delete memories: %w", err)
	}
	return borradas, nil
}

// retirarCopiasExactas borra los originales idénticos a una memoria borrada, en
// su forma normal o como «título: contenido» de los packs, comparando por hash:
// no descomprime nada y el coste no depende del tamaño del almacén. Si una memoria
// viva aún contiene ese texto, se conservan. Un original que solo incluye el texto
// dentro de otro sobrevive hasta caducar (HCD-013, Q12).
func retirarCopiasExactas(ctx context.Context, tx *sql.Tx, borradas []memoriaBorrada) error {
	// Un hash puede corresponder a varias memorias: «t» + X y otra con contenido «t: X».
	porHash := make(map[string][]candidataCopia, len(borradas)*2)
	hashes := make([]string, 0, len(borradas)*2)
	anotar := func(b memoriaBorrada, texto string) {
		hash := hashOriginal(b.project, texto)
		porHash[hash] = append(porHash[hash], candidataCopia{memoriaBorrada: b, forma: texto})
		hashes = append(hashes, hash)
	}
	for _, b := range borradas {
		anotar(b, b.content)
		if b.title != "" {
			anotar(b, b.title+": "+b.content)
		}
	}
	presentes, err := originalesPresentes(ctx, tx, hashes)
	if err != nil {
		return err
	}
	for _, hash := range presentes {
		vivo, err := algunContenidoVivo(ctx, tx, porHash[hash])
		if err != nil {
			return err
		}
		if vivo {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM compression_originals WHERE hash = ?`, hash); err != nil {
			return fmt.Errorf("retirar copias comprimidas: %w", err)
		}
	}
	return nil
}

// candidataCopia es una memoria borrada junto a la forma exacta cuyo hash se
// anotó: su contenido o «título: contenido» (ADM-01).
type candidataCopia struct {
	memoriaBorrada
	forma string
}

// algunContenidoVivo comprueba la misma forma que produjo el hash: un original
// «T: abc» sigue vivo si una memoria viva contiene «T: abc». Para la forma con
// título también cuenta el contenido plano vivo; conservar de más solo retrasa
// la retirada hasta la caducidad, retirar de menos rompe una ref viva.
func algunContenidoVivo(ctx context.Context, tx *sql.Tx, candidatas []candidataCopia) (bool, error) {
	for _, c := range candidatas {
		textos := []string{c.forma}
		if c.forma != c.content {
			textos = append(textos, c.content)
		}
		for _, texto := range textos {
			vivo, err := contenidoVivo(ctx, tx, c.project, texto)
			if err != nil || vivo {
				return vivo, err
			}
		}
	}
	return false, nil
}

// originalesPresentes pasa los hashes como un solo parámetro JSON: un lote grande
// agotaría el límite de variables de SQLite.
func originalesPresentes(ctx context.Context, tx *sql.Tx, hashes []string) ([]string, error) {
	lista, err := json.Marshal(hashes)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT hash FROM compression_originals WHERE hash IN (SELECT value FROM json_each(?))`, string(lista))
	if err != nil {
		return nil, fmt.Errorf("buscar copias comprimidas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var presentes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("buscar copias comprimidas: %w", err)
		}
		presentes = append(presentes, hash)
	}
	return presentes, rows.Err()
}

// contenidoVivo usa el mismo umbral que domain.CopiaDeTexto: un contenido breve
// solo sigue vivo si coincide entero, no si aparece dentro de otro texto.
func contenidoVivo(ctx context.Context, tx *sql.Tx, project, content string) (bool, error) {
	limpio := strings.TrimSpace(content)
	var vivo bool
	var err error
	if utf8.RuneCountInString(limpio) < domain.MinContenidoIntegro {
		// Igualdad byte a byte: SQLite trim solo quita espacios ASCII y no iguala la
		// normalización de Go. También cuenta una fila que difiere en esos espacios.
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memories WHERE project = ? AND (content = ? OR trim(content) = ?))`,
			project, content, limpio).Scan(&vivo)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memories WHERE project = ? AND instr(content, ?) > 0)`,
			project, limpio).Scan(&vivo)
	}
	if err != nil {
		return false, fmt.Errorf("comprobar memorias vivas: %w", err)
	}
	return vivo, nil
}

// vaciarOriginales retira todos los originales del alcance de una purga total.
func vaciarOriginales(ctx context.Context, tx *sql.Tx, project string) error {
	query := `DELETE FROM compression_originals`
	var alcance []any
	if project != "" {
		query += ` WHERE project = ?`
		alcance = append(alcance, project)
	}
	if _, err := tx.ExecContext(ctx, query, alcance...); err != nil {
		return fmt.Errorf("vaciar copias comprimidas: %w", err)
	}
	return nil
}

// borrarConGarantia borra en una sola transacción y limpia el archivo después. Un
// error antes de confirmar no borra nada.
func borrarConGarantia(db *sql.DB, where string, args []any, alcance alcanceBorrado) (int64, error) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	deleted, err := borrarMemorias(ctx, conn, where, args, alcance)
	if err != nil {
		return 0, err
	}
	// Una purga total pudo vaciar originales sin borrar memorias.
	if deleted == 0 && !alcance.total {
		return 0, nil
	}
	return deleted, limpiarTrasBorrar(ctx, conn, deleted, alcance.fisico)
}

// limpiarTrasBorrar saca el texto del archivo. secure_delete ya sobrescribe lo
// que libera este borrado; VACUUM retira además restos anteriores en páginas
// libres, y solo corre en purge y gc porque reescribe toda la base. La espera es
// corta: el borrado ya terminó y un lector activo no debe retener a quien lo pidió.
func limpiarTrasBorrar(ctx context.Context, conn *sql.Conn, deleted int64, fisico bool) error {
	restaurar := conEsperaCorta(ctx, conn)
	defer restaurar()
	if fisico {
		if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
			return clasificarLimpieza(deleted, err)
		}
	}
	if err := vaciarWAL(ctx, conn); err != nil {
		return clasificarLimpieza(deleted, err)
	}
	return nil
}

// conEsperaCorta acota el busy_timeout de la conexión y devuelve cómo restaurarlo.
func conEsperaCorta(ctx context.Context, conn *sql.Conn) func() {
	_, _ = conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA busy_timeout = %d`, esperaLimpiezaMs))
	return func() {
		_, _ = conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA busy_timeout = %d`, esperaOcupadaMs))
	}
}

// errWALRetenido lleva la marca que reconoce isSQLiteBusy: un lector activo impide
// vaciar el WAL sin que sea un fallo de la base.
var errWALRetenido = errors.New("SQLITE_BUSY: otro proceso retiene el WAL")

func vaciarWAL(ctx context.Context, conn *sql.Conn) error {
	var busy, logFrames, checkpointed int
	if err := conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return errWALRetenido
	}
	return nil
}

// clasificarLimpieza separa por causa: solo un bloqueo es limpieza pendiente.
// Corrupción, E/S o disco lleno siguen siendo un fallo, con el conteo borrado.
func clasificarLimpieza(deleted int64, err error) error {
	if isSQLiteBusy(err) {
		return fmt.Errorf("%d memoria(s) borrada(s), pero otra conexión o proceso ocupa la base: el texto sigue en disco hasta que termine; repite `mem compact` después: %w",
			deleted, domain.ErrLimpiezaPendiente)
	}
	return fmt.Errorf("%d memoria(s) borrada(s), pero la limpieza falló: %w", deleted, err)
}

// memorySearchEsFTS5 evita tratar como índice una tabla memory_search plana.
func memorySearchEsFTS5(tx *sql.Tx) (bool, error) {
	var definicion sql.NullString
	err := tx.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'memory_search'`).Scan(&definicion)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("índice de búsqueda: %w", err)
	}
	return strings.Contains(strings.ToLower(definicion.String), "using fts5"), nil
}

// limpiarIndiceDeMemorias retira de memory_search las filas sin memoria y
// reescribe el índice: un DELETE en FTS5 solo marca el término como borrado y el
// texto seguiría en los segmentos antiguos hasta una fusión. Sin FTS5 no hay
// índice que limpiar y la búsqueda cae a LIKE.
func limpiarIndiceDeMemorias(tx *sql.Tx) error {
	esFTS5, err := memorySearchEsFTS5(tx)
	if err != nil || !esFTS5 {
		return err
	}
	res, err := tx.Exec(`DELETE FROM memory_search WHERE memory_id NOT IN (SELECT id FROM memories)`)
	if err != nil {
		return fmt.Errorf("índice de búsqueda: %w", err)
	}
	huerfanas, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("índice de búsqueda: %w", err)
	}
	if huerfanas == 0 {
		return nil
	}
	if _, err := tx.Exec(`INSERT INTO memory_search (memory_search) VALUES ('optimize')`); err != nil {
		return fmt.Errorf("reescribir el índice de búsqueda: %w", err)
	}
	return nil
}

// PurgeMemories borra memorias según filter con garantía física y retira de
// compression_originals las copias de ese texto.
func PurgeMemories(db *sql.DB, filter ports.PurgeFilter) (int64, error) {
	if !filter.All && filter.Project == "" {
		return 0, fmt.Errorf("purge: alcance ambiguo — se requiere Project o All=true")
	}

	where := `1=1`
	args := []any{}
	if !filter.All {
		where += ` AND project = ?`
		args = append(args, filter.Project)
	}
	if filter.Type != "" {
		where += ` AND type = ?`
		args = append(args, filter.Type)
	}
	if filter.OlderThanDays > 0 {
		where += fmt.Sprintf(` AND created_at < datetime('now', '-%d days')`, filter.OlderThanDays)
	}

	alcance := alcanceBorrado{
		project: filter.Project,
		total:   filter.Type == "" && filter.OlderThanDays == 0,
		fisico:  true,
	}
	if filter.All {
		alcance.project = ""
	}
	deleted, err := borrarConGarantia(db, where, args, alcance)
	if err != nil {
		return deleted, fmt.Errorf("purge: %w", err)
	}
	return deleted, nil
}

// tamanoEnDisco suma la base y su WAL: el texto puede estar en cualquiera de los dos.
func tamanoEnDisco(dbPath string) (int64, error) {
	total, err := FileSize(dbPath)
	if err != nil {
		return 0, err
	}
	if wal, err := os.Stat(dbPath + "-wal"); err == nil {
		total += wal.Size()
	}
	return total, nil
}

// CompactDB ejecuta VACUUM y vacía el WAL para reclamar el espacio y el texto de
// filas borradas (FR-006); nunca toca ni pierde filas sobrevivientes. El checkpoint
// corre aunque el tamaño no cambie. Si un lector retiene el WAL, devuelve
// ErrLimpiezaPendiente junto con los tamaños.
func CompactDB(db *sql.DB, dbPath string) (beforeBytes, afterBytes int64, err error) {
	before, err := tamanoEnDisco(dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("compact: %w", err)
	}

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("compact: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
		return 0, 0, fmt.Errorf("compact: vacuum: %w", err)
	}
	limpieza := vaciarWAL(ctx, conn)

	after, err := tamanoEnDisco(dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("compact: %w", err)
	}
	if limpieza != nil {
		if isSQLiteBusy(limpieza) {
			return before, after, fmt.Errorf("otro proceso retiene el WAL: el texto puede seguir en disco hasta que termine: %w", domain.ErrLimpiezaPendiente)
		}
		return before, after, fmt.Errorf("compact: checkpoint: %w", limpieza)
	}
	return before, after, nil
}
