package cli

import (
	"strings"
	"testing"
)

// HCD-028 C-06: agy no admite texto en PreToolUse; el contexto de edición se
// guarda por conversación y llega en su siguiente PreInvocation.
func TestEditContext_CuatroRuntimesAgyPendiente(t *testing.T) {
	dir := agyProjectDir(t)
	store := agyPendingStore{}
	run := func(_ string, call agyCall) (string, error) {
		if strings.Join(call.args, " ") == "expand pre-tool --emit=agy" {
			return "Contexto de domain/rules.go (gomemory):\n- [7] decision · d → get_memory 7", nil
		}
		return "", nil
	}
	pre := agyEventJSON(t, map[string]any{"conversationId": "conv", "workspacePaths": []string{dir},
		"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": dir + "/domain/rules.go"}}})
	resp := string(agyHookResponseWith("PreToolUse", pre, false, run, store))
	if resp != `{"decision":"allow"}` {
		t.Fatalf("PreToolUse solo decide; el texto se guarda: %s", resp)
	}
	inv := agyEventJSON(t, map[string]any{"conversationId": "conv", "workspacePaths": []string{dir}, "invocationNum": 3})
	got := string(agyHookResponseWith("PreInvocation", inv, false, run, store))
	if !strings.Contains(got, "get_memory 7") || !strings.Contains(got, "ephemeralMessage") {
		t.Fatalf("el siguiente PreInvocation debe entregar el contexto pendiente: %s", got)
	}
	if again := string(agyHookResponseWith("PreInvocation", inv, false, run, store)); strings.Contains(again, "get_memory 7") {
		t.Fatalf("el pendiente se entrega una sola vez: %s", again)
	}
}
