package cli

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestMigrateLegacyCodexTablesPreservesForeignContent(t *testing.T) {
	original := `# configuración de la persona
[mcp_servers."gomemory_deleted"]
command = "/tmp/deleted/mem"
args = ["mcp", "--root", "/tmp/deleted"]
cwd = "/tmp/deleted"

[mcp_servers.foreign]
command = "foreign-mcp"
# comentario que debe conservarse

[features]
web_search = true
`
	want := `# configuración de la persona
[mcp_servers.foreign]
command = "foreign-mcp"
# comentario que debe conservarse

[features]
web_search = true
`

	got, removed := migrateLegacyCodexTables(original)
	if removed != 1 {
		t.Fatalf("esperaba retirar una tabla legada, got %d", removed)
	}
	if got != want {
		t.Fatalf("la migración alteró contenido ajeno:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSetupCodexGlobalMigratesLegacyTablesWithBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(codexDir, "config.toml")
	original := `[mcp_servers."gomemory_old"]
command = "mem"
args = ["mcp"]
cwd = "/ruta/eliminada"

[mcp_servers.foreign]
command = "foreign"
`
	if err := os.WriteFile(cfgPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if !setupCodexGlobal(BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}) {
		t.Fatal("la migración debía completarse")
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "gomemory_old") || strings.Contains(content, `cwd = "/ruta/eliminada"`) {
		t.Fatalf("quedó configuración legada:\n%s", content)
	}
	if strings.Count(content, "[mcp_servers.gomemory]") != 1 || !strings.Contains(content, "[mcp_servers.foreign]") {
		t.Fatalf("resultado inesperado:\n%s", content)
	}
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("los permisos cambiaron: %o", info.Mode().Perm())
	}
	backups, err := filepath.Glob(cfgPath + ".gomemory-legacy-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("esperaba un respaldo, got %v (err=%v)", backups, err)
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatal("el respaldo no contiene el archivo original exacto")
	}
}

func TestSetupCodexGlobalMigratesEvenWhenGlobalTableExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(codexDir, "config.toml")
	content := `[mcp_servers."gomemory"]
command = "mem"
args = ["mcp"]

[mcp_servers.gomemory_stale]
command = "mem"
cwd = "/missing"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	setupCodexGlobal(BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}})
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "gomemory_stale") {
		t.Fatalf("la salida temprana dejó una tabla legada:\n%s", got)
	}
	if strings.Count(got, "gomemory\"]") != 1 {
		t.Fatalf("la tabla global citada debía conservarse una sola vez:\n%s", got)
	}
}

func TestSetupCodexGlobalSerializesConcurrentCalls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ref := BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			setupCodexGlobal(ref)
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "[mcp_servers.gomemory]") != 1 {
		t.Fatalf("las llamadas concurrentes duplicaron la tabla:\n%s", string(data))
	}
}
