package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
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
func TestSpecificationStartRecordsApproachDecision(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := persistence.NewHCDStore(db, "project")
	files, err := hcdfiles.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	flow := usecases.NewHCD(repo, repo, files, repo, specificationClock{}, nil)
	flow.ConfigureDocuments("project", nil, "# {Título}\n", "playbook", "constitution")
	m := model{screen: screenSpecification, spec: newSpecificationState(flow)}
	m, _ = specificationKey(t, m, "enter")
	m.spec.title.SetValue("Guardar datos")
	m.spec.implementer.SetValue("dev")
	m, cmd := specificationKey(t, m, "enter")
	if cmd == nil {
		t.Fatal("no creation command")
	}
	result := cmd().(specificationStartedMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	decisions, err := repo.ListDecisions(context.Background(), result.unit.ID)
	if err != nil || len(decisions) != 1 || decisions[0].Kind != "approach" || decisions[0].Status != "accepted" {
		t.Fatalf("decisions=%+v %v", decisions, err)
	}
	approvals, err := repo.ListApprovals(context.Background(), result.unit.ID)
	if err != nil || len(approvals) != 0 {
		t.Fatalf("creation approved definition: %+v %v", approvals, err)
	}
}

func (s *specificationStarterStub) Start(_ context.Context, input usecases.HCDStartInput) (domain.Unit, error) {
	s.calls++
	s.input = input
	return domain.Unit{ID: "HCD-001", State: domain.StateDraft}, s.err
}
func specificationKey(t *testing.T, m model, key string) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.updateSpecification(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	return next.(model), cmd
}
func TestSpecificationOptionsAndLegacyDoesNotStart(t *testing.T) {
	flow := &specificationStarterStub{}
	m := model{screen: screenSpecification, spec: newSpecificationState(flow)}
	for _, text := range []string{"HCD Playbook — recomendado", "Spec Kit — legado"} {
		if !strings.Contains(m.specificationView(), text) {
			t.Fatalf("missing option %s", text)
		}
	}
	m, _ = specificationKey(t, m, "down")
	m, cmd := specificationKey(t, m, "enter")
	if cmd != nil || flow.calls != 0 || !strings.Contains(m.specificationView(), "/speckit-specify") {
		t.Fatal("legacy started HCD or lacks guidance")
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
		m.spec.implementer.SetValue("dev")
		m, cmd := specificationKey(t, m, "enter")
		if cmd == nil {
			t.Fatal("no start command")
		}
		result := cmd()
		if flow.calls != 1 || flow.input.Client != "tui" || flow.input.Title != "Guardar datos" {
			t.Fatalf("input=%+v calls=%d", flow.input, flow.calls)
		}
		next, _ := m.Update(result)
		m = next.(model)
		if m.spec.created == failure || m.spec.busy {
			t.Fatalf("result=%+v", m.spec)
		}
	}
}
