package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	AutoApprove      bool     `json:"auto_approve"`
	AutoApproveTools []string `json:"auto_approve_tools"`
}

func DefaultSettings() Settings {
	return Settings{
		AutoApprove:      false,
		AutoApproveTools: []string{"save_memory", "start_session", "end_session", "search_memories", "get_memory", "get_context", "judge_memories"},
	}
}

func SettingsPath(root string) string {
	return filepath.Join(root, MemDir, "settings.json")
}

func ReadSettings(root string) Settings {
	path := SettingsPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultSettings()
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings()
	}
	return s
}

func WriteSettings(root string, s Settings) error {
	path := SettingsPath(root)
	if err := EnsureDir(root); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ApplyAutoApprove(root string, s Settings) {
	if !s.AutoApprove || len(s.AutoApproveTools) == 0 {
		return
	}
	tools := s.AutoApproveTools
	setAAP := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return
		}
		ms, ok := cfg["mcpServers"].(map[string]interface{})
		if !ok {
			return
		}
		entry, ok := ms["gomemory"].(map[string]interface{})
		if !ok {
			return
		}
		entry["autoApprove"] = tools
		ms["gomemory"] = entry
		cfg["mcpServers"] = ms
		out, _ := json.MarshalIndent(cfg, "", "  ")
		os.WriteFile(path, out, 0644)
	}
	removeAAP := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return
		}
		ms, ok := cfg["mcpServers"].(map[string]interface{})
		if !ok {
			return
		}
		entry, ok := ms["gomemory"].(map[string]interface{})
		if !ok {
			return
		}
		delete(entry, "autoApprove")
		ms["gomemory"] = entry
		cfg["mcpServers"] = ms
		out, _ := json.MarshalIndent(cfg, "", "  ")
		os.WriteFile(path, out, 0644)
	}

	paths := []string{
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, ".cursor", "mcp.json"),
		filepath.Join(root, ".windsurf", "mcp_config.json"),
		filepath.Join(root, ".cline", "mcp_settings.json"),
	}
	for _, p := range paths {
		if s.AutoApprove {
			setAAP(p)
		} else {
			removeAAP(p)
		}
	}
}
