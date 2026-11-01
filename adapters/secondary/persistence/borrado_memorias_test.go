package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/application/ports"
	"mem/domain"
)

type relojFijo struct{}

func (relojFijo) Now() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }

func abrirBaseDeBorrado(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, DbPath(dir)
}

func insertarMemoria(t *testing.T, db *sql.DB, tipo domain.MemoryType, titulo, contenido string) int64 {
	t.Helper()
	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: tipo, Title: titulo, Content: contenido})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func filasDelIndice(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM memory_search`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestBorradoUnico_RutasDejanIndiceYRelacionesLimpios es el contrato de la
// política única: forget y purge, con o sin filtro, no dejan índice ni
// relaciones de una memoria borrada, y conservan los de las supervivientes.
func TestBorradoUnico_RutasDejanIndiceYRelacionesLimpios(t *testing.T) {
	rutas := map[string]func(*testing.T, *sql.DB, int64){
		"forget": func(t *testing.T, db *sql.DB, id int64) {
			if ok, err := DeleteMemory(db, "proj", id); err != nil || !ok {
				t.Fatalf("forget: %v %v", ok, err)
			}
		},
		"purge": func(t *testing.T, db *sql.DB, _ int64) {
			if n, err := PurgeMemories(db, ports.PurgeFilter{Project: "proj", Type: "decision"}); err != nil || n != 1 {
				t.Fatalf("purge: %d %v", n, err)
			}
		},
		// gc y la TUI llaman a MaintenanceRepository.Purge con estos filtros.
		"gc": func(t *testing.T, db *sql.DB, id int64) {
			if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now', '-100 days') WHERE id = ?`, id); err != nil {
				t.Fatal(err)
			}
			repo := NewMaintenanceRepository(db, "")
			if n, err := repo.Purge(ports.PurgeFilter{Project: "proj", OlderThanDays: 90}); err != nil || n != 1 {
				t.Fatalf("gc: %d %v", n, err)
			}
		},
	}
	for nombre, borrar := range rutas {
		t.Run(nombre, func(t *testing.T) {
			db, _ := abrirBaseDeBorrado(t)
			viva := insertarMemoria(t, db, domain.Learning, "viva", "supervivienteunica conservar")
			otra := insertarMemoria(t, db, domain.Learning, "otra", "segunda superviviente")
			borrada := insertarMemoria(t, db, domain.Decision, "borrada", "contenidoborrado retirar")
			for _, par := range [][2]int64{{viva, otra}, {viva, borrada}} {
				if _, err := db.Exec(`INSERT INTO memory_relations
					(project, memory_id_a, memory_id_b, relation, reasoning)
					VALUES ('proj', ?, ?, 'related', 'r')`, par[0], par[1]); err != nil {
					t.Fatal(err)
				}
			}

			borrar(t, db, borrada)

			if n := filasDelIndice(t, db); n != 2 {
				t.Fatalf("filas del índice = %d, quiero 2", n)
			}
			var relaciones int
			if err := db.QueryRow(`SELECT count(*) FROM memory_relations`).Scan(&relaciones); err != nil {
				t.Fatal(err)
			}
			if relaciones != 1 {
				t.Fatalf("relaciones = %d, quiero 1: la de la memoria borrada debe irse y la otra quedarse", relaciones)
			}
			halladas, err := SearchMemories(db, "proj", "supervivienteunica", 5)
			if err != nil || len(halladas) != 1 || halladas[0].ID != viva {
				t.Fatalf("la superviviente debe seguir encontrándose: %v %v", halladas, err)
			}
		})
	}
}

// TestBorradoUnico_PurgaDeLaTUI usa el filtro que construye la TUI: todo el
// proyecto. Lo de otro proyecto se conserva y sigue indexado.
func TestBorradoUnico_PurgaDeLaTUI(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	insertarMemoria(t, db, domain.Decision, "borrada", "contenidoborrado retirar")
	if _, err := InsertMemory(db, &domain.Memory{Project: "otro", Type: domain.Learning, Title: "ajena", Content: "de otro proyecto"}); err != nil {
		t.Fatal(err)
	}
	if n, err := NewMaintenanceRepository(db, "").Purge(ports.PurgeFilter{Project: "proj"}); err != nil || n != 1 {
		t.Fatalf("tui: %d %v", n, err)
	}
	if n := filasDelIndice(t, db); n != 1 {
		t.Fatalf("filas del índice = %d, quiero 1", n)
	}
}

// TestPurgaFisica_TextoFueraDelArchivo cubre C-03: sin compactar, el texto
// purgado no queda ni en la base ni en su WAL.
func TestPurgaFisica_TextoFueraDelArchivo(t *testing.T) {
	db, ruta := abrirBaseDeBorrado(t)
	insertarMemoria(t, db, domain.Learning, "viva", "CANARIOVIVO conservar")
	insertarMemoria(t, db, domain.Decision, "CANARIOTITULO", "CANARIOPURGA retirar")

	if _, err := PurgeMemories(db, ports.PurgeFilter{Project: "proj", Type: "decision"}); err != nil {
		t.Fatal(err)
	}

	for _, archivo := range []string{ruta, ruta + "-wal"} {
		datos, err := os.ReadFile(archivo)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, canario := range []string{"CANARIOPURGA", "CANARIOTITULO"} {
			if bytes.Contains(datos, []byte(canario)) {
				t.Errorf("%s sigue en %s", canario, filepath.Base(archivo))
			}
		}
	}
}

// guardarOriginal guarda una copia y devuelve su ref.
func guardarOriginal(t *testing.T, almacen *OriginalStoreRepository, proyecto, texto string) string {
	t.Helper()
	ref, err := almacen.Put(context.Background(), proyecto, texto, "context_item", "budget")
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func originalExiste(t *testing.T, db *sql.DB, ref string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM compression_originals WHERE ref = ?`, ref).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// TestPurgaCopias_SoloCopiasExactas: se retiran los originales idénticos al
// contenido o a «título: contenido». Compartir título, línea o bloque no basta.
func TestPurgaCopias_SoloCopiasExactas(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	contenido := "## Sesiones\nprimera linea secreta de la memoria\nsegunda linea secreta de la memoria"
	insertarMemoria(t, db, domain.Learning, "memoria viva", "## Sesiones\nlinea de la memoria viva")
	insertarMemoria(t, db, domain.Decision, "Decisiones", contenido)
	almacen := NewOriginalStoreRepository(db, relojFijo{}, 1, 1)
	exacta := guardarOriginal(t, almacen, "proj", contenido)
	forma := guardarOriginal(t, almacen, "proj", "Decisiones: "+contenido)
	bloque := guardarOriginal(t, almacen, "proj", "contexto previo\n"+contenido+"\ncontexto posterior")
	comparteTitulo := guardarOriginal(t, almacen, "proj", "Decisiones del equipo sobre la memoria viva")
	comparteLinea := guardarOriginal(t, almacen, "proj", "## Sesiones\nsegunda linea secreta de la memoria\ncierre")
	ajena := guardarOriginal(t, almacen, "otro", contenido)

	if n, err := PurgeMemories(db, ports.PurgeFilter{Project: "proj", Type: "decision"}); err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}

	for nombre, ref := range map[string]string{"exacta": exacta, "título: contenido": forma} {
		if originalExiste(t, db, ref) {
			t.Errorf("la copia %s debía retirarse", nombre)
		}
	}
	for nombre, ref := range map[string]string{"bloque": bloque, "título": comparteTitulo, "línea": comparteLinea, "otro proyecto": ajena} {
		if !originalExiste(t, db, ref) {
			t.Errorf("el original %s debe conservarse", nombre)
		}
	}
}

// TestPurgaCopias_SinTituloNoInventaLaForma: sin título no existe «: contenido».
func TestPurgaCopias_SinTituloNoInventaLaForma(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	contenido := "contenido suficientemente largo sin ningun titulo"
	res, err := db.Exec(`INSERT INTO memories (project, type, title, content) VALUES ('proj', 'decision', '', ?)`, contenido)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	ajena := guardarOriginal(t, NewOriginalStoreRepository(db, relojFijo{}, 1, 1), "proj", ": "+contenido)
	if n, err := DeleteMemories(db, "proj", []int64{id}); err != nil || n != 1 {
		t.Fatalf("lote: %d %v", n, err)
	}
	if !originalExiste(t, db, ajena) {
		t.Fatal("sin título no hay forma título: contenido")
	}
}

// TestPurgaCopias_TotalSinMemoriasVaciaElAlcance cubre M-01.
func TestPurgaCopias_TotalSinMemoriasVaciaElAlcance(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	ref := guardarOriginal(t, NewOriginalStoreRepository(db, relojFijo{}, 1, 1), "proj", "copia sin memoria")
	if n, err := PurgeMemories(db, ports.PurgeFilter{All: true}); err != nil || n != 0 {
		t.Fatalf("purge: %d %v", n, err)
	}
	if originalExiste(t, db, ref) {
		t.Fatal("una purga total vacía su alcance aunque no borre memorias")
	}
}

// TestPurgaCopias_PurgaTotalVaciaElAlcance: sin filtros de tipo ni antigüedad se
// retira todo el proyecto; --all retira todos.
func TestPurgaCopias_PurgaTotalVaciaElAlcance(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	insertarMemoria(t, db, domain.Decision, "borrada", "contenido retirar")
	almacen := NewOriginalStoreRepository(db, relojFijo{}, 1, 1)
	salida := guardarOriginal(t, almacen, "proj", "salida de herramienta sin relación")
	ajena := guardarOriginal(t, almacen, "otro", "copia de otro proyecto")

	if _, err := PurgeMemories(db, ports.PurgeFilter{Project: "proj"}); err != nil {
		t.Fatal(err)
	}
	if originalExiste(t, db, salida) {
		t.Error("una purga total debe vaciar su alcance")
	}
	if !originalExiste(t, db, ajena) {
		t.Error("una purga de proyecto no toca otro proyecto")
	}

	if _, err := InsertMemory(db, &domain.Memory{Project: "otro", Type: domain.Learning, Title: "x", Content: "de otro proyecto"}); err != nil {
		t.Fatal(err)
	}
	if _, err := PurgeMemories(db, ports.PurgeFilter{All: true}); err != nil {
		t.Fatal(err)
	}
	if originalExiste(t, db, ajena) {
		t.Error("--all debe vaciar todos los proyectos")
	}
}

// TestBorradoPuntual_RetiraCopiaIntegra: forget retira la copia exacta y no toca
// lo demás.
func TestBorradoPuntual_RetiraCopiaIntegra(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	contenido := "contenido suficientemente largo de la memoria olvidada"
	id := insertarMemoria(t, db, domain.Decision, "olvidada", contenido)
	almacen := NewOriginalStoreRepository(db, relojFijo{}, 1, 1)
	copia := guardarOriginal(t, almacen, "proj", contenido)
	otra := guardarOriginal(t, almacen, "proj", "salida de herramienta sin relación")

	if ok, err := DeleteMemory(db, "proj", id); err != nil || !ok {
		t.Fatalf("forget: %v %v", ok, err)
	}
	if originalExiste(t, db, copia) {
		t.Error("forget debe retirar la copia íntegra")
	}
	if !originalExiste(t, db, otra) {
		t.Error("forget no debe tocar los demás originales")
	}
}

// TestHistorial_TextoFueraDelArchivo cubre C-22: las fusiones del índice dejan
// texto en páginas libres que ni optimize ni el checkpoint reescriben. Debe
// usar una base con historial: una recién creada no tiene páginas libres.
func TestHistorial_TextoFueraDelArchivo(t *testing.T) {
	rutas := map[string]func(*testing.T, *sql.DB, int64){
		"forget": func(t *testing.T, db *sql.DB, id int64) {
			if ok, err := DeleteMemory(db, "proj", id); err != nil || !ok {
				t.Fatalf("forget: %v %v", ok, err)
			}
		},
		"purge": func(t *testing.T, db *sql.DB, _ int64) {
			if _, err := PurgeMemories(db, ports.PurgeFilter{Project: "proj", Type: "decision"}); err != nil {
				t.Fatal(err)
			}
		},
		"lote": func(t *testing.T, db *sql.DB, id int64) {
			if n, err := DeleteMemories(db, "proj", []int64{id}); err != nil || n != 1 {
				t.Fatalf("lote: %d %v", n, err)
			}
		},
	}
	for nombre, borrar := range rutas {
		t.Run(nombre, func(t *testing.T) {
			db, ruta := abrirBaseDeBorrado(t)
			id := insertarMemoria(t, db, domain.Decision, "canario", "CANARIOHIST retirar")
			for i := 0; i < 70; i++ {
				insertarMemoria(t, db, domain.Learning, fmt.Sprintf("relleno %d", i), fmt.Sprintf("texto%d distinto %d", i, i))
			}
			borrar(t, db, id)
			for _, archivo := range []string{ruta, ruta + "-wal"} {
				datos, err := os.ReadFile(archivo)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(datos, []byte("CANARIOHIST")) {
					t.Errorf("el texto sigue en %s", filepath.Base(archivo))
				}
			}
		})
	}
}

// TestBorradoPuntual_TextoFueraDelWAL cubre C-15: forget deja la misma garantía
// física que la purga, con la base abierta.
func TestBorradoPuntual_TextoFueraDelWAL(t *testing.T) {
	db, ruta := abrirBaseDeBorrado(t)
	id := insertarMemoria(t, db, domain.Decision, "CANARIOTITULO", "CANARIOFORGET retirar")
	if ok, err := DeleteMemory(db, "proj", id); err != nil || !ok {
		t.Fatalf("forget: %v %v", ok, err)
	}
	for _, archivo := range []string{ruta, ruta + "-wal"} {
		datos, err := os.ReadFile(archivo)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(datos, []byte("CANARIOFORGET")) {
			t.Errorf("el texto olvidado sigue en %s", filepath.Base(archivo))
		}
	}
}

// TestBorradoPorLote_DejaIndiceYWALLimpios cubre C-16: varios ids en una sola
// operación, acotada al proyecto.
func TestBorradoPorLote_DejaIndiceYWALLimpios(t *testing.T) {
	db, ruta := abrirBaseDeBorrado(t)
	viva := insertarMemoria(t, db, domain.Learning, "viva", "supervivienteunica conservar")
	uno := insertarMemoria(t, db, domain.Decision, "uno", "CANARIOLOTEUNO retirar")
	dos := insertarMemoria(t, db, domain.Decision, "dos", "CANARIOLOTEDOS retirar")
	ajena, err := InsertMemory(db, &domain.Memory{Project: "otro", Type: domain.Learning, Title: "ajena", Content: "de otro proyecto"})
	if err != nil {
		t.Fatal(err)
	}

	n, err := DeleteMemories(db, "proj", []int64{uno, dos, ajena})
	if err != nil || n != 2 {
		t.Fatalf("lote: %d %v", n, err)
	}
	if n := filasDelIndice(t, db); n != 2 {
		t.Fatalf("filas del índice = %d, quiero la viva (%d) y la ajena", n, viva)
	}
	if datos, err := os.ReadFile(ruta + "-wal"); err == nil && bytes.Contains(datos, []byte("CANARIOLOTE")) {
		t.Error("el texto del lote sigue en el WAL")
	}
	if n, err := DeleteMemories(db, "proj", nil); err != nil || n != 0 {
		t.Fatalf("un lote vacío no borra nada: %d %v", n, err)
	}
}

// TestMigracionHuerfanas_LimpiaUnaVez cubre C-04: una base de versión anterior
// pierde las filas huérfanas al abrirse y conserva las vivas.
func TestMigracionHuerfanas_LimpiaUnaVez(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	insertarMemoria(t, db, domain.Learning, "viva", "supervivienteunica conservar")
	if _, err := db.Exec(`INSERT INTO memory_search (rowid, title, content, memory_id) VALUES (9999, 't', 'huerfanaunica', 9999)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 6`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if n := filasDelIndice(t, db); n != 1 {
		t.Fatalf("filas del índice = %d, quiero 1", n)
	}
	halladas, err := SearchMemories(db, "proj", "supervivienteunica", 5)
	if err != nil || len(halladas) != 1 {
		t.Fatalf("la memoria viva debe seguir indexada: %v %v", halladas, err)
	}
}

// C-24: el saneamiento de privacidad no puede dejar secure_delete apagado en una
// conexión del pool.
func TestSecureDeleteActivo_TrasSaneamiento(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	db.SetMaxOpenConns(1)
	store := NewPrivacyMaintenanceStore(db)
	if err := store.WithTransaction(context.Background(), "proj", func(ports.PrivacyTransaction) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var valor int
	if err := db.QueryRow(`PRAGMA secure_delete`).Scan(&valor); err != nil {
		t.Fatal(err)
	}
	if valor != 1 {
		t.Fatalf("secure_delete = %d, quiero 1", valor)
	}
}

func contarEnDisco(t *testing.T, ruta, canario string) int {
	t.Helper()
	total := 0
	for _, archivo := range []string{ruta, ruta + "-wal"} {
		datos, err := os.ReadFile(archivo)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		total += bytes.Count(datos, []byte(canario))
	}
	return total
}

// abrirInstantanea deja una lectura en curso: una conexión ociosa no retiene el
// WAL, una instantánea sí.
func abrirInstantanea(t *testing.T, db *sql.DB) func() {
	t.Helper()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, `BEGIN DEFERRED`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM memories`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return func() {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		_ = conn.Close()
	}
}

// C-25 y C-26: con una instantánea abierta el borrado queda hecho y se avisa de
// que el texto sigue en disco; mem compact lo informa y, sin lector, lo retira.
func TestLectorAbierto_AvisaYCompactRetira(t *testing.T) {
	db, ruta := abrirBaseDeBorrado(t)
	insertarMemoria(t, db, domain.Decision, "canario", "CANARIOLECTOR retirar")
	cerrar := abrirInstantanea(t, db)

	n, err := PurgeMemories(db, ports.PurgeFilter{Project: "proj", Type: "decision"})
	if n != 1 || !errors.Is(err, domain.ErrLimpiezaPendiente) {
		t.Fatalf("purge: %d %v", n, err)
	}
	if !strings.Contains(err.Error(), "sigue en disco") {
		t.Fatalf("la advertencia debe decir que el texto sigue en disco: %v", err)
	}
	if _, _, err := CompactDB(db, ruta); !errors.Is(err, domain.ErrLimpiezaPendiente) {
		t.Fatalf("compact con lector debe avisar: %v", err)
	}

	cerrar()
	if _, _, err := CompactDB(db, ruta); err != nil {
		t.Fatalf("compact sin lector: %v", err)
	}
	if c := contarEnDisco(t, ruta, "CANARIOLECTOR"); c != 0 {
		t.Fatalf("el texto sigue en disco: %d", c)
	}
}

// C-26: compact hace checkpoint aunque el tamaño de la base no cambie.
func TestLectorAbierto_CompactVaciaElWALSinCambioDeTamano(t *testing.T) {
	db, ruta := abrirBaseDeBorrado(t)
	insertarMemoria(t, db, domain.Learning, "texto", "contenido en el wal")
	if _, _, err := CompactDB(db, ruta); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(ruta + "-wal")
	if err == nil && info.Size() != 0 {
		t.Fatalf("el WAL debe quedar vacío: %d", info.Size())
	}
}

// C-25: solo un bloqueo es limpieza pendiente.
func TestLimpiezaPendiente_SoloPorBloqueo(t *testing.T) {
	if !errors.Is(clasificarLimpieza(1, errors.New("database is locked (5) (SQLITE_BUSY)")), domain.ErrLimpiezaPendiente) {
		t.Error("un bloqueo es limpieza pendiente")
	}
	for _, causa := range []string{"database disk image is malformed (11)", "disk I/O error (10)", "database or disk is full (13)"} {
		err := clasificarLimpieza(1, errors.New(causa))
		if err == nil || errors.Is(err, domain.ErrLimpiezaPendiente) {
			t.Errorf("%s debe ser un fallo: %v", causa, err)
		}
		if !strings.Contains(err.Error(), "1 memoria") {
			t.Errorf("el fallo debe conservar el conteo: %v", err)
		}
	}
}

// C-28: lo que sigue vivo conserva su original; lo que no, se retira.
func TestCopiaViva_ConservaSoloLoQueSigueVivo(t *testing.T) {
	largo := "contenido suficientemente largo de una memoria repetida"
	casos := []struct {
		nombre, titulo, contenido, viva, original string
		retirar                                   bool
	}{
		{"viva idéntica", "t", largo, largo, largo, false},
		{"viva que lo incluye", "t", largo, "fusion\n\n---\n\n" + largo, largo, false},
		{"sin viva", "t", largo, "otra cosa distinta", largo, true},
		{"bloque mixto", "t", largo, "otra cosa distinta", "otra cosa distinta\n" + largo, false},
		{"corta con título", "Titulo", "corto", "otra cosa distinta", "Titulo: corto", true},
		{"corta viva", "Titulo", "corto", "corto", "Titulo: corto", false},
		{"corta dentro de otra viva", "Titulo", "corto", "esto es corto y largo", "Titulo: corto", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			db, _ := abrirBaseDeBorrado(t)
			id := insertarMemoria(t, db, domain.Decision, c.titulo, c.contenido)
			insertarMemoriaCruda(t, db, c.viva)
			almacen := NewOriginalStoreRepository(db, relojFijo{}, 1, 1)
			ref := guardarOriginal(t, almacen, "proj", c.original)

			if n, err := DeleteMemories(db, "proj", []int64{id}); err != nil || n != 1 {
				t.Fatalf("lote: %d %v", n, err)
			}
			if originalExiste(t, db, ref) == c.retirar {
				t.Fatalf("retirar=%v y el original existe=%v", c.retirar, originalExiste(t, db, ref))
			}
		})
	}
}

// IND-01: dos memorias borradas pueden compartir hash («t» + X y otra con contenido
// «t: X»). Basta que una de las dos siga viva para conservar el original.
func TestCopiaViva_HashCompartidoEntreBorradas(t *testing.T) {
	x := "contenido suficientemente largo que sigue vivo en otra memoria"
	for _, orden := range [][2]string{{"t", ""}, {"", "t"}} {
		db, _ := abrirBaseDeBorrado(t)
		var ids []int64
		for _, titulo := range orden {
			contenido := x
			if titulo == "" {
				contenido = "t: " + x
			}
			res, err := db.Exec(`INSERT INTO memories (project, type, title, content) VALUES ('proj', 'decision', ?, ?)`, titulo, contenido)
			if err != nil {
				t.Fatal(err)
			}
			id, _ := res.LastInsertId()
			ids = append(ids, id)
		}
		insertarMemoriaCruda(t, db, x)
		ref := guardarOriginal(t, NewOriginalStoreRepository(db, relojFijo{}, 1, 1), "proj", "t: "+x)
		if n, err := DeleteMemories(db, "proj", ids); err != nil || n != 2 {
			t.Fatalf("lote: %d %v", n, err)
		}
		if !originalExiste(t, db, ref) {
			t.Fatal("el original se borró aunque su contenido sigue vivo")
		}
	}
}

// P2-01: una memoria viva idéntica byte a byte conserva el original aunque el
// contenido breve lleve espacios que SQLite trim no retira.
func TestCopiaViva_ContenidoBreveConEspacios(t *testing.T) {
	for _, contenido := range []string{"\ttexto compartido\n", "texto compartido\r\n", "\u00a0texto compartido"} {
		db, _ := abrirBaseDeBorrado(t)
		id := insertarMemoria(t, db, domain.Decision, "", contenido)
		insertarMemoriaCruda(t, db, contenido)
		ref := guardarOriginal(t, NewOriginalStoreRepository(db, relojFijo{}, 1, 1), "proj", contenido)
		if n, err := DeleteMemories(db, "proj", []int64{id}); err != nil || n != 1 {
			t.Fatalf("lote: %d %v", n, err)
		}
		if !originalExiste(t, db, ref) {
			t.Fatalf("original compartido eliminado con memoria viva: %q", contenido)
		}
	}
}

// insertarMemoriaCruda evita la deduplicación de InsertMemory: dos memorias con
// el mismo contenido son justo el caso que se prueba.
func insertarMemoriaCruda(t *testing.T, db *sql.DB, contenido string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO memories (project, type, title, content) VALUES ('proj', 'learning', 'viva', ?)`, contenido); err != nil {
		t.Fatal(err)
	}
}

// C-31: memorias y copias se borran juntas: si fallan las copias, la memoria sigue.
func TestCopiasEnTransaccion_FalloNoBorraLaMemoria(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	id := insertarMemoria(t, db, domain.Decision, "memoria", "contenido que se intenta borrar")
	if _, err := db.Exec(`DROP TABLE compression_originals`); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteMemory(db, "proj", id); err == nil {
		t.Fatal("el fallo de las copias debe propagarse")
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM memories WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("la memoria no puede borrarse sin sus copias")
	}
}

// L-01: un lote grande no agota las variables de SQLite.
func TestBorradoPorLote_MuchosIDs(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	id := insertarMemoria(t, db, domain.Decision, "memoria", "contenido del lote")
	ids := make([]int64, 40000)
	for i := range ids {
		ids[i] = id + int64(i)
	}
	if n, err := DeleteMemories(db, "proj", ids); err != nil || n != 1 {
		t.Fatalf("lote: %d %v", n, err)
	}
}

// crearRestos deja texto antiguo en páginas libres, como una versión anterior.
func crearRestos(t *testing.T, db *sql.DB, canario string) {
	t.Helper()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA secure_delete = OFF`); err != nil {
		t.Fatal(err)
	}
	id := insertarMemoria(t, db, domain.Decision, "antigua", canario+" retirar")
	for i := 0; i < 70; i++ {
		insertarMemoria(t, db, domain.Learning, fmt.Sprintf("relleno %d", i), fmt.Sprintf("texto%d distinto %d", i, i))
	}
	for _, q := range []string{
		fmt.Sprintf(`DELETE FROM memories WHERE id = %d`, id),
		fmt.Sprintf(`DELETE FROM memory_search WHERE memory_id = %d`, id),
		`INSERT INTO memory_search (memory_search) VALUES ('optimize')`,
		`PRAGMA secure_delete = ON`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
}

// M-05: forget no reescribe la base; purge y compact sí retiran restos antiguos.
func TestAlcanceFisico_SoloPurgaYCompactReescriben(t *testing.T) {
	limpiar := map[string]func(*testing.T, *sql.DB, string, int64){
		"purge": func(t *testing.T, db *sql.DB, _ string, _ int64) {
			if _, err := PurgeMemories(db, ports.PurgeFilter{Project: "proj", Type: "bugfix"}); err != nil {
				t.Fatal(err)
			}
		},
		"compact": func(t *testing.T, db *sql.DB, ruta string, _ int64) {
			if _, _, err := CompactDB(db, ruta); err != nil {
				t.Fatal(err)
			}
		},
	}
	for nombre, ejecutar := range limpiar {
		t.Run(nombre, func(t *testing.T) {
			db, ruta := abrirBaseDeBorrado(t)
			crearRestos(t, db, "CANARIORESTO")
			id := insertarMemoria(t, db, domain.Bugfix, "reciente", "memoria reciente a borrar")
			if ok, err := DeleteMemory(db, "proj", insertarMemoria(t, db, domain.Learning, "otra", "otra memoria a olvidar")); err != nil || !ok {
				t.Fatalf("forget: %v %v", ok, err)
			}
			if c := contarEnDisco(t, ruta, "CANARIORESTO"); c == 0 {
				t.Fatal("forget no debe reescribir la base: los restos antiguos siguen")
			}
			ejecutar(t, db, ruta, id)
			if c := contarEnDisco(t, ruta, "CANARIORESTO"); c != 0 {
				t.Fatalf("quedan %d restos", c)
			}
		})
	}
}

// M-04: la limpieza tras borrar no espera todo el busy_timeout y lo restaura.
func TestEsperaCorta_LimpiezaNoBloquea(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	id := insertarMemoria(t, db, domain.Decision, "canario", "texto a olvidar")
	cerrar := abrirInstantanea(t, db)
	defer cerrar()
	inicio := time.Now()
	ok, err := DeleteMemory(db, "proj", id)
	if !ok || !errors.Is(err, domain.ErrLimpiezaPendiente) {
		t.Fatalf("forget: %v %v", ok, err)
	}
	if d := time.Since(inicio); d > 2*time.Second {
		t.Fatalf("la limpieza esperó %v", d)
	}
	var espera int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&espera); err != nil {
		t.Fatal(err)
	}
	if espera != esperaOcupadaMs {
		t.Fatalf("busy_timeout = %d", espera)
	}
}

// H-03 e IND-02: una base vaciada antes de actualizar también recibe la limpieza
// única, también sin versión de esquema.
func TestMigracionFisica_BaseVaciada(t *testing.T) {
	for _, version := range []int{6, 0} {
		dir := t.TempDir()
		db, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		crearRestos(t, db, "CANARIOVACIA")
		insertarMemoria(t, db, domain.Decision, "vaciada", "CANARIOVACIA2 retirar")
		for _, q := range []string{`PRAGMA secure_delete = OFF`, `DELETE FROM memories`, `DELETE FROM memory_search`, fmt.Sprintf(`PRAGMA user_version = %d`, version)} {
			if _, err := db.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if contarEnDisco(t, DbPath(dir), "CANARIOVACIA2") == 0 {
			t.Fatal("la base de prueba debe conservar restos")
		}
		db, err = Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if c := contarEnDisco(t, DbPath(dir), "CANARIOVACIA2"); c != 0 {
			t.Fatalf("versión %d: quedan %d restos en una base vaciada", version, c)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// C-29: la limpieza única retira restos de páginas libres sin filas huérfanas.
func TestMigracionFisica_RetiraRestosSinHuerfanas(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA secure_delete = OFF`); err != nil {
		t.Fatal(err)
	}
	id := insertarMemoria(t, db, domain.Decision, "canario", "CANARIOANTIGUO retirar")
	for i := 0; i < 70; i++ {
		insertarMemoria(t, db, domain.Learning, fmt.Sprintf("relleno %d", i), fmt.Sprintf("texto%d distinto %d", i, i))
	}
	for _, q := range []string{
		fmt.Sprintf(`DELETE FROM memories WHERE id = %d`, id),
		fmt.Sprintf(`DELETE FROM memory_search WHERE memory_id = %d`, id),
		`INSERT INTO memory_search (memory_search) VALUES ('optimize')`,
		`PRAGMA user_version = 6`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if c := contarEnDisco(t, DbPath(dir), "CANARIOANTIGUO"); c == 0 {
		t.Fatal("la base de prueba debe conservar restos")
	}

	db, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version = %d", version)
	}
	if c := contarEnDisco(t, DbPath(dir), "CANARIOANTIGUO"); c != 0 {
		t.Fatalf("quedan %d restos tras la limpieza única", c)
	}
}

// C-30: una tabla memory_search que no es FTS5 no impide abrir ni borrar.
func TestSinFTS_TablaPlanaNoImpideAbrirNiBorrar(t *testing.T) {
	definiciones := map[string]string{
		"sin memory_id": `CREATE TABLE memory_search (title TEXT, content TEXT)`,
		"con memory_id": `CREATE TABLE memory_search (title TEXT, content TEXT, memory_id INTEGER)`,
	}
	for nombre, definicion := range definiciones {
		t.Run(nombre, func(t *testing.T) {
			dir := t.TempDir()
			db, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			id := insertarMemoria(t, db, domain.Decision, "canario", "textounico retirar")
			viva := insertarMemoria(t, db, domain.Learning, "viva", "supervivienteunica conservar")
			for _, q := range []string{`DROP TABLE memory_search`, definicion, `PRAGMA user_version = 6`} {
				if _, err := db.Exec(q); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = Open(dir)
			if err != nil {
				t.Fatalf("abrir: %v", err)
			}
			defer func() { _ = db.Close() }()
			if ok, err := DeleteMemory(db, "proj", id); err != nil || !ok {
				t.Fatalf("borrar: %v %v", ok, err)
			}
			halladas, err := SearchMemories(db, "proj", "supervivienteunica", 5)
			if err != nil || len(halladas) != 1 || halladas[0].ID != viva {
				t.Fatalf("la búsqueda debe caer a LIKE: %v %v", halladas, err)
			}
		})
	}
}

// C-32: la retirada no descomprime: un original ilegible no bloquea ni avisa.
func TestOriginalIlegible_SeRetiraPorHash(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	contenido := "contenido suficientemente largo de la memoria"
	id := insertarMemoria(t, db, domain.Decision, "borrada", contenido)
	almacen := NewOriginalStoreRepository(db, relojFijo{}, 1, 1)
	ref := guardarOriginal(t, almacen, "proj", contenido)
	if _, err := db.Exec(`UPDATE compression_originals SET content_gz = x'00' WHERE ref = ?`, ref); err != nil {
		t.Fatal(err)
	}
	if ok, err := DeleteMemory(db, "proj", id); !ok || err != nil {
		t.Fatalf("forget: %v %v", ok, err)
	}
	if originalExiste(t, db, ref) {
		t.Fatal("la copia exacta debe retirarse por hash")
	}
}

// TestConsensoClaim_ReenvioConSecretoEsIdempotente cubre C-12: la persistencia
// redacta el Claim, así que la huella debe coincidir con la de las filas.
func TestConsensoClaim_ReenvioConSecretoEsIdempotente(t *testing.T) {
	db, _ := abrirBaseDeBorrado(t)
	repo := &ReviewRepository{db: db}
	target, _ := domain.NewTarget(domain.TargetDiff, "wt", "sha256:v0", nil)
	review := &domain.Review{ID: "acr_huella", Project: "proj", Target: target, Status: domain.ReviewAwaitingReviewers}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	fila := func(claim string) []domain.ConsensusFinding {
		return []domain.ConsensusFinding{{
			ConsensusLocalID: "C-001", Status: domain.ConsensusSuspect, Severity: domain.SeverityLow,
			Claim: claim, SourceFindingIDs: []int64{1},
		}}
	}
	conSecreto := "fuga <private>valor oculto</private>"
	huella := domain.ClassificationFingerprint(fila(conSecreto))
	if _, idem, err := repo.ReplaceConsensusRound("proj", review.ID, 0, huella, fila(conSecreto)); err != nil || idem {
		t.Fatalf("primera escritura: %v %v", idem, err)
	}
	if _, idem, err := repo.ReplaceConsensusRound("proj", review.ID, 0, huella, fila(conSecreto)); err != nil || !idem {
		t.Fatalf("el reenvío exacto debe ser idempotente: %v %v", idem, err)
	}
}
