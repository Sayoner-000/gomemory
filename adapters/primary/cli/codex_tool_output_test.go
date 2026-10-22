package cli

import (
	"os"
	"path/filepath"
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

// Codex rechaza siempre updatedMCPToolOutput: el registro global (mem install
// en cualquier ámbito y directorio) retira el hook de salidas, sin mirar el
// ajuste del proyecto ni tocar hooks ajenos.
func TestSetupCodexGlobalRetiraElHookDeSalidas(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := "[mcp_servers.gomemory]\ncommand = \"mem\"\n\n[[hooks.PostToolUse]]\nmatcher = \"shell\"\n[[hooks.PostToolUse.hooks]]\ntype = \"command\"\ncommand = \"otro-hook\"\n"
	on, _, err := syncCodexToolOutputHook([]byte(base), "mem", true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfg, on, 0o600); err != nil {
		t.Fatal(err)
	}

	if !setupCodexGlobal(BinRef{MCPCommand: "mem"}) {
		t.Fatal("setupCodexGlobal falló")
	}

	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "tool-output") {
		t.Errorf("el hook de salidas debe retirarse:\n%s", got)
	}
	if !strings.Contains(string(got), "otro-hook") || !strings.Contains(string(got), "[mcp_servers.gomemory]") {
		t.Errorf("no debe tocar hooks ajenos ni el registro MCP:\n%s", got)
	}
	if info, _ := os.Stat(cfg); info.Mode().Perm() != 0o600 {
		t.Errorf("debe conservar los permisos: %v", info.Mode().Perm())
	}
}

// R4-02: solo la acción `mem hook tool-output` es propia. Un hook ajeno con
// ese literal en su comando o su matcher, o agrupado con la nuestra, se
// conserva; nuestra acción se retira igual que antes.
func TestSyncCodexToolOutputHookKeepsThirdParty(t *testing.T) {
	base := []byte(`[[hooks.PostToolUse]]
matcher = "read"
[[hooks.PostToolUse.hooks]]
type = "command"
command = "bash /opt/acme/hooks/entry.sh hook tool-output"

[[hooks.PostToolUse]]
matcher = "hook tool-output"
[[hooks.PostToolUse.hooks]]
type = "command"
command = "acme-audit"

[[hooks.PostToolUse]]
[[hooks.PostToolUse.hooks]]
type = "command"
command = "mem hook tool-output codex"
[[hooks.PostToolUse.hooks]]
type = "command"
command = "acme-junto"

[[hooks.PreToolUse]]
[[hooks.PreToolUse.hooks]]
type = "command"
command = "bash /opt/acme/hooks/entry.sh hook tool-output"
`)
	off, changed, err := syncCodexToolOutputHook(base, "mem", false)
	if err != nil || !changed {
		t.Fatalf("desactivar: changed=%v err=%v", changed, err)
	}
	if strings.Contains(string(off), "mem hook tool-output") {
		t.Errorf("la acción de gomemory debe retirarse:\n%s", off)
	}
	for _, want := range []string{`matcher = 'read'`, "acme-audit", "acme-junto"} {
		if !strings.Contains(string(off), want) && !strings.Contains(string(off), strings.ReplaceAll(want, "'", `"`)) {
			t.Errorf("se pierde %q:\n%s", want, off)
		}
	}
	if strings.Count(string(off), "entry.sh hook tool-output") != 2 {
		t.Errorf("los dos hooks ajenos con el literal deben conservarse:\n%s", off)
	}
	if _, changed, _ := syncCodexToolOutputHook(off, "mem", false); changed {
		t.Error("sin acción propia, desactivar no debe escribir")
	}
}
