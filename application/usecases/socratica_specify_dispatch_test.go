package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

// approvedSpecify returns a specify flow approved at revision 3.
func approvedSpecify(t *testing.T) (*SocraticaFlow, *socraticaFakeRepo, string, string) {
	t.Helper()
	u, r, id := reviewedSpecify(t)
	res, err := u.ApproveSpecify(context.Background(), id, 3)
	if err != nil {
		t.Fatal(err)
	}
	return u, r, id, res.Flow.DispatchID
}

func ackEvent(dispatch, nonce string) domain.Event {
	return domain.Event{Kind: "ack", DispatchID: dispatch, Nonce: nonce}
}

// commitFailingUOW runs the transaction and then fails like a commit error:
// the wrapped fake restores the state, as a rollback does.
type commitFailingUOW struct {
	inner socraticaFakeUOW
	fail  bool
}

func (u *commitFailingUOW) WithTransaction(ctx context.Context, fn func(ports.SocraticaRepository) error) error {
	if !u.fail {
		return u.inner.WithTransaction(ctx, fn)
	}
	u.fail = false
	return u.inner.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		if err := fn(repo); err != nil {
			return err
		}
		return errors.New("commit failed")
	})
}

func TestSpecifyDispatch_GrantIsPublishedOnlyAfterTheCommit(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := approvedSpecify(t)
	failing := &commitFailingUOW{inner: socraticaFakeUOW{r}, fail: true}
	u.UOW = failing
	res, err := u.Apply(ctx, id, ackEvent(dispatch, "n1"), OriginMCP)
	if err == nil || res.Flow.ExecutionGranted || res.Flow.Acked {
		t.Fatalf("a rolled back ack must grant nothing: %+v, %v", res.Flow, err)
	}
	if r.flow.Phase != domain.PhaseApproved || r.flow.DispatchAck {
		t.Fatalf("rollback left the flow %s (ack %v)", r.flow.Phase, r.flow.DispatchAck)
	}
	granted, err := u.Apply(ctx, id, ackEvent(dispatch, "n1"), OriginMCP)
	if err != nil || !granted.Flow.ExecutionGranted || !granted.Flow.Acked {
		t.Fatalf("committed ack = %+v, %v", granted.Flow, err)
	}
}

func TestSpecifyDispatch_ReplayLostResponseAndOtherNonce(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := approvedSpecify(t)
	first, err := u.Apply(ctx, id, ackEvent(dispatch, "n1"), OriginMCP)
	if err != nil || !first.Flow.ExecutionGranted {
		t.Fatal(first.Flow, err)
	}
	if !strings.Contains(first.Next, "speckit-specify") || !strings.Contains(first.Next, "approved_prompt") || !strings.Contains(first.Next, "report") {
		t.Fatalf("grant next = %q", first.Next)
	}
	// The first response is lost: the agent repeats the same nonce.
	replay, err := u.Apply(ctx, id, ackEvent(dispatch, "n1"), OriginMCP)
	if err != nil || !replay.Flow.Acked || replay.Flow.ExecutionGranted {
		t.Fatalf("replay = %+v, %v", replay.Flow, err)
	}
	for _, want := range []string{"NO ejecutes", "replay", "force_resend"} {
		if !strings.Contains(replay.Next, want) {
			t.Errorf("replay next lacks %q: %q", want, replay.Next)
		}
	}
	other, err := u.Apply(ctx, id, ackEvent(dispatch, "n2"), OriginMCP)
	if err != nil || other.Flow.Acked || !other.Flow.AlreadyDispatched || other.Flow.ExecutionGranted || !strings.Contains(other.Next, "NO ejecutes") {
		t.Fatalf("other nonce = %+v next %q, %v", other.Flow, other.Next, err)
	}
	if _, err = u.Apply(ctx, id, ackEvent("viejo", "n1"), OriginMCP); !errors.Is(err, ports.ErrAckRejected) {
		t.Fatalf("stale dispatch = %v", err)
	}
	got, _ := u.Get(ctx, id)
	if got.Flow.ExecutionGranted || !strings.Contains(got.Next, "NO ejecutes") || !strings.Contains(got.Next, "report") {
		t.Fatalf("get after dispatch = %q", got.Next)
	}
	// The hook provenance mark is a different thing and stays untouched.
	if r.flow.DispatchConsumed {
		t.Fatal("ack must not consume the hook provenance mark")
	}
}

func TestSpecifyDispatch_ClaimedFlowIsUncertainAndNeverGrants(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := approvedSpecify(t)
	r.flow.DispatchClaimOwner = "otro-proceso"
	got, err := u.Get(ctx, id)
	if err != nil || !got.Flow.Uncertain || got.Next != NextUncertain || got.Flow.ExecutionGranted {
		t.Fatalf("get of a claimed dispatch = %+v next %q, %v", got.Flow, got.Next, err)
	}
	if _, err = u.Apply(ctx, id, ackEvent(dispatch, "n1"), OriginMCP); err == nil {
		t.Fatal("an MCP ack over a claimed dispatch was accepted")
	}
	if r.flow.DispatchAck {
		t.Fatal("nothing may be consumed")
	}
	// Another process holds the lock: in flight, no actions.
	lock := u.Lock.(*socraticaFakeLock)
	lock.held = true
	busy, err := u.Get(ctx, id)
	if err != nil || !busy.Flow.InFlight || busy.Next != NextInFlight {
		t.Fatalf("in flight = %+v next %q, %v", busy.Flow, busy.Next, err)
	}
}

func TestSpecifyDispatch_ExplicitResendCreatesANewSingleAuthorization(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := approvedSpecify(t)
	if _, err := u.Apply(ctx, id, ackEvent(dispatch, "n1"), OriginMCP); err != nil {
		t.Fatal(err)
	}
	if _, err := u.ReportOutcome(ctx, id, dispatch, domain.SpecifyOutcome{Status: domain.OutcomeUncertain, Message: "no se pudo verificar"}, false); err != nil {
		t.Fatal(err)
	}
	resent, err := u.Apply(ctx, id, domain.Event{Kind: "force_resend", DispatchID: dispatch}, OriginMCP)
	if err != nil {
		t.Fatal(err)
	}
	f := resent.Flow
	if f.Phase != domain.PhaseApproved || f.DispatchID == dispatch || f.DispatchAck || f.ExecutionGranted || f.Preparation.Outcome != nil {
		t.Fatalf("resend = phase %s dispatch %q (old %q) outcome %v", f.Phase, f.DispatchID, dispatch, f.Preparation.Outcome)
	}
	if r.flow.Preparation.Outcome != nil || r.flow.DispatchID != f.DispatchID || r.flow.ApprovedPrompt == "" {
		t.Fatal("the new authorization was not saved with the approved content kept")
	}
	if _, err = u.Apply(ctx, id, ackEvent(dispatch, "n1"), OriginMCP); !errors.Is(err, ports.ErrAckRejected) {
		t.Fatalf("the old dispatch must be dead: %v", err)
	}
	again, err := u.Apply(ctx, id, ackEvent(f.DispatchID, "n9"), OriginMCP)
	if err != nil || !again.Flow.ExecutionGranted {
		t.Fatalf("new authorization = %+v, %v", again.Flow, err)
	}
	// A resend of content that no longer matches is refused.
	r.flow.Preparation.Content.Stories.Stories[0].Need = "[REDACTED:key]"
	if _, err = u.Apply(ctx, id, domain.Event{Kind: "force_resend", DispatchID: f.DispatchID}, OriginMCP); !errors.Is(err, domain.ErrReviewStale) {
		t.Fatalf("resend over sanitised content = %v", err)
	}
}

func TestSpecifyDispatch_ConcurrentAcksOfTheFakeGrantOnce(t *testing.T) {
	ctx := context.Background()
	u, _, id, dispatch := approvedSpecify(t)
	grants := 0
	for _, nonce := range []string{"a", "b", "c", "d"} {
		res, err := u.Apply(ctx, id, ackEvent(dispatch, nonce), OriginMCP)
		if err != nil {
			t.Fatal(err)
		}
		if res.Flow.ExecutionGranted {
			grants++
		}
	}
	if grants != 1 {
		t.Fatalf("grants = %d, want 1", grants)
	}
}
