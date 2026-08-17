package usecases

import (
	"testing"

	"mem/domain"
)

type fakeInspector struct {
	channels []domain.ActivationChannel
}

func (f *fakeInspector) Inspect(root string) []domain.ActivationChannel { return f.channels }

func TestBuildActivationReport_ProblemsExcludesDegradations(t *testing.T) {
	inspector := &fakeInspector{channels: []domain.ActivationChannel{
		{Agent: "claude", Kind: domain.KindPlanGuard, Scope: domain.ScopeProject, State: domain.StateOK},
		{Agent: "opencode", Kind: domain.KindPlanGuard, Scope: domain.ScopeProject, State: domain.StateNotApplicable, Detail: "sin punto de decisión antes de presentar el plan"},
	}}

	report := BuildActivationReport(inspector, "/tmp/proyecto")

	if report.Problems() != 0 {
		t.Errorf("Problems() = %d, se esperaba 0 (not_applicable no es un problema)", report.Problems())
	}
	if len(report.Degradations) != 1 {
		t.Fatalf("esperaba 1 degradación declarada, got %d: %v", len(report.Degradations), report.Degradations)
	}
}

func TestBuildActivationReport_OrdenEstable(t *testing.T) {
	inspector := &fakeInspector{channels: []domain.ActivationChannel{
		{Arm: domain.ArmGomemory, Agent: "opencode", Scope: domain.ScopeProject, Kind: domain.KindInstructions, State: domain.StateOK},
		{Arm: domain.ArmGomemory, Agent: "claude", Scope: domain.ScopeProject, Kind: domain.KindPlanGuard, State: domain.StateOK},
	}}

	first := BuildActivationReport(inspector, "/tmp/x")
	second := BuildActivationReport(inspector, "/tmp/x")

	for i := range first.Channels {
		if first.Channels[i] != second.Channels[i] {
			t.Fatalf("el orden debe ser estable entre invocaciones, posición %d difiere", i)
		}
	}
}

func TestBuildActivationReport_UnaFilaFicticiaAparece(t *testing.T) {
	inspector := &fakeInspector{channels: []domain.ActivationChannel{
		{Agent: "agente-ficticio", Kind: domain.KindInstructions, Scope: domain.ScopeProject, State: domain.StateOK},
	}}

	report := BuildActivationReport(inspector, "/tmp/x")

	found := false
	for _, c := range report.Channels {
		if c.Agent == "agente-ficticio" {
			found = true
		}
	}
	if !found {
		t.Fatal("un canal que el inspector reporte debe aparecer en el reporte sin tocar este caso de uso")
	}
}
