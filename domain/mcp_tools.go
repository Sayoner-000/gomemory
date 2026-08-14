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

	ToolPackBuild    = "pack_build"
	ToolPackShow     = "pack_show"
	ToolPackCompress = "pack_compress"
	ToolPackStats    = "pack_stats"
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

// MCPContextPackTools son las tools del Context Optimization Engine (feature
// 015): construyen/inspeccionan un domain.ContextPack. Ninguna es
// destructiva — solo cómputo y lectura, nada borra memorias.
var MCPContextPackTools = []string{
	ToolPackBuild,
	ToolPackShow,
	ToolPackCompress,
	ToolPackStats,
}

// MCPDestructiveTools nunca se pre-aprueban: son irreversibles y deben pasar
// siempre por confirmación explícita de la persona.
var MCPDestructiveTools = []string{ToolForgetMemory}

// MCPAllTools es el conjunto completo que registra el servidor.
func MCPAllTools() []string {
	out := make([]string, 0, len(MCPMemoryTools)+len(MCPCodeTools)+len(MCPContextPackTools))
	out = append(out, MCPMemoryTools...)
	out = append(out, MCPCodeTools...)
	out = append(out, MCPContextPackTools...)
	return out
}

// MCPAutoApprovableTools es el conjunto completo menos las destructivas: lo que
// puede pre-aprobarse sin riesgo por ser de solo lectura o de escritura acotada
// y reversible.
func MCPAutoApprovableTools() []string {
	out := make([]string, 0, len(MCPAllTools()))
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

// --- Proveedor externo de grafo de código (codebase-memory-mcp) ---
//
// Es un servidor MCP DISTINTO, no registrado por gomemory — por eso vive
// separado de MCPAllTools() y no puede formar parte de ella: el test de
// contrato levanta `mem mcp` y compara MCPAllTools() contra su propio
// tools/list, que nunca incluirá tools de otro servidor.
//
// Existe porque el protocolo de gomemory se declara "OBLIGATORIO y SIEMPRE
// ACTIVO" sin excepción por tipo de tarea (chat, plan, resumen): cuando el
// proveedor externo está habilitado (!CodeGraphDisabled, el mismo interruptor
// "Grafo de código externo" de la TUI), su materialización debe forzarse en
// el mismo bootstrap, siempre, no solo cuando la tarea "parece" necesitar
// código. Si el proveedor no está conectado, ToolSearch simplemente no
// encuentra esos nombres — degradación silenciosa, sin caso especial.
const CodebaseMemoryMCPPrefix = "mcp__codebase-memory-mcp__"

// CodebaseMemoryMCPDiscoveryTools son las tools de solo lectura que el propio
// hook de arranque del proveedor externo ya declara como uso obligatorio para
// exploración de código. Deliberadamente NO incluye sus operaciones de
// administración (index_repository, delete_project, manage_adr, ingest_traces,
// list_projects, detect_changes, get_graph_schema, index_status): forzar la
// materialización no debe extenderse a operaciones de escritura/administración
// de otro servidor.
var CodebaseMemoryMCPDiscoveryTools = []string{
	"search_graph",
	"trace_path",
	"get_code_snippet",
	"query_graph",
	"get_architecture",
	"search_code",
}
