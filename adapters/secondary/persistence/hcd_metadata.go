package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mem/domain"
)

// SaveMetadata deja una intención durable en la misma transacción que cambia
// la ruta. Si el historial falla o el proceso termina, Show puede reintentarlo.
// Solo modifica los campos del renombre; nunca reabre ni reenfoca una unidad.
func (s *HCDStore) SaveMetadata(ctx context.Context, u domain.Unit, from string, queueRetarget bool) error {
	if err := domain.ValidateMemoryMetadata(u.DocPath, u.Slug); err != nil {
		return err
	}
	if err := domain.ValidateMemoryMetadata(from, ""); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var pending int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hcd_pending_retargets WHERE project = ? AND unit_id = ?`, s.project, u.ID).Scan(&pending); err != nil {
		return err
	}
	if pending != 0 {
		return fmt.Errorf("%w: reapunte pendiente de %s", domain.ErrNotAllowed, u.ID)
	}
	res, err := tx.ExecContext(ctx, `UPDATE hcd_units SET title = ?, slug = ?, doc_path = ?, updated_at = ? WHERE project = ? AND id = ? AND doc_path = ?`, u.Title, u.Slug, u.DocPath, u.UpdatedAt.UTC().Format(hcdTime), s.project, u.ID, from)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return fmt.Errorf("%w: ruta de %s cambió", domain.ErrNotAllowed, u.ID)
	}
	if queueRetarget && from != u.DocPath {
		if _, err = tx.ExecContext(ctx, `INSERT INTO hcd_pending_retargets(project, unit_id, from_path, to_path) VALUES(?,?,?,?)`, s.project, u.ID, from, u.DocPath); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *HCDStore) PendingRetarget(ctx context.Context, id string) (from, to string, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT from_path, to_path FROM hcd_pending_retargets WHERE project = ? AND unit_id = ?`, s.project, id).Scan(&from, &to)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return
}

func (s *HCDStore) CompleteRetarget(ctx context.Context, id, from, to string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM hcd_pending_retargets WHERE project = ? AND unit_id = ? AND from_path = ? AND to_path = ?`, s.project, id, from, to)
	return err
}
