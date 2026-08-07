package domain

// Nombres de las tools MCP que expone gomemory. Esta es la ÚNICA fuente de
// verdad: el bootstrap de ToolSearch, las listas de auto-aprobación, el bloque
// de protocolo y la documentación derivan de aquí.
//
// Existe porque el mismo nombre estaba copiado a mano en media docena de sitios
// y una tool nueva (get_plan_context, feature 013) quedó fuera de casi todos:
// el agente leía "usa esta tool" en el protocolo y no podía invocarla porque
// ningún ToolSearch la materializaba. Un test de contrato compara esta lista
// contra las tools realmente registradas por el servidor, así que añadir una
// tool sin añadirla aquí rompe la compilación de la suite, no el runtime del
// usuario.
const (
	ToolSaveMemory     = "save_memory"
	ToolSearchMemories = "search_memories"
	ToolListMemories   = "list_memories"
	ToolGetMemory      = "get_memory"
	ToolForgetMemory   = "forget_memory"
	ToolJudgeMemories  = "judge_memories"
	ToolStartSession   = "start_session"
	ToolEndSession     = "end_session"
	ToolGetContext     = "get_context"
	ToolGetPlanContext = "get_plan_context"

	ToolIndexProject     = "index_project"
	ToolGraphStatus      = "graph_status"
	ToolSearchCode       = "search_code"
	ToolGetSymbol        = "get_symbol"
	ToolListDependencies = "list_dependencies"
)

// MCPMemoryTools son las tools del núcleo de memoria.
var MCPMemoryTools = []string{
	ToolGetContext,
	ToolGetPlanContext,
	ToolSaveMemory,
	ToolSearchMemories,
	ToolListMemories,
	ToolGetMemory,
	ToolForgetMemory,
	ToolJudgeMemories,
	ToolStartSession,
	ToolEndSession,
}

// MCPCodeTools son las tools del grafo de código PROPIO de gomemory (Go, vía
// go/parser). No dependen de ningún proveedor externo.
var MCPCodeTools = []string{
	ToolSearchCode,
	ToolGetSymbol,
	ToolListDependencies,
	ToolGraphStatus,
	ToolIndexProject,
}

// MCPDestructiveTools nunca se pre-aprueban: son irreversibles y deben pasar
// siempre por confirmación explícita de la persona.
var MCPDestructiveTools = []string{ToolForgetMemory}

// MCPAllTools es el conjunto completo que registra el servidor.
func MCPAllTools() []string {
	out := make([]string, 0, len(MCPMemoryTools)+len(MCPCodeTools))
	out = append(out, MCPMemoryTools...)
	out = append(out, MCPCodeTools...)
	return out
}

// MCPAutoApprovableTools es el conjunto completo menos las destructivas: lo que
// puede pre-aprobarse sin riesgo por ser de solo lectura o de escritura acotada
// y reversible.
func MCPAutoApprovableTools() []string {
	out := make([]string, 0, len(MCPMemoryTools)+len(MCPCodeTools))
	for _, t := range MCPAllTools() {
		if !isDestructiveTool(t) {
			out = append(out, t)
		}
	}
	return out
}

func isDestructiveTool(name string) bool {
	for _, d := range MCPDestructiveTools {
		if d == name {
			return true
		}
	}
	return false
}

// MCPPrefixed devuelve los nombres con el prefijo que usa un agente concreto
// para las tools de un servidor MCP (p. ej. "mcp__gomemory__" en Claude Code).
func MCPPrefixed(prefix string, names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, prefix+n)
	}
	return out
}
