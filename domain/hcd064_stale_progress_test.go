package domain

import (
	"slices"
	"strings"
	"testing"
)

// HCD-064 (Q1 → A): mientras queden tareas pendientes, una tarea caducada
// cuenta como progreso: el paso es implement, no figura entre las no
// verificadas, la siguiente acción no pide reverificar y analyze no la
// señala. Sin pendientes, el motor pide verify --stale y el cierre exige
// frescura.
func TestHCD064StaleCuentaComoProgresoConPendientes(t *testing.T) {
	doc := fill(render(t, KindFeature, LaneFast), LaneFast)
	doc = strings.Replace(doc, "## Checklist\n", "## Checklist\n- [x] 1. Hecha → go test ./a\n- [ ] 2. Pendiente → go test ./b\n", 1)
	s := stateOf(t, doc, KindFeature, LaneFast)
	s.Evidence = []Evidence{green(s, "f1", t0, "1")}
	s.Fingerprint = "f2" // se editó código para la tarea 2

	if st := s.TaskStatuses()["1"]; st != TaskStale {
		t.Fatalf("tarea 1 = %s, want stale (el estado sigue siendo informativo)", st)
	}
	if s.Stage() != StageImplement {
		t.Fatalf("stage = %s, want implement", s.Stage())
	}
	if got := s.Unverified(); !slices.Equal(got, []string{"2"}) {
		t.Fatalf("Unverified() = %v, want [2]: la caducada cuenta como progreso", got)
	}
	next := s.NextAction()
	if next.Skill != "hcd-implement" || strings.Contains(next.Why, "--stale") || strings.Contains(next.Why, "caduc") || strings.Contains(next.Why, "1") {
		t.Fatalf("next = %+v: no debe pedir reverificar la tarea 1", next)
	}
	for _, f := range s.Analyze() {
		if f.Ref == "1" {
			t.Fatalf("analyze señala la tarea caducada durante implement: %+v", f)
		}
	}
}

func TestHCD064SinPendientesElCierreExigeFrescura(t *testing.T) {
	doc := fill(render(t, KindFeature, LaneFast), LaneFast)
	doc = strings.Replace(doc, "## Checklist\n", "## Checklist\n- [x] 1. Hecha → go test ./a\n- [x] 2. Hecha → go test ./b\n", 1)
	s := stateOf(t, doc, KindFeature, LaneFast)
	s.Evidence = []Evidence{green(s, "f1", t0, "1"), green(s, "f2", t0, "2")}
	s.Fingerprint = "f2"

	if s.Stage() != StageVerify {
		t.Fatalf("stage = %s, want verify", s.Stage())
	}
	if got := s.Unverified(); !slices.Equal(got, []string{"1"}) {
		t.Fatalf("Unverified() = %v, want [1]", got)
	}
	if next := s.NextAction(); next.Skill != "hcd-verify" || !strings.Contains(next.Why, "--stale") {
		t.Fatalf("next = %+v: sin pendientes debe pedir verify --stale", next)
	}
	if !HasCritical(s.Analyze()) {
		t.Fatal("sin pendientes, una tarea caducada es crítica para cerrar")
	}
}
