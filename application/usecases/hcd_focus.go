package usecases

import (
	"context"
	"errors"
	"mem/application/ports"
	"mem/domain"
	"time"
)

type HCDFocus struct {
	repo                     ports.HCDRepository
	clock                    ports.ClockPort
	client, sessionID, setBy string
}

type realHCDClock struct{}

func (realHCDClock) Now() time.Time { return time.Now() }

func NewHCDFocus(repo ports.HCDRepository, clock ports.ClockPort, client, sessionID string) *HCDFocus {
	if clock == nil {
		clock = realHCDClock{}
	}
	return &HCDFocus{repo: repo, clock: clock, client: client, sessionID: sessionID, setBy: "resume"}
}

func (f *HCDFocus) Set(ctx context.Context, unitID string) error {
	u, err := f.repo.GetUnit(ctx, unitID)
	if err != nil {
		return err
	}
	if u.State == domain.StateCompleted || u.State == domain.StateCancelled || u.State == domain.StateSuperseded {
		return domain.ErrNotAllowed
	}
	now := f.clock.Now().UTC().Format(time.RFC3339Nano)
	return f.repo.SetFocus(ctx, domain.Focus{UnitID: unitID, Client: f.client, SessionID: f.sessionID, SetAt: now, SetBy: f.setBy})
}
func (f *HCDFocus) Current(ctx context.Context) (domain.Focus, error) { return f.repo.GetFocus(ctx) }
func (f *HCDFocus) Clear(ctx context.Context) error                   { return f.repo.ClearFocus(ctx) }

type HCDHeartbeatInput struct {
	Client, SessionID string
	Files             []string
}

func (h *HCD) Heartbeat(ctx context.Context, input HCDHeartbeatInput) error {
	focus, e := h.repo.GetFocus(ctx)
	if errors.Is(e, domain.ErrNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	u, e := h.repo.GetUnit(ctx, focus.UnitID)
	if e != nil {
		return e
	}
	executions, e := h.repo.ListExecutions(ctx, u.ID)
	if e != nil {
		return e
	}
	granted := false
	for _, x := range executions {
		if x.State == "granted" {
			granted = true
			break
		}
	}
	var drift []string
	if len(input.Files) > 0 && !granted {
		drift = append(drift, "files_without_execution")
	}
	if granted {
		drift = append(drift, "execution_unreported")
	}
	u.GrantedExecution = granted
	return h.repo.RecordHeartbeat(ctx, domain.Heartbeat{ID: "HB-" + h.newID(), UnitID: u.ID, SessionID: input.SessionID, Client: input.Client, Stage: string(domain.StageFor(u)), StateRevision: u.StateRevision, Files: append([]string(nil), input.Files...), Drift: drift, SeenAt: h.clock.Now().UTC().Format(time.RFC3339Nano)})
}

func (h *HCD) ConfigureSettings(s domain.HCDSettings) error {
	validated, e := domain.ValidateHCDSettings(s)
	if e == nil {
		h.hcdSettings = validated
	}
	return e
}
func (h *HCD) Drift(ctx context.Context, id string) ([]string, error) {
	return h.drift(ctx, h.repo, id)
}
func (h *HCD) drift(ctx context.Context, repo ports.HCDRepository, id string) ([]string, error) {
	events, e := repo.ListHeartbeats(ctx, id)
	if e != nil {
		return nil, e
	}
	executions, e := repo.ListExecutions(ctx, id)
	if e != nil {
		return nil, e
	}
	granted := false
	for _, x := range executions {
		granted = granted || x.State == "granted"
	}
	var out []string
	seen := map[string]bool{}
	for _, event := range events {
		for _, d := range event.Drift {
			// execution_unreported describe un estado actual: deja de valer en
			// cuanto la ejecución se informa. files_without_execution es histórico.
			if d == "execution_unreported" && !granted {
				continue
			}
			if !seen[d] {
				out = append(out, d)
				seen[d] = true
			}
		}
	}
	at, e := repo.HCDActivityAt(ctx, id)
	if e != nil {
		return nil, e
	}
	when, e := time.Parse(time.RFC3339Nano, at)
	if e != nil {
		return nil, e
	}
	days := 7
	if h.hcdSettings.InactiveDays != nil {
		days = *h.hcdSettings.InactiveDays
	}
	if h.clock.Now().Sub(when) > time.Duration(days)*24*time.Hour {
		out = append(out, "inactive_unit")
	}
	return out, nil
}
func (h *HCD) ActiveHCDUnit(ctx context.Context) (domain.Unit, error) {
	id, e := h.ActiveUnitID(ctx)
	if e != nil {
		return domain.Unit{}, e
	}
	return h.repo.GetUnit(ctx, id)
}
func (h *HCD) ActiveHCDContext(ctx context.Context) (domain.HCDActiveContext, error) {
	id, e := h.ActiveUnitID(ctx)
	if e != nil {
		return domain.HCDActiveContext{}, e
	}
	v, e := h.query(ctx, id)
	return domain.HCDActiveContext{Unit: v.Unit, Stage: v.Stage, NextAction: v.NextAction, Clarifications: v.OpenClarifications, Decisions: v.CurrentDecisions, Drift: v.Drift}, e
}
