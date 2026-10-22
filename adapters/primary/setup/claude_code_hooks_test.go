package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/domain"
)

// sessionStartEntries extrae las entradas (matcher → comando) del hook
// SessionStart de un settings.json ya parseado.
func sessionStartEntries(t *testing.T, settings map[string]interface{}) map[string]string {
	t.Helper()
	hooks, _ := settings["hooks"].(map[string]interface{})
	raw, _ := hooks["SessionStart"].([]interface{})
	out := map[string]string{}
	for _, e := range raw {
		m, _ := e.(map[string]interface{})
		matcher, _ := m["matcher"].(string)
		inner, _ := m["hooks"].([]interface{})
		if len(inner) == 0 {
			continue
		}
		h, _ := inner[0].(map[string]interface{})
		cmd, _ := h["command"].(string)
		out[matcher] = cmd
	}
	return out
}

func TestWriteClaudeHooksSplitsSessionStartByMatcher(t *testing.T) {
	root := t.TempDir()
	if err := writeClaudeHooks(root, AgentRef{HookCommand: "mem"}); err != nil {
		t.Fatalf("writeClaudeHooks: %v", err)
	}

	settings := readSettings(t, root)
	entries := sessionStartEntries(t, settings)

	if got := entries["startup|resume|clear"]; got != "mem hook session-start" {
		t.Errorf("matcher startup|resume|clear debía disparar session-start, got %q", got)
	}
	if got := entries["compact"]; got != "mem hook post-compact" {
		t.Errorf("matcher compact debía disparar post-compact (re-inyección post-compactación), got %q", got)
	}

	// PreCompact quedó retirado del registro: su salida no sobrevive a la
	// compactación, la reemplaza SessionStart(compact).
	hooks, _ := settings["hooks"].(map[string]interface{})
	if _, ok := hooks["PreCompact"]; ok {
		t.Error("PreCompact no debe registrarse: fue reemplazado por SessionStart(compact)")
	}
}

func TestWriteClaudeHooksIsIdempotent(t *testing.T) {
	root := t.TempDir()
	ref := AgentRef{HookCommand: "mem"}
	if err := writeClaudeHooks(root, ref); err != nil {
		t.Fatalf("writeClaudeHooks (1): %v", err)
	}
	if err := writeClaudeHooks(root, ref); err != nil {
		t.Fatalf("writeClaudeHooks (2): %v", err)
	}

	// session-start, post-compact y expand session-start (feature 040).
	entries := sessionStartEntries(t, readSettings(t, root))
	if len(entries) != 3 {
		t.Errorf("tras dos corridas esperaba exactamente 3 entradas SessionStart, got %d: %v", len(entries), entries)
	}
}

func TestWriteClaudeHooksRemovesRetiredGomemoryEvents(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	// Simula una instalación anterior: PreCompact de gomemory (ya retirado) +
	// un PreCompact de un tercero que NO debe tocarse.
	existing := `{
		"hooks": {
			"PreCompact": [
				{"matcher": "", "hooks": [{"type": "command", "command": "mem hook pre-compact"}]},
				{"matcher": "", "hooks": [{"type": "command", "command": "otra-tool hook whatever"}]}
			]
		}
	}`
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(existing), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	if err := writeClaudeHooks(root, AgentRef{HookCommand: "mem"}); err != nil {
		t.Fatalf("writeClaudeHooks: %v", err)
	}

	settings := readSettings(t, root)
	hooks, _ := settings["hooks"].(map[string]interface{})
	pre, _ := hooks["PreCompact"].([]interface{})
	for _, e := range pre {
		if IsGomemoryHookEntry(e) {
			t.Error("la entrada retirada de gomemory (pre-compact) debió limpiarse al re-correr setup")
		}
	}
	// El hook de un tercero en PreCompact debe sobrevivir.
	if len(pre) != 1 {
		t.Errorf("esperaba conservar solo el PreCompact de terceros, got %d entradas", len(pre))
	}
}

func TestHookCommandIsGomemoryRecognizesAllSubcommands(t *testing.T) {
	// Cada subcomando registrado debe ser reconocido por la desinstalación,
	// o quedarían hooks huérfanos de gomemory tras `mem uninstall`. La lista
	// fija cubre los retirados del registro (pre-compact), que un settings.json
	// antiguo aún puede contener, y los que dependen de un nivel del registro
	// de capacidades (plan-guard, plan-entered).
	subs := []string{
		"session-start", "session-end", "pre-compact", "post-compact",
		"user-prompt-submit", "turn-end", "subagent-start", "subagent-stop",
		"compact-summary", "plan-approved", "plan-entered", "plan-guard",
	}
	// Y todo lo que el setup registra hoy, para que un subcomando nuevo no
	// pueda quedar fuera sin que este test lo detecte.
	for _, regs := range claudeHookEvents {
		for _, r := range regs {
			subs = append(subs, r.sub)
		}
	}
	// Y el registro único entero (feature 040, R2), incluido `expand`.
	subs = append(subs, domain.GomemoryHookSubcommands...)
	subs = append(subs, "expand pre-tool", "tool-output claude")
	for _, sub := range subs {
		if !hookCommandIsGomemory("mem hook " + sub) {
			t.Errorf("hookCommandIsGomemory no reconoce %q", "mem hook "+sub)
		}
	}
	if hookCommandIsGomemory("some-other-tool hook whatever") {
		t.Error("no debe reconocer comandos de terceros como de gomemory")
	}
}

// Feature 040 (T028): hook expand en los 5 eventos, una sola vez tras dos
// escrituras, sin perder plan-guard ni los hooks de terceros.
func TestWriteClaudeHooks_ExpandWiredIdempotentAndPreservesThirdParty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	third := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"my-linter check"}]}]}}`
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(third), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := AgentRef{HookCommand: "mem"}
	for i := 0; i < 2; i++ {
		if err := writeClaudeHooks(root, ref); err != nil {
			t.Fatal(err)
		}
	}
	hooks, _ := readSettings(t, root)["hooks"].(map[string]interface{})
	count := func(event, needle string) int {
		n := 0
		entries, _ := hooks[event].([]interface{})
		for _, e := range entries {
			for _, c := range hookEntryCommands(e) {
				if strings.Contains(c, needle) {
					n++
				}
			}
		}
		return n
	}
	for event, reg := range claudeExpandHooks {
		if got := count(event, "mem hook "+reg.sub); got != 1 {
			t.Errorf("%s: %d entradas de %q, want 1", event, got, reg.sub)
		}
	}
	if count("PreToolUse", "hook plan-guard") != 1 {
		t.Error("plan-guard debe seguir registrado")
	}
	if count("PreToolUse", "my-linter check") != 1 {
		t.Error("el hook de terceros debe conservarse")
	}
}

// R2-04: la invocación debe ser del binario de gomemory, no solo contener
// `hook <sub>`.
func TestHookCommandIsGomemoryRequiresMemBinary(t *testing.T) {
	cases := map[string]bool{
		"mem hook user-prompt-submit":                                  true,
		"${CLAUDE_PROJECT_DIR}/mem hook expand pre-tool --emit=claude": true,
		"/usr/local/bin/mem hook turn-end":                             true,
		`"C:\Program Files\gomemory\mem.exe" hook session-start`:       true,
		`C:\tools\mem.exe hook turn-end`:                               true,
		`'/Users/a b/mem' hook session-end`:                            true,
		"bash /opt/acme/hooks/entry.sh hook user-prompt-submit":        false,
		"acme-mem hook turn-end":                                       false,
		"memo hook turn-end":                                           false,
		"mem hook desconocido":                                         false,
	}
	for cmd, want := range cases {
		if got := hookCommandIsGomemory(cmd); got != want {
			t.Errorf("hookCommandIsGomemory(%q) = %v, want %v", cmd, got, want)
		}
	}
	if GomemoryHookIndex("mem hook expand pre-tool --emit=json", "expand pre-tool") != 4 ||
		GomemoryHookIndex("mem hook expand pre-tool-x", "expand pre-tool") != -1 ||
		GomemoryHookIndex("bash x.sh hook expand pre-tool", "expand pre-tool") != -1 {
		t.Error("GomemoryHookIndex")
	}
}
