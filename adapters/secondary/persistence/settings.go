package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	AutoApprove      bool     `json:"auto_approve"`
	AutoApproveTools []string `json:"auto_approve_tools"`
	// CodeGraphDisabled apaga el proveedor de grafo de código EXTERNO
	// (codebase-memory-mcp). Ausente/false = auto-detección activada.
	CodeGraphDisabled bool `json:"code_graph_disabled,omitempty"`
	// CodeGraphCommand permite apuntar a otro binario del proveedor. Vacío =
	// se busca "codebase-memory-mcp" en el PATH.
	CodeGraphCommand string `json:"code_graph_command,omitempty"`
	// Budget: techo blando (CARACTERES) de get_context. Semántica normalizada en
	// ReadSettings: ausente/0 → default; negativo → sin límite (opt-out).
	Budget int `json:"budget,omitempty"`
	// CompactThreshold: huella (caracteres emitidos/sesión) que dispara el
	// recordatorio de compactación. Ausente/0 → default; negativo → desactivado.
	CompactThreshold int `json:"compact_threshold,omitempty"`
	// DedupWindowDays: ventana (días) del dedup por identidad. Ausente/0 →
	// default; negativo → sin dedup por identidad.
	DedupWindowDays int `json:"dedup_window_days,omitempty"`
}

// Defaults de la huella de contexto (feature 008). En CARACTERES emitidos salvo
// DedupWindowDays (días). ~24k chars ≈ 6k tokens; ~48k chars ≈ 12k tokens.
const (
	DefaultBudget           = 24000
	DefaultCompactThreshold = 48000
	DefaultDedupWindowDays  = 7
)

func DefaultSettings() Settings {
	return Settings{
		AutoApprove:      false,
		AutoApproveTools: []string{"save_memory", "start_session", "end_session", "search_memories", "get_memory", "get_context", "judge_memories"},
		Budget:           DefaultBudget,
		CompactThreshold: DefaultCompactThreshold,
		DedupWindowDays:  DefaultDedupWindowDays,
	}
}

// applyFootprintDefaults normaliza los tunables de la feature 008 tras leer un
// settings.json que puede no traer las claves nuevas: valor 0 (ausente) toma el
// default; un valor negativo se conserva (opt-out explícito). Así la reducción
// de huella queda activa por defecto para bases previas sin romper el opt-out.
func applyFootprintDefaults(s *Settings) {
	if s.Budget == 0 {
		s.Budget = DefaultBudget
	}
	if s.CompactThreshold == 0 {
		s.CompactThreshold = DefaultCompactThreshold
	}
	if s.DedupWindowDays == 0 {
		s.DedupWindowDays = DefaultDedupWindowDays
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
	applyFootprintDefaults(&s)
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
