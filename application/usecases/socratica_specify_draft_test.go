package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/domain"
)

// The four request classes of the spec: clear, ambiguous, technical, too big.
var specifyRequestClasses = map[string]string{
	"clara":     "Quiero que una persona pueda exportar su historial en CSV.",
	"ambigua":   "Hay que mejorar el panel porque la gente se queja.",
	"tecnica":   "Migrar el job nocturno de cron a una cola con reintentos idempotentes.",
	"demasiado": "Rehacer toda la plataforma: cobros, reportes, notificaciones, permisos y la app móvil.",
}

func specifyFixture() (*SocraticaFlow, *socraticaFakeRepo) {
	u, r := socraticaFixture()
	r.flow = domain.RefinementFlow{}
	return u, r
}

const intentJSON = `{"objective":"Exportar historial","expected_result":"Un archivo","actors":[{"id":"A1","name":"Titular"}]}`

func TestSpecifyStart_FourRequestClassesKeepOriginalAndDoNotExecute(t *testing.T) {
	for name, prompt := range specifyRequestClasses {
		u, r := specifyFixture()
		res, err := u.Start(context.Background(), "p", "codex", domain.ModeSpecify, prompt)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		f := res.Flow
		if f.Mode != domain.ModeSpecify || f.Phase != domain.PhaseCaptured || f.OriginalPrompt != prompt || f.Revision != 0 {
			t.Errorf("%s: flow = %+v", name, f)
		}
		if f.Preparation == nil || f.Preparation.Stage != domain.StageIntent || f.Preparation.SchemaVersion != domain.PreparationSchemaVersion {
			t.Errorf("%s: preparation = %+v, want a fresh intent-stage aggregate", name, f.Preparation)
		}
		if r.flow.Preparation == nil || f.ApprovedPrompt != "" || f.DispatchID != "" {
			t.Errorf("%s: start must not approve or authorise anything: %+v", name, f)
		}
		if !strings.Contains(res.Next, "no ejecutes") {
			t.Errorf("%s: next = %q, want an instruction that does not execute", name, res.Next)
		}
	}
}

func TestSpecifyStart_PolicyFollowsPrivateSettingAtCreation(t *testing.T) {
	for _, private := range []bool{false, true} {
		u, _ := specifyFixture()
		u.Settings.Private = private
		res, err := u.Start(context.Background(), "p", "codex", domain.ModeSpecify, "hola")
		want := domain.PayloadPersistent
		if private {
			want = domain.PayloadEphemeral
		}
		if err != nil || res.Flow.PayloadPolicy != want {
			t.Fatalf("private=%v: policy %q, %v; want %q", private, res.Flow.PayloadPolicy, err, want)
		}
	}
}

func TestSpecifyStart_RecoversPendingFlowOfAnotherModeUnchanged(t *testing.T) {
	u, r := socraticaFixture() // a pending refine flow of project p / claude
	res, err := u.Start(context.Background(), "p", "claude", domain.ModeSpecify, "otra necesidad")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Flow.Recovered || res.Flow.Mode == domain.ModeSpecify || res.Flow.ID != "f" || r.created != 0 {
		t.Fatalf("start over a pending flow = %+v (created %d); it must be returned, not replaced or converted", res.Flow, r.created)
	}
	if r.flow.Preparation != nil {
		t.Fatal("a pending refine flow was given a preparation")
	}
}

func TestSpecifyPrepareSet_ReplacesSectionWithExpectedRevision(t *testing.T) {
	ctx := context.Background()
	u, r := specifyFixture()
	started, _ := u.Start(ctx, "p", "codex", domain.ModeSpecify, specifyRequestClasses["clara"])
	id := started.Flow.ID
	res, err := u.PrepareSet(ctx, id, 0, domain.StageIntent, []byte(intentJSON))
	if err != nil {
		t.Fatal(err)
	}
	if res.Flow.Revision != 1 || res.Flow.Preparation.Content.Intent.Objective != "Exportar historial" || r.flow.Revision != 1 {
		t.Fatalf("after set: rev %d, %+v", res.Flow.Revision, res.Flow.Preparation.Content.Intent)
	}
	// The same value again is a no-op that keeps the revision.
	again, err := u.PrepareSet(ctx, id, 1, domain.StageIntent, []byte(intentJSON))
	if err != nil || again.Flow.Revision != 1 {
		t.Fatalf("identical set: rev %d, %v; want revision kept", again.Flow.Revision, err)
	}
	// A stale revision changes nothing and says to read again.
	_, err = u.PrepareSet(ctx, id, 0, domain.StageIntent, []byte(strings.Replace(intentJSON, "Exportar", "Importar", 1)))
	if !errors.Is(err, domain.ErrRevisionConflict) || r.flow.Preparation.Content.Intent.Objective != "Exportar historial" || r.flow.Revision != 1 {
		t.Fatalf("stale set = %v; state %+v", err, r.flow.Preparation.Content.Intent)
	}
}

func TestSpecifyPrepareSet_ProtectsBeforeSaving(t *testing.T) {
	ctx := context.Background()
	u, r := specifyFixture()
	started, _ := u.Start(ctx, "p", "codex", domain.ModeSpecify, "x")
	secret := `{"objective":"con <private>interno-042</private> dentro","expected_result":"r","actors":[{"id":"A1","name":"clave AKIAIOSFODNN7EXAMPLE"}]}`
	if _, err := u.PrepareSet(ctx, started.Flow.ID, 0, domain.StageIntent, []byte(secret)); err != nil {
		t.Fatal(err)
	}
	stored := r.flow.Preparation.Content.Intent
	if strings.Contains(stored.Objective, "interno-042") || strings.Contains(stored.Actors[0].Name, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("unprotected content reached the repository: %+v", stored)
	}
}

func TestSpecifyPrepareSet_RejectsComputedFieldsAndNeverGenerates(t *testing.T) {
	ctx := context.Background()
	u, r := specifyFixture()
	started, _ := u.Start(ctx, "p", "codex", domain.ModeSpecify, "x")
	for _, raw := range []string{
		`{"objective":"o","stage":"approved"}`,
		`{"objective":"o","checks":[{"passed":true}]}`,
		`{"objective":"o","ready":true}`,
	} {
		if _, err := u.PrepareSet(ctx, started.Flow.ID, 0, domain.StageIntent, []byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if r.flow.Revision != 0 || r.flow.Preparation.Content.Intent != nil {
		t.Fatalf("a rejected value changed the draft: %+v", r.flow.Preparation)
	}
	// socraticaFixture has no Generator: any generation attempt would panic.
	if u.Generator != nil {
		t.Fatal("the fixture must not provide a generator")
	}
}

func TestSpecifyPrepareSet_OnlyForSpecifyFlows(t *testing.T) {
	u, _ := socraticaFixture() // refine flow "f"
	if _, err := u.PrepareSet(context.Background(), "f", 0, domain.StageIntent, []byte(intentJSON)); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("prepare on refine = %v, want ErrInvalidTransition", err)
	}
}

func TestSpecifyView_StageActionsAndPendingSections(t *testing.T) {
	ctx := context.Background()
	u, _ := specifyFixture()
	started, _ := u.Start(ctx, "p", "codex", domain.ModeSpecify, "x")
	v := SpecifyView(started.Flow)
	if v.Stage != domain.StageIntent || v.QuestionsUsed != 0 || v.ReviewHash != "" {
		t.Fatalf("view = %+v", v)
	}
	paths := map[string]bool{}
	for _, c := range v.PendingChecks {
		paths[c.Path] = c.Code == "section_missing"
	}
	if !paths["intent"] || !paths["scope"] || !paths["stories"] {
		t.Fatalf("pending checks = %+v; every missing section is reported", v.PendingChecks)
	}
	for _, a := range v.AllowedActions {
		if a == "approve" || a == "ack" || a == "force_resend" {
			t.Fatalf("allowed actions %v open delivery before its guards exist", v.AllowedActions)
		}
	}
	res, _ := u.PrepareSet(ctx, started.Flow.ID, 0, domain.StageIntent, []byte(intentJSON))
	for _, c := range SpecifyView(res.Flow).PendingChecks {
		if c.Path == "intent" {
			t.Fatalf("a provided section is still reported missing: %+v", c)
		}
	}
	// The budget shows in the view and closes the questions action at three.
	f := res.Flow
	f.Questions = make([]domain.Question, 3)
	f.Answers = make([]domain.Answer, 3)
	view := SpecifyView(f)
	if view.QuestionsUsed != 3 || contains(view.AllowedActions, "answer") {
		t.Fatalf("exhausted budget view = %+v", view)
	}
}

func TestSpecifyView_UnavailablePrivateStateOnlyReadsAndCancels(t *testing.T) {
	f := domain.RefinementFlow{Mode: domain.ModeSpecify, Phase: domain.PhaseReview, PrivateStateUnavailable: true}
	v := SpecifyView(f)
	if len(v.AllowedActions) != 2 || !contains(v.AllowedActions, "get") || !contains(v.AllowedActions, "cancel") {
		t.Fatalf("actions = %v", v.AllowedActions)
	}
	if !strings.Contains(SocraticaNext(f), "cancel") {
		t.Fatalf("next = %q", SocraticaNext(f))
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestSpecifyView_ActionsAndNextPerPhase(t *testing.T) {
	cases := []struct {
		phase   domain.SocraticaPhase
		actions []string
		next    string
	}{
		{domain.PhaseQuestions, []string{"get", "resume", "prepare", "answer", "cancel"}, "pregunta con opciones"},
		{domain.PhaseReview, []string{"get", "resume", "prepare", "approve", "cancel"}, "Aprobar y generar especificación"},
		{domain.PhaseFailed, []string{"get", "cancel"}, "no ejecutes"},
		{domain.PhaseRewriting, []string{"get", "cancel"}, "no ejecutes"},
		{domain.PhaseCancelled, []string{"get"}, "devuelve el original"},
	}
	for _, c := range cases {
		f := domain.RefinementFlow{Mode: domain.ModeSpecify, Phase: c.phase, OriginalPrompt: "original", Preparation: domain.NewSpecifyPreparation()}
		v := SpecifyView(f)
		if strings.Join(v.AllowedActions, ",") != strings.Join(c.actions, ",") {
			t.Errorf("%s: actions = %v, want %v", c.phase, v.AllowedActions, c.actions)
		}
		if next := SocraticaNext(f); !strings.Contains(next, c.next) || strings.Contains(next, "Enviar mi original") {
			t.Errorf("%s: next = %q, want it to contain %q", c.phase, next, c.next)
		}
	}
	// A flow without preparation (never created by Start) still renders safely.
	if v := SpecifyView(domain.RefinementFlow{Mode: domain.ModeSpecify, Phase: domain.PhaseCaptured}); v.Stage != "" || len(v.AllowedActions) == 0 {
		t.Fatalf("view without preparation = %+v", v)
	}
}

// A cancelled flow whose ephemeral payload was dropped has no original to
// return, so next must not promise one (C-003, ACR 042).
func TestSocraticaNext_CancelledWithoutOriginal(t *testing.T) {
	f := domain.RefinementFlow{Mode: domain.ModeSpecify, Phase: domain.PhaseCancelled}
	if next := SocraticaNext(f); next != NextCancelledWithoutOriginal {
		t.Fatalf("next = %q, want %q", next, NextCancelledWithoutOriginal)
	}
	f.OriginalPrompt = "original"
	if next := SocraticaNext(f); !strings.Contains(next, "devuelve el original") {
		t.Fatalf("next with original = %q", next)
	}
}

func TestSpecifyView_DispatchedReservationWithoutPayload(t *testing.T) {
	f := domain.RefinementFlow{Mode: domain.ModeSpecify, Phase: domain.PhaseDispatched, PayloadPolicy: domain.PayloadEphemeral, PrivateStateUnavailable: true}
	if v := SpecifyView(f); strings.Join(v.AllowedActions, ",") != "get,cancel" {
		t.Fatalf("actions = %v, want get,cancel", v.AllowedActions)
	}
	if next := SocraticaNext(f); strings.Contains(next, "report") || strings.Contains(next, "force_resend") {
		t.Fatalf("next = %q offers routes that need the payload", next)
	}
}
