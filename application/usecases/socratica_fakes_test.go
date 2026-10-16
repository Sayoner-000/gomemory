package usecases

import (
	"context"
	"errors"
	"mem/application/ports"
	"mem/domain"
	"sync"
	"time"
)

// socraticaFakeRepo models one flow plus its history row; enough for the use
// case, whose SQL semantics are covered against SQLite in persistence.
type socraticaFakeRepo struct {
	mu           sync.Mutex
	flow         domain.RefinementFlow
	history      map[string]domain.RefinementHistory
	historyCalls int
	failHistory  bool
	ackFailures  int
	calls        int
	created      int
	redacted     int
	deleted      int
	purgeCutoffs []time.Time
	purgedIDs    []string
}

func (r *socraticaFakeRepo) touch() { r.calls++ }

func (r *socraticaFakeRepo) CreateFlow(_ context.Context, f domain.RefinementFlow) error {
	r.touch()
	r.created++
	r.flow = f
	return nil
}
func (r *socraticaFakeRepo) GetFlow(_ context.Context, id string) (domain.RefinementFlow, error) {
	r.touch()
	if r.flow.ID != id {
		return domain.RefinementFlow{}, errors.New("not found")
	}
	return r.flow, nil
}
func (r *socraticaFakeRepo) ActiveFlow(_ context.Context, project, client string) (domain.RefinementFlow, bool, error) {
	r.touch()
	f := r.flow
	return f, f.ID != "" && f.Project == project && f.Client == client && !f.Phase.Terminal(), nil
}
func (r *socraticaFakeRepo) SaveTransition(_ context.Context, _, after domain.RefinementFlow) error {
	r.touch()
	r.flow = after
	return nil
}
func (r *socraticaFakeRepo) UpsertHistory(_ context.Context, h domain.RefinementHistory, _ int) error {
	r.touch()
	if r.failHistory {
		return errors.New("history failed")
	}
	r.historyCalls++
	if r.history == nil {
		r.history = map[string]domain.RefinementHistory{}
	}
	r.history[h.FlowID] = h
	return nil
}
func (r *socraticaFakeRepo) ListHistory(context.Context, string, string, int) ([]domain.RefinementHistory, error) {
	r.touch()
	var rows []domain.RefinementHistory
	for _, h := range r.history {
		rows = append(rows, h)
	}
	return rows, nil
}
func (r *socraticaFakeRepo) Approve(_ context.Context, _, id, prompt string) (domain.RefinementFlow, bool, error) {
	r.touch()
	switch r.flow.Phase {
	case domain.PhaseReview, domain.PhaseFailed:
		r.flow.Phase = domain.PhaseApproved
		if r.flow.DispatchID == "" {
			r.flow.DispatchID = id
		}
		r.flow.ApprovedPrompt = prompt
		return r.flow, false, nil
	case domain.PhaseApproved, domain.PhaseDispatched:
		return r.flow, true, nil
	}
	return r.flow, false, domain.ErrInvalidTransition
}
func (r *socraticaFakeRepo) ClaimDispatch(_ context.Context, _, dispatch, owner string, now time.Time) (bool, error) {
	r.touch()
	if r.flow.DispatchClaimOwner != "" || r.flow.DispatchID != dispatch || r.flow.Phase != domain.PhaseApproved {
		return false, nil
	}
	r.flow.DispatchClaimOwner = owner
	r.flow.DispatchClaimedAt = &now
	return true, nil
}
func (r *socraticaFakeRepo) ReleaseDispatch(_ context.Context, _, owner string) error {
	r.touch()
	if r.flow.DispatchClaimOwner == owner {
		r.flow.DispatchClaimOwner = ""
		r.flow.DispatchClaimedAt = nil
	}
	return nil
}
func (r *socraticaFakeRepo) Ack(_ context.Context, _, id, owner, nonce string) (bool, bool, error) {
	r.touch()
	if r.ackFailures > 0 {
		r.ackFailures--
		return false, false, errors.New("ack unavailable")
	}
	if id != r.flow.DispatchID || owner != r.flow.DispatchClaimOwner {
		return false, false, ports.ErrAckRejected
	}
	if r.flow.DispatchAck {
		return nonce != "" && nonce == r.flow.AckNonce, nonce == "" || nonce != r.flow.AckNonce, nil
	}
	r.flow.DispatchAck = true
	r.flow.Phase = domain.PhaseDispatched
	r.flow.AckNonce = nonce
	return true, false, nil
}
func (r *socraticaFakeRepo) MarkDispatched(_ context.Context, _, dispatch string) error {
	r.touch()
	if r.flow.DispatchID == dispatch && r.flow.Phase == domain.PhaseApproved {
		r.flow.Phase = domain.PhaseDispatched
		r.flow.DispatchAck = true
	}
	return nil
}
func (r *socraticaFakeRepo) ForceResend(_ context.Context, _, oldID, newID string) error {
	r.touch()
	if r.flow.DispatchID != oldID {
		return errors.New("stale dispatch")
	}
	r.flow.Phase = domain.PhaseApproved
	r.flow.DispatchID = newID
	r.flow.DispatchAck = false
	r.flow.AckNonce = ""
	r.flow.DispatchClaimOwner = ""
	r.flow.DispatchClaimedAt = nil
	return nil
}
func (r *socraticaFakeRepo) RedactFlow(context.Context, string) error {
	r.touch()
	r.redacted++
	r.flow.OriginalPrompt = ""
	r.flow.ProposedPrompt = ""
	r.flow.ApprovedPrompt = ""
	return nil
}
func (r *socraticaFakeRepo) DeleteFlow(context.Context, string) error {
	r.touch()
	r.deleted++
	r.flow = domain.RefinementFlow{}
	return nil
}
func (r *socraticaFakeRepo) FindByDispatchID(context.Context, string) (domain.RefinementFlow, bool, error) {
	r.touch()
	return r.flow, r.flow.DispatchID != "", nil
}
func (r *socraticaFakeRepo) ConsumeDispatch(context.Context, string, string) (bool, error) {
	r.touch()
	return false, nil
}
func (r *socraticaFakeRepo) PurgeStale(_ context.Context, _ string, before time.Time) ([]string, error) {
	r.touch()
	r.purgeCutoffs = append(r.purgeCutoffs, before)
	return r.purgedIDs, nil
}
func (r *socraticaFakeRepo) PurgeTerminal(_ context.Context, _ string, before time.Time) ([]string, error) {
	r.touch()
	r.purgeCutoffs = append(r.purgeCutoffs, before)
	return nil, nil
}
func (r *socraticaFakeRepo) PruneTerminal(context.Context, string, string, int) ([]string, error) {
	r.touch()
	return nil, nil
}

type socraticaFakeUOW struct{ repo *socraticaFakeRepo }

// WithTransaction restores flow and history when fn fails, like a rollback.
func (u socraticaFakeUOW) WithTransaction(_ context.Context, fn func(ports.SocraticaRepository) error) error {
	before := u.repo.flow
	history := map[string]domain.RefinementHistory{}
	for k, v := range u.repo.history {
		history[k] = v
	}
	err := fn(u.repo)
	if err != nil {
		u.repo.flow = before
		u.repo.history = history
	}
	return err
}

type socraticaFakeClock struct{}

func (socraticaFakeClock) Now() time.Time { return time.Unix(1000, 0).UTC() }

// socraticaFakeLock stands in for the per-flow OS lock; hold simulates another
// process (or a paused owner) keeping it.
type socraticaFakeLock struct {
	mu      sync.Mutex
	held    bool
	removed []string
}

func (l *socraticaFakeLock) TryAcquire(string) (func(), bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return nil, false, nil
	}
	l.held = true
	return func() { l.mu.Lock(); defer l.mu.Unlock(); l.held = false }, true, nil
}
func (l *socraticaFakeLock) Remove(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.removed = append(l.removed, id)
	return nil
}

type socraticaFakeGenerator struct{ calls int }

func (g *socraticaFakeGenerator) Name() string    { return "fake" }
func (g *socraticaFakeGenerator) Available() bool { return true }
func (g *socraticaFakeGenerator) GenerateQuestions(context.Context, ports.SocraticGenerationRequest) (domain.GenerationResult, error) {
	g.calls++
	return domain.GenerationResult{}, nil
}
func (g *socraticaFakeGenerator) Rewrite(context.Context, ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	g.calls++
	return domain.RewriteResult{Proposed: "rewrite"}, nil
}

// socraticaFakeLauncher follows the AgentLauncher contract: start, onStarted,
// then wait. started/proceed pause it between the claim and Start.
type socraticaFakeLauncher struct {
	mu        sync.Mutex
	calls     int
	repo      *socraticaFakeRepo
	fail      bool
	started   chan struct{}
	proceed   chan struct{}
	requests  []ports.LaunchRequest
	ackInside bool
}

func (l *socraticaFakeLauncher) Launch(_ context.Context, req ports.LaunchRequest, onStarted func() error) error {
	l.mu.Lock()
	l.calls++
	l.requests = append(l.requests, req)
	l.mu.Unlock()
	if l.fail {
		return ports.ErrLaunchFailed
	}
	if l.started != nil {
		close(l.started)
		<-l.proceed
	}
	if err := onStarted(); err != nil {
		return errors.Join(ports.ErrConfirmFailed, err)
	}
	// The process is still "running" here: the ack must already exist.
	l.ackInside = l.repo.flow.DispatchAck
	return nil
}
