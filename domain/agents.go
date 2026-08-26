package domain

// AgentLevel es una capacidad que un agente puede sostener respecto al modo
// plan atómico (feature 019, contracts/agent-integration.md). Un agente pide
// lo que puede sostener: ninguno es obligatorio salvo AgentLevelTextFloor.
type AgentLevel string

const (
	// AgentLevelGuard (nivel 1): el agente puede invocar un comando ANTES de
	// presentar el plan y respetar la decisión que recibe. Da la garantía de
	// forma determinista (Historia 1).
	AgentLevelGuard AgentLevel = "guard"
	// AgentLevelEntry (nivel 2): el agente puede inyectar texto en el contexto
	// del modelo al entrar en modo plan. Da el método y el historial antes de
	// redactar (Historia 2).
	AgentLevelEntry AgentLevel = "entry"
	// AgentLevelTextFloor (nivel 3): el agente puede leer un archivo de
	// instrucciones. Obligatorio en toda entrada del registro (FR-A5).
	AgentLevelTextFloor AgentLevel = "text_floor"
)

// AgentDialect es la traducción de salida del contrato neutral
// (contracts/agent-integration.md) para un agente concreto. Describe una
// TRADUCCIÓN, nunca la definición de la capacidad (INV-6): ante cualquier
// duda sobre qué dialecto usar, el que manda es DialectNeutral.
type AgentDialect string

const (
	DialectNeutral AgentDialect = "neutral"
	DialectJSON    AgentDialect = "json"
	DialectClaude  AgentDialect = "claude"
	DialectText    AgentDialect = "text"
)

// AgentScope es un ámbito de configuración donde un agente puede recibir la
// capacidad: por proyecto o de una vez por máquina (usuario).
type AgentScope string

const (
	ScopeProject AgentScope = "project"
	ScopeUser    AgentScope = "user"
)

// AgentCapability es la fila del registro ÚNICO de capacidades por agente
// (data-model.md §5). Añadir un agente es añadir una entrada aquí; el reporte
// de cobertura (`mem doctor`) y la verificación de regresión se alimentan de
// esta tabla en vez de listas propias, así que un agente nuevo aparece en
// ambos sin tocarlos (FR-A4, SC-A2).
//
// Un agente AUSENTE de este registro no queda sin soporte: si invoca el
// contrato neutral, obtiene la garantía igual (FR-A3, INV-6). El registro
// sirve para instalar y reportar, no para autorizar.
//
// Alcance declarado (research.md §13.3): este registro nace como fuente
// única de lo que introduce la feature 019 — niveles, dialectos, reporte.
// Las tablas por agente que ya existen dispersas en el instalador
// (globalScopeAgents, atomicPlanWrappers, agentFiles) no se migran aquí
// todavía.
//
// Codex entró al registro después: estaba dentro del alcance del proyecto y
// recibía instalación real (registro MCP global, migración de hooks), pero al
// no figurar aquí no producía NI UNA fila en `mem doctor`, que cerraba con
// «Sin problemas» mientras el agente corría sobre un solo canal de cinco. Un
// agente que se instala pero no se observa es justo el punto ciego
// «presencia contra actividad» que el reporte existe para cerrar.
type AgentCapability struct {
	Name    string
	Dialect AgentDialect
	Levels  map[AgentLevel]bool
	Scopes  map[AgentScope]bool
	// GuardUnavailableReason explica, en una frase, por qué este agente no
	// declara AgentLevelGuard — vacío si sí lo declara. Vive en el registro
	// (no en el inspector) para que la degradación reportada en `mem doctor`
	// sea consecuencia de esta fila, nunca un caso especial escrito a mano en
	// el adaptador (FR-017, T049).
	GuardUnavailableReason string

	// EntryUnavailableReason es el equivalente para AgentLevelEntry. Existe por
	// el mismo motivo que su hermano: sin él, un agente que no declara el nivel
	// simplemente no producía fila alguna en `mem doctor`, y una ausencia sin
	// motivo es indistinguible de un olvido. Una degradación declarada es
	// información; un hueco silencioso es el defecto que este registro existe
	// para impedir.
	EntryUnavailableReason string
}

// HasLevel reporta si esta capacidad declara el nivel dado.
func (c AgentCapability) HasLevel(l AgentLevel) bool {
	return c.Levels[l]
}

// KnownAgents es el registro único de capacidades (data-model.md §5). Toda
// entrada DEBE declarar AgentLevelTextFloor (FR-A5). AgentLevelGuard solo se
// declara para agentes que puedan invocar un comando ANTES de presentar el
// plan y respetar su decisión — declararlo sin esa capacidad produciría un
// fallo silencioso.
var KnownAgents = []AgentCapability{
	{
		Name:    "claude",
		Dialect: DialectClaude,
		Levels: map[AgentLevel]bool{
			AgentLevelGuard:     true,
			AgentLevelEntry:     true,
			AgentLevelTextFloor: true,
		},
		Scopes: map[AgentScope]bool{
			ScopeProject: true,
			ScopeUser:    true,
		},
	},
	{
		Name:    "opencode",
		Dialect: DialectText,
		Levels: map[AgentLevel]bool{
			// Sin AgentLevelGuard: su ciclo no ofrece un punto de decisión
			// antes de presentar el plan — degradación declarada
			// (research.md §11), no cobertura oculta.
			AgentLevelEntry:     true,
			AgentLevelTextFloor: true,
		},
		Scopes: map[AgentScope]bool{
			ScopeProject: true,
			ScopeUser:    true,
		},
		GuardUnavailableReason: "el ciclo del agente no ofrece un punto de decisión antes de presentar el plan",
	},
	{
		Name:    "codex",
		Dialect: DialectText,
		Levels: map[AgentLevel]bool{
			// Ni Guard ni Entry: el ciclo de Codex no expone una herramienta de
			// entrada ni de salida del modo plan sobre la que enganchar un hook.
			// Lo que sí sostiene —y por eso entra al registro— son los eventos
			// SessionStart y Stop de su config.toml, que dan inyección de
			// contexto al arrancar y checkpoint al cerrar cada turno.
			AgentLevelTextFloor: true,
		},
		Scopes: map[AgentScope]bool{
			// Solo usuario: Codex registra su configuración una vez por máquina
			// en ~/.codex/config.toml, sin equivalente por proyecto.
			ScopeUser: true,
		},
		GuardUnavailableReason: "el ciclo del agente no ofrece un punto de decisión antes de presentar el plan",
		EntryUnavailableReason: "el ciclo del agente no expone un borde de entrada al modo plan sobre el que enganchar",
	},
}

// AgentByName busca una entrada del registro por nombre. ok=false significa
// que el agente no está declarado — NO que no pueda usar el contrato neutral
// (FR-A3): solo que no hay instalación dirigida ni entrada en el reporte para
// él todavía.
func AgentByName(name string) (AgentCapability, bool) {
	for _, a := range KnownAgents {
		if a.Name == name {
			return a, true
		}
	}
	return AgentCapability{}, false
}
