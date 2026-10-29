package usecases_test

import (
	"context"
	"errors"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

type hcdMemoryFailure struct {
	ports.MemoryRepository
	fail bool
}

func (r *hcdMemoryFailure) Insert(m *domain.Memory) (int64, error) {
	if r.fail {
		return 0, errors.New("memory unavailable")
	}
	return r.MemoryRepository.Insert(m)
}
func TestHCDMemorySyncRetriesWithoutDuplicates(t *testing.T) {
	uc, repo, db, _, _ := newHCDFlow(t)
	mem := &hcdMemoryFailure{MemoryRepository: persistence.NewMemoryRepository(db), fail: true}
	uc.ConfigureMemory(mem, persistence.NewRelationRepository(db))
	u, e := uc.Start(context.Background(), HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	decisions, e := repo.ListDecisions(context.Background(), u.ID)
	if e != nil || decisions[0].MemorySync != "pending" {
		t.Fatalf("decisions=%+v %v", decisions, e)
	}
	mem.fail = false
	for i := 0; i < 2; i++ {
		if e = uc.MemorySync(context.Background(), u.ID); e != nil {
			t.Fatal(e)
		}
	}
	decisions, e = repo.ListDecisions(context.Background(), u.ID)
	if e != nil || decisions[0].MemorySync != "synced" || decisions[0].MemoryID == 0 {
		t.Fatalf("sync=%+v %v", decisions, e)
	}
	var n int
	if e = db.QueryRow("SELECT count(*) FROM memories WHERE topic_key=?", "hcd:"+u.ID+":D-01").Scan(&n); e != nil || n != 1 {
		t.Fatalf("memories=%d err=%v", n, e)
	}
	m, e := mem.Get("project", decisions[0].MemoryID)
	if e != nil || m.Type != domain.Decision || m.Filepath == "" {
		t.Fatalf("memory=%+v %v", m, e)
	}
}

func TestHCDDecisionAcceptanceAndSupersedesMirror(t *testing.T) {
	uc, repo, db, _, _ := newHCDFlow(t)
	uc.ConfigureMemory(persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db))
	ctx := context.Background()
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	old, e := uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "propose-old", Action: "propose", Kind: "scope_change", Summary: "primera decisión", Refs: []string{"C-01"}})
	if e != nil {
		t.Fatal(e)
	}
	v, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "uncited", Action: "accept", ID: old.ID, Client: "codex"}); !errors.Is(e, domain.ErrInvalidQuote) {
		t.Fatalf("uncited acceptance=%v", e)
	}
	_, e = uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "accept-old", Action: "accept", ID: old.ID, Quote: "apruebo", Client: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	v, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	next, e := uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "propose-new", Action: "propose", Kind: "scope_change", Summary: "segunda decisión", Refs: []string{"C-01"}})
	if e != nil {
		t.Fatal(e)
	}
	v, e = uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "replace", Action: "supersede", ID: old.ID, By: next.ID, Quote: "apruebo", Client: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	var count int
	if e = db.QueryRow("SELECT count(*) FROM memory_relations WHERE relation='supersedes'").Scan(&count); e != nil || count != 1 {
		t.Fatalf("supersedes=%d e=%v", count, e)
	}
	if e = uc.MemorySync(ctx, u.ID); e != nil {
		t.Fatal(e)
	}
	decisions, e := repo.ListDecisions(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range decisions {
		if d.ID == next.ID && d.MemorySync != "synced" {
			t.Fatalf("new decision pending: %+v", d)
		}
	}
}

func TestHCDNotApplicableRequiresCitedDecision(t *testing.T) {
	uc, u := executableHCD(t)
	ctx := context.Background()
	d, e := uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "propose-na", Action: "propose", Kind: "not_applicable", Summary: "Criterio fuera de esta plataforma", Refs: []string{"C-01"}})
	if e != nil {
		t.Fatal(e)
	}
	v, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "accept-na", Action: "accept", ID: d.ID, Quote: "apruebo", Client: "codex"})
	if e != nil {
		t.Fatal(e)
	}
	status, e := uc.Status(ctx, u.ID)
	if e != nil || status.Criteria["C-01"] != domain.CriterionNotApplicable {
		t.Fatalf("criterion=%v %v", status.Criteria, e)
	}
}

func TestHCDCloseWaitsForAcceptanceMemory(t *testing.T) {
	uc, u := verifiedHCD(t)
	db, e := persistence.Init(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	mem := &hcdMemoryFailure{MemoryRepository: persistence.NewMemoryRepository(db), fail: true}
	uc.ConfigureMemory(mem, persistence.NewRelationRepository(db))
	input := usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "memory-gated-close", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex"}
	if _, e = uc.Close(context.Background(), input); !errors.Is(e, domain.ErrBlocked) {
		t.Fatalf("pending close=%v", e)
	}
	v, e := uc.Show(context.Background(), u.ID)
	if e != nil || v.Unit.State != domain.StateVerification {
		t.Fatalf("closed before memory sync: %+v %v", v.Unit, e)
	}
	status, e := uc.Status(context.Background(), u.ID)
	if e != nil || status.PendingMemories == 0 {
		t.Fatalf("pending memories hidden: %+v %v", status, e)
	}
	mem.fail = false
	closed, e := uc.Close(context.Background(), input)
	if e != nil || closed.State != domain.StateCompleted {
		t.Fatalf("retry close=%+v %v", closed, e)
	}
	_, e = uc.Close(context.Background(), input)
	if e != nil {
		t.Fatalf("close replay failed: %v", e)
	}
}

func TestHCDDecisionProjectionAcrossProtocolSteps(t *testing.T) {
	for _, kind := range []string{"approach", "approval", "clarification_answer", "scope_change", "not_applicable", "block_open", "close", "reject"} {
		t.Run(kind, func(t *testing.T) {
			var uc *usecases.HCD
			var u domain.Unit
			if kind == "close" || kind == "reject" {
				uc, u = verifiedHCD(t)
			} else {
				uc, u = executableHCD(t)
			}
			db, e := persistence.Init(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			mem := persistence.NewMemoryRepository(db)
			uc.ConfigureMemory(mem, persistence.NewRelationRepository(db))
			ctx := context.Background()
			if e = uc.MemorySync(ctx, u.ID); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "approval":
				d, e := uc.ReadDefinition(ctx, u.ID)
				if e != nil {
					t.Fatal(e)
				}
				u, e = uc.Define(ctx, u.ID, u.StateRevision, "table-define", d, hcdfiles.RenderDefinition(d))
				if e != nil {
					t.Fatal(e)
				}
				_, e = uc.Approve(ctx, usecases.HCDApproveInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "table-approve", Object: "definition", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex", Definition: d})
				if e != nil {
					t.Fatal(e)
				}
			case "clarification_answer":
				c, e := uc.Clarify(ctx, usecases.HCDClarifyInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "table-open", Action: "open", TargetRef: "C-01", Question: "¿Cómo?"})
				if e != nil {
					t.Fatal(e)
				}
				v, e := uc.Show(ctx, u.ID)
				if e != nil {
					t.Fatal(e)
				}
				_, e = uc.Clarify(ctx, usecases.HCDClarifyInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "table-answer", Action: "answer", ClarificationID: c.ID, Choice: "Test"})
				if e != nil {
					t.Fatal(e)
				}
			case "scope_change", "not_applicable":
				d, e := uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "table-propose", Action: "propose", Kind: kind, Summary: "Decisión explícita", Refs: []string{"C-01"}})
				if e != nil {
					t.Fatal(e)
				}
				v, e := uc.Show(ctx, u.ID)
				if e != nil {
					t.Fatal(e)
				}
				_, e = uc.Decision(ctx, usecases.HCDDecisionInput{UnitID: u.ID, ExpectedRevision: v.Unit.StateRevision, OperationID: "table-accept", Action: "accept", ID: d.ID, Quote: "apruebo", Client: "codex"})
				if e != nil {
					t.Fatal(e)
				}
			case "block_open":
				_, e = uc.Block(ctx, usecases.HCDBlockInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "table-block", TargetRef: "T-001", Reason: "datos pendientes"})
				if e != nil {
					t.Fatal(e)
				}
			case "close", "reject":
				_, e = uc.Close(ctx, usecases.HCDCloseInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "table-close", Revision: u.DefinitionRev, Quote: "apruebo", Client: "codex", Reject: kind == "reject", Reason: "ajustar entrega"})
				if e != nil {
					t.Fatal(e)
				}
			}
			view, e := uc.Show(ctx, u.ID)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, decision := range view.Decisions {
				if decision.Kind != kind || decision.Status != "accepted" {
					continue
				}
				found = true
				if decision.Revision == "" || len(decision.Refs) == 0 || decision.MemoryID == 0 {
					t.Fatalf("incomplete decision: %+v", decision)
				}
				m, e := mem.Get("project", decision.MemoryID)
				if e != nil {
					t.Fatal(e)
				}
				for _, required := range []string{"Unidad:", "Revisión:", "Elementos:", "Origen:", "Fecha:", string(u.Kind)} {
					if !strings.Contains(m.Content, required) {
						t.Fatalf("memory missing %q: %+v", required, m)
					}
				}
			}
			if !found {
				t.Fatalf("decision %s missing", kind)
			}
		})
	}
}
