package usecases

import (
	"context"
	"errors"
	"mem/domain"
	"os"
	"strings"
)

type HCDView struct {
	DecisionConflicts  []HCDDecisionConflict
	CurrentDecisions   []domain.HCDDecision
	Drift              []string
	Unit               domain.Unit            `json:"unit"`
	Stage              domain.Stage           `json:"stage"`
	AllowedActions     []string               `json:"allowed_actions"`
	PendingChecks      []domain.PendingCheck  `json:"pending_checks"`
	OpenClarifications []domain.Clarification `json:"open_clarifications"`
	Decisions          []domain.HCDDecision   `json:"decisions"`
	NextAction         domain.NextStep        `json:"next_action"`
}

// Show returns the current control view without changing state_revision.
func (h *HCD) Show(ctx context.Context, unitID string) (HCDView, error) { return h.query(ctx, unitID) }
func (h *HCD) Next(ctx context.Context, unitID string) (HCDView, error) { return h.query(ctx, unitID) }

// Resume selects the unit as focus, a project-level pointer that is deliberately
// independent of the unit's optimistic state revision.
func (h *HCD) Resume(ctx context.Context, unitID, client, sessionID string) (HCDView, error) {
	u, e := h.repo.GetUnit(ctx, unitID)
	if e != nil {
		return HCDView{}, e
	}
	if u.State == domain.StateCompleted || u.State == domain.StateCancelled || u.State == domain.StateSuperseded {
		return h.query(ctx, unitID)
	}
	focus := NewHCDFocus(h.repo, h.clock, client, sessionID)
	if err := focus.Set(ctx, unitID); err != nil {
		return HCDView{}, err
	}
	return h.query(ctx, unitID)
}

func (h *HCD) query(ctx context.Context, unitID string) (HCDView, error) {
	u, err := h.repo.GetUnit(ctx, unitID)
	if err != nil {
		return HCDView{}, err
	}
	clarifications, err := h.repo.ListOpenClarifications(ctx, unitID)
	if err != nil {
		return HCDView{}, err
	}
	checks := make([]domain.PendingCheck, 0)
	var definition domain.Definition
	if h.parseDefinition != nil && h.artifacts != nil {
		dir := strings.TrimPrefix(u.ID, "HCD-") + "-" + u.Slug
		data, e := h.artifacts.Read(ctx, dir, "definition.md")
		// Sin definition.md la consulta muestra la definición vacía y sus pendientes.
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return HCDView{}, e
		}
		definition, e = h.parseDefinition(data)
		if e != nil {
			return HCDView{}, e
		}
		checks = append(checks, domain.ValidateDefinition(definition, u.Kind)...)
		u.IntentComplete = strings.TrimSpace(definition.Problem) != "" && strings.TrimSpace(definition.Actor) != "" && strings.TrimSpace(definition.ExpectedResult) != ""

	}
	for _, c := range clarifications {
		if c.Blocking {
			checks = append(checks, domain.PendingCheck{Code: "CLARIFICATION-OPEN", Field: c.TargetRef, Message: "aclaración bloqueante abierta: " + c.ID})
		}
	}
	u.OpenClarification = len(clarifications) > 0
	u.ValidationPassed = len(checks) == 0 && u.DefinitionRev != "" && u.ApprovedRev == ""
	if err := h.deriveDeliveryFacts(ctx, &u); err != nil {
		return HCDView{}, err
	}
	decisions, err := h.repo.ListDecisions(ctx, unitID)
	if err != nil {
		return HCDView{}, err
	}
	drift, err := h.Drift(ctx, unitID)
	if err != nil {
		return HCDView{}, err
	}
	conflicts, err := h.decisionConflicts(decisions)
	if err != nil {
		return HCDView{}, err
	}
	for _, conflict := range conflicts {
		checks = append(checks, domain.PendingCheck{Code: "DECISION-CONFLICT", Field: conflict.A.ID + ":" + conflict.B.ID, Message: "revisar decisiones con sus fuentes y revisiones; " + conflict.SuggestedAction})
	}
	var currentDecisions []domain.HCDDecision
	for _, d := range decisions {
		if d.Revision == u.DefinitionRev {
			currentDecisions = append(currentDecisions, d)
		}
	}
	return HCDView{DecisionConflicts: conflicts, CurrentDecisions: currentDecisions, Drift: drift, Unit: u, Stage: domain.StageFor(u), AllowedActions: domain.AllowedActions(u), PendingChecks: checks, OpenClarifications: clarifications, Decisions: decisions, NextAction: domain.NextAction(u, checks)}, nil
}

// deriveDeliveryFacts rellena los hechos de entrega que StageFor necesita para
// pasar de slicing a decomposition, execution o verification. Sin ellos la
// etapa de una unidad ready o in_progress se quedaba siempre en slicing.
func (h *HCD) deriveDeliveryFacts(ctx context.Context, u *domain.Unit) error {
	if (u.State != domain.StateReady && u.State != domain.StateInProgress) || h.parseDelivery == nil || h.artifacts == nil {
		return nil
	}
	d, err := h.readDelivery(ctx, *u)
	if err != nil {
		return err
	}
	decisions, err := h.repo.ListDecisions(ctx, u.ID)
	if err != nil {
		return err
	}
	d.Tasks = domain.ActiveTasks(d.Tasks, decisions)
	withTasks := map[string]bool{}
	for _, t := range d.Tasks {
		withTasks[t.SliceID] = true
	}
	for _, s := range d.Slices {
		ready := len(domain.SliceViolations(s)) == 0
		u.ReadySlice = u.ReadySlice || ready
		// Un slice listo sin tareas todavía pide descomposición.
		u.UnadmittedTask = u.UnadmittedTask || (ready && !withTasks[s.ID])
	}
	executions, err := h.repo.ListExecutions(ctx, u.ID)
	if err != nil {
		return err
	}
	reported := map[string]bool{}
	for _, x := range executions {
		reported[x.TaskID] = reported[x.TaskID] || x.State == "reported"
		u.GrantedExecution = u.GrantedExecution || x.State == "granted"
	}
	for _, t := range d.Tasks {
		if reported[t.ID] {
			continue
		}
		a, err := h.check(ctx, h.repo, *u, t.ID)
		// Una edición externa de delivery.md la reconcilia la siguiente mutación;
		// la consulta no falla por ello (FR-046).
		if errors.Is(err, domain.ErrRevisionConflict) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(a.Checks) == 0 {
			u.AdmittedTask = true
		} else {
			u.UnadmittedTask = true
		}
	}
	return nil
}

func (h *HCD) ActiveUnitID(ctx context.Context) (string, error) {
	f, err := h.repo.GetFocus(ctx)
	return f.UnitID, err
}
func (h *HCD) Focus(ctx context.Context, id, client, session string, clear bool) (domain.Focus, error) {
	f := NewHCDFocus(h.repo, h.clock, client, session)
	if clear {
		return domain.Focus{}, f.Clear(ctx)
	}
	if id != "" {
		if err := f.Set(ctx, id); err != nil {
			return domain.Focus{}, err
		}
	}
	return f.Current(ctx)
}
func (h *HCD) ParseDefinition(data []byte) (domain.Definition, error) {
	if h.parseDefinition == nil {
		return domain.Definition{}, domain.ErrNotAllowed
	}
	return h.parseDefinition(data)
}
func (h *HCD) ReadDefinition(ctx context.Context, id string) (domain.Definition, error) {
	u, err := h.repo.GetUnit(ctx, id)
	if err != nil {
		return domain.Definition{}, err
	}
	data, err := h.artifacts.Read(ctx, strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "definition.md")
	if err != nil {
		return domain.Definition{}, err
	}
	return h.ParseDefinition(data)
}
