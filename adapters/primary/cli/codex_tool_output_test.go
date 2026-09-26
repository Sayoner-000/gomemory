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

// C-006 (acr_961a1676) — el hook vive en la configuración GLOBAL de Codex y
// comprueba el ajuste de cada proyecto al ejecutarse. Instalar en un proyecto
// con el ajuste apagado no puede retirárselo a los que lo tienen activo.
func TestSyncCodexToolOutputNoRetiraElHookGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	on, _, err := syncCodexToolOutputHook([]byte("[mcp_servers.gomemory]\ncommand = \"mem\"\n"), "mem", true)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfg, on, 0o644); err != nil {
		t.Fatal(err)
	}

	syncCodexToolOutput(t.TempDir(), "mem") // proyecto sin el ajuste activo

	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "hook tool-output codex") {
		t.Errorf("el hook global no debe retirarse por el ajuste de un proyecto:\n%s", got)
	}
}
