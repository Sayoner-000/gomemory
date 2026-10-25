package persistence

import (
	"context"
	"slices"
	"sync"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// socraticaPayload is the private content of an ephemeral flow. It lives only
// in the memory of one MCP instance: never in SQL, temporary files, locks,
// logs or history.
type socraticaPayload struct {
	original, proposed, approved, lastError string
	questions                               []domain.Question
	answers                                 []domain.Answer
	scrutiny                                *domain.ScrutinySynthesis
	additions                               []string
	preparation                             *domain.SpecifyPreparation
}

func clonePayloadQuestions(qs []domain.Question) []domain.Question {
	if qs == nil {
		return nil
	}
	out := make([]domain.Question, len(qs))
	for i, q := range qs {
		out[i] = q
		out[i].Options = slices.Clone(q.Options)
	}
	return out
}

func cloneScrutiny(s *domain.ScrutinySynthesis) *domain.ScrutinySynthesis {
	if s == nil {
		return nil
	}
	c := *s
	c.Assumptions = slices.Clone(s.Assumptions)
	c.Alternatives = slices.Clone(s.Alternatives)
	c.MissingEvidence = slices.Clone(s.MissingEvidence)
	c.DecisionCriteria = slices.Clone(s.DecisionCriteria)
	c.Hypotheses = slices.Clone(s.Hypotheses)
	return &c
}

func payloadOf(f domain.RefinementFlow) *socraticaPayload {
	return &socraticaPayload{
		original: f.OriginalPrompt, proposed: f.ProposedPrompt, approved: f.ApprovedPrompt, lastError: f.LastError,
		questions: clonePayloadQuestions(f.Questions), answers: slices.Clone(f.Answers),
		scrutiny: cloneScrutiny(f.Scrutiny), additions: slices.Clone(f.Additions), preparation: f.Preparation.Clone(),
	}
}

// apply copies the payload into f, so callers never alias the stored one.
func (p *socraticaPayload) apply(f *domain.RefinementFlow) {
	f.OriginalPrompt, f.ProposedPrompt, f.ApprovedPrompt, f.LastError = p.original, p.proposed, p.approved, p.lastError
	f.Questions, f.Answers = clonePayloadQuestions(p.questions), slices.Clone(p.answers)
	f.Scrutiny, f.Additions, f.Preparation = cloneScrutiny(p.scrutiny), slices.Clone(p.additions), p.preparation.Clone()
}

// stripPayload is the reservation written to SQL: identity, scope, mode,
// policy, phases, revisions, dates and dispatch marks, without any text.
func stripPayload(f domain.RefinementFlow) domain.RefinementFlow {
	f.OriginalPrompt, f.ProposedPrompt, f.ApprovedPrompt, f.LastError = "", "", "", ""
	// The provenance hash is a digest of the approved private text: not stored.
	f.ApprovedHash = ""
	f.Questions, f.Answers, f.Scrutiny, f.Additions, f.Preparation = nil, nil, nil, nil, nil
	return f
}

func isEphemeral(f domain.RefinementFlow) bool {
	return f.EffectivePayloadPolicy() == domain.PayloadEphemeral
}

// ephemeralState is shared by the root store and its transaction scopes.
type ephemeralState struct {
	mu       sync.Mutex
	payloads map[string]*socraticaPayload
}

// EphemeralSocraticaStore coordinates the SQL reservation with the in-memory
// payload. Changes are staged inside a transaction and published, under the
// mutex, only after the commit, so readers of this instance see either the
// whole change or the previous state. A rollback publishes nothing. Persistent
// flows pass through to the SQL store unchanged.
type EphemeralSocraticaStore struct {
	inner  *SocraticaStore
	state  *ephemeralState
	staged map[string]*socraticaPayload // non-nil only inside a transaction; nil value = delete
}

func NewEphemeralSocraticaStore(inner *SocraticaStore) *EphemeralSocraticaStore {
	return &EphemeralSocraticaStore{inner: inner, state: &ephemeralState{payloads: map[string]*socraticaPayload{}}}
}

var _ ports.SocraticaRepository = (*EphemeralSocraticaStore)(nil)
var _ ports.SocraticaUnitOfWork = (*EphemeralSocraticaStore)(nil)

func (w *EphemeralSocraticaStore) WithTransaction(ctx context.Context, fn func(ports.SocraticaRepository) error) error {
	if w.staged != nil {
		return fn(w)
	}
	w.state.mu.Lock()
	defer w.state.mu.Unlock()
	scoped := &EphemeralSocraticaStore{state: w.state, staged: map[string]*socraticaPayload{}}
	err := w.inner.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		scoped.inner = repo.(*SocraticaStore)
		return fn(scoped)
	})
	if err != nil {
		return err
	}
	for id, p := range scoped.staged {
		if p == nil {
			delete(w.state.payloads, id)
		} else {
			w.state.payloads[id] = p
		}
	}
	return nil
}

// guard runs fn holding the mutex, unless a transaction scope already does.
func (w *EphemeralSocraticaStore) guard(fn func() error) error {
	if w.staged == nil {
		w.state.mu.Lock()
		defer w.state.mu.Unlock()
	}
	return fn()
}

// write runs fn in a transaction scope so its payload changes are staged.
func (w *EphemeralSocraticaStore) write(ctx context.Context, fn func(*EphemeralSocraticaStore) error) error {
	if w.staged != nil {
		return fn(w)
	}
	return w.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		return fn(repo.(*EphemeralSocraticaStore))
	})
}

func (w *EphemeralSocraticaStore) lookup(id string) (*socraticaPayload, bool) {
	if p, staged := w.staged[id]; staged {
		return p, p != nil
	}
	p, ok := w.state.payloads[id]
	return p, ok
}

func (w *EphemeralSocraticaStore) stage(id string, p *socraticaPayload) { w.staged[id] = p }

// merge completes an ephemeral row with its payload or marks it unavailable.
// A cancelled flow has nothing to recover, so it is never marked, and one that
// another process cancelled drops the payload this instance still held, so the
// private texts do not outlive the cancellation. A dispatched one without its
// payload is marked: its outcome and resend need the private text.
func (w *EphemeralSocraticaStore) merge(f domain.RefinementFlow) domain.RefinementFlow {
	if !isEphemeral(f) {
		return f
	}
	if f.Phase == domain.PhaseCancelled {
		if _, ok := w.lookup(f.ID); ok {
			if w.staged != nil {
				w.stage(f.ID, nil)
			} else {
				delete(w.state.payloads, f.ID)
			}
		}
		return f
	}
	if p, ok := w.lookup(f.ID); ok {
		p.apply(&f)
	} else {
		f.PrivateStateUnavailable = true
	}
	return f
}

// requirePayload refuses to act on an ephemeral flow without its payload.
func (w *EphemeralSocraticaStore) requirePayload(ctx context.Context, id string) (*socraticaPayload, bool, error) {
	f, err := w.inner.GetFlow(ctx, id)
	if err != nil || !isEphemeral(f) {
		return nil, false, err
	}
	p, ok := w.lookup(id)
	if !ok {
		return nil, true, domain.ErrPrivateStateUnavailable
	}
	return p, true, nil
}

func (w *EphemeralSocraticaStore) CreateFlow(ctx context.Context, f domain.RefinementFlow) error {
	return w.write(ctx, func(s *EphemeralSocraticaStore) error {
		if !isEphemeral(f) {
			return s.inner.CreateFlow(ctx, f)
		}
		if err := s.inner.CreateFlow(ctx, stripPayload(f)); err != nil {
			return err
		}
		s.stage(f.ID, payloadOf(f))
		return nil
	})
}

func (w *EphemeralSocraticaStore) read(fn func() (domain.RefinementFlow, error)) (domain.RefinementFlow, error) {
	var f domain.RefinementFlow
	err := w.guard(func() error {
		var e error
		if f, e = fn(); e == nil {
			f = w.merge(f)
		}
		return e
	})
	return f, err
}

func (w *EphemeralSocraticaStore) GetFlow(ctx context.Context, id string) (domain.RefinementFlow, error) {
	return w.read(func() (domain.RefinementFlow, error) { return w.inner.GetFlow(ctx, id) })
}

func (w *EphemeralSocraticaStore) ActiveFlow(ctx context.Context, project, client string) (domain.RefinementFlow, bool, error) {
	var found bool
	f, err := w.read(func() (domain.RefinementFlow, error) {
		f, ok, e := w.inner.ActiveFlow(ctx, project, client)
		found = ok
		return f, e
	})
	return f, found, err
}

func (w *EphemeralSocraticaStore) FindByDispatchID(ctx context.Context, id string) (domain.RefinementFlow, bool, error) {
	var found bool
	f, err := w.read(func() (domain.RefinementFlow, error) {
		f, ok, e := w.inner.FindByDispatchID(ctx, id)
		found = ok
		return f, e
	})
	return f, found, err
}

// SaveTransition without the payload only admits the explicit cancellation of
// the reservation; anything else would rebuild private state from input.
func (w *EphemeralSocraticaStore) SaveTransition(ctx context.Context, before, after domain.RefinementFlow) error {
	return w.write(ctx, func(s *EphemeralSocraticaStore) error {
		if !isEphemeral(before) {
			return s.inner.SaveTransition(ctx, before, after)
		}
		if _, ok := s.lookup(before.ID); !ok && after.Phase != domain.PhaseCancelled {
			return domain.ErrPrivateStateUnavailable
		}
		if err := s.inner.SaveTransition(ctx, stripPayload(before), stripPayload(after)); err != nil {
			return err
		}
		if after.Phase == domain.PhaseCancelled {
			s.stage(before.ID, nil)
		} else {
			s.stage(before.ID, payloadOf(after))
		}
		return nil
	})
}

func (w *EphemeralSocraticaStore) Approve(ctx context.Context, id, dispatch, prompt string) (domain.RefinementFlow, bool, error) {
	var f domain.RefinementFlow
	var dup bool
	err := w.write(ctx, func(s *EphemeralSocraticaStore) error {
		p, ephemeral, e := s.requirePayload(ctx, id)
		if e != nil {
			return e
		}
		if !ephemeral {
			f, dup, e = s.inner.Approve(ctx, id, dispatch, prompt)
			return e
		}
		if f, dup, e = s.inner.approveRow(ctx, id, dispatch, "", nil); e != nil {
			return e
		}
		if !dup {
			next := *p
			next.approved = domain.ProtectText(prompt)
			s.stage(id, &next)
		}
		f = s.merge(f)
		return nil
	})
	return f, dup, err
}

func (w *EphemeralSocraticaStore) Ack(ctx context.Context, id, dispatch, owner, nonce string) (ports.AckResult, error) {
	var res ports.AckResult
	err := w.guard(func() error {
		if _, _, e := w.requirePayload(ctx, id); e != nil {
			return e
		}
		var e error
		res, e = w.inner.Ack(ctx, id, dispatch, owner, nonce)
		return e
	})
	return res, err
}

func (w *EphemeralSocraticaStore) ClaimDispatch(ctx context.Context, id, dispatch, owner string, now time.Time) (bool, error) {
	var ok bool
	err := w.guard(func() error {
		if _, _, e := w.requirePayload(ctx, id); e != nil {
			return e
		}
		var e error
		ok, e = w.inner.ClaimDispatch(ctx, id, dispatch, owner, now)
		return e
	})
	return ok, err
}

func (w *EphemeralSocraticaStore) ReleaseDispatch(ctx context.Context, id, owner string) error {
	return w.inner.ReleaseDispatch(ctx, id, owner)
}

func (w *EphemeralSocraticaStore) MarkDispatched(ctx context.Context, id, dispatch string) error {
	return w.guard(func() error {
		if _, _, e := w.requirePayload(ctx, id); e != nil {
			return e
		}
		return w.inner.MarkDispatched(ctx, id, dispatch)
	})
}

// ForceResend creates a new authorisation for the approved text kept in the
// payload; a lost private payload cannot be resent.
func (w *EphemeralSocraticaStore) ForceResend(ctx context.Context, id, oldID, newID string) error {
	return w.guard(func() error {
		p, ephemeral, e := w.requirePayload(ctx, id)
		if e != nil {
			return e
		}
		if !ephemeral {
			return w.inner.ForceResend(ctx, id, oldID, newID)
		}
		if p.approved == "" {
			return ports.ErrSocraticaNoMatch
		}
		return w.inner.forceResendRow(ctx, id, oldID, newID, "")
	})
}

func (w *EphemeralSocraticaStore) DeleteFlow(ctx context.Context, id string) error {
	return w.write(ctx, func(s *EphemeralSocraticaStore) error {
		if err := s.inner.DeleteFlow(ctx, id); err != nil {
			return err
		}
		s.stage(id, nil)
		return nil
	})
}

func (w *EphemeralSocraticaStore) RedactFlow(ctx context.Context, id string) error {
	return w.write(ctx, func(s *EphemeralSocraticaStore) error {
		if err := s.inner.RedactFlow(ctx, id); err != nil {
			return err
		}
		s.stage(id, nil)
		return nil
	})
}

func (w *EphemeralSocraticaStore) FlowExists(ctx context.Context, id string) (bool, error) {
	return w.inner.FlowExists(ctx, id)
}

func (w *EphemeralSocraticaStore) ConsumeDispatch(ctx context.Context, id, text string) (bool, error) {
	return w.inner.ConsumeDispatch(ctx, id, text)
}

// UpsertHistory never records a private specify flow, whatever the caller's
// current settings: the policy fixed at creation decides. The policy check and
// the write share one transaction, so a concurrent purge cannot slip between.
func (w *EphemeralSocraticaStore) UpsertHistory(ctx context.Context, h domain.RefinementHistory, limit int) error {
	return w.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		inner := repo.(*EphemeralSocraticaStore).inner
		exists, err := inner.FlowExists(ctx, h.FlowID)
		if err != nil {
			return err
		}
		if exists {
			f, err := inner.GetFlow(ctx, h.FlowID)
			if err != nil {
				return err
			}
			if isEphemeral(f) {
				return nil
			}
		}
		return inner.UpsertHistory(ctx, h, limit)
	})
}

func (w *EphemeralSocraticaStore) ListHistory(ctx context.Context, project, client string, limit int) ([]domain.RefinementHistory, error) {
	return w.inner.ListHistory(ctx, project, client, limit)
}

// forget drops the payloads of purged flows after their rows are gone.
func (w *EphemeralSocraticaStore) forget(ids []string, err error) ([]string, error) {
	if err != nil {
		return ids, err
	}
	_ = w.guard(func() error {
		for _, id := range ids {
			if w.staged != nil {
				w.stage(id, nil)
			} else {
				delete(w.state.payloads, id)
			}
		}
		return nil
	})
	return ids, nil
}

func (w *EphemeralSocraticaStore) PurgeStale(ctx context.Context, project string, before time.Time) ([]string, error) {
	return w.forget(w.inner.PurgeStale(ctx, project, before))
}

func (w *EphemeralSocraticaStore) PurgeTerminal(ctx context.Context, project string, before time.Time) ([]string, error) {
	return w.forget(w.inner.PurgeTerminal(ctx, project, before))
}

func (w *EphemeralSocraticaStore) PruneTerminal(ctx context.Context, project, client string, limit int) ([]string, error) {
	return w.forget(w.inner.PruneTerminal(ctx, project, client, limit))
}
