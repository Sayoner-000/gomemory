package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func InstallOpenCode(root string, ref AgentRef) error {
	ctx := &PluginContext{
		ProjectRoot: root,
		BinPath:     ref.MCPCommand,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("obtener home del usuario: %w", err)
	}

	// OpenCode auto-descubre plugins como archivos sueltos en
	// ~/.config/opencode/plugins/ (no en subcarpetas). Instalamos gomemory.ts
	// directamente ahí.
	pluginsDir := filepath.Join(homeDir, ".config", "opencode", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("create opencode plugins dir: %w", err)
	}

	// Limpiar la instalación legada anidada (~/.config/opencode/plugins/gomemory/),
	// que OpenCode nunca cargaba.
	_ = os.RemoveAll(filepath.Join(pluginsDir, "gomemory"))

	count, err := InstallPlugin(PluginFS, "plugin/opencode", pluginsDir, ctx)
	if err != nil {
		return fmt.Errorf("install opencode plugin: %w", err)
	}
	if count > 0 {
		fmt.Printf("  ✅ opencode: plugin instalado en %s\n", filepath.Join(pluginsDir, "gomemory.ts"))
	} else {
		fmt.Println("  ✅ opencode: plugin ya instalado")
	}

	if err := WriteOpenCodeMCP(root, ref); err != nil {
		return err
	}
	fmt.Printf("  ✅ opencode: MCP configurado en %s\n", filepath.Join(root, "opencode.json"))
	return nil
}

// WriteOpenCodeMCP escribe/actualiza la entrada gomemory en opencode.json con el
// esquema real que espera OpenCode: clave "mcp", type "local" y "command" como
// arreglo. Antes usábamos .opencode.json con "mcpServers"+{command,args}, formato
// que OpenCode ignora por completo (de ahí que las tools nunca aparecieran).
// Es idempotente y preserva el resto de la config.
func WriteOpenCodeMCP(root string, ref AgentRef) error {
	cfgPath := filepath.Join(root, "opencode.json")

	cfg := map[string]interface{}{}
	if data, _ := os.ReadFile(cfgPath); len(data) > 0 {
		json.Unmarshal(data, &cfg)
	}
	if _, ok := cfg["$schema"]; !ok {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}

	mcp, _ := cfg["mcp"].(map[string]interface{})
	if mcp == nil {
		mcp = map[string]interface{}{}
	}
	command := append([]string{ref.MCPCommand}, ref.MCPArgs...)
	mcp["gomemory"] = map[string]interface{}{
		"type":    "local",
		"command": command,
		"enabled": true,
	}
	cfg["mcp"] = mcp

	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("write opencode.json: %w", err)
	}

	// Limpiar el .opencode.json legado si era nuestro artefacto (tenía la entrada
	// gomemory en mcpServers), para no dejar config muerta que confunda.
	cleanupLegacyOpenCodeConfig(filepath.Join(root, ".opencode.json"))
	return nil
}

func cleanupLegacyOpenCodeConfig(legacyPath string) {
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return
	}
	var legacy map[string]interface{}
	if json.Unmarshal(data, &legacy) != nil {
		return
	}
	ms, _ := legacy["mcpServers"].(map[string]interface{})
	if ms == nil {
		return
	}
	if _, ours := ms["gomemory"]; !ours {
		return
	}
	delete(ms, "gomemory")
	if len(ms) == 0 {
		delete(legacy, "mcpServers")
	} else {
		legacy["mcpServers"] = ms
	}
	// Si no queda nada útil, borrar el archivo; si no, reescribir sin gomemory.
	if len(legacy) == 0 {
		_ = os.Remove(legacyPath)
		return
	}
	out, _ := json.MarshalIndent(legacy, "", "  ")
	_ = os.WriteFile(legacyPath, out, 0644)
}
