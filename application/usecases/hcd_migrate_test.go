package usecases_test

import (
	"context"
	"database/sql"
	"errors"
	"mem/adapters/secondary/speckit"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migrateSpec = `# Feature Specification: Demo legado

**Input**: descripción original

### User Story 1 - Guardar (Priority: P1)

1. **Given** un dato, **When** se guarda, **Then** persiste.

- **FR-001**: El sistema DEBE guardar datos.
- **SC-001**: El guardado tarda menos de un segundo.
- [NEEDS CLARIFICATION: ¿qué límite de tamaño?]
- Nota suelta sin categoría.
`

const migratePlan = `# Plan

## Technical Context

- Lenguaje: Go

## Constitution Check

- Gate superado
`

const migrateTasks = `# Tasks

## Phase 1

- [x] T001 Preparar el esquema
- [ ] T002 Escribir pruebas

## Dependencies & Execution Order

- La fase 1 bloquea la 2.
`

func newMigrateFixture(t *testing.T) (*usecases.HCD, string, string, *sql.DB) {
	t.Helper()
	uc, _, db, _, root := newHCDFlow(t)
	uc.ConfigureSpecKitReader(speckit.Reader{})
	dir := filepath.Join(root, "specs", "044-demo-legado")
	if e := os.MkdirAll(dir, 0o755); e != nil {
		t.Fatal(e)
	}
	for name, body := range map[string]string{"spec.md": migrateSpec, "plan.md": migratePlan, "tasks.md": migrateTasks} {
		if e := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); e != nil {
			t.Fatal(e)
		}
	}
	return uc, root, dir, db
}

func approaches(t *testing.T, uc *usecases.HCD) map[string]string {
	t.Helper()
	list, e := uc.List(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	out := map[string]string{}
	for _, u := range list {
		out[u.ID] = u.Approach
	}
	return out
}

func TestHCDMigratePreviewClassifiesEveryItemAndLeavesSpecKitActive(t *testing.T) {
	uc, root, dir, _ := newMigrateFixture(t)
	m, e := uc.MigratePreview(context.Background(), "044-demo-legado")
	if e != nil {
		t.Fatal(e)
	}
	if m.ID != "M-01" || m.Status != domain.MigrationProposed || m.UnitID != "" || len(m.Items) == 0 {
		t.Fatalf("migration=%+v", m)
	}
	valid := map[string]bool{domain.MigrationMigrated: true, domain.MigrationConsolidated: true, domain.MigrationReference: true, domain.MigrationDiscarded: true, domain.MigrationPending: true}
	byText := map[string]domain.MigrationItem{}
	for _, item := range m.Items {
		if !valid[item.Class] {
			t.Fatalf("unclassified item %+v", item)
		}
		if item.Class == domain.MigrationDiscarded && item.Justification == "" {
			t.Fatalf("discarded without justification %+v", item)
		}
		byText[item.Text] = item
	}
	for class, want := range map[string]int{domain.MigrationMigrated: 3, domain.MigrationDiscarded: 3} {
		if m.Counts[class] < want {
			t.Fatalf("counts=%v", m.Counts)
		}
	}
	for _, c := range []string{domain.MigrationMigrated, domain.MigrationConsolidated, domain.MigrationReference, domain.MigrationDiscarded, domain.MigrationPending} {
		if m.Counts[c] == 0 {
			t.Fatalf("class %s missing: %v", c, m.Counts)
		}
	}
	done := byText["- [x] T001 Preparar el esquema"]
	if done.Class != domain.MigrationReference || !strings.Contains(done.Justification, "no se promueve a verificada") {
		t.Fatalf("done task=%+v", done)
	}
	if byText["- [ ] T002 Escribir pruebas"].Class != domain.MigrationPending || byText["- [NEEDS CLARIFICATION: ¿qué límite de tamaño?]"].Class != domain.MigrationPending {
		t.Fatal("open items must stay pending")
	}
	// C-20: sin activar, la unidad sigue en Spec Kit y las fuentes no cambian.
	if got := approaches(t, uc); got["044-demo-legado"] != "speckit" || len(got) != 1 {
		t.Fatalf("approaches=%v", got)
	}
	if content, _ := os.ReadFile(filepath.Join(dir, "tasks.md")); string(content) != migrateTasks {
		t.Fatal("preview modified a source")
	}
	if _, err := os.Stat(filepath.Join(root, "hcd", "features")); err == nil {
		t.Fatal("preview wrote HCD artifacts")
	}
}

func TestHCDMigrateActivateKeepsSourcesAndSingleApproach(t *testing.T) {
	uc, root, dir, _ := newMigrateFixture(t)
	ctx := context.Background()
	m, e := uc.MigratePreview(ctx, "044-demo-legado")
	if e != nil {
		t.Fatal(e)
	}
	in := usecases.HCDMigrateActivateInput{MigrationID: m.ID, Revision: m.Revision, Quote: "apruebo", Client: "codex", Implementer: "dev"}
	bad := in
	bad.Quote = "mmm"
	if _, e = uc.MigrateActivate(ctx, bad); !errors.Is(e, domain.ErrInvalidQuote) {
		t.Fatalf("quote err=%v", e)
	}
	bad = in
	bad.Revision = "000000000000"
	if _, e = uc.MigrateActivate(ctx, bad); !errors.Is(e, domain.ErrRevisionConflict) {
		t.Fatalf("revision err=%v", e)
	}
	tui := in
	tui.Client = "tui"
	if _, e = uc.MigrateActivate(ctx, tui); !errors.Is(e, domain.ErrNotAllowed) {
		t.Fatalf("tui err=%v", e)
	}
	unit, e := uc.MigrateActivate(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	// C-21: un único enfoque operativo (HCD) y la unidad no queda verificada.
	if unit.Approach != "hcd" || unit.State != domain.StateDraft || unit.ApprovedRev != "" {
		t.Fatalf("unit=%+v", unit)
	}
	if got := approaches(t, uc); len(got) != 1 || got[unit.ID] != "hcd" {
		t.Fatalf("approaches=%v", got)
	}
	// Las fuentes se conservan intactas, como referencia legada.
	for name, body := range map[string]string{"spec.md": migrateSpec, "plan.md": migratePlan, "tasks.md": migrateTasks} {
		if content, _ := os.ReadFile(filepath.Join(dir, name)); string(content) != body {
			t.Fatalf("%s changed", name)
		}
	}
	if _, e = os.Stat(filepath.Join(root, "hcd", "features", "001-demo-legado", "definition.md")); e != nil {
		t.Fatal(e)
	}
	done, e := uc.MigratePreview(ctx, "044-demo-legado")
	if !errors.Is(e, domain.ErrAlreadyExists) || done.ID != "" {
		t.Fatalf("second preview=%+v err=%v", done, e)
	}
	if _, e = uc.MigrateActivate(ctx, in); !errors.Is(e, domain.ErrNotAllowed) {
		t.Fatalf("reactivation err=%v", e)
	}
}

func TestHCDMigrateActivateRequiresNewProposalWhenSourcesChange(t *testing.T) {
	uc, _, dir, _ := newMigrateFixture(t)
	ctx := context.Background()
	m, e := uc.MigratePreview(ctx, "044-demo-legado")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(migrateTasks+"- [ ] T003 Nueva tarea\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	in := usecases.HCDMigrateActivateInput{MigrationID: m.ID, Revision: m.Revision, Quote: "apruebo", Client: "codex", Implementer: "dev"}
	if _, e = uc.MigrateActivate(ctx, in); !errors.Is(e, domain.ErrRevisionConflict) {
		t.Fatalf("err=%v", e)
	}
	if got := approaches(t, uc); got["044-demo-legado"] != "speckit" {
		t.Fatalf("approaches=%v", got)
	}
	fresh, e := uc.MigratePreview(ctx, "044-demo-legado")
	if e != nil || fresh.ID != "M-02" || fresh.Revision == m.Revision {
		t.Fatalf("fresh=%+v err=%v", fresh, e)
	}
	if _, e = uc.MigrateActivate(ctx, in); !errors.Is(e, domain.ErrRevisionConflict) {
		t.Fatalf("stale proposal err=%v", e)
	}
	in.MigrationID, in.Revision = fresh.ID, fresh.Revision
	if _, e = uc.MigrateActivate(ctx, in); e != nil {
		t.Fatal(e)
	}
}

func TestHCDMigrateActivateFailureLeavesSingleRecoverableApproach(t *testing.T) {
	uc, root, _, db := newMigrateFixture(t)
	ctx := context.Background()
	m, e := uc.MigratePreview(ctx, "044-demo-legado")
	if e != nil {
		t.Fatal(e)
	}
	// Falla a mitad de la activación: la unidad, el enlace y la decisión ya se habrían escrito.
	if _, e = db.Exec(`CREATE TRIGGER boom BEFORE INSERT ON hcd_decisions BEGIN SELECT RAISE(ABORT, 'boom'); END`); e != nil {
		t.Fatal(e)
	}
	in := usecases.HCDMigrateActivateInput{MigrationID: m.ID, Revision: m.Revision, Quote: "apruebo", Client: "codex", Implementer: "dev"}
	if _, e = uc.MigrateActivate(ctx, in); e == nil {
		t.Fatal("activation must fail")
	}
	if got := approaches(t, uc); len(got) != 1 || got["044-demo-legado"] != "speckit" {
		t.Fatalf("partially migrated: %v", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "hcd", "features")); len(entries) != 0 {
		t.Fatalf("leftover artifacts: %v", entries)
	}
	var units int
	if e = db.QueryRow("SELECT count(*) FROM hcd_units").Scan(&units); e != nil || units != 0 {
		t.Fatalf("units=%d err=%v", units, e)
	}
	if got, e := uc.MigratePreview(ctx, "044-demo-legado"); e != nil || got.Status != domain.MigrationProposed {
		t.Fatalf("recovery preview=%+v err=%v", got, e)
	}
	// Recuperable: sin el fallo, una propuesta nueva se activa.
	if _, e = db.Exec(`DROP TRIGGER boom`); e != nil {
		t.Fatal(e)
	}
	in.MigrationID = "M-02"
	fresh, e := uc.MigratePreview(ctx, "044-demo-legado")
	if e != nil {
		t.Fatal(e)
	}
	in.MigrationID, in.Revision = fresh.ID, fresh.Revision
	if unit, e := uc.MigrateActivate(ctx, in); e != nil || unit.Approach != "hcd" {
		t.Fatalf("recovery unit=%+v err=%v", unit, e)
	}
}
