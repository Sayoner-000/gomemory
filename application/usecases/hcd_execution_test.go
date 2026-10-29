package usecases_test

import (
	"context"
	"errors"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func executableHCD(t *testing.T) (*usecases.HCD, domain.Unit) {
	uc, u, _ := executableHCDStore(t)
	return uc, u
}
func executableHCDStore(t *testing.T) (*usecases.HCD, domain.Unit, *persistence.HCDStore) {
	t.Helper()
	uc, u, repo, _ := executableHCDRoot(t)
	return uc, u, repo
}
func executableHCDRoot(t *testing.T) (*usecases.HCD, domain.Unit, *persistence.HCDStore, string) {
	t.Helper()
	uc, repo, _, _, root := newHCDFlow(t)
	ctx := context.Background()
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	d := validApprovalDefinition(u)
	u, e = uc.Define(ctx, u.ID, u.StateRevision, "definition", d, hcdfiles.RenderDefinition(d))
	if e != nil {
		t.Fatal(e)
	}
	u.State = domain.StateReady
	u.ApprovedRev = u.DefinitionRev
	if e = repo.UpdateUnit(ctx, u, u.StateRevision); e != nil {
		t.Fatal(e)
	}
	u, e = uc.Slice(ctx, u.ID, u.StateRevision, "slice", []domain.Slice{{ID: "VS-01", Observable: "Guardar datos", Limits: "Guardar", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}})
	if e != nil {
		t.Fatal(e)
	}
	u, e = uc.Tasks(ctx, u.ID, u.StateRevision, "tasks", "VS-01", []domain.Task{{ID: "T-001", Title: "Guardar datos", SliceID: "VS-01", Type: "implementation", Stories: []string{"US1"}, Criteria: []string{"C-01"}, Result: "Datos guardados", Scope: "Persistencia", Excluded: "Borrado", Verification: "Test", Rules: []string{"HCD-TRACE-01"}}})
	if e != nil {
		t.Fatal(e)
	}
	return uc, u, repo, root
}
func TestHCDExecutionGrantReportAndIdempotency(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	a, e := uc.Check(ctx, u.ID, "T-001")
	if e != nil || len(a.Checks) > 0 {
		t.Fatalf("admission=%+v e=%v", a, e)
	}
	if _, e = uc.Run(ctx, u.ID, u.StateRevision, "no-executor", "T-001", "terminal"); !errors.Is(e, domain.ErrNoExecutor) {
		t.Fatalf("executor=%v", e)
	}
	r, e := uc.Run(ctx, u.ID, u.StateRevision, "run", "T-001", "codex")
	if e != nil {
		t.Fatal(e)
	}
	if r.Execution.ID == "" || r.Admission.Task.Scope == "" || r.Admission.DefinitionRev != u.DefinitionRev {
		t.Fatalf("run=%+v", r)
	}
	view, e := uc.Show(ctx, u.ID)
	if e != nil || view.Unit.State != domain.StateInProgress {
		t.Fatalf("state=%+v %v", view, e)
	}
	ev := domain.Evidence{Procedure: "go test ./domain", Environment: "temporary", Result: "pass", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, Executor: "codex"}
	x, e := uc.Report(ctx, u.ID, view.Unit.StateRevision, "report", r.Execution.ID, ev)
	if e != nil {
		t.Fatal(e)
	}
	again, e := uc.Report(ctx, u.ID, view.Unit.StateRevision, "report", r.Execution.ID, ev)
	if e != nil || again.ResultHash != x.ResultHash {
		t.Fatalf("replay=%+v %v", again, e)
	}
	view, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	ev.Result = "fail"
	if _, e = uc.Report(ctx, u.ID, view.Unit.StateRevision, "changed-report", r.Execution.ID, ev); !errors.Is(e, domain.ErrOperationConflict) {
		t.Fatalf("changed result=%v", e)
	}
}
func TestHCDReportRejectsMissingFields(t *testing.T) {
	uc, u := executableHCD(t)
	if _, e := uc.Report(context.Background(), u.ID, u.StateRevision, "bad", "X-missing", domain.Evidence{Result: "pass"}); !errors.Is(e, domain.ErrValidation) {
		t.Fatalf("missing fields=%v", e)
	}
}

// La etapa de una unidad ready/in_progress se quedaba siempre en slicing
// porque nadie derivaba ReadySlice ni la admisión de tareas.
func TestHCDNextActionFollowsDeliveryProgress(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	step := func(want string) usecases.HCDView {
		t.Helper()
		view, e := uc.Show(ctx, u.ID)
		if e != nil || view.NextAction.Skill != want {
			t.Fatalf("next=%+v stage=%s err=%v; want %s", view.NextAction, view.Stage, e, want)
		}
		return view
	}
	step("hcd-implement")
	r, e := uc.Run(ctx, u.ID, u.StateRevision, "run", "T-001", "claude")
	if e != nil {
		t.Fatal(e)
	}
	view := step("hcd-implement")
	ev := domain.Evidence{Procedure: "go test", Environment: "temporary", Result: "pass", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, Executor: "claude"}
	if _, e = uc.Report(ctx, u.ID, view.Unit.StateRevision, "report", r.Execution.ID, ev); e != nil {
		t.Fatal(e)
	}
	step("hcd-verify")
}

func TestHCDReadySliceWithoutTasksAsksForDecomposition(t *testing.T) {
	uc, repo, _, _, _ := newHCDFlow(t)
	ctx := context.Background()
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	d := validApprovalDefinition(u)
	if u, e = uc.Define(ctx, u.ID, u.StateRevision, "definition", d, hcdfiles.RenderDefinition(d)); e != nil {
		t.Fatal(e)
	}
	u.State, u.ApprovedRev = domain.StateReady, u.DefinitionRev
	if e = repo.UpdateUnit(ctx, u, u.StateRevision); e != nil {
		t.Fatal(e)
	}
	if view, _ := uc.Show(ctx, u.ID); view.NextAction.Skill != "hcd-slice" {
		t.Fatalf("before slice next=%+v", view.NextAction)
	}
	if _, e = uc.Slice(ctx, u.ID, u.StateRevision, "slice", []domain.Slice{{ID: "VS-01", Observable: "Guardar datos", Limits: "Guardar", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}}); e != nil {
		t.Fatal(e)
	}
	if view, _ := uc.Show(ctx, u.ID); view.NextAction.Skill != "hcd-tasks" {
		t.Fatalf("after slice next=%+v", view.NextAction)
	}
}

// Una tarea mal planteada no tenía salida: tasks solo actualiza y block no la
// saca del cálculo, así que el slice nunca podía verificarse.
func TestHCDCancelledTaskLeavesAdmissionAndTotals(t *testing.T) {
	uc, u, repo := executableHCDStore(t)
	ctx := context.Background()
	bad := domain.Task{ID: "T-002", Title: "Documentar", SliceID: "VS-01", Type: "documentation", Stories: []string{"US1"}, Criteria: []string{"C-99"}, Result: "Doc", Scope: "README", Excluded: "Código", Verification: "Lectura", Rules: []string{"HCD-TRACE-01"}}
	u, e := uc.Tasks(ctx, u.ID, u.StateRevision, "tasks-bad", "VS-01", []domain.Task{bad})
	if e != nil {
		t.Fatal(e)
	}
	if a, e := uc.Check(ctx, u.ID, "T-002"); e != nil || len(a.Checks) == 0 {
		t.Fatalf("bad task should be unadmitted: %+v %v", a, e)
	}
	d, e := uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "propose", Action: "propose", Kind: domain.DecisionTaskCancelled, Summary: "Retirar T-002", Refs: []string{"T-002"}, Client: "claude"})
	if e != nil {
		t.Fatal(e)
	}
	u, _ = repo.GetUnit(ctx, u.ID)
	if _, e = uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "accept", Action: "accept", ID: d.ID, Quote: "retira T-002 " + u.DefinitionRev, Client: "claude"}); e != nil {
		t.Fatal(e)
	}
	if _, e = uc.Check(ctx, u.ID, "T-002"); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("cancelled task check err=%v", e)
	}
	view, e := uc.Show(ctx, u.ID)
	if e != nil || view.NextAction.Skill != "hcd-implement" {
		t.Fatalf("next=%+v err=%v", view.NextAction, e)
	}
}

// FR-032: una evidencia caducada debe poder revalidarse con otra ejecución;
// mientras siga vigente, repetir run sigue prohibido.
func TestHCDRunRevalidatesOnlyStaleEvidence(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	current := true
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) { return current, nil })
	r, e := uc.Run(ctx, u.ID, u.StateRevision, "run", "T-001", "claude")
	if e != nil {
		t.Fatal(e)
	}
	view, _ := uc.Show(ctx, u.ID)
	ev := domain.Evidence{Procedure: "go test", Environment: "temporary", Result: "pass", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, Executor: "claude"}
	if _, e = uc.Report(ctx, u.ID, view.Unit.StateRevision, "report", r.Execution.ID, ev); e != nil {
		t.Fatal(e)
	}
	view, _ = uc.Show(ctx, u.ID)
	if _, e = uc.Run(ctx, u.ID, view.Unit.StateRevision, "rerun-fresh", "T-001", "claude"); !errors.Is(e, domain.ErrNotAllowed) {
		t.Fatalf("rerun with fresh evidence err=%v", e)
	}
	current = false
	if _, e = uc.Run(ctx, u.ID, view.Unit.StateRevision, "rerun-stale", "T-001", "claude"); e != nil {
		t.Fatalf("rerun with stale evidence err=%v", e)
	}
}
