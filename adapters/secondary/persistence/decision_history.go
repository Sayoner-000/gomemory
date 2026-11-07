package persistence

import (
	"database/sql"
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// DecisionHistoryRepository guarda las fichas del historial de decisiones
// (HCD-046) y su enlace con los archivos que citan.
type DecisionHistoryRepository struct{ db *sql.DB }

var (
	_ ports.DecisionHistory = (*DecisionHistoryRepository)(nil)
	_ ports.HistoryReader   = (*DecisionHistoryRepository)(nil)
)

const digestColumns = `id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
	COALESCE(filepath,''), COALESCE(origin_prompt,''), COALESCE(topic_key,''), COALESCE(source_review_id,''), created_at, updated_at`

// digestFilter elige las fichas por su prefijo de topic_key. substr evita
// que «_» actúe como comodín de LIKE.
const digestFilter = `(substr(topic_key, 1, 4) = 'hcd/' OR substr(topic_key, 1, 6) = 'audit/')`

// Digests devuelve las fichas del proyecto.
func (r *DecisionHistoryRepository) Digests(project string) ([]domain.Memory, error) {
	rows, err := r.db.Query(`SELECT `+digestColumns+` FROM memories WHERE project = ? AND `+digestFilter, project)
	if err != nil {
		return nil, fmt.Errorf("fichas del historial: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanMemories(rows)
}

// DigestsByFile devuelve las fichas enlazadas a path.
func (r *DecisionHistoryRepository) DigestsByFile(project, path string) ([]domain.Memory, error) {
	rows, err := r.db.Query(`SELECT `+digestColumns+` FROM memories WHERE project = ? AND `+digestFilter+`
		AND id IN (SELECT memory_id FROM memory_files WHERE project = ? AND path = ?)`, project, project, path)
	if err != nil {
		return nil, fmt.Errorf("fichas del archivo: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanMemories(rows)
}

func NewDecisionHistoryRepository(db *sql.DB) *DecisionHistoryRepository {
	return &DecisionHistoryRepository{db: db}
}

// UpsertDigest guarda la ficha por la misma vía que cualquier memoria (upsert
// por topic_key, protección de texto) y después reemplaza sus enlaces. Una
// ruta con texto protegido no se enlaza.
func (r *DecisionHistoryRepository) UpsertDigest(m *domain.Memory, files []string, inert bool) (int64, error) {
	id, err := insertMemory(r.db, m, insertOpts{inerte: inert})
	if err != nil {
		return 0, err
	}
	tx, err := beginMemoryTx(r.db)
	if err != nil {
		return id, fmt.Errorf("enlaces de la ficha: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM memory_files WHERE memory_id = ?`, id); err != nil {
		return id, fmt.Errorf("enlaces de la ficha: %w", err)
	}
	seen := map[string]bool{}
	for _, p := range files {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] || domain.NeedsSanitize(p) {
			continue
		}
		seen[p] = true
		if _, err := tx.Exec(`INSERT INTO memory_files (project, memory_id, path) VALUES (?, ?, ?)`, m.Project, id, p); err != nil {
			return id, fmt.Errorf("enlaces de la ficha: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return id, fmt.Errorf("enlaces de la ficha: %w", err)
	}
	return id, nil
}
