package hcd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mem/adapters/primary/cli/clikit"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/speckit"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC) }

func template(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infrastructure", "templates", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type env struct {
	t     *testing.T
	deps  *clikit.Deps
	root  string
	files string
	cur   Response
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := persistence.NewHCDStore(db, "project")
	artifacts, err := hcdfiles.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	memRepo := persistence.NewMemoryRepository(db)
	topics := memRepo.(ports.MemoryTopicQuerier)
	uc := usecases.NewHCD(store, store, artifacts, store, fixedClock{}, nil)
	uc.ConfigureSpecKitReader(speckit.Reader{})
	uc.ConfigureSubjectInspector(hcdfiles.SubjectInspector{Root: root}.Current)
	uc.ConfigureMemory(memRepo, persistence.NewRelationRepository(db))
	uc.ConfigureDeliveryCodec(hcdfiles.ParseDelivery, hcdfiles.RenderDelivery)
	uc.ConfigureEvidenceParser(hcdfiles.ParseEvidence)
	uc.ConfigureDefinitionParser(func(data []byte) (domain.Definition, error) { d, _, e := hcdfiles.ParseDefinition(data); return d, e })
	uc.ConfigureDocuments("project", topics, template(t, "hcd/playbook/templates/definition.md"), template(t, "hcd/hcd-playbook.md"), template(t, "speckit-constitution-gen.md"))
	return &env{t: t, deps: &clikit.Deps{HCD: uc}, root: root, files: t.TempDir()}
}

func (e *env) raw(args ...string) (int, string, string) {
	e.t.Helper()
	var out, errOut bytes.Buffer
	code := run(e.deps, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// call ejecuta un comando con --json y exige éxito; actualiza la revisión corriente.
func (e *env) call(command string, args ...string) Response {
	e.t.Helper()
	argv := []string{command}
	if command != "start" && command != "migrate" && command != "classify" {
		argv = append(argv, e.cur.UnitID)
	}
	argv = append(argv, args...)
	argv = append(argv, "--json", "--client", "codex", "--operation-id", "op-"+command+strconv.FormatInt(e.cur.Revision.State, 10), "--expected-revision", strconv.FormatInt(e.cur.Revision.State, 10))
	code, out, errOut := e.raw(argv...)
	var r Response
	if err := json.Unmarshal([]byte(out), &r); err != nil || !r.OK || code != 0 {
		e.t.Fatalf("%v: code=%d out=%s err=%s (%v)", argv, code, out, errOut, err)
	}
	if r.UnitID != "" {
		e.cur = r
	}
	return r
}

func (e *env) write(name string, content []byte) string {
	e.t.Helper()
	path := filepath.Join(e.files, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		e.t.Fatal(err)
	}
	return path
}

func TestExecuteFullFlowInProcess(t *testing.T) {
	e := newEnv(t)
	code := []byte("datos guardados\n")
	if err := os.WriteFile(filepath.Join(e.root, "result.txt"), code, 0o600); err != nil {
		t.Fatal(err)
	}
	start := e.call("start", "--kind", "fix", "--title", "Guardar datos", "--implementer", "dev")
	// La plantilla sin editar no se puede aprobar: validate la enumera y approve falla.
	if r := e.call("validate"); len(r.PendingChecks) == 0 {
		t.Fatal("la plantilla sin editar debe tener pendientes")
	}
	d := domain.Definition{ID: start.UnitID, Kind: domain.KindFix, Title: "Guardar datos", Implementer: "dev", Playbook: start.PlaybookRevision, Problem: "Falla al guardar", Actor: "usuario", ExpectedResult: "Datos guardados", Included: "Persistencia", Excluded: "Borrado", Restrictions: "Compatibilidad", ObservedFailure: "No guarda", Reproduction: "Guardar datos", RootCause: "pendiente", Stories: []domain.Story{{ID: "US1", Title: "Guardar", Actor: "usuario", Text: "Guardar datos"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "Guardado", Stories: []string{"US1"}, Condition: "Datos persistidos", Verification: domain.VerificationAutomated}}}
	defined := e.call("define", "--file", e.write("definition.md", hcdfiles.RenderDefinition(d)))
	if r := e.call("validate"); len(r.PendingChecks) != 0 {
		t.Fatalf("pending=%+v", r.PendingChecks)
	}
	e.call("approve", "--revision", defined.Revision.Definition, "--quote", "apruebo")
	delivery := domain.Delivery{Slices: []domain.Slice{{ID: "VS-01", Observable: "Guardar datos", Limits: "Sin borrar", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}}, Tasks: []domain.Task{{ID: "T-001", Title: "Guardar datos", SliceID: "VS-01", Type: "implementation", Stories: []string{"US1"}, Criteria: []string{"C-01"}, Result: "Datos guardados", Scope: "Persistencia", Excluded: "Borrado", Verification: "Test", Rules: []string{"HCD-TRACE-01"}}}}
	file := e.write("delivery.md", hcdfiles.RenderDelivery(start.UnitID, delivery))
	e.call("slice", "--file", file)
	e.call("tasks", "--slice", "VS-01", "--file", file)
	e.call("check", "--task", "T-001")
	run := e.call("run", "--task", "T-001", "--timeout", "2")
	sum := sha256.New()
	fmt.Fprintf(sum, "result.txt\x00%d\x00", len(code))
	sum.Write(code)
	subject := "tree:" + hex.EncodeToString(sum.Sum(nil))[:12] + " (archivos: result.txt)"
	evidence := fmt.Sprintf("Cubre: T-001, C-01\nResultado: pass\nProcedimiento: comprobar result.txt\nEntorno: prueba\nSujeto: %s\nEjecutor: codex\n", subject)
	e.call("report", "--run", run.ExecutionID, "--file", e.write("evidence.md", []byte(evidence)))
	e.call("verify")
	for _, q := range []string{"status", "next", "show", "list", "decision", "focus"} {
		args := []string{q, e.cur.UnitID, "--json", "--client", "codex"}
		if q == "decision" {
			args = append(args, "--expected-revision", strconv.FormatInt(e.cur.Revision.State, 10), "--file", e.write("decision.md", []byte("")))
			continue
		}
		if code, out, errOut := e.raw(args...); code != 0 {
			t.Fatalf("%s: code=%d out=%s err=%s", q, code, out, errOut)
		}
	}
	if code, out, _ := e.raw("status", e.cur.UnitID); code != 0 || out == "" {
		t.Fatalf("texto: code=%d out=%q", code, out)
	}
	closed := e.call("close", "--revision", defined.Revision.Definition, "--quote", "apruebo")
	if closed.State != "completed" {
		t.Fatalf("close=%+v", closed)
	}
}

func TestExecuteUsageErrors(t *testing.T) {
	e := newEnv(t)
	for name, args := range map[string][]string{
		"sin argumentos":      {},
		"comando desconocido": {"inexistente", "--json"},
		"flag desconocido":    {"start", "--nope", "--json"},
		"valor faltante":      {"start", "--title"},
		"dos unidades":        {"show", "a", "b", "--json"},
		"sin unidad activa":   {"show", "--json"},
		"cliente desconocido": {"start", "--kind", "fix", "--title", "x", "--client", "zzz", "--json"},
		"kind inválido":       {"start", "--kind", "otro", "--title", "x", "--json"},
		"playbook inválido":   {"playbook", "borrar", "HCD-001", "--json"},
		"sin expected":        {"define", "HCD-001", "--file", "x", "--json"},
		"migrate sin modo":    {"migrate", "044-x", "--json"},
		"migrate ambos":       {"migrate", "044-x", "--preview", "--activate", "--json"},
		"migrate inexistente": {"migrate", "--preview", "044-x", "--json"},
	} {
		code, out, errOut := e.raw(args...)
		if code == 0 {
			t.Errorf("%s: debería fallar (out=%s err=%s)", name, out, errOut)
		}
	}
	if _, err := os.Stat(filepath.Join(e.root, "hcd")); !os.IsNotExist(err) {
		t.Fatal("un error de uso no debe crear artefactos")
	}
	if code, _, _ := run0(); code != 1 {
		t.Fatalf("sin deps: code=%d", code)
	}
}

func run0() (int, string, string) {
	var out, errOut bytes.Buffer
	return run(&clikit.Deps{}, []string{"list", "--json"}, &out, &errOut), out.String(), errOut.String()
}

func TestExecuteMigrateInProcess(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.root, "specs", "044-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"spec.md": "# Feature Specification: Demo\n\n- **FR-001**: guardar\n", "tasks.md": "- [x] T001 hecho\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errOut := e.raw("migrate", "--preview", "044-demo", "--json", "--client", "codex")
	var preview struct {
		Data domain.Migration
	}
	if err := json.Unmarshal([]byte(out), &preview); code != 0 || err != nil || preview.Data.ID != "M-01" {
		t.Fatalf("preview code=%d out=%s err=%s (%v)", code, out, errOut, err)
	}
	if code, out, _ = e.raw("migrate", "--preview", "044-demo", "--client", "codex"); code != 0 || out == "" {
		t.Fatalf("preview texto: code=%d out=%q", code, out)
	}
	if code, _, _ = e.raw("migrate", "--activate", "M-02", "--revision", preview.Data.Revision, "--quote", "vale", "--client", "codex", "--json"); code == 0 {
		t.Fatal("una cita inválida no activa")
	}
	r := e.call("migrate", "--activate", "M-02", "--revision", preview.Data.Revision, "--quote", "apruebo", "--implementer", "dev")
	if r.Approach != "hcd" || r.State != "draft" {
		t.Fatalf("activate=%+v", r)
	}
	if code, out, _ = e.raw("playbook", "show", r.UnitID, "--json", "--client", "codex"); code != 0 {
		t.Fatalf("playbook show: %s", out)
	}
	if code, out, _ = e.raw("playbook", "validate", r.UnitID, "--json", "--client", "codex"); code != 0 {
		t.Fatalf("playbook validate: %s", out)
	}
}

// --file - lee de stdin: el agente pasa el contenido con un heredoc y no
// necesita un temporal, que Codex y OpenCode bloquean fuera del proyecto.
func TestExecuteFileFromStdin(t *testing.T) {
	e := newEnv(t)
	start := e.call("start", "--kind", "fix", "--title", "Guardar datos", "--implementer", "dev")
	d := domain.Definition{ID: start.UnitID, Kind: domain.KindFix, Title: "Guardar datos", Implementer: "dev", Playbook: start.PlaybookRevision, Problem: "Falla al guardar", Actor: "usuario", ExpectedResult: "Datos guardados", Included: "Persistencia", Excluded: "Borrado", Restrictions: "Compatibilidad", ObservedFailure: "No guarda", Reproduction: "Guardar datos", RootCause: "pendiente", Stories: []domain.Story{{ID: "US1", Title: "Guardar", Actor: "usuario", Text: "Guardar datos"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "Guardado", Stories: []string{"US1"}, Condition: "Datos persistidos", Verification: domain.VerificationAutomated}}}
	content := hcdfiles.RenderDefinition(d)
	old := stdin
	stdin = bytes.NewReader(content)
	t.Cleanup(func() { stdin = old })
	defined := e.call("define", "--file", "-")
	if defined.Revision.Definition != domain.ContentRevision(content) {
		t.Fatalf("definition rev=%s, want content from stdin", defined.Revision.Definition)
	}
	if r := e.call("validate"); len(r.PendingChecks) != 0 {
		t.Fatalf("pending=%+v", r.PendingChecks)
	}
}

// Una fila guardada antes del saneado al escribir puede traer controles de
// terminal en el título: list no debe emitirlos (OSC 0 cambia el título de la
// ventana, OSC 52 escribe el portapapeles).
func TestListStripsTerminalControlsFromStoredTitles(t *testing.T) {
	e := newEnv(t)
	e.call("start", "--kind", "fix", "--title", "Guardar datos", "--implementer", "dev")
	db, err := persistence.Init(e.root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("UPDATE hcd_units SET title=?", "Guardar \x1b]52;c;eA==\x07datos\x9b"); err != nil {
		t.Fatal(err)
	}
	code, out, _ := e.raw("list")
	if code != 0 || strings.ContainsAny(out, "\x1b\x07\u009b") || !strings.Contains(out, "Guardar ]52;c;eA==datos") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
