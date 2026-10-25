package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"mem/application/ports"
	"mem/domain"
)

func payloadCount(s *EphemeralSocraticaStore) int {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return len(s.state.payloads)
}

// The policy is fixed when the flow is created: whatever the private setting
// says later, no history row and no durable payload ever appears.
func TestSpecifyRetention_EphemeralFlowNeverReachesDurableStorage(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	s := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	f := privateSpecifyFlow("priv")
	f.Phase, f.LastStablePhase, f.ProposedPrompt = domain.PhaseReview, domain.PhaseReview, "propuesta "+ephemeralSecret
	if err := s.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	// The setting is now off: callers record history and send the approval as usual.
	if _, _, err := s.Approve(ctx, f.ID, "d1", f.ProposedPrompt); err != nil {
		t.Fatal(err)
	}
	h := domain.RefinementHistory{ID: f.ID, FlowID: f.ID, Project: "p", Client: "codex", Mode: domain.ModeSpecify, OriginalPrompt: f.OriginalPrompt, ApprovedPrompt: f.ProposedPrompt, Preparation: f.Preparation, CompletedAt: time.Unix(1, 0)}
	if err := s.UpsertHistory(ctx, h, 5); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := db.QueryRow("SELECT COUNT(*) FROM socratica_history").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("history rows = %d, %v", rows, err)
	}
	requireNoSecretOnDisk(t, db)
}

// Deleting the reservation (retention, purge or explicit delete) leaves no
// active copy of the payload in this instance, and no other instance can bring it back.
func TestSpecifyRetention_DeletingTheReservationDropsEveryCopy(t *testing.T) {
	ctx := context.Background()
	for name, remove := range map[string]func(*EphemeralSocraticaStore, string) error{
		"delete": func(s *EphemeralSocraticaStore, id string) error { return s.DeleteFlow(ctx, id) },
		"purge stale": func(s *EphemeralSocraticaStore, _ string) error {
			_, err := s.PurgeStale(ctx, "p", time.Unix(1<<40, 0))
			return err
		},
		"purge terminal": func(s *EphemeralSocraticaStore, id string) error {
			if _, err := s.inner.db.Exec("UPDATE socratica_flows SET phase='cancelled' WHERE id=?", id); err != nil {
				return err
			}
			_, err := s.PurgeTerminal(ctx, "p", time.Unix(1<<40, 0))
			return err
		},
	} {
		db := ephemeralTestDB(t)
		s := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
		f := privateSpecifyFlow("priv")
		if err := s.CreateFlow(ctx, f); err != nil {
			t.Fatal(err)
		}
		if payloadCount(s) != 1 {
			t.Fatalf("%s: setup payloads = %d", name, payloadCount(s))
		}
		if err := remove(s, f.ID); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n := payloadCount(s); n != 0 {
			t.Fatalf("%s: %d payloads survive their deleted reservation", name, n)
		}
		if exists, _ := s.FlowExists(ctx, f.ID); exists {
			t.Fatalf("%s: the reservation row survives", name)
		}
		other := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
		if _, found, _ := other.ActiveFlow(ctx, "p", "codex"); found {
			t.Fatalf("%s: another instance still sees a pending flow", name)
		}
		requireNoSecretOnDisk(t, db)
	}
}

// Retention of terminal flows drops the payload of the pruned ones only.
func TestSpecifyRetention_PruningDropsThePayloadOfThePrunedFlowOnly(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	s := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))
	old, newer := privateSpecifyFlow("old"), privateSpecifyFlow("newer")
	// Dispatched: the terminal phase that keeps its payload for the report (a
	// cancellation drops it).
	old.Phase, old.UpdatedAt = domain.PhaseDispatched, time.Unix(100, 0).UTC()
	newer.Phase, newer.UpdatedAt = domain.PhaseDispatched, time.Unix(900, 0).UTC()
	for _, f := range []domain.RefinementFlow{old, newer} {
		if err := s.CreateFlow(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := s.PruneTerminal(ctx, "p", "codex", 1)
	if err != nil || len(ids) != 1 || ids[0] != "old" {
		t.Fatalf("pruned = %v, %v", ids, err)
	}
	if payloadCount(s) != 1 {
		t.Fatalf("payloads after pruning = %d, want only the newer flow's", payloadCount(s))
	}
	if got, err := s.GetFlow(ctx, "newer"); err != nil || got.OriginalPrompt == "" {
		t.Fatalf("the newer flow lost its payload: %+v, %v", got, err)
	}
	requireNoSecretOnDisk(t, db)
}

// Persistent specify flows keep the preparation only in the flow row and in
// their history snapshot; deleting the flow removes the aggregate.
func TestSpecifyRetention_PersistentFlowLeavesNoOrphanedPreparation(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	storedSpecifyFlow(t, r, "flow", "review")
	var n int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM socratica_flows WHERE preparation_json IS NOT NULL").Scan(&n); err != nil || n != 1 {
		t.Fatalf("setup: %d, %v", n, err)
	}
	if err := r.DeleteFlow(ctx, "flow"); err != nil {
		t.Fatal(err)
	}
	if err := r.db.QueryRow("SELECT COUNT(*) FROM socratica_flows").Scan(&n); err != nil || n != 0 {
		t.Fatalf("flows left: %d, %v", n, err)
	}
	// Retention of terminal flows deletes the whole aggregate too.
	storedSpecifyFlow(t, r, "old", "dispatched")
	if _, err := r.db.Exec("UPDATE socratica_flows SET updated_at='2020-01-01T00:00:00Z' WHERE id='old'"); err != nil {
		t.Fatal(err)
	}
	ids, err := r.PurgeTerminal(ctx, "p", time.Unix(1<<40, 0))
	if err != nil || len(ids) != 1 {
		t.Fatalf("purge = %v, %v", ids, err)
	}
	if err := r.db.QueryRow("SELECT COUNT(*) FROM socratica_flows WHERE preparation_json IS NOT NULL").Scan(&n); err != nil || n != 0 {
		t.Fatalf("orphaned preparation rows: %d, %v", n, err)
	}
}

// Branches of the coordinated adapter that the main scenarios do not reach.
func TestEphemeralStore_PassThroughAndEdgeBranches(t *testing.T) {
	ctx := context.Background()
	db := ephemeralTestDB(t)
	s := NewEphemeralSocraticaStore(NewSocraticaStore(db, time.Second))

	// History of a flow that has no row yet goes to the SQL store as usual.
	h := domain.RefinementHistory{ID: "h", FlowID: "no-row", Project: "p", Client: "claude", Mode: domain.ModeRefine, OriginalPrompt: "o", CompletedAt: time.Unix(1, 0)}
	if err := s.UpsertHistory(ctx, h, 5); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ListHistory(ctx, "p", "claude", 5); err != nil || len(rows) != 1 {
		t.Fatalf("history = %v, %v", rows, err)
	}
	if err := s.UpsertHistory(ctx, h, 0); err == nil {
		t.Fatal("an invalid retention limit was accepted")
	}

	// A persistent flow resends through the SQL store; an ephemeral one without an
	// approved text has nothing to resend.
	p := socraticaTestFlow("p1", "claude")
	if err := s.CreateFlow(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Approve(ctx, p.ID, "d1", "texto"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDispatched(ctx, p.ID, "d1"); err != nil {
		t.Fatal(err)
	}
	if err := s.ForceResend(ctx, p.ID, "d1", "d2"); err != nil {
		t.Fatalf("persistent resend = %v", err)
	}
	eph := privateSpecifyFlow("e1")
	eph.Phase, eph.Client = domain.PhaseApproved, "codex"
	if err := s.CreateFlow(ctx, eph); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE socratica_flows SET dispatch_id='d9',dispatch_ack=1,phase='dispatched' WHERE id='e1'"); err != nil {
		t.Fatal(err)
	}
	if err := s.ForceResend(ctx, "e1", "d9", "d10"); !errors.Is(err, ports.ErrSocraticaNoMatch) {
		t.Fatalf("resend without an approved text = %v", err)
	}

	// Purging with nothing to purge, and a failing purge, keep the payloads alone.
	if ids, err := s.PurgeStale(ctx, "otro-proyecto", time.Unix(1<<40, 0)); err != nil || len(ids) != 0 {
		t.Fatalf("purge of another project = %v, %v", ids, err)
	}
	if _, err := s.PruneTerminal(ctx, "p", "codex", 0); err == nil {
		t.Fatal("an invalid retention limit was accepted")
	}
	if payloadCount(s) != 1 {
		t.Fatalf("payloads = %d", payloadCount(s))
	}
}
