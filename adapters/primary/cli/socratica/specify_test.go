package socratica

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

const cliIntent = `{"objective":"Exportar historial","expected_result":"Un archivo","actors":[{"id":"A1","name":"Titular"}]}`

// specifyInPhase drives a specify flow of the env's project through the use case.
func (e *cliEnv) specifyInPhase(client, phase string) string {
	e.t.Helper()
	ctx := context.Background()
	res, err := e.u.Start(ctx, "p", client, domain.ModeSpecify, "Quiero exportar mi historial")
	if err != nil {
		e.t.Fatal(err)
	}
	id := res.Flow.ID
	if phase == "captured" {
		return id
	}
	if phase == "questions" {
		q := []domain.Question{{Text: "¿Formato?", Options: []string{"CSV", "JSON"}}}
		if _, err = e.u.Apply(ctx, id, domain.Event{Kind: "questions_generated", Questions: q}, usecases.OriginMCP); err != nil {
			e.t.Fatal(err)
		}
		return id
	}
	sections := loadCLISections()
	for i, s := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope, domain.StageStories} {
		if _, err = e.u.PrepareSet(ctx, id, i, s, []byte(sections[i])); err != nil {
			e.t.Fatal(err)
		}
	}
	for _, s := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope, domain.StageStories, domain.StageValidation} {
		if _, err = e.u.PrepareValidate(ctx, id, 3, s); err != nil {
			e.t.Fatal(err)
		}
	}
	if phase == "approved" {
		if _, err = e.u.ApproveSpecify(ctx, id, 3); err != nil {
			e.t.Fatal(err)
		}
	}
	return id
}

func loadCLISections() []string {
	return []string{
		cliIntent,
		`{"included":["Exportar el historial propio"],"excluded":{"declared":true,"items":[]},"constraints":{"declared":true,"items":[]},"assumptions":{"declared":true,"items":[]},"pending":{"declared":true,"items":[]}}`,
		`{"stories":[{"id":"US1","actor_id":"A1","need":"Exportar","outcome":"Consultar"}],"criteria":[{"id":"C1","story_ids":["US1"],"condition":"Tiene registros","expected_result":"El archivo los contiene"}],"deliveries":[{"id":"D1","title":"Exportación","outcome":"Exportable","story_ids":["US1"],"criterion_ids":["C1"]}]}`,
	}
}

// Querying a specify flow never enters the generic local routes: no generation,
// no dispatch, no text editing, no change of state.
func TestCmdSocraticaSpecify_QueryNeverRunsTheGenericRoutes(t *testing.T) {
	for _, phase := range []string{"captured", "questions", "review", "approved"} {
		e := newCLIEnv(t)
		launcher := &countingLauncher{}
		e.deps.SocraticaExec = launcher
		id := e.specifyInPhase("codex", phase)
		before, _ := e.repo.GetFlow(context.Background(), id)
		// Every key the generic review offers (edit, execute, copy, cancel) is "typed".
		code, out, errOut := e.run(true, "e\nx\nd\nc\n1\n2\nS\no\ns\n", "pending", "--resume", id)
		if code != socraticaExitOK {
			t.Fatalf("%s: exit %d out %q err %q", phase, code, out, errOut)
		}
		after, _ := e.repo.GetFlow(context.Background(), id)
		if after.Phase != before.Phase || after.Revision != before.Revision || after.DispatchAck || after.ApprovedPrompt != before.ApprovedPrompt || len(after.Answers) != len(before.Answers) {
			t.Fatalf("%s: the local query changed the flow: %s → %s", phase, before.Phase, after.Phase)
		}
		if launcher.calls != 0 || len(e.gen.seen) != 0 {
			t.Fatalf("%s: launches %d, generations %d", phase, launcher.calls, len(e.gen.seen))
		}
		for _, want := range []string{"specify", "Quiero exportar mi historial", "socratica_flow", "codex", "no se genera ni se ejecuta nada", "--cancel " + id} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: output lacks %q:\n%s", phase, want, out)
			}
		}
	}
}

func TestCmdSocraticaSpecify_ShowsStageRevisionAndPendingChecks(t *testing.T) {
	e := newCLIEnv(t)
	id := e.specifyInPhase("codex", "captured")
	_, out, _ := e.run(false, "", "pending", "--resume", id)
	for _, want := range []string{"Etapa: intent", "Revisión: 0", "Preguntas: 0/3", "section_missing", "intent", "scope", "stories"} {
		if !strings.Contains(out, want) {
			t.Errorf("captured: lacks %q:\n%s", want, out)
		}
	}
	rid := e.specifyInPhase("claude", "review")
	_, out, _ = e.run(false, "", "pending", "--resume", rid)
	for _, want := range []string{"Etapa: review", "Revisión: 3", "comprobada", "Aprobar y generar especificación"} {
		if !strings.Contains(out, want) {
			t.Errorf("review: lacks %q:\n%s", want, out)
		}
	}
}

func TestCmdSocraticaSpecify_ListMarksTheMode(t *testing.T) {
	e := newCLIEnv(t)
	e.specifyInPhase("codex", "review")
	_, out, _ := e.run(false, "", "pending")
	if !strings.Contains(out, "specify") || !strings.Contains(out, "review") {
		t.Fatalf("pending list = %q", out)
	}
}

func TestCmdSocraticaSpecify_ApprovedFlowStaysApprovedAndNothingIsConsumed(t *testing.T) {
	e := newCLIEnv(t)
	launcher := &countingLauncher{}
	e.deps.SocraticaExec = launcher
	id := e.specifyInPhase("codex", "approved")
	before, _ := e.repo.GetFlow(context.Background(), id)
	code, out, _ := e.run(true, "d\n", "pending", "--resume", id)
	after, _ := e.repo.GetFlow(context.Background(), id)
	if code != socraticaExitOK || after.Phase != domain.PhaseApproved || after.DispatchID != before.DispatchID || after.DispatchAck || launcher.calls != 0 {
		t.Fatalf("code %d phase %s ack %v launches %d", code, after.Phase, after.DispatchAck, launcher.calls)
	}
	if !strings.Contains(out, "Aprobado") {
		t.Fatalf("out = %q", out)
	}
}

func TestCmdSocraticaSpecify_ExplicitCancelFreesTheSlotAndPrintsTheOriginal(t *testing.T) {
	e := newCLIEnv(t)
	id := e.specifyInPhase("codex", "review")
	code, out, _ := e.run(false, "", "pending", "--cancel", id)
	if code != socraticaExitOK || !strings.Contains(out, "Cancelado") || !strings.Contains(out, "Quiero exportar mi historial") {
		t.Fatalf("cancel = %d %q", code, out)
	}
	f, _ := e.repo.GetFlow(context.Background(), id)
	if f.Phase != domain.PhaseCancelled || f.Preparation != nil || f.ApprovedPrompt != "" {
		t.Fatalf("cancelled flow = %+v", f)
	}
	if _, out, _ = e.run(false, "", "pending"); !strings.Contains(out, "Sin flujos pendientes") {
		t.Fatalf("pending after cancel = %q", out)
	}
}

// A private reservation seen from another instance explains where the content
// lives, offers the explicit cancel and never promises to show or recover it.
func TestCmdSocraticaSpecify_PrivateReservationFromAnotherInstance(t *testing.T) {
	e := newCLIEnv(t)
	owner := persistence.NewEphemeralSocraticaStore(e.repo)
	e.u.Repo, e.u.UOW = owner, owner
	e.u.Settings.Private = true
	id := e.specifyInPhase("codex", "review")

	other := persistence.NewEphemeralSocraticaStore(persistence.NewSocraticaStore(e.db, time.Second))
	e.u.Repo, e.u.UOW = other, other
	code, out, _ := e.run(true, "d\n", "pending", "--resume", id)
	if code != socraticaExitOK {
		t.Fatalf("exit %d %q", code, out)
	}
	for _, want := range []string{"solo existe en la instancia", "no puede mostrarlo ni recuperarlo", "--cancel " + id} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Quiero exportar mi historial") {
		t.Fatal("the private original was shown by an instance that does not hold it")
	}
	if f, _ := e.repo.GetFlow(context.Background(), id); f.Phase != domain.PhaseReview {
		t.Fatalf("the query changed the reservation: %s", f.Phase)
	}
	code, out, _ = e.run(false, "", "pending", "--cancel", id)
	if code != socraticaExitOK || !strings.Contains(out, "no está disponible") {
		t.Fatalf("cancel of the reservation = %d %q", code, out)
	}
	if _, out, _ = e.run(false, "", "pending"); !strings.Contains(out, "Sin flujos pendientes") {
		t.Fatalf("pending after the cancel = %q", out)
	}
}

// history --show never resends a specify flow locally: that would drop its
// outcome and the local executor refuses it anyway (C-002, ACR 042 r2).
func TestCmdSocraticaSpecify_HistoryShowNeverResends(t *testing.T) {
	e := newCLIEnv(t)
	ctx := context.Background()
	launcher := &countingLauncher{}
	e.deps.SocraticaExec = launcher
	id := e.specifyInPhase("codex", "approved")
	f, _ := e.repo.GetFlow(ctx, id)
	if _, err := e.u.Apply(ctx, id, domain.Event{Kind: "ack", DispatchID: f.DispatchID, Nonce: "n1"}, usecases.OriginMCP); err != nil {
		t.Fatal(err)
	}
	before, _ := e.repo.GetFlow(ctx, id)
	if err := e.repo.UpsertHistory(ctx, domain.RefinementHistory{ID: id, FlowID: id, Project: before.Project, Client: before.Client, Mode: domain.ModeSpecify, OriginalPrompt: before.OriginalPrompt, CompletedAt: time.Unix(100, 0).UTC()}, 5); err != nil {
		t.Fatal(err)
	}
	code, out, _ := e.run(true, "r\nreenviar\n", "history", "--show", id)
	after, _ := e.repo.GetFlow(ctx, id)
	if code != socraticaExitOK || after.DispatchID != before.DispatchID || after.Phase != domain.PhaseDispatched || launcher.calls != 0 {
		t.Fatalf("code %d phase %s dispatch %s→%s launches %d\n%s", code, after.Phase, before.DispatchID, after.DispatchID, launcher.calls, out)
	}
	if !strings.Contains(out, "desde el agente") || strings.Contains(out, "[r]eenviar") {
		t.Fatalf("out = %q", out)
	}
}

// A dispatched specify flow is not offered resume or cancel (C-003, ACR 042 r2).
func TestCmdSocraticaSpecify_DispatchedOffersNoCancel(t *testing.T) {
	e := newCLIEnv(t)
	ctx := context.Background()
	id := e.specifyInPhase("codex", "approved")
	f, _ := e.repo.GetFlow(ctx, id)
	if _, err := e.u.Apply(ctx, id, domain.Event{Kind: "ack", DispatchID: f.DispatchID, Nonce: "n1"}, usecases.OriginMCP); err != nil {
		t.Fatal(err)
	}
	_, out, _ := e.run(false, "", "pending", "--resume", id)
	if !strings.Contains(out, "Ya enviado") || strings.Contains(out, "--cancel") || strings.Contains(out, "resume") {
		t.Fatalf("out = %q", out)
	}
}

// Resending a dispatched flow while another one is pending for the agent is a
// domain error, never the raw UNIQUE failure of SQLite (C-009, ACR 042 r2).
func TestCmdSocraticaSpecify_ResendWithAnotherPendingFlow(t *testing.T) {
	e := newCLIEnv(t)
	ctx := context.Background()
	id := e.specifyInPhase("codex", "approved")
	f, _ := e.repo.GetFlow(ctx, id)
	if _, err := e.u.Apply(ctx, id, domain.Event{Kind: "ack", DispatchID: f.DispatchID, Nonce: "n1"}, usecases.OriginMCP); err != nil {
		t.Fatal(err)
	}
	if _, err := e.u.Start(ctx, "p", "codex", domain.ModeSpecify, "Otra necesidad"); err != nil {
		t.Fatal(err)
	}
	before, _ := e.repo.GetFlow(ctx, id)
	_, err := e.u.Apply(ctx, id, domain.Event{Kind: "force_resend", DispatchID: before.DispatchID}, usecases.OriginMCP)
	if !errors.Is(err, usecases.ErrResendPendingExists) {
		t.Fatalf("resend with another pending flow = %v, want ErrResendPendingExists", err)
	}
	if after, _ := e.repo.GetFlow(ctx, id); after.Phase != domain.PhaseDispatched || after.DispatchID != before.DispatchID {
		t.Fatalf("the refused resend changed the flow: %s %s", after.Phase, after.DispatchID)
	}
}
