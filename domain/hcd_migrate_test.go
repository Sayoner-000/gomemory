package domain

import (
	"errors"
	"strings"
	"testing"
)

const migrateSpecFixture = `# Feature Specification: Demo

**Input**: texto

### User Story 1 - Guardar

1. **Given** un dato, **When** se guarda, **Then** persiste.

- **FR-001**: guardar datos.
- **SC-001**: rápido.
- [NEEDS CLARIFICATION: ¿límite?]
- suelto

## Assumptions

- supuesto

| a | b |
|---|---|
| 1 | 2 |

` + "```" + `
bloque
` + "```" + `
`

func classOf(items []MigrationItem, text string) string {
	for _, i := range items {
		if strings.Contains(i.Text, text) {
			return i.Class
		}
	}
	return "(sin elemento)"
}

func TestClassifyMigration_ClassesAndJustifications(t *testing.T) {
	items := ClassifyMigration(map[string][]byte{
		"spec.md":  []byte(migrateSpecFixture),
		"plan.md":  []byte("# Plan\n\n## Constitution Check\n\n- gate\n\n## Design\n\n- decisión\n"),
		"tasks.md": []byte("## Phase 1\n\n- [x] T001 hecha\n- [ ] T002 abierta\n\n## Notes\n\n- nota\n"),
	})
	for text, want := range map[string]string{
		"**FR-001**": MigrationMigrated, "**SC-001**": MigrationMigrated, "**Given**": MigrationMigrated,
		"NEEDS CLARIFICATION": MigrationPending, "suelto": MigrationPending, "supuesto": MigrationConsolidated,
		"| 1 | 2 |": MigrationConsolidated, "bloque": "(sin elemento)", "gate": MigrationDiscarded, "decisión": MigrationReference,
		"T001": MigrationReference, "T002": MigrationPending, "nota": MigrationDiscarded,
	} {
		if got := classOf(items, text); got != want {
			t.Errorf("%q → %s, quiero %s", text, got, want)
		}
	}
	for _, i := range items {
		if i.Class == MigrationDiscarded && i.Justification == "" || i.Source == "" {
			t.Errorf("elemento sin justificación u origen: %+v", i)
		}
	}
}

func TestMigrationRevisionsAndCounts(t *testing.T) {
	a := map[string][]byte{"spec.md": []byte("# A\n- **FR-001**: x\n")}
	b := map[string][]byte{"spec.md": []byte("# A\n- **FR-001**: y\n")}
	if MigrationSourceRevision(a) == MigrationSourceRevision(b) || MigrationSourceRevision(a) != MigrationSourceRevision(a) {
		t.Fatal("la revisión de fuentes debe depender solo del contenido")
	}
	m := Migration{SourceDir: "specs/001-x", SourceRev: MigrationSourceRevision(a), Items: ClassifyMigration(a)}
	m.Recount()
	if m.Counts[MigrationMigrated] != 1 || len(m.Revision) != 12 {
		t.Fatalf("migration=%+v", m)
	}
	first := m.Revision
	m.Status, m.CreatedAt = MigrationActivated, "2026-10-28"
	if m.ProposalRevision() != first {
		t.Fatal("la revisión de la propuesta no depende del estado ni de las fechas")
	}
	long := ClassifyMigration(map[string][]byte{"spec.md": []byte(strings.Repeat("x", 300))})
	if len([]rune(long[0].Text)) > 201 {
		t.Fatalf("texto sin acotar: %d", len([]rune(long[0].Text)))
	}
}

func TestHCDNextActionAndErrors(t *testing.T) {
	draft := Unit{Approach: "hcd", State: StateDraft}
	if got := NextAction(draft, []PendingCheck{{Code: "CLARIFICATION-OPEN", Message: "m"}}); got.Skill != "hcd-clarify" {
		t.Fatalf("next=%+v", got)
	}
	if got := NextAction(draft, []PendingCheck{{Code: "DEF-PENDING", Message: "m"}}); got.Skill != "hcd-define" || got.Command != "validate" {
		t.Fatalf("next=%+v", got)
	}
	if got := NextAction(Unit{Approach: "hcd", State: StateReady}, nil); got.Skill == "" {
		t.Fatalf("next=%+v", got)
	}
	for _, state := range []State{StateReady, StateInProgress, StateVerification, StateBlocked, StateCompleted, "otro"} {
		if len(AllowedActions(Unit{Approach: "hcd", State: state})) == 0 {
			t.Errorf("sin acciones para %s", state)
		}
	}
	if actions := AllowedActions(Unit{Approach: "speckit"}); len(actions) != 2 {
		t.Fatalf("speckit=%v", actions)
	}
	err := error(&HCDValidationError{})
	if !errors.Is(err, ErrValidation) || err.Error() == "" {
		t.Fatal("HCDValidationError envuelve ErrValidation")
	}
	if rules := PlaybookRules("### HCD-X-01. Regla\n### otra\n"); len(rules) != 1 || rules[0].ID != "HCD-X-01" {
		t.Fatalf("rules=%+v", rules)
	}
}
