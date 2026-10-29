package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mem/domain"
)

func (s *HCDStore) NextMigrationID(ctx context.Context) (string, error) {
	var n int
	if err := s.q.QueryRowContext(ctx, "SELECT COALESCE(MAX(CAST(substr(id,3) AS INTEGER)),0)+1 FROM hcd_migrations WHERE project=?", s.project).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("M-%02d", n), nil
}

// SaveMigration inserta o actualiza la migración; el contenido clasificado va en proposal_json.
func (s *HCDStore) SaveMigration(ctx context.Context, m domain.Migration) error {
	proposal, err := json.Marshal(m.Items)
	if err != nil {
		return err
	}
	_, err = s.writeContext(ctx, `INSERT INTO hcd_migrations(project,id,unit_id,source_dir,source_rev,proposal_json,status,created_at,activated_at) VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(project,id) DO UPDATE SET unit_id=excluded.unit_id,status=excluded.status,activated_at=excluded.activated_at`,
		s.project, m.ID, m.UnitID, m.SourceDir, m.SourceRev, string(proposal), m.Status, m.CreatedAt, m.ActivatedAt)
	return err
}

func scanMigration(row interface{ Scan(...any) error }) (domain.Migration, error) {
	var m domain.Migration
	var proposal string
	if err := row.Scan(&m.ID, &m.UnitID, &m.SourceDir, &m.SourceRev, &proposal, &m.Status, &m.CreatedAt, &m.ActivatedAt); err != nil {
		return m, err
	}
	if err := json.Unmarshal([]byte(proposal), &m.Items); err != nil {
		return m, err
	}
	m.Recount()
	return m, nil
}

func (s *HCDStore) GetMigration(ctx context.Context, id string) (domain.Migration, error) {
	m, err := scanMigration(s.q.QueryRowContext(ctx, `SELECT id,unit_id,source_dir,source_rev,proposal_json,status,created_at,activated_at FROM hcd_migrations WHERE project=? AND id=?`, s.project, id))
	if errors.Is(err, sql.ErrNoRows) {
		return m, fmt.Errorf("HCD migration %s: %w", id, domain.ErrNotFound)
	}
	return m, err
}

func (s *HCDStore) ListMigrations(ctx context.Context) ([]domain.Migration, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT id,unit_id,source_dir,source_rev,proposal_json,status,created_at,activated_at FROM hcd_migrations WHERE project=? ORDER BY id`, s.project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Migration
	for rows.Next() {
		m, err := scanMigration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
