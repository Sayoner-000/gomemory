# Contrato MCP: tools de Context Optimization

**Feature**: [../spec.md](../spec.md) · Justificación: [../research.md](../research.md) §9
(Constitución, Principio Operativo #9: "MCP como integración primaria").

## Regla de oro: agnóstico de agente y de proveedor (FR-014)

Estas tools son **el mismo servidor MCP `mem mcp` que ya expone gomemory hoy**, sobre
stdio estándar — no un servidor nuevo, no un protocolo nuevo. Ningún código de esta
feature puede:

- Asumir un cliente MCP concreto (Claude Code, OpenCode, Codex CLI, Gemini CLI, etc.):
  el payload de entrada/salida es JSON puro contra el esquema de la tool, igual que
  `save_memory`/`search_memories` ya funcionan hoy en los 4 canales de distribución
  del proyecto (ver `specs/014-codebase-memory-activation-tests`).
- Asumir un proveedor de LLM concreto: nada aquí llama a una API de OpenAI/Anthropic/
  Google/etc. — la compresión v1 es determinista (research.md §5) precisamente para
  no introducir esa dependencia.
- Vivir fuera de `domain/mcp_tools.go` como fuente de verdad de nombres: igual que el
  resto de tools del proyecto, agregar una tool aquí sin agregarla a esa lista única
  rompe el test de contrato existente que compara `MCPAllTools()` contra el
  `tools/list` real del servidor — ese test YA existe y ya atrapó este mismo tipo de
  bug una vez (`get_memory 321`, `get_memory 335`, `get_memory 350`).

## Tools nuevas

Prefijo de nombres: sin prefijo interno (el prefijo `mcp__gomemory__` o equivalente lo
añade cada cliente MCP, no gomemory — mismo patrón que `ToolGetContext`/`ToolSaveMemory`).

```go
// domain/mcp_tools.go — agregar a la única fuente de verdad
const (
    ToolPackBuild    = "pack_build"
    ToolPackShow     = "pack_show"
    ToolPackCompress = "pack_compress"
    ToolPackStats    = "pack_stats"
)

var MCPContextPackTools = []string{
    ToolPackBuild,
    ToolPackShow,
    ToolPackCompress,
    ToolPackStats,
}
```

`MCPAllTools()` pasa a incluir `MCPContextPackTools`; ninguna entra en
`MCPDestructiveTools` (todas son de solo cómputo/lectura, nada borra memorias).

### `pack_build`

**Entrada**: `{ task: string, project?: string, max_tokens: int, min_relevance?: number, max_items?: int, include_speckit?: bool, compression?: "none"|"structural" }`

**Salida**: `domain.ContextPack` serializado (ver data-model.md), o un error MCP con
`code` distinguible para `ErrCriticalContextOverflow` vs `ErrInvalidContextRequest` —
el cliente MCP (cualquiera que sea) debe poder diferenciarlos sin parsear el mensaje
en texto libre.

### `pack_show`

**Entrada**: `{ pack: <ContextPack JSON ya construido> }` — mismo modelo stateless que
`mem pack show` (contracts/cli.md): no hay "último paquete" guardado del lado del
servidor MCP entre llamadas.

**Salida**: el mismo `ContextPack`, reformateado a Markdown legible (`content` como
texto, no solo JSON) para que un agente lo pueda insertar directo en su propio
contexto de conversación.

### `pack_compress`

**Entrada**: `{ text: string }`

**Salida**: `{ content: string, raw_tokens: int, tokens: int, compressed: bool }` —
expone `CompressionResult` (go-api.md) sin pasar por retrieval ni budget.

### `pack_stats`

**Entrada**: `{ pack: <ContextPack JSON ya construido> }`

**Salida**: `domain.ContextStats` serializado.

## No se expone por MCP

`OptimizeToolDescription` (go-api.md) es una función de biblioteca para integradores
que registran sus propias tools MCP — no tiene sentido como tool MCP en sí misma
(optimizar la descripción de una tool vía otra tool es un caso de uso de integración,
no de agente). Queda disponible solo vía API Go y, si hace falta un empaquetado CLI
más adelante, un comando separado — no bloquea el resto de esta feature.
