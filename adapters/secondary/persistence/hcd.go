package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mem/domain"
	"strings"
	"time"
)

// HCDStore implementa ports.HCDStore sobre las tablas hcd_* (feature 045).
type HCDStore struct {
	db      *sql.DB
	project string
}

func NewHCDStore(db *sql.DB, project string) *HCDStore { return &HCDStore{db: db, project: project} }

const hcdTime = time.RFC3339Nano

const hcdUnitColumns = "id, number, title, slug, kind, lane, why, state, active, doc_path, created_at, updated_at"

func (s *HCDStore) NextNumber(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(number), 0) + 1 FROM hcd_units WHERE project = ?", s.project).Scan(&n)
	return n, err
}

func (s *HCDStore) Create(ctx context.Context, u domain.Unit) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO hcd_units(project, `+hcdUnitColumns+`) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.project, u.ID, u.Number, u.Title, u.Slug, string(u.Kind), string(u.Lane), u.Why, string(u.State), boolToInt(u.Active), u.DocPath, u.CreatedAt.UTC().Format(hcdTime), u.UpdatedAt.UTC().Format(hcdTime))
	return err
}

func (s *HCDStore) Get(ctx context.Context, id string) (domain.Unit, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+hcdUnitColumns+" FROM hcd_units WHERE project = ? AND id = ?", s.project, id)
	u, err := scanHCDUnit(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Unit{}, fmt.Errorf("%w: unidad %s", domain.ErrNotFound, id)
	}
	return u, err
}

func (s *HCDStore) Active(ctx context.Context) (domain.Unit, bool, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+hcdUnitColumns+" FROM hcd_units WHERE project = ? AND active = 1 LIMIT 1", s.project)
	u, err := scanHCDUnit(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Unit{}, false, nil
	}
	return u, err == nil, err
}

func (s *HCDStore) Activate(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE hcd_units SET active = 0 WHERE project = ? AND active = 1", s.project); err != nil {
		return err
	}
	if id != "" {
		res, err := tx.ExecContext(ctx, "UPDATE hcd_units SET active = 1 WHERE project = ? AND id = ?", s.project, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: unidad %s", domain.ErrNotFound, id)
		}
	}
	return tx.Commit()
}

func (s *HCDStore) Save(ctx context.Context, u domain.Unit) error {
	res, err := s.db.ExecContext(ctx, `UPDATE hcd_units SET title = ?, lane = ?, why = ?, state = ?, active = ?, doc_path = ?, updated_at = ? WHERE project = ? AND id = ?`,
		u.Title, string(u.Lane), u.Why, string(u.State), boolToInt(u.Active), u.DocPath, u.UpdatedAt.UTC().Format(hcdTime), s.project, u.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: unidad %s", domain.ErrNotFound, u.ID)
	}
	return nil
}

func (s *HCDStore) AddEvidence(ctx context.Context, e domain.Evidence) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO hcd_evidence(id, project, unit_id, tasks, kind, expect_fail, command, exit_code, excerpt, revision, fingerprint, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, s.project, e.UnitID, strings.Join(e.Tasks, ","), e.Kind, boolToInt(e.ExpectFail), e.Command, e.ExitCode, e.Excerpt, e.Revision, e.Fingerprint, e.CreatedAt.UTC().Format(hcdTime))
	return err
}

func (s *HCDStore) Evidence(ctx context.Context, unitID string) ([]domain.Evidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, tasks, kind, expect_fail, command, exit_code, excerpt, revision, fingerprint, created_at FROM hcd_evidence WHERE project = ? AND unit_id = ? ORDER BY created_at, id`, s.project, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Evidence
	for rows.Next() {
		e := domain.Evidence{UnitID: unitID}
		var tasks, created string
		var expectFail int
		if err := rows.Scan(&e.ID, &tasks, &e.Kind, &expectFail, &e.Command, &e.ExitCode, &e.Excerpt, &e.Revision, &e.Fingerprint, &created); err != nil {
			return nil, err
		}
		e.Tasks = strings.Split(tasks, ",")
		e.ExpectFail = expectFail == 1
		e.CreatedAt, _ = time.Parse(hcdTime, created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *HCDStore) AddApproval(ctx context.Context, a domain.Approval) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO hcd_approvals(id, project, unit_id, action, revision, fingerprint, origin, client, quote, constitution_rev, playbook_rev, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, s.project, a.UnitID, a.Action, a.Revision, a.Fingerprint, a.Origin, a.Client, a.Quote, a.ConstitutionRev, a.PlaybookRev, a.CreatedAt.UTC().Format(hcdTime))
	return err
}

func (s *HCDStore) Approvals(ctx context.Context, unitID string) ([]domain.Approval, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, action, revision, fingerprint, origin, client, quote, constitution_rev, playbook_rev, created_at FROM hcd_approvals WHERE project = ? AND unit_id = ? ORDER BY created_at, id`, s.project, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Approval
	for rows.Next() {
		a := domain.Approval{UnitID: unitID}
		var created string
		if err := rows.Scan(&a.ID, &a.Action, &a.Revision, &a.Fingerprint, &a.Origin, &a.Client, &a.Quote, &a.ConstitutionRev, &a.PlaybookRev, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(hcdTime, created)
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanHCDUnit(row *sql.Row) (domain.Unit, error) {
	var u domain.Unit
	var kind, lane, state, created, updated string
	var active int
	if err := row.Scan(&u.ID, &u.Number, &u.Title, &u.Slug, &kind, &lane, &u.Why, &state, &active, &u.DocPath, &created, &updated); err != nil {
		return domain.Unit{}, err
	}
	u.Kind, u.Lane, u.State, u.Active = domain.Kind(kind), domain.Lane(lane), domain.UnitState(state), active == 1
	u.CreatedAt, _ = time.Parse(hcdTime, created)
	u.UpdatedAt, _ = time.Parse(hcdTime, updated)
	return u, nil
}
