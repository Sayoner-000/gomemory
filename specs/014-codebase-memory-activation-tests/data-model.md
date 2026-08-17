# Data Model: codebase-memory-mcp activation regression tests

**Date**: 2026-08-08

Este feature no maneja persistencia de datos. El "data model" es la estructura de los outputs que el script valida.

## Entidades validadas

### Canal de distribución

Cada mecanismo por el que gomemory inyecta instrucciones al agente.

| Canal | Fuente | Output validado |
|-------|--------|-----------------|
| Claude Code hook | `mem hook user-prompt-submit` | JSON con `hookSpecificOutput.additionalContext` |
| Subagente | `mem hook subagent-start` | JSON con `hookSpecificOutput.additionalContext` |
| OpenCode plugin | `gomemory.ts` | Texto plano con `EXTERNAL CODE GRAPH` |
| Integración | `cmd_install.go` | Código Go con referencia a `codebase-memory-mcp` |
| MCP instructions | `cmd_mcp.go` | Código Go con `buildIntegrationBlock` |

### Tool de descubrimiento (codebase-memory-mcp)

Las 6 tools de solo lectura del proveedor externo que se materializan para exploración de código.

| Tool | Prefijo Claude Code | Prefijo OpenCode |
|------|--------------------|--------------------|
| search_graph | `mcp__codebase-memory-mcp__search_graph` | `codebase-memory-mcp_search_graph` |
| trace_path | `mcp__codebase-memory-mcp__trace_path` | `codebase-memory-mcp_trace_path` |
| get_code_snippet | `mcp__codebase-memory-mcp__get_code_snippet` | `codebase-memory-mcp_get_code_snippet` |
| query_graph | `mcp__codebase-memory-mcp__query_graph` | `codebase-memory-mcp_query_graph` |
| get_architecture | `mcp__codebase-memory-mcp__get_architecture` | `codebase-memory-mcp_get_architecture` |
| search_code | `mcp__codebase-memory-mcp__search_code` | `codebase-memory-mcp_search_code` |

### Tools admin (NO deben aparecer en el bootstrap)

| Tool | Razón de exclusión |
|------|-------------------|
| index_repository | Escritura/admin de otro servidor |
| delete_project | Operación destructiva |
| manage_adr | Escritura |
| ingest_traces | Escritura |

### Bootstrap (select: de ToolSearch)

El `select:` es una lista separada por comas de nombres de tools que Claude Code materializa vía ToolSearch. Debe incluir:
- Todas las tools de gomemory (15)
- Las 6 tools de codebase-memory-mcp (cuando CodeGraphDisabled=false)
- Ninguna tool admin

## Estado del test

```
PASS  → check exitoso (string encontrado/no encontrado como se esperaba)
FAIL  → check fallido (string encontrado cuando no debía, o no encontrado cuando debía)
```

El script termina con:
- `exit 0` si PASS > 0 y FAIL == 0
- `exit 1` si FAIL > 0
