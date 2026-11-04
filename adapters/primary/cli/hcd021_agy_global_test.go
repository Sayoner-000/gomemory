package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// HCD-021: con agy en el defecto de setup-mcp global, una máquina sin agy no
// debe ganar un ~/.gemini/config vacío de gomemory.
func TestSetupAgyGlobal_SinAgyNoCreaConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", t.TempDir())
	if setupAgyGlobal(BinRef{HookCommand: "mem", MCPCommand: "mem", MCPArgs: []string{"mcp"}}) {
		t.Fatal("sin agy instalado no hay registro que informar")
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini")); !os.IsNotExist(err) {
		t.Fatalf("no debe crear ~/.gemini sin agy: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".gemini", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !setupAgyGlobal(BinRef{HookCommand: "mem", MCPCommand: "mem", MCPArgs: []string{"mcp"}}) {
		t.Fatal("con ~/.gemini/config presente, agy se registra")
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "config", "mcp_config.json")); err != nil {
		t.Fatalf("mcp_config.json no escrito: %v", err)
	}
}
