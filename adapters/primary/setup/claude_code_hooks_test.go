package setup

import (
	"os"
	"path/filepath"
	"testing"
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

	entries := sessionStartEntries(t, readSettings(t, root))
	if len(entries) != 2 {
		t.Errorf("tras dos corridas esperaba exactamente 2 entradas SessionStart, got %d: %v", len(entries), entries)
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
	// o quedarían hooks huérfanos de gomemory tras `mem uninstall`.
	for _, sub := range []string{
		"session-start", "session-end", "pre-compact", "post-compact",
		"user-prompt-submit", "turn-end", "subagent-stop",
	} {
		if !hookCommandIsGomemory("mem hook " + sub) {
			t.Errorf("hookCommandIsGomemory no reconoce %q", "mem hook "+sub)
		}
	}
	if hookCommandIsGomemory("some-other-tool hook whatever") {
		t.Error("no debe reconocer comandos de terceros como de gomemory")
	}
}
