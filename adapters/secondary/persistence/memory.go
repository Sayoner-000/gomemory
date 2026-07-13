package persistence

import (
	"database/sql"
	"fmt"
	"strings"

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
	title := domain.RedactPrivate(m.Title)
	content := domain.RedactPrivate(m.Content)
	if strings.TrimSpace(content) == "" {
		return 0, fmt.Errorf("insert memory: contenido vacío tras redactar <private>")
	}

	// Provenance: si el llamador no fijó un prompt originante, se hereda del
	// último prompt de la sesión activa (lo persiste el hook/plugin por turno).
	// Choke point único: así TODAS las vías de guardado (MCP, checkpoint, CLI,
	// TUI) adjuntan la provenance sin tocar cada call site. Vacío si el agente
	// no expone el prompt (clientes MCP sin hooks) — degradación limpia.
	origin := domain.RedactPrivate(m.OriginPrompt)
	if strings.TrimSpace(origin) == "" {
		origin = activeSessionLastPrompt(db, m.Project)
	}

	res, err := db.Exec(
		`INSERT INTO memories (project, session_id, type, title, content, filepath, origin_prompt, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, `+Now+`, `+Now+`)`,
		m.Project, m.SessionID, string(m.Type), title, content, m.Filepath, origin,
	)
	if err != nil {
		return 0, fmt.Errorf("insert memory: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Consolidación sináptica ("siempre sinapsis"): en el mismo choke point que la
	// provenance, la memoria recién codificada forma una sinapsis con el engrama
	// sustantivo más reciente de su sesión. Determinista, sin tokens del agente y
	// transversal a todas las vías de guardado. Best-effort: una sinapsis fallida
	// NUNCA debe hacer fallar el guardado del engrama.
	formSynapse(db, m.Project, m.SessionID, id)

	return id, nil
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

func DeleteMemory(db *sql.DB, project string, id int64) (bool, error) {
	res, err := db.Exec(`DELETE FROM memories WHERE id = ? AND project = ?`, id, project)
	if err != nil {
		return false, fmt.Errorf("delete memory: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete memory: %w", err)
	}
	return affected > 0, nil
}

func SearchMemories(db *sql.DB, project, query string, limit int) ([]domain.Memory, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
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
