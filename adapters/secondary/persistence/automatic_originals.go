package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mem/application/ports"
)

type AutomaticOriginalGrantsRepository struct{ db *sql.DB }

func NewAutomaticOriginalGrantsRepository(db *sql.DB) *AutomaticOriginalGrantsRepository {
	return &AutomaticOriginalGrantsRepository{db: db}
}

func (r *AutomaticOriginalGrantsRepository) Grant(ctx context.Context, scope ports.AutomaticOriginalScope, refs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin automatic grants: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, ref := range refs {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM compression_originals WHERE ref = ? AND project = ?", ref, scope.Project).Scan(&exists); err != nil {
			return fmt.Errorf("check grant original: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO automatic_original_grants(project,agent,session_id,ref) VALUES(?,?,?,?) ON CONFLICT DO NOTHING`, scope.Project, scope.Agent, scope.SessionID, ref); err != nil {
			return fmt.Errorf("insert automatic grant: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit automatic grants: %w", err)
	}
	return nil
}

func (r *AutomaticOriginalGrantsRepository) Allows(ctx context.Context, scope ports.AutomaticOriginalScope, ref string) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM automatic_original_grants WHERE project=? AND agent=? AND session_id=? AND ref=?`, scope.Project, scope.Agent, scope.SessionID, ref).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read automatic grant: %w", err)
	}
	return true, nil
}
