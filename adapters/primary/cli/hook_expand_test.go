package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"mem/application/usecases"
	"mem/domain"
)

type recorded struct{ kind, detail string }

func expandTestDeps(t *testing.T, rec *[]recorded) hookExpandDeps {
	t.Helper()
	return hookExpandDeps{
		root: t.TempDir(),
		now:  func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) },
		record: func(_, kind, detail string) {
			*rec = append(*rec, recorded{kind, detail})
		},
	}
}

const bashPayload = `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s","tool_input":{"command":"ls"}}`

func hasKind(rec []recorded, kind string) bool {
	for _, r := range rec {
		if r.kind == kind {
			return true
		}
	}
	return false
}

// 2.º analyze K1/C1: el pánico de una dependencia inyectada permite y deja
// rastro, sin disparadores de prueba en el binario.
func TestRunHookExpand_PanicAllowsAndRecords(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	d.run = func(context.Context, domain.ExpandSignals, []domain.SupertoolAction, usecases.ExpandHookDeps) domain.ExpandOutcome {
		panic("boom")
	}
	out := runHookExpand(context.Background(), d, "pre-tool", "", []byte(bashPayload))
	if out.stdout != "{}" || out.exitCode != 0 {
		t.Fatalf("pánico → allow: %+v", out)
	}
	if !hasKind(rec, "expand.error") {
		t.Fatalf("debe registrar expand.error: %+v", rec)
	}
}

type slowRules struct{ sawDone bool }

func (s *slowRules) Evaluate(ctx context.Context, _ string, _ domain.ExpandSignals) (usecases.RuleResult, error) {
	select {
	case <-ctx.Done():
		s.sawDone = true
		return usecases.RuleResult{}, ctx.Err()
	case <-time.After(time.Second):
		return usecases.RuleResult{Decision: domain.DecisionBlock, Reason: "tarde", Rule: "lenta"}, nil
	}
}

// Un evaluador que tarda más que el plazo: allow, sin contexto, timeout
// registrado y el evaluador recibió ctx.Done().
func TestRunHookExpand_TimeoutAllows(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	rules := &slowRules{}
	d.hook = usecases.ExpandHookDeps{Rules: rules}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	out := runHookExpand(ctx, d, "pre-tool", "", []byte(bashPayload))
	if out.stdout != "{}" || out.exitCode != 0 {
		t.Fatalf("timeout → allow sin contexto: %+v", out)
	}
	if !rules.sawDone || !hasKind(rec, "expand.timeout") {
		t.Fatalf("el evaluador debe ver ctx.Done() y registrarse el timeout: sawDone=%v rec=%+v", rules.sawDone, rec)
	}
}

// B-12 del ACR: aunque venza el plazo, un comando destructivo se sigue
// bloqueando con la regla de serie; uno inocuo, y con rules apagado, permite.
func TestRunHookExpand_TimeoutStillBlocksDestructive(t *testing.T) {
	run := func(cmd string, off []string) hookRenderedOutput {
		var rec []recorded
		d := expandTestDeps(t, &rec)
		d.settings.ExpandOff = off
		d.hook = usecases.ExpandHookDeps{Rules: &slowRules{}}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s","tool_input":{"command":"` + cmd + `"}}`
		return runHookExpand(ctx, d, "pre-tool", "claude", []byte(payload))
	}
	if out := run("rm -rf /tmp/x", nil); !strings.Contains(out.stdout, `"permissionDecision":"deny"`) {
		t.Fatalf("timeout con rm -rf debe bloquear: %+v", out)
	}
	if out := run("ls", nil); out.stdout != "{}" {
		t.Fatalf("timeout con comando inocuo permite: %+v", out)
	}
	if out := run("rm -rf /tmp/x", []string{"rules"}); out.stdout != "{}" {
		t.Fatalf("con rules apagado, permite: %+v", out)
	}
}

// B-11 del ACR: un relleno delante no oculta el final del comando, y un
// pre-tool de shell que desborda el tope se bloquea; un Write desbordado no.
func TestRunHookExpand_LongAndOversizedPayloads(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	d.hook = usecases.ExpandHookDeps{Rules: usecases.ExpandRules{Source: expandStaticRules(domain.BuiltinRules())}}
	padded := `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s","tool_input":{"command":"echo ` + strings.Repeat("a", 70*1024) + ` && rm -rf /tmp/x"}}`
	if out := runHookExpand(context.Background(), d, "pre-tool", "claude", []byte(padded)); !strings.Contains(out.stdout, `"permissionDecision":"deny"`) {
		t.Fatalf("rm -rf tras 70 KiB de relleno debe bloquearse: %.200s", out.stdout)
	}
	d.oversized = true
	truncated := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s","tool_input":{"command":"echo aaaa`)
	if out := runHookExpand(context.Background(), d, "pre-tool", "claude", truncated); !strings.Contains(out.stdout, `"permissionDecision":"deny"`) {
		t.Fatalf("shell desbordado debe bloquearse: %+v", out)
	}
	write := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Write","session_id":"s","tool_input":{"file_path":"a","content":"aaaa`)
	if out := runHookExpand(context.Background(), d, "pre-tool", "claude", write); strings.Contains(out.stdout, "deny") {
		t.Fatalf("un Write desbordado no se bloquea: %+v", out)
	}
	nested := []byte(`{"tool_input":{"tool_name":"Bash"},"tool_name":"Write","x":"aaaa`)
	if oversizedShellPayload(nested) {
		t.Fatal("solo cuenta tool_name del primer nivel")
	}
	if !oversizedShellPayload([]byte(`{"sessionID":"x","args":{"command":"a"},"tool":"bash","z":"aaa`)) {
		t.Fatal("forma OpenCode")
	}
	// C-004 (ACR acr_a635820b): sin tool_name antes del corte no se puede
	// clasificar; el orden de las claves no decide.
	late := []byte(`{"hook_event_name":"PreToolUse","session_id":"s","tool_input":{"command":"rm -rf /tmp/x; echo aaaa`)
	if out := runHookExpand(context.Background(), d, "pre-tool", "claude", late); !strings.Contains(out.stdout, `"permissionDecision":"deny"`) {
		t.Fatalf("tool_input antes de tool_name debe bloquearse: %+v", out)
	}
	if !oversizedShellPayload([]byte(`{"tool_input":{"tool_name":"Bash"},"x":"aaaa`)) {
		t.Fatal("sin tool_name de primer nivel no se puede clasificar")
	}
}

type failingObserver struct{ queued int }

func (f *failingObserver) Observe(context.Context, domain.ExpandSignals) error {
	f.queued++ // la observación ya está en la cola falsa…
	return nil
}
func (f *failingObserver) Fold(context.Context) error { return context.DeadlineExceeded }

// Edge case de la spec (2.º analyze U2): sin registrador ni base, el hook
// permite y la observación queda en la cola.
func TestRunHookExpand_NoRecorderStillAllows(t *testing.T) {
	d := hookExpandDeps{root: t.TempDir()}
	obs := &failingObserver{}
	d.hook = usecases.ExpandHookDeps{Instincts: obs}
	post := `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"go test ./..."}}`
	if out := runHookExpand(context.Background(), d, "post-tool", "", []byte(post)); out != (hookRenderedOutput{}) {
		t.Fatalf("post-tool silencioso: %+v", out)
	}
	if obs.queued != 1 {
		t.Fatalf("la observación debe quedar en la cola: %d", obs.queued)
	}
	if out := runHookExpand(context.Background(), d, "stop", "json", []byte(`{}`)); out.stdout != "{}" {
		t.Fatalf("un plegado fallido no bloquea: %+v", out)
	}
}

func TestNormalizeExpandPayload(t *testing.T) {
	claude := []byte(`{"session_id":"a","tool_name":"Write","tool_input":{"file_path":"x.go","content":"hola"}}`)
	p, _ := parseHookPayload(claude)
	in := normalizeExpandPayload(p, claude)
	if in.sessionID != "a" || in.tool != "Write" || in.filePath != "x.go" || in.content != "hola" {
		t.Errorf("claude: %+v", in)
	}
	opencode := []byte(`{"sessionID":"b","tool":"bash","args":{"command":"ls"},"source":"compact"}`)
	p, _ = parseHookPayload(opencode)
	in = normalizeExpandPayload(p, opencode)
	if in.sessionID != "b" || in.tool != "bash" || in.command != "ls" || in.source != "compact" {
		t.Errorf("opencode: %+v", in)
	}
	in = normalizeExpandPayload(nil, []byte("añade la librería zod"))
	if in.prompt != "añade la librería zod" {
		t.Errorf("texto plano: %+v", in)
	}
}

type blockAll struct{}

func (blockAll) Evaluate(context.Context, string, domain.ExpandSignals) (usecases.RuleResult, error) {
	return usecases.RuleResult{Decision: domain.DecisionBlock, Reason: "sigue", Rule: "r"}, nil
}

// Verificado en real con Claude Code: sin respetar stop_hook_active, una
// regla stop que siempre coincide bloquea cada intento de terminar.
func TestRunHookExpand_StopHookActiveBlocksOnce(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	d.hook = usecases.ExpandHookDeps{Rules: blockAll{}}
	first := runHookExpand(context.Background(), d, "stop", "claude", []byte(`{"hook_event_name":"Stop","stop_hook_active":false}`))
	if !strings.Contains(first.stdout, `"decision":"block"`) {
		t.Fatalf("primer stop debe bloquear: %+v", first)
	}
	again := runHookExpand(context.Background(), d, "stop", "claude", []byte(`{"hook_event_name":"Stop","stop_hook_active":true}`))
	if again.stdout != "{}" {
		t.Fatalf("con stop_hook_active no se vuelve a bloquear: %+v", again)
	}
}

// C-003: suprimir el bloqueo repetido no borra los warn ya emitidos.
func TestRunHookExpand_StopHookActiveKeepsWarnRecords(t *testing.T) {
	var rec []recorded
	d := expandTestDeps(t, &rec)
	d.run = func(context.Context, domain.ExpandSignals, []domain.SupertoolAction, usecases.ExpandHookDeps) domain.ExpandOutcome {
		return domain.ExpandOutcome{Decision: domain.DecisionBlock, Rule: "r", Reason: "sigue",
			Fired: []string{"expand.warn:w", "expand.block:r"}}
	}
	runHookExpand(context.Background(), d, "stop", "claude", []byte(`{"hook_event_name":"Stop","stop_hook_active":true}`))
	warn, block := false, false
	for _, r := range rec {
		warn = warn || r.kind == "expand.warn"
		block = block || r.kind == "expand.block"
	}
	if !warn || block {
		t.Fatalf("se registra el warn y no el bloqueo suprimido: %+v", rec)
	}
}

type expandStaticRules []domain.Rule

func (s expandStaticRules) Load(context.Context, string) ([]domain.Rule, []domain.RuleLoadError) {
	return s, nil
}

// A-001 (ronda 5): NotebookEdit y MultiEdit llegan a las reglas de archivo
// con su ruta y su contenido; un cambio inocuo sigue permitido.
func TestRunHookExpand_NotebookAndMultiEditReachFileRules(t *testing.T) {
	var rules expandStaticRules
	for name, body := range map[string]string{
		"ipynb.md":   "---\nevent: file\nmatch: regex\npattern: '\\.ipynb$'\naction: block\n---\nnotebooks no",
		"secreto.md": "---\nevent: file\npattern: DROP TABLE\naction: block\n---\nsql destructivo",
	} {
		r, err := domain.ParseRule(name, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		rules = append(rules, r)
	}
	cases := []struct {
		name, payload string
		deny          bool
	}{
		{"notebook por ruta", `{"hook_event_name":"PreToolUse","tool_name":"NotebookEdit","session_id":"s","tool_input":{"notebook_path":"/p/a.ipynb","new_source":"print(1)"}}`, true},
		{"notebook por contenido", `{"hook_event_name":"PreToolUse","tool_name":"NotebookEdit","session_id":"s","tool_input":{"notebook_path":"/p/a.nb","new_source":"DROP TABLE users"}}`, true},
		{"multiedit por contenido", `{"hook_event_name":"PreToolUse","tool_name":"MultiEdit","session_id":"s","tool_input":{"file_path":"/p/a.sql","edits":[{"old_string":"x","new_string":"y"},{"old_string":"z","new_string":"DROP TABLE t"}]}}`, true},
		{"notebook inocuo", `{"hook_event_name":"PreToolUse","tool_name":"NotebookEdit","session_id":"s","tool_input":{"notebook_path":"/p/a.nb","new_source":"print(1)"}}`, false},
	}
	for _, c := range cases {
		var rec []recorded
		d := expandTestDeps(t, &rec)
		d.hook = usecases.ExpandHookDeps{Rules: usecases.ExpandRules{Source: rules}}
		out := runHookExpand(context.Background(), d, "pre-tool", "claude", []byte(c.payload))
		if got := strings.Contains(out.stdout, `"permissionDecision":"deny"`); got != c.deny {
			t.Errorf("%s: deny=%v, want %v (%s)", c.name, got, c.deny, out.stdout)
		}
	}
}
