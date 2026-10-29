package usecases_test

import (
	"context"
	"errors"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestHCDExpiredGrantIsUncertainAndLateReportReconciles(t *testing.T) {
	uc, u, repo := executableHCDStore(t)
	ctx := context.Background()
	execution := domain.Execution{ID: "X-old", UnitID: u.ID, TaskID: "T-001", State: "granted", DefinitionRev: u.DefinitionRev, DeliveryRev: u.DeliveryRev, GrantedAt: "2026-10-25T00:00:00Z"}
	if e := repo.SaveExecution(ctx, execution); e != nil {
		t.Fatal(e)
	}
	view, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = uc.Run(ctx, u.ID, view.Unit.StateRevision, "rerun-expired", "T-001", "codex"); !errors.Is(e, domain.ErrUncertain) {
		t.Fatalf("expired regrant=%v", e)
	}
	saved, e := repo.GetExecution(ctx, u.ID, execution.ID)
	if e != nil || saved.State != "uncertain" {
		t.Fatalf("expired=%+v %v", saved, e)
	}
	_, e = uc.Report(ctx, u.ID, view.Unit.StateRevision, "late-report", saved.ID, domain.Evidence{Result: "pass", Procedure: "test", Environment: "temporary", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, Executor: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	saved, e = repo.GetExecution(ctx, u.ID, execution.ID)
	if e != nil || saved.State != "reported" {
		t.Fatalf("late report=%+v %v", saved, e)
	}
}

func TestHCDGrantKeepsDeadlineSnapshot(t *testing.T) {
	uc, u, repo := executableHCDStore(t)
	r, e := uc.Run(context.Background(), u.ID, u.StateRevision, "snapshot-grant", "T-001", "codex", 1)
	if e != nil {
		t.Fatal(e)
	}
	stored, e := repo.GetExecution(context.Background(), u.ID, r.Execution.ID)
	if e != nil || stored.ExpiresAt != r.Execution.ExpiresAt || stored.ExpiresAt == "" {
		t.Fatalf("deadline=%+v %v", stored, e)
	}
}
func TestHCDUncertainResolvedByBlockDoesNotRegrant(t *testing.T) {
	uc, u, repo := executableHCDStore(t)
	ctx := context.Background()
	x := domain.Execution{ID: "X-unknown", UnitID: u.ID, TaskID: "T-001", State: "uncertain", DefinitionRev: u.DefinitionRev, DeliveryRev: u.DeliveryRev, GrantedAt: "2026-10-25T00:00:00Z"}
	if e := repo.SaveExecution(ctx, x); e != nil {
		t.Fatal(e)
	}
	b, e := uc.Block(ctx, usecases.HCDBlockInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "uncertain-block", TargetRef: "T-001", Reason: "resultado incierto"})
	if e != nil {
		t.Fatal(e)
	}
	v, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = uc.Block(ctx, usecases.HCDBlockInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "uncertain-resolve", ResolveID: b.ID, Resolution: "se cancela la concesión; revisar alcance"})
	if e != nil {
		t.Fatal(e)
	}
	v, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = uc.Run(ctx, u.ID, v.Unit.StateRevision, "do-not-repeat", "T-001", "codex"); !errors.Is(e, domain.ErrNotAllowed) {
		t.Fatalf("resolved uncertain regranted=%v", e)
	}
}
