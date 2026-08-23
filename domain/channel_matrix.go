package domain

import (
	"fmt"
	"path/filepath"
)

// KindServerConfig y KindPermissions amplían el vocabulario de canales para
// cubrir lo que la instalación escribe y el diagnóstico todavía no observa: el
// registro del servidor en la configuración del agente, y los permisos que
// pre-aprueban sus operaciones.
//
// Se añaden aquí y no en activation.go porque nacen con la matriz: son canales
// que existen desde siempre en el instalador, pero que ninguna declaración
// nombraba. Esa falta de nombre es lo que permitió que una entrada de
// configuración sobreviviera a toda desinstalación.
const (
	KindServerConfig ChannelKind = "server_config"
	KindPermissions  ChannelKind = "permissions"
)

// ActivityName identifica una actividad del ciclo de vida que opera sobre la
// matriz.
type ActivityName string

const (
	ActivityInstall       ActivityName = "install"
	ActivityInstallGlobal ActivityName = "install_global"
	ActivityUninstall     ActivityName = "uninstall"
	ActivityCleanup       ActivityName = "cleanup"
	ActivityInspect       ActivityName = "inspect"
)

// LifecycleActivity declara el alcance de una actividad. El alcance no es
// documentación: es lo que hace verificable que una operación dirigida a un
// proyecto no alcance artefactos que comparten todos los proyectos de la
// máquina.
type LifecycleActivity struct {
	Name  ActivityName
	Scope AgentScope
	// ReadOnly marca la actividad que observa sin escribir. Recorre ambos
	// ámbitos y queda fuera de la contención: lo que se restringe es escribir
	// o retirar fuera del alcance, nunca leer.
	ReadOnly bool
}

// LifecycleActivities es la declaración de alcance de cada actividad.
var LifecycleActivities = []LifecycleActivity{
	{Name: ActivityInstall, Scope: ScopeProject},
	{Name: ActivityInstallGlobal, Scope: ScopeUser},
	{Name: ActivityUninstall, Scope: ScopeProject},
	{Name: ActivityCleanup, Scope: ScopeProject},
	{Name: ActivityInspect, ReadOnly: true},
}

// MatrixCell une un agente, un tipo de canal y un ámbito con el artefacto que
// le corresponde (data-model.md).
//
// INV-1: declara Path o NotApplicableReason, nunca ambos ni ninguno. Una celda
// sin ninguno de los dos es indistinguible de un olvido, y ese fue el estado
// que produjo los cuatro defectos que originaron esta feature.
//
// INV-2: Path es siempre relativa. Resolverla contra un directorio concreto es
// trabajo del adaptador; el dominio no conoce el sistema de archivos.
type MatrixCell struct {
	Agent string
	Kind  ChannelKind
	Scope AgentScope

	// Path es la ruta del artefacto relativa a la raíz del ámbito: el
	// directorio del proyecto, o el del usuario.
	Path []string

	// ConfigKey es la clave bajo la que ese agente registra sus servidores.
	// Vive en la celda y no en una constante compartida porque los agentes NO
	// comparten esquema, y asumir que sí lo hacían dejaba huérfana la entrada
	// de un agente en cada desinstalación.
	ConfigKey string

	// Managed distingue lo que gomemory escribe de lo que solo observa.
	Managed bool

	// Legacy marca lo que generaban versiones anteriores. INV-5: nunca se
	// escribe, solo se retira.
	Legacy bool

	// NotApplicableReason explica por qué esta celda no aplica a este agente.
	// Un motivo declarado es información válida; una ausencia sin motivo no.
	NotApplicableReason string
}

// Key identifica la celda de forma estable, para comparar conjuntos entre
// actividades.
func (c MatrixCell) Key() string {
	return fmt.Sprintf("%s|%s|%s|%s", c.Agent, c.Scope, c.Kind, filepath.Join(c.Path...))
}

// String da una descripción legible para los mensajes de fallo de la
// verificación: quien lea el error debe poder ubicar la celda sin leer código.
func (c MatrixCell) String() string {
	destino := filepath.Join(c.Path...)
	if destino == "" {
		destino = "(sin artefacto)"
	}
	return fmt.Sprintf("celda [%s · %s · %s → %s]", c.Agent, c.Scope, c.Kind, destino)
}

// Applies reporta si la celda materializa un artefacto.
func (c MatrixCell) Applies() bool { return len(c.Path) > 0 }

// ChannelMatrix es la declaración única de qué artefacto corresponde a cada
// combinación de canal, agente y ámbito.
//
// Añadir un agente es añadir sus filas aquí. Si alguna actividad del ciclo de
// vida queda sin cubrirlo, la verificación de tests/contract falla nombrando la
// celda: es la garantía de que un agente nuevo no puede quedarse a medias en
// silencio.
var ChannelMatrix = []MatrixCell{
	// ── claude · proyecto ────────────────────────────────────────────────
	{Agent: "claude", Kind: KindPlanEntry, Scope: ScopeProject,
		Path: []string{".claude", "settings.json"}, Managed: true},
	{Agent: "claude", Kind: KindPlanGuard, Scope: ScopeProject,
		Path: []string{".claude", "settings.json"}, Managed: true},
	{Agent: "claude", Kind: KindTurnReminder, Scope: ScopeProject,
		Path: []string{".claude", "settings.json"}, Managed: true},
	{Agent: "claude", Kind: KindPermissions, Scope: ScopeProject,
		Path: []string{".claude", "settings.json"}, Managed: true},
	{Agent: "claude", Kind: KindServerConfig, Scope: ScopeProject,
		Path: []string{".mcp.json"}, ConfigKey: "mcpServers", Managed: true},
	{Agent: "claude", Kind: KindNativeWrapper, Scope: ScopeProject,
		Path: []string{".claude", "skills"}, Managed: true},
	{Agent: "claude", Kind: KindInstructions, Scope: ScopeProject,
		NotApplicableReason: "el protocolo viaja en las Instructions del MCP; gomemory ya no escribe archivos de instrucciones en el proyecto"},

	// ── claude · usuario ─────────────────────────────────────────────────
	{Agent: "claude", Kind: KindPlanEntry, Scope: ScopeUser,
		Path: []string{".claude", "settings.json"}, Managed: true},
	{Agent: "claude", Kind: KindPlanGuard, Scope: ScopeUser,
		Path: []string{".claude", "settings.json"}, Managed: true},
	{Agent: "claude", Kind: KindTurnReminder, Scope: ScopeUser,
		Path: []string{".claude", "settings.json"}, Managed: true},
	{Agent: "claude", Kind: KindInstructions, Scope: ScopeUser,
		Path: []string{".claude", "CLAUDE.md"}, Managed: true},
	{Agent: "claude", Kind: KindNativeWrapper, Scope: ScopeUser,
		Path: []string{".claude", "skills"}, Managed: true},

	// ── opencode · proyecto ──────────────────────────────────────────────
	{Agent: "opencode", Kind: KindServerConfig, Scope: ScopeProject,
		Path: []string{"opencode.json"}, ConfigKey: "mcp", Managed: true},
	{Agent: "opencode", Kind: KindPermissions, Scope: ScopeProject,
		Path: []string{"opencode.json"}, Managed: true},
	{Agent: "opencode", Kind: KindNativeWrapper, Scope: ScopeProject,
		Path: []string{".opencode", "commands"}, Managed: true},
	{Agent: "opencode", Kind: KindTurnReminder, Scope: ScopeProject,
		Path: []string{"opencode.json"}, Managed: true},
	{Agent: "opencode", Kind: KindInstructions, Scope: ScopeProject,
		NotApplicableReason: "el protocolo viaja en las Instructions del MCP; gomemory ya no escribe archivos de instrucciones en el proyecto"},
	{Agent: "opencode", Kind: KindPlanEntry, Scope: ScopeProject,
		NotApplicableReason: "este agente instala su mecanismo de entrada de forma global, no por proyecto"},
	{Agent: "opencode", Kind: KindPlanGuard, Scope: ScopeProject,
		NotApplicableReason: "el ciclo del agente no ofrece un punto de decisión antes de presentar el plan"},

	// ── opencode · usuario ───────────────────────────────────────────────
	{Agent: "opencode", Kind: KindPlanEntry, Scope: ScopeUser,
		Path: []string{".config", "opencode", "plugins", "gomemory.ts"}, Managed: true},
	{Agent: "opencode", Kind: KindTurnReminder, Scope: ScopeUser,
		Path: []string{".config", "opencode", "plugins", "gomemory.ts"}, Managed: true},
	{Agent: "opencode", Kind: KindInstructions, Scope: ScopeUser,
		Path: []string{".config", "opencode", "AGENTS.md"}, Managed: true},
	{Agent: "opencode", Kind: KindNativeWrapper, Scope: ScopeUser,
		Path: []string{".config", "opencode", "commands"}, Managed: true},
	{Agent: "opencode", Kind: KindServerConfig, Scope: ScopeUser,
		Path: []string{".config", "opencode", "opencode.json"}, ConfigKey: "mcp", Managed: true},
	{Agent: "opencode", Kind: KindPlanGuard, Scope: ScopeUser,
		NotApplicableReason: "el ciclo del agente no ofrece un punto de decisión antes de presentar el plan"},

	// ── artefactos heredados · proyecto ──────────────────────────────────
	// INV-5: se retiran, nunca se escriben. Los dejó `mem install` antes de
	// la versión 2.9.0.
	{Agent: "claude", Kind: KindInstructions, Scope: ScopeProject,
		Path: []string{"CLAUDE.md"}, Legacy: true},
	{Agent: "claude", Kind: KindInstructions, Scope: ScopeProject,
		Path: []string{"CLAUDE.txt"}, Legacy: true},
	{Agent: "opencode", Kind: KindInstructions, Scope: ScopeProject,
		Path: []string{"AGENTS.md"}, Legacy: true},
	{Agent: "opencode", Kind: KindServerConfig, Scope: ScopeProject,
		Path: []string{".opencode.json"}, ConfigKey: "mcpServers", Legacy: true},
}

// CellsFor devuelve las celdas de un agente para un canal y ámbito dados.
func CellsFor(agent string, kind ChannelKind, scope AgentScope) []MatrixCell {
	var out []MatrixCell
	for _, c := range ChannelMatrix {
		if c.Agent == agent && c.Kind == kind && c.Scope == scope {
			out = append(out, c)
		}
	}
	return out
}

// CellsForActivity devuelve las celdas sobre las que opera una actividad,
// aplicando su alcance declarado.
//
// El filtro por alcance vive aquí y no en cada llamador: es lo que garantiza
// que una actividad dirigida a un proyecto no pueda alcanzar un artefacto que
// comparten todos los proyectos de la máquina, sin depender de que quien la
// escriba se acuerde.
func CellsForActivity(name ActivityName) []MatrixCell {
	var act LifecycleActivity
	for _, a := range LifecycleActivities {
		if a.Name == name {
			act = a
			break
		}
	}
	if act.Name == "" {
		return nil
	}

	var out []MatrixCell
	for _, c := range ChannelMatrix {
		if !c.Applies() {
			continue // una celda sin artefacto no se instala ni se retira
		}
		if !act.ReadOnly && c.Scope != act.Scope {
			continue
		}
		switch name {
		case ActivityInstall, ActivityInstallGlobal:
			if !c.Managed || c.Legacy {
				continue
			}
		case ActivityUninstall:
			if !c.Managed && !c.Legacy {
				continue
			}
		case ActivityCleanup:
			if !c.Legacy {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// ServerConfigCells devuelve las celdas de registro de servidor de un ámbito,
// con la clave de esquema que cada agente usa.
func ServerConfigCells(scope AgentScope) []MatrixCell {
	var out []MatrixCell
	for _, c := range ChannelMatrix {
		if c.Kind == KindServerConfig && c.Scope == scope && c.Applies() {
			out = append(out, c)
		}
	}
	return out
}
