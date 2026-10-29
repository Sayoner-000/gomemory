package main

import (
	"encoding/json"
	hcdcli "mem/adapters/primary/cli/hcd"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// hcdBinary conduce el motor real `mem hcd` sobre un repo git temporal.
type hcdBinary struct {
	t          *testing.T
	bin, root  string
	home, data string
}

func newHCDBinary(t *testing.T) hcdBinary {
	t.Helper()
	h := hcdBinary{t: t, bin: buildMemBinary(t), root: t.TempDir(), home: t.TempDir(), data: t.TempDir()}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		h.git(args...)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "README.md", "save.go"} {
		h.write(name, "contenido previo que HCD debe preservar\n")
	}
	h.git("add", ".")
	h.git("commit", "-qm", "init")
	return h
}

func (h hcdBinary) git(args ...string) {
	h.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = h.root
	if out, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (h hcdBinary) write(rel, body string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.root, rel), []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

// edit aplica pares de reemplazo al documento, como haría el agente.
func (h hcdBinary) edit(doc string, pairs ...string) {
	h.t.Helper()
	path := filepath.Join(h.root, doc)
	data, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatal(err)
	}
	out := strings.NewReplacer(pairs...).Replace(string(data))
	if out == string(data) {
		h.t.Fatalf("edit did not change %s:\n%s", doc, data)
	}
	h.write(doc, out)
}

// run invoca `mem hcd`; --json y --client van antes de «--».
func (h hcdBinary) run(wantOK bool, command string, args ...string) hcdcli.Response {
	h.t.Helper()
	argv := append([]string{"hcd", command, "--json", "--client", "codex"}, args...)
	cmd := exec.Command(h.bin, argv...)
	cmd.Dir = h.root
	cmd.Env = append(os.Environ(), "HOME="+h.home, "USERPROFILE="+h.home, "GOMEMORY_DATA_HOME="+h.data, "GOMEMORY_NO_CODE_REFRESH=1")
	out, _ := cmd.Output()
	var r hcdcli.Response
	if err := json.Unmarshal(out, &r); err != nil {
		h.t.Fatalf("mem %v: %v\n%s", argv, err, out)
	}
	if r.OK != wantOK {
		h.t.Fatalf("mem %v: ok=%v, want %v: %s", argv, r.OK, wantOK, out)
	}
	return r
}

func (h hcdBinary) expect(r hcdcli.Response, stage, skill string) {
	h.t.Helper()
	if r.Stage != stage || r.NextAction.Skill != skill {
		h.t.Fatalf("stage = %s (%s), want %s/%s: %+v", r.Stage, r.NextAction.Skill, stage, skill, r)
	}
}

var hcdCommonFill = []string{
	"## Reproducción\npendiente", "## Reproducción\ngrep -q fixed save.go",
	"Incluido: pendiente", "Incluido: guardado",
	"Excluido: pendiente", "Excluido: exportación",
	"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nse pierden datos",
	"## Alternativa más barata\npendiente", "## Alternativa más barata\nninguna",
}

// SC-002: un fix por la Vía rápida con rojo, verde y una sola aprobación.
func TestHCDBinaryFastFix(t *testing.T) {
	h := newHCDBinary(t)
	before := hcdProjectSnapshot(t, h.root)
	r := h.run(true, "start", "--kind", "fix", "--lane", "fast", "--title", "Guardado pierde datos", "--why", "fix claro", "--intent", "El guardado pierde datos")
	h.expect(r, "define", "hcd-define")
	doc := r.DocPath
	h.edit(doc, hcdCommonFill...)
	h.expect(h.run(true, "show"), "implement", "hcd-implement")
	if r = h.run(false, "approve", "--quote", "apruebo"); r.Error.Code != "not_allowed" {
		t.Fatalf("approve in fast lane = %+v", r.Error)
	}
	r = h.run(true, "verify", "--task", "1", "--expect-fail", "--", "grep -q fixed save.go")
	if r.Data["evidence"].(map[string]any)["red_observed"] != true {
		t.Fatalf("red = %+v", r.Data)
	}
	h.write("save.go", "package save // fixed\n")
	h.expect(h.run(true, "verify", "--task", "1", "--", "grep -q fixed save.go"), "close", "hcd-close")
	r = h.run(true, "close", "--quote", "apruebo")
	if r.State != "closed" || r.Stage != "closed" {
		t.Fatalf("close = %+v", r)
	}
	after := hcdProjectSnapshot(t, h.root)
	delete(before, "save.go")
	delete(after, "save.go")
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("HCD changed files outside hcd/:\nbefore=%v\nafter=%v", before, after)
	}
}

// SC-001: una feature por la ceremonia completa, paso a paso.
func TestHCDBinaryFullCeremony(t *testing.T) {
	h := newHCDBinary(t)
	before := hcdProjectSnapshot(t, h.root)
	r := h.run(true, "start", "--kind", "feature", "--lane", "full", "--title", "Exportar CSV", "--why", "decisiones de formato interdependientes", "--intent", "Exportar informes a CSV")
	h.expect(r, "define", "hcd-define")
	doc := r.DocPath
	h.edit(doc, append(hcdCommonFill, "### US1. pendiente\npendiente\n- C-01: pendiente", "### US1. Exportar\nComo analista, quiero CSV, para compartir\n- C-01: Dado un informe, cuando exporto, entonces obtengo CSV")...)
	h.expect(h.run(true, "clarify", "--ask", "¿Separador coma o punto y coma?"), "clarify", "hcd-clarify")
	h.expect(h.run(true, "clarify", "--id", "Q1", "--answer", "coma"), "approve", "hcd-approve")
	h.expect(h.run(true, "approve", "--quote", "apruebo"), "slice", "hcd-slice")
	h.edit(doc, "## Slices\n", "## Slices\n### VS-01. Exportar\nTareas: 1\nCriterios: C-01\n")
	h.expect(h.run(true, "show"), "tasks", "hcd-tasks")
	h.edit(doc, "## Checklist\n", "## Checklist\n- [ ] 1. Exportador CSV → grep -q csv save.go\n")
	h.expect(h.run(true, "show"), "implement", "hcd-implement")
	h.write("save.go", "package save // csv\n")
	h.expect(h.run(true, "verify", "--task", "1", "--", "grep -q csv save.go"), "close", "hcd-close")
	for _, f := range h.run(true, "analyze").Findings {
		if f.Severity == "CRITICAL" {
			t.Fatalf("critical finding before close: %+v", f)
		}
	}
	if r = h.run(true, "close", "--quote", "apruebo"); r.State != "closed" {
		t.Fatalf("close = %+v", r)
	}
	after := hcdProjectSnapshot(t, h.root)
	delete(before, "save.go")
	delete(after, "save.go")
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("HCD changed files outside hcd/:\nbefore=%v\nafter=%v", before, after)
	}
}
