package usecases_test

import (
	"context"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestHCDVerificationGolden(t *testing.T) {
	u := domain.Unit{ID: "HCD-001", Approach: "hcd", DefinitionRev: "def", DeliveryRev: "delivery", PlaybookRev: "playbook", ConstitutionRev: "constitution"}
	r := usecases.HCDVerification{Verdict: domain.HCDClosureVerdict{CanClose: true}, Criteria: map[string]domain.CriterionState{"C-01": domain.CriterionVerified}, Evidence: map[string]string{"EV-01": "fresh"}, Total: 1, Implemented: 1, Verified: 1, Approvals: []domain.Approval{{ID: "APR-01", Object: "definition", Revision: "def", Origin: "conversation", Client: "codex", Quote: "apruebo"}}, Drift: []string{"files_without_execution"}}
	want, e := os.ReadFile("testdata/hcd_verification.golden.md")
	if e != nil {
		t.Fatal(e)
	}
	got := usecases.RenderHCDVerification(u, r)
	if string(got) != string(want) {
		t.Fatalf("golden mismatch:\n%s", got)
	}
}
func TestHCDVerificationWarnsOnManualEdits(t *testing.T) {
	uc, repo, _, _, root := newHCDFlow(t)
	ctx := context.Background()
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	u.State = domain.StateReady
	if e = repo.UpdateUnit(ctx, u, u.StateRevision); e != nil {
		t.Fatal(e)
	}
	_, e = uc.Verify(ctx, u.ID, u.StateRevision, "verify-first")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "hcd", "features", "001-guardar-datos", "verification.md")
	if e = os.WriteFile(path, []byte("manual edit"), 0600); e != nil {
		t.Fatal(e)
	}
	v, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	r, e := uc.Verify(ctx, u.ID, v.Unit.StateRevision, "verify-again")
	if e != nil || len(r.Warnings) == 0 {
		t.Fatalf("warnings=%v e=%v", r.Warnings, e)
	}
}
