package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"mem/adapters/secondary/persistence"
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

// claudeToolOutputMatcher: herramientas cuya salida comprime el hook opt-in de
// la feature 033. Read/Edit/Write quedan fuera a propósito (Edit necesita el
// texto exacto de Read), y las de gomemory las excluye el propio subcomando.
const claudeToolOutputMatcher = "Bash|Grep|Glob|WebFetch|WebSearch|mcp__.*"

// claudeHookEventsFor añade a los eventos fijos el hook de salidas de
// herramientas SOLO con tool_output_compression activo (opt-in, research R11).
// Con el ajuste apagado devuelve exactamente claudeHookEvents: huella cero.
func claudeHookEventsFor(toolOutput bool) map[string][]hookReg {
	if !toolOutput {
		return claudeHookEvents
	}
	out := make(map[string][]hookReg, len(claudeHookEvents))
	for k, v := range claudeHookEvents {
		out[k] = append([]hookReg(nil), v...)
	}
	out["PostToolUse"] = append(out["PostToolUse"], hookReg{matcher: claudeToolOutputMatcher, sub: "tool-output claude"})
	return out
}

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
		// PostCompact (feature 030, US2, capacidad C6): evento DISTINTO de
		// SessionStart(compact) — dispara con el resumen ya calculado por el
		// cliente (compact_summary) y lo persiste sin cerrar la sesión.
		// PreCompact sigue sin registrarse (research.md R12: C2+C6 ya cubren
		// la misma necesidad).
		"PostCompact":   {{matcher: "", sub: "compact-summary"}},
		"Stop":          {{matcher: "", sub: "turn-end"}},
		"SubagentStart": {{matcher: "", sub: "subagent-start"}},
		"SubagentStop":  {{matcher: "", sub: "subagent-stop"}},
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

	// mem expand (feature 040, research R3): activo desde la instalación, sin
	// opt-in. Un único subcomando por evento; el dialecto va explícito porque
	// los payloads de SessionStart, UserPromptSubmit y Stop no traen
	// tool_name y la detección los tomaría por neutrales. PreToolUse solo
	// intercepta las herramientas sobre las que actúan las reglas (shell,
	// escritura y delegación); Read/Grep/Glob no pagan el proceso.
	for event, reg := range claudeExpandHooks {
		events[event] = append(events[event], reg)
	}

	return events
}

// claudeExpandHooks son las entradas de `mem hook expand` en Claude Code.
var claudeExpandHooks = map[string]hookReg{
	"SessionStart":     {matcher: "startup|resume|clear|compact", sub: "expand session-start --emit=claude"},
	"UserPromptSubmit": {matcher: "", sub: "expand prompt --emit=claude"},
	"PreToolUse":       {matcher: "Bash|Write|Edit|MultiEdit|NotebookEdit|Task|Agent", sub: "expand pre-tool --emit=claude"},
	"PostToolUse":      {matcher: "Bash", sub: "expand post-tool --emit=claude"},
	"Stop":             {matcher: "", sub: "expand stop --emit=claude"},
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
	// La limpieza de artefactos legacy no es requisito para una instalación
	// fresca: no debe bloquearla si el filesystem impide borrar residuos.
	_ = os.RemoveAll(filepath.Join(pluginDir, "scripts"))
	_ = os.RemoveAll(filepath.Join(pluginDir, "hooks"))
	_ = os.Remove(filepath.Join(pluginDir, ".mcp.json"))

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

	// Los hooks globales sirven a todos los proyectos: el proyecto solo añade
	// los subcomandos que el global no cubre. Registrar los dos hacía que Claude
	// Code ejecutara cada evento dos veces (feature 035, FR-001).
	skip := globalClaudeHookSubs(root)
	if err := writeClaudeHooksSkipping(root, ref, skip); err != nil {
		return err
	}
	fmt.Printf("  ✅ claude-code: hooks portables configurados en %s\n", filepath.Join(root, ".claude", "settings.json"))
	if len(skip) > 0 {
		fmt.Println("  ℹ️  claude-code: hooks globales activos; el proyecto solo añade los que el global no cubre")
	}

	if err := writeClaudePermissions(root); err != nil {
		return err
	}
	fmt.Printf("  ✅ claude-code: tools MCP pre-aprobadas en %s\n", filepath.Join(root, ".claude", "settings.json"))
	return nil
}

// claudeAutoAllowToolsFor son las tools MCP de gomemory seguras para
// pre-aprobar automáticamente cuando Octopus AAR está en el estado dado: de
// solo lectura, o de escritura acotada y reversible. forget_memory queda
// deliberadamente afuera por ser destructiva/irreversible. Se deriva de
// domain para que no pueda desincronizarse del servidor. Las de grafo son de
// solo lectura salvo index_project, que solo escribe en .memory/ y nunca toca
// el código fuente.
func claudeAutoAllowToolsFor(octopusEnabled bool) []string {
	return domain.MCPPrefixed("mcp__gomemory__", domain.MCPAutoApprovableToolsFor(octopusEnabled))
}

// ClaudeAutoAllowTools es el superset (incluye las 4 tools de Octopus) usado
// solo para RemoveClaudePermissions: al desinstalar hay que limpiar cualquier
// entrada de gomemory, sin importar si Octopus estaba habilitado cuando se
// instaló. writeClaudePermissions NO usa esta variable — usa
// claudeAutoAllowToolsFor(octopusEnabled real del proyecto) para no
// pre-aprobar tools de un módulo apagado (contrato de huella cero,
// specs/029-octopus-aar/contracts/module-off.md; ACR 029, hallazgo A-001).
var ClaudeAutoAllowTools = claudeAutoAllowToolsFor(true)

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
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("decode claude settings: %w", err)
		}
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
	octopusEnabled := persistence.ReadSettings(root).OctopusEnabled
	for _, tool := range claudeAutoAllowToolsFor(octopusEnabled) {
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
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("decode existing claude mcp config: %w", err)
		}
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
	return writeClaudeHooksSkipping(root, ref, nil)
}

// writeClaudeHooksSkipping es writeClaudeHooks sin registrar los subcomandos de
// skip (los que ya cubre el ámbito global, feature 035).
func writeClaudeHooksSkipping(root string, ref AgentRef, skip map[string]bool) error {
	settingsDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	settingsPath := filepath.Join(settingsDir, "settings.json")

	settings := map[string]interface{}{}
	if data, _ := os.ReadFile(settingsPath); len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("decode existing claude settings: %w", err)
		}
	}
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
	}

	for event, regs := range claudeHookEventsFor(persistence.ReadSettings(root).ToolOutputCompression) {
		kept := filterOutGomemoryHooks(hooks[event])
		for _, r := range regs {
			if skip[hookSubKey(r.sub)] {
				continue
			}
			command := ref.HookCommand + " hook " + r.sub
			kept = append(kept, map[string]interface{}{
				"matcher": r.matcher,
				"hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": command},
				},
			})
		}
		if len(kept) == 0 {
			delete(hooks, event)
			continue
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

// hookCommandIsGomemory reconoce un comando de hook de gomemory por su
// subcomando, consultando el registro único domain.GomemoryHookSubcommands
// (feature 040, research R2), o por las rutas legadas del plugin.
func hookCommandIsGomemory(cmd string) bool {
	for _, m := range gomemoryHookSubRe.FindAllStringSubmatch(cmd, -1) {
		if domain.IsGomemoryHookSubcommand(m[2]) {
			return true
		}
	}
	return strings.Contains(cmd, filepath.Join("plugins", "gomemory")) ||
		strings.Contains(cmd, "plugins/gomemory")
}

// ClaudeHookSubs devuelve los subcomandos de `mem hook` que instala Claude
// Code (sin argumentos), incluido el opt-in de tool-output. Lo consume la
// prueba de contrato del registro único (feature 040, research R2).
func ClaudeHookSubs() []string {
	var subs []string
	for _, regs := range claudeHookEventsFor(true) {
		for _, r := range regs {
			subs = append(subs, hookSubName(r.sub))
		}
	}
	return subs
}

// hookSubName devuelve el subcomando de `mem hook` de un registro
// ("tool-output claude" → "tool-output").
func hookSubName(sub string) string {
	if i := strings.IndexByte(sub, ' '); i >= 0 {
		return sub[:i]
	}
	return sub
}

// hookSubKeyRe separa las dos primeras palabras de un subcomando; los flags
// (`--emit=claude`) no cuentan como palabra.
var hookSubKeyRe = regexp.MustCompile(`^([a-z-]+)(?:[ \t]+([a-z][a-z-]*))?`)

// hookSubKey identifica un subcomando para deduplicar entre ámbitos. Es
// hookSubName salvo en `expand`, que agrupa un subcomando por evento: sin la
// segunda palabra, un global con solo `expand stop` haría que el proyecto
// omitiera también `expand pre-tool` ("expand pre-tool --emit=claude" →
// "expand pre-tool", "tool-output claude" → "tool-output").
func hookSubKey(sub string) string {
	m := hookSubKeyRe.FindStringSubmatch(sub)
	if m == nil {
		return hookSubName(sub)
	}
	if m[1] == "expand" && m[2] != "" {
		return m[1] + " " + m[2]
	}
	return m[1]
}

// gomemoryHookKey devuelve la clave (hookSubKey) del primer `mem hook <sub>`
// de cmd, o false si cmd no invoca a gomemory.
func gomemoryHookKey(cmd string) (string, bool) {
	m := gomemoryHookSubRe.FindStringSubmatchIndex(cmd)
	if m == nil {
		return "", false
	}
	return hookSubKey(cmd[m[4]:]), true
}

// gomemoryHookSubRe exige que `hook <sub>` lo invoque el binario de
// gomemory: el token anterior es `mem` o `mem.exe`, por nombre, con ruta
// (`/usr/local/bin/mem`, `${CLAUDE_PROJECT_DIR}/mem`, `C:\…\mem.exe`) o
// entre comillas. Un hook de terceros que pase `hook prompt` a su propio
// script no es de gomemory (R2-04).
var gomemoryHookSubRe = regexp.MustCompile(`(?:^|[\s"'/\\])mem(?:\.exe)?["']?[ \t]+(hook)[ \t]+([a-z-]+)`)

// GomemoryHookIndex devuelve la posición de `hook` en la invocación
// `mem hook <sub>` de cmd, o -1 si cmd no invoca ese subcomando de gomemory.
// sub puede tener varias palabras ("expand pre-tool").
func GomemoryHookIndex(cmd, sub string) int {
	for _, m := range gomemoryHookSubRe.FindAllStringSubmatchIndex(cmd, -1) {
		rest, ok := strings.CutPrefix(cmd[m[4]:], sub)
		if ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
			return m[2]
		}
	}
	return -1
}

// GomemoryHookSubs devuelve los subcomandos de gomemory registrados en un
// settings.json de Claude Code (vacío si no existe o no se puede leer).
func GomemoryHookSubs(settingsPath string) map[string]bool {
	return gomemoryHookSet(settingsPath, func(cmd string) (string, bool) {
		if m := gomemoryHookSubRe.FindStringSubmatch(cmd); m != nil {
			return m[2], true
		}
		return "", false
	})
}

// GomemoryHookKeys es GomemoryHookSubs con la clave de deduplicación
// (hookSubKey): distingue cada subcomando de `expand`.
func GomemoryHookKeys(settingsPath string) map[string]bool {
	return gomemoryHookSet(settingsPath, gomemoryHookKey)
}

func gomemoryHookSet(settingsPath string, keyOf func(cmd string) (string, bool)) map[string]bool {
	subs := map[string]bool{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return subs
	}
	var settings map[string]interface{}
	if json.Unmarshal(data, &settings) != nil {
		return subs
	}
	hooks, _ := settings["hooks"].(map[string]interface{})
	for _, raw := range hooks {
		entries, _ := raw.([]interface{})
		for _, e := range entries {
			if !IsGomemoryHookEntry(e) {
				continue
			}
			for _, cmd := range hookEntryCommands(e) {
				if key, ok := keyOf(cmd); ok {
					subs[key] = true
				}
			}
		}
	}
	return subs
}

func hookEntryCommands(e interface{}) []string {
	switch v := e.(type) {
	case string:
		return []string{v}
	case map[string]interface{}:
		var out []string
		inner, _ := v["hooks"].([]interface{})
		for _, h := range inner {
			hm, _ := h.(map[string]interface{})
			if cmd, _ := hm["command"].(string); cmd != "" {
				out = append(out, cmd)
			}
		}
		return out
	}
	return nil
}

// globalClaudeHookSubs devuelve los subcomandos que ya registra el
// settings.json del usuario, salvo que root sea el propio HOME (entonces no
// hay un ámbito superior con el que duplicar).
func globalClaudeHookSubs(root string) map[string]bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || sameDir(home, root) {
		return nil
	}
	return GomemoryHookKeys(filepath.Join(home, ".claude", "settings.json"))
}

func sameDir(a, b string) bool {
	ca, errA := filepath.Abs(a)
	cb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(ca) == filepath.Clean(cb)
}

// DuplicateClaudeHookSubs devuelve, ordenados, los subcomandos de gomemory
// registrados a la vez en el settings.json global y en el del proyecto: cada
// uno se ejecuta dos veces por evento. Solo lee.
func DuplicateClaudeHookSubs(home, root string) []string {
	if home == "" || sameDir(home, root) {
		return nil
	}
	global := GomemoryHookKeys(filepath.Join(home, ".claude", "settings.json"))
	var dups []string
	for sub := range GomemoryHookKeys(filepath.Join(root, ".claude", "settings.json")) {
		if global[sub] {
			dups = append(dups, sub)
		}
	}
	sort.Strings(dups)
	return dups
}

// DedupClaudeProjectHooks retira del settings.json del proyecto las entradas
// de gomemory cuyo subcomando ya registra el global. Nunca toca el global ni
// las entradas ajenas. Idempotente: sin duplicados no escribe nada.
func DedupClaudeProjectHooks(home, root string) ([]string, error) {
	dups := DuplicateClaudeHookSubs(home, root)
	if len(dups) == 0 {
		return nil, nil
	}
	drop := map[string]bool{}
	for _, d := range dups {
		drop[d] = true
	}
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("decode claude settings: %w", err)
	}
	hooks, _ := settings["hooks"].(map[string]interface{})
	for event, raw := range hooks {
		entries, _ := raw.([]interface{})
		kept := make([]interface{}, 0, len(entries))
		for _, e := range entries {
			if e = withoutDuplicateCommands(e, drop); e != nil {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		return nil, fmt.Errorf("write .claude/settings.json: %w", err)
	}
	return dups, nil
}

// withoutDuplicateCommands quita de una entrada solo los comandos de gomemory
// duplicados; una entrada que agrupa varios conserva los demás (S-001 de
// acr_715249c3). Devuelve nil si la entrada queda vacía.
func withoutDuplicateCommands(e interface{}, drop map[string]bool) interface{} {
	isDup := func(cmd string) bool {
		key, ok := gomemoryHookKey(cmd)
		return ok && drop[key] && hookCommandIsGomemory(cmd)
	}
	switch v := e.(type) {
	case string:
		if isDup(v) {
			return nil
		}
		return v
	case map[string]interface{}:
		inner, _ := v["hooks"].([]interface{})
		keptInner := make([]interface{}, 0, len(inner))
		for _, h := range inner {
			hm, _ := h.(map[string]interface{})
			if cmd, _ := hm["command"].(string); isDup(cmd) {
				continue
			}
			keptInner = append(keptInner, h)
		}
		if len(keptInner) == 0 {
			return nil
		}
		out := make(map[string]interface{}, len(v))
		for k, val := range v {
			out[k] = val
		}
		out["hooks"] = keptInner
		return out
	}
	return e
}
