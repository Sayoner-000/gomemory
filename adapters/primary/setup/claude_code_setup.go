package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentRef describe cómo referenciar el binario `mem` de forma portable en la
// config de un agente. Lo construye el paquete cli (binRefFor) y se pasa aquí
// para que el paquete setup no dependa de la lógica de resolución de PATH.
type AgentRef struct {
	// HookCommand es el prefijo del comando de hooks (ej. "mem" o
	// "${CLAUDE_PROJECT_DIR}/mem"). Se le concatena " hook <evento>".
	HookCommand string
	// MCPCommand es el "command" del server MCP (ej. "mem").
	MCPCommand string
	// MCPArgs son los argumentos del server MCP (ej. ["mcp"]).
	MCPArgs []string
}

// claudeHookEvents mapea cada evento de hook de Claude Code al subcomando
// `mem hook <evento>` que lo implementa de forma portable (sin scripts .sh).
var claudeHookEvents = map[string]string{
	"SessionStart":     "session-start",
	"PreCompact":       "pre-compact",
	"UserPromptSubmit": "user-prompt-submit",
	"SessionEnd":       "session-end",
}

func InstallClaudeCode(root string, ref AgentRef) error {
	ctx := &PluginContext{
		ProjectRoot: root,
		BinPath:     ref.HookCommand,
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

	if err := writeMCPConfig(filepath.Join(root, ".mcp.json"), ref); err != nil {
		return err
	}
	fmt.Printf("  ✅ claude-code: MCP configurado en %s\n", filepath.Join(root, ".mcp.json"))

	if err := writeClaudeHooks(root, ref); err != nil {
		return err
	}
	fmt.Printf("  ✅ claude-code: hooks portables configurados en %s\n", filepath.Join(root, ".claude", "settings.json"))
	return nil
}

// writeMCPConfig escribe/actualiza la entrada gomemory en un archivo .mcp.json
// de forma portable (command por PATH, sin --root absoluto).
func writeMCPConfig(mcpPath string, ref AgentRef) error {
	entry := map[string]interface{}{
		"command": ref.MCPCommand,
		"args":    ref.MCPArgs,
	}

	existing := map[string]interface{}{}
	if data, _ := os.ReadFile(mcpPath); len(data) > 0 {
		json.Unmarshal(data, &existing)
	}
	ms, _ := existing["mcpServers"].(map[string]interface{})
	if ms == nil {
		ms = map[string]interface{}{}
	}
	ms["gomemory"] = entry
	existing["mcpServers"] = ms

	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(mcpPath, data, 0644); err != nil {
		return fmt.Errorf("write .mcp.json: %w", err)
	}
	return nil
}

// writeClaudeHooks escribe los hooks de gomemory en .claude/settings.json en el
// formato objeto que espera Claude Code, usando `mem hook <evento>` (portable).
// Antes de añadir, elimina cualquier entrada previa de gomemory (incluidas las
// rutas absolutas rotas de instalaciones anteriores entre máquinas).
func writeClaudeHooks(root string, ref AgentRef) error {
	settingsDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	settingsPath := filepath.Join(settingsDir, "settings.json")

	settings := map[string]interface{}{}
	if data, _ := os.ReadFile(settingsPath); len(data) > 0 {
		json.Unmarshal(data, &settings)
	}
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
	}

	for event, sub := range claudeHookEvents {
		command := ref.HookCommand + " hook " + sub
		kept := filterOutGomemoryHooks(hooks[event])
		kept = append(kept, map[string]interface{}{
			"matcher": "",
			"hooks": []interface{}{
				map[string]interface{}{"type": "command", "command": command},
			},
		})
		hooks[event] = kept
	}
	settings["hooks"] = hooks

	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("write .claude/settings.json: %w", err)
	}
	return nil
}

// filterOutGomemoryHooks devuelve las entradas de hooks que NO son de gomemory,
// preservando hooks de terceros. Reconoce entradas de gomemory por su comando:
// el subcomando `hook` del binario, o rutas legadas a scripts del plugin.
func filterOutGomemoryHooks(raw interface{}) []interface{} {
	entries, ok := raw.([]interface{})
	if !ok {
		return []interface{}{}
	}
	kept := make([]interface{}, 0, len(entries))
	for _, e := range entries {
		if IsGomemoryHookEntry(e) {
			continue
		}
		kept = append(kept, e)
	}
	return kept
}

// IsGomemoryHookEntry detecta si una entrada de hook pertenece a gomemory,
// soportando tanto el formato objeto nuevo como el legado ([]string a scripts).
// Lo usa también el desinstalador para limpiar hooks sin tocar los de terceros.
func IsGomemoryHookEntry(e interface{}) bool {
	switch v := e.(type) {
	case string:
		return hookCommandIsGomemory(v)
	case map[string]interface{}:
		inner, _ := v["hooks"].([]interface{})
		for _, h := range inner {
			hm, _ := h.(map[string]interface{})
			if cmd, _ := hm["command"].(string); hookCommandIsGomemory(cmd) {
				return true
			}
		}
	}
	return false
}

func hookCommandIsGomemory(cmd string) bool {
	return strings.Contains(cmd, "hook session-start") ||
		strings.Contains(cmd, "hook session-end") ||
		strings.Contains(cmd, "hook pre-compact") ||
		strings.Contains(cmd, "hook user-prompt-submit") ||
		strings.Contains(cmd, filepath.Join("plugins", "gomemory")) ||
		strings.Contains(cmd, "plugins/gomemory")
}
