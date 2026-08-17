package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/domain"
	"mem/version"
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

// hookReg es una entrada de hook a registrar para un evento de Claude Code: el
// subcomando `mem hook <sub>` que lo implementa (portable, sin scripts .sh) y el
// matcher que lo dispara ("" = todos los orígenes del evento).
type hookReg struct {
	matcher string
	sub     string
}

// claudeHookEvents mapea cada evento de hook de Claude Code a las entradas
// `mem hook <sub>` que lo implementan. SessionStart se divide por matcher:
// startup/resume/clear cargan la sesión y su contexto; compact re-inyecta la
// recuperación DESPUÉS de compactar (sobrevive a la compactación, a diferencia
// del antiguo PreCompact, ya retirado del registro).
//
// Se construye con buildClaudeHookEvents() en vez de un literal fijo: las
// entradas del modo plan atómico (feature 019) se derivan del registro único
// de capacidades (domain.KnownAgents) para que el nivel declarado ahí sea la
// única fuente de verdad — ver buildClaudeHookEvents.
var claudeHookEvents = buildClaudeHookEvents()

// buildClaudeHookEvents arma el mapa de eventos de Claude Code. Las entradas
// que no dependen de una capacidad concreta del modo plan atómico van fijas;
// PreToolUse(ExitPlanMode) → plan-guard se añade SOLO si domain.KnownAgents
// declara AgentLevelGuard para "claude" (feature 019, FR-A4): si ese nivel se
// retirara del registro algún día, este archivo no necesitaría tocarse.
func buildClaudeHookEvents() map[string][]hookReg {
	events := map[string][]hookReg{
		"SessionStart": {
			{matcher: "startup|resume|clear", sub: "session-start"},
			{matcher: "compact", sub: "post-compact"},
		},
		"UserPromptSubmit": {{matcher: "", sub: "user-prompt-submit"}},
		"SessionEnd":       {{matcher: "", sub: "session-end"}},
		"Stop":             {{matcher: "", sub: "turn-end"}},
		"SubagentStart":    {{matcher: "", sub: "subagent-start"}},
		"SubagentStop":     {{matcher: "", sub: "subagent-stop"}},
		// PostToolUse(ExitPlanMode): captura determinista de las decisiones al
		// aprobar un plan. Un turno de plan mode no toca archivos ni corre
		// comandos, así que el checkpoint de Stop/turn-end lo descarta por
		// vacío; este hook lo cubre.
		"PostToolUse": {{matcher: "ExitPlanMode", sub: "plan-approved"}},
	}

	// PreToolUse(ExitPlanMode) → plan-guard: el borde de salida determinista
	// del modo plan atómico (feature 019, Historia 1). Dispara ANTES de que
	// el plan llegue a la persona, para poder devolverlo si no tiene forma de
	// árbol (contracts/hook-plan-guard.md).
	if claude, ok := domain.AgentByName("claude"); ok && claude.HasLevel(domain.AgentLevelGuard) {
		events["PreToolUse"] = []hookReg{{matcher: "ExitPlanMode", sub: "plan-guard"}}
	}

	// PostToolUse(EnterPlanMode) → plan-entered: el borde de entrada, mejor
	// esfuerzo (feature 019, Historia 2). Verificado en vivo (T001-T004,
	// specs/019-deterministic-plan-trigger/research.md) que este evento
	// acepta hookSpecificOutput.additionalContext en esta versión de Claude
	// Code.
	if claude, ok := domain.AgentByName("claude"); ok && claude.HasLevel(domain.AgentLevelEntry) {
		events["PostToolUse"] = append(events["PostToolUse"],
			hookReg{matcher: "EnterPlanMode", sub: "plan-entered"})
	}

	return events
}

func InstallClaudeCode(root string, ref AgentRef) error {
	ctx := &PluginContext{
		ProjectRoot: root,
		BinPath:     ref.HookCommand,
		Version:     version.Version,
	}

	pluginDir := filepath.Join(root, ".claude", "plugins", "gomemory")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("create claude plugin dir: %w", err)
	}

	// Limpieza best-effort de artefactos legados de versiones anteriores del
	// plugin (scripts .sh, .mcp.json con ruta absoluta, hooks.json): ya no se
	// generan, pero pueden sobrevivir de una instalación previa sincronizada
	// desde otra máquina. InstallPlugin solo agrega/actualiza, no borra.
	os.RemoveAll(filepath.Join(pluginDir, "scripts"))
	os.RemoveAll(filepath.Join(pluginDir, "hooks"))
	os.Remove(filepath.Join(pluginDir, ".mcp.json"))

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

	if err := writeClaudePermissions(root); err != nil {
		return err
	}
	fmt.Printf("  ✅ claude-code: tools MCP pre-aprobadas en %s\n", filepath.Join(root, ".claude", "settings.json"))
	return nil
}

// ClaudeAutoAllowTools son las tools MCP de gomemory seguras para pre-aprobar
// automáticamente: de solo lectura, o de escritura acotada y reversible.
// forget_memory queda deliberadamente afuera por ser destructiva/irreversible.
// Se deriva de domain para que no pueda desincronizarse del servidor: son todas
// las tools registradas menos las destructivas (forget_memory). Las de grafo son
// de solo lectura salvo index_project, que solo escribe en .memory/ y nunca toca
// el código fuente.
var ClaudeAutoAllowTools = domain.MCPPrefixed("mcp__gomemory__", domain.MCPAutoApprovableTools())

// staleAllowPrefixes son prefijos de entradas de permisos obsoletas de
// instalaciones/servidores MCP previos que ya no existen, y que se limpian al
// reinstalar para no dejar basura pidiendo aprobación de tools inexistentes.
var staleAllowPrefixes = []string{
	"mcp__plugin_engram_engram__",
}

// writeClaudePermissions asegura que .claude/settings.json pre-apruebe las
// tools MCP seguras de gomemory (permissions.allow) y habilite el server sin
// el prompt de confianza de .mcp.json (enabledMcpjsonServers). Sin esto, cada
// llamada del agente a una tool gomemory queda bloqueada pidiendo permiso,
// que es la causa más común de que el protocolo de memoria no se aplique
// automáticamente. Idempotente: dedupe y preserva entradas de terceros.
func writeClaudePermissions(root string) error {
	settingsDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	settingsPath := filepath.Join(settingsDir, "settings.json")

	settings := map[string]interface{}{}
	if data, _ := os.ReadFile(settingsPath); len(data) > 0 {
		json.Unmarshal(data, &settings)
	}

	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		perms = map[string]interface{}{}
	}

	rawAllow, _ := perms["allow"].([]interface{})
	allow := make([]string, 0, len(rawAllow))
	seen := map[string]bool{}
	for _, v := range rawAllow {
		s, ok := v.(string)
		if !ok || seen[s] || isStaleAllowEntry(s) {
			continue
		}
		seen[s] = true
		allow = append(allow, s)
	}
	for _, tool := range ClaudeAutoAllowTools {
		if seen[tool] {
			continue
		}
		seen[tool] = true
		allow = append(allow, tool)
	}
	perms["allow"] = allow
	settings["permissions"] = perms

	rawServers, _ := settings["enabledMcpjsonServers"].([]interface{})
	servers := make([]string, 0, len(rawServers)+1)
	hasGomemory := false
	for _, v := range rawServers {
		s, ok := v.(string)
		if !ok {
			continue
		}
		servers = append(servers, s)
		if s == "gomemory" {
			hasGomemory = true
		}
	}
	if !hasGomemory {
		servers = append(servers, "gomemory")
	}
	settings["enabledMcpjsonServers"] = servers

	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("write .claude/settings.json: %w", err)
	}
	return nil
}

func isStaleAllowEntry(entry string) bool {
	for _, prefix := range staleAllowPrefixes {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

// RemoveClaudePermissions quita las entradas de permisos/servidores habilitados
// que gomemory agregó, preservando el resto. La usa CmdUninstall. Devuelve
// changed=false cuando no había nada que limpiar (sin settings.json, JSON
// inválido, o sin entradas de gomemory), para que el llamador pueda distinguir
// ese caso de una limpieza real.
func RemoveClaudePermissions(root string) (changed bool, err error) {
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false, nil // sin settings.json, nada que limpiar
	}

	settings := map[string]interface{}{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false, nil // JSON inválido, se conserva sin tocar
	}

	changed = false

	if perms, ok := settings["permissions"].(map[string]interface{}); ok {
		if rawAllow, ok := perms["allow"].([]interface{}); ok {
			kept := make([]interface{}, 0, len(rawAllow))
			gomemorySet := map[string]bool{}
			for _, t := range ClaudeAutoAllowTools {
				gomemorySet[t] = true
			}
			for _, v := range rawAllow {
				s, ok := v.(string)
				if ok && gomemorySet[s] {
					changed = true
					continue
				}
				kept = append(kept, v)
			}
			perms["allow"] = kept
		}
		settings["permissions"] = perms
	}

	if rawServers, ok := settings["enabledMcpjsonServers"].([]interface{}); ok {
		kept := make([]interface{}, 0, len(rawServers))
		for _, v := range rawServers {
			s, ok := v.(string)
			if ok && s == "gomemory" {
				changed = true
				continue
			}
			kept = append(kept, v)
		}
		settings["enabledMcpjsonServers"] = kept
	}

	if !changed {
		return false, nil
	}

	out, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		return false, err
	}
	return true, nil
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

// WriteClaudeHooksGlobal escribe los hooks del modo plan atómico (y el resto
// de hooks de gomemory) en <home>/.claude/settings.json — el ámbito de
// USUARIO, no de proyecto (feature 019, Historia 4). Antes de esta feature,
// el ámbito global registraba el servidor MCP y el texto de instrucciones,
// pero nunca los hooks (research.md §7): un proyecto nuevo sin instalación
// propia tenía el texto del protocolo pero no el determinismo de la
// Historia 1. Envoltorio delgado sobre writeClaudeHooks — misma idempotencia
// y preservación de entradas ajenas que el ámbito de proyecto, porque es
// literalmente la misma función.
func WriteClaudeHooksGlobal(home string, ref AgentRef) error {
	return writeClaudeHooks(home, ref)
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

	for event, regs := range claudeHookEvents {
		kept := filterOutGomemoryHooks(hooks[event])
		for _, r := range regs {
			command := ref.HookCommand + " hook " + r.sub
			kept = append(kept, map[string]interface{}{
				"matcher": r.matcher,
				"hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": command},
				},
			})
		}
		hooks[event] = kept
	}

	// Limpia entradas de gomemory en eventos que ya NO gestionamos (p. ej.
	// PreCompact, retirado a favor de SessionStart(compact)): al subir de versión
	// y re-correr setup no deben quedar hooks huérfanos que dupliquen trabajo.
	// Solo toca entradas de gomemory; preserva las de terceros y elimina la clave
	// del evento si queda vacía.
	for event, raw := range hooks {
		if _, managed := claudeHookEvents[event]; managed {
			continue
		}
		if kept := filterOutGomemoryHooks(raw); len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
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
		strings.Contains(cmd, "hook post-compact") ||
		strings.Contains(cmd, "hook user-prompt-submit") ||
		strings.Contains(cmd, "hook turn-end") ||
		strings.Contains(cmd, "hook subagent-start") ||
		strings.Contains(cmd, "hook subagent-stop") ||
		strings.Contains(cmd, "hook plan-approved") ||
		strings.Contains(cmd, "hook plan-guard") ||
		strings.Contains(cmd, "hook plan-entered") ||
		strings.Contains(cmd, filepath.Join("plugins", "gomemory")) ||
		strings.Contains(cmd, "plugins/gomemory")
}
