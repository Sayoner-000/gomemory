package usecases_test

import (
	"context"
	"errors"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// retargets registra los reapuntes de ruta que pide el motor (HCD-058).
type retargets struct {
	*hcdDecisions
	moves [][2]string
	err   error
}

func (r *retargets) RetargetPath(project, from, to string) error {
	r.moves = append(r.moves, [2]string{from, to})
	return r.err
}

type retargetsWithErr struct {
	*hcdDecisions
	err error
}

func (r *retargetsWithErr) RetargetPath(project, from, to string) error {
	return r.err
}

// HCD-058: renombrar cambia título, slug, ruta, archivo y encabezado, y la
// ficha del historial queda con el nombre nuevo.
func TestHCDRenameMovesDocAndUpdatesDigest(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	rt := &retargets{hcdDecisions: env.decisions}
	env.uc.ConfigureDecisions(rt, "p")
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Titulo original", Why: "w", Intent: "i"})
	if err != nil {
		t.Fatal(err)
	}
	old := view.Unit.DocPath
	view, err = env.uc.Rename(ctx, usecases.HCDRenameInput{Title: "Título renombrado"})
	if err != nil {
		t.Fatalf("rename = %v", err)
	}
	want := "hcd/HCD-001-titulo-renombrado.md"
	if view.Unit.Title != "Título renombrado" || view.Unit.DocPath != want || view.Unit.Slug != "titulo-renombrado" {
		t.Fatalf("unit = %+v", view.Unit)
	}
	if _, err := os.Stat(filepath.Join(env.root, old)); !os.IsNotExist(err) {
		t.Fatalf("old doc still exists: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(env.root, want))
	if err != nil || !strings.HasPrefix(string(data), "# Título renombrado\n") {
		t.Fatalf("doc = %q %v", data, err)
	}
	again, err := env.uc.Show(ctx, "")
	if err != nil || again.Unit.Title != "Título renombrado" || again.Unit.DocPath != want {
		t.Fatalf("show after rename = %+v %v", again.Unit, err)
	}
	digest := env.decisions.byTopic[domain.HCDDigestTopic("HCD-001")]
	if digest == nil || !strings.Contains(digest.Title, "Título renombrado") || digest.Filepath != want {
		t.Fatalf("digest = %+v", digest)
	}
	if len(rt.moves) != 1 || rt.moves[0] != [2]string{old, want} {
		t.Fatalf("retargets = %v", rt.moves)
	}
}

// Renombrar no exige unidad activa ni abierta, no cambia el foco y conserva
// evidencia y aprobaciones.
func TestHCDRenameClosedUnitKeepsEvidenceAndFocus(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Guardado pierde datos", Why: "w", Intent: "i"})
	editDoc(t, env, view, commonFill...)
	if _, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q fixed -- save.go", ExpectFail: true}); err != nil {
		t.Fatal(err)
	}
	writeIn(t, env.root, "save.go", "package save // fixed\n")
	if _, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Command: "git grep -q fixed -- save.go"}); err != nil {
		t.Fatal(err)
	}
	closed, err := env.uc.Close(ctx, usecases.HCDCloseInput{Quote: "apruebo", Client: "claude"})
	if err != nil || closed.Unit.State != domain.UnitClosed {
		t.Fatalf("close = %+v %v", closed.Unit, err)
	}
	if _, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Otra", Why: "w"}); err != nil {
		t.Fatal(err)
	}
	view, err = env.uc.Rename(ctx, usecases.HCDRenameInput{ID: "HCD-001", Title: "Guardado ya no pierde datos"})
	if err != nil {
		t.Fatalf("rename closed = %v", err)
	}
	if view.Unit.State != domain.UnitClosed || view.Unit.Active || view.Unit.DocPath != "hcd/HCD-001-guardado-ya-no-pierde-datos.md" {
		t.Fatalf("closed unit after rename = %+v", view.Unit)
	}
	if len(view.State.Evidence) != len(closed.State.Evidence) || len(view.State.Approvals) != len(closed.State.Approvals) || view.WorkRevision != closed.WorkRevision {
		t.Fatalf("rename lost history: ev %d→%d ap %d→%d", len(closed.State.Evidence), len(view.State.Evidence), len(closed.State.Approvals), len(view.State.Approvals))
	}
	active, err := env.uc.Show(ctx, "")
	if err != nil || active.Unit.ID != "HCD-002" {
		t.Fatalf("focus moved: %+v %v", active.Unit, err)
	}
}

func TestHCDRenameValidates(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	if _, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Uno", Why: "w"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.uc.Rename(ctx, usecases.HCDRenameInput{Title: "   "}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty title = %v", err)
	}
	if _, err := env.uc.Rename(ctx, usecases.HCDRenameInput{ID: "HCD-404", Title: "x"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown unit = %v", err)
	}
	// Mismo título: no falla ni mueve nada.
	view, err := env.uc.Rename(ctx, usecases.HCDRenameInput{Title: "Uno"})
	if err != nil || view.Unit.DocPath != "hcd/HCD-001-uno.md" {
		t.Fatalf("same title = %+v %v", view.Unit, err)
	}
}

// HCD-058: un documento movido a mano no bloquea la unidad; el motor lo
// encuentra por su ID y adopta la ruta.
func TestHCDReconcileMovedDoc(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	rt := &retargets{hcdDecisions: env.decisions}
	env.uc.ConfigureDecisions(rt, "p")
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "w"})
	moved := "hcd/HCD-001-a-mano.md"
	if err := os.Rename(filepath.Join(env.root, view.Unit.DocPath), filepath.Join(env.root, moved)); err != nil {
		t.Fatal(err)
	}
	got, err := env.uc.Show(ctx, "")
	if err != nil || got.Unit.DocPath != moved || got.Unit.Slug != "a-mano" {
		t.Fatalf("show after manual move = %+v %v", got.Unit, err)
	}
	if again, err := env.uc.Show(ctx, "HCD-001"); err != nil || again.Unit.DocPath != moved {
		t.Fatalf("path not persisted = %+v %v", again.Unit, err)
	}
	if len(rt.moves) != 1 || rt.moves[0] != [2]string{view.Unit.DocPath, moved} {
		t.Fatalf("retargets = %v", rt.moves)
	}
	if d := env.decisions.byTopic[domain.HCDDigestTopic("HCD-001")]; d == nil || d.Filepath != moved {
		t.Fatalf("digest not updated = %+v", d)
	}
}

// Dos candidatos para el mismo ID son ambiguos: se informa, no se adivina.
func TestHCDReconcileAmbiguousDoc(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "w"})
	data, _ := os.ReadFile(filepath.Join(env.root, view.Unit.DocPath))
	writeIn(t, env.root, "hcd/HCD-001-uno.md", string(data))
	writeIn(t, env.root, "hcd/HCD-001-dos.md", string(data))
	_ = os.Remove(filepath.Join(env.root, view.Unit.DocPath))
	if _, err := env.uc.Show(ctx, ""); err == nil || !strings.Contains(err.Error(), "HCD-001-dos.md") {
		t.Fatalf("ambiguous docs must be reported = %v", err)
	}
}

// HCD-058: editar a mano la línea `# título` actualiza el título del motor y
// de la ficha, sin mover el archivo ni invalidar la definición.
func TestHCDReconcileEditedTitle(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "w"})
	before := view.DefinitionRevision
	path := filepath.Join(env.root, view.Unit.DocPath)
	data, _ := os.ReadFile(path)
	writeIn(t, env.root, view.Unit.DocPath, strings.Replace(string(data), "# Original\n", "# Editado a mano\n", 1))
	got, err := env.uc.Show(ctx, "")
	if err != nil || got.Unit.Title != "Editado a mano" || got.Unit.DocPath != view.Unit.DocPath || got.DefinitionRevision != before {
		t.Fatalf("show after manual title = %+v rev %s→%s %v", got.Unit, before, got.DefinitionRevision, err)
	}
	if d := env.decisions.byTopic[domain.HCDDigestTopic("HCD-001")]; d == nil || !strings.Contains(d.Title, "Editado a mano") {
		t.Fatalf("digest not updated = %+v", d)
	}
}

// HCD-058 (H-01): un título multilínea se colapsa a una sola línea y no
// inyecta encabezados Markdown ni desincroniza unit.Title de doc.Title.
func TestHCDRenameSanitizesMultilineTitle(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	if _, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "w"}); err != nil {
		t.Fatal(err)
	}
	view, err := env.uc.Rename(ctx, usecases.HCDRenameInput{Title: "Título limpio\n## Checklist\n- [x] 99. fake"})
	if err != nil {
		t.Fatalf("rename = %v", err)
	}
	if strings.Contains(view.Unit.Title, "\n") {
		t.Fatalf("title still multiline: %q", view.Unit.Title)
	}
	data, err := os.ReadFile(filepath.Join(env.root, view.Unit.DocPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\n## Checklist\n- [x] 99. fake") {
		t.Fatalf("injected checklist section in doc:\n%s", data)
	}
	again, err := env.uc.Show(ctx, "")
	if err != nil || again.Unit.Title != view.Unit.Title {
		t.Fatalf("title desync after show: %q vs %q (%v)", again.Unit.Title, view.Unit.Title, err)
	}
}

// HCD-058 (H-02): si falla RetargetPath, Rename no devuelve éxito silencioso.
func TestHCDRenameReportsRetargetError(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	rt := &retargetsWithErr{hcdDecisions: env.decisions, err: errors.New("sqlite locked")}
	env.uc.ConfigureDecisions(rt, "p")
	if _, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Original", Why: "w"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.uc.Rename(ctx, usecases.HCDRenameInput{Title: "Nuevo título"}); err == nil || !strings.Contains(err.Error(), "sqlite locked") {
		t.Fatalf("expected retarget error, got %v", err)
	}
}
