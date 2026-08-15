# Contrato MCP (delta): `pack_build` con `no_code_graph`

**Feature**: [../spec.md](../spec.md) ·
**Base**: [../../015-context-optimization/contracts/mcp-tools.md](../../015-context-optimization/contracts/mcp-tools.md)

Sin tools nuevas — `domain/mcp_tools.go` no cambia (`ToolPackBuild` ya existe desde la
feature 015). Solo se agrega un campo de entrada a `pack_build`.

## `pack_build` (entrada agregada)

**Entrada**: `{ task: string, project?: string, max_tokens: int, min_relevance?: number,
max_items?: int, include_speckit?: bool, compression?: "none"|"structural",
no_code_graph?: bool }`

`no_code_graph` (no `include_code_graph`): ver research.md §5 para por qué el signo es
negativo — con un bool `omitempty`, el zero-value (`false`) debe significar "no
desactivar" para que el default sea "activado" (FR-007) también para clientes MCP que no
conocen este campo todavía, sin reproducir el default apagado que ya tiene
`include_speckit` por la misma razón de zero-value.

**Salida**: sin cambios — sigue siendo `domain.ContextPack` serializado
(`formatContextPack`). Cuando `IncludeCodeGraph` está activo y hay snapshot disponible, el
`ContextPack` puede incluir un ítem adicional con `Source` igual al nombre del proveedor
— ningún cliente MCP necesita tratarlo distinto de cualquier otro `ContextItem`.

**Wiring** (`registerTools`, `cmd_mcp.go:357`):

```go
func(ctx context.Context, req *mcp.CallToolRequest, in struct {
    Task           string  `json:"task" jsonschema:"..."`
    Project        string  `json:"project,omitempty" jsonschema:"..."`
    MaxTokens      int     `json:"max_tokens" jsonschema:"..."`
    MinRelevance   float64 `json:"min_relevance,omitempty" jsonschema:"..."`
    MaxItems       int     `json:"max_items,omitempty" jsonschema:"..."`
    IncludeSpeckit bool    `json:"include_speckit,omitempty" jsonschema:"..."`
    // Nuevo:
    NoCodeGraph bool `json:"no_code_graph,omitempty" jsonschema:"Desactiva la señal de grafo de código externo para esta llamada"`
}) (*mcp.CallToolResult, any, error) {
    // ...
    pack, err := usecases.BuildContextPack(deps.MemoryRepo, deps.Compressor, deps.TokenCounter, deps.SpecKitReader, usecases.ContextRequest{
        // ... campos existentes sin cambios ...
        IncludeCodeGraph: !in.NoCodeGraph,
        CodeProviders:    deps.CodeProviders,
    })
    // ...
}
```

`deps.CodeProviders` ya existe en `Deps` y ya se construye en el composition root — mismo
wiring que el CLI (contracts/cli.md), sin superficie nueva de configuración del lado MCP.
