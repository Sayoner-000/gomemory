package usecases

import (
	"context"
	"errors"
	"mem/application/ports"
	"mem/domain"
	"testing"
)

func socraticaFixture() (*SocraticaFlow, *socraticaFakeRepo) {
	r := &socraticaFakeRepo{flow: domain.RefinementFlow{ID: "f", Project: "p", Client: "claude", OriginalPrompt: "original", ProposedPrompt: "proposal", Phase: domain.PhaseReview, LastStablePhase: domain.PhaseReview}}
	n := 0
	newID := func() string { n++; return "id" + string(rune('0'+n)) }
	return &SocraticaFlow{Repo: r, UOW: socraticaFakeUOW{r}, Clock: socraticaFakeClock{}, NewID: newID, Lock: &socraticaFakeLock{}}, r
}

func TestSocraticaFlowUseCase(t *testing.T) {
	ctx := context.Background()
	t.Run("rollback", func(t *testing.T) {
		u, r := socraticaFixture()
		r.failHistory = true
		_, err := u.Apply(ctx, "f", domain.Event{Kind: "edit", Proposed: "new"}, OriginMCP)
		if err == nil || r.flow.ProposedPrompt != "proposal" || r.flow.Revision != 0 {
			t.Fatal(r.flow, err)
		}
	})
	t.Run("dispatch acks inside onStarted", func(t *testing.T) {
		u, r := socraticaFixture()
		l := &socraticaFakeLauncher{repo: r}
		res, err := u.Execute(ctx, "f", "claude", l)
		if err != nil || !res.Flow.DispatchAck || l.calls != 1 || !l.ackInside {
			t.Fatal(res, err, l.ackInside)
		}
		if got := l.requests[0]; got.Prompt != "proposal" || got.Env["GOMEMORY_SOCRATICA_DISPATCH"] != res.Flow.DispatchID || got.Client != "claude" {
			t.Fatal(got)
		}
		res, err = u.Execute(ctx, "f", "claude", l)
		if err != nil || l.calls != 1 || !res.Flow.AlreadyDispatched || res.Next != NextConsumed {
			t.Fatal(l.calls, res, err)
		}
	})
	t.Run("start failure releases own claim", func(t *testing.T) {
		u, r := socraticaFixture()
		l := &socraticaFakeLauncher{repo: r, fail: true}
		_, err := u.Execute(ctx, "f", "claude", l)
		if !errors.Is(err, ports.ErrLaunchFailed) || r.flow.DispatchAck || r.flow.DispatchClaimOwner != "" || r.flow.Phase != domain.PhaseApproved {
			t.Fatal(r.flow, err)
		}
		res, err := u.Resume(ctx, "f", OriginLocal)
		if err != nil || res.Flow.Phase != domain.PhaseReview || !res.Flow.Recovered || res.Next != NextRecoverable {
			t.Fatal(res, err)
		}
	})
	t.Run("confirm failure keeps claim", func(t *testing.T) {
		u, r := socraticaFixture()
		r.ackFailures = 2
		l := &socraticaFakeLauncher{repo: r}
		_, err := u.Execute(ctx, "f", "claude", l)
		if !errors.Is(err, ErrDispatchUnconfirmed) || r.flow.DispatchClaimOwner == "" {
			t.Fatal(r.flow, err)
		}
		_, err = u.Execute(ctx, "f", "claude", l)
		if !errors.Is(err, ErrDispatchUncertain) || l.calls != 1 {
			t.Fatal(l.calls, err)
		}
		res, err := u.Resume(ctx, "f", OriginMCP)
		if err != nil || !res.Flow.Uncertain || res.Flow.Phase != domain.PhaseApproved || res.Next != NextUncertain {
			t.Fatal(res, err)
		}
	})
	t.Run("confirm retry after wait succeeds", func(t *testing.T) {
		u, r := socraticaFixture()
		r.ackFailures = 1
		l := &socraticaFakeLauncher{repo: r}
		res, err := u.Execute(ctx, "f", "claude", l)
		if err != nil || res.Flow.Phase != domain.PhaseDispatched || l.calls != 1 {
			t.Fatal(res, err)
		}
	})
	t.Run("paused owner blocks reconciliation", func(t *testing.T) {
		u, r := socraticaFixture()
		l := &socraticaFakeLauncher{repo: r, started: make(chan struct{}), proceed: make(chan struct{})}
		done := make(chan error, 1)
		go func() { _, err := u.Execute(ctx, "f", "claude", l); done <- err }()
		<-l.started
		claimed := r.flow
		for _, kind := range []string{"force_resend", "mark_dispatched", "cancel"} {
			res, err := u.Apply(ctx, "f", domain.Event{Kind: kind, DispatchID: claimed.DispatchID}, OriginLocal)
			if err != nil || !res.Flow.InFlight || res.Next != NextInFlight || r.flow.DispatchID != claimed.DispatchID || r.flow.Phase != domain.PhaseApproved {
				t.Fatal(kind, res, err)
			}
		}
		res, err := u.Execute(ctx, "f", "claude", l)
		if err != nil || !res.Flow.InFlight {
			t.Fatal(res, err)
		}
		close(l.proceed)
		if err = <-done; err != nil || r.flow.Phase != domain.PhaseDispatched {
			t.Fatal(r.flow, err)
		}
		if l.calls != 1 {
			t.Fatal(l.calls)
		}
	})
	t.Run("dead owner needs explicit resend", func(t *testing.T) {
		u, r := socraticaFixture()
		r.flow.Phase = domain.PhaseApproved
		r.flow.ApprovedPrompt = "proposal"
		r.flow.DispatchID = "old"
		r.flow.DispatchClaimOwner = "A"
		res, err := u.Resume(ctx, "f", OriginLocal)
		if err != nil || !res.Flow.Uncertain {
			t.Fatal(res, err)
		}
		l := &socraticaFakeLauncher{repo: r}
		if _, err = u.Execute(ctx, "f", "claude", l); !errors.Is(err, ErrDispatchUncertain) || l.calls != 0 {
			t.Fatal(l.calls, err)
		}
		res, err = u.Apply(ctx, "f", domain.Event{Kind: "force_resend", DispatchID: "old"}, OriginLocal)
		if err != nil || res.Flow.DispatchID == "old" || r.flow.DispatchClaimOwner != "" {
			t.Fatal(res, err)
		}
		if _, err = u.Execute(ctx, "f", "claude", l); err != nil || l.calls != 1 {
			t.Fatal(l.calls, err)
		}
		if _, _, err = r.Ack(ctx, "f", "old", "A", "late"); !errors.Is(err, ports.ErrAckRejected) {
			t.Fatal("late ack accepted", err)
		}
	})
	t.Run("MCP never generates", func(t *testing.T) {
		u, r := socraticaFixture()
		r.flow.Phase = domain.PhaseCaptured
		g := &socraticaFakeGenerator{}
		u.Generator = g
		res, err := u.Apply(ctx, "f", domain.Event{Kind: "resume"}, OriginMCP)
		if err != nil || g.calls != 0 || res.Next == "" {
			t.Fatal(g.calls, err)
		}
		_, err = u.Apply(ctx, "f", domain.Event{Kind: "resume"}, OriginLocal)
		if err != nil || g.calls != 2 || r.flow.ProposedPrompt != "rewrite" {
			t.Fatal(g.calls, r.flow, err)
		}
		// Resuming a stable phase does not generate again (idempotent).
		if _, err = u.Resume(ctx, "f", OriginLocal); err != nil || g.calls != 2 {
			t.Fatal(g.calls, err)
		}
		r.flow.Phase = domain.PhaseRewriting
		if _, err = u.Resume(ctx, "f", OriginLocal); err != nil || g.calls != 3 {
			t.Fatal(g.calls, err)
		}
	})
	t.Run("concurrent Execute launches once", func(t *testing.T) {
		u, r := socraticaFixture()
		l := &socraticaFakeLauncher{repo: r, started: make(chan struct{}), proceed: make(chan struct{})}
		done := make(chan error, 1)
		go func() { _, err := u.Execute(ctx, "f", "claude", l); done <- err }()
		<-l.started
		res, err := u.Execute(ctx, "f", "claude", l)
		if err != nil || !res.Flow.InFlight {
			t.Fatal(res, err)
		}
		close(l.proceed)
		if err = <-done; err != nil {
			t.Fatal(err)
		}
		if l.calls != 1 {
			t.Fatal(l.calls)
		}
	})
	t.Run("Execute from failed sends the original", func(t *testing.T) {
		u, r := socraticaFixture()
		r.flow.Phase = domain.PhaseFailed
		l := &socraticaFakeLauncher{repo: r}
		res, err := u.Execute(ctx, "f", "claude", l)
		if err != nil || l.requests[0].Prompt != "original" || res.Flow.Phase != domain.PhaseDispatched {
			t.Fatal(res, err)
		}
	})
	t.Run("invalid settings fail fast", func(t *testing.T) {
		u, r := socraticaFixture()
		zero := 0
		u.Settings.HistoryLimit = &zero
		var invalid *domain.ErrInvalidSocraticaSettings
		if _, err := u.Apply(ctx, "f", domain.Event{Kind: "approve"}, OriginMCP); !errors.As(err, &invalid) {
			t.Fatal(err)
		}
		if _, err := u.Execute(ctx, "f", "claude", &socraticaFakeLauncher{repo: r}); !errors.As(err, &invalid) {
			t.Fatal(err)
		}
		if _, err := u.Start(ctx, "p", "claude", domain.ModeRefine, "x"); !errors.As(err, &invalid) {
			t.Fatal(err)
		}
		if r.calls != 0 {
			t.Fatalf("repository touched %d times", r.calls)
		}
	})
	t.Run("start recovers the pending flow", func(t *testing.T) {
		u, r := socraticaFixture()
		res, err := u.Start(ctx, "p", "claude", domain.ModeRefine, "another")
		if err != nil || !res.Flow.Recovered || res.Flow.ID != "f" || r.created != 0 {
			t.Fatal(res, err)
		}
		r.flow.Client = "codex"
		res, err = u.Start(ctx, "p", "claude", domain.ModeRefine, "fresh")
		if err != nil || res.Flow.Recovered || r.created != 1 || res.Flow.Phase != domain.PhaseCaptured {
			t.Fatal(res, err)
		}
	})
	t.Run("history follows review, edit and approve", func(t *testing.T) {
		u, r := socraticaFixture()
		r.flow.Phase = domain.PhaseRewriting
		if _, err := u.Apply(ctx, "f", domain.Event{Kind: "rewrite_ready", Proposed: "first"}, OriginMCP); err != nil || r.history["f"].ProposedPrompt != "first" {
			t.Fatal(r.history, err)
		}
		if _, err := u.Apply(ctx, "f", domain.Event{Kind: "edit", Proposed: "second"}, OriginMCP); err != nil || r.history["f"].ProposedPrompt != "second" {
			t.Fatal(r.history, err)
		}
		if _, err := u.Apply(ctx, "f", domain.Event{Kind: "approve"}, OriginMCP); err != nil || r.history["f"].ApprovedPrompt != "second" || r.historyCalls != 3 {
			t.Fatal(r.history, err)
		}
	})
	t.Run("private mode", func(t *testing.T) {
		u, r := socraticaFixture()
		u.Settings.Private = true
		l := &socraticaFakeLauncher{repo: r}
		res, err := u.Execute(ctx, "f", "claude", l)
		if err != nil || r.historyCalls != 0 || r.redacted != 1 || r.flow.DispatchID == "" || r.flow.ApprovedPrompt != "" || res.Flow.ApprovedPrompt != "" {
			t.Fatal(r.flow, err)
		}
		u2, r2 := socraticaFixture()
		u2.Settings.Private = true
		if _, err = u2.Apply(ctx, "f", domain.Event{Kind: "cancel"}, OriginMCP); err != nil || r2.deleted != 1 {
			t.Fatal(r2.deleted, err)
		}
		if got := u2.Lock.(*socraticaFakeLock).removed; len(got) != 1 || got[0] != "f" {
			t.Fatal("lock file of deleted flow kept", got)
		}
	})
	t.Run("MCP ack in private mode redacts", func(t *testing.T) {
		u, r := socraticaFixture()
		u.Settings.Private = true
		res, err := u.Apply(ctx, "f", domain.Event{Kind: "approve"}, OriginMCP)
		if err != nil {
			t.Fatal(err)
		}
		res, err = u.Apply(ctx, "f", domain.Event{Kind: "ack", DispatchID: res.Flow.DispatchID, Nonce: "n"}, OriginMCP)
		if err != nil || !res.Flow.Acked || r.redacted != 1 || r.deleted != 0 || res.Flow.OriginalPrompt != "" {
			t.Fatal(res, err)
		}
	})
	t.Run("resume of approved depends on claim and lock", func(t *testing.T) {
		u, r := socraticaFixture()
		r.flow.Phase = domain.PhaseApproved
		r.flow.DispatchID = "d"
		r.flow.ApprovedPrompt = "proposal"
		got, err := u.Get(ctx, "f")
		if err != nil || got.Next != NextRecoverable || got.Flow.Phase != domain.PhaseApproved {
			t.Fatal(got, err)
		}
		r.flow.DispatchClaimOwner = "A"
		if got, err = u.Get(ctx, "f"); err != nil || !got.Flow.Uncertain || got.Next != NextUncertain {
			t.Fatal(got, err)
		}
		lock := u.Lock.(*socraticaFakeLock)
		lock.held = true
		if got, err = u.Resume(ctx, "f", OriginMCP); err != nil || !got.Flow.InFlight || got.Flow.Phase != domain.PhaseApproved {
			t.Fatal(got, err)
		}
		if got, err = u.Get(ctx, "f"); err != nil || !got.Flow.InFlight {
			t.Fatal(got, err)
		}
	})
	t.Run("PurgeStale delegates and drops locks", func(t *testing.T) {
		u, r := socraticaFixture()
		r.purgedIDs = []string{"old"}
		if err := u.PurgeStale(ctx, "p"); err != nil {
			t.Fatal(err)
		}
		now := socraticaFakeClock{}.Now()
		if len(r.purgeCutoffs) != 2 || !r.purgeCutoffs[0].Equal(now.Add(-domain.PendingMaxAge)) || !r.purgeCutoffs[1].Equal(now.Add(-domain.TerminalRetention)) {
			t.Fatal(r.purgeCutoffs)
		}
		if got := u.Lock.(*socraticaFakeLock).removed; len(got) != 1 || got[0] != "old" {
			t.Fatal(got)
		}
	})
	t.Run("foreign ack is rejected", func(t *testing.T) {
		u, _ := socraticaFixture()
		res, err := u.Apply(ctx, "f", domain.Event{Kind: "approve"}, OriginMCP)
		if err != nil || res.Next != NextReadyToAck {
			t.Fatal(res, err)
		}
		if _, err = u.Apply(ctx, "f", domain.Event{Kind: "ack", DispatchID: "other", Nonce: "n"}, OriginMCP); !errors.Is(err, ports.ErrAckRejected) {
			t.Fatal(err)
		}
	})
}
