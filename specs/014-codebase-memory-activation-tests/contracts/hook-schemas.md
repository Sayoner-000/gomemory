# Contracts: Hook JSON Schemas

**Date**: 2026-08-08

Los hooks de Claude Code producen JSON que el agente parsea. Estos son los schemas reales verificados contra el binario compilado.

## Hook: user-prompt-submit (primer prompt)

**Invocación**: `mem hook user-prompt-submit <project-dir>`

**Precondición**: No existe marker `.memory/.session-started` (es el primer prompt de la sesión).

**Output JSON**:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "PRIMERA ACCIÓN — ejecuta este ToolSearch AHORA...\nselect:<tool_names>\n..."
  }
}
```

**Campos requeridos**:
- `hookSpecificOutput.hookEventName`: `"UserPromptSubmit"`
- `hookSpecificOutput.additionalContext`: String con `select:` que nombra las tools

**Campos prohibidos**:
- `systemMessage`: NO debe existir — Claude Code lo muestra solo al humano, nunca al modelo

**Contenido de additionalContext**:
- `select:` con 21 tools (15 gomemory + 6 codebase-memory-mcp)
- Instrucción `get_plan_context` para modo plan
- Protocolo de memoria persistente
- Sección `GRAFO DE CÓDIGO EXTERNO` con las 6 tools

## Hook: subagent-start

**Invocación**: `mem hook subagent-start <project-dir>`

**Precondición**: Ninguna (un subagente puede arrancar sin sesión previa).

**Output JSON**:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SubagentStart",
    "additionalContext": "PRIMERA ACCIÓN — ejecuta este ToolSearch AHORA...\nselect:<tool_names>\n..."
  }
}
```

**Campos requeridos**: Mismos que user-prompt-submit excepto `hookEventName: "SubagentStart"`.

**Diferencia con user-prompt-submit**: No hayrama de nudge (prompts subsiguientes) — el subagente solo recibe el bootstrap una vez.

## Hook: user-prompt-submit (prompt subsiguiente)

**Invocación**: `mem hook user-prompt-submit <project-dir>`

**Precondición**: Existe marker `.memory/.session-started`.

**Output JSON** (cuando hay nudge):

```json
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "RECORDATORIO DE MEMORIA: pasaron más de 15 minutos..."
  }
}
```

**Output JSON** (sin nudge):

```json
{}
```

**Nota**: Este caso NO es validado por el script de regresión — el script solo valida el primer prompt (bootstrap).

## Contrato de tools

### Tools de gomemory (15, prefijo `mcp__gomemory__`)

```
get_context, get_plan_context, save_memory, search_memories, list_memories,
get_memory, forget_memory, judge_memories, start_session, end_session,
search_code, get_symbol, list_dependencies, graph_status, index_project
```

### Tools de codebase-memory-mcp (6, prefijo `mcp__codebase-memory-mcp__`)

```
search_graph, trace_path, get_code_snippet, query_graph, get_architecture, search_code
```

### Tools admin (4, NO deben aparecer en bootstrap)

```
index_repository, delete_project, manage_adr, ingest_traces
```
