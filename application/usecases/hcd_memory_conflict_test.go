package usecases_test

import (
	"context"
	"mem/adapters/secondary/persistence"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCDDecisionConflictsDoNotChooseNewest(t *testing.T) {
	uc, _, db, _, _ := newHCDFlow(t)
	uc.ConfigureMemory(persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db))
	ctx := context.Background()
	u, e := uc.Start(ctx, HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	a, e := uc.RecordDecision(ctx, u.ID, "functional", "accepted", "Guardar en archivo", u.DefinitionRev, []string{"C-01"}, "conversation-A")
	if e != nil {
		t.Fatal(e)
	}
	b, e := uc.RecordDecision(ctx, u.ID, "functional", "accepted", "Guardar en base", u.DefinitionRev, []string{"C-01"}, "conversation-B")
	if e != nil {
		t.Fatal(e)
	}
	view, e := uc.Resume(ctx, u.ID, "codex", "session")
	if e != nil {
		t.Fatal(e)
	}
	if len(view.DecisionConflicts) != 1 || view.DecisionConflicts[0].A.ID != a.ID || view.DecisionConflicts[0].B.ID != b.ID {
		t.Fatalf("conflicts=%+v", view.DecisionConflicts)
	}
	found := false
	for _, c := range view.PendingChecks {
		found = found || c.Code == "DECISION-CONFLICT"
	}
	if !found || len(view.OpenClarifications) != 1 {
		t.Fatalf("checks=%+v clarifications=%+v", view.PendingChecks, view.OpenClarifications)
	}
	decisions := view.Decisions
	var aMemory, bMemory int64
	for _, d := range decisions {
		if d.ID == a.ID {
			aMemory = d.MemoryID
		}
		if d.ID == b.ID {
			bMemory = d.MemoryID
		}
	}
	relations := persistence.NewRelationRepository(db)
	_, e = relations.Insert(&domain.Relation{Project: "project", MemoryIDA: aMemory, MemoryIDB: bMemory, Relation: domain.ConflictsWith, Confidence: 1})
	if e != nil {
		t.Fatal(e)
	}
	// A conflict raised by GoMemory itself must suggest its impartial judge.
	all, e := uc.Show(ctx, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if len(all.DecisionConflicts) == 0 || !strings.Contains(all.DecisionConflicts[0].SuggestedAction, "judge_memories") {
		t.Fatal("memory conflict omitted")
	}
}
