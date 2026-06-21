package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type MCPEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type OpenCodeConfig struct {
	MCPServers map[string]MCPEntry `json:"mcpServers"`
}

func InstallOpenCode(root, binPath string, port int) error {
	ctx := &PluginContext{
		ProjectRoot: root,
		BinPath:     binPath,
		Port:        port,
	}

	pluginDir := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "plugins", "gomemory")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("create opencode plugin dir: %w", err)
	}

	count, err := InstallPlugin(PluginFS, "plugin/opencode", pluginDir, ctx)
	if err != nil {
		return fmt.Errorf("install opencode plugin: %w", err)
	}
	if count > 0 {
		fmt.Printf("  ✅ opencode: plugin instalado en %s\n", pluginDir)
	} else {
		fmt.Println("  ✅ opencode: plugin ya instalado")
	}

	// OpenCode auto-descubre plugins de ~/.config/opencode/plugins/<name>/

	cfgPath := filepath.Join(root, ".opencode.json")
	entry := MCPEntry{
		Command: binPath,
		Args:    []string{"mcp", "--root", root},
	}

	var cfg OpenCodeConfig
	if data, err := os.ReadFile(cfgPath); err == nil {
		json.Unmarshal(data, &cfg)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPEntry)
	}
	cfg.MCPServers["gomemory"] = entry

	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("write .opencode.json: %w", err)
	}
	fmt.Printf("  ✅ opencode: MCP configurado en %s\n", cfgPath)
	return nil
}
