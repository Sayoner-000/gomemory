package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mem/domain"
)

// ActivationInspector implementa ports.ActivationInspector inspeccionando
// las rutas reales de cada agente declarado en domain.KnownAgents (feature
// 019, Historia 3). Solo lectura: nunca escribe ni corrige nada, ni siquiera
// para el brazo propio — el reporte es un diagnóstico, no un instalador.
type ActivationInspector struct{}

func NewActivationInspector() *ActivationInspector { return &ActivationInspector{} }

// agentInstructionFiles son los archivos de instrucciones que se inspeccionan
// en un directorio dado, en el mismo orden que cmd_install.go.
//
// La lista es COMÚN a todos los agentes, no de Claude Code: `AGENTS.md` es el de
// OpenCode (y el estándar que también leen Codex y Gemini), `CLAUDE.md` el de
// Claude Code, y los dos `.*rules` son legados que solo se reportan si
// conservan un bloque de una versión anterior. El nombre anterior sugería lo
// contrario y hacía leer como sesgo lo que es una tabla compartida.
var agentInstructionFiles = []string{"AGENTS.md", "CLAUDE.md", "CLAUDE.txt", ".cursorrules", ".windsurfrules"}

// Inspect recorre domain.KnownAgents y devuelve un canal por combinación
// agente/ámbito/tipo aplicable, más los canales de solo lectura del brazo
// extensor de grafo de código si está instalado (data-model.md §3).
func (a *ActivationInspector) Inspect(root string) []domain.ActivationChannel {
	var channels []domain.ActivationChannel

	home, _ := os.UserHomeDir()

	for _, agent := range domain.KnownAgents {
		if agent.Scopes[domain.ScopeProject] {
			channels = append(channels, a.inspectAgentScope(agent, root, domain.ScopeProject)...)
		}
		if agent.Scopes[domain.ScopeUser] && home != "" {
			channels = append(channels, a.inspectAgentScope(agent, home, domain.ScopeUser)...)
		}
	}

	channels = append(channels, a.inspectCodegraph(root, home)...)

	domain.SortChannels(channels)
	return channels
}

func (a *ActivationInspector) inspectAgentScope(agent domain.AgentCapability, dir string, scope domain.AgentScope) []domain.ActivationChannel {
	var out []domain.ActivationChannel

	// El archivo de instrucciones de ámbito USUARIO vive en el subdirectorio
	// propio de cada agente (home/.claude/CLAUDE.md para claude,
	// home/.config/opencode/AGENTS.md para opencode) — nunca directo bajo
	// home. Bug real encontrado en T055 (verificación en vivo contra $HOME):
	// el inspector miraba home/CLAUDE.md y reportaba "ok" por coincidencia
	// con un archivo señuelo, sin comprobar nunca el archivo real. En ámbito
	// de proyecto, en cambio, las instrucciones sí viven directo en la raíz
	// (dir == root), así que solo se resuelve el subdirectorio para usuario.
	instructionsDir := dir
	if scope == domain.ScopeUser {
		instructionsDir = userInstructionsDir(agent.Name, dir)
	}
	out = append(out, a.inspectInstructions(agent, instructionsDir, scope, scope == domain.ScopeProject))

	if agent.HasLevel(domain.AgentLevelGuard) {
		out = append(out, a.inspectClaudeHook(agent, dir, scope, domain.KindPlanGuard, "PreToolUse", "ExitPlanMode", "hook plan-guard"))
	} else {
		// Degradación DECLARADA (FR-017, T049): el agente no sostiene el
		// nivel 1 — se reporta como not_applicable con motivo, nunca se
		// omite en silencio. La razón vive en el registro, no aquí: este
		// inspector solo traduce lo que domain.KnownAgents ya declaró.
		reason := agent.GuardUnavailableReason
		if reason == "" {
			reason = "el agente no declara soporte para el borde de salida determinista"
		}
		out = append(out, domain.ActivationChannel{
			Arm: domain.ArmGomemory, Agent: agent.Name, Scope: scope, Kind: domain.KindPlanGuard,
			State: domain.StateNotApplicable, Detail: reason,
		})
	}
	if agent.HasLevel(domain.AgentLevelEntry) {
		switch {
		case agent.Dialect == domain.DialectClaude:
			out = append(out, a.inspectClaudeHook(agent, dir, scope, domain.KindPlanEntry, "PostToolUse", "EnterPlanMode", "hook plan-entered"))
		case scope == domain.ScopeUser:
			// inspectClaudeHook busca en .claude/settings.json — un mecanismo
			// exclusivo de Claude Code. Un agente con otro dialecto (opencode)
			// sostiene el nivel 2 por OTRO medio real (su plugin, siempre
			// global): comprobarlo aquí en vez de reportar "ok" por la mera
			// coincidencia de que el hook de claude viva en el mismo $HOME.
			// Bug real encontrado en T055 (verificación en vivo).
			out = append(out, a.inspectAgentEntryFile(agent, dir, scope))
		default:
			// El plugin de OpenCode no tiene copia por proyecto (siempre se
			// instala en $HOME, auto-descubierto) — no hay nada que
			// inspeccionar en el ámbito de proyecto para este agente.
			out = append(out, domain.ActivationChannel{
				Arm: domain.ArmGomemory, Agent: agent.Name, Scope: scope, Kind: domain.KindPlanEntry,
				State: domain.StateNotApplicable, Detail: "este agente instala su mecanismo de entrada de forma global, no por proyecto",
			})
		}
	} else {
		// Simétrico al tratamiento de plan_guard: un agente que no sostiene el
		// nivel 2 declara POR QUÉ. Antes no se emitía fila alguna, y una
		// ausencia sin motivo es indistinguible de un olvido — así fue como un
		// agente entero pudo faltar del reporte sin que nada lo señalara.
		reason := agent.EntryUnavailableReason
		if reason == "" {
			reason = "el agente no declara soporte para el borde de entrada al modo plan"
		}
		out = append(out, domain.ActivationChannel{
			Arm: domain.ArmGomemory, Agent: agent.Name, Scope: scope, Kind: domain.KindPlanEntry,
			State: domain.StateNotApplicable, Detail: reason,
		})
	}

	// turn_reminder: el recordatorio de una línea (feature 019, Historia 2)
	// lo emite el binario en cada turno mientras la planificación atómica no
	// esté apagada — no depende de una entrada de configuración propia, así
	// que se reporta ok siempre que el agente declare el ámbito, salvo que
	// el ajuste esté explícitamente apagado en ese ámbito.
	disabled := false
	if s := readSettingsAtomicPlanDisabled(dir); s {
		disabled = true
	}
	state := domain.StateOK
	detail := "recordatorio por turno activo"
	if disabled {
		state = domain.StateMissing
		detail = "atomic_plan_disabled=true en este ámbito"
	}
	out = append(out, domain.ActivationChannel{
		Arm: domain.ArmGomemory, Agent: agent.Name, Scope: scope,
		Kind: domain.KindTurnReminder, State: state, Detail: detail,
	})

	if agent.Name == "opencode" && scope == domain.ScopeUser {
		out = append(out, inspectOpenCodeServerConfig())
	}
	if agent.Name == "codex" && scope == domain.ScopeUser {
		out = append(out, inspectCodexServerConfig(), inspectCodexHooks())
	}

	return out
}

// inspectCodexServerConfig verifica que ~/.codex/config.toml registre el
// servidor gomemory. Es el canal por el que Codex obtiene memoria: sin él, y
// sin hooks, el agente corre sin memoria y sin ningún síntoma visible.
func inspectCodexServerConfig() domain.ActivationChannel {
	ch := domain.ActivationChannel{
		Arm: domain.ArmGomemory, Agent: "codex", Scope: domain.ScopeUser,
		Kind: domain.KindServerConfig,
	}
	if CodexGlobalRegistrationExists() {
		ch.State = domain.StateOK
		ch.Detail = "registro MCP global en ~/.codex/config.toml"
		return ch
	}
	ch.State = domain.StateMissing
	ch.Detail = "sin registro MCP global para gomemory"
	return ch
}

// inspectCodexHooks comprueba que el ciclo por turno de Codex esté enganchado.
// Es la diferencia entre que el agente PUEDA consultar memoria (registro MCP) y
// que gomemory EJERZA su ciclo: inyectar contexto al arrancar y registrar la
// actividad al cerrar cada turno, sin gastar tokens del agente.
func inspectCodexHooks() domain.ActivationChannel {
	ch := domain.ActivationChannel{
		Arm: domain.ArmGomemory, Agent: "codex", Scope: domain.ScopeUser,
		Kind: domain.KindLifecycleHook,
	}
	faltantes := CodexMissingGomemoryHooks()
	if len(faltantes) == 0 {
		ch.State = domain.StateOK
		ch.Detail = "hooks de gomemory en ~/.codex/config.toml (session-start, post-compact, turn-end)"
		return ch
	}
	ch.State = domain.StateMissing
	ch.Detail = "faltan hooks de gomemory en ~/.codex/config.toml: " + strings.Join(faltantes, ", ")
	return ch
}

// inspectOpenCodeServerConfig verifica que la configuración global del agente
// (~/.config/opencode/opencode.json) registre el servidor gomemory.
//
// Desde que install dejó de escribir el registro por proyecto —solo duplicaba
// lo que el merge de OpenCode ya resuelve— este canal es LA evidencia de que
// el agente tiene memoria disponible. Sin él y sin plugin, OpenCode perdería
// la memoria sin ningún síntoma.
func inspectOpenCodeServerConfig() domain.ActivationChannel {
	ch := domain.ActivationChannel{
		Arm: domain.ArmGomemory, Agent: "opencode", Scope: domain.ScopeUser,
		Kind: domain.KindServerConfig,
	}
	if OpenCodeGlobalRegistrationExists() {
		ch.State = domain.StateOK
		ch.Detail = "registro MCP global en ~/.config/opencode/opencode.json"
		return ch
	}
	ch.State = domain.StateMissing
	ch.Detail = "sin registro MCP global para gomemory"
	return ch
}

// userInstructionsDir resuelve el subdirectorio real donde vive el archivo
// de instrucciones de ÁMBITO USUARIO de agentName, dentro de home. Reutiliza
// globalTargets (atomic_plan_global.go) — la MISMA tabla que
// InstallAtomicPlanGlobal usa para escribir de verdad — en vez de asumir que
// el archivo vive directo bajo home. Transparente al canal de instalación
// (curl/irm solo colocan el binario; quien realmente escribe estos archivos
// es `mem setup-mcp --scope global`, que ya usa esta misma tabla) y al
// sistema operativo (filepath.Join ya resuelve el separador correcto).
// Un agente ausente de globalTargets (sin ámbito de usuario conocido) cae de
// vuelta a home directo, sin fallar.
func userInstructionsDir(agentName, home string) string {
	for _, t := range globalTargets {
		if t.agent == agentName {
			return filepath.Join(append([]string{home}, t.dir...)...)
		}
	}
	return home
}

// agentEntryFiles mapea, por agente, la ruta (relativa a home) del archivo
// cuya sola presencia sostiene su nivel 2 (AgentLevelEntry) cuando ese agente
// NO usa el sistema de hooks de Claude Code. Hoy solo aplica a opencode: su
// plugin (infrastructure/plugin/opencode/gomemory.ts) se instala SIEMPRE en
// esta ruta fija, independientemente del ámbito de proyecto (installOpenCodePlugin,
// opencode_setup.go) — inyecta el recordatorio de modo plan en cada turno vía
// chat.system.transform mientras el archivo exista, sin ningún hook que
// registrar.
var agentEntryFiles = map[string][]string{
	"opencode": {".config", "opencode", "plugins", "gomemory.ts"},
}

// inspectAgentEntryFile comprueba la presencia del archivo real que sostiene
// el nivel 2 de agent cuando su mecanismo no es un hook de Claude Code (ver
// agentEntryFiles). home es el directorio de usuario resuelto por Inspect.
func (a *ActivationInspector) inspectAgentEntryFile(agent domain.AgentCapability, home string, scope domain.AgentScope) domain.ActivationChannel {
	ch := domain.ActivationChannel{Arm: domain.ArmGomemory, Agent: agent.Name, Scope: scope, Kind: domain.KindPlanEntry}

	rel, known := agentEntryFiles[agent.Name]
	if !known {
		ch.State = domain.StateMissing
		ch.Detail = "no se conoce el mecanismo de entrada de este agente"
		return ch
	}
	path := filepath.Join(append([]string{home}, rel...)...)
	if _, err := os.Stat(path); err != nil {
		ch.State = domain.StateMissing
		ch.Detail = "no encontrado: " + path
		return ch
	}
	ch.State = domain.StateOK
	ch.Detail = "plugin instalado en " + path
	return ch
}

func readSettingsAtomicPlanDisabled(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".memory", "settings.json"))
	if err != nil {
		return false
	}
	var parsed struct {
		AtomicPlanDisabled bool `json:"atomic_plan_disabled"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return false
	}
	return parsed.AtomicPlanDisabled
}

// inspectInstructions busca, entre los archivos de instrucciones reconocidos en
// dir, el bloque de protocolo y reporta su versión.
//
// opcional cambia SOLO el veredicto de la ausencia (feature 021, FR-028): desde
// que `mem install` dejó de escribir el bloque en archivos del proyecto, no
// encontrarlo en ámbito de PROYECTO ya no es una falta —el protocolo llega en la
// respuesta initialize del MCP— sino un canal que no aplica.
//
// El matiz importa y es deliberado: un archivo legado que TODAVÍA conserva un
// bloque de una versión anterior se sigue reportando como desactualizado. Eso no
// es falsa alarma sino información verdadera sobre un duplicado obsoleto que
// conviene retirar; la limpieza del instalador se encarga de que deje de
// ocurrir.
func (a *ActivationInspector) inspectInstructions(agent domain.AgentCapability, dir string, scope domain.AgentScope, opcional bool) domain.ActivationChannel {
	ch := domain.ActivationChannel{Arm: domain.ArmGomemory, Agent: agent.Name, Scope: scope, Kind: domain.KindInstructions}

	for _, fname := range agentInstructionFiles {
		data, err := os.ReadFile(filepath.Join(dir, fname))
		if err != nil {
			continue
		}
		content := string(data)
		loc := domain.ProtocolVersionPattern.FindString(content)
		if loc == "" {
			continue
		}
		if loc == domain.ProtocolVersionMarker {
			ch.State = domain.StateOK
			ch.Detail = fname + ": " + loc
		} else {
			ch.State = domain.StateOutdated
			ch.Detail = fname + ": " + loc + "; vigente " + domain.ProtocolVersionMarker
		}
		return ch
	}

	if opcional {
		ch.State = domain.StateNotApplicable
		ch.Detail = "el protocolo viaja en las Instructions del MCP; gomemory ya no escribe archivos de instrucciones en el proyecto"
		return ch
	}
	ch.State = domain.StateMissing
	ch.Detail = "ningún archivo de instrucciones con el bloque de protocolo en " + dir
	return ch
}

// inspectClaudeHook cuenta las entradas registradas para (event, matcher) en
// <dir>/.claude/settings.json que invocan cmdSuffix, y traduce la cuenta a
// missing/ok/duplicated.
func (a *ActivationInspector) inspectClaudeHook(agent domain.AgentCapability, dir string, scope domain.AgentScope, kind domain.ChannelKind, event, matcher, cmdSuffix string) domain.ActivationChannel {
	ch := domain.ActivationChannel{Arm: domain.ArmGomemory, Agent: agent.Name, Scope: scope, Kind: kind}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		ch.State = domain.StateMissing
		ch.Detail = "sin .claude/settings.json en " + dir
		return ch
	}

	var settings struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &settings) != nil {
		ch.State = domain.StateMissing
		ch.Detail = "settings.json ilegible"
		return ch
	}

	count := 0
	for _, entry := range settings.Hooks[event] {
		if entry.Matcher != matcher {
			continue
		}
		for _, h := range entry.Hooks {
			if hasCommandSuffix(h.Command, cmdSuffix) {
				count++
			}
		}
	}

	switch {
	case count == 0:
		ch.State = domain.StateMissing
		ch.Detail = event + ":" + matcher + " no registrado"
	case count == 1:
		ch.State = domain.StateOK
		ch.Detail = event + ":" + matcher + " → " + cmdSuffix
	default:
		ch.State = domain.StateDuplicated
		ch.Detail = event + ":" + matcher + " registrado " + itoa(count) + " veces"
	}
	return ch
}

func hasCommandSuffix(cmd, suffix string) bool {
	return len(cmd) >= len(suffix) && cmd[len(cmd)-len(suffix):] == suffix
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// codegraphHookPattern reconoce los scripts del brazo extensor instalados
// como hooks de Claude Code (ver ~/.claude/hooks/cbm-*, memoria 346).
var codegraphHookPattern = regexp.MustCompile(`cbm-[a-z-]+`)

// inspectCodegraph observa —nunca escribe— la presencia del brazo extensor
// de grafo de código (INV-1, INV-4): si no está instalado, no se reporta
// ningún canal codegraph y no se emite ningún aviso por su ausencia.
func (a *ActivationInspector) inspectCodegraph(root, home string) []domain.ActivationChannel {
	if home == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return nil
	}
	if !codegraphHookPattern.Match(data) {
		return nil
	}
	return []domain.ActivationChannel{{
		Arm: domain.ArmCodegraph, Agent: "claude", Scope: domain.ScopeUser,
		Kind: domain.KindInstructions, State: domain.StateOK,
		Detail: "hooks cbm-* presentes en ~/.claude/settings.json (solo lectura)",
	}}
}
