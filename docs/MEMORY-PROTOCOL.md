# Memory Protocol — Referencia Técnica

## Propósito

El Memory Protocol es un conjunto de reglas que se inyectan en el system prompt
del agente AI para definir cuándo y cómo interactuar con el sistema de memoria
de gomemory. Su objetivo es maximizar el valor del contexto persistido mientras
se minimiza el consumo de tokens.

## Estructura del Protocolo

```
Memory Protocol (~380 tokens total)
├── PROACTIVE SAVE (~80 tokens)
│   ├── Cuándo guardar (6 triggers)
│   ├── Autochequeo post-tarea
│   └── Formato de título
├── WHEN TO SEARCH (~60 tokens)
│   ├── Búsqueda reactiva
│   └── Búsqueda proactiva
├── PROGRESSIVE DISCLOSURE (~60 tokens)
│   ├── Capa 1: search (100 tokens)
│   ├── Capa 2: get (500+ tokens)
│   └── Regla: no volcar todo
├── SESSION CLOSE (~100 tokens)
│   ├── Goal
│   ├── Discoveries
│   ├── Accomplished
│   ├── Next Steps
│   └── Relevant Files
└── AFTER COMPACTION (~80 tokens)
    ├── Paso 1: end_session(summary)
    ├── Paso 2: get_context()
    └── Paso 3: continuar
```

## Inyección en Agentes

### OpenCode

El plugin TypeScript inyecta el protocolo via `chat.system.transform`,
concatenándolo al mensaje de sistema existente (no como mensaje separado).
Esto garantiza compatibilidad con modelos que solo aceptan un bloque de
sistema (Qwen, Mistral/Ministral via llama.cpp).

### Claude Code

El protocolo se inyecta como skill de memoria en `skills/memory/SKILL.md`,
disponible permanentemente para el agente. Además, los hooks portables
(`mem hook <evento>`, sin scripts shell ni servidor HTTP) inyectan contexto y
recordatorios en momentos específicos.

### Cualquier agente MCP (Cursor, Windsurf, Cline, Codex, o Claude/OpenCode sin `mem install`)

Estos agentes no tienen plugin ni hooks propios, pero el protocolo llega igual
porque vive en el propio servidor `mem mcp` (`adapters/primary/cli/cmd_mcp.go`),
no en un archivo del proyecto:

1. **`initialize.instructions`** — el campo `Instructions` de `ServerOptions`
   lleva el bloque completo del protocolo (mismo texto que `buildIntegrationBlock()`
   inserta en `AGENTS.md`/`CLAUDE.md` vía `mem install`, una sola fuente de verdad).
   Es el mecanismo que el propio spec de MCP definió para esto; el cliente
   decide si lo muestra al modelo.
2. **Descripciones de tools** — `save_memory`, `get_context`, `start_session`,
   `end_session` incluyen el "cuándo llamar" directamente en su `Description`.
   Esta capa es 100% garantizada: cualquier cliente MCP lista las tools con su
   descripción, sin excepción.
3. **`get_context` embebe el recordatorio en su propia respuesta** — el texto de
   `memoryProtocolReminder` (el mismo que usa el hook `user-prompt-submit` de
   Claude Code) va concatenado al inicio del resultado de la tool y del recurso
   `mem://context`. Es la capa más fuerte de las tres: el resultado de una tool
   siempre vuelve al modelo, en cualquier cliente, sin depender de que honre
   `instructions`.

Con esto, el bloque estático en `AGENTS.md`/`CLAUDE.md` deja de ser necesario
para que el protocolo funcione — queda como refuerzo opcional (útil como red de
seguridad si un cliente ignora `instructions`), no como requisito.

### Memoria dinámica (recordatorio por hook)

El sistema no depende de la fuerza de voluntad del modelo: el hook lo empuja.

- `mem hook user-prompt-submit` — en el **primer prompt** de la sesión fuerza la
  carga de las tools MCP (que en Claude Code llegan diferidas) con un
  `systemMessage` que incluye un `ToolSearch select:` con los nombres reales de
  gomemory, e inyecta el recordatorio del protocolo como `additionalContext`. En
  los prompts **siguientes** ya no es mudo: si el agente lleva más de 15 minutos
  sin un guardado real (y la sesión tiene más de 5 minutos), inyecta un
  recordatorio de guardado con enfriamiento de 15 minutos (marcador
  `.memory/.last-nudge`). El marcador `.memory/.session-tools-injected` controla
  el "primer prompt".
- `mem hook nudge` — punto de entrada **transversal** del recordatorio de
  guardado: imprime el texto (o nada) según la misma decisión que usa el hook de
  Claude Code. Lo consumen integraciones que no leen el JSON de Claude Code, como
  el plugin de OpenCode (que lo invoca por turno). Así el umbral y el debounce
  son idénticos en todos los agentes.
- `mem hook pre-compact` — antes de compactar, inyecta instrucciones de
  recuperación + el contexto previo.
- `mem hook session-end` — cierra la sesión como **red de seguridad**, aunque el
  modelo no haya llamado `end_session`. El resumen rico lo aporta el modelo.

> **Guardado real vs checkpoint**: el recordatorio mide el tiempo desde la última
> memoria cuyo tipo NO es `checkpoint`. Los checkpoints automáticos (actividad de
> turno) no cuentan, para que el agente sea recordado justamente cuando trabaja
> sin registrar decisiones ni hallazgos.

Decisión de diseño: el agente decide qué guardar (con el empujón del
recordatorio); no se hace extracción autónoma con LLM ni se requieren API keys.

## Contenido del Protocolo

### Save Triggers

```text
Call save_memory() IMMEDIATELY after:
- Architecture or design decision made
- Bug fix completed (include root cause)
- Team convention or workflow change agreed
- Tool or library choice with tradeoffs
- Non-obvious discovery about the codebase
- Pattern established (naming, structure, convention)
- User preference or constraint learned (type=preference — interactive/session
  memory about how the user wants to be worked with; save it here, not in an
  external store)

Self-check after EVERY task: "Did I make a decision, fix a bug,
discover something, or establish a convention?"
```

### Search Triggers

```text
Call search_memories() REACTIVELY when:
- User says "remember", "recall", "what did we do"
- User references past work in any language

Call search_memories() PROACTIVELY when:
- Starting work on something that might overlap with past sessions
- Task mentions a topic you have no context on
```

### Progressive Disclosure

```text
Token-efficient memory retrieval:
1. search_memories(query) → compact results (~100 tokens each)
2. get_memory(id) → full untruncated content only when needed
3. Never dump all memory — search first, drill only if necessary
```

### Session Close Protocol

```text
Before ending, call end_session() with:

## Goal
[What we were working on this session]

## Discoveries
- [Technical findings, gotchas, non-obvious learnings]

## Accomplished
- [Completed items with key details]

## Next Steps
- [What remains to be done]

## Relevant Files
- path/to/file — [what it does or what changed]
```

### Compaction Recovery

```text
After compaction, IMMEDIATELY:
1. Call end_session(summary) with the compacted content
2. Call get_context() to recover previous state
3. Only THEN continue working
```

## Capas de Inyección

| Capa | Dónde se inyecta | Requiere `mem install`/archivo en el repo? | Sobrevive compactación? |
|------|------------------|---------------------------------------------|------------------------|
| System Prompt (OpenCode) | Via plugin transform | Sí (o `mem setup opencode`) | ✅ Siempre |
| Bootstrap de tools + reminder (Claude Code) | `mem hook user-prompt-submit` (1er prompt) | Sí (o `mem setup claude-code`) | ✅ Se reinyecta por sesión |
| Recordatorio de guardado (transversal) | `mem hook user-prompt-submit` (siguientes) / `mem hook nudge` (OpenCode y otros) | Sí (o `mem setup <agente>`) | ✅ Por turno, con debounce |
| Compaction Hook (Claude Code) | `mem hook pre-compact` | Sí (o `mem setup claude-code`) | ✅ Se ejecuta pre-compactación |
| `initialize.instructions` (MCP nativo) | `mem mcp` (`ServerOptions.Instructions`) | **No** — cualquier scope/agente | ✅ Una vez por conexión |
| Descripciones de tools (MCP nativo) | `mem mcp` (`Tool.Description`) | **No** — cualquier scope/agente | ✅ Siempre visibles |
| `get_context` embebido (MCP nativo) | `mem mcp` (tool + recurso `mem://context`) | **No** — cualquier scope/agente | ✅ Cada llamada |
| Agent Config (refuerzo opcional) | AGENTS.md / CLAUDE.md | Sí (`mem install`) | ✅ Siempre |

## Token Budget

| Componente | Tokens | Frecuencia |
|------------|--------|------------|
| Memory Protocol | ~380 | Cada inferencia |
| Context Injection | ~180 | Al iniciar sesión |
| Búsqueda (Layer 1) | ~100 | Bajo demanda |
| Timeline (Layer 2) | ~200 | Bajo demanda |
| Contenido completo (Layer 3) | ~500+ | Bajo demanda |

**Ahorro estimado**: ~60% vs volcar toda la memoria (~5000+ tokens cada vez).
