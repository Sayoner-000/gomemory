package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

var tuiSections = []string{
	`{"objective":"Exportar historial","expected_result":"Un archivo","actors":[{"id":"A1","name":"Titular"}]}`,
	`{"included":["Exportar el historial propio"],"excluded":{"declared":true,"items":[]},"constraints":{"declared":true,"items":[]},"assumptions":{"declared":true,"items":[]},"pending":{"declared":true,"items":[]}}`,
	`{"stories":[{"id":"US1","actor_id":"A1","need":"Exportar","outcome":"Consultar"}],"criteria":[{"id":"C1","story_ids":["US1"],"condition":"Tiene registros","expected_result":"El archivo los contiene"}],"deliveries":[{"id":"D1","title":"Exportación","outcome":"Exportable","story_ids":["US1"],"criterion_ids":["C1"]}]}`,
}

const tuiOriginal = "Quiero exportar mi historial autorizado"

// specify drives a specify flow through the use case up to the given phase.
func (s *socTUI) specify(phase string) string {
	s.t.Helper()
	ctx := context.Background()
	u := s.m.soc.deps.Flow
	res, err := u.Start(ctx, "p", "codex", domain.ModeSpecify, tuiOriginal)
	if err != nil {
		s.t.Fatal(err)
	}
	id := res.Flow.ID
	switch phase {
	case "captured":
		return id
	case "questions":
		q := []domain.Question{{Text: "¿Formato?", Options: []string{"CSV", "JSON"}}}
		if _, err = u.Apply(ctx, id, domain.Event{Kind: "questions_generated", Questions: q}, usecases.OriginMCP); err != nil {
			s.t.Fatal(err)
		}
		return id
	}
	for i, st := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope, domain.StageStories} {
		if _, err = u.PrepareSet(ctx, id, i, st, []byte(tuiSections[i])); err != nil {
			s.t.Fatal(err)
		}
	}
	for _, st := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope, domain.StageStories, domain.StageValidation} {
		if _, err = u.PrepareValidate(ctx, id, 3, st); err != nil {
			s.t.Fatal(err)
		}
	}
	if phase == "approved" || phase == "dispatched" {
		approved, e := u.ApproveSpecify(ctx, id, 3)
		if e != nil {
			s.t.Fatal(e)
		}
		if phase == "dispatched" {
			if _, e = u.Apply(ctx, id, domain.Event{Kind: "ack", DispatchID: approved.Flow.DispatchID, Nonce: "n1"}, usecases.OriginMCP); e != nil {
				s.t.Fatal(e)
			}
		}
	}
	return id
}

// openDetail opens Socratica and enters the first pending row.
func (s *socTUI) openDetail() {
	s.t.Helper()
	s.press("p")
	s.press("enter")
}

func (s *socTUI) storedFlow(id string) domain.RefinementFlow {
	s.t.Helper()
	f, err := s.store.GetFlow(context.Background(), id)
	if err != nil {
		s.t.Fatal(err)
	}
	return f
}

func TestSocraticaTUISpecify_DetailShowsStageAndPointsBackToTheAgent(t *testing.T) {
	for _, phase := range []string{"captured", "questions", "review", "approved"} {
		s := newSocTUI(t)
		s.specify(phase)
		s.openDetail()
		if s.m.screen != screenSocraticaDetail {
			t.Fatalf("%s: screen %v; a specify flow never opens the generic questions screen", phase, s.m.screen)
		}
		view := s.view()
		for _, want := range []string{"specify", "Etapa", "Revisión", tuiOriginal, "agente", "x Cancelar"} {
			if !strings.Contains(view, want) {
				t.Errorf("%s: the detail lacks %q:\n%s", phase, want, view)
			}
		}
		for _, generic := range []string{"e Ejecutar", "d Editar", "c Copiar", "r Reenviar", "o Enviar mi original", "Reintentar"} {
			if strings.Contains(view, generic) {
				t.Errorf("%s: the detail offers the generic action %q", phase, generic)
			}
		}
	}
}

func TestSocraticaTUISpecify_GenericKeysDoNothing(t *testing.T) {
	for _, phase := range []string{"captured", "questions", "review", "approved", "dispatched"} {
		s := newSocTUI(t)
		id := s.specify(phase)
		before := s.storedFlow(id)
		s.openDetail()
		for _, key := range []string{"e", "d", "c", "r", "o", "a", "h", "m", "y", "1", "2", "enter"} {
			s.press(key)
		}
		after := s.storedFlow(id)
		if s.launcher.calls.Load() != 0 || s.gen.calls.Load() != 0 {
			t.Fatalf("%s: launches %d generations %d", phase, s.launcher.calls.Load(), s.gen.calls.Load())
		}
		if s.m.soc.inputMode != "" || s.m.soc.busy != "" {
			t.Fatalf("%s: input mode %q busy %q", phase, s.m.soc.inputMode, s.m.soc.busy)
		}
		if after.Phase != before.Phase || after.Revision != before.Revision || after.DispatchID != before.DispatchID || after.DispatchAck != before.DispatchAck || len(after.Answers) != len(before.Answers) {
			t.Fatalf("%s: a generic key changed the flow: %s → %s", phase, before.Phase, after.Phase)
		}
	}
}

func TestSocraticaTUISpecify_DispatchedHasNoResendFromHere(t *testing.T) {
	s := newSocTUI(t)
	id := s.specify("dispatched")
	before := s.storedFlow(id)
	s.openDetail()
	s.press("r")
	s.press("y")
	view := s.view()
	if strings.Contains(view, "r Reenviar") || strings.Contains(view, "confirmar reenvío") || !strings.Contains(view, "Ya enviado") {
		t.Fatalf("dispatched specify detail:\n%s", view)
	}
	if after := s.storedFlow(id); after.DispatchID != before.DispatchID || after.Phase != domain.PhaseDispatched {
		t.Fatal("a resend was authorised from the local view")
	}
}

func TestSocraticaTUISpecify_CancelIsTheOnlyAction(t *testing.T) {
	s := newSocTUI(t)
	id := s.specify("review")
	s.openDetail()
	s.press("x")
	f := s.storedFlow(id)
	if f.Phase != domain.PhaseCancelled || f.Preparation != nil || f.ApprovedPrompt != "" {
		t.Fatalf("cancel = %+v", f)
	}
	if s.m.screen != screenSocratica {
		t.Fatalf("screen after cancel = %v", s.m.screen)
	}
}

func TestSocraticaTUISpecify_ListMarksTheMode(t *testing.T) {
	s := newSocTUI(t)
	s.specify("review")
	s.press("p")
	if view := s.view(); !strings.Contains(view, "specify") {
		t.Fatalf("the list does not mark the mode:\n%s", view)
	}
}

// A reservation held by another instance says so, shows no content and only
// offers the explicit cancel.
func TestSocraticaTUISpecify_PrivateReservationWithoutPayload(t *testing.T) {
	s := newSocTUI(t)
	owner := persistence.NewEphemeralSocraticaStore(s.store)
	ownerFlow := &usecases.SocraticaFlow{Repo: owner, UOW: owner, Lock: persistence.NewSocraticaDispatchLock(s.root), Clock: clock.SystemClock{}, NewID: func() string { return uuid.Must(uuid.NewV7()).String() }}
	ownerFlow.Settings.Private = true
	s.m.soc.deps.Flow = ownerFlow
	id := s.specify("review")

	other := persistence.NewEphemeralSocraticaStore(persistence.NewSocraticaStore(s.db, time.Second))
	s.m.soc = newSocraticaState(SocraticaDeps{Flow: &usecases.SocraticaFlow{Repo: other, UOW: other, Lock: persistence.NewSocraticaDispatchLock(s.root), Clock: clock.SystemClock{}, NewID: ownerFlow.NewID}, Launcher: s.launcher})
	s.openDetail()
	view := s.view()
	for _, want := range []string{"solo existe en la instancia", "x Cancelar"} {
		if !strings.Contains(view, want) {
			t.Errorf("the reservation view lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, tuiOriginal) {
		t.Fatal("private content shown by an instance that does not hold it")
	}
	s.press("e", "d", "c", "r", "y")
	if got := s.storedFlow(id); got.Phase != domain.PhaseReview || s.launcher.calls.Load() != 0 {
		t.Fatalf("a generic key acted on the reservation: %s", got.Phase)
	}
	s.press("x")
	if got := s.storedFlow(id); got.Phase != domain.PhaseCancelled {
		t.Fatalf("explicit cancel of the reservation: %s", got.Phase)
	}
	// Nothing of a private flow is kept, so the status must not promise it (C-006, ACR 042 r2).
	if strings.Contains(s.m.statusMsg, "se conserva") || !strings.Contains(s.m.statusMsg, "descartó") {
		t.Fatalf("status after a private cancel = %q", s.m.statusMsg)
	}
}

// Drawing is pure and fits every width: no repository, generator or launcher
// call, no overflow, and the key facts stay visible.
func TestSocraticaTUISpecify_DrawingIsPureAtEveryWidth(t *testing.T) {
	for _, phase := range []string{"captured", "review", "approved", "dispatched"} {
		s := newSocTUI(t)
		s.specify(phase)
		s.openDetail()
		for _, width := range []int{20, 40, 59, 60, 80, 99, 100, 140} {
			s.m.width = width
			repoBefore, genBefore := s.repo.calls.Load(), s.gen.calls.Load()
			first := s.m.renderView()
			for i := 0; i < 20; i++ {
				_ = s.m.View()
				if got := s.m.renderView(); got != first {
					t.Fatalf("%s/%d: View is not deterministic", phase, width)
				}
			}
			if s.repo.calls.Load() != repoBefore || s.gen.calls.Load() != genBefore || s.launcher.calls.Load() != 0 {
				t.Fatalf("%s/%d: drawing touched the repository, generator or launcher", phase, width)
			}
			for _, line := range strings.Split(ansi.Strip(first), "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("%s/%d: line wider than the terminal (%d): %q", phase, width, ansi.StringWidth(line), line)
				}
			}
		}
	}
}
