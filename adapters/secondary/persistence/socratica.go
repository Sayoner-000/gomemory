package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"reflect"
	"strings"
	"time"
)

// ErrSocraticaConcurrentUpdate: the revision CAS lost to another writer. It is a
// revision conflict for the contract, so callers map it like a stale
// expected_revision.
var ErrSocraticaConcurrentUpdate = fmt.Errorf("%w: concurrent socratica update", domain.ErrRevisionConflict)
var ErrSocraticaAckRejected = ports.ErrAckRejected

type socraticaQuerier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type SocraticaStore struct {
	db            *sql.DB
	q             socraticaQuerier
	timeout       time.Duration
	inTransaction bool
}

// SQLite's busy handler may outlive context cancellation. Bound each wait on
// this exclusively checked-out connection, then retry only while ctx is alive.
// Restore its original setting before returning it to the shared pool.
func (s *SocraticaStore) writeContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.inTransaction {
		return s.q.ExecContext(ctx, query, args...)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	var previous int
	if err = conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&previous); err != nil {
		return nil, err
	}
	wait := max(1, min(10, int(s.timeout.Milliseconds()/10)))
	if _, err = conn.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", wait)); err != nil {
		return nil, err
	}
	defer func() {
		restore, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		_, _ = conn.ExecContext(restore, fmt.Sprintf("PRAGMA busy_timeout=%d", previous))
	}()
	for {
		result, e := conn.ExecContext(ctx, query, args...)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if e == nil || (!strings.Contains(e.Error(), "SQLITE_BUSY") && !strings.Contains(e.Error(), "SQLITE_LOCKED")) {
			return result, e
		}
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func NewSocraticaStore(db *sql.DB, timeout time.Duration) *SocraticaStore {
	if timeout <= 0 {
		timeout = domain.DefaultQueryTimeout
	}
	return &SocraticaStore{db: db, q: db, timeout: timeout}
}

var _ ports.SocraticaRepository = (*SocraticaStore)(nil)
var _ ports.SocraticaUnitOfWork = (*SocraticaStore)(nil)

func (s *SocraticaStore) WithTransaction(ctx context.Context, fn func(ports.SocraticaRepository) error) error {
	if s.inTransaction {
		return fn(s)
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var previous int
	if err = conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&previous); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA busy_timeout=10"); err != nil {
		return err
	}
	defer func() {
		restore, c := context.WithTimeout(context.Background(), s.timeout)
		defer c()
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
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	scoped := *s
	scoped.q = tx
	scoped.inTransaction = true
	if err = fn(&scoped); err != nil {
		return err
	}
	return tx.Commit()
}
func socTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func socNullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func protectSocraticaJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var tree any
	if err = json.Unmarshal(raw, &tree); err != nil {
		return "", err
	}
	var protect func(any) any
	protect = func(v any) any {
		switch x := v.(type) {
		case string:
			return domain.ProtectText(x)
		case []any:
			for i := range x {
				x[i] = protect(x[i])
			}
		case map[string]any:
			for k, v := range x {
				x[k] = protect(v)
			}
		}
		return v
	}
	raw, err = json.Marshal(protect(tree))
	return string(raw), err
}

const socraticaColumns = `id,schema_version,project,client,mode,original_prompt,questions_json,answers_json,proposed_prompt,approved_prompt,scrutiny_json,phase,last_stable_phase,revision,dispatch_id,dispatch_ack,dispatch_consumed,dispatch_claimed_at,dispatch_claim_owner,ack_nonce,approved_hash,last_error,created_at,updated_at,preparation_json,payload_policy`

func socraticaValues(f domain.RefinementFlow) ([]any, error) {
	q, e := protectSocraticaJSON(f.Questions)
	if e != nil {
		return nil, e
	}
	a, e := protectSocraticaJSON(f.Answers)
	if e != nil {
		return nil, e
	}
	sc, e := protectSocraticaJSON(f.Scrutiny)
	if e != nil {
		return nil, e
	}
	prep, e := socraticaPreparationValue(f.Preparation)
	if e != nil {
		return nil, e
	}
	var claimed any
	if f.DispatchClaimedAt != nil {
		claimed = socTime(*f.DispatchClaimedAt)
	}
	return []any{f.ID, f.SchemaVersion, f.Project, f.Client, f.Mode, domain.ProtectText(f.OriginalPrompt), q, a, domain.ProtectText(f.ProposedPrompt), domain.ProtectText(f.ApprovedPrompt), sc, f.Phase, f.LastStablePhase, f.Revision, socNullable(f.DispatchID), f.DispatchAck, f.DispatchConsumed, claimed, socNullable(f.DispatchClaimOwner), socNullable(f.AckNonce), f.ApprovedHash, domain.ProtectText(f.LastError), socTime(f.CreatedAt), socTime(f.UpdatedAt), prep, f.EffectivePayloadPolicy()}, nil
}

// socraticaPreparationValue keeps a missing preparation as SQL NULL, so legacy
// and refine/scrutiny rows are never read back as prepared.
func socraticaPreparationValue(p *domain.SpecifyPreparation) (any, error) {
	if p == nil {
		return nil, nil
	}
	return protectSocraticaJSON(p)
}

// scanSocraticaPreparation decodes and protects a stored preparation. A newer
// unsupported version is returned with its version so writers can refuse it.
func scanSocraticaPreparation(raw sql.NullString) (*domain.SpecifyPreparation, error) {
	if !raw.Valid {
		return nil, nil
	}
	var p domain.SpecifyPreparation
	if err := json.Unmarshal([]byte(raw.String), &p); err != nil {
		return nil, err
	}
	safe, err := protectSocraticaJSON(p)
	if err != nil {
		return nil, err
	}
	var out domain.SpecifyPreparation
	if err = json.Unmarshal([]byte(safe), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type socraticaScanner interface{ Scan(...any) error }

func scanSocratica(row socraticaScanner) (domain.RefinementFlow, error) {
	var f domain.RefinementFlow
	var qs, ans, sc, created, updated string
	var dispatch, owner, nonce, claimed, hash, last, prep sql.NullString
	err := row.Scan(&f.ID, &f.SchemaVersion, &f.Project, &f.Client, &f.Mode, &f.OriginalPrompt, &qs, &ans, &f.ProposedPrompt, &f.ApprovedPrompt, &sc, &f.Phase, &f.LastStablePhase, &f.Revision, &dispatch, &f.DispatchAck, &f.DispatchConsumed, &claimed, &owner, &nonce, &hash, &last, &created, &updated, &prep, &f.PayloadPolicy)
	if err != nil {
		return f, err
	}
	if f.Preparation, err = scanSocraticaPreparation(prep); err != nil {
		return f, err
	}
	f.DispatchID = dispatch.String
	f.DispatchClaimOwner = owner.String
	f.AckNonce = nonce.String
	f.ApprovedHash = hash.String
	f.LastError = domain.ProtectText(last.String)
	if f.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return f, err
	}
	if f.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return f, err
	}
	if claimed.Valid {
		t, e := time.Parse(time.RFC3339Nano, claimed.String)
		if e != nil {
			return f, e
		}
		f.DispatchClaimedAt = &t
	}
	// Decode first, then protect string leaves so redaction cannot break JSON syntax.
	for _, pair := range []struct {
		raw string
		v   any
	}{{qs, &f.Questions}, {ans, &f.Answers}, {sc, &f.Scrutiny}} {
		if err = json.Unmarshal([]byte(pair.raw), pair.v); err != nil {
			return f, err
		}
		safe, e := protectSocraticaJSON(pair.v)
		if e != nil {
			return f, e
		}
		if err = json.Unmarshal([]byte(safe), pair.v); err != nil {
			return f, err
		}
	}
	f.OriginalPrompt = domain.ProtectText(f.OriginalPrompt)
	f.ProposedPrompt = domain.ProtectText(f.ProposedPrompt)
	f.ApprovedPrompt = domain.ProtectText(f.ApprovedPrompt)
	return f, nil
}
func (s *SocraticaStore) CreateFlow(ctx context.Context, f domain.RefinementFlow) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	args, e := socraticaValues(f)
	if e != nil {
		return e
	}
	_, e = s.writeContext(ctx, "INSERT INTO socratica_flows ("+socraticaColumns+") VALUES ("+strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")+")", args...)
	return e
}
func (s *SocraticaStore) GetFlow(ctx context.Context, id string) (domain.RefinementFlow, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	f, e := scanSocratica(s.q.QueryRowContext(ctx, "SELECT "+socraticaColumns+" FROM socratica_flows WHERE id=?", id))
	if errors.Is(e, sql.ErrNoRows) {
		// The caller learns the flow is absent, not how storage reports it.
		return f, fmt.Errorf("%w: %w", domain.ErrNotFound, e)
	}
	return f, e
}
func (s *SocraticaStore) ActiveFlow(ctx context.Context, project, client string) (domain.RefinementFlow, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	f, e := scanSocratica(s.q.QueryRowContext(ctx, "SELECT "+socraticaColumns+" FROM socratica_flows WHERE project=? AND client=? AND phase NOT IN ('dispatched','cancelled')", project, client))
	if errors.Is(e, sql.ErrNoRows) {
		return f, false, nil
	}
	return f, e == nil, e
}
func (s *SocraticaStore) SaveTransition(ctx context.Context, before, after domain.RefinementFlow) error {
	// A newer preparation is kept and never rewritten; cancelling it, with the
	// aggregate untouched, is the only mutation allowed so the slot can be freed.
	newer := before.Preparation != nil && before.Preparation.SchemaVersion > domain.PreparationSchemaVersion
	if newer && !(after.Phase == domain.PhaseCancelled && reflect.DeepEqual(before.Preparation, after.Preparation)) {
		return domain.ErrUnsupportedPreparation
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	values, e := socraticaValues(after)
	if e != nil {
		return e
	}
	// payload_policy is fixed at creation: a transition never rewrites it. A
	// newer preparation is left as stored: re-encoding it would drop the fields
	// this binary does not know.
	cols := strings.Split(socraticaColumns, ",")
	sets := make([]string, 0, len(cols)-1)
	args := make([]any, 0, len(values)+3)
	for i, col := range cols[1:] {
		if col == "payload_policy" || (newer && col == "preparation_json") {
			continue
		}
		sets = append(sets, col+"=?")
		args = append(args, values[i+1])
	}
	args = append(args, before.ID, before.Phase, before.Revision, socTime(before.UpdatedAt))
	res, e := s.writeContext(ctx, "UPDATE socratica_flows SET "+strings.Join(sets, ",")+" WHERE id=? AND phase=? AND revision=? AND updated_at=?", args...)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e == nil && n != 1 {
		return ErrSocraticaConcurrentUpdate
	}
	return e
}
func (s *SocraticaStore) Approve(ctx context.Context, id, dispatch, prompt string) (domain.RefinementFlow, bool, error) {
	prompt = domain.ProtectText(prompt)
	return s.approveRow(ctx, id, dispatch, prompt, domain.SocraticaPromptHash(prompt))
}

// approveRow stores the approval; the ephemeral adapter passes an empty prompt
// and a NULL hash so no private text, nor a digest of it, reaches the disk.
func (s *SocraticaStore) approveRow(ctx context.Context, id, dispatch, prompt string, hash any) (domain.RefinementFlow, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	res, e := s.writeContext(ctx, `UPDATE socratica_flows SET phase='approved',last_stable_phase='approved',dispatch_id=COALESCE(dispatch_id,?),approved_prompt=?,approved_hash=? WHERE id=? AND phase IN ('review','failed')`, dispatch, prompt, hash, id)
	if e != nil {
		return domain.RefinementFlow{}, false, e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return domain.RefinementFlow{}, false, e
	}
	f, e := s.GetFlow(ctx, id)
	if e == nil && n == 0 && f.Phase != domain.PhaseApproved && f.Phase != domain.PhaseDispatched {
		return f, false, domain.ErrInvalidTransition
	}
	return f, n == 0, e
}

// Ack confirms a dispatch. Only the UPDATE that performs the first transition
// (RowsAffected==1) grants execution; a same-nonce replay is acked without it.
func (s *SocraticaStore) Ack(ctx context.Context, id, dispatch, owner, nonce string) (ports.AckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	res, e := s.writeContext(ctx, `UPDATE socratica_flows SET phase='dispatched',dispatch_ack=1,ack_nonce=? WHERE id=? AND dispatch_id=? AND dispatch_claim_owner IS ? AND phase='approved' AND dispatch_ack=0`, socNullable(nonce), id, dispatch, socNullable(owner))
	if e != nil {
		return ports.AckResult{}, e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return ports.AckResult{}, e
	}
	if n == 1 {
		return ports.AckResult{Acked: true, Granted: true}, nil
	}
	f, e := s.GetFlow(ctx, id)
	if e != nil {
		return ports.AckResult{}, e
	}
	if f.DispatchID != dispatch || f.DispatchClaimOwner != owner {
		return ports.AckResult{}, ErrSocraticaAckRejected
	}
	if f.DispatchAck {
		if nonce != "" && f.AckNonce == nonce {
			return ports.AckResult{Acked: true}, nil
		}
		return ports.AckResult{AlreadyDispatched: true}, nil
	}
	return ports.AckResult{}, ErrSocraticaAckRejected
}
func (s *SocraticaStore) ClaimDispatch(ctx context.Context, id, dispatch, owner string, now time.Time) (bool, error) {
	if owner == "" {
		return false, fmt.Errorf("dispatch owner is required")
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	r, e := s.writeContext(ctx, `UPDATE socratica_flows SET dispatch_claim_owner=?,dispatch_claimed_at=? WHERE id=? AND dispatch_id=? AND phase='approved' AND dispatch_ack=0 AND dispatch_claim_owner IS NULL`, owner, socTime(now), id, dispatch)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	return n == 1, e
}
func (s *SocraticaStore) ReleaseDispatch(ctx context.Context, id, owner string) error {
	return s.execOne(ctx, `UPDATE socratica_flows SET dispatch_claim_owner=NULL,dispatch_claimed_at=NULL WHERE id=? AND dispatch_claim_owner=?`, id, owner)
}
func (s *SocraticaStore) MarkDispatched(ctx context.Context, id, dispatch string) error {
	return s.execOne(ctx, `UPDATE socratica_flows SET phase='dispatched',dispatch_ack=1 WHERE id=? AND dispatch_id=? AND phase='approved'`, id, dispatch)
}
func (s *SocraticaStore) ForceResend(ctx context.Context, id, oldID, newID string) error {
	return s.forceResendRow(ctx, id, oldID, newID, " AND approved_prompt<>''")
}

// forceResendRow lets the ephemeral adapter check the approved text in its
// payload instead of the column, which it keeps empty.
func (s *SocraticaStore) forceResendRow(ctx context.Context, id, oldID, newID, guard string) error {
	return s.execOne(ctx, `UPDATE socratica_flows SET phase='approved',dispatch_id=?,dispatch_ack=0,ack_nonce=NULL,dispatch_claim_owner=NULL,dispatch_claimed_at=NULL,dispatch_consumed=0 WHERE id=? AND dispatch_id=?`+guard, newID, id, oldID)
}
func (s *SocraticaStore) DeleteFlow(ctx context.Context, id string) error {
	return s.execOne(ctx, `DELETE FROM socratica_flows WHERE id=?`, id)
}
func (s *SocraticaStore) FlowExists(ctx context.Context, id string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	var one int
	err := s.q.QueryRowContext(ctx, `SELECT 1 FROM socratica_flows WHERE id=?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func (s *SocraticaStore) FindByDispatchID(ctx context.Context, id string) (domain.RefinementFlow, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	f, e := scanSocratica(s.q.QueryRowContext(ctx, "SELECT "+socraticaColumns+" FROM socratica_flows WHERE dispatch_id=?", id))
	if errors.Is(e, sql.ErrNoRows) {
		return f, false, nil
	}
	return f, e == nil, e
}
func (s *SocraticaStore) ConsumeDispatch(ctx context.Context, id, text string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	r, e := s.writeContext(ctx, `UPDATE socratica_flows SET dispatch_consumed=1 WHERE dispatch_id=? AND approved_hash=? AND dispatch_consumed=0 AND phase IN ('approved','dispatched')`, id, domain.SocraticaPromptHash(text))
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	return n == 1, e
}
func (s *SocraticaStore) RedactFlow(ctx context.Context, id string) error {
	return s.execOne(ctx, `UPDATE socratica_flows SET original_prompt='',proposed_prompt='',approved_prompt='',questions_json='[]',answers_json='[]',scrutiny_json='null',last_error='',preparation_json=NULL WHERE id=?`, id)
}

// execOne runs a mutation addressed to one flow and fails with
// ErrSocraticaNoMatch when it touched no row (H-02, auditoría 2026-10-16).
func (s *SocraticaStore) execOne(ctx context.Context, query string, args ...any) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	res, err := s.writeContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ports.ErrSocraticaNoMatch
	}
	return nil
}
func (s *SocraticaStore) exec(ctx context.Context, query string, args ...any) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	_, e := s.writeContext(ctx, query, args...)
	return e
}
func (s *SocraticaStore) UpsertHistory(ctx context.Context, h domain.RefinementHistory, limit int) error {
	if limit < 1 {
		return fmt.Errorf("invalid socratica history limit")
	}
	return s.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		tx := repo.(*SocraticaStore)
		sc, e := protectSocraticaJSON(h.Scrutiny)
		if e != nil {
			return e
		}
		add, e := protectSocraticaJSON(h.Additions)
		if e != nil {
			return e
		}
		prep, e := socraticaPreparationValue(h.Preparation)
		if e != nil {
			return e
		}
		e = tx.exec(ctx, `INSERT INTO socratica_history(id,flow_id,project,client,mode,original_prompt,proposed_prompt,approved_prompt,scrutiny_json,additions_json,words_before,words_after,completed_at,preparation_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(flow_id) DO UPDATE SET proposed_prompt=excluded.proposed_prompt,approved_prompt=excluded.approved_prompt,scrutiny_json=excluded.scrutiny_json,additions_json=excluded.additions_json,words_before=excluded.words_before,words_after=excluded.words_after,completed_at=excluded.completed_at,preparation_json=excluded.preparation_json`, h.ID, h.FlowID, h.Project, h.Client, h.Mode, domain.ProtectText(h.OriginalPrompt), domain.ProtectText(h.ProposedPrompt), domain.ProtectText(h.ApprovedPrompt), sc, add, h.WordsBefore, h.WordsAfter, socTime(h.CompletedAt), prep)
		if e != nil {
			return e
		}
		return tx.exec(ctx, `DELETE FROM socratica_history WHERE project=? AND client=? AND id NOT IN (SELECT id FROM socratica_history WHERE project=? AND client=? ORDER BY completed_at DESC,id DESC LIMIT ?)`, h.Project, h.Client, h.Project, h.Client, limit)
	})
}
func (s *SocraticaStore) ListHistory(ctx context.Context, project, client string, limit int) ([]domain.RefinementHistory, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if limit < 1 {
		limit = domain.DefaultHistoryLimit
	}
	rows, e := s.q.QueryContext(ctx, `SELECT id,flow_id,project,client,mode,original_prompt,proposed_prompt,approved_prompt,scrutiny_json,additions_json,words_before,words_after,completed_at,preparation_json FROM socratica_history WHERE project=? AND (?='' OR client=?) ORDER BY completed_at DESC,id DESC LIMIT ?`, project, client, client, limit)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	result := []domain.RefinementHistory{}
	for rows.Next() {
		var h domain.RefinementHistory
		var approved, sc, prep sql.NullString
		var add, at string
		e = rows.Scan(&h.ID, &h.FlowID, &h.Project, &h.Client, &h.Mode, &h.OriginalPrompt, &h.ProposedPrompt, &approved, &sc, &add, &h.WordsBefore, &h.WordsAfter, &at, &prep)
		if e != nil {
			return nil, e
		}
		if h.Preparation, e = scanSocraticaPreparation(prep); e != nil {
			return nil, e
		}
		h.OriginalPrompt = domain.ProtectText(h.OriginalPrompt)
		h.ProposedPrompt = domain.ProtectText(h.ProposedPrompt)
		h.ApprovedPrompt = domain.ProtectText(approved.String)
		if h.CompletedAt, e = time.Parse(time.RFC3339Nano, at); e != nil {
			return nil, e
		}
		if sc.Valid {
			if e = json.Unmarshal([]byte(sc.String), &h.Scrutiny); e != nil {
				return nil, e
			}
		}
		if e = json.Unmarshal([]byte(add), &h.Additions); e != nil {
			return nil, e
		}
		safe, e := protectSocraticaJSON(h)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(safe), &h); e != nil {
			return nil, e
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

// deleteReturningIDs deletes the rows selected by where and returns their ids,
// atomically, so lock files can be removed for exactly the purged flows.
func (s *SocraticaStore) deleteReturningIDs(ctx context.Context, where string, args ...any) ([]string, error) {
	var ids []string
	err := s.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		tx := repo.(*SocraticaStore)
		qctx, cancel := context.WithTimeout(ctx, s.timeout)
		defer cancel()
		rows, e := tx.q.QueryContext(qctx, `SELECT id FROM socratica_flows WHERE `+where, args...)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return e
			}
			ids = append(ids, id)
		}
		if e = rows.Err(); e != nil {
			return e
		}
		return tx.exec(ctx, `DELETE FROM socratica_flows WHERE `+where, args...)
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}
func (s *SocraticaStore) purge(ctx context.Context, project string, before time.Time, terminal bool) ([]string, error) {
	phases := `'captured','questions','rewriting','review','failed'`
	if terminal {
		phases = `'dispatched','cancelled'`
	}
	return s.deleteReturningIDs(ctx, `project=? AND updated_at<? AND phase IN (`+phases+`)`, project, socTime(before))
}
func (s *SocraticaStore) PurgeStale(ctx context.Context, p string, b time.Time) ([]string, error) {
	return s.purge(ctx, p, b, false)
}
func (s *SocraticaStore) PurgeTerminal(ctx context.Context, p string, b time.Time) ([]string, error) {
	return s.purge(ctx, p, b, true)
}

func (s *SocraticaStore) PruneTerminal(ctx context.Context, project, client string, limit int) ([]string, error) {
	if limit < 1 {
		return nil, fmt.Errorf("invalid socratica retention limit")
	}
	return s.deleteReturningIDs(ctx, `project=? AND client=? AND phase IN ('dispatched','cancelled') AND id NOT IN (SELECT id FROM socratica_flows WHERE project=? AND client=? AND phase IN ('dispatched','cancelled') ORDER BY updated_at DESC,id DESC LIMIT ?)`, project, client, project, client, limit)
}
