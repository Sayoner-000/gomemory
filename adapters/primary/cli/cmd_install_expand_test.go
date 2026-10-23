package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"mem/adapters/primary/setup"
)

func TestExpandInstallUpgradeUninstall(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0700); err != nil {
		t.Fatal(err)
	}
	old := "[hooks]\n[[hooks.PreToolUse]]\nmatcher = 'Bash'\n[[hooks.PreToolUse.hooks]]\ntype = 'command'\ncommand = 'third-party'\n"
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := setup.WriteClaudeHooksGlobal(root, setup.AgentRef{HookCommand: "mem"}); err != nil {
			t.Fatal(err)
		}
		if !setupCodexGlobal(BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}) {
			t.Fatal("codex setup")
		}
	}
	raw, _ := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	var doc map[string]any
	if err := toml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]any)
	for _, h := range setup.CodexGomemoryHooks() {
		if strings.HasPrefix(h.Sub, "expand ") && !setup.CodexHookPresente(hooks, h) {
			t.Errorf("missing %s", h.Sub)
		}
	}
	out, changed, err := removeCodexGomemory(raw)
	if err != nil || !changed || strings.Contains(string(out), "hook expand") || !strings.Contains(string(out), "third-party") {
		t.Fatalf("codex cleanup: %v %s", err, out)
	}
	if !setup.GomemoryHookSubs(filepath.Join(root, ".claude", "settings.json"))["expand"] {
		t.Fatal("claude missing expand")
	}
	removeClaudePlugin(root)
	raw, _ = os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if strings.Contains(string(raw), "hook expand") {
		t.Fatal("orphan expand hook")
	}
	oldProtocol := "antes\n<!-- gomemory-protocol-v9 -->\nlegacy\n<!-- gomemory-protocol-end -->\ndespués"
	fresh, changed := composeAgentFile(oldProtocol, "", buildIntegrationBlock())
	if !changed || !strings.Contains(fresh, "protocol-v10") || strings.Contains(fresh, "protocol-v9") || !strings.Contains(fresh, "después") {
		t.Fatal(fresh)
	}
	again, changed := composeAgentFile(fresh, "", buildIntegrationBlock())
	if changed || again != fresh {
		t.Fatal("not idempotent")
	}
}
