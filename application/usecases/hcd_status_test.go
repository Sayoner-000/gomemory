package usecases_test

import (
	"context"
	"mem/application/usecases"
	"testing"
)

func TestHCDStatusSeparatesImplementedAndVerifiedAndCloseDecisions(t *testing.T) {
	uc, u := verifiedHCD(t)
	ctx := context.Background()
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) { return false, nil })
	r, e := uc.Status(ctx, u.ID)
	if e != nil || r.Implemented != 1 || r.Verified != 0 || r.Verdict.CanClose {
		t.Fatalf("status=%+v %v", r, e)
	}
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) { return true, nil })
	u, e = uc.Close(ctx, usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-status", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	v, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, d := range v.Decisions {
		found = found || d.Kind == "close"
	}
	if !found {
		t.Fatal("close decision missing")
	}
	uc, u = verifiedHCD(t)
	u, e = uc.Close(ctx, usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "reject-status", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex", Reject: true, Reason: "ajustar"})
	if e != nil {
		t.Fatal(e)
	}
	v, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	found = false
	for _, d := range v.Decisions {
		found = found || d.Kind == "reject"
	}
	if !found {
		t.Fatal("reject decision missing")
	}
}
