package setup

import (
	"strings"
	"testing"
)

// Feature 040 (T029): la tabla de Codex declara expand en los cinco eventos,
// CodexHookPresente los distingue de los demás hooks del mismo evento, y
// expand prompt no se confunde con el hook de entrada al modo plan.
func TestCodexExpandHooksPresenceAndPlanEntry(t *testing.T) {
	var expand []CodexHook
	for _, h := range CodexGomemoryHooks() {
		if strings.HasPrefix(h.Sub, "expand ") {
			expand = append(expand, h)
		}
	}
	want := map[string]string{
		"SessionStart":     "startup|resume|clear|compact",
		"UserPromptSubmit": "",
		"PreToolUse":       "Bash|apply_patch|collaborationspawn_agent",
		"PostToolUse":      "Bash",
		"Stop":             "",
	}
	if len(expand) != len(want) {
		t.Fatalf("expand en %d eventos, want %d: %+v", len(expand), len(want), expand)
	}
	for _, h := range expand {
		if m, ok := want[h.Event]; !ok || m != h.Matcher || h.Emit != "json" {
			t.Errorf("hook expand inesperado: %+v", h)
		}
	}

	// Config con todo el ciclo previo pero sin expand → faltan exactamente
	// los de expand; con todo instalado → no falta nada.
	without, all := map[string]any{}, map[string]any{}
	for _, h := range CodexGomemoryHooks() {
		g := CodexHookGroup(h, "mem")
		all[h.Event] = append(asSlice(all[h.Event]), g)
		if !strings.HasPrefix(h.Sub, "expand ") {
			without[h.Event] = append(asSlice(without[h.Event]), g)
		}
	}
	for _, h := range CodexGomemoryHooks() {
		isExpand := strings.HasPrefix(h.Sub, "expand ")
		if CodexHookPresente(without, h) == isExpand {
			t.Errorf("sin expand, presencia de %q = %v", h.Sub, !isExpand)
		}
		if !CodexHookPresente(all, h) {
			t.Errorf("con todo instalado falta %q", h.Sub)
		}
	}

	entry, ok := CodexPlanEntryHook()
	if !ok || entry.Sub != "user-prompt-submit" {
		t.Fatalf("la entrada al modo plan debe seguir siendo user-prompt-submit: %+v", entry)
	}
	for _, h := range expand {
		if h.SostieneEntradaDePlan() {
			t.Errorf("%q no sostiene la entrada al modo plan", h.Sub)
		}
	}
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
