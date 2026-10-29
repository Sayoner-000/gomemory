package usecases_test

import (
	"context"
	"errors"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func verifiedHCD(t *testing.T) (*usecases.HCD, domain.Unit) {
	t.Helper()
	uc, u, _ := verifiedHCDRoot(t)
	return uc, u
}

func TestHCDVerifyAutomaticChecks(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		change     func(*domain.Delivery, *domain.Unit)
		invalid    bool
	}{
		{"references", "ATOM-LINK", func(d *domain.Delivery, _ *domain.Unit) { d.Tasks[0].Criteria = []string{"C-99"} }, false},
		{"identifiers", "", func(d *domain.Delivery, _ *domain.Unit) { d.Tasks = append(d.Tasks, d.Tasks[0]) }, true},
		{"required fields", "ATOM-SCOPE", func(d *domain.Delivery, _ *domain.Unit) { d.Tasks[0].Scope = "" }, false},
		{"dependencies", "DEPENDENCY-CYCLE", func(d *domain.Delivery, _ *domain.Unit) { d.Tasks[0].Dependencies = []string{d.Tasks[0].ID} }, false},
		{"states", "", func(_ *domain.Delivery, u *domain.Unit) { u.State = domain.StateDraft }, true},
		{"registered evidence", "SLICE-UNVERIFIED", func(_ *domain.Delivery, _ *domain.Unit) {}, false},
		{"revisions", "DEFINITION-UNAPPROVED", func(_ *domain.Delivery, u *domain.Unit) { u.ApprovedRev = "old" }, false},
		{"approach isolation", "APPROACH", func(_ *domain.Delivery, u *domain.Unit) { u.Approach = "speckit" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			uc, u, repo, root := executableHCDRoot(t)
			d, err := uc.ReadDelivery(ctx, u.ID)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&d, &u)
			content := hcdfiles.RenderDelivery(u.ID, d)
			path := filepath.Join(root, "hcd", "features", strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "delivery.md")
			if err = os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			u.DeliveryRev = domain.ContentRevision(content)
			if err = repo.UpdateUnit(ctx, u, u.StateRevision); err != nil {
				t.Fatal(err)
			}
			r, err := uc.Verify(ctx, u.ID, u.StateRevision, "verify-matrix")
			if tc.invalid {
				if err == nil {
					t.Fatalf("invalid structure/state accepted: %+v", r)
				}
				return
			}
			if err != nil || r.Verdict.CanClose {
				t.Fatalf("verification=%+v %v", r, err)
			}
			for _, cause := range r.Verdict.Causes {
				if cause.Code == tc.code {
					return
				}
			}
			t.Fatalf("missing %s: %+v", tc.code, r.Verdict)
		})
	}
	// Exceptions are a domain fixture in the MVP; registration arrives in phase 10.
	t.Run("exceptions", func(t *testing.T) {
		now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
		u := domain.Unit{ID: "HCD-001", Approach: "hcd"}
		r := domain.ClosureVerdict(domain.HCDWork{Unit: u}, nil, domain.HCDVerificationContext{Unit: u}, nil, []domain.Exception{{ID: "E-01", Status: "approved", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)}}, now)
		for _, cause := range r.Causes {
			if cause.Code == "EXCEPTION-EXPIRED" && !r.CanClose {
				return
			}
		}
		t.Fatalf("expired exception ignored: %+v", r)
	})
}
func verifiedHCDRoot(t *testing.T) (*usecases.HCD, domain.Unit, string) {
	t.Helper()
	uc, u, _, root := executableHCDRoot(t)
	ctx := context.Background()
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) { return true, nil })
	x, e := uc.Run(ctx, u.ID, u.StateRevision, "run", "T-001", "codex")
	if e != nil {
		t.Fatal(e)
	}
	view, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = uc.Report(ctx, u.ID, view.Unit.StateRevision, "report", x.Execution.ID, domain.Evidence{Result: "pass", Procedure: "test", Environment: "temporary", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, Executor: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	view, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	r, e := uc.Verify(ctx, u.ID, view.Unit.StateRevision, "verify")
	if e != nil || !r.Verdict.CanClose {
		t.Fatalf("verify=%+v %v", r, e)
	}
	view, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	return uc, view.Unit, root
}

func TestHCDExternalDeliveryEditPreventsClose(t *testing.T) {
	ctx := context.Background()
	uc, u, root := verifiedHCDRoot(t)
	path := filepath.Join(root, "hcd", "features", strings.TrimPrefix(u.ID, "HCD-")+"-"+u.Slug, "delivery.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(content, []byte("\nCambio externo de entrega\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = uc.Close(ctx, usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-edited-delivery", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("edited delivery close=%v", err)
	}
	view, err := uc.Show(ctx, u.ID)
	if err != nil || view.Unit.State != domain.StateInProgress {
		t.Fatalf("state after stale close=%+v %v", view, err)
	}
}

func TestHCDDefinitionChangeKeepsLateReportStale(t *testing.T) {
	for _, state := range []string{"granted", "uncertain"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			uc, u, repo := executableHCDStore(t)
			x, err := uc.Run(ctx, u.ID, u.StateRevision, "run-before-change", "T-001", "codex")
			if err != nil {
				t.Fatal(err)
			}
			if state == "uncertain" {
				x.Execution.State = state
				if err = repo.SaveExecution(ctx, x.Execution); err != nil {
					t.Fatal(err)
				}
			}
			view, err := uc.Show(ctx, u.ID)
			if err != nil {
				t.Fatal(err)
			}
			d := validApprovalDefinition(view.Unit)
			d.Problem = "El fallo requiere revisar el alcance"
			changed, err := uc.Define(ctx, u.ID, view.Unit.StateRevision, "change-definition", d, hcdfiles.RenderDefinition(d))
			if err != nil {
				t.Fatal(err)
			}
			if changed.State != domain.StateDraft || changed.ApprovedRev != "" {
				t.Fatalf("changed=%+v", changed)
			}
			if _, err = uc.Run(ctx, u.ID, changed.StateRevision, "rerun-after-change", "T-001", "codex"); err == nil {
				t.Fatal("new run granted without approval")
			}
			saved, err := repo.GetExecution(ctx, u.ID, x.Execution.ID)
			if err != nil || saved.State != state {
				t.Fatalf("execution changed=%+v %v", saved, err)
			}
			if state == "uncertain" {
				changed, err = uc.Approve(ctx, usecases.HCDApproveInput{UnitID: u.ID, ExpectedRevision: changed.StateRevision, OperationID: "approve-new-definition", Object: "definition", Revision: changed.DefinitionRev, Quote: "apruebo", Client: "codex", Definition: d})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = uc.Run(ctx, u.ID, changed.StateRevision, "rerun-uncertain-after-approval", "T-001", "codex"); !errors.Is(err, domain.ErrUncertain) {
					t.Fatalf("uncertain execution regranted after approval: %v", err)
				}
			}
			_, err = uc.Report(ctx, u.ID, changed.StateRevision, "late-after-change", x.Execution.ID, domain.Evidence{Result: "pass", Procedure: "test", Environment: "temporary", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, Executor: "codex"})
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := repo.ListEvidence(ctx, u.ID)
			if err != nil || len(evidence) != 1 {
				t.Fatalf("evidence=%+v %v", evidence, err)
			}
			if got := domain.EvidenceFreshness(evidence[0], domain.HCDVerificationContext{Unit: changed, Subjects: map[string]bool{"tree:abc": true}}); got != "stale" {
				t.Fatalf("late report freshness=%s", got)
			}
		})
	}
}
func TestHCDVerifyPassDoesNotCloseAndCloseReleasesFocus(t *testing.T) {
	uc, u := verifiedHCD(t)
	if u.State != domain.StateVerification {
		t.Fatalf("PASS closed unit: %+v", u)
	}
	closed, e := uc.Close(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"})
	if e != nil || closed.State != domain.StateCompleted {
		t.Fatalf("close=%+v %v", closed, e)
	}
	if _, e = uc.ActiveUnitID(context.Background()); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("focus not released: %v", e)
	}
}

func TestHCDVerifySliceDoesNotAuthorizeIncompleteUnit(t *testing.T) {
	ctx := context.Background()
	uc, u := executableHCD(t)
	u, err := uc.Slice(ctx, u.ID, u.StateRevision, "second-slice", []domain.Slice{{ID: "VS-02", Observable: "Consultar datos", Limits: "Consulta", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}})
	if err != nil {
		t.Fatal(err)
	}
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) { return true, nil })
	x, err := uc.Run(ctx, u.ID, u.StateRevision, "slice-run", "T-001", "codex")
	if err != nil {
		t.Fatal(err)
	}
	v, err := uc.Show(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.Report(ctx, u.ID, v.Unit.StateRevision, "slice-report", x.Execution.ID, domain.Evidence{Result: "pass", Procedure: "test", Environment: "temporary", SubjectRef: []string{"tree:abc"}, Covers: []string{"T-001", "C-01"}, Executor: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = uc.Show(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	r, err := uc.Verify(ctx, u.ID, v.Unit.StateRevision, "verify-first-slice", "VS-01")
	if err != nil || !r.Verdict.CanClose || r.SliceID != "VS-01" || r.Verified != 1 {
		t.Fatalf("slice verification=%+v %v", r, err)
	}
	v, err = uc.Show(ctx, u.ID)
	if err != nil || v.Unit.State != domain.StateInProgress {
		t.Fatalf("partial PASS advanced unit=%+v %v", v, err)
	}
	if _, err = uc.Verify(ctx, u.ID, v.Unit.StateRevision, "verify-first-slice", "VS-02"); !errors.Is(err, domain.ErrOperationConflict) {
		t.Fatalf("slice idempotency conflict=%v", err)
	}
	if _, err = uc.Verify(ctx, u.ID, v.Unit.StateRevision, "verify-missing-slice", "VS-99"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown slice=%v", err)
	}
	r, err = uc.Verify(ctx, u.ID, v.Unit.StateRevision, "verify-whole-unit")
	if err != nil || r.Verdict.CanClose {
		t.Fatalf("incomplete unit verification=%+v %v", r, err)
	}
}

func TestHCDVerifySliceAdvancesFullyVerifiedUnit(t *testing.T) {
	uc, u := verifiedHCD(t)
	r, err := uc.Verify(context.Background(), u.ID, u.StateRevision, "verify-only-slice", "VS-01")
	if err != nil || !r.Verdict.CanClose {
		t.Fatalf("slice=%+v %v", r, err)
	}
	v, err := uc.Show(context.Background(), u.ID)
	if err != nil || v.Unit.State != domain.StateVerification {
		t.Fatalf("unit=%+v %v", v, err)
	}
}
func TestHCDCloseRejectAndCancel(t *testing.T) {
	uc, u := verifiedHCD(t)
	rejected, e := uc.Close(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "reject", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex", Reject: true, Reason: "ajustar presentación"})
	if e != nil || rejected.State != domain.StateInProgress {
		t.Fatalf("reject=%+v %v", rejected, e)
	}
	cancelled, e := uc.Cancel(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: rejected.StateRevision, OperationID: "cancel", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"})
	if e != nil || cancelled.State != domain.StateCancelled {
		t.Fatalf("cancel=%+v %v", cancelled, e)
	}
}
func TestHCDChangedSubjectPreventsClose(t *testing.T) {
	uc, u := verifiedHCD(t)
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) { return false, nil })
	_, e := uc.Close(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-stale", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"})
	if !errors.Is(e, domain.ErrNotAllowed) {
		t.Fatalf("stale close=%v", e)
	}
	v, e := uc.Show(context.Background(), u.ID)
	if e != nil || v.Unit.State != domain.StateInProgress {
		t.Fatalf("stale verification still authorizes: %+v %v", v, e)
	}
}

func TestHCDCloseRequiresApprovedDefinitionRevision(t *testing.T) {
	uc, u := verifiedHCD(t)
	_, err := uc.Close(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-wrong-revision", Revision: "000000000000", Quote: "apruebo", Client: "codex"})
	if !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("close with wrong approved revision: %v", err)
	}
	view, err := uc.Show(context.Background(), u.ID)
	if err != nil || view.Unit.State != domain.StateVerification {
		t.Fatalf("failed close changed state: %+v %v", view, err)
	}
}

func TestHCDCancelSupersededReleasesFocus(t *testing.T) {
	uc, u := verifiedHCD(t)
	replacement, err := uc.Start(context.Background(), usecases.HCDStartInput{Kind: domain.KindFix, Title: "Reemplazo", Implementer: "dev", Client: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	// Start focuses the new unit; restore focus to exercise cancellation of the old one.
	if _, err = uc.Focus(context.Background(), u.ID, "codex", "test", false); err != nil {
		t.Fatal(err)
	}
	cancelled, err := uc.Cancel(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "cancel-superseded", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex", SupersededBy: replacement.ID})
	if err != nil || cancelled.State != domain.StateSuperseded {
		t.Fatalf("superseded=%+v %v", cancelled, err)
	}
	if _, err = uc.ActiveUnitID(context.Background()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("focus not released: %v", err)
	}
}

func TestHCDCloseRejectsMissingQuote(t *testing.T) {
	uc, u := verifiedHCD(t)
	_, err := uc.Close(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-no-quote", Revision: u.DefinitionRev, Client: "codex"})
	if !errors.Is(err, domain.ErrInvalidQuote) {
		t.Fatalf("missing quote: %v", err)
	}
	view, err := uc.Show(context.Background(), u.ID)
	if err != nil || view.Unit.State != domain.StateVerification {
		t.Fatalf("missing quote changed state: %+v %v", view, err)
	}
}

func TestHCDVerifyReportsOtherApproachAsFailure(t *testing.T) {
	uc, u, store := executableHCDStore(t)
	u.Approach = "speckit"
	if err := store.UpdateUnit(context.Background(), u, u.StateRevision); err != nil {
		t.Fatal(err)
	}
	r, err := uc.Verify(context.Background(), u.ID, u.StateRevision, "verify-other-approach")
	if err != nil || r.Verdict.CanClose || len(r.Verdict.Causes) != 1 || r.Verdict.Causes[0].Code != "APPROACH" {
		t.Fatalf("other approach verification: %+v %v", r, err)
	}
}

// ACR C-002: si la verificación cambia entre prepareClose y la transacción
// final, el cierre se rechaza y la aceptación preparada no queda accepted.
func TestHCDCloseWithdrawsPreparedAcceptanceWhenFinalVerifyFails(t *testing.T) {
	uc, u, root := verifiedHCDRoot(t)
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	uc.ConfigureMemory(persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db))
	calls := 0
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) {
		calls++
		return calls == 1, nil
	})
	_, e := uc.Close(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-race", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"})
	if !errors.Is(e, domain.ErrNotAllowed) {
		t.Fatalf("close err=%v (inspector calls=%d)", e, calls)
	}
	v, e := uc.Show(context.Background(), u.ID)
	if e != nil || v.Unit.State != domain.StateInProgress {
		t.Fatalf("unit=%+v %v", v.Unit, e)
	}
	withdrawn := false
	for _, d := range v.Decisions {
		if d.Kind == "close" && d.Status == "accepted" {
			t.Fatalf("close acceptance still accepted after rejected close: %+v", d)
		}
		withdrawn = withdrawn || (d.Kind == "close" && d.Status == "superseded")
	}
	if !withdrawn {
		t.Fatalf("prepared acceptance not recorded as withdrawn: %+v", v.Decisions)
	}
}

// Si definition.md cambia entre prepareClose y la transacción final, la
// mutación falla al reconciliar: la aceptación preparada se retira y el mismo
// operation-id no puede reutilizarla para cerrar.
func TestHCDCloseWithdrawsPreparedAcceptanceWhenFinalMutationFails(t *testing.T) {
	uc, u, root := verifiedHCDRoot(t)
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	uc.ConfigureMemory(persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db))
	paths, err := filepath.Glob(filepath.Join(root, "hcd", "features", "*", "definition.md"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("definition paths=%v %v", paths, err)
	}
	edited := false
	uc.ConfigureSubjectInspector(func(context.Context, string) (bool, error) {
		if !edited {
			edited = true
			data, e := os.ReadFile(paths[0])
			if e != nil {
				return false, e
			}
			if e = os.WriteFile(paths[0], append(data, []byte("\nEdición externa.\n")...), 0o644); e != nil {
				return false, e
			}
		}
		return true, nil
	})
	in := usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-edited", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"}
	if _, e := uc.Close(context.Background(), in); e == nil {
		t.Fatal("el cierre debía fallar tras la edición externa")
	}
	v, e := uc.Show(context.Background(), u.ID)
	if e != nil || v.Unit.State == domain.StateCompleted {
		t.Fatalf("unit=%+v %v", v.Unit, e)
	}
	withdrawn := false
	for _, d := range v.Decisions {
		if d.Kind == "close" && d.Status == "accepted" {
			t.Fatalf("aceptación de cierre sigue accepted: %+v", d)
		}
		withdrawn = withdrawn || (d.Kind == "close" && d.Status == "superseded")
	}
	if !withdrawn {
		t.Fatalf("la aceptación preparada no quedó retirada: %+v", v.Decisions)
	}
	in.ExpectedRevision = v.Unit.StateRevision
	if got, e := uc.Close(context.Background(), in); e == nil || got.State == domain.StateCompleted {
		t.Fatalf("reintento cerró con la aceptación retirada: %+v %v", got, e)
	}
}
