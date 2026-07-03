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

### Memoria dinámica (recordatorio por hook)

El sistema no depende de la fuerza de voluntad del modelo: el hook lo empuja.

- `mem hook user-prompt-submit` — en el primer prompt de la sesión inyecta el
  bootstrap de tools MCP + un recordatorio corto del protocolo ("¿decisión / bug
  / patrón / hallazgo? → `save_memory` ahora"). Los prompts siguientes son
  pasivos (sin overhead). El marcador `.memory/.session-tools-injected` controla
  el "primer prompt".
- `mem hook pre-compact` — antes de compactar, inyecta instrucciones de
  recuperación + el contexto previo.
- `mem hook session-end` — cierra la sesión como **red de seguridad**, aunque el
  modelo no haya llamado `end_session`. El resumen rico lo aporta el modelo.

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

| Capa | Dónde se inyecta | Sobrevive compactación? |
|------|------------------|------------------------|
| System Prompt | Via plugin transform | ✅ Siempre |
| Per-prompt reminder | `mem hook user-prompt-submit` | ✅ Se reinyecta por sesión |
| Compaction Hook | `mem hook pre-compact` | ✅ Se ejecuta pre-compactación |
| Agent Config | AGENTS.md / CLAUDE.md | ✅ Siempre |

## Token Budget

| Componente | Tokens | Frecuencia |
|------------|--------|------------|
| Memory Protocol | ~380 | Cada inferencia |
| Context Injection | ~180 | Al iniciar sesión |
| Búsqueda (Layer 1) | ~100 | Bajo demanda |
| Timeline (Layer 2) | ~200 | Bajo demanda |
| Contenido completo (Layer 3) | ~500+ | Bajo demanda |

**Ahorro estimado**: ~60% vs volcar toda la memoria (~5000+ tokens cada vez).
