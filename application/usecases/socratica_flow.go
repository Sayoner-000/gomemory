package usecases

import (
	"context"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"strings"
)

type SocraticaOrigin string

const (
	OriginMCP   SocraticaOrigin = "mcp"
	OriginLocal SocraticaOrigin = "local"
)

var (
	ErrDispatchUncertain   = errors.New("dispatch uncertain: explicit reconciliation required")
	ErrDispatchUnconfirmed = errors.New("agent started but dispatch could not be confirmed")
	ErrNoGenerator         = ports.ErrNoGenerator
)

type SocraticaResult struct {
	Flow domain.RefinementFlow
	Next string
}
type ExecuteResult = SocraticaResult
type SocraticaFlow struct {
	Repo          ports.SocraticaRepository
	UOW           ports.SocraticaUnitOfWork
	Lock          ports.SocraticaDispatchLock
	Generator     ports.SocraticGenerator
	Clock         ports.ClockPort
	NewID         func() string
	Settings      domain.SocraticaSettings
	SettingsError error
}

// ValidatedSettings fails fast with the load error or ErrInvalidSocraticaSettings;
// adapters use it instead of re-validating the raw settings themselves.
func (u *SocraticaFlow) ValidatedSettings() (domain.SocraticaSettings, error) {
	return u.settings()
}
func (u *SocraticaFlow) settings() (domain.SocraticaSettings, error) {
	if u.SettingsError != nil {
		return domain.SocraticaSettings{}, u.SettingsError
	}
	return domain.ValidateSocraticaSettings(u.Settings)
}

// dropLocks removes the lock files of purged flows after their rows are gone.
func (u *SocraticaFlow) dropLocks(ids []string) error {
	if u.Lock == nil {
		return nil
	}
	var errs []error
	for _, id := range ids {
		errs = append(errs, u.Lock.Remove(id))
	}
	return errors.Join(errs...)
}
func (u *SocraticaFlow) Start(ctx context.Context, project, client string, mode domain.SocraticaMode, prompt string) (SocraticaResult, error) {
	if _, err := u.settings(); err != nil {
		return SocraticaResult{}, err
	}
	if !domain.IsSocraticaClient(client) || (mode != domain.ModeRefine && mode != domain.ModeScrutiny) || strings.TrimSpace(prompt) == "" {
		return SocraticaResult{}, fmt.Errorf("invalid socratica input")
	}
	var f domain.RefinementFlow
	err := u.UOW.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		active, found, err := repo.ActiveFlow(ctx, project, client)
		if err != nil {
			return err
		}
		if found {
			f = active
			f.Recovered = true
			if f.Phase == domain.PhaseApproved && f.DispatchClaimOwner != "" {
				f.Uncertain = true
			}
			return nil
		}
		now := u.Clock.Now()
		f = domain.RefinementFlow{ID: u.NewID(), SchemaVersion: 1, Project: project, Client: client, Mode: mode, OriginalPrompt: domain.ProtectText(prompt), Phase: domain.PhaseCaptured, LastStablePhase: domain.PhaseCaptured, CreatedAt: now, UpdatedAt: now}
		return repo.CreateFlow(ctx, f)
	})
	return SocraticaResult{Flow: f, Next: SocraticaNext(f)}, err
}
func (u *SocraticaFlow) Get(ctx context.Context, id string) (SocraticaResult, error) {
	if _, err := u.settings(); err != nil {
		return SocraticaResult{}, err
	}
	f, err := u.Repo.GetFlow(ctx, id)
	if err != nil {
		return SocraticaResult{}, err
	}
	if f.Phase == domain.PhaseApproved && f.DispatchClaimOwner != "" {
		release, ok, e := u.Lock.TryAcquire(id)
		if e != nil {
			return SocraticaResult{}, e
		}
		f.InFlight = !ok
		f.Uncertain = ok
		if ok {
			release()
		}
	}
	f.AlreadyDispatched = f.DispatchAck
	if SocraticaRecoverable(f) {
		return SocraticaResult{Flow: f, Next: NextRecoverable}, nil
	}
	return SocraticaResult{Flow: f, Next: SocraticaNext(f)}, nil
}
func (u *SocraticaFlow) Resume(ctx context.Context, id string, origin SocraticaOrigin) (SocraticaResult, error) {
	return u.Apply(ctx, id, domain.Event{Kind: "resume"}, origin)
}
func (u *SocraticaFlow) Apply(ctx context.Context, id string, ev domain.Event, origin SocraticaOrigin) (SocraticaResult, error) {
	settings, err := u.settings()
	if err != nil {
		return SocraticaResult{}, err
	}
	ev.Private = settings.Private
	// Hold the same OS lock used by Execute while reconciling a dispatch.
	switch ev.Kind {
	case "resume", "force_resend", "mark_dispatched", "cancel":
		release, ok, e := u.Lock.TryAcquire(id)
		if e != nil {
			return SocraticaResult{}, e
		}
		if !ok {
			return u.inFlight(ctx, id)
		}
		defer release()
	}
	var flow domain.RefinementFlow
	var effect domain.Effect
	var purged []string
	err = u.UOW.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
		before, e := repo.GetFlow(ctx, id)
		if e != nil {
			return e
		}
		flow, effect, e = domain.Reduce(before, ev, u.Clock.Now(), u.NewID)
		if e != nil {
			if ev.Kind == "ack" {
				return errors.Join(ports.ErrAckRejected, e)
			}
			return e
		}
		switch ev.Kind {
		case "approve", "send_original":
			var dup bool
			flow, dup, e = repo.Approve(ctx, id, flow.DispatchID, flow.ApprovedPrompt)
			flow.Duplicate = dup
			flow.AlreadyDispatched = flow.DispatchAck
		case "ack":
			var acked, already bool
			acked, already, e = repo.Ack(ctx, id, ev.DispatchID, ev.Owner, ev.Nonce)
			flow.Acked = acked
			flow.AlreadyDispatched = already
		case "force_resend":
			e = repo.ForceResend(ctx, id, before.DispatchID, flow.DispatchID)
		case "mark_dispatched":
			e = repo.MarkDispatched(ctx, id, before.DispatchID)
		default:
			e = repo.SaveTransition(ctx, before, flow)
		}
		if e != nil {
			return e
		}
		if effect == domain.EffectRecordHistory && !settings.Private {
			if e = u.record(ctx, repo, flow, *settings.HistoryLimit); e != nil {
				return e
			}
		}
		if effect == domain.EffectDeleteFlow {
			purged = append(purged, id)
			return repo.DeleteFlow(ctx, id)
		}
		if flow.Phase.Terminal() {
			if purged, e = repo.PruneTerminal(ctx, flow.Project, flow.Client, *settings.HistoryLimit); e != nil {
				return e
			}
		}
		if effect == domain.EffectRedactFlow {
			if e = repo.RedactFlow(ctx, id); e != nil {
				return e
			}
			flow.OriginalPrompt = ""
			flow.ProposedPrompt = ""
			flow.ApprovedPrompt = ""
			flow.Questions = nil
			flow.Answers = nil
			flow.Scrutiny = nil
			flow.LastError = ""
		}
		return nil
	})
	if err != nil {
		return SocraticaResult{}, err
	}
	if err = u.dropLocks(purged); err != nil {
		return SocraticaResult{}, err
	}
	result := SocraticaResult{Flow: flow, Next: SocraticaNext(flow)}
	if origin != OriginLocal {
		return result, nil
	}
	switch effect {
	case domain.EffectGenerateQuestions, domain.EffectGenerateRewrite:
		if u.Generator == nil || !u.Generator.Available() {
			return result, ErrNoGenerator
		}
		request := ports.SocraticGenerationRequest{Flow: flow}
		var g domain.GenerationResult
		if effect == domain.EffectGenerateQuestions {
			g, err = u.Generator.GenerateQuestions(ctx, request)
		} else {
			g, err = u.Generator.Rewrite(ctx, request)
		}
		if err != nil {
			_, failErr := u.Apply(ctx, id, domain.Event{Kind: "model_error", Error: domain.ProtectText(err.Error())}, OriginMCP)
			return result, errors.Join(err, failErr)
		}
		if effect == domain.EffectGenerateQuestions {
			return u.Apply(ctx, id, domain.Event{Kind: "questions_generated", Questions: g.Questions}, origin)
		}
		return u.Apply(ctx, id, domain.Event{Kind: "rewrite_ready", Proposed: g.Proposed, Scrutiny: g.Scrutiny, Additions: g.Additions}, origin)
	}
	return result, nil
}
func (u *SocraticaFlow) record(ctx context.Context, repo ports.SocraticaRepository, f domain.RefinementFlow, limit int) error {
	return repo.UpsertHistory(ctx, domain.RefinementHistory{ID: f.ID, FlowID: f.ID, Project: f.Project, Client: f.Client, Mode: f.Mode, OriginalPrompt: f.OriginalPrompt, ProposedPrompt: f.ProposedPrompt, ApprovedPrompt: f.ApprovedPrompt, Scrutiny: f.Scrutiny, Additions: f.Additions, WordsBefore: domain.WordCount(f.OriginalPrompt), WordsAfter: domain.WordCount(f.ProposedPrompt), CompletedAt: u.Clock.Now()}, limit)
}
func (u *SocraticaFlow) Execute(ctx context.Context, id, target string, launcher ports.AgentLauncher) (ExecuteResult, error) {
	settings, err := u.settings()
	if err != nil {
		return ExecuteResult{}, err
	}
	release, ok, err := u.Lock.TryAcquire(id)
	if err != nil {
		return ExecuteResult{}, err
	}
	if !ok {
		return u.inFlight(ctx, id)
	}
	defer release()
	f, err := u.Repo.GetFlow(ctx, id)
	if err != nil {
		return ExecuteResult{}, err
	}
	if f.DispatchAck {
		f.AlreadyDispatched = true
		return ExecuteResult{Flow: f, Next: SocraticaNext(f)}, nil
	}
	if f.DispatchClaimOwner != "" {
		return ExecuteResult{Flow: f}, ErrDispatchUncertain
	}
	if target != f.Client {
		return ExecuteResult{}, fmt.Errorf("dispatch client differs from flow client")
	}
	if f.Phase == domain.PhaseReview || f.Phase == domain.PhaseFailed {
		kind := "approve"
		if f.Phase == domain.PhaseFailed {
			kind = "send_original"
		}
		r, e := u.Apply(ctx, id, domain.Event{Kind: kind}, OriginMCP)
		if e != nil {
			return r, e
		}
		f = r.Flow
	}
	if f.Phase != domain.PhaseApproved {
		return ExecuteResult{Flow: f}, domain.ErrInvalidTransition
	}
	owner := u.NewID()
	claimed, err := u.Repo.ClaimDispatch(ctx, id, f.DispatchID, owner, u.Clock.Now())
	if err != nil {
		return ExecuteResult{}, err
	}
	if !claimed {
		return ExecuteResult{Flow: f}, ErrDispatchUncertain
	}
	nonce := u.NewID()
	var purged []string
	confirm := func() error {
		return u.UOW.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
			acked, already, e := repo.Ack(ctx, id, f.DispatchID, owner, nonce)
			if e != nil {
				return e
			}
			if !acked && !already {
				return ErrDispatchUnconfirmed
			}
			if purged, e = repo.PruneTerminal(ctx, f.Project, f.Client, *settings.HistoryLimit); e != nil {
				return e
			}
			if settings.Private {
				return repo.RedactFlow(ctx, id)
			}
			return nil
		})
	}
	err = launcher.Launch(ctx, ports.LaunchRequest{Client: target, Prompt: f.ApprovedPrompt, Env: map[string]string{"GOMEMORY_SOCRATICA_DISPATCH": f.DispatchID}}, confirm)
	if errors.Is(err, ports.ErrLaunchFailed) {
		return ExecuteResult{Flow: f}, errors.Join(err, u.Repo.ReleaseDispatch(ctx, id, owner))
	}
	if errors.Is(err, ports.ErrConfirmFailed) {
		if retryErr := confirm(); retryErr != nil {
			return ExecuteResult{Flow: f}, errors.Join(ErrDispatchUnconfirmed, err, retryErr)
		}
		err = nil
	}
	current, getErr := u.Repo.GetFlow(ctx, id)
	return ExecuteResult{Flow: current, Next: SocraticaNext(current)}, errors.Join(err, getErr, u.dropLocks(purged))
}

// inFlight reports a flow whose lock another process holds, without changes.
func (u *SocraticaFlow) inFlight(ctx context.Context, id string) (SocraticaResult, error) {
	f, err := u.Repo.GetFlow(ctx, id)
	if err != nil {
		return SocraticaResult{}, err
	}
	f.InFlight = true
	return SocraticaResult{Flow: f, Next: SocraticaNext(f)}, nil
}
func (u *SocraticaFlow) History(ctx context.Context, project, client string) ([]domain.RefinementHistory, error) {
	s, e := u.settings()
	if e != nil {
		return nil, e
	}
	return u.Repo.ListHistory(ctx, project, client, *s.HistoryLimit)
}

// Pending returns the non-terminal flow of each agent (one at most), with the
// dispatch state a reader needs (uncertain / in flight).
func (u *SocraticaFlow) Pending(ctx context.Context, project string) ([]SocraticaResult, error) {
	if _, err := u.settings(); err != nil {
		return nil, err
	}
	var out []SocraticaResult
	for _, client := range []string{"claude", "codex", "opencode"} {
		f, found, err := u.Repo.ActiveFlow(ctx, project, client)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		r, err := u.Get(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (u *SocraticaFlow) PurgeStale(ctx context.Context, project string) error {
	if _, e := u.settings(); e != nil {
		return e
	}
	stale, e := u.Repo.PurgeStale(ctx, project, u.Clock.Now().Add(-domain.PendingMaxAge))
	if e != nil {
		return e
	}
	terminal, e := u.Repo.PurgeTerminal(ctx, project, u.Clock.Now().Add(-domain.TerminalRetention))
	if e != nil {
		return e
	}
	return u.dropLocks(append(stale, terminal...))
}

// PromptClassification is what the prompt hooks need from Socratica for one
// turn. SkipProvenance keeps Socratica prompts and answers out of memory origin
// (FR-026); Capture/Forward/Directive belong to automatic interception.
type PromptClassification struct {
	SkipProvenance, Capture, Forwarded bool
	Forward, Directive                 string
}

// PromptSignals are what a hook knows about the prompt. DispatchID is the
// GOMEMORY_SOCRATICA_DISPATCH of the agent's environment, consumed once.
type PromptSignals struct {
	Text              string
	Attachments, Busy bool
	DispatchID        string
}

// ClassifyPrompt is the single decision of the prompt hooks; they hold no
// rules (I1). An explicit invocation is recognised without the database.
// Otherwise one ActiveFlow read tells whether the prompt answers a pending
// flow (provenance, FR-026). Only with interception enabled for this agent
// (capability, or unverified opt-in) is the dispatch mark consumed and the
// capture decided. Errors are for stderr: they never block the turn.
func (u *SocraticaFlow) ClassifyPrompt(ctx context.Context, project, client string, sig PromptSignals) (PromptClassification, error) {
	c := PromptClassification{SkipProvenance: domain.IsSocraticaInvocation(sig.Text)}
	if c.SkipProvenance || strings.TrimSpace(sig.Text) == "" {
		return c, nil
	}
	settings, err := u.settings()
	if err != nil {
		return c, err
	}
	f, found, err := u.Repo.ActiveFlow(ctx, project, client)
	if err != nil {
		return c, err
	}
	now := u.Clock.Now()
	if found {
		pending := f.Phase != domain.PhaseApproved && !f.Phase.Terminal()
		c.SkipProvenance = pending && now.Sub(f.UpdatedAt) <= domain.ContinuationWindow
	}
	enabled := settings.Interception[client] && (domain.CapabilitiesForSocratica(client).Interception || settings.InterceptionUnverified[client])
	if !enabled {
		return c, nil
	}
	in := domain.InterceptionInput{Text: sig.Text, Enabled: true, Attachments: sig.Attachments, Busy: sig.Busy, Now: now}
	if found {
		in.PendingPhase, in.PendingUpdatedAt = f.Phase, f.UpdatedAt
	}
	if sig.DispatchID != "" {
		matched, err := u.Repo.ConsumeDispatch(ctx, sig.DispatchID, sig.Text)
		if err != nil {
			return c, err
		}
		if matched {
			in.Origin = domain.OriginSocraticaApproved
		}
	}
	d := domain.DecideInterception(in)
	c.Capture, c.Forwarded, c.Forward, c.Directive = d.Capture, d.Forwarded, d.Forward, d.Directive
	c.SkipProvenance = c.SkipProvenance || d.Capture
	return c, nil
}

// Next texts per dispatch state (contracts/mcp-tool.md, I11); get, resume and
// approve share them so the agent never sees two instructions for one state.
const (
	NextRecoverable = "ofrece Ejecutar (approve → ack → ejecutar) o Cancelar; cancelar libera el pendiente"
	NextInFlight    = "NO ejecutes ni ofrezcas acciones: otro proceso está despachando; vuelve a consultar con get"
	NextUncertain   = "NO ejecutes: puede que ya se haya enviado; pregunta a la persona: marcar como enviado, reenviar o cancelar"
	NextConsumed    = "NO ejecutes: ya fue enviado; si la persona confirma que la tarea no empezó, ofrece reenviar (force_resend)"
	NextReadyToAck  = "llama a ack con este dispatch_id y un ack_nonce antes de ejecutar"
	NextExecute     = "ejecuta exactamente el prompt aprobado una sola vez"
)

// SocraticaRecoverable: approved but neither acked nor claimed, so nothing ran.
func SocraticaRecoverable(f domain.RefinementFlow) bool {
	return f.Phase == domain.PhaseApproved && !f.DispatchAck && f.DispatchClaimOwner == ""
}
func SocraticaNext(f domain.RefinementFlow) string {
	if f.InFlight {
		return NextInFlight
	}
	if f.Uncertain {
		return NextUncertain
	}
	switch f.Phase {
	case domain.PhaseCaptured:
		return "genera entre cero y tres preguntas; no ejecutes la tarea"
	case domain.PhaseQuestions:
		return "pregunta con opciones Otra, Omitir y Omitir el resto; no ejecutes"
	case domain.PhaseRewriting:
		return "genera una propuesta fiel con marcadores y adiciones; no ejecutes"
	case domain.PhaseReview:
		if f.Recovered {
			return NextRecoverable
		}
		return "muestra original y propuesta; ofrece Ejecutar, Editar, Copiar y Cancelar; espera aprobación"
	case domain.PhaseApproved:
		return NextReadyToAck
	case domain.PhaseDispatched:
		if f.Acked {
			return NextExecute
		}
		return NextConsumed
	case domain.PhaseFailed:
		return "conserva el original; ofrece Reintentar, Enviar mi original o Cancelar"
	case domain.PhaseCancelled:
		return "devuelve el original; no ejecutes"
	}
	return "no ejecutes"
}
