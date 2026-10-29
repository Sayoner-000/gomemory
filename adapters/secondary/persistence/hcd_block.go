package persistence

import (
	"context"
	"fmt"
	"mem/domain"
)

func (s *HCDStore) NextBlockID(ctx context.Context) (string, error) {
	var n int
	e := s.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(CAST(SUBSTR(id,3) AS INTEGER)),0)+1 FROM hcd_blocks WHERE project=?`, s.project).Scan(&n)
	return fmt.Sprintf("B-%02d", n), e
}
func (s *HCDStore) SaveBlock(ctx context.Context, b domain.Block) error {
	_, e := s.writeContext(ctx, `INSERT INTO hcd_blocks(project,id,unit_id,target_ref,reason,critical,status,resolution) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(project,id) DO UPDATE SET status=excluded.status,resolution=excluded.resolution`, s.project, b.ID, b.UnitID, b.TargetRef, domain.ProtectText(b.Reason), boolInt(b.Critical), b.Status, domain.ProtectText(b.Resolution))
	return e
}
func (s *HCDStore) ListBlocks(ctx context.Context, unit string) ([]domain.Block, error) {
	rows, e := s.q.QueryContext(ctx, `SELECT id,unit_id,target_ref,reason,critical,status,resolution FROM hcd_blocks WHERE project=? AND unit_id=? ORDER BY id`, s.project, unit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Block
	for rows.Next() {
		var b domain.Block
		var c int
		if e = rows.Scan(&b.ID, &b.UnitID, &b.TargetRef, &b.Reason, &c, &b.Status, &b.Resolution); e != nil {
			return nil, e
		}
		b.Critical = c != 0
		out = append(out, b)
	}
	return out, rows.Err()
}
