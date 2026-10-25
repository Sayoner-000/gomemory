package usecases

import (
	"context"
	"fmt"

	"mem/domain"
)

// PrepareSet replaces one section of a specify flow (prepare/set). The value is
// parsed strictly, so computed data sent by the agent is refused, and protected
// before it is saved, checked or rendered. Everything runs inside Apply's
// UnitOfWork with the expected revision, and no generator is involved.
func (u *SocraticaFlow) PrepareSet(ctx context.Context, id string, expectedRevision int, section domain.SpecifyStage, raw []byte) (SocraticaResult, error) {
	content, err := domain.ParseSpecifySection(section, raw)
	if err != nil {
		return SocraticaResult{}, err
	}
	if content, err = domain.ProtectSpecifyContent(content); err != nil {
		return SocraticaResult{}, err
	}
	return u.Apply(ctx, id, domain.Event{Kind: "prepare_set", Section: section, ExpectedRevision: expectedRevision, Content: content}, OriginMCP)
}

// PrepareValidate checks one stage (prepare/validate). Gomemory computes the
// result from the current content: a failed check is a normal result carrying
// the missing items, not an error. The final pass leaves the flow in review,
// which is not an approval.
func (u *SocraticaFlow) PrepareValidate(ctx context.Context, id string, expectedRevision int, stage domain.SpecifyStage) (SocraticaResult, error) {
	return u.Apply(ctx, id, domain.Event{Kind: "prepare_validate", Section: stage, ExpectedRevision: expectedRevision}, OriginMCP)
}

// ApproveSpecify approves the reviewed version of a specify flow. The person
// approves what was shown, so the caller names the revision it showed; the
// reducer checks it against the stored content, checks and text inside the
// same UnitOfWork. Approving never grants execution.
func (u *SocraticaFlow) ApproveSpecify(ctx context.Context, id string, expectedRevision int) (SocraticaResult, error) {
	return u.Apply(ctx, id, domain.Event{Kind: "approve", ExpectedRevision: expectedRevision}, OriginMCP)
}

// ReportOutcome records the result the agent communicates for the consumed
// dispatch (report). It is information: it never grants nor re-opens an
// authorization, and the backend checks format and references, not the
// generated document.
func (u *SocraticaFlow) ReportOutcome(ctx context.Context, id, dispatchID string, outcome domain.SpecifyOutcome, correctPrevious bool) (SocraticaResult, error) {
	return u.Apply(ctx, id, domain.Event{Kind: "report", DispatchID: dispatchID, Outcome: outcome, CorrectPrevious: correctPrevious}, OriginMCP)
}

// SpecifyViewData is what gomemory computes for the agent about a specify flow.
// The agent follows it; it never supplies these values.
type SpecifyViewData struct {
	Stage          domain.SpecifyStage
	PendingChecks  []domain.SpecifyIssue
	AllowedActions []string
	QuestionsUsed  int
	ReviewHash     string
}

// SpecifyView computes stage, missing sections and allowed actions. Until the
// stage checks exist, pending checks only name the sections still missing.
func SpecifyView(f domain.RefinementFlow) SpecifyViewData {
	v := SpecifyViewData{QuestionsUsed: len(f.Questions), PendingChecks: []domain.SpecifyIssue{}, AllowedActions: specifyAllowedActions(f)}
	p := f.Preparation
	if p == nil {
		return v
	}
	// Past review the stage follows the phase, which is what the SQL-only ack
	// and mark_dispatched transitions do persist.
	v.Stage = p.Stage
	switch f.Phase {
	case domain.PhaseReview:
		v.Stage = domain.StageReview
	case domain.PhaseApproved:
		v.Stage = domain.StageApproved
	case domain.PhaseDispatched:
		v.Stage = domain.StageDelivery
	}
	if p.Review != nil {
		v.ReviewHash = p.Review.PromptHash
	}
	if f.PrivateStateUnavailable {
		return v
	}
	v.PendingChecks = pendingChecks(p)
	return v
}

// pendingChecks lists what still blocks the current stage, computed from the
// current content (never from a stored result that could be stale), followed by
// the sections not provided yet so the agent sees the whole road ahead.
func pendingChecks(p *domain.SpecifyPreparation) []domain.SpecifyIssue {
	issues := []domain.SpecifyIssue{}
	seen := map[string]bool{}
	add := func(list []domain.SpecifyIssue) {
		for _, i := range list {
			if key := i.Code + "|" + i.Path; !seen[key] {
				seen[key] = true
				issues = append(issues, i)
			}
		}
	}
	switch p.Stage {
	case domain.StageIntent, domain.StageScope, domain.StageStories, domain.StageValidation:
		add(domain.ValidateSpecifyStage(p.Content, p.Stage))
	}
	for _, s := range []struct {
		section domain.SpecifyStage
		missing bool
	}{{domain.StageIntent, p.Content.Intent == nil}, {domain.StageScope, p.Content.Scope == nil}, {domain.StageStories, p.Content.Stories == nil}} {
		if s.missing {
			add([]domain.SpecifyIssue{{Code: domain.IssueSectionMissing, Path: string(s.section), Message: fmt.Sprintf("falta la sección %s", s.section)}})
		}
	}
	return issues
}

func specifyAllowedActions(f domain.RefinementFlow) []string {
	switch {
	case f.PrivateStateUnavailable:
		return []string{"get", "cancel"}
	case f.Phase == domain.PhaseDispatched:
		// Resending needs the person's explicit choice and a new authorisation.
		return []string{"get", "report", "force_resend"}
	case f.Phase.Terminal():
		return []string{"get"}
	case f.Phase == domain.PhaseApproved && f.DispatchClaimOwner != "":
		return []string{"get", "force_resend", "mark_dispatched", "cancel"}
	}
	switch f.Phase {
	case domain.PhaseCaptured:
		actions := []string{"get", "resume", "prepare"}
		if len(f.Questions) < domain.MaxSocraticaQuestions && len(f.Answers) == len(f.Questions) {
			actions = append(actions, "answer")
		}
		return append(actions, "cancel")
	case domain.PhaseQuestions:
		return []string{"get", "resume", "prepare", "answer", "cancel"}
	case domain.PhaseReview:
		return []string{"get", "resume", "prepare", "approve", "cancel"}
	case domain.PhaseApproved:
		return []string{"get", "resume", "prepare", "ack", "cancel"}
	}
	return []string{"get", "cancel"}
}

// Next texts of delivery. The grant is only in the response to the first
// committed ack; every other state tells the agent not to execute.
const (
	nextSpecifyPreflight = "antes de confirmar comprueba que puedes cargar e invocar la skill speckit-specify en este proyecto (un binario en PATH no basta); si no puedes, conserva la entrada aprobada, muestra la dependencia y NO llames a ack; si puedes, llama a ack con dispatch_id y un ack_nonce nuevo y conserva ese nonce para reintentar si se pierde la respuesta"
	nextSpecifyGranted   = "ejecución concedida UNA sola vez: invoca la skill speckit-specify con exactamente approved_prompt como argumento, sin reescribirlo ni construir un shell con él, y no implementes la necesidad descrita; después abre el spec.md y el checklist reales, comprueba alcance y trazabilidad de cada criterio y comunica el resultado con report (dispatch_id y outcome); un fallo se comunica como failed o uncertain"
	nextSpecifyReplay    = "NO ejecutes desde este replay: la concesión ya se entregó una vez; consulta con get si hay resultado; si no hay evidencia de que se entregó, pregunta a la persona si quiere reenviar (force_resend) o dejarlo como está"
	nextSpecifyConsumed  = "NO ejecutes: la autorización ya se consumió; si ya ejecutaste y tienes el resultado, comunícalo con report; si la persona confirma que no empezó, ofrece reenviar (force_resend)"
)

// SpecifyNext is the single instruction for each state of a specify flow.
func SpecifyNext(f domain.RefinementFlow) string {
	if f.PrivateStateUnavailable {
		return "esta instancia no tiene el contenido privado: solo consulta con get o cancela; no reconstruyas ni ejecutes"
	}
	switch f.Phase {
	case domain.PhaseApproved:
		return nextSpecifyPreflight
	case domain.PhaseDispatched:
		switch {
		case f.ExecutionGranted:
			return nextSpecifyGranted
		case f.Acked:
			return nextSpecifyReplay
		case f.Preparation != nil && f.Preparation.Outcome != nil:
			return fmt.Sprintf("resultado comunicado por el agente (%s): infórmalo a la persona como tal, sin presentarlo como verificado por gomemory; no ejecutes", f.Preparation.Outcome.Status)
		}
		return nextSpecifyConsumed
	case domain.PhaseCaptured:
		stage := domain.StageIntent
		if f.Preparation != nil {
			stage = f.Preparation.Stage
		}
		budget := fmt.Sprintf("hasta %d preguntas en total (usadas %d)", domain.MaxSocraticaQuestions, len(f.Questions))
		if f.Preparation != nil && len(domain.ValidateSpecifyStage(f.Preparation.Content, stage)) == 0 {
			return fmt.Sprintf("comprueba la etapa %s con prepare(validate, stage=%s, expected_revision=%d); %s; no ejecutes la tarea ni generes la especificación", stage, stage, f.Revision, budget)
		}
		return fmt.Sprintf("recupera contexto del proyecto antes de preguntar; completa lo que indica pending_checks de la etapa %s con prepare(set) y expected_revision=%d; %s; no ejecutes la tarea ni generes la especificación", stage, f.Revision, budget)
	case domain.PhaseQuestions:
		return "pregunta con opciones Otra, Omitir y Omitir el resto; al terminar vuelve a preparar secciones; no ejecutes"
	case domain.PhaseReview:
		return fmt.Sprintf("la preparación está comprobada: muestra el original y la entrada final completa (proposed, review_hash) con sus supuestos y pendientes, y aclara que las comprobaciones cubren completitud, relaciones y bloqueos mientras la persona confirma que refleja su necesidad; ofrece «Aprobar y generar especificación», «Editar», «Copiar» (texto seleccionable si no hay portapapeles) y «Cancelar»; aprueba con approve y expected_revision=%d solo si la persona elige aprobar esa versión; no ejecutes", f.Revision)
	}
	return "no ejecutes"
}
