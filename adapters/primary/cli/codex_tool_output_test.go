package cli

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// T060 — el hook de salidas de Codex se añade y se retira sin tocar lo demás.
func TestSyncCodexToolOutputHook(t *testing.T) {
	base := []byte(`[mcp_servers.gomemory]
command = "mem"
args = ["mcp"]

[[hooks.PostToolUse]]
matcher = "shell"
[[hooks.PostToolUse.hooks]]
type = "command"
command = "otro-hook"
`)
	on, changed, err := syncCodexToolOutputHook(base, "mem", true)
	if err != nil || !changed || !strings.Contains(string(on), "hook tool-output codex") {
		t.Fatalf("activar: changed=%v err=%v\n%s", changed, err, on)
	}
	if !strings.Contains(string(on), "otro-hook") || !strings.Contains(string(on), "[mcp_servers.gomemory]") {
		t.Error("activar no debe tocar hooks ajenos ni el registro MCP")
	}
	if _, changed, _ := syncCodexToolOutputHook(on, "mem", true); changed {
		t.Error("activar dos veces no debe duplicar")
	}
	off, changed, err := syncCodexToolOutputHook(on, "mem", false)
	if err != nil || !changed || strings.Contains(string(off), "tool-output") || !strings.Contains(string(off), "otro-hook") {
		t.Fatalf("desactivar: changed=%v err=%v\n%s", changed, err, off)
	}
	var doc map[string]any
	if err := toml.Unmarshal(off, &doc); err != nil {
		t.Errorf("TOML inválido tras desactivar: %v", err)
	}
	if _, changed, _ := syncCodexToolOutputHook(base, "mem", false); changed {
		t.Error("desactivar sin hook no debe escribir")
	}
}
