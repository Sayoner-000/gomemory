package usecases_test

import (
	"context"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"strings"
	"testing"
)

func TestBuildHCDContextOnlyWithActiveUnitAndWithinBudget(t *testing.T) {
	env := newHCDEnv(t)
	b := usecases.New(persistence.NewMemoryRepository(env.db), persistence.NewSessionRepository(env.db), persistence.NewRelationRepository(env.db), env.root, "p")
	b.HCD = env.uc
	b.Budget = 4000
	before, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(before, "## HCD activo") {
		t.Fatal("active context without an active unit")
	}
	view, err := env.uc.Start(context.Background(), usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Guardar", Why: "w", Intent: "i"})
	if err != nil {
		t.Fatal(err)
	}
	after, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## HCD activo", view.Unit.ID, "paso `define`", view.Unit.DocPath, "Siguiente: `/hcd-define`"} {
		if !strings.Contains(after, want) {
			t.Fatalf("missing %q in HCD context:\n%s", want, after)
		}
	}
	if len(after) > b.Budget {
		t.Fatalf("budget exceeded %d", len(after))
	}
}
