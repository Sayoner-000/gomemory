package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/expandstate"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/rulesfs"
	"mem/application/usecases"
	"mem/domain"
)

// fakeGuardLog implementa ChannelActivityLog y HookGuardRecorder.
type fakeGuardLog struct {
	events []domain.GuardCount
	days   int
}

func TestExpandInstinctListConfirmReject(t *testing.T) {
	h := newExpandHarness(t)
	t.Setenv("GOMEMORY_DATA_HOME", h.home)
	db, err := persistence.Open(h.root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := persistence.NewInstinctRepository(db, "project")
	h.deps.Instincts = &usecases.Instincts{Repo: repo, Project: "project", Memories: persistence.NewMemoryRepository(db)}
	for _, slug := range []string{"cmd:go-test", "cmd:git-status"} {
		if err := repo.Fold(context.Background(), []domain.Observation{{ID: slug, Kind: "cmd", Key: slug, Sample: "run tests"}}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	out, _, code := h.run(t, "", "instincts", "list", "--json")
	if code != 0 || !strings.Contains(out, "cmd:go-test") {
		t.Fatal(out, code)
	}
	if _, _, code := h.run(t, "", "instincts", "confirm", "cmd:go-test"); code != 0 {
		t.Fatal(code)
	}
	memories, err := h.deps.Instincts.Memories.List("project", 10)
	if err != nil || len(memories) != 1 || memories[0].TopicKey != "instinct:cmd:go-test" || memories[0].Type != domain.Preference {
		t.Fatalf("%+v %v", memories, err)
	}
	if _, _, code := h.run(t, "", "instincts", "reject", "cmd:git-status"); code != 0 {
		t.Fatal(code)
	}
	out, _, code = h.run(t, "", "instincts", "list", "--json")
	if code != 0 || strings.Contains(out, "cmd:git-status") {
		t.Fatal(out, code)
	}
	out, _, code = h.run(t, "", "instincts", "list", "--json", "--all")
	if code != 0 || !strings.Contains(out, "rejected") {
		t.Fatal(out, code)
	}
	if _, _, code := h.run(t, "", "instincts", "confirm", "missing"); code != 1 {
		t.Fatal(code)
	}
}

func (f *fakeGuardLog) RecordFired(string, string, string) error         { return nil }
func (f *fakeGuardLog) RecordError(string, string, string, string) error { return nil }
func (f *fakeGuardLog) Last(string, string, string) (time.Time, string, bool) {
	return time.Time{}, "", false
}
func (f *fakeGuardLog) SessionsSince(time.Time) int              { return 0 }
func (f *fakeGuardLog) RecordGuard(string, string, string) error { return nil }
func (f *fakeGuardLog) GuardSince(days int) ([]domain.GuardCount, error) {
	f.days = days
	return f.events, nil
}

type expandHarness struct {
	root, home string
	deps       *Deps
	log        *fakeGuardLog
}

func newExpandHarness(t *testing.T) expandHarness {
	t.Helper()
	root, home := t.TempDir(), t.TempDir()
	log := &fakeGuardLog{events: []domain.GuardCount{
		{Agent: "claude", Kind: "expand.block", Count: 2, LastDetail: "destructive-shell"},
		{Agent: "codex", Kind: "tool-output.excluded", Count: 5},
	}}
	deps := &Deps{
		Root:            root,
		SettingsRepo:    persistence.NewSettingsRepository(),
		ProjectRepo:     persistence.NewProjectRepository(),
		RuleSource:      rulesfs.New(),
		ShieldState:     expandstate.NewShieldState(filepath.Join(root, persistence.MemDir)),
		ChannelActivity: log,
	}
	return expandHarness{root: root, home: home, deps: deps, log: log}
}

func (h expandHarness) run(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runExpand(h.deps, args, expandIO{in: strings.NewReader(stdin), out: &out, err: &errb, home: h.home,
		now: func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) }})
	return out.String(), errb.String(), code
}

func TestExpandStatusReportsWiringSwitchesAndActivations(t *testing.T) {
	h := newExpandHarness(t)
	// Claude cableado en el proyecto; Codex y OpenCode sin expand.
	_ = os.MkdirAll(filepath.Join(h.root, ".claude"), 0o755)
	_ = os.WriteFile(filepath.Join(h.root, ".claude", "settings.json"),
		[]byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"mem hook expand pre-tool --emit=claude"}]}]}}`), 0o644)
	_ = os.MkdirAll(filepath.Join(h.home, ".config", "opencode", "plugins"), 0o755)
	_ = os.WriteFile(filepath.Join(h.home, ".config", "opencode", "plugins", "gomemory.ts"), []byte(`memWithStdin(["hook", "turn-end"])`), 0o644)

	out, _, code := h.run(t, "", "status")
	if code != 0 {
		t.Fatalf("status → %d", code)
	}
	for _, want := range []string{"claude", "✓", "opencode", "✗", "rules", "destructive-shell", "expand.block"} {
		if !strings.Contains(out, want) {
			t.Errorf("status no muestra %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tool-output.excluded") {
		t.Error("status solo debe listar activaciones expand.*")
	}
	if h.log.days != 7 {
		t.Errorf("activaciones de los últimos 7 días, pidió %d", h.log.days)
	}

	jsonOut, _, _ := h.run(t, "", "status", "--json")
	var st map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &st); err != nil {
		t.Fatalf("status --json no es JSON: %v\n%s", err, jsonOut)
	}
	wiring, _ := st["wiring"].(map[string]any)
	if wiring["claude"] != true || wiring["opencode"] != false {
		t.Errorf("cableado: %v", wiring)
	}
	if out2, _, _ := h.run(t, ""); out2 != out {
		t.Error("`mem expand` sin argumentos equivale a status")
	}
}

func TestExpandOffOnPersistsSwitches(t *testing.T) {
	h := newExpandHarness(t)
	read := func() (bool, []string) {
		s := h.deps.SettingsRepo.Read(h.root)
		return s.ExpandDisabled, s.ExpandOff
	}
	if _, _, code := h.run(t, "", "off"); code != 0 {
		t.Fatal("off")
	}
	if dis, _ := read(); !dis {
		t.Fatal("off debe activar expand_disabled")
	}
	if _, _, code := h.run(t, "", "off", "rules"); code != 0 {
		t.Fatal("off rules")
	}
	if _, off := read(); len(off) != 1 || off[0] != "rules" {
		t.Fatalf("expand_off: %v", off)
	}
	if _, _, code := h.run(t, "", "on", "rules"); code != 0 {
		t.Fatal("on rules")
	}
	if _, off := read(); len(off) != 0 {
		t.Fatalf("on rules debe quitarla: %v", off)
	}
	_, _, _ = h.run(t, "", "off", "shield")
	out, _, code := h.run(t, "", "on")
	if dis, off := read(); code != 0 || dis || len(off) != 0 || !strings.Contains(out, "todas") {
		t.Fatalf("on reactiva todo: dis=%v off=%v out=%q", dis, off, out)
	}
	if _, errOut, code := h.run(t, "", "off", "bogus"); code != 2 || !strings.Contains(errOut, "bogus") {
		t.Fatalf("supertool inválida → 2: %d %q", code, errOut)
	}
	if _, _, code := h.run(t, "", "xyz"); code != 2 {
		t.Fatalf("subcomando desconocido → 2: %d", code)
	}
	// Nada se escribe fuera de .memory/.
	entries, _ := os.ReadDir(h.root)
	for _, e := range entries {
		if e.Name() != persistence.MemDir {
			t.Errorf("escribió fuera de .memory/: %s", e.Name())
		}
	}
}

func TestExpandRulesListAndCheck(t *testing.T) {
	h := newExpandHarness(t)
	rd := filepath.Join(h.root, ".gomemory", "rules")
	_ = os.MkdirAll(rd, 0o755)
	_ = os.WriteFile(filepath.Join(rd, "env.md"), []byte("---\nevent: file\nmatch: regex\npattern: '\\.env$'\naction: warn\n---\nno toques .env\n"), 0o644)
	_ = os.WriteFile(filepath.Join(rd, "rota.md"), []byte("sin frontmatter"), 0o644)

	out, _, code := h.run(t, "", "rules", "--json")
	var listed struct {
		Rules  []map[string]any `json:"rules"`
		Errors []map[string]any `json:"errors"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &listed) != nil || len(listed.Rules) != 2 || len(listed.Errors) != 1 {
		t.Fatalf("rules --json: %d %s", code, out)
	}

	rmDiff := "--- a/run.sh\n+++ b/run.sh\n@@\n-echo hola\n+rm -rf build\n"
	if _, _, code := h.run(t, rmDiff, "rules", "--check", "--event", "bash"); code != 2 {
		t.Errorf("--event bash con rm -rf → 2, got %d", code)
	}
	if _, _, code := h.run(t, "--- a/main.go\n+++ b/main.go\n@@\n+fmt.Println(1)\n", "rules", "--check"); code != 0 {
		t.Errorf("diff inocuo → 0, got %d", code)
	}
	out, _, code = h.run(t, "--- a/config/.env\n+++ b/config/.env\n@@\n+X=1\n", "rules", "--check", "--json")
	var v map[string]any
	if code != 1 || json.Unmarshal([]byte(out), &v) != nil || v["decision"] != "warn" || v["rule"] != "env" {
		t.Errorf("regla warn sobre la ruta → 1: %d %s", code, out)
	}
	if _, _, code := h.run(t, "\xff\xfe", "rules", "--check"); code != 3 {
		t.Errorf("stdin ilegible → 3, got %d", code)
	}
}

// H-03 (CWE-150): un nombre de regla con ESC y un informe shield cacheado con
// OSC vienen del clon; status y rules los pintan depurados, y status solo
// resume el shield.
func TestExpandStatusAndRulesStripTerminalControls(t *testing.T) {
	h := newExpandHarness(t)
	rd := filepath.Join(h.root, ".gomemory", "rules")
	_ = os.MkdirAll(rd, 0o755)
	_ = os.WriteFile(filepath.Join(rd, "a\x1b[31mb.md"), []byte("sin frontmatter"), 0o644)
	memDir := filepath.Join(h.root, persistence.MemDir)
	_ = os.MkdirAll(memDir, 0o755)
	_ = os.WriteFile(filepath.Join(memDir, "expand-shield-last.json"),
		[]byte(`{"hash":"h","scanned_at":"2026-10-21T10:00:00Z","findings":[{"file":"/x\u001b]52;c;cHduZWQ=\u0007","category":"secret","severity":"critical","detail":"x\u001b]0;pwned\u0007"}],"skipped":[]}`), 0o644)

	for _, args := range [][]string{{"status"}, {"rules"}} {
		out, _, _ := h.run(t, "", args...)
		if strings.ContainsAny(out, "\x1b\x07") {
			t.Errorf("%v emite controles de terminal: %q", args, out)
		}
	}
	out, _, _ := h.run(t, "", "status")
	if !strings.Contains(out, "Shield: 1 hallazgos (1 critical)") || strings.Contains(out, "pwned") {
		t.Errorf("status debe resumir el shield sin volcar detalles:\n%s", out)
	}
	if r, ok := h.deps.ShieldState.Last(context.Background()); !ok || strings.ContainsAny(r.Findings[0].Detail+r.Findings[0].File, "\x1b\x07") {
		t.Errorf("ShieldState.Last debe depurar al leer: %+v", r)
	}
}
