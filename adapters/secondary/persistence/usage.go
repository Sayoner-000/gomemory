package persistence

import (
	"database/sql"
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// RecordUsage inserta una emisión de contexto medida. Parámetros bind,
// timestamp con la constante Now (UTC-5, Principio II).
func RecordUsage(db *sql.DB, rec domain.UsageRecord) error {
	_, err := db.Exec(
		`INSERT INTO usage_records (project, session_id, operation, channel, baseline_tokens, emitted_tokens)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		rec.Project, nullableString(rec.SessionID), rec.Operation, rec.Channel,
		rec.BaselineTokens, rec.EmittedTokens,
	)
	if err != nil {
		return fmt.Errorf("record usage: %w", err)
	}
	return nil
}

// UsageBySession devuelve los registros de una sesión. Slice vacío, no error,
// si no hay ninguno.
func UsageBySession(db *sql.DB, project, sessionID string) ([]domain.UsageRecord, error) {
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), operation, channel, baseline_tokens, emitted_tokens, created_at
		 FROM usage_records WHERE project = ? AND session_id = ? ORDER BY id ASC`,
		project, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("usage by session: %w", err)
	}
	defer rows.Close()
	return scanUsageRecords(rows)
}

// UsageSessions devuelve los identificadores de sesión con registros, del más
// reciente al más antiguo.
func UsageSessions(db *sql.DB, project string, limit int) ([]string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT session_id FROM usage_records
		 WHERE project = ? AND session_id IS NOT NULL AND session_id != ''
		 GROUP BY session_id ORDER BY MAX(created_at) DESC LIMIT ?`,
		project, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("usage sessions: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("usage sessions scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UsageTotals devuelve todos los registros del proyecto (ámbito "todas las
// sesiones").
func UsageTotals(db *sql.DB, project string) ([]domain.UsageRecord, error) {
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), operation, channel, baseline_tokens, emitted_tokens, created_at
		 FROM usage_records WHERE project = ? ORDER BY id ASC`,
		project,
	)
	if err != nil {
		return nil, fmt.Errorf("usage totals: %w", err)
	}
	defer rows.Close()
	return scanUsageRecords(rows)
}

func scanUsageRecords(rows *sql.Rows) ([]domain.UsageRecord, error) {
	var recs []domain.UsageRecord
	for rows.Next() {
		var r domain.UsageRecord
		if err := rows.Scan(&r.ID, &r.Project, &r.SessionID, &r.Operation, &r.Channel,
			&r.BaselineTokens, &r.EmittedTokens, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan usage record: %w", err)
		}
		recs = append(recs, r)
	}
	return recs, rows.Err()
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// UsageRepository implementa ports.UsageRepository.
type UsageRepository struct {
	db *sql.DB
}

func NewUsageRepository(db *sql.DB) ports.UsageRepository {
	return &UsageRepository{db: db}
}

func (r *UsageRepository) Record(rec domain.UsageRecord) error {
	return RecordUsage(r.db, rec)
}

func (r *UsageRepository) BySession(project, sessionID string) ([]domain.UsageRecord, error) {
	return UsageBySession(r.db, project, sessionID)
}

func (r *UsageRepository) Sessions(project string, limit int) ([]string, error) {
	return UsageSessions(r.db, project, limit)
}

func (r *UsageRepository) Totals(project string) ([]domain.UsageRecord, error) {
	return UsageTotals(r.db, project)
}

var _ ports.UsageRepository = (*UsageRepository)(nil)
