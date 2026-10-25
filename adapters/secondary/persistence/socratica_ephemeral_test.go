package persistence

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/application/ports"
	"mem/domain"
)

const ephemeralSecret = "SECRETO-PRIVADO-042"

// ephemeralTestDB opens one migrated database that several instances share,
// as two MCP processes of the same project would.
func ephemeralTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func privateSpecifyFlow(id string) domain.RefinementFlow {
	f := specifyTestFlow(id)
	f.PayloadPolicy = domain.PayloadEphemeral
	f.OriginalPrompt = "original " + ephemeralSecret
	f.Questions = []domain.Question{{Text: "¿" + ephemeralSecret + "?", Options: []string{"a", "b"}}}
	f.Preparation.Content.Intent.Objective = "objetivo " + ephemeralSecret
	f.LastError = "error " + ephemeralSecret
	return f
}

// requireNoSecretOnDisk scans every text column of the Socratica tables.
func requireNoSecretOnDisk(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"socratica_flows", "socratica_history"} {
		rows, err := db.Query("SELECT * FROM " + table)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			for i, v := range values {
				var text string
				switch x := v.(type) {
				case string:
					text = x
				case []byte:
					text = string(x)
				}
				if strings.Contains(text, ephemeralSecret) {
					t.Errorf("%s.%s stores private content: %q", table, cols[i], text)
				}
			}
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEphemeralStore_PrivatePayloadStaysInMemory(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	s := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	f := privateSpecifyFlow("priv")
	if err := s.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PrivateStateUnavailable || got.OriginalPrompt != f.OriginalPrompt || got.Preparation.Content.Intent.Objective != f.Preparation.Content.Intent.Objective || len(got.Questions) != 1 {
		t.Fatalf("same instance must read its payload: %+v", got)
	}
	var policy string
	var prep sql.NullString
	if err = db.QueryRow("SELECT payload_policy, preparation_json FROM socratica_flows WHERE id=?", f.ID).Scan(&policy, &prep); err != nil {
		t.Fatal(err)
	}
	if policy != "ephemeral" || prep.Valid {
		t.Errorf("reservation = policy %q preparation %v; want ephemeral and NULL", policy, prep)
	}
	// History of a private specify flow is never written, whatever the caller.
	if err = s.UpsertHistory(ctx, domain.RefinementHistory{ID: f.ID, FlowID: f.ID, Project: "p", Client: "codex", Mode: domain.ModeSpecify, OriginalPrompt: f.OriginalPrompt, CompletedAt: time.Unix(1, 0)}, 5); err != nil {
		t.Fatal(err)
	}
	next := got
	next.Preparation = got.Preparation.Clone()
	next.Preparation.Content.Intent.ExpectedResult = "resultado " + ephemeralSecret
	next.Phase, next.ProposedPrompt = domain.PhaseReview, "propuesta "+ephemeralSecret
	next.Revision++
	if err = s.SaveTransition(ctx, got, next); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Approve(ctx, f.ID, "d1", next.ProposedPrompt); err != nil {
		t.Fatal(err)
	}
	approved, err := s.GetFlow(ctx, f.ID)
	if err != nil || approved.ApprovedPrompt != next.ProposedPrompt || approved.Phase != domain.PhaseApproved {
		t.Fatalf("approved = %+v, %v", approved, err)
	}
	requireNoSecretOnDisk(t, db)
	var hash sql.NullString
	if err = db.QueryRow("SELECT approved_hash FROM socratica_flows WHERE id=?", f.ID).Scan(&hash); err != nil || hash.String != "" {
		t.Errorf("approved_hash = %q, %v; a hash of private text is not stored", hash.String, err)
	}
	if res, err := s.Ack(ctx, f.ID, "d1", "", "n1"); err != nil || !res.Granted {
		t.Fatalf("ack = %+v, %v", res, err)
	}
}

func TestEphemeralStore_RollbackPublishesNothing(t *testing.T) {
	ctx := context.Background()
	s := NewEphemeralSocraticaStore(NewSocraticaStore(ephemeralTestDB(t), time.Second))
	f := privateSpecifyFlow("priv")
	if err := s.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	errAbort := errors.New("abort")
	err := s.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		cur, e := repo.GetFlow(ctx, f.ID)
		if e != nil {
			return e
		}
		next := cur
		next.OriginalPrompt = "cambiado"
		next.Revision++
		if e = repo.SaveTransition(ctx, cur, next); e != nil {
			return e
		}
		if inTx, e := repo.GetFlow(ctx, f.ID); e != nil || inTx.OriginalPrompt != "cambiado" {
			t.Errorf("transaction must read its own staged payload: %q, %v", inTx.OriginalPrompt, e)
		}
		return errAbort
	})
	if !errors.Is(err, errAbort) {
		t.Fatal(err)
	}
	got, err := s.GetFlow(ctx, f.ID)
	if err != nil || got.OriginalPrompt != f.OriginalPrompt || got.Revision != 0 {
		t.Fatalf("after rollback = %q rev %d, %v; want previous state", got.OriginalPrompt, got.Revision, err)
	}
}

// Another instance (a second MCP process, or this one after a restart) only
// sees the reservation: it may read and cancel, never prepare, approve, ack or
// resend, and it cannot open a second pending flow for the same agent.
func TestEphemeralStore_LostPayloadGrantsNothing(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	owner := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	f := privateSpecifyFlow("priv")
	f.Phase, f.LastStablePhase, f.ProposedPrompt = domain.PhaseReview, domain.PhaseReview, "propuesta "+ephemeralSecret
	if err := owner.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	other := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	got, err := other.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.PrivateStateUnavailable || got.OriginalPrompt != "" || got.Preparation != nil {
		t.Fatalf("other instance = %+v; want reservation only", got)
	}
	active, found, err := other.ActiveFlow(ctx, "p", "codex")
	if err != nil || !found || active.ID != f.ID || !active.PrivateStateUnavailable {
		t.Fatalf("ActiveFlow = %+v %v %v; the reservation keeps the pending slot", active, found, err)
	}
	if err = other.CreateFlow(ctx, privateSpecifyFlow("second")); err == nil {
		t.Fatal("a second pending flow for the same project/agent was created")
	}
	next := got
	next.Revision++
	next.Phase = domain.PhaseApproved
	if err = other.SaveTransition(ctx, got, next); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
		t.Errorf("SaveTransition without payload = %v, want private_state_unavailable", err)
	}
	if _, _, err = other.Approve(ctx, f.ID, "d1", "reconstruido"); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
		t.Errorf("Approve without payload = %v, want private_state_unavailable", err)
	}
	if _, err = other.Ack(ctx, f.ID, "d1", "", "n"); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
		t.Errorf("Ack without payload = %v, want private_state_unavailable", err)
	}
	if err = other.ForceResend(ctx, f.ID, "d1", "d2"); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
		t.Errorf("ForceResend without payload = %v, want private_state_unavailable", err)
	}
	cancelled := got
	cancelled.Phase = domain.PhaseCancelled
	if err = other.SaveTransition(ctx, got, cancelled); err != nil {
		t.Fatalf("explicit cancel of the reservation = %v", err)
	}
	if _, found, err = owner.ActiveFlow(ctx, "p", "codex"); err != nil || found {
		t.Fatalf("pending after cancel = %v, %v", found, err)
	}
	requireNoSecretOnDisk(t, db)
}

func TestEphemeralStore_PersistentFlowsPassThrough(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	s := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	f := socraticaTestFlow("refine", "claude")
	if err := s.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	got, err := NewSocraticaStore(db, time.Second).GetFlow(ctx, f.ID)
	if err != nil || got.OriginalPrompt != "original" || got.PrivateStateUnavailable {
		t.Fatalf("persistent flow on disk = %+v, %v", got, err)
	}
	if err = s.DeleteFlow(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
}

// The remaining reservation paths: dispatch marks, resend from the payload,
// redaction and purges drop the payload, and provenance never matches a
// private flow because no digest of its text is stored.
func TestEphemeralStore_DispatchRedactAndPurge(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	s := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	f := privateSpecifyFlow("priv")
	f.Phase, f.LastStablePhase, f.ProposedPrompt = domain.PhaseReview, domain.PhaseReview, "propuesta "+ephemeralSecret
	f.Scrutiny = &domain.ScrutinySynthesis{Restatement: ephemeralSecret, Alternatives: []string{"x"}}
	if err := s.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Approve(ctx, f.ID, "d1", f.ProposedPrompt); err != nil {
		t.Fatal(err)
	}
	byDispatch, found, err := s.FindByDispatchID(ctx, "d1")
	if err != nil || !found || byDispatch.ApprovedPrompt != f.ProposedPrompt || byDispatch.Scrutiny.Restatement != ephemeralSecret {
		t.Fatalf("FindByDispatchID = %+v %v %v", byDispatch, found, err)
	}
	byDispatch.Scrutiny.Alternatives[0] = "mutated"
	if again, _ := s.GetFlow(ctx, f.ID); again.Scrutiny.Alternatives[0] != "x" {
		t.Fatal("a returned flow aliases the stored payload")
	}
	if matched, err := s.ConsumeDispatch(ctx, "d1", f.ProposedPrompt); err != nil || matched {
		t.Fatalf("ConsumeDispatch = %v, %v; a private flow has no provenance digest", matched, err)
	}
	other := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	if _, err = other.ClaimDispatch(ctx, f.ID, "d1", "owner", time.Unix(1, 0)); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
		t.Errorf("ClaimDispatch without payload = %v", err)
	}
	if err = other.MarkDispatched(ctx, f.ID, "d1"); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
		t.Errorf("MarkDispatched without payload = %v", err)
	}
	if ok, err := s.ClaimDispatch(ctx, f.ID, "d1", "owner", time.Unix(1, 0)); err != nil || !ok {
		t.Fatalf("ClaimDispatch = %v, %v", ok, err)
	}
	if err = s.ReleaseDispatch(ctx, f.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	if err = s.MarkDispatched(ctx, f.ID, "d1"); err != nil {
		t.Fatal(err)
	}
	if err = s.ForceResend(ctx, f.ID, "d1", "d2"); err != nil {
		t.Fatalf("ForceResend with payload = %v", err)
	}
	if err = s.MarkDispatched(ctx, f.ID, "d2"); err != nil {
		t.Fatal(err)
	}
	if err = s.RedactFlow(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	redacted, err := s.GetFlow(ctx, f.ID)
	// Without its payload a dispatched reservation can neither report nor resend,
	// so it says so instead of offering those routes (C-001, ACR 042 r2).
	if err != nil || redacted.OriginalPrompt != "" || !redacted.PrivateStateUnavailable {
		t.Fatalf("redacted dispatched flow = %+v, %v; want an unavailable reservation", redacted, err)
	}
	if err = s.ForceResend(ctx, f.ID, "d2", "d3"); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
		t.Errorf("ForceResend after redaction = %v; a lost payload cannot be resent", err)
	}
	if exists, err := s.FlowExists(ctx, f.ID); err != nil || !exists {
		t.Fatal(exists, err)
	}
	if hs, err := s.ListHistory(ctx, "p", "codex", 5); err != nil || len(hs) != 0 {
		t.Fatalf("private history = %+v, %v", hs, err)
	}
	if ids, err := s.PruneTerminal(ctx, "p", "codex", 1); err != nil || len(ids) != 0 {
		t.Fatal(ids, err)
	}
	if ids, err := s.PurgeTerminal(ctx, "p", time.Unix(1<<40, 0)); err != nil || len(ids) != 1 {
		t.Fatalf("PurgeTerminal = %v, %v", ids, err)
	}
	pending := privateSpecifyFlow("stale")
	if err = s.CreateFlow(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if ids, err := s.PurgeStale(ctx, "p", time.Unix(1<<40, 0)); err != nil || len(ids) != 1 {
		t.Fatalf("PurgeStale = %v, %v", ids, err)
	}
	s.state.mu.Lock()
	left := len(s.state.payloads)
	s.state.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d payloads survive their purged rows", left)
	}
	requireNoSecretOnDisk(t, db)
}

// A flow cancelled by another instance drops the owner's payload on its next
// read, so the private texts do not outlive the cancellation (C-006, ACR 042).
func TestEphemeralStore_CancelElsewhereDropsTheOwnerPayload(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	owner := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	f := privateSpecifyFlow("priv")
	if err := owner.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	other := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	got, err := other.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	cancelled := got
	cancelled.Phase = domain.PhaseCancelled
	if err = other.SaveTransition(ctx, got, cancelled); err != nil {
		t.Fatal(err)
	}
	read, err := owner.GetFlow(ctx, f.ID)
	if err != nil || read.Phase != domain.PhaseCancelled || read.OriginalPrompt != "" || read.Preparation != nil {
		t.Fatalf("owner read = %+v, %v; want the cancelled reservation without text", read, err)
	}
	if _, held := owner.state.payloads[f.ID]; held {
		t.Fatal("the owner still holds the private payload")
	}
}
