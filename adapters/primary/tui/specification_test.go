package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type specificationStarterStub struct {
	calls int
	input usecases.HCDStartInput
	err   error
}

type specificationClock struct{}

func (specificationClock) Now() time.Time { return time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC) }

func TestSpecificationStartCreatesFastUnitAndDocument(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	flow := usecases.NewHCD(persistence.NewHCDStore(db, "project"), hcdfiles.Workspace{Root: root}, specificationClock{}, func() string { return "id" }, "# {TITLE}\n\nCarril: {LANE}\n")
	m := model{screen: screenSpecification, spec: newSpecificationState(flow)}
	m, _ = specificationKey(t, m, "enter")
	m.spec.title.SetValue("Guardar datos")
	m.spec.why.SetValue("cambio claro")
	m, cmd := specificationKey(t, m, "enter")
	if cmd == nil {
		t.Fatal("no creation command")
	}
	result := cmd().(specificationStartedMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.unit.Lane != domain.LaneFast || result.unit.Why != "cambio claro" {
		t.Fatalf("unit = %+v", result.unit)
	}
	doc, err := os.ReadFile(filepath.Join(root, result.unit.DocPath))
	if err != nil || !strings.Contains(string(doc), "Carril: fast") {
		t.Fatalf("doc = %q %v", doc, err)
	}
}

func (s *specificationStarterStub) Start(_ context.Context, input usecases.HCDStartInput) (usecases.HCDView, error) {
	s.calls++
	s.input = input
	return usecases.HCDView{Unit: domain.Unit{ID: "HCD-001", State: domain.UnitActive}}, s.err
}
func specificationKey(t *testing.T, m model, key string) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.updateSpecification(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	return next.(model), cmd
}
func TestSpecificationOptionsAndSpecKitDoesNotStart(t *testing.T) {
	flow := &specificationStarterStub{}
	m := model{screen: screenSpecification, spec: newSpecificationState(flow)}
	for _, o := range specificationOptions {
		text := o.label
		if !strings.Contains(m.specificationView(), text) {
			t.Fatalf("missing option %s", text)
		}
	}
	m, _ = specificationKey(t, m, "down")
	m, _ = specificationKey(t, m, "down")
	m, cmd := specificationKey(t, m, "enter")
	if cmd != nil || flow.calls != 0 || !strings.Contains(m.specificationView(), "/speckit-specify") {
		t.Fatal("Spec Kit started HCD or lacks guidance")
	}
}
func TestSpecificationFullLaneStartsCeremony(t *testing.T) {
	flow := &specificationStarterStub{}
	m := model{screen: screenSpecification, spec: newSpecificationState(flow)}
	m, _ = specificationKey(t, m, "down")
	m, _ = specificationKey(t, m, "enter")
	m.spec.title.SetValue("Exportar")
	m.spec.why.SetValue("riesgo alto")
	_, cmd := specificationKey(t, m, "enter")
	if cmd == nil {
		t.Fatal("no start command")
	}
	cmd()
	if flow.input.Lane != "full" {
		t.Fatalf("lane = %q, want full", flow.input.Lane)
	}
}

func TestSpecificationHCDStartAndFailure(t *testing.T) {
	for _, failure := range []bool{false, true} {
		flow := &specificationStarterStub{}
		if failure {
			flow.err = errors.New("no disponible")
		}
		m := model{screen: screenSpecification, spec: newSpecificationState(flow)}
		m, _ = specificationKey(t, m, "enter")
		m.spec.title.SetValue("Guardar datos")
		m.spec.why.SetValue("cambio claro")
		m, cmd := specificationKey(t, m, "enter")
		if cmd == nil {
			t.Fatal("no start command")
		}
		result := cmd()
		if flow.calls != 1 || flow.input.Client != "tui" || flow.input.Lane != "fast" || flow.input.Title != "Guardar datos" {
			t.Fatalf("input=%+v calls=%d", flow.input, flow.calls)
		}
		next, _ := m.Update(result)
		m = next.(model)
		if m.spec.created == failure || m.spec.busy {
			t.Fatalf("result=%+v", m.spec)
		}
	}
}
