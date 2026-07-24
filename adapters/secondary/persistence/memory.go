package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// SecondsSinceLastSave devuelve cuántos segundos pasaron desde la última
// memoria REAL del proyecto, excluyendo los checkpoints automáticos (type =
// 'checkpoint') que se insertan en cada turno con actividad: esos no reflejan
// que el agente haya registrado un aprendizaje, así que no deben reiniciar el
// reloj del recordatorio de guardado. El segundo valor es false si el proyecto
// todavía no tiene ninguna memoria real. El offset '-5 hours' es el mismo que
// usa Now al escribir created_at (reusado literalmente aquí), así que se cancela
// en la resta y el resultado es tiempo real transcurrido.
func SecondsSinceLastSave(db *sql.DB, project string) (int64, bool, error) {
	var secs sql.NullFloat64
	err := db.QueryRow(
		`SELECT (julianday(`+Now+`) - julianday(MAX(created_at))) * 86400
		 FROM memories WHERE project = ? AND type != 'checkpoint'`,
		project,
	).Scan(&secs)
	if err != nil {
		return 0, false, fmt.Errorf("seconds since last save: %w", err)
	}
	if !secs.Valid {
		return 0, false, nil
	}
	return int64(secs.Float64), true, nil
}

func InsertMemory(db *sql.DB, m *domain.Memory) (int64, error) {
	title := domain.RedactSecrets(domain.RedactPrivate(m.Title))
	content := domain.RedactSecrets(domain.RedactPrivate(m.Content))
	if strings.TrimSpace(content) == "" {
		return 0, fmt.Errorf("insert memory: contenido vacío tras redactar <private>")
	}

	// Anotación de impacto (feature 010, Historia 1): mismo choke point que
	// provenance/sinapsis. Best-effort y no-bloqueante — codeImpactProvider
	// solo lee su snapshot cacheado (ver ports.CodeGraphProvider.ImpactFor),
	// nunca invoca al proveedor externo desde acá.
	content = annotateImpact(content, m.Filepath)

	// Provenance: si el llamador no fijó un prompt originante, se hereda del
	// último prompt de la sesión activa (lo persiste el hook/plugin por turno).
	// Choke point único: así TODAS las vías de guardado (MCP, checkpoint, CLI,
	// TUI) adjuntan la provenance sin tocar cada call site. Vacío si el agente
	// no expone el prompt (clientes MCP sin hooks) — degradación limpia.
	origin := domain.RedactSecrets(domain.RedactPrivate(m.OriginPrompt))
	if strings.TrimSpace(origin) == "" {
		origin = activeSessionLastPrompt(db, m.Project)
	}

	// Dedup/upsert en la fuente (feature 008): consolida una memoria equivalente
	// ya existente en vez de crear una fila nueva, para que el contexto no se
	// infle con repeticiones. Best-effort: si no hay match, sigue el INSERT normal.
	if existingID, ok := findDuplicate(db, m, title); ok {
		if _, err := db.Exec(
			`UPDATE memories SET content = ?, title = ?, type = ?, filepath = ?, topic_key = ?, updated_at = `+Now+`
			 WHERE id = ?`,
			content, title, string(m.Type), m.Filepath, nullableTopic(m.TopicKey), existingID,
		); err != nil {
			return 0, fmt.Errorf("update memory (dedup): %w", err)
		}
		upsertMemorySearch(db, existingID, title, content)
		exportToADR(m.Project, m.Type, title, content, existingID)
		return existingID, nil
	}

	res, err := db.Exec(
		`INSERT INTO memories (project, session_id, type, title, content, filepath, origin_prompt, topic_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, `+Now+`, `+Now+`)`,
		m.Project, m.SessionID, string(m.Type), title, content, m.Filepath, origin, nullableTopic(m.TopicKey),
	)
	if err != nil {
		return 0, fmt.Errorf("insert memory: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	upsertMemorySearch(db, id, title, content)

	// Consolidación sináptica ("siempre sinapsis"): en el mismo choke point que la
	// provenance, la memoria recién codificada forma una sinapsis con el engrama
	// sustantivo más reciente de su sesión. Determinista, sin tokens del agente y
	// transversal a todas las vías de guardado. Best-effort: una sinapsis fallida
	// NUNCA debe hacer fallar el guardado del engrama.
	formSynapse(db, m.Project, m.SessionID, id)

	exportToADR(m.Project, m.Type, title, content, id)
	return id, nil
}

// upsertMemorySearch mantiene memory_search (FTS5, specs/009-mitigacion-riesgos
// Historia de Usuario 3) sincronizada 1:1 con memories: actualiza la fila de
// índice si ya existía (consolidación por dedup) o la crea si es nueva.
// Best-effort y silencioso: si memory_search no existe (build sin soporte
// FTS5, ver migrate() en db.go), ambos Exec fallan sin afectar el guardado de
// la memoria — SearchMemories cae a LIKE en ese caso.
func upsertMemorySearch(db *sql.DB, id int64, title, content string) {
	res, err := db.Exec(`UPDATE memory_search SET title = ?, content = ? WHERE memory_id = ?`, title, content, id)
	if err != nil {
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return
	}
	db.Exec(`INSERT INTO memory_search (rowid, title, content, memory_id) VALUES (?, ?, ?, ?)`, id, title, content, id)
}

// deleteMemorySearch borra la fila de índice asociada a una memoria borrada.
// Best-effort: ver upsertMemorySearch.
func deleteMemorySearch(db *sql.DB, id int64) {
	db.Exec(`DELETE FROM memory_search WHERE memory_id = ?`, id)
}

// dedupWindowDays es la ventana (días) del dedup por identidad. Singleton de
// proceso: el composition root lo fija desde settings (SetDedupWindowDays);
// default seguro. <=0 desactiva el dedup por identidad (el upsert por topic_key
// sigue vigente, es explícito).
var dedupWindowDays = DefaultDedupWindowDays

// SetDedupWindowDays ajusta la ventana de dedup por identidad (feature 008).
func SetDedupWindowDays(n int) { dedupWindowDays = n }

// codeImpactProvider es el proveedor de grafo de código "activo" para
// anotación de impacto (feature 010, Historia 1). nil = capacidad
// desactivada (sin proveedor disponible, o `code_impact_annotation_disabled`
// en settings) — mismo patrón singleton que dedupWindowDays: el composition
// root lo fija una vez al construir el proceso.
var codeImpactProvider ports.CodeGraphProvider

// SetCodeImpactProvider fija (o apaga, con nil) el proveedor usado por
// annotateImpact. Llamarlo con nil es el modo por defecto: InsertMemory no
// toca el content de nadie hasta que el composition root lo active.
func SetCodeImpactProvider(p ports.CodeGraphProvider) { codeImpactProvider = p }

// annotateImpact anexa una nota de impacto al content cuando filepath casa
// con un hotspot del proveedor activo. Best-effort y silencioso: sin
// proveedor, sin filepath, o sin match, devuelve content sin tocar — nunca
// hace fallar el guardado ni añade latencia perceptible (ImpactFor solo lee
// el snapshot cacheado).
func annotateImpact(content, filepath string) string {
	if codeImpactProvider == nil || strings.TrimSpace(filepath) == "" {
		return content
	}
	ann, ok := codeImpactProvider.ImpactFor(filepath)
	if !ok || !ann.Hotspot {
		return content
	}
	return fmt.Sprintf("%s\n\n[impacto: %s es un hotspot con %d llamadores directos]", content, ann.Symbol, ann.FanIn)
}

// adrSyncProvider/adrSyncRepo/settingsAdrSyncEnabled: sincronización
// bidireccional de ADR (feature 010, Historia 2). Mismo patrón singleton que
// codeImpactProvider — nil/false = capacidad desactivada (default: opt-in
// explícito, a diferencia de la anotación de impacto que es opt-out).
var (
	adrSyncProvider        ports.ADRSyncProvider
	adrSyncRepo            ports.ADRSyncRepository
	settingsAdrSyncEnabled bool
)

// SetADRSync fija (o apaga, con ambos nil) el proveedor y repositorio usados
// por exportToADR.
func SetADRSync(provider ports.ADRSyncProvider, repo ports.ADRSyncRepository) {
	adrSyncProvider = provider
	adrSyncRepo = repo
}

// SetAdrSyncEnabled activa/desactiva el export a ADR (default false).
func SetAdrSyncEnabled(enabled bool) { settingsAdrSyncEnabled = enabled }

// adrSyncTimeout acota el peor caso de exportToADR (dos llamadas CLI:
// get + update). Corre SÍNCRONO dentro de InsertMemory a propósito —no en
// una goroutine— para que el estado de adr_sync_records sea observable
// inmediatamente después de que InsertMemory retorna (útil para tests y para
// `mem adr-sync status`); el timeout acotado + que la capacidad es opt-in
// (default false) es lo que mantiene el guardado sin demora perceptible en
// el caso común, en vez de asincronía real.
const adrSyncTimeout = 4 * time.Second

// adrSectionForType mapea el tipo de memoria a una de las 6 secciones fijas
// del documento único de ADR del proveedor (no hay sección "decisions" en su
// esquema, así que decision cae en TRADEOFFS — heurística documentada en
// specs/010-codegraph-integration-evolution/research.md §2).
func adrSectionForType(t domain.MemoryType) (string, bool) {
	switch t {
	case domain.Architecture:
		return domain.ADRSectionArchitecture, true
	case domain.Decision:
		return domain.ADRSectionTradeoffs, true
	default:
		return "", false
	}
}

// exportToADR refleja una memoria architecture/decision como bloque marcado
// dentro del documento único de ADR del proveedor externo (gomemory→
// proveedor). Best-effort en cada paso: cualquier fallo (proveedor
// desactivado, no disponible, error de escritura) deja constancia en
// adr_sync_records sin jamás propagar un error a InsertMemory.
func exportToADR(project string, memType domain.MemoryType, title, content string, id int64) {
	if adrSyncProvider == nil || adrSyncRepo == nil || !settingsAdrSyncEnabled {
		return
	}
	section, ok := adrSectionForType(memType)
	if !ok {
		return
	}
	heading := strings.TrimSpace(title)
	if heading == "" {
		heading = fmt.Sprintf("Memoria #%d", id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), adrSyncTimeout)
	defer cancel()

	existing, _ := adrSyncRepo.GetByMemory(project, id)
	hash := contentHash(content)

	// Si no se puede LEER el documento actual, no se debe escribir nada:
	// reconstruir un doc "vacío" y hacer UpdateDocument lo pisaría por
	// completo en el proveedor. Se registra pending y se reintenta en el
	// próximo guardado/refresco.
	docContent, err := adrSyncProvider.GetDocument(ctx)
	if err != nil {
		recordADRSyncAttempt(project, id, section, existing, domain.SyncStatusPending, hash)
		return
	}

	doc := domain.ParseADRDocument(docContent)
	doc.UpsertBlock(section, id, heading, content)

	if err := adrSyncProvider.UpdateDocument(ctx, doc.Render()); err != nil {
		recordADRSyncAttempt(project, id, section, existing, domain.SyncStatusFailed, hash)
		return
	}
	recordADRSyncAttempt(project, id, section, existing, domain.SyncStatusOK, hash)
}

// recordADRSyncAttempt deja constancia del resultado de exportToADR:
// actualiza el registro existente de la memoria si ya había uno (nunca
// duplica), o crea uno nuevo con origin=gomemory y block_key=id=<memoryID>
// (identidad estable del lado gomemory, ver ADRDocument). Best-effort: un
// fallo acá nunca se propaga.
func recordADRSyncAttempt(project string, memID int64, section string, existing *domain.ADRSyncRecord, status domain.SyncStatus, hash string) {
	if existing != nil {
		adrSyncRepo.UpdateStatus(existing.ID, status, hash)
		return
	}
	id := memID
	adrSyncRepo.Insert(&domain.ADRSyncRecord{
		Project: project, MemoryID: &id, Provider: adrSyncProvider.Name(),
		Section: section, BlockKey: fmt.Sprintf("id=%d", memID),
		Origin: domain.SyncOriginGomemory, Status: status, ContentHash: hash,
	})
}

func contentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// findDuplicate localiza una memoria existente que la nueva debe consolidar en
// lugar de duplicar:
//   - por topic_key (explícito, cualquier tipo): agrupa revisiones del mismo tópico;
//   - por identidad (mismo project+type+title dentro de la ventana): NUNCA aplica
//     a checkpoints (su contenido varía por turno) ni a memorias sin título (un
//     título vacío no es una clave de dedup fiable).
//
// Best-effort: ante cualquier error o ausencia de match, (0,false) ⇒ INSERT normal.
func findDuplicate(db *sql.DB, m *domain.Memory, title string) (int64, bool) {
	if tk := strings.TrimSpace(m.TopicKey); tk != "" {
		var id int64
		if err := db.QueryRow(
			`SELECT id FROM memories WHERE project = ? AND topic_key = ? ORDER BY id DESC LIMIT 1`,
			m.Project, tk,
		).Scan(&id); err == nil {
			return id, true
		}
		return 0, false // topic explícito sin match: no cae a identidad, el tópico manda.
	}

	if dedupWindowDays <= 0 || m.Type == domain.Checkpoint || strings.TrimSpace(title) == "" {
		return 0, false
	}
	var id int64
	if err := db.QueryRow(
		`SELECT id FROM memories
		 WHERE project = ? AND type = ? AND title = ? AND type != 'checkpoint'
		   AND julianday(`+Now+`) - julianday(created_at) <= ?
		 ORDER BY id DESC LIMIT 1`,
		m.Project, string(m.Type), title, dedupWindowDays,
	).Scan(&id); err == nil {
		return id, true
	}
	return 0, false
}

// nullableTopic devuelve nil para un topic_key vacío (así el índice parcial solo
// indexa filas con tópico y las memorias sin tópico no colisionan entre sí).
func nullableTopic(tk string) any {
	if strings.TrimSpace(tk) == "" {
		return nil
	}
	return tk
}

// formSynapse crea la arista de consolidación sináptica de la memoria recién
// insertada (newID) hacia el "ancla" de su sesión: el engrama sustantivo (no
// checkpoint) más reciente registrado antes en la misma sesión. Así se teje el
// hilo de decisiones de una sesión y cada checkpoint queda enlazado a la decisión
// que lo gobierna, sin generar ruido checkpoint↔checkpoint. Idempotente (no
// duplica una arista existente) y best-effort (traga cualquier error).
func formSynapse(db *sql.DB, project, sessionID string, newID int64) {
	if strings.TrimSpace(sessionID) == "" {
		return // Sin sesión no hay co-activación que enlazar.
	}

	var anchorID int64
	err := db.QueryRow(
		`SELECT id FROM memories
		 WHERE project = ? AND session_id = ? AND id <> ? AND type <> ?
		 ORDER BY id DESC LIMIT 1`,
		project, sessionID, newID, string(domain.Checkpoint),
	).Scan(&anchorID)
	if err != nil {
		return // sql.ErrNoRows (aún no hay ancla) o cualquier error: no enlazar.
	}

	if existing, _ := GetRelationByPair(db, project, newID, anchorID); existing != nil {
		return // Ya sinaptizadas: no duplicar.
	}

	InsertRelation(db, &domain.Relation{
		Project:    project,
		MemoryIDA:  newID,
		MemoryIDB:  anchorID,
		Relation:   domain.Related,
		Confidence: 0.5,
		Reasoning:  "sinapsis auto: co-activadas en la misma sesión de trabajo",
	})
}

// activeSessionLastPrompt devuelve el último prompt registrado en la sesión
// activa del proyecto (o "" si no hay sesión activa o prompt). Best-effort: ante
// cualquier error devuelve "" para no bloquear el guardado.
func activeSessionLastPrompt(db *sql.DB, project string) string {
	var p sql.NullString
	err := db.QueryRow(
		`SELECT last_prompt FROM sessions
		 WHERE project = ? AND ended_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		project,
	).Scan(&p)
	if err != nil || !p.Valid {
		return ""
	}
	return p.String
}

// GetMemoryByID devuelve una memoria por ID dentro de un proyecto, o nil si
// no existe (ID inexistente o de otro proyecto).
func GetMemoryByID(db *sql.DB, project string, id int64) (*domain.Memory, error) {
	var m domain.Memory
	var memType string
	err := db.QueryRow(
		`SELECT id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
		        COALESCE(filepath,''), COALESCE(origin_prompt,''), created_at, updated_at
		 FROM memories WHERE id = ? AND project = ?`,
		id, project,
	).Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title, &m.Content, &m.Filepath, &m.OriginPrompt, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get memory by id: %w", err)
	}
	m.Type = domain.MemoryType(memType)
	return &m, nil
}

// UpdateMemoryContent actualiza título/contenido de una memoria existente
// SIN pasar por el choke point de InsertMemory: no forma sinapsis ni dispara
// exportToADR (mismo criterio "sin efectos en cadena" que ImportMemory) —
// usado por ImportADRs para reflejar un bloque de ADR actualizado sobre la
// memoria ya importada, sin reexportarla de vuelta al proveedor.
func UpdateMemoryContent(db *sql.DB, project string, id int64, title, content string) error {
	redTitle := domain.RedactSecrets(domain.RedactPrivate(title))
	redContent := domain.RedactSecrets(domain.RedactPrivate(content))
	if strings.TrimSpace(redContent) == "" {
		return fmt.Errorf("update memory content: contenido vacío tras redactar <private>")
	}
	res, err := db.Exec(
		`UPDATE memories SET title = ?, content = ?, updated_at = `+Now+` WHERE id = ? AND project = ?`,
		redTitle, redContent, id, project,
	)
	if err != nil {
		return fmt.Errorf("update memory content: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memory %d not found in project %s", id, project)
	}
	upsertMemorySearch(db, id, redTitle, redContent)
	return nil
}

func ListMemories(db *sql.DB, project string, limit int) ([]domain.Memory, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
		        COALESCE(filepath,''), COALESCE(origin_prompt,''), created_at, updated_at
		 FROM memories WHERE project = ? ORDER BY created_at DESC LIMIT ?`,
		project, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var mems []domain.Memory
	for rows.Next() {
		var m domain.Memory
		var memType string
		err := rows.Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title,
			&m.Content, &m.Filepath, &m.OriginPrompt, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.Type = domain.MemoryType(memType)
		mems = append(mems, m)
	}
	if mems == nil {
		mems = []domain.Memory{}
	}
	return mems, rows.Err()
}

// ListAllMemories devuelve TODAS las memorias del proyecto, en orden estable por
// id (sin tope), para el export. Orden por id ASC para que los RefID crezcan de
// forma reproducible entre exports.
func ListAllMemories(db *sql.DB, project string) ([]domain.Memory, error) {
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
		        COALESCE(filepath,''), COALESCE(origin_prompt,''), created_at, updated_at
		 FROM memories WHERE project = ? ORDER BY id ASC`,
		project,
	)
	if err != nil {
		return nil, fmt.Errorf("list all memories: %w", err)
	}
	defer rows.Close()

	var mems []domain.Memory
	for rows.Next() {
		var m domain.Memory
		var memType string
		if err := rows.Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title,
			&m.Content, &m.Filepath, &m.OriginPrompt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.Type = domain.MemoryType(memType)
		mems = append(mems, m)
	}
	if mems == nil {
		mems = []domain.Memory{}
	}
	return mems, rows.Err()
}

// ImportMemory inserta una memoria PRESERVANDO sus timestamps de origen y SIN
// formar la sinapsis automática (formSynapse) — las relaciones del bundle se
// importan explícitamente aparte. Redacta <private> como InsertMemory (seguro e
// idempotente sobre datos ya redactados). Si created_at/updated_at vienen vacíos
// se usa el reloj local.
func ImportMemory(db *sql.DB, m *domain.Memory) (int64, error) {
	title := domain.RedactSecrets(domain.RedactPrivate(m.Title))
	content := domain.RedactSecrets(domain.RedactPrivate(m.Content))
	if strings.TrimSpace(content) == "" {
		return 0, fmt.Errorf("import memory: contenido vacío tras redactar <private>")
	}
	origin := domain.RedactSecrets(domain.RedactPrivate(m.OriginPrompt))

	res, err := db.Exec(
		`INSERT INTO memories (project, session_id, type, title, content, filepath, origin_prompt, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?,''), `+Now+`), COALESCE(NULLIF(?,''), `+Now+`))`,
		m.Project, m.SessionID, string(m.Type), title, content, m.Filepath, origin, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("import memory: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	upsertMemorySearch(db, id, title, content)
	return id, nil
}

func DeleteMemory(db *sql.DB, project string, id int64) (bool, error) {
	res, err := db.Exec(`DELETE FROM memories WHERE id = ? AND project = ?`, id, project)
	if err != nil {
		return false, fmt.Errorf("delete memory: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete memory: %w", err)
	}
	if affected > 0 {
		deleteMemorySearch(db, id)
	}
	return affected > 0, nil
}

// SearchMemories busca memorias por relevancia (specs/009-mitigacion-riesgos,
// Historia de Usuario 3): intenta primero FTS5+bm25 (searchMemoriesFTS) y cae a
// LIKE (searchMemoriesLike) si memory_search no existe (build sin soporte FTS5)
// o cualquier otro error de la ruta FTS — mismo patrón que SearchCodeNodes.
func SearchMemories(db *sql.DB, project, query string, limit int) ([]domain.Memory, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mems, err := searchMemoriesFTS(db, project, query, limit); err == nil {
		return mems, nil
	}
	return searchMemoriesLike(db, project, query, limit)
}

func searchMemoriesFTS(db *sql.DB, project, query string, limit int) ([]domain.Memory, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search memories fts: query vacía")
	}
	ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	rows, err := db.Query(
		`SELECT m.id, m.project, COALESCE(m.session_id,''), m.type, COALESCE(m.title,''), m.content,
		        COALESCE(m.filepath,''), COALESCE(m.origin_prompt,''), m.created_at, m.updated_at
		 FROM memory_search s
		 JOIN memories m ON m.id = s.memory_id
		 WHERE s.memory_search MATCH ? AND m.project = ?
		 ORDER BY rank
		 LIMIT ?`,
		ftsQuery, project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func searchMemoriesLike(db *sql.DB, project, query string, limit int) ([]domain.Memory, error) {
	like := "%" + query + "%"
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
		        COALESCE(filepath,''), COALESCE(origin_prompt,''), created_at, updated_at
		 FROM memories WHERE project = ? AND (content LIKE ? OR title LIKE ?)
		 ORDER BY
		   CASE
		     WHEN title LIKE ? THEN 0
		     WHEN content LIKE ? THEN 1
		     ELSE 2
		   END,
		   created_at DESC
		 LIMIT ?`,
		project, like, like, like, like, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// scanMemories escanea filas con el orden de columnas común a
// searchMemoriesFTS y searchMemoriesLike (id, project, session_id, type,
// title, content, filepath, origin_prompt, created_at, updated_at).
func scanMemories(rows *sql.Rows) ([]domain.Memory, error) {
	var mems []domain.Memory
	for rows.Next() {
		var m domain.Memory
		var memType string
		err := rows.Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title,
			&m.Content, &m.Filepath, &m.OriginPrompt, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.Type = domain.MemoryType(memType)
		mems = append(mems, m)
	}
	if mems == nil {
		mems = []domain.Memory{}
	}
	return mems, rows.Err()
}
