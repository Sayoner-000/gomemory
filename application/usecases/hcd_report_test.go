package usecases_test

import (
	"context"
	"errors"
	"mem/adapters/secondary/hcdfiles"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCDReportProtectsSecretsAndRejectsInaccessibleInspection(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) { return true, nil })
	x, e := uc.Run(ctx, u.ID, u.StateRevision, "report-run", "T-001", "codex")
	if e != nil {
		t.Fatal(e)
	}
	v, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = uc.Report(ctx, u.ID, v.Unit.StateRevision, "secret-report", x.Execution.ID, domain.Evidence{Result: "pass", Procedure: "test <private>secret-value</private>", Environment: "temporary", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, InspectRef: []string{"https://inaccessible.invalid"}, Executor: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	status, e := uc.Status(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if len(status.EvidenceRecords) != 1 || strings.Contains(status.EvidenceRecords[0].Procedure, "secret-value") {
		t.Fatalf("unprotected evidence=%+v", status.EvidenceRecords)
	}
	if status.Verdict.CanClose {
		t.Fatal("inaccessible external evidence authorizes close")
	}
	d, e := uc.ReadDefinition(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	d.Problem = "problem <private>artifact-secret</private>"
	v, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = uc.Define(ctx, u.ID, v.Unit.StateRevision, "secret-definition", d, hcdfiles.RenderDefinition(d))
	if e != nil {
		t.Fatal(e)
	}
	got, e := uc.ReadDefinition(ctx, u.ID)
	if e != nil || strings.Contains(got.Problem, "artifact-secret") {
		t.Fatalf("artifact secret=%q %v", got.Problem, e)
	}
}
func TestHCDReportNamesMissingFields(t *testing.T) {
	uc, u := executableHCD(t)
	_, e := uc.Report(context.Background(), u.ID, u.StateRevision, "missing", "X-1", domain.Evidence{})
	var invalid *domain.HCDValidationError
	if !errors.As(e, &invalid) || len(invalid.Checks) != 4 {
		t.Fatalf("missing fields=%+v %v", invalid, e)
	}
}
