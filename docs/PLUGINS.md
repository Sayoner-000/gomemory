# Plugin System — gomemory

## Descripción

gomemory ahora incluye un sistema de plugins multi-agente que permite inyectar
contexto de memoria automáticamente en cada inferencia del agente, sin que el
usuario tenga que invocar herramientas MCP manualmente.

## Plugins Disponibles

| Agente | Tipo | Archivos |
|--------|------|----------|
| OpenCode | Plugin TypeScript | `infrastructure/plugin/opencode/plugin.ts` |
| Claude Code | Hooks + Skill | `infrastructure/plugin/claude-code/` |

## Arquitectura

Cada agente integra la memoria por dos vías comunes (MCP stdio para las tools, y
el bloque del Memory Protocol en sus instrucciones) pero el **ciclo de vida**
—abrir/cerrar sesión, inyectar contexto, recuperar tras compactación— se conecta
distinto según el agente:

```
                 ┌──────────────────────────── OpenCode ───────────────────────┐
                 │  plugin.ts  ──HTTP──▶  mem serve (127.0.0.1:9735)            │
                 │                                  │                           │
 Agente ─────────┤                                  ├──▶ SQLite (.memory/mem.db)│
                 │                                  │                           │
                 │  ┌────────────────────────── Claude Code ────────────────┐  │
                 │  │  hooks  ──▶  mem hook <evento>  ──directo──▶ repos ─────┘  │
                 │  └────────────────────────────────────────────────────────┘  │
                 └──────────────────────────────────────────────────────────────┘
        + común a ambos:  MCP stdio (mem mcp → tools)  ·  Memory Protocol en instrucciones
```

- **OpenCode** (`plugin.ts`): gestiona el ciclo de vida vía el **servidor HTTP**
  (`mem serve`), que auto-inicia.
- **Claude Code** (hooks): gestiona el ciclo de vida con **hooks portables**
  (`mem hook <evento>`) que hablan **directo a los repositorios** — sin HTTP, sin
  `bash`/`curl`. Funcionan igual en Windows.

### Componentes

1. **Plugin / hooks del agente**: integra la memoria en su ciclo de vida —
   gestiona sesiones, inyecta contexto y recupera tras compactación.

2. **Servidor HTTP** (`mem serve`): background para **OpenCode**. Maneja sesiones
   y genera contexto. Escucha en `127.0.0.1:9735` por defecto. Claude Code no lo usa.

3. **Memory Protocol**: conjunto de reglas inyectadas en las instrucciones del
   agente que definen cuándo guardar, buscar y cerrar memoria.

### Hooks de Claude Code — para qué sirve cada uno

| Evento | Subcomando | Función |
|--------|-----------|---------|
| `SessionStart` | `session-start` | Abre sesión si no hay activa + inyecta contexto de sesiones previas |
| `SessionEnd` | `session-end` | Cierra la sesión activa como red de seguridad (acepta `summary` por stdin) |
| `PreCompact` | `pre-compact` | Inyecta instrucciones de recuperación + contexto antes de compactar |
| `UserPromptSubmit` | `user-prompt-submit` | Primer prompt: activa tools MCP + recordatorio del protocolo; luego pasivo |

### Capas de Resiliencia

| Capa | Mecanismo | Sobrevive Compactación? |
|------|-----------|------------------------|
| System Prompt | Memory Protocol inyectado en system message | Siempre presente |
| Compaction Hook | Salva checkpoint + inyecta contexto + instruye al compressor | Se activa en compactación |
| Config del Agente | "Después de compactar, llama get_context()" | Siempre presente |

## Instalación

```bash
# Plugin para OpenCode
mem setup opencode

# Plugin para Claude Code
mem setup claude-code

# Con puerto personalizado (los flags van ANTES del agente)
mem setup --port 9735 opencode
```

## Servidor HTTP

El servidor HTTP de background es auto-iniciado por los plugins. También puede
iniciarse manualmente:

```bash
mem serve              # Puerto por defecto 9735
mem serve --port 9735  # Puerto personalizado
```

### API HTTP

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `POST /session/start` | Crear sesión (o reusar activa) | |
| `POST /session/end` | Cerrar sesión con resumen | |
| `GET /context` | Obtener contexto de sesiones previas | |
| `GET /health` | Healthcheck | |

## Memory Protocol

El Memory Protocol es un bloque de ~380 tokens inyectado en el system prompt
del agente. Define:

### Cuándo Guardar (Save Triggers)
- Después de decisiones de arquitectura
- Después de corregir bugs (incluir causa raíz)
- Después de establecer patrones o convenciones
- Después de descubrimientos no obvios sobre el código
- Autochequeo después de CADA tarea

### Cuándo Buscar (Search Triggers)
- **Reactivo**: cuando el usuario pregunta "recuerdas..." o similar
- **Proactivo**: al iniciar tareas que podrían solaparse con trabajo previo

### Revelación Progresiva (3 Capas)
1. `search_memories()` — resúmenes compactos (~100 tokens)
2. `get_memory()` — contenido completo (~500+ tokens)
3. Nunca volcar toda la memoria — buscar primero, profundizar solo si necesario

### Protocolo de Cierre de Sesión
Antes de terminar, el agente DEBE llamar `end_session()` con:
- Goal, Discoveries, Accomplished, Next Steps, Relevant Files

### Recuperación Post-Compactación
Después de compactación, el agente DEBE:
1. Persistir resumen via `end_session(summary)`
2. Recuperar estado via `get_context()`
3. Solo entonces continuar trabajando
