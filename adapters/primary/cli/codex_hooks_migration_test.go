package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestConsolidateCodexHooksPreservesAndDeduplicatesAnyCommand(t *testing.T) {
	config := []byte(`[features]
hooks = true

[[hooks.SessionStart]]
matcher = "startup|resume"

[[hooks.SessionStart.hooks]]
type = "command"
command = "mem hook session-start"
timeout = 10

[[hooks.SessionStart]]
matcher = "startup|resume"

[[hooks.SessionStart.hooks]]
type = "command"
command = "mem hook session-start"
timeout = 10

[hooks.state."/tmp/hooks.json:session_start:0:0"]
trusted_hash = "obsolete"

[mcp_servers.foreign]
command = "foreign"
`)
	legacy := []byte(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"mem hook session-start","timeout":10}],"matcher":"startup|resume"},{"hooks":[{"type":"command","command":"another-tool start","timeout":7}]}],"Notification":[{"hooks":[{"type":"command","command":"notify"}],"custom":{"keep":true}}]}}`)

	got, migrated, err := consolidateCodexHooks(config, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if migrated != 2 {
		t.Fatalf("esperaba migrar dos grupos únicos, got %d\n%s", migrated, got)
	}
	if strings.Count(string(got), `command = 'mem hook session-start'`) != 1 && strings.Count(string(got), `command = "mem hook session-start"`) != 1 {
		t.Fatalf("el grupo equivalente debía quedar una sola vez:\n%s", got)
	}
	if !strings.Contains(string(got), "another-tool start") || !strings.Contains(string(got), "notify") {
		t.Fatalf("se perdieron hooks ajenos:\n%s", got)
	}
	if strings.Contains(string(got), "obsolete") || !strings.Contains(string(got), "[mcp_servers.foreign]") {
		t.Fatalf("el estado obsoleto debía retirarse sin tocar config ajena:\n%s", got)
	}
	var decoded map[string]any
	if err := toml.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("resultado TOML inválido: %v", err)
	}
}

func TestSetupCodexGlobalConsolidatesHooksJSONWithBackupAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(codexDir, "config.toml")
	hooksPath := filepath.Join(codexDir, "hooks.json")
	if err := os.WriteFile(configPath, []byte("[features]\nhooks = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hooksPath, []byte(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"vendor-neutral start","timeout":10}]}]}}`), 0600); err != nil {
		t.Fatal(err)
	}

	ref := BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}
	if !setupCodexGlobal(ref) || !setupCodexGlobal(ref) {
		t.Fatal("la instalación y su repetición debían completar")
	}
	if _, err := os.Stat(hooksPath); !os.IsNotExist(err) {
		t.Fatalf("hooks.json debía retirarse, err=%v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "vendor-neutral start") != 1 || strings.Count(string(data), "[mcp_servers.gomemory]") != 1 {
		t.Fatalf("la migración no fue idempotente:\n%s", data)
	}
	if info, err := os.Stat(configPath); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("los permisos de config.toml cambiaron: info=%v err=%v", info, err)
	}
	backups, err := filepath.Glob(hooksPath + ".gomemory-legacy-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("esperaba respaldo recuperable de hooks.json: %v, err=%v", backups, err)
	}
}

func TestSetupCodexGlobalKeepsInvalidHooksJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(codexDir, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(`{"hooks":`), 0600); err != nil {
		t.Fatal(err)
	}
	if !setupCodexGlobal(BinRef{MCPCommand: "mem", MCPArgs: []string{"mcp"}}) {
		t.Fatal("un JSON ajeno inválido no debe impedir registrar el MCP")
	}
	if _, err := os.Stat(hooksPath); err != nil {
		t.Fatalf("el JSON inválido debía conservarse: %v", err)
	}
}
