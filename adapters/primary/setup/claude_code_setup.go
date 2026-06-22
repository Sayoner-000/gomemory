package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func InstallClaudeCode(root, binPath string, port int) error {
	ctx := &PluginContext{
		ProjectRoot: root,
		BinPath:     binPath,
		Port:        port,
	}

	pluginDir := filepath.Join(root, ".claude", "plugins", "gomemory")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("create claude plugin dir: %w", err)
	}

	count, err := InstallPlugin(PluginFS, "plugin/claude-code", pluginDir, ctx)
	if err != nil {
		return fmt.Errorf("install claude-code plugin: %w", err)
	}
	if count > 0 {
		fmt.Printf("  ✅ claude-code: plugin instalado en %s\n", pluginDir)
	} else {
		fmt.Println("  ✅ claude-code: plugin ya instalado")
	}

	mcpPath := filepath.Join(root, ".mcp.json")
	mcpCfg := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"gomemory": map[string]interface{}{
				"command": binPath,
				"args":    []string{"mcp", "--root", root},
			},
		},
	}

	var existing map[string]interface{}
	if data, _ := os.ReadFile(mcpPath); len(data) > 0 {
		json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = mcpCfg
	} else {
		ms, _ := existing["mcpServers"].(map[string]interface{})
		if ms == nil {
			ms = make(map[string]interface{})
		}
		ms["gomemory"] = map[string]interface{}{
			"command": binPath,
			"args":    []string{"mcp", "--root", root},
		}
		existing["mcpServers"] = ms
	}

	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		return fmt.Errorf("write .mcp.json: %w", err)
	}
	fmt.Printf("  ✅ claude-code: MCP configurado en %s\n", mcpPath)

	settingsDir := filepath.Join(root, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.json")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	hooksCfg := map[string]interface{}{
		"hooks": map[string]interface{}{
			"SessionStart":     []string{filepath.Join(pluginDir, "scripts", "session-start.sh")},
			"PreCompact":       []string{filepath.Join(pluginDir, "scripts", "post-compaction.sh")},
			"UserPromptSubmit": []string{filepath.Join(pluginDir, "scripts", "user-prompt-submit.sh")},
			"SessionEnd":       []string{filepath.Join(pluginDir, "scripts", "session-stop.sh")},
		},
	}

	var settings map[string]interface{}
	if data, _ := os.ReadFile(settingsPath); len(data) > 0 {
		json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = hooksCfg
	} else {
		h, _ := settings["hooks"].(map[string]interface{})
		if h == nil {
			h = make(map[string]interface{})
		}
		if _, has := h["SessionStart"]; !has {
			h["SessionStart"] = hooksCfg["hooks"].(map[string]interface{})["SessionStart"]
			h["PreCompact"] = hooksCfg["hooks"].(map[string]interface{})["PreCompact"]
			h["UserPromptSubmit"] = hooksCfg["hooks"].(map[string]interface{})["UserPromptSubmit"]
			h["SessionEnd"] = hooksCfg["hooks"].(map[string]interface{})["SessionEnd"]
		}
		settings["hooks"] = h
	}

	data2, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data2, 0644); err != nil {
		return fmt.Errorf("write .claude/settings.json: %w", err)
	}
	fmt.Printf("  ✅ claude-code: hooks configurados en %s\n", settingsPath)
	return nil
}
