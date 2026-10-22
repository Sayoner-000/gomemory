package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookSubKey(t *testing.T) {
	for sub, want := range map[string]string{
		"session-start":                      "session-start",
		"tool-output claude":                 "tool-output",
		"expand pre-tool --emit=claude":      "expand pre-tool",
		"expand session-start --emit=json":   "expand session-start",
		"expand --emit=claude":               "expand",
		"expand":                             "expand",
		"expand stop\" --emit=claude":        "expand stop",
		"plan-guard --emit=claude extra-arg": "plan-guard",
	} {
		if got := hookSubKey(sub); got != want {
			t.Errorf("hookSubKey(%q) = %q, want %q", sub, got, want)
		}
	}
}

func writeClaudeSettingsFile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Un global con solo uno de los hooks de expand no cubre los demás: el dedup
// retira únicamente ese.
func TestDedupClaudeProjectHooks_ExpandSeComparaPorSubcomando(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	writeClaudeSettingsFile(t, home, `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"mem hook expand stop --emit=claude"}]}]}}`)
	writeClaudeSettingsFile(t, root, `{"hooks":{`+
		`"Stop":[{"matcher":"","hooks":[{"type":"command","command":"mem hook expand stop --emit=claude"}]}],`+
		`"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"mem hook expand pre-tool --emit=claude"}]}]}}`)

	removed, err := DedupClaudeProjectHooks(home, root)
	if err != nil || strings.Join(removed, ",") != "expand stop" {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	got := string(data)
	if strings.Contains(got, "hook expand stop") {
		t.Error("expand stop ya lo cubre el global y debe retirarse")
	}
	if !strings.Contains(got, "hook expand pre-tool") {
		t.Errorf("expand pre-tool no está en el global y debe conservarse:\n%s", got)
	}
}

// El instalador del proyecto omite solo los subcomandos de expand que el
// global ya registra.
func TestWriteClaudeHooks_GlobalParcialDeExpandNoOcultaElResto(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows
	writeClaudeSettingsFile(t, home, `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"mem hook expand stop --emit=claude"}]}]}}`)

	if err := writeClaudeHooksSkipping(root, AgentRef{HookCommand: "mem"}, globalClaudeHookSubs(root)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	got := string(data)
	if strings.Contains(got, "hook expand stop") {
		t.Error("expand stop ya lo cubre el global: el proyecto no debe registrarlo")
	}
	for _, sub := range []string{"expand session-start", "expand prompt", "expand pre-tool", "expand post-tool"} {
		if !strings.Contains(got, "hook "+sub) {
			t.Errorf("el global no cubre %s: el proyecto debe registrarlo\n%s", sub, got)
		}
	}
}
