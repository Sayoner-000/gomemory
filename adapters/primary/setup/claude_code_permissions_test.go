package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func readSettings(t *testing.T, root string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal settings.json: %v", err)
	}
	return out
}

func allowList(t *testing.T, settings map[string]interface{}) []string {
	t.Helper()
	perms, _ := settings["permissions"].(map[string]interface{})
	raw, _ := perms["allow"].([]interface{})
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func TestWriteClaudePermissionsAddsSafeToolsOnly(t *testing.T) {
	root := t.TempDir()
	if err := writeClaudePermissions(root); err != nil {
		t.Fatalf("writeClaudePermissions: %v", err)
	}

	settings := readSettings(t, root)
	allow := allowList(t, settings)

	// Octopus está apagado por defecto (sin .memory/settings.json): sus 4
	// tools NO deben aparecer en permissions.allow (huella cero,
	// specs/029-octopus-aar/contracts/module-off.md; ACR 029, hallazgo A-001).
	for _, tool := range claudeAutoAllowToolsFor(false) {
		if !contains(allow, tool) {
			t.Errorf("esperaba %q en permissions.allow, no está: %v", tool, allow)
		}
	}
	for _, tool := range domain.MCPPrefixed("mcp__gomemory__", domain.MCPOctopusTools) {
		if contains(allow, tool) {
			t.Errorf("Octopus está apagado, %q NO debe estar pre-aprobada: %v", tool, allow)
		}
	}
	if contains(allow, "mcp__gomemory__forget_memory") {
		t.Error("forget_memory NO debe pre-aprobarse (destructiva/irreversible)")
	}

	servers, _ := settings["enabledMcpjsonServers"].([]interface{})
	found := false
	for _, s := range servers {
		if s == "gomemory" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperaba 'gomemory' en enabledMcpjsonServers, got: %v", servers)
	}
}

func TestWriteClaudePermissionsIncludesOctopusToolsWhenEnabled(t *testing.T) {
	root := t.TempDir()
	if err := persistence.WriteSettings(root, persistence.Settings{OctopusEnabled: true}); err != nil {
		t.Fatalf("WriteSettings: %v", err)
	}
	if err := writeClaudePermissions(root); err != nil {
		t.Fatalf("writeClaudePermissions: %v", err)
	}

	allow := allowList(t, readSettings(t, root))
	for _, tool := range domain.MCPPrefixed("mcp__gomemory__", domain.MCPOctopusTools) {
		if !contains(allow, tool) {
			t.Errorf("Octopus está encendido, esperaba %q pre-aprobada: %v", tool, allow)
		}
	}
}

func TestWriteClaudePermissionsIsIdempotentAndPreservesThirdParty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	existing := `{
		"permissions": {"allow": ["mcp__other_server__some_tool"]},
		"enabledMcpjsonServers": ["other_server"]
	}`
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(existing), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	if err := writeClaudePermissions(root); err != nil {
		t.Fatalf("writeClaudePermissions: %v", err)
	}
	if err := writeClaudePermissions(root); err != nil {
		t.Fatalf("writeClaudePermissions (2nd run): %v", err)
	}

	settings := readSettings(t, root)
	allow := allowList(t, settings)

	if !contains(allow, "mcp__other_server__some_tool") {
		t.Error("no debió eliminar el permiso de un servidor de terceros")
	}
	if !contains(allow, "mcp__gomemory__save_memory") {
		t.Error("esperaba que save_memory esté pre-aprobado")
	}

	count := 0
	for _, v := range allow {
		if v == "mcp__gomemory__save_memory" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("esperaba exactamente 1 entrada de save_memory tras correr dos veces, got %d", count)
	}

	servers, _ := settings["enabledMcpjsonServers"].([]interface{})
	hasOther, hasGomemory := false, false
	for _, s := range servers {
		if s == "other_server" {
			hasOther = true
		}
		if s == "gomemory" {
			hasGomemory = true
		}
	}
	if !hasOther {
		t.Error("no debió eliminar el server de terceros de enabledMcpjsonServers")
	}
	if !hasGomemory {
		t.Error("esperaba 'gomemory' en enabledMcpjsonServers")
	}
}

func TestWriteClaudePermissionsRemovesStaleEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	existing := `{"permissions": {"allow": ["mcp__plugin_engram_engram__mem_search"]}}`
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(existing), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	if err := writeClaudePermissions(root); err != nil {
		t.Fatalf("writeClaudePermissions: %v", err)
	}

	allow := allowList(t, readSettings(t, root))
	if contains(allow, "mcp__plugin_engram_engram__mem_search") {
		t.Error("la entrada obsoleta de un servidor MCP difunto debió limpiarse")
	}
}

func TestRemoveClaudePermissionsPreservesThirdParty(t *testing.T) {
	root := t.TempDir()
	if err := writeClaudePermissions(root); err != nil {
		t.Fatalf("writeClaudePermissions: %v", err)
	}

	// Agregar manualmente un permiso de terceros antes de desinstalar.
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	settings := readSettings(t, root)
	perms := settings["permissions"].(map[string]interface{})
	allow := perms["allow"].([]interface{})
	allow = append(allow, "mcp__other_server__some_tool")
	perms["allow"] = allow
	settings["permissions"] = perms
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	changed, err := RemoveClaudePermissions(root)
	if err != nil {
		t.Fatalf("RemoveClaudePermissions: %v", err)
	}
	if !changed {
		t.Fatal("esperaba changed=true, había entradas de gomemory para remover")
	}

	after := readSettings(t, root)
	allowAfter := allowList(t, after)
	if contains(allowAfter, "mcp__gomemory__save_memory") {
		t.Error("las tools de gomemory debieron removerse")
	}
	if !contains(allowAfter, "mcp__other_server__some_tool") {
		t.Error("no debió eliminar el permiso de un servidor de terceros")
	}

	servers, _ := after["enabledMcpjsonServers"].([]interface{})
	for _, s := range servers {
		if s == "gomemory" {
			t.Error("'gomemory' debió removerse de enabledMcpjsonServers")
		}
	}
}

// TestClaudePermissions_IncluyeGetPlanContext cubre la feature 013: la tool de
// planificación debe quedar pre-aprobada. Sin esto, cada entrada en modo plan
// queda pidiendo permiso y la activación autónoma deja de ser autónoma — el
// propio comentario de writeClaudePermissions señala esa omisión como la causa
// más común de que el protocolo de memoria no se aplique.
func TestClaudePermissions_IncluyeGetPlanContext(t *testing.T) {
	root := t.TempDir()
	if err := writeClaudePermissions(root); err != nil {
		t.Fatalf("writeClaudePermissions: %v", err)
	}

	allow := allowList(t, readSettings(t, root))
	for _, e := range allow {
		if e == "mcp__gomemory__get_plan_context" {
			return
		}
	}
	t.Errorf("falta mcp__gomemory__get_plan_context en permissions.allow; hay: %v", allow)
}

// TestClaudeAutoAllowTools_ExcluyeForgetMemory protege una decisión de seguridad
// ya tomada: forget_memory es irreversible y NO debe pre-aprobarse. Se verifica
// aquí para que una adición futura no la cuele por descuido.
func TestClaudeAutoAllowTools_ExcluyeForgetMemory(t *testing.T) {
	for _, tool := range ClaudeAutoAllowTools {
		if tool == "mcp__gomemory__forget_memory" {
			t.Error("forget_memory no debe pre-aprobarse: es destructiva e irreversible")
		}
	}
}
