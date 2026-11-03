package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var agyRef = AgentRef{HookCommand: "mem", MCPCommand: "mem", MCPArgs: []string{"mcp"}}

func readAgyJSON(t *testing.T, home, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(agyConfigDir(home), name))
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func writeAgyFile(t *testing.T, home, name, content string) {
	t.Helper()
	dir := agyConfigDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func agyBackups(t *testing.T, home string) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(agyConfigDir(home), "*.bak-*"))
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestHCD015InstallAgy(t *testing.T) {
	home := t.TempDir()
	writeAgyFile(t, home, "mcp_config.json", `{"mcpServers":{"ajeno":{"command":"otro"}}}`)
	writeAgyFile(t, home, "hooks.json", `{"ajeno":{"Stop":[{"type":"command","command":"otro"}]}}`)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)

	changed, err := InstallAgyGlobal(home, agyRef, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 3 {
		t.Fatalf("archivos cambiados: %v", changed)
	}
	if len(agyBackups(t, home)) != 2 {
		t.Errorf("copias previas: %v", agyBackups(t, home))
	}
	servers := readAgyJSON(t, home, "mcp_config.json")["mcpServers"].(map[string]any)
	if servers["ajeno"] == nil || servers["gomemory"] == nil {
		t.Errorf("servidores: %v", servers)
	}
	hooks := readAgyJSON(t, home, "hooks.json")
	if hooks["ajeno"] == nil {
		t.Error("se perdió el hook ajeno")
	}
	group, _ := json.Marshal(hooks[agyHookGroup])
	for _, want := range []string{"mem hook agy SessionStart", "mem hook agy PreInvocation", "mem hook agy PreToolUse", "mem hook agy PostToolUse", "mem hook agy Stop"} {
		if !strings.Contains(string(group), want) {
			t.Errorf("falta %s en %s", want, group)
		}
	}
	if strings.Contains(string(group), "jq") || strings.Contains(string(group), ".claude") {
		t.Errorf("el grupo depende de jq o de Claude: %s", group)
	}

	again, err := InstallAgyGlobal(home, agyRef, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 || len(agyBackups(t, home)) != 2 {
		t.Errorf("no es idempotente: %v, %v", again, agyBackups(t, home))
	}

	writeAgyFile(t, home, "skills.json", `{roto`)
	if _, err := InstallAgyGlobal(home, agyRef, now); err == nil {
		t.Error("un JSON inválido debe rechazarse")
	}
	if b, _ := os.ReadFile(filepath.Join(agyConfigDir(home), "skills.json")); string(b) != `{roto` {
		t.Errorf("se tocó un archivo inválido: %s", b)
	}
}

func TestHCD015MigrateManualGroup(t *testing.T) {
	home := t.TempDir()
	manual := `{"gomemory-lifecycle":{"enabled":true,"Stop":[{"type":"command","command":"PAYLOAD=$(cat); jq -r . ~/.local/bin/mem hook turn-end"}]}}`
	writeAgyFile(t, home, "hooks.json", manual)

	if _, err := InstallAgyGlobal(home, agyRef, time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	hooks := readAgyJSON(t, home, "hooks.json")
	if len(hooks) != 1 {
		t.Errorf("grupos duplicados: %v", hooks)
	}
	rendered, _ := json.Marshal(hooks)
	if strings.Contains(string(rendered), "jq") || strings.Count(string(rendered), "mem hook agy Stop") != 1 {
		t.Errorf("no se reemplazó el grupo manual: %s", rendered)
	}
	backups := agyBackups(t, home)
	if len(backups) == 0 {
		t.Fatal("falta la copia previa")
	}
	found := false
	for _, path := range backups {
		if b, _ := os.ReadFile(path); string(b) == manual {
			found = true
		}
	}
	if !found {
		t.Error("la copia previa no conserva el grupo manual")
	}
}
