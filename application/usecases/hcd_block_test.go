package usecases_test

import (
	"context"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestHCDBlockLocalAndCritical(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	d, e := uc.ReadDelivery(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	task := d.Tasks[0]
	task.ID = "T-002"
	u, e = uc.Tasks(ctx, u.ID, u.StateRevision, "independent-task", "VS-01", []domain.Task{task})
	if e != nil {
		t.Fatal(e)
	}
	b, e := uc.Block(ctx, usecases.HCDBlockInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "local-block", TargetRef: "T-001", Reason: "esperar datos"})
	if e != nil {
		t.Fatal(e)
	}
	v, e := uc.Show(ctx, u.ID)
	if e != nil || v.Unit.State != domain.StateReady {
		t.Fatalf("local block state=%s e=%v", v.Unit.State, e)
	}
	a, e := uc.Check(ctx, u.ID, "T-001")
	if e != nil || len(a.Checks) == 0 {
		t.Fatalf("blocked task=%+v %v", a, e)
	}
	independent, e := uc.Check(ctx, u.ID, "T-002")
	if e != nil || len(independent.Checks) > 0 {
		t.Fatalf("independent=%+v %v", independent, e)
	}
	_, e = uc.Block(ctx, usecases.HCDBlockInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "resolve-local", ResolveID: b.ID, Resolution: "datos disponibles"})
	if e != nil {
		t.Fatal(e)
	}
	v, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	b, e = uc.Block(ctx, usecases.HCDBlockInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "critical", TargetRef: u.ID, Reason: "contrato incompatible", Critical: true})
	if e != nil {
		t.Fatal(e)
	}
	v, e = uc.Show(ctx, u.ID)
	if e != nil || v.Unit.State != domain.StateBlocked {
		t.Fatalf("critical=%+v %v", v.Unit, e)
	}
	_, e = uc.Block(ctx, usecases.HCDBlockInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "resolve-critical", ResolveID: b.ID, Resolution: "contrato restaurado"})
	if e != nil {
		t.Fatal(e)
	}
	v, e = uc.Show(ctx, u.ID)
	if e != nil || v.Unit.State != domain.StateReady {
		t.Fatalf("restored=%+v %v", v.Unit, e)
	}
}
