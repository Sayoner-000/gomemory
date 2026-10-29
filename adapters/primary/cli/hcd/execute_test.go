package hcd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mem/adapters/primary/cli/clikit"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type tick struct{ t time.Time }

func (c *tick) Now() time.Time { c.t = c.t.Add(time.Second); return c.t }

func newDeps(t *testing.T) (*clikit.Deps, string) {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	tpl, err := os.ReadFile("../../../../infrastructure/templates/hcd/unit/unit.md")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	uc := usecases.NewHCD(persistence.NewHCDStore(db, "p"), hcdfiles.Workspace{Root: root}, &tick{t: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)}, func() string { n++; return fmt.Sprint(n) }, string(tpl))
	return &clikit.Deps{HCD: uc}, root
}

func call(t *testing.T, deps *clikit.Deps, args ...string) (int, Response) {
	t.Helper()
	var out, errOut bytes.Buffer
	// --json va antes de «--»: lo que sigue pertenece al comando de verify.
	code := run(deps, append([]string{args[0], "--json"}, args[1:]...), &out, &errOut)
	var r Response
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("decode %q (stderr %q): %v", out.String(), errOut.String(), err)
	}
	return code, r
}

func TestParseRejectsUnknownFlagsAndCommands(t *testing.T) {
	for _, args := range [][]string{
		{"migrate"},
		{"show", "--quote", "x"},
		{"close", "--task", "1"},
		{"approve", "--answer", "x"},
		{"promote", "--spec", "specs/x"},
		{"start", "--client", "robot"},
		{"show", "A", "B"},
		{"close", "--", "rm -rf /"},
		{"start", "--title"},
	} {
		if _, err := parse(args); err == nil {
			t.Errorf("parse(%v) accepted", args)
		}
	}
	p, err := parse([]string{"verify", "HCD-001", "--task", "1,2", "--expect-fail", "--", "go test ./x -run 'A B'"})
	if err != nil || p.id != "HCD-001" || p.flags["task"] != "1,2" || p.flags["expect-fail"] != "true" || p.run != "go test ./x -run 'A B'" {
		t.Fatalf("parse verify = %+v %v", p, err)
	}
}

func TestEngineJSONFlowAndExitCodes(t *testing.T) {
	deps, root := newDeps(t)
	if code, r := call(t, deps, "show"); code != 3 || r.OK || r.Error.Code != "not_found" {
		t.Fatalf("show without unit = %d %+v", code, r)
	}
	code, r := call(t, deps, "start", "--kind", "fix", "--lane", "fast", "--title", "Guardar", "--why", "fix claro", "--client", "claude")
	if code != 0 || !r.OK || r.UnitID != "HCD-001" || r.Stage != "define" || r.ProtocolVersion != 2 || len(r.PendingChecks) == 0 || r.NextAction.Skill != "hcd-define" {
		t.Fatalf("start = %d %+v", code, r)
	}
	if code, r = call(t, deps, "verify", "--task", "1", "--", "true"); code != 5 || r.Error.Code != "not_allowed" {
		t.Fatalf("verify in define = %d %+v", code, r)
	}
	doc := filepath.Join(root, r.DocPath)
	if r.DocPath == "" {
		doc = filepath.Join(root, "hcd", "HCD-001-guardar.md")
	}
	data, _ := os.ReadFile(doc)
	filled := strings.NewReplacer("## Intención\npendiente", "## Intención\nno guarda", "## Reproducción\npendiente", "## Reproducción\ntest -f fixed", "Incluido: pendiente", "Incluido: a", "Excluido: pendiente", "Excluido: b", "## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nx", "## Alternativa más barata\npendiente", "## Alternativa más barata\ny").Replace(string(data))
	if err := os.WriteFile(doc, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, r = call(t, deps, "verify", "--task", "1", "--expect-fail", "--", "test -f fixed"); code != 0 || r.Data["evidence"].(map[string]any)["red_observed"] != true {
		t.Fatalf("red = %d %+v", code, r)
	}
	if err := os.WriteFile(filepath.Join(root, "fixed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, r = call(t, deps, "verify", "--task", "1", "--", "test -f fixed"); code != 0 || r.Stage != "close" || r.Tasks[0].Status != "verified" {
		t.Fatalf("green = %d %+v", code, r)
	}
	if code, r = call(t, deps, "close", "--quote", "ok"); code != 5 || r.Error.Code != "invalid_quote" {
		t.Fatalf("close bad quote = %d %+v", code, r)
	}
	if code, r = call(t, deps, "close", "--quote", "apruebo", "--client", "claude"); code != 0 || r.State != "closed" || r.Stage != "closed" {
		t.Fatalf("close = %d %+v", code, r)
	}
	for _, field := range []string{"protocol_version", "ok", "unit_id", "kind", "lane", "state", "stage", "doc_path", "definition_revision", "work_revision", "approved", "constitution_revision", "playbook_revision", "norms_changed", "fingerprint", "tasks", "pending_checks", "red_missing", "findings", "next_action", "warnings", "data"} {
		var out, errOut bytes.Buffer
		run(deps, []string{"show", "HCD-001", "--json"}, &out, &errOut)
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(out.Bytes(), &raw)
		if _, ok := raw[field]; !ok {
			t.Errorf("response lacks %q", field)
		}
	}
}

func TestUsageWithoutArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(&clikit.Deps{}, nil, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "mem hcd") {
		t.Fatalf("usage = %d %q", code, errOut.String())
	}
}

func TestEngineFullCeremony(t *testing.T) {
	deps, root := newDeps(t)
	_, r := call(t, deps, "start", "--kind", "feature", "--lane", "full", "--title", "Exportar", "--why", "decisiones interdependientes")
	doc := filepath.Join(root, r.DocPath)
	data, _ := os.ReadFile(doc)
	filled := strings.NewReplacer(
		"## Intención\npendiente", "## Intención\nexportar",
		"Incluido: pendiente", "Incluido: a", "Excluido: pendiente", "Excluido: b",
		"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nx",
		"## Alternativa más barata\npendiente", "## Alternativa más barata\ny",
		"### US1. pendiente\npendiente\n- C-01: pendiente", "### US1. Exportar\nComo analista, quiero CSV, para compartir\n- C-01: Dado un informe, cuando exporto, entonces obtengo CSV",
	).Replace(string(data))
	if err := os.WriteFile(doc, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, r := call(t, deps, "clarify", "--ask", "¿Separador?"); code != 0 || r.Stage != "clarify" || r.Data["clarification"] != "Q1" || r.NextAction.Skill != "hcd-clarify" {
		t.Fatalf("clarify ask = %d %+v", code, r)
	}
	if code, r := call(t, deps, "clarify", "--id", "Q1", "--answer", "coma"); code != 0 || r.Stage != "approve" || r.Approved {
		t.Fatalf("clarify answer = %d %+v", code, r)
	}
	if code, r := call(t, deps, "analyze"); code != 0 || len(r.Findings) == 0 || r.Findings[0].Severity != "CRITICAL" {
		t.Fatalf("analyze before approve = %d %+v", code, r.Findings)
	}
	if code, r := call(t, deps, "approve", "--quote", "apruebo", "--client", "claude"); code != 0 || r.Stage != "slice" || !r.Approved || r.NextAction.Skill != "hcd-slice" {
		t.Fatalf("approve = %d %+v", code, r)
	}
	if code, r := call(t, deps, "approve", "--quote", "apruebo"); code != 5 || r.Error.Code != "not_allowed" {
		t.Fatalf("second approve = %d %+v", code, r)
	}
	if code, r := call(t, deps, "promote"); code != 5 || r.Error.Code != "not_allowed" {
		t.Fatalf("promote a full unit = %d %+v", code, r)
	}
}
