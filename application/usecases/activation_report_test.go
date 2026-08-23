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

// TestBuildActivationReport_DegradacionesSinRepetir: una misma degradación
// declarada en dos ámbitos es UNA degradación, no dos. `mem doctor` la
// imprimía repetida porque la lista se construía por canal, y el texto de la
// línea (agente + tipo + motivo) no distingue el ámbito.
func TestBuildActivationReport_DegradacionesSinRepetir(t *testing.T) {
	motivo := "el ciclo del agente no ofrece un punto de decisión antes de presentar el plan"
	inspector := &fakeInspector{channels: []domain.ActivationChannel{
		{Agent: "opencode", Kind: domain.KindPlanGuard, Scope: domain.ScopeProject, State: domain.StateNotApplicable, Detail: motivo},
		{Agent: "opencode", Kind: domain.KindPlanGuard, Scope: domain.ScopeUser, State: domain.StateNotApplicable, Detail: motivo},
	}}

	report := BuildActivationReport(inspector, "/tmp/proyecto")

	if len(report.Degradations) != 1 {
		t.Fatalf("esperaba 1 degradación, got %d: %v", len(report.Degradations), report.Degradations)
	}
}

// TestBuildActivationReport_DegradacionesDistintasSeConservan: la deduplicación
// no puede tragarse degradaciones que sí son distintas.
func TestBuildActivationReport_DegradacionesDistintasSeConservan(t *testing.T) {
	inspector := &fakeInspector{channels: []domain.ActivationChannel{
		{Agent: "opencode", Kind: domain.KindPlanGuard, Scope: domain.ScopeProject, State: domain.StateNotApplicable, Detail: "motivo A"},
		{Agent: "opencode", Kind: domain.KindPlanEntry, Scope: domain.ScopeProject, State: domain.StateNotApplicable, Detail: "motivo B"},
		{Agent: "claude", Kind: domain.KindInstructions, Scope: domain.ScopeProject, State: domain.StateNotApplicable, Detail: "motivo C"},
	}}

	report := BuildActivationReport(inspector, "/tmp/proyecto")

	if len(report.Degradations) != 3 {
		t.Fatalf("esperaba 3 degradaciones distintas, got %d: %v", len(report.Degradations), report.Degradations)
	}
}
