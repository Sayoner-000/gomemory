package setup

import (
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
