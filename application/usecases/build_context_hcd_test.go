package usecases_test

import (
	"context"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"strings"
	"testing"
)

func TestBuildHCDContextOnlyWithFocusAndWithinBudget(t *testing.T) {
	uc, _, db, _, root := newHCDFlow(t)
	b := usecases.New(persistence.NewMemoryRepository(db), persistence.NewSessionRepository(db), persistence.NewRelationRepository(db), root, "project")
	b.HCD = uc
	b.Budget = 4000
	before, e := b.Build()
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(before, "## HCD activo") {
		t.Fatal("active context without focus")
	}
	u, e := uc.Start(context.Background(), HCDStartInputForTest())
	if e != nil {
		t.Fatal(e)
	}
	after, e := b.Build()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(after, "## HCD activo") || !strings.Contains(after, u.ID) || !strings.Contains(after, "/hcd-define") {
		t.Fatalf("missing HCD context:\n%s", after)
	}
	if len(after) > b.Budget {
		t.Fatalf("budget exceeded %d", len(after))
	}
}
