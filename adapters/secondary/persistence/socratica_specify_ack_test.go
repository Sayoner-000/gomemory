package persistence

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

// approvedSpecifyFlow stores a specify flow approved for dispatch "d1".
func approvedSpecifyFlow(t *testing.T, r *SocraticaStore, id string) {
	t.Helper()
	f := specifyTestFlow(id)
	f.Phase, f.LastStablePhase, f.ProposedPrompt = domain.PhaseReview, domain.PhaseReview, "texto revisado"
	if err := r.CreateFlow(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Approve(context.Background(), id, "d1", "texto revisado"); err != nil {
		t.Fatal(err)
	}
}

func TestSpecifyAck_OnlyFirstConfirmationGrants(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	approvedSpecifyFlow(t, r, "spec")
	first, err := r.Ack(ctx, "spec", "d1", "", "n1")
	if err != nil || first != (ports.AckResult{Acked: true, Granted: true}) {
		t.Fatalf("first ack = %+v, %v; want acked and granted", first, err)
	}
	replay, err := r.Ack(ctx, "spec", "d1", "", "n1")
	if err != nil || replay != (ports.AckResult{Acked: true}) {
		t.Fatalf("same-nonce replay = %+v, %v; want acked without grant", replay, err)
	}
	other, err := r.Ack(ctx, "spec", "d1", "", "n2")
	if err != nil || other != (ports.AckResult{AlreadyDispatched: true}) {
		t.Fatalf("other nonce = %+v, %v; want already dispatched without grant", other, err)
	}
	if _, err = r.Ack(ctx, "spec", "stale", "", "n1"); !errors.Is(err, ErrSocraticaAckRejected) {
		t.Fatalf("stale dispatch = %v, want rejected", err)
	}
}

func TestSpecifyAck_ConcurrentAcksGrantOnce(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	approvedSpecifyFlow(t, r, "spec")
	var wg sync.WaitGroup
	var mu sync.Mutex
	grants := 0
	for i := range 8 {
		wg.Add(1)
		go func(nonce string) {
			defer wg.Done()
			res, err := r.Ack(ctx, "spec", "d1", "", nonce)
			if err != nil {
				t.Error(err)
				return
			}
			if res.Granted {
				mu.Lock()
				grants++
				mu.Unlock()
			}
		}(fmt.Sprintf("n%d", i))
	}
	wg.Wait()
	if grants != 1 {
		t.Fatalf("grants = %d, want exactly 1", grants)
	}
}

// A first transition whose transaction is rolled back grants nothing that
// survives: the flow stays approved and the next committed ack is the grant.
func TestSpecifyAck_RolledBackGrantIsNotConsumed(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	approvedSpecifyFlow(t, r, "spec")
	errCommit := errors.New("commit fails")
	err := r.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		res, e := repo.Ack(ctx, "spec", "d1", "", "n1")
		if e != nil || !res.Granted {
			t.Fatalf("ack inside transaction = %+v, %v", res, e)
		}
		return errCommit
	})
	if !errors.Is(err, errCommit) {
		t.Fatal(err)
	}
	f, err := r.GetFlow(ctx, "spec")
	if err != nil || f.Phase != domain.PhaseApproved || f.DispatchAck {
		t.Fatalf("after rollback = %s ack=%v, %v; want approved", f.Phase, f.DispatchAck, err)
	}
	res, err := r.Ack(ctx, "spec", "d1", "", "n1")
	if err != nil || !res.Granted {
		t.Fatalf("committed ack after rollback = %+v, %v; want granted", res, err)
	}
}
