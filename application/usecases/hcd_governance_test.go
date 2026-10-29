package usecases_test

import (
	"context"
	"errors"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"strings"
	"testing"
)

func governanceTemplates(t *testing.T) (string, string) {
	t.Helper()
	p, e := os.ReadFile("../../infrastructure/templates/hcd/hcd-playbook.md")
	if e != nil {
		t.Fatal(e)
	}
	c, e := os.ReadFile("../../infrastructure/templates/speckit-constitution-gen.md")
	if e != nil {
		t.Fatal(e)
	}
	return string(p), string(c)
}
func TestHCDPlaybookValidationAndAdoption(t *testing.T) {
	ctx := context.Background()
	uc, repo, _, _, _ := newHCDFlow(t)
	p, c := governanceTemplates(t)
	uc.ConfigureDocuments("project", nil, "# {Título}\n", p, c)
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	view, e := uc.PlaybookValidate(ctx, u.ID)
	if e != nil || len(view.Checks) != 0 {
		t.Fatalf("validation=%+v %v", view, e)
	}
	updated := p + "\nUrgente: seguridad\n"
	uc.ConfigureDocuments("project", nil, "# {Título}\n", updated, c)
	view, e = uc.PlaybookShow(ctx, u.ID, "playbook")
	if e != nil || view.FromRevision == view.ToRevision || !view.Urgent {
		t.Fatalf("impact=%+v %v", view, e)
	}
	old, e := repo.GetUnit(ctx, u.ID)
	if e != nil || old.PlaybookRev != u.PlaybookRev {
		t.Fatal("new document silently changed unit")
	}
	status, e := uc.Status(ctx, u.ID)
	if e != nil || len(status.RequiredAdoptions) != 1 {
		t.Fatalf("urgent=%+v %v", status, e)
	}
	in := usecases.HCDAdoptInput{UnitID: u.ID, Doc: "playbook", Revision: view.ToRevision, ExpectedRevision: u.StateRevision, OperationID: "adopt", Reason: "import", Actor: "dev", Client: "codex"}
	if _, e = uc.PlaybookAdopt(ctx, in); !errors.Is(e, domain.ErrInvalidQuote) {
		t.Fatalf("uncited adopt=%v", e)
	}
	in.Quote = "apruebo"
	u, e = uc.PlaybookAdopt(ctx, in)
	if e != nil || u.PlaybookRev != view.ToRevision {
		t.Fatalf("adopt=%+v %v", u, e)
	}
	decisions, e := repo.ListDecisions(ctx, u.ID)
	if e != nil || len(decisions) != 2 || decisions[1].Kind != "adopt" {
		t.Fatalf("decisions=%+v %v", decisions, e)
	}
}
func TestHCDExceptionValidation(t *testing.T) {
	ctx := context.Background()
	uc, repo, _, _, _ := newHCDFlow(t)
	p, c := governanceTemplates(t)
	uc.ConfigureDocuments("project", nil, "# {Título}\n", p, c)
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	x := domain.Exception{RuleID: "HCD-MEM-01", Scope: "close", Reason: "memoria no disponible", Alternatives: "esperar", Risks: "decisión pendiente", Compensating: "sincronizar después", Owner: "dev", ReviewCondition: "al recuperar memoria"}
	for _, field := range []string{"owner", "review", "reason", "compensating", "alternatives", "risks"} {
		t.Run(field, func(t *testing.T) {
			bad := x
			switch field {
			case "owner":
				bad.Owner = ""
			case "review":
				bad.ReviewCondition = ""
			case "reason":
				bad.Reason = ""
			case "compensating":
				bad.Compensating = ""
			case "alternatives":
				bad.Alternatives = ""
			case "risks":
				bad.Risks = ""
			}
			_, e := uc.Exception(ctx, usecases.HCDExceptionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, Quote: "apruebo", Client: "codex", Exception: bad})
			if !errors.Is(e, domain.ErrValidation) {
				t.Fatalf("missing %s=%v", field, e)
			}
		})
	}
	created, e := uc.Exception(ctx, usecases.HCDExceptionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, Quote: "apruebo", Client: "codex", OperationID: "exception", Exception: x})
	if e != nil || created.ApprovalID == "" {
		t.Fatalf("exception=%+v %v", created, e)
	}
	approvals, e := repo.ListApprovals(ctx, u.ID)
	if e != nil || len(approvals) != 1 || approvals[0].Object != "exception:"+created.ID {
		t.Fatalf("approval=%+v %v", approvals, e)
	}
}

func TestHCDDecisionRequiresMatchingNormativeException(t *testing.T) {
	ctx := context.Background()
	uc, repo, _, _, _ := newHCDFlow(t)
	p, c := governanceTemplates(t)
	uc.ConfigureDocuments("project", nil, "# {Título}\n", p, c)
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	refs := []string{"override:HCD-MEM-01:close"}
	if _, e = uc.RecordDecision(ctx, u.ID, "functional", "accepted", "Cerrar con memoria pendiente", u.DefinitionRev, refs, "test"); !errors.Is(e, domain.ErrBlocked) {
		t.Fatalf("unapproved override=%v", e)
	}
	x := domain.Exception{RuleID: "HCD-MEM-01", Scope: "close", Reason: "fallo memoria", Alternatives: "esperar", Risks: "pendiente", Compensating: "reintentar", Owner: "dev", ReviewCondition: "al recuperar"}
	if _, e = uc.Exception(ctx, usecases.HCDExceptionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "allow-memory", Quote: "apruebo", Client: "codex", Exception: x}); e != nil {
		t.Fatal(e)
	}
	if _, e = uc.RecordDecision(ctx, u.ID, "functional", "accepted", "Cerrar con memoria pendiente", u.DefinitionRev, refs, "test"); e != nil {
		t.Fatal(e)
	}
	ds, e := repo.ListDecisions(ctx, u.ID)
	if e != nil || len(ds) != 3 {
		t.Fatalf("decisions=%+v %v", ds, e)
	}
	if _, e = uc.RecordDecision(ctx, u.ID, "functional", "accepted", "Cambiar arquitectura", u.DefinitionRev, []string{"override:§3:unit"}, "test"); !errors.Is(e, domain.ErrBlocked) {
		t.Fatalf("constitutional override=%v", e)
	}
}

func TestHCDExceptionCannotRelaxConstitutionOrProtectedRule(t *testing.T) {
	for _, origin := range []string{"§3", "playbook"} {
		t.Run(origin, func(t *testing.T) {
			ctx := context.Background()
			uc, _, _, _, _ := newHCDFlow(t)
			p, c := governanceTemplates(t)
			p += "\n### HCD-PROTECTED-01. Regla protegida\nDescripción: proteger datos\nÁmbito: unidad\nComprobación: structural\nEvidencia: test\nExcepción: revisión\nOrigen: " + origin + "\n"
			if origin == "playbook" {
				p += "non_exceptionable: true\n"
			}
			uc.ConfigureDocuments("project", nil, "# {Título}\n", p, c)
			u, e := uc.Start(ctx, HCDStartInputForTest())
			if e != nil {
				t.Fatal(e)
			}
			_, e = uc.Exception(ctx, usecases.HCDExceptionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, Quote: "apruebo", Client: "codex", Exception: domain.Exception{RuleID: "HCD-PROTECTED-01", Scope: "unit", Reason: "motivo", Alternatives: "esperar", Risks: "riesgo", Compensating: "control", Owner: "dev", ReviewCondition: "revisar"}})
			if !errors.Is(e, domain.ErrBlocked) {
				t.Fatalf("protected override=%v", e)
			}
		})
	}
}
func TestHCDStructuralPlaybookConflicts(t *testing.T) {
	p, c := governanceTemplates(t)
	for _, tc := range []struct{ name, content, code string }{
		{"missing section", strings.Replace(p, "complementa §3", "complementa §999", 1), "CONSTITUTION-SECTION"},
		{"override", strings.Replace(p, "Descripción: Cada unidad", "Descripción: Esta regla sustituye §3. Cada unidad", 1), "CONSTITUTION-OVERRIDE"},
		{"copied constitution", p + "\n" + c, "CONSTITUTION-COPY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, check := range domain.ValidatePlaybook(tc.content, c) {
				if check.Code == tc.code {
					return
				}
			}
			t.Fatalf("missing %s", tc.code)
		})
	}
}
func TestHCDMemoryExceptionAllowsCloseAndExpiredExceptionBlocks(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "expired"}[expired], func(t *testing.T) {
			uc, u, root := verifiedHCDRoot(t)
			db, e := persistence.Init(root)
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			repo := persistence.NewHCDStore(db, "project")
			x := domain.Exception{ID: "E-01", UnitID: u.ID, RuleID: "HCD-MEM-01", Scope: "close", Status: "approved", ApprovalID: "AP-test", Owner: "dev", ReviewCondition: "recuperación"}
			if expired {
				x.ExpiresAt = "2020-01-01T00:00:00Z"
			}
			if e = repo.SaveException(context.Background(), x); e != nil {
				t.Fatal(e)
			}
			uc.ConfigureMemory(&hcdMemoryFailure{fail: true}, nil)
			closed, e := uc.Close(context.Background(), usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "close-memory-exception", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"})
			if expired {
				if e == nil {
					t.Fatal("expired exception allowed close")
				}
			} else if e != nil || closed.State != domain.StateCompleted {
				t.Fatalf("close=%+v %v", closed, e)
			}
		})
	}
}

// Una Constitución con la §18 previa a la enmienda exige Spec Kit por unidad:
// Start no crea la unidad ni deja artefactos. La plantilla distribuida sí
// reconoce HCD.
func TestHCDStartRejectsConstitutionWithoutHCDAmendment(t *testing.T) {
	ctx := context.Background()
	uc, _, db, _, root := newHCDFlow(t)
	p, c := governanceTemplates(t)
	legacy := "## 18\n\n```text\nspecs/{NNN}-{name}/\n```\n\n`spec.md`, `plan.md`, `tasks.md` y `checklists/requirements.md` son obligatorios.\n"
	uc.ConfigureDocuments("project", nil, "# {Título}\n", p, legacy)
	if _, e := uc.Start(ctx, HCDStartInputForTest()); !errors.Is(e, domain.ErrConstitutionConflict) {
		t.Fatalf("err=%v", e)
	}
	var units int
	if e := db.QueryRow("SELECT count(*) FROM hcd_units").Scan(&units); e != nil || units != 0 {
		t.Fatalf("units=%d err=%v", units, e)
	}
	if _, e := os.Stat(root + "/hcd"); !os.IsNotExist(e) {
		t.Fatalf("quedaron artefactos: %v", e)
	}
	uc.ConfigureDocuments("project", nil, "# {Título}\n", p, c)
	if _, e := uc.Start(ctx, HCDStartInputForTest()); e != nil {
		t.Fatal(e)
	}
}
