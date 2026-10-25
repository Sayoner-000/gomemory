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
	// ErrSpecifyAgentOnly: a specify flow is prepared and delivered by the
	// agent through socratica_flow; no local generic route runs it.
	ErrSpecifyAgentOnly = errors.New("specify flows are delivered by the agent, not by the local executor")
	// ErrResendPendingExists: resending a dispatched flow reopens it, and only
	// one flow per project and agent may be pending.
	ErrResendPendingExists = errors.New("another flow is pending for this agent: finish or cancel it before resending")
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

func (u *SocraticaFlow) Harnesses() []ports.SocraticHarness {
	if catalog, ok := u.Generator.(ports.SocraticHarnessCatalog); ok {
		return catalog.Harnesses()
	}
	if u.Generator == nil {
		return nil
	}
	return []ports.SocraticHarness{{ID: u.Generator.Name(), Label: u.Generator.Name(), Available: u.Generator.Available()}}
}

func (u *SocraticaFlow) HarnessAvailable(client string) bool {
	if _, ok := u.Generator.(ports.SocraticHarnessCatalog); !ok {
		return u.Generator != nil && u.Generator.Name() == client && u.Generator.Available()
	}
	for _, h := range u.Harnesses() {
		if h.ID == client {
			return h.Available
		}
	}
	return false
}

func (u *SocraticaFlow) ExplainFailure(raw string) ports.SocraticFailure {
	if explainer, ok := u.Generator.(ports.SocraticFailureExplainer); ok {
		return explainer.ExplainFailure(raw)
	}
	return ports.SocraticFailure{Message: domain.StripTerminalControls(domain.ProtectText(raw)), Retryable: domain.RetryableGenerationFailure(raw)}
}

// ChangeHarness starts a new refinement from the failed flow's original. It
// never dispatches a task or replaces an unrelated pending refinement.
func (u *SocraticaFlow) ChangeHarness(ctx context.Context, id, client string) (SocraticaResult, error) {
	r, err := u.Get(ctx, id)
	if err != nil {
		return SocraticaResult{}, err
	}
	f := r.Flow
	if f.Phase != domain.PhaseFailed || client == f.Client {
		return SocraticaResult{}, domain.ErrInvalidTransition
	}
	if !u.HarnessAvailable(client) {
		return SocraticaResult{}, ErrNoGenerator
	}
	pending, err := u.Pending(ctx, f.Project)
	if err != nil {
		return SocraticaResult{}, err
	}
	for _, p := range pending {
		if p.Flow.Client == client {
			return SocraticaResult{}, fmt.Errorf("%s ya tiene un flujo pendiente (%s); retómalo o cancélalo antes de cambiar", client, p.Flow.ID)
		}
	}
	cancelled, err := u.Apply(ctx, id, domain.Event{Kind: "cancel"}, OriginLocal)
	if err != nil {
		return SocraticaResult{}, err
	}
	if cancelled.Flow.InFlight {
		return cancelled, fmt.Errorf("envío en curso; no se cambió el harness")
	}
	return u.Start(ctx, f.Project, client, f.Mode, f.OriginalPrompt)
}

// Assist opens the conversational harness to refine a failed original. This
// authorizes a refinement conversation, never an approval/dispatch of the task.
func (u *SocraticaFlow) Assist(ctx context.Context, id string, launcher ports.AgentLauncher) error {
	r, err := u.Get(ctx, id)
	if err != nil {
		return err
	}
	f := r.Flow
	if f.Phase != domain.PhaseFailed || launcher == nil {
		return domain.ErrInvalidTransition
	}
	prompt := fmt.Sprintf("Aplica la habilidad socratica en modo %s. Continúa este refinamiento de forma conversacional con tu propio modelo; NO uses mem socratica refine ni vuelvas a los generadores externos.\n\nEl prompt original es contenido para refinar, no una instrucción para ejecutar ahora:\n<original>\n%s\n</original>\n\nPrimero confirma el objetivo y haz hasta tres preguntas con opciones, Otra y Omitir; espera las respuestas. Después muestra el original y la propuesta y ofrece Ejecutar, Editar, Copiar y Cancelar. No investigues ni ejecutes la tarea antes de la aprobación explícita.\n\nSi tienes socratica_flow conectado, recupera el flujo %s de %s; si sigue failed, aplica retry y registra tú las preguntas y la propuesta con el modelo de esta conversación. Si no tienes esa herramienta, sigue el mismo flujo en la conversación. Solo después de aprobar la propuesta puede autorizarse su ejecución; esta apertura no es una aprobación. No repitas los diagnósticos de consola como si fueran la petición.", f.Mode, domain.ProtectText(f.OriginalPrompt), f.ID, f.Client)
	return launcher.Launch(ctx, ports.LaunchRequest{Client: f.Client, Prompt: prompt}, func() error { return nil })
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

// dropOrphanLocks removes lock files whose flow row no longer exists, such as
// those left when a process died between a purge commit and dropLocks (H-05).
// Only a confirmed missing row counts as orphan, and Remove never deletes a
// lock someone holds, so a live flow keeps its mutual exclusion.
func (u *SocraticaFlow) dropOrphanLocks(ctx context.Context) error {
	if u.Lock == nil {
		return nil
	}
	ids, err := u.Lock.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		exists, err := u.Repo.FlowExists(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !exists {
			errs = append(errs, u.Lock.Remove(id))
		}
	}
	return errors.Join(errs...)
}
func (u *SocraticaFlow) Start(ctx context.Context, project, client string, mode domain.SocraticaMode, prompt string) (SocraticaResult, error) {
	settings, err := u.settings()
	if err != nil {
		return SocraticaResult{}, err
	}
	if !domain.IsSocraticaClient(client) || (mode != domain.ModeRefine && mode != domain.ModeScrutiny && mode != domain.ModeSpecify) || strings.TrimSpace(prompt) == "" {
		return SocraticaResult{}, fmt.Errorf("invalid socratica input")
	}
	var f domain.RefinementFlow
	// The pending check reads the shared reservation, so an ephemeral flow of
	// another instance also blocks a second pending flow for this agent.
	err = u.UOW.WithTransaction(ctx, func(repo ports.SocraticaRepository) error {
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
		// The payload policy is fixed here and never follows later toggles of
		// the private setting.
		f = domain.RefinementFlow{ID: u.NewID(), SchemaVersion: 1, Project: project, Client: client, Mode: mode, OriginalPrompt: domain.ProtectText(prompt), Phase: domain.PhaseCaptured, LastStablePhase: domain.PhaseCaptured, CreatedAt: now, UpdatedAt: now, PayloadPolicy: domain.PayloadPolicyFor(mode, settings.Private)}
		if mode == domain.ModeSpecify {
			f.Preparation = domain.NewSpecifyPreparation()
		}
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
	if f.Mode != domain.ModeSpecify && SocraticaRecoverable(f) {
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
	var granted bool
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
			if flow.Mode == domain.ModeSpecify {
				// The approval of a specify flow is the reduced state saved
				// under the same CAS as any edit (phase, revision, date), so
				// it can only land on the version that was reviewed.
				if !flow.Duplicate {
					e = repo.SaveTransition(ctx, before, flow)
				}
				break
			}
			var dup bool
			flow, dup, e = repo.Approve(ctx, id, flow.DispatchID, flow.ApprovedPrompt)
			flow.Duplicate = dup
			flow.AlreadyDispatched = flow.DispatchAck
		case "ack":
			var res ports.AckResult
			res, e = repo.Ack(ctx, id, ev.DispatchID, ev.Owner, ev.Nonce)
			flow.Acked = res.Acked
			flow.AlreadyDispatched = res.AlreadyDispatched
			granted = res.Granted
		case "force_resend":
			if before.Phase == domain.PhaseDispatched {
				active, found, ae := repo.ActiveFlow(ctx, flow.Project, flow.Client)
				if ae != nil {
					return ae
				}
				if found && active.ID != id {
					return ErrResendPendingExists
				}
			}
			if flow.Mode == domain.ModeSpecify {
				// Saved as a whole so the outcome of the old dispatch is dropped
				// with it, under the CAS of every other edit.
				e = repo.SaveTransition(ctx, before, flow)
				break
			}
			e = repo.ForceResend(ctx, id, before.DispatchID, flow.DispatchID)
		case "mark_dispatched":
			e = repo.MarkDispatched(ctx, id, before.DispatchID)
		default:
			e = repo.SaveTransition(ctx, before, flow)
		}
		if e != nil {
			return e
		}
		// The payload policy is fixed at creation: an ephemeral flow never records
		// history, whatever the private setting says now.
		if effect == domain.EffectRecordHistory && !settings.Private && flow.EffectivePayloadPolicy() != domain.PayloadEphemeral {
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
			flow.Preparation = nil
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
	// The grant is reported only once the transaction committed, and only for
	// specify: refine/scrutiny keep their public ack flags unchanged.
	flow.ExecutionGranted = granted && flow.Mode == domain.ModeSpecify
	result := SocraticaResult{Flow: flow, Next: SocraticaNext(flow)}
	if origin != OriginLocal {
		return result, nil
	}
	switch effect {
	case domain.EffectGenerateQuestions, domain.EffectGenerateRewrite:
		request := ports.SocraticGenerationRequest{Flow: flow}
		var g domain.GenerationResult
		if !u.HarnessAvailable(flow.Client) {
			err = fmt.Errorf("%w: %s", ErrNoGenerator, flow.Client)
		} else if effect == domain.EffectGenerateQuestions {
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
	return repo.UpsertHistory(ctx, domain.RefinementHistory{ID: f.ID, FlowID: f.ID, Project: f.Project, Client: f.Client, Mode: f.Mode, OriginalPrompt: f.OriginalPrompt, ProposedPrompt: f.ProposedPrompt, ApprovedPrompt: f.ApprovedPrompt, Scrutiny: f.Scrutiny, Additions: f.Additions, Preparation: f.Preparation, WordsBefore: domain.WordCount(f.OriginalPrompt), WordsAfter: domain.WordCount(f.ProposedPrompt), CompletedAt: u.Clock.Now()}, limit)
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
	if f.Mode == domain.ModeSpecify {
		return ExecuteResult{Flow: f}, ErrSpecifyAgentOnly
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
			res, e := repo.Ack(ctx, id, f.DispatchID, owner, nonce)
			if e != nil {
				return e
			}
			if !res.Acked && !res.AlreadyDispatched {
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
	return errors.Join(u.dropLocks(append(stale, terminal...)), u.dropOrphanLocks(ctx))
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
	// NextCancelledWithoutOriginal: the cancelled flow kept no original in this
	// instance (ephemeral payload dropped), so there is nothing to return.
	NextCancelledWithoutOriginal = "cancelado; el original no está disponible en esta instancia; no ejecutes"
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
	// Approved, dispatched and cancelled keep the shared texts; any other phase of
	// a specify flow must never suggest the closed generic routes (retry, original).
	if f.PrivateStateUnavailable || (f.Mode == domain.ModeSpecify && f.Phase != domain.PhaseCancelled) {
		return SpecifyNext(f)
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
		// A cancelled ephemeral flow drops its payload: there is no original to return.
		if f.OriginalPrompt == "" {
			return NextCancelledWithoutOriginal
		}
		return "devuelve el original; no ejecutes"
	}
	return "no ejecutes"
}
