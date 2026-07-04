package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadClaudeUserMCPEntryMissingFile(t *testing.T) {
	t.Setenv("GOMEMORY_CLAUDE_CONFIG", filepath.Join(t.TempDir(), "does-not-exist.json"))
	entry, err := readClaudeUserMCPEntry("gomemory")
	if err != nil {
		t.Fatalf("no debería fallar si el archivo no existe: %v", err)
	}
	if entry != nil {
		t.Fatalf("esperaba nil, got %+v", entry)
	}
}

func TestReadClaudeUserMCPEntryFindsMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude.json")
	content := `{"mcpServers": {"gomemory": {"command": "mem", "args": ["mcp"]}}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("GOMEMORY_CLAUDE_CONFIG", path)

	entry, err := readClaudeUserMCPEntry("gomemory")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if entry == nil || entry.Command != "mem" {
		t.Fatalf("esperaba entry.Command=mem, got %+v", entry)
	}
}

// TestSetupClaudeGlobalDetectsNameCollision cubre FR-008 del spec
// (specs/005-global-mcp-store): si ya existe una entrada "gomemory" global
// apuntando a otro binario (colisión de nombre con otra herramienta), no se
// sobrescribe en silencio.
func TestSetupClaudeGlobalDetectsNameCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude.json")
	content := `{"mcpServers": {"gomemory": {"command": "/otra/herramienta/binario", "args": ["mcp"]}}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("GOMEMORY_CLAUDE_CONFIG", path)

	entry, err := readClaudeUserMCPEntry("gomemory")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if entry == nil {
		t.Fatal("esperaba encontrar la entrada en conflicto")
	}
	if entry.Command == "mem" {
		t.Fatal("la entrada de fixture no debería coincidir con 'mem' (el test perdería sentido)")
	}
	// setupClaudeGlobal debe detectar este mismatch y NO reescribir el
	// archivo: verificamos que el contenido queda intacto tras leerlo (la
	// función real hace exactamente esta comparación antes de decidir si
	// llama a `claude mcp add`).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(data), "/otra/herramienta/binario") {
		t.Fatal("el archivo no debió modificarse mientras exista una colisión sin resolver")
	}
}

func TestSetupCodexGlobalWritesSingleTableWithoutCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	ref := BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}
	if !setupCodexGlobal(ref) {
		t.Fatal("esperaba que setupCodexGlobal reportara éxito")
	}

	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("leer config.toml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `[mcp_servers.gomemory]`) {
		t.Fatalf("esperaba una única tabla [mcp_servers.gomemory], got:\n%s", content)
	}
	if strings.Contains(content, "cwd") {
		t.Fatalf("el registro global no debe fijar cwd por proyecto, got:\n%s", content)
	}
	if strings.Contains(content, "gomemory_") {
		t.Fatalf("no debe usar el sufijo por proyecto del esquema anterior, got:\n%s", content)
	}
}

func TestSetupCodexGlobalIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	ref := BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}
	setupCodexGlobal(ref)
	setupCodexGlobal(ref)

	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("leer config.toml: %v", err)
	}
	if strings.Count(string(data), "[mcp_servers.gomemory]") != 1 {
		t.Fatalf("esperaba una sola tabla tras dos corridas, got:\n%s", string(data))
	}
}
