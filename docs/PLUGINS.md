# Plugin System — gomemory

## Descripción

gomemory ahora incluye un sistema de plugins multi-agente que permite inyectar
contexto de memoria automáticamente en cada inferencia del agente, sin que el
usuario tenga que invocar herramientas MCP manualmente.

## Plugins Disponibles

| Agente | Tipo | Archivos |
|--------|------|----------|
| OpenCode | Plugin TypeScript | `infrastructure/plugin/opencode/gomemory.ts` |
| Claude Code | Hooks + Skill | `infrastructure/plugin/claude-code/` |

## Arquitectura

Cada agente integra la memoria por dos vías comunes (MCP stdio para las tools, y
el bloque del Memory Protocol en sus instrucciones) pero el **ciclo de vida**
—abrir/cerrar sesión, inyectar contexto, recuperar tras compactación— se conecta
distinto según el agente. En ambos casos se habla **directo al binario `mem`**
(sin servidor HTTP): Claude Code vía hooks `mem hook <evento>`, OpenCode vía un
plugin que ejecuta `mem <cmd>` como subproceso.

```
                 ┌──────────────────────────── OpenCode ───────────────────────┐
                 │  gomemory.ts  ──subproceso──▶  mem <cmd> / mem hook <evento> │
                 │                                  │                           │
 Agente ─────────┤                                  ├──▶ SQLite (store global)  │
                 │                                  │                           │
                 │  ┌────────────────────────── Claude Code ────────────────┐  │
                 │  │  hooks  ──▶  mem hook <evento>  ──directo──▶ repos ─────┘  │
                 │  └────────────────────────────────────────────────────────┘  │
                 └──────────────────────────────────────────────────────────────┘
        + común a ambos:  MCP stdio (mem mcp → tools)  ·  Memory Protocol en instrucciones
```

- **OpenCode** (`gomemory.ts`): gestiona el ciclo de vida ejecutando el binario
  `mem` como subproceso (`mem session start/end`, `mem context`, `mem hook nudge`,
  `mem hook post-compact`, `mem hook prompt`, `mem hook turn-end`). No usa HTTP.
- **Claude Code** (hooks): gestiona el ciclo de vida con **hooks portables**
  (`mem hook <evento>`) que hablan **directo a los repositorios** — sin HTTP, sin
  `bash`/`curl`. Funcionan igual en Windows.

### Componentes

1. **Plugin / hooks del agente**: integra la memoria en su ciclo de vida —
   gestiona sesiones, inyecta contexto y recupera tras compactación. Ambos
   invocan el binario `mem` directamente (subproceso en OpenCode, hooks en
   Claude Code).

2. **Memory Protocol**: conjunto de reglas inyectadas en las instrucciones del
   agente que definen cuándo guardar, buscar y cerrar memoria.

> **Nota**: el servidor HTTP legado (`mem serve` en `127.0.0.1:9735`) fue
> **retirado en v1.18.0**. Los plugins hablan directo al binario `mem`
> (subproceso/hooks); el MCP va por `stdio`, sin abrir ningún puerto.

### Hooks de Claude Code — para qué sirve cada uno

| Evento | Subcomando | Función |
|--------|-----------|---------|
| `SessionStart` (`startup\|resume\|clear`) | `session-start` | Abre sesión si no hay activa + inyecta contexto de sesiones previas |
| `SessionStart` (`compact`) | `post-compact` | **Después** de compactar, re-inyecta recuperación + contexto y reactiva las tools MCP diferidas. Sobrevive a la compactación (reemplaza al `pre-compact` legado) |
| `SessionEnd` | `session-end` | Cierra la sesión activa como red de seguridad (acepta `summary` por stdin) |
| `SubagentStop` | `subagent-stop` | Al terminar un subagente (`Task`), registra un checkpoint con su actividad |
| `UserPromptSubmit` | `user-prompt-submit` | Primer prompt: activa tools MCP + recordatorio del protocolo; luego pasivo. Además persiste el prompt del turno como provenance (equivale a `mem hook prompt`) |

### Eventos de OpenCode — para qué sirve cada uno

El plugin `gomemory.ts` mapea los eventos de OpenCode al binario `mem`, en
paralelo a los hooks de Claude Code (misma lógica, resuelta en Go):

| Evento OpenCode | Invoca | Función |
|-----------------|--------|---------|
| `session.created` | `mem session start` | Abre la sesión de gomemory |
| `session.idle` | `mem hook turn-end` (stdin) | Checkpoint de actividad del turno (archivos/comandos) |
| `chat.message` | `mem hook prompt` (stdin) | Persiste el prompt del turno como provenance (`origin_prompt`) |
| `experimental.chat.system.transform` | `mem context` + `mem hook nudge` | Inyecta protocolo + contexto histórico + recordatorio de guardado por turno |
| `experimental.session.compacting` | `mem hook post-compact` | Empuja recuperación + contexto al contexto retenido (sobrevive a la compactación) |
| `dispose` | `mem session end` | Cierra la sesión al descargarse el plugin |

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
```

## Servidor HTTP (retirado en v1.18.0)

El servidor HTTP legado (`mem serve` en `127.0.0.1:9735`) fue **retirado**. Ni
OpenCode ni Claude Code lo necesitan: hablan directo al binario `mem` vía
subproceso/hooks, y el MCP va por `stdio` sin puerto.

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
