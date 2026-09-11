package setup

import "testing"

// TestWriteClaudeHooks_PostCompactRegistraCompactSummary cubre US2 (feature
// 030, capacidad C6): además de post-compact (recuperación), el matcher
// "compact" de SessionStart... no — PostCompact es un evento DISTINTO de
// SessionStart. Este test verifica que se registre el evento PostCompact con
// `mem hook compact-summary`, sin tocar la entrada de SessionStart(compact)
// existente ni reabrir PreCompact (decisión R12: sigue sin usarse).
func TestWriteClaudeHooks_PostCompactRegistraCompactSummary(t *testing.T) {
	root := t.TempDir()
	if err := writeClaudeHooks(root, AgentRef{HookCommand: "mem"}); err != nil {
		t.Fatalf("writeClaudeHooks: %v", err)
	}

	settings := readSettings(t, root)
	hooks, _ := settings["hooks"].(map[string]interface{})

	raw, ok := hooks["PostCompact"].([]interface{})
	if !ok || len(raw) == 0 {
		t.Fatalf("PostCompact debía registrarse, got %v", hooks["PostCompact"])
	}
	entry, _ := raw[0].(map[string]interface{})
	inner, _ := entry["hooks"].([]interface{})
	if len(inner) == 0 {
		t.Fatal("PostCompact sin hooks internos")
	}
	h, _ := inner[0].(map[string]interface{})
	if cmd, _ := h["command"].(string); cmd != "mem hook compact-summary" {
		t.Errorf("PostCompact debía disparar compact-summary, got %q", cmd)
	}

	if _, ok := hooks["PreCompact"]; ok {
		t.Error("PreCompact sigue sin registrarse (research.md R12)")
	}

	// La entrada SessionStart(compact) —recuperación tras compactar— no debe
	// desaparecer ni cambiar por añadir PostCompact.
	entries := sessionStartEntries(t, settings)
	if got := entries["compact"]; got != "mem hook post-compact" {
		t.Errorf("SessionStart(compact) debía seguir intacto, got %q", got)
	}
}

// TestWriteClaudeHooks_PostCompactEsIdempotente protege contra duplicados al
// reinstalar.
func TestWriteClaudeHooks_PostCompactEsIdempotente(t *testing.T) {
	root := t.TempDir()
	ref := AgentRef{HookCommand: "mem"}
	if err := writeClaudeHooks(root, ref); err != nil {
		t.Fatalf("writeClaudeHooks (1): %v", err)
	}
	if err := writeClaudeHooks(root, ref); err != nil {
		t.Fatalf("writeClaudeHooks (2): %v", err)
	}

	settings := readSettings(t, root)
	hooks, _ := settings["hooks"].(map[string]interface{})
	raw, _ := hooks["PostCompact"].([]interface{})
	if len(raw) != 1 {
		t.Errorf("tras dos corridas esperaba exactamente 1 entrada PostCompact, got %d", len(raw))
	}
}

// TestHookCommandIsGomemory_RecognizesCompactSummary protege la
// desinstalación: sin este reconocimiento, `mem uninstall` dejaría el hook
// PostCompact huérfano.
func TestHookCommandIsGomemory_RecognizesCompactSummary(t *testing.T) {
	if !hookCommandIsGomemory("mem hook compact-summary") {
		t.Error("hookCommandIsGomemory debe reconocer 'mem hook compact-summary'")
	}
}
