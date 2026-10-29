package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"strings"
	"time"
)

type hcdQuerier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type HCDStore struct {
	db            *sql.DB
	q             hcdQuerier
	project       string
	timeout       time.Duration
	inTransaction bool
}

func NewHCDStore(db *sql.DB, project string) *HCDStore {
	return &HCDStore{db: db, q: db, project: project, timeout: domain.DefaultQueryTimeout}
}

var _ ports.HCDRepository = (*HCDStore)(nil)
var _ ports.HCDUnitOfWork = (*HCDStore)(nil)
var _ ports.HCDContextRepository = (*HCDStore)(nil)

func (s *HCDStore) WithTransaction(ctx context.Context, fn func(ports.HCDRepository) error) error {
	if s.inTransaction {
		return fn(s)
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var previous int
	if err = conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&previous); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA busy_timeout=10"); err != nil {
		return err
	}
	defer func() {
		restore, done := context.WithTimeout(context.Background(), s.timeout)
		defer done()
		_, _ = conn.ExecContext(restore, fmt.Sprintf("PRAGMA busy_timeout=%d", previous))
	}()
	var tx *sql.Tx
	for {
		tx, err = conn.BeginTx(ctx, nil)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "SQLITE_BUSY") && !strings.Contains(err.Error(), "SQLITE_LOCKED") {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
	repo := *s
	repo.q, repo.inTransaction = tx, true
	if err := fn(&repo); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *HCDStore) writeContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.inTransaction {
		return s.q.ExecContext(ctx, query, args...)
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var previous int
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&previous); err != nil {
		return nil, err
	}
	wait := max(1, min(10, int(s.timeout.Milliseconds()/10)))
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", wait)); err != nil {
		return nil, err
	}
	defer func() {
		restore, done := context.WithTimeout(context.Background(), s.timeout)
		defer done()
		_, _ = conn.ExecContext(restore, fmt.Sprintf("PRAGMA busy_timeout=%d", previous))
	}()
	for {
		result, execErr := conn.ExecContext(ctx, query, args...)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if execErr == nil || (!strings.Contains(execErr.Error(), "SQLITE_BUSY") && !strings.Contains(execErr.Error(), "SQLITE_LOCKED")) {
			return result, execErr
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func (s *HCDStore) CreateUnit(ctx context.Context, u domain.Unit) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.writeContext(ctx, `INSERT INTO hcd_units(project,id,kind,title,slug,approach,state,state_revision,definition_rev,delivery_rev,approved_rev,playbook_rev,constitution_rev,implementer,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.project, u.ID, u.Kind, domain.ProtectHCDField(u.Title), u.Slug, u.Approach, u.State, u.StateRevision, u.DefinitionRev, u.DeliveryRev, u.ApprovedRev, u.PlaybookRev, u.ConstitutionRev, domain.ProtectHCDField(u.Implementer), now, now)
	return err
}

func (s *HCDStore) GetUnit(ctx context.Context, id string) (domain.Unit, error) {
	var u domain.Unit
	var kind, state string
	err := s.q.QueryRowContext(ctx, `SELECT id,kind,title,slug,approach,state,state_revision,definition_rev,delivery_rev,approved_rev,playbook_rev,constitution_rev,implementer,prev_state FROM hcd_units WHERE project=? AND id=?`, s.project, id).
		Scan(&u.ID, &kind, &u.Title, &u.Slug, &u.Approach, &state, &u.StateRevision, &u.DefinitionRev, &u.DeliveryRev, &u.ApprovedRev, &u.PlaybookRev, &u.ConstitutionRev, &u.Implementer, &u.PrevState)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Unit{}, fmt.Errorf("HCD unit %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Unit{}, err
	}
	u.Kind, u.State = domain.Kind(kind), domain.State(state)
	return u, nil
}

func (s *HCDStore) UpdateUnit(ctx context.Context, u domain.Unit, expected int64) error {
	res, err := s.writeContext(ctx, `UPDATE hcd_units SET kind=?,title=?,slug=?,approach=?,state=?,state_revision=?,definition_rev=?,delivery_rev=?,approved_rev=?,playbook_rev=?,constitution_rev=?,implementer=?,prev_state=?,updated_at=? WHERE project=? AND id=? AND state_revision=?`, u.Kind, domain.ProtectHCDField(u.Title), u.Slug, u.Approach, u.State, u.StateRevision, u.DefinitionRev, u.DeliveryRev, u.ApprovedRev, u.PlaybookRev, u.ConstitutionRev, domain.ProtectHCDField(u.Implementer), u.PrevState, time.Now().UTC().Format(time.RFC3339Nano), s.project, u.ID, expected)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrRevisionConflict
	}
	return nil
}

func (s *HCDStore) NextUnitID(ctx context.Context) (string, error) {
	var n int
	if err := s.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(CAST(SUBSTR(id,5) AS INTEGER)),0)+1 FROM hcd_units WHERE project=?`, s.project).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("HCD-%03d", n), nil
}

func (s *HCDStore) RecordOperation(ctx context.Context, opID, unitID, command, payloadHash, resultJSON string) (string, bool, error) {
	var oldUnit, oldCommand, oldHash, oldResult string
	err := s.q.QueryRowContext(ctx, `SELECT unit_id,command,payload_hash,result_json FROM hcd_operations WHERE project=? AND operation_id=?`, s.project, opID).Scan(&oldUnit, &oldCommand, &oldHash, &oldResult)
	if err == nil {
		// ACR C-006: el mismo id en otra unidad u otro comando es otra operación.
		if oldHash != payloadHash || oldUnit != unitID || oldCommand != command {
			return "", false, domain.ErrOperationConflict
		}
		return oldResult, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if resultJSON == "" {
		return "", false, nil
	}
	_, err = s.writeContext(ctx, `INSERT INTO hcd_operations(project,operation_id,unit_id,command,payload_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?)`, s.project, opID, unitID, command, payloadHash, resultJSON, time.Now().UTC().Format(time.RFC3339Nano))
	return resultJSON, false, err
}

func (s *HCDStore) SaveNormativeRevision(ctx context.Context, doc, revision, content, source, changeKind string, urgent bool) error {
	flag := 0
	if urgent {
		flag = 1
	}
	_, err := s.writeContext(ctx, `INSERT OR IGNORE INTO hcd_normative_revisions(project,doc,revision,content,first_pinned_at,source,change_kind,urgent) VALUES(?,?,?,?,?,?,?,?)`, s.project, doc, revision, content, time.Now().UTC().Format(time.RFC3339Nano), source, changeKind, flag)
	return err
}

func (s *HCDStore) NextDecisionID(ctx context.Context, unitID string) (string, error) {
	var n int
	if err := s.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(CAST(SUBSTR(id,3) AS INTEGER)),0)+1 FROM hcd_decisions WHERE project=?`, s.project).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("D-%02d", n), nil
}

func (s *HCDStore) SaveDecision(ctx context.Context, d domain.HCDDecision) error {
	refs, err := json.Marshal(d.Refs)
	if err != nil {
		return err
	}
	_, err = s.writeContext(ctx, `INSERT INTO hcd_decisions(project,id,unit_id,kind,status,summary,revision,refs_json,source,memory_id,memory_sync) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, s.project, d.ID, d.UnitID, d.Kind, d.Status, domain.ProtectText(d.Summary), d.Revision, string(refs), d.Source, nullID(d.MemoryID), d.MemorySync)
	return err
}

func (s *HCDStore) ListDecisions(ctx context.Context, unitID string) ([]domain.HCDDecision, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT id,unit_id,kind,status,summary,revision,refs_json,source,memory_id,memory_sync FROM hcd_decisions WHERE project=? AND unit_id=? ORDER BY id`, s.project, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.HCDDecision
	for rows.Next() {
		var d domain.HCDDecision
		var refs string
		var memory sql.NullInt64
		if err := rows.Scan(&d.ID, &d.UnitID, &d.Kind, &d.Status, &d.Summary, &d.Revision, &refs, &d.Source, &memory, &d.MemorySync); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(refs), &d.Refs)
		if memory.Valid {
			d.MemoryID = memory.Int64
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func (s *HCDStore) NextClarificationID(ctx context.Context, unitID string) (string, error) {
	var n int
	if err := s.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(CAST(SUBSTR(id,4) AS INTEGER)),0)+1 FROM hcd_clarifications WHERE project=?`, s.project).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("CL-%02d", n), nil
}
func (s *HCDStore) SaveClarification(ctx context.Context, c domain.Clarification) error {
	opts, err := json.Marshal(c.Options)
	if err != nil {
		return err
	}
	_, err = s.writeContext(ctx, `INSERT INTO hcd_clarifications(project,id,unit_id,target_ref,question,options_json,recommendation,answer,status,blocking) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(project,id) DO UPDATE SET answer=excluded.answer,status=excluded.status,blocking=excluded.blocking`, s.project, c.ID, c.UnitID, c.TargetRef, domain.ProtectText(c.Question), string(opts), domain.ProtectText(c.Recommendation), domain.ProtectText(c.Answer), c.Status, boolInt(c.Blocking))
	return err
}
func (s *HCDStore) GetClarification(ctx context.Context, unitID, id string) (domain.Clarification, error) {
	var c domain.Clarification
	var raw string
	var blocking int
	err := s.q.QueryRowContext(ctx, `SELECT id,unit_id,target_ref,question,options_json,recommendation,answer,status,blocking FROM hcd_clarifications WHERE project=? AND unit_id=? AND id=?`, s.project, unitID, id).Scan(&c.ID, &c.UnitID, &c.TargetRef, &c.Question, &raw, &c.Recommendation, &c.Answer, &c.Status, &blocking)
	if errors.Is(err, sql.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	if err != nil {
		return c, err
	}
	_ = json.Unmarshal([]byte(raw), &c.Options)
	c.Blocking = blocking != 0
	return c, nil
}
func (s *HCDStore) ListOpenClarifications(ctx context.Context, unitID string) ([]domain.Clarification, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT id,unit_id,target_ref,question,options_json,recommendation,answer,status,blocking FROM hcd_clarifications WHERE project=? AND unit_id=? AND status='open' ORDER BY id`, s.project, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Clarification
	for rows.Next() {
		var c domain.Clarification
		var raw string
		var blocking int
		if err := rows.Scan(&c.ID, &c.UnitID, &c.TargetRef, &c.Question, &raw, &c.Recommendation, &c.Answer, &c.Status, &blocking); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &c.Options)
		c.Blocking = blocking != 0
		out = append(out, c)
	}
	return out, rows.Err()
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (s *HCDStore) SaveApprovalPrompt(ctx context.Context, p domain.ApprovalPrompt) error {
	opts, err := json.Marshal(p.Options)
	if err != nil {
		return err
	}
	_, err = s.writeContext(ctx, `INSERT INTO hcd_approval_prompts(project,id,unit_id,object,revision,options_json,asked_at,consumed) VALUES(?,?,?,?,?,?,?,?)`, s.project, p.ID, p.UnitID, p.Object, p.Revision, string(opts), p.AskedAt, boolInt(p.Consumed))
	return err
}

func (s *HCDStore) GetApprovalPrompt(ctx context.Context, id, unitID string) (domain.ApprovalPrompt, error) {
	var p domain.ApprovalPrompt
	var raw string
	var consumed int
	err := s.q.QueryRowContext(ctx, `SELECT id,unit_id,object,revision,options_json,asked_at,consumed FROM hcd_approval_prompts WHERE project=? AND id=? AND unit_id=?`, s.project, id, unitID).Scan(&p.ID, &p.UnitID, &p.Object, &p.Revision, &raw, &p.AskedAt, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	_ = json.Unmarshal([]byte(raw), &p.Options)
	p.Consumed = consumed != 0
	return p, nil
}

func (s *HCDStore) ConsumeApprovalPrompt(ctx context.Context, id, unitID string) error {
	r, err := s.writeContext(ctx, `UPDATE hcd_approval_prompts SET consumed=1 WHERE project=? AND id=? AND unit_id=? AND consumed=0`, s.project, id, unitID)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotAllowed
	}
	return nil
}

func (s *HCDStore) NextApprovalID(ctx context.Context, unitID string) (string, error) {
	var n int
	err := s.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(CAST(SUBSTR(id,5) AS INTEGER)),0)+1 FROM hcd_approvals WHERE project=?`, s.project).Scan(&n)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("APR-%02d", n), nil
}

func (s *HCDStore) SaveApproval(ctx context.Context, a domain.Approval) error {
	_, err := s.writeContext(ctx, `INSERT INTO hcd_approvals(project,id,unit_id,object,revision,origin,quote,client,approved_at) VALUES(?,?,?,?,?,?,?,?,?)`, s.project, a.ID, a.UnitID, a.Object, a.Revision, a.Origin, domain.ProtectText(a.Quote), a.Client, a.ApprovedAt)
	return err
}

func (s *HCDStore) AddLink(ctx context.Context, l domain.Link) error {
	_, err := s.writeContext(ctx, `INSERT OR IGNORE INTO hcd_links(project,unit_id,from_kind,from_id,to_kind,to_id,relation,created_at) VALUES(?,?,?,?,?,?,?,?)`, s.project, l.UnitID, l.FromKind, l.FromID, l.ToKind, l.ToID, l.Relation, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *HCDStore) ListLinks(ctx context.Context, unitID string) ([]domain.Link, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT unit_id,from_kind,from_id,to_kind,to_id,relation FROM hcd_links WHERE project=? AND unit_id=? ORDER BY from_kind,from_id,to_kind,to_id,relation`, s.project, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Link
	for rows.Next() {
		var l domain.Link
		if err := rows.Scan(&l.UnitID, &l.FromKind, &l.FromID, &l.ToKind, &l.ToID, &l.Relation); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *HCDStore) SetFocus(ctx context.Context, f domain.Focus) error {
	_, err := s.writeContext(ctx, `INSERT INTO hcd_focus(project,unit_id,client,session_id,set_at,set_by) VALUES(?,?,?,?,?,?) ON CONFLICT(project) DO UPDATE SET unit_id=excluded.unit_id,client=excluded.client,session_id=excluded.session_id,set_at=excluded.set_at,set_by=excluded.set_by`, s.project, f.UnitID, f.Client, f.SessionID, f.SetAt, f.SetBy)
	return err
}
func (s *HCDStore) GetFocus(ctx context.Context) (domain.Focus, error) {
	var f domain.Focus
	err := s.q.QueryRowContext(ctx, `SELECT unit_id,client,session_id,set_at,set_by FROM hcd_focus WHERE project=?`, s.project).Scan(&f.UnitID, &f.Client, &f.SessionID, &f.SetAt, &f.SetBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Focus{}, domain.ErrNotFound
	}
	return f, err
}
func (s *HCDStore) ClearFocus(ctx context.Context) error {
	_, err := s.writeContext(ctx, `DELETE FROM hcd_focus WHERE project=?`, s.project)
	return err
}
func (s *HCDStore) RecordHeartbeat(ctx context.Context, h domain.Heartbeat) error {
	files, _ := json.Marshal(h.Files)
	drift, _ := json.Marshal(h.Drift)
	_, err := s.writeContext(ctx, `INSERT INTO hcd_heartbeats(project,id,unit_id,session_id,client,stage,state_revision,files_json,drift_json,seen_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, s.project, h.ID, h.UnitID, h.SessionID, h.Client, h.Stage, h.StateRevision, string(files), string(drift), h.SeenAt)
	if err != nil {
		return err
	}
	_, err = s.writeContext(ctx, `DELETE FROM hcd_heartbeats WHERE project=? AND unit_id=? AND session_id=? AND drift_json IN ('[]','null') AND rowid NOT IN (SELECT rowid FROM hcd_heartbeats WHERE project=? AND unit_id=? AND session_id=? ORDER BY seen_at DESC,rowid DESC LIMIT 1)`, s.project, h.UnitID, h.SessionID, s.project, h.UnitID, h.SessionID)
	return err
}

func (s *HCDStore) ActiveHCDUnit(ctx context.Context) (domain.Unit, error) {
	var id string
	if err := s.q.QueryRowContext(ctx, `SELECT unit_id FROM hcd_focus WHERE project=?`, s.project).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Unit{}, domain.ErrNotFound
		}
		return domain.Unit{}, err
	}
	return s.GetUnit(ctx, id)
}

func (s *HCDStore) ListHCDUnits(ctx context.Context) ([]domain.Unit, error) {
	rows, e := s.q.QueryContext(ctx, `SELECT id FROM hcd_units WHERE project=? ORDER BY id`, s.project)
	if e != nil {
		return nil, e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	var out []domain.Unit
	for _, id := range ids {
		u, e := s.GetUnit(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, u)
	}
	return out, nil
}

func (s *HCDStore) ListApprovals(ctx context.Context, unit string) ([]domain.Approval, error) {
	rows, e := s.q.QueryContext(ctx, `SELECT id,unit_id,object,revision,origin,quote,client,approved_at FROM hcd_approvals WHERE project=? AND unit_id=? ORDER BY approved_at,id`, s.project, unit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Approval
	for rows.Next() {
		var a domain.Approval
		if e = rows.Scan(&a.ID, &a.UnitID, &a.Object, &a.Revision, &a.Origin, &a.Quote, &a.Client, &a.ApprovedAt); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *HCDStore) LastVerificationRevision(ctx context.Context, unit string) (string, error) {
	var raw string
	e := s.q.QueryRowContext(ctx, `SELECT result_json FROM hcd_operations WHERE project=? AND unit_id=? AND command='verify' ORDER BY created_at DESC,operation_id DESC LIMIT 1`, s.project, unit).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	var v struct{ ReportRevision string }
	if e = json.Unmarshal([]byte(raw), &v); e != nil {
		return "", e
	}
	return v.ReportRevision, nil
}

func (s *HCDStore) MarkDecisionSynced(ctx context.Context, unit, id string, memoryID int64) error {
	_, e := s.writeContext(ctx, `UPDATE hcd_decisions SET memory_id=?,memory_sync='synced' WHERE project=? AND unit_id=? AND id=?`, memoryID, s.project, unit, id)
	return e
}

func (s *HCDStore) ListHeartbeats(ctx context.Context, unit string) ([]domain.Heartbeat, error) {
	rows, e := s.q.QueryContext(ctx, `SELECT id,unit_id,session_id,client,stage,state_revision,files_json,drift_json,seen_at FROM hcd_heartbeats WHERE project=? AND unit_id=? ORDER BY seen_at DESC,rowid DESC`, s.project, unit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Heartbeat
	for rows.Next() {
		var h domain.Heartbeat
		var files, drift string
		if e = rows.Scan(&h.ID, &h.UnitID, &h.SessionID, &h.Client, &h.Stage, &h.StateRevision, &files, &drift, &h.SeenAt); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(files), &h.Files); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(drift), &h.Drift); e != nil {
			return nil, e
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *HCDStore) HCDActivityAt(ctx context.Context, unit string) (string, error) {
	var at string
	e := s.q.QueryRowContext(ctx, `SELECT COALESCE((SELECT MAX(seen_at) FROM hcd_heartbeats WHERE project=? AND unit_id=?),created_at) FROM hcd_units WHERE project=? AND id=?`, s.project, unit, s.project, unit).Scan(&at)
	return at, e
}

func (s *HCDStore) UpdateDecision(ctx context.Context, d domain.HCDDecision) error {
	refs, e := json.Marshal(d.Refs)
	if e != nil {
		return e
	}
	_, e = s.writeContext(ctx, `UPDATE hcd_decisions SET status=?,summary=?,revision=?,refs_json=?,source=?,memory_sync=? WHERE project=? AND unit_id=? AND id=?`, d.Status, domain.ProtectText(d.Summary), d.Revision, string(refs), d.Source, d.MemorySync, s.project, d.UnitID, d.ID)
	return e
}
