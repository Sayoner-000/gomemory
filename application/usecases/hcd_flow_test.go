package usecases_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type hcdFixedClock struct{ at time.Time }

func (c hcdFixedClock) Now() time.Time { return c.at }

func newHCDFlow(t *testing.T) (*usecases.HCD, *persistence.HCDStore, *sql.DB, ports.MemoryTopicQuerier, string) {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := persistence.NewHCDStore(db, "project")
	files, err := hcdfiles.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	mem := persistence.NewMemoryRepository(db)
	topics, ok := mem.(ports.MemoryTopicQuerier)
	if !ok {
		t.Fatal("memory repository lacks pinned doc lookup")
	}
	counter := 0
	uc := usecases.NewHCD(store, store, files, store, hcdFixedClock{at: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)}, func() string { counter++; return fmt.Sprintf("op-%d", counter) })
	uc.ConfigureDeliveryCodec(hcdfiles.ParseDelivery, hcdfiles.RenderDelivery)
	uc.ConfigureDefinitionParser(func(data []byte) (domain.Definition, error) { d, _, e := hcdfiles.ParseDefinition(data); return d, e })
	uc.ConfigureDocuments("project", topics, "# {Título}\n\nID: HCD-001\nTipo: fix\nImplementador: {persona}\nPlaybook: {revisión}\n", "PLAYBOOK DEFAULT", "CONSTITUTION DEFAULT")
	return uc, store, db, topics, root
}

func TestHCDStartCreatesUnitArtifactNormativeRevisionsAndFocus(t *testing.T) {
	uc, store, db, _, root := newHCDFlow(t)
	unit, err := uc.Start(context.Background(), usecases.HCDStartInput{Kind: domain.KindFix, Title: "Guardar datos", Implementer: "dev", Client: "claude", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	if unit.ID != "HCD-001" || unit.State != domain.StateDraft || unit.Approach != "hcd" || unit.PlaybookRev == "" || unit.ConstitutionRev == "" {
		t.Fatalf("unit=%+v", unit)
	}
	path := filepath.Join(root, "hcd", "features", "001-guardar-datos", "definition.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if domain.ContentRevision(content) != unit.DefinitionRev {
		t.Fatalf("definition revision mismatch: %s != %s", domain.ContentRevision(content), unit.DefinitionRev)
	}
	got, err := store.GetUnit(context.Background(), unit.ID)
	if err != nil || got.ID != unit.ID {
		t.Fatalf("GetUnit=%+v err=%v", got, err)
	}
	focus, err := store.GetFocus(context.Background())
	if err != nil || focus.UnitID != unit.ID {
		t.Fatalf("focus=%+v err=%v", focus, err)
	}
	var playbook, constitution int
	if err := db.QueryRow("SELECT count(*) FROM hcd_normative_revisions WHERE project=? AND doc=?", "project", "playbook").Scan(&playbook); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT count(*) FROM hcd_normative_revisions WHERE project=? AND doc=?", "project", "constitution").Scan(&constitution); err != nil {
		t.Fatal(err)
	}
	if playbook != 1 || constitution != 1 {
		t.Fatalf("normative revisions: playbook=%d constitution=%d", playbook, constitution)
	}
}

func TestHCDDefineRevisesContentAndValidateIsReadOnly(t *testing.T) {
	uc, store, _, _, root := newHCDFlow(t)
	unit, err := uc.Start(context.Background(), usecases.HCDStartInput{Kind: domain.KindFix, Title: "Guardar datos", Implementer: "dev", Client: "claude", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	definition := domain.Definition{Title: "Guardar datos", Stories: []domain.Story{{ID: "US1", Title: "guardar"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "se guarda", Stories: []string{"US1"}}}}
	content := []byte("# Guardar datos\n\ncontenido actualizado\n")
	updated, err := uc.Define(context.Background(), unit.ID, unit.StateRevision, "op-define", definition, content)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DefinitionRev != domain.ContentRevision(content) || updated.StateRevision != unit.StateRevision+1 || updated.State != domain.StateDraft {
		t.Fatalf("updated = %+v", updated)
	}
	before := updated.StateRevision
	checks := uc.Validate(definition, domain.KindFix)
	if len(checks) == 0 {
		t.Fatal("incomplete definition should have pending checks")
	}
	after, err := store.GetUnit(context.Background(), unit.ID)
	if err != nil || after.StateRevision != before {
		t.Fatalf("Validate changed revision: %+v %v", after, err)
	}
	replayed, err := uc.Define(context.Background(), unit.ID, unit.StateRevision, "op-define", definition, content)
	if err != nil || replayed.StateRevision != updated.StateRevision {
		t.Fatalf("idempotent replay = %+v, %v", replayed, err)
	}
	if err := os.WriteFile(filepath.Join(root, "hcd", "features", "001-guardar-datos", "definition.md"), []byte("external edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Define(context.Background(), unit.ID, updated.StateRevision, "op-new", definition, []byte("new")); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("external edit should conflict: %v", err)
	}
}

func TestHCDDefineAfterReadyRecordsScopeChangeDecision(t *testing.T) {
	uc, store, db, _, _ := newHCDFlow(t)
	unit, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	unit.State, unit.StateRevision = domain.StateReady, unit.StateRevision+1
	if err := store.UpdateUnit(context.Background(), unit, unit.StateRevision-1); err != nil {
		t.Fatal(err)
	}
	definition := domain.Definition{Title: "Nuevo alcance", Stories: []domain.Story{{ID: "US1"}}, Criteria: []domain.Criterion{{ID: "C-01", Stories: []string{"US1"}}}}
	if _, err := uc.Define(context.Background(), unit.ID, unit.StateRevision, "scope-change", definition, []byte("# Nuevo alcance\n")); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM hcd_decisions WHERE project=? AND unit_id=? AND kind=?", "project", unit.ID, "scope_change").Scan(&count); err != nil || count != 1 {
		t.Fatalf("scope change decisions=%d err=%v", count, err)
	}
}

func TestHCDApproveRequiresExactValidatedRevisionAndQuote(t *testing.T) {
	uc, store, db, _, _ := newHCDFlow(t)
	unit, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	definition := domain.Definition{ID: unit.ID, Kind: domain.KindFix, Title: unit.Title, Implementer: unit.Implementer, Playbook: unit.PlaybookRev, Problem: "falla", Actor: "usuario", ExpectedResult: "resultado", Included: "alcance", Excluded: "fuera", Restrictions: "compatibilidad", ObservedFailure: "error", Reproduction: "reproducir", RootCause: "pendiente", Stories: []domain.Story{{ID: "US1", Title: "guardar", Actor: "usuario", Text: "Como usuario quiero guardar"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "queda guardado", Stories: []string{"US1"}, Condition: "se guarda", Verification: domain.VerificationAutomated}}}
	content := []byte("definition revisada")
	unit, err = uc.Define(context.Background(), unit.ID, unit.StateRevision, "define-for-approval", definition, content)
	if err != nil {
		t.Fatal(err)
	}
	if checks := uc.Validate(definition, domain.KindFix); len(checks) != 0 {
		t.Fatalf("definition checks=%+v", checks)
	}
	approved, err := uc.Approve(context.Background(), usecases.HCDApproveInput{UnitID: unit.ID, ExpectedRevision: unit.StateRevision, OperationID: "approve", Object: "definition", Revision: unit.DefinitionRev, Quote: "apruebo", Client: "claude", Definition: definition})
	if err != nil {
		t.Fatal(err)
	}
	if approved.State != domain.StateReady || approved.ApprovedRev != unit.DefinitionRev {
		t.Fatalf("approved=%+v", approved)
	}
	if _, err := store.GetUnit(context.Background(), unit.ID); err != nil {
		t.Fatal(err)
	}
	var origin, quote string
	if err := db.QueryRow("SELECT origin,quote FROM hcd_approvals WHERE project=? AND unit_id=?", "project", unit.ID).Scan(&origin, &quote); err != nil || origin != "conversation" || quote != "apruebo" {
		t.Fatalf("approval origin=%q quote=%q err=%v", origin, quote, err)
	}
	if _, err := uc.Approve(context.Background(), usecases.HCDApproveInput{UnitID: unit.ID, ExpectedRevision: approved.StateRevision, OperationID: "approve-again", Object: "definition", Revision: approved.DefinitionRev, Quote: "apruebo", Client: "claude", Definition: definition}); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("repeat approval = %v", err)
	}
}

func TestHCDApprovalPromptIsRevisionBoundAndSingleUse(t *testing.T) {
	uc, _, db, _, _ := newHCDFlow(t)
	u, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	d := validApprovalDefinition(u)
	u, err = uc.Define(context.Background(), u.ID, u.StateRevision, "define-prompt", d, []byte("prompt definition"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := uc.RequestApproval(context.Background(), u.ID, u.StateRevision, "definition", u.DefinitionRev, d)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := uc.Approve(context.Background(), usecases.HCDApproveInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "approve-prompt", Object: "definition", Revision: u.DefinitionRev, Quote: "A", Client: "terminal", PromptID: p.ID, Definition: d})
	if err != nil {
		t.Fatal(err)
	}
	if approved.State != domain.StateReady {
		t.Fatalf("approved=%+v", approved)
	}
	var origin string
	if err := db.QueryRow("SELECT origin FROM hcd_approvals WHERE project=? AND unit_id=?", "project", u.ID).Scan(&origin); err != nil || origin != "terminal" {
		t.Fatalf("origin=%q err=%v", origin, err)
	}
	if _, err := uc.Approve(context.Background(), usecases.HCDApproveInput{UnitID: u.ID, ExpectedRevision: approved.StateRevision, OperationID: "approve-prompt-replay", Object: "definition", Revision: u.DefinitionRev, Quote: "recommended", Client: "terminal", PromptID: p.ID, Definition: d}); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("already approved = %v", err)
	}
	var consumed int
	if err := db.QueryRow("SELECT consumed FROM hcd_approval_prompts WHERE project=? AND id=?", "project", p.ID).Scan(&consumed); err != nil || consumed != 1 {
		t.Fatalf("consumed=%d err=%v", consumed, err)
	}
}

func validApprovalDefinition(u domain.Unit) domain.Definition {
	return domain.Definition{ID: u.ID, Kind: u.Kind, Title: u.Title, Implementer: u.Implementer, Playbook: u.PlaybookRev, Problem: "falla", Actor: "usuario", ExpectedResult: "resultado", Included: "alcance", Excluded: "fuera", Restrictions: "compatibilidad", ObservedFailure: "error", Reproduction: "reproducir", RootCause: "pendiente", Stories: []domain.Story{{ID: "US1", Title: "guardar", Actor: "usuario", Text: "Como usuario quiero guardar"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "queda guardado", Stories: []string{"US1"}, Condition: "se guarda", Verification: domain.VerificationAutomated}}}
}

func TestHCDApprovalBlockedByBlockingClarification(t *testing.T) {
	uc, store, _, _, _ := newHCDFlow(t)
	u, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	d := validApprovalDefinition(u)
	u, err = uc.Define(context.Background(), u.ID, u.StateRevision, "define-blocked", d, []byte("blocked definition"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.Clarify(context.Background(), usecases.HCDClarifyInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "open-block", Action: "open", TargetRef: "C-01", Question: "¿Cuál?", Blocking: true})
	if err != nil {
		t.Fatal(err)
	}
	u, err = store.GetUnit(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = uc.Approve(context.Background(), usecases.HCDApproveInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "approve-blocked", Object: "definition", Revision: u.DefinitionRev, Quote: "apruebo", Client: "claude", Definition: d}); !errors.Is(err, domain.ErrBlocked) {
		t.Fatalf("approve with open blocker = %v", err)
	}
	checks, err := uc.ValidateUnit(context.Background(), u.ID, d)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range checks {
		if c.Code == "CLARIFICATION-OPEN" {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks=%+v", checks)
	}
}

func TestHCDDefineAfterApprovalInvalidatesApproval(t *testing.T) {
	uc, _, _, _, _ := newHCDFlow(t)
	u, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	d := validApprovalDefinition(u)
	u, err = uc.Define(context.Background(), u.ID, u.StateRevision, "define-before-approve", d, []byte("before approval"))
	if err != nil {
		t.Fatal(err)
	}
	u, err = uc.Approve(context.Background(), usecases.HCDApproveInput{UnitID: u.ID, ExpectedRevision: u.StateRevision, OperationID: "approve-before-define", Object: "definition", Revision: u.DefinitionRev, Quote: "apruebo", Client: "claude", Definition: d})
	if err != nil {
		t.Fatal(err)
	}
	d.Title = "Alcance cambiado"
	u, err = uc.Define(context.Background(), u.ID, u.StateRevision, "define-after-approve", d, []byte("after approval"))
	if err != nil {
		t.Fatal(err)
	}
	if u.State != domain.StateDraft || u.ApprovedRev != "" || u.DefinitionRev == "" {
		t.Fatalf("old approval still authorizes: %+v", u)
	}
}

func TestHCDQueriesAndResumeDoNotChangeStateRevision(t *testing.T) {
	uc, store, _, _, _ := newHCDFlow(t)
	u, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	before := u.StateRevision
	if _, err = uc.Show(context.Background(), u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = uc.Next(context.Background(), u.ID); err != nil {
		t.Fatal(err)
	}
	view, err := uc.Resume(context.Background(), u.ID, "claude", "session-resume")
	if err != nil {
		t.Fatal(err)
	}
	if view.Unit.StateRevision != before {
		t.Fatalf("resume view revision=%d want=%d", view.Unit.StateRevision, before)
	}
	got, err := store.GetUnit(context.Background(), u.ID)
	if err != nil || got.StateRevision != before {
		t.Fatalf("query mutated revision: %+v err=%v", got, err)
	}
	focus, err := store.GetFocus(context.Background())
	if err != nil || focus.UnitID != u.ID || focus.SetBy != "resume" {
		t.Fatalf("resume focus=%+v err=%v", focus, err)
	}
}

func TestHCDStartMultipleUnitsHasUniqueDecisionIDs(t *testing.T) {
	uc, _, db, _, _ := newHCDFlow(t)
	for i := 0; i < 2; i++ {
		if _, e := uc.Start(context.Background(), HCDStartInputForTest()); e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if e := db.QueryRow("SELECT count(*) FROM hcd_decisions").Scan(&count); e != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, e)
	}
}

// Dos Start concurrentes calculaban el mismo MAX(id)+1: el perdedor borraba
// el definition.md de la unidad que sí se guardó. Cada unidad persistida debe
// conservar su propio artefacto, y un perdedor no deja rastros.
func TestHCDStartConcurrentKeepsEveryPersistedArtifact(t *testing.T) {
	uc, _, db, _, root := newHCDFlow(t)
	ctx := context.Background()
	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = uc.Start(ctx, usecases.HCDStartInput{Kind: domain.KindFix, Title: "Guardar datos", Client: "claude", SessionID: "s"})
		}()
	}
	wg.Wait()
	rows, err := db.Query("SELECT id, slug FROM hcd_units")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	units := 0
	for rows.Next() {
		var id, slug string
		if err := rows.Scan(&id, &slug); err != nil {
			t.Fatal(err)
		}
		units++
		content, err := os.ReadFile(filepath.Join(root, "hcd", "features", strings.TrimPrefix(id, "HCD-")+"-"+slug, "definition.md"))
		if err != nil || !strings.Contains(string(content), "ID: "+id) {
			t.Fatalf("unidad %s sin su artefacto: %q %v", id, content, err)
		}
	}
	if units == 0 {
		t.Fatal("ningún Start concurrente terminó")
	}
	artifacts, err := filepath.Glob(filepath.Join(root, "hcd", "features", "*", "definition.md"))
	if err != nil || len(artifacts) != units {
		t.Fatalf("artefactos=%d unidades=%d err=%v", len(artifacts), units, err)
	}
}

// El implementador declarado en definition.md debe llegar al estado de control;
// antes solo se copiaba el título y la unidad quedaba con el valor de start.
func TestHCDDefineRecordsImplementerFromDefinition(t *testing.T) {
	uc, store, _, _, _ := newHCDFlow(t)
	ctx := context.Background()
	unit, err := uc.Start(ctx, usecases.HCDStartInput{Kind: domain.KindFix, Title: "Guardar datos", Client: "claude", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	definition := domain.Definition{Title: "Guardar datos", Implementer: "Jose"}
	if _, err = uc.Define(ctx, unit.ID, unit.StateRevision, "op-define", definition, []byte("# Guardar datos\n")); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetUnit(ctx, unit.ID)
	if err != nil || got.Implementer != "Jose" {
		t.Fatalf("implementer = %q, %v", got.Implementer, err)
	}
}

// ACR C-005: borrar definition.md dejaba la unidad atascada: status fallaba y
// toda mutación, incluido el define que lo repone, devolvía un error crudo.
func TestHCDMissingDefinitionIsReconciledAndRecoverable(t *testing.T) {
	uc, store, _, _, root := newHCDFlow(t)
	ctx := context.Background()
	unit, err := uc.Start(ctx, HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "hcd", "features", "001-guardar-datos", "definition.md")); err != nil {
		t.Fatal(err)
	}
	if _, err = uc.Show(ctx, unit.ID); err != nil {
		t.Fatalf("show without definition.md: %v", err)
	}
	definition := domain.Definition{Title: "Guardar datos", Implementer: "dev"}
	content := []byte("# Guardar datos\n")
	if _, err = uc.Define(ctx, unit.ID, unit.StateRevision, "op-restore", definition, content); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("first mutation should reconcile with revision_conflict: %v", err)
	}
	current, err := store.GetUnit(ctx, unit.ID)
	if err != nil || current.DefinitionRev != "" || current.State != domain.StateDraft {
		t.Fatalf("reconciled unit=%+v %v", current, err)
	}
	restored, err := uc.Define(ctx, unit.ID, current.StateRevision, "op-restore-2", definition, content)
	if err != nil || restored.DefinitionRev != domain.ContentRevision(content) {
		t.Fatalf("restore=%+v %v", restored, err)
	}
}

// Título e implementador llegan de argumentos o de definition.md: lo privado,
// los secretos y los controles de terminal no deben quedar en SQLite, en el
// artefacto ni en el nombre de la carpeta.
func TestHCDStartAndDefineProtectUnitFields(t *testing.T) {
	uc, _, db, _, root := newHCDFlow(t)
	ctx := context.Background()
	const secret = "valor-sintetico-42"
	unit, err := uc.Start(ctx, usecases.HCDStartInput{Kind: domain.KindFix, Title: "Pagar <private>" + secret + "</private> \x1b]0;pwn\x07ya", Implementer: "<private>" + secret + "</private>", Client: "claude", SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(unit.Title+unit.Implementer+unit.Slug, secret) || strings.ContainsRune(unit.Title, 0x1b) {
		t.Fatalf("unidad sin proteger: %+v", unit)
	}
	definition := domain.Definition{Title: "Otro <private>" + secret + "</private>\x1b]52;c;eA==\x07", Implementer: "<private>" + secret + "</private>"}
	if _, err = uc.Define(ctx, unit.ID, unit.StateRevision, "op-define", definition, []byte("# Otro\n")); err != nil {
		t.Fatal(err)
	}
	var title, implementer string
	if err = db.QueryRow("SELECT title, implementer FROM hcd_units WHERE id=?", unit.ID).Scan(&title, &implementer); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(title+implementer, secret) || strings.ContainsRune(title, 0x1b) {
		t.Fatalf("SQLite conserva el dato: %q %q", title, implementer)
	}
	err = filepath.WalkDir(filepath.Join(root, "hcd"), func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if strings.Contains(p, secret) {
			t.Fatalf("ruta con el dato: %s", p)
		}
		if !d.IsDir() {
			if data, _ := os.ReadFile(p); strings.Contains(string(data), secret) || strings.ContainsRune(string(data), 0x1b) {
				t.Fatalf("artefacto con el dato: %s", p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// cancelAfterWrite cancela el contexto de la operación en cuanto el artefacto
// queda escrito: el commit falla por cancelación con el archivo ya en disco.
type cancelAfterWrite struct {
	ports.HCDArtifactRepository
	cancel context.CancelFunc
}

func (c cancelAfterWrite) Write(ctx context.Context, dir, name string, data []byte) error {
	err := c.HCDArtifactRepository.Write(ctx, dir, name, data)
	c.cancel()
	return err
}

// La compensación no puede reutilizar el contexto cancelado: Delete retornaría
// sin borrar y quedaría un definition.md sin unidad en SQLite.
func TestHCDStartCancelledAfterWriteLeavesNoArtifact(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := persistence.NewHCDStore(db, "project")
	files, err := hcdfiles.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	uc := usecases.NewHCD(store, store, cancelAfterWrite{files, cancel}, store, hcdFixedClock{at: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)}, func() string { return "op" })
	uc.ConfigureDocuments("project", nil, "# {Título}\n\nID: HCD-001\n", "PLAYBOOK DEFAULT", "CONSTITUTION DEFAULT")
	if _, err = uc.Start(ctx, HCDStartInputForTest()); err == nil {
		t.Fatal("se esperaba el fallo por cancelación")
	}
	var units int
	if err = db.QueryRow("SELECT count(*) FROM hcd_units").Scan(&units); err != nil || units != 0 {
		t.Fatalf("units=%d err=%v", units, err)
	}
	if left, _ := filepath.Glob(filepath.Join(root, "hcd", "features", "*", "definition.md")); len(left) != 0 {
		t.Fatalf("artefacto huérfano: %v", left)
	}
}
