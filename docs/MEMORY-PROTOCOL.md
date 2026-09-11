# Memory Protocol — Referencia Técnica

## Propósito

El Memory Protocol es un conjunto de reglas que se inyectan en el system prompt
del agente AI para definir cuándo y cómo interactuar con el sistema de memoria
de gomemory. Su objetivo es maximizar el valor del contexto persistido mientras
se minimiza el consumo de tokens.

El protocolo se apoya en un [baseline universal de agentes](./UNIVERSAL-AGENT-INSTRUCTIONS.md)
instalado una vez por usuario. Ese baseline es neutral; este documento describe
la capa específica de GoMemory y sus inyecciones dinámicas.

## Estructura del Protocolo

```
Memory Protocol
├── PROACTIVE SAVE
│   ├── Cuándo guardar (6 triggers)
│   ├── Autochequeo post-tarea
│   └── Formato de título
├── WHEN TO SEARCH
│   ├── Búsqueda reactiva
│   └── Búsqueda proactiva
├── PROGRESSIVE DISCLOSURE
│   ├── Capa 1: search/list (extractos)
│   ├── Capa 2: get (contenido completo)
│   └── Regla: no volcar todo
├── SESSION CLOSE
│   ├── Goal
│   ├── Discoveries
│   ├── Accomplished
│   ├── Next Steps
│   └── Relevant Files
└── AFTER COMPACTION
    ├── Paso 1: save_session_summary(summary), si aún no se guardó
    ├── Paso 2: revisar la memoria de la sesión
    └── Paso 3: continuar sin cerrar la sesión
```

## Inyección en Agentes

### OpenCode

El plugin TypeScript añade el protocolo y el contexto dinámico desde
`experimental.chat.system.transform`. También consume los eventos de sesión,
compactación y herramientas para conservar la continuidad sin depender de
archivos de instrucciones dentro del proyecto.

### Claude Code

El protocolo se inyecta como skill de memoria en `skills/memory/SKILL.md`,
disponible permanentemente para el agente. Además, los hooks portables
(`mem hook <evento>`, sin scripts shell ni servidor HTTP) inyectan contexto y
recordatorios en momentos específicos.

### Cualquier agente MCP

El protocolo llega incluso cuando el cliente no tiene una integración de ciclo
de vida, porque vive en el propio servidor `mem mcp`
(`adapters/primary/cli/cmd_mcp.go`) y no en un archivo del proyecto:

1. **`initialize.instructions`** — el campo `Instructions` de `ServerOptions`
   lleva el bloque completo del protocolo generado por la misma fuente que usan
   las integraciones de ámbito de usuario.
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

`mem install` no escribe este bloque en `AGENTS.md` ni `CLAUDE.md`. El refuerzo
textual opcional vive en los archivos de instrucciones de ámbito de usuario que
genera `mem setup-mcp --scope global`.

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
- `mem hook turn-end` — al cerrar cada turno, además del checkpoint
  automático de actividad, reevalúa dos recordatorios que comparten el mismo
  contador de huella (`compact_threshold`): si la huella superó el umbral
  completo, sugiere compactar; si no, pero superó **un tercio** del umbral,
  reinyecta el **título y contenido real** de las preferencias del usuario
  (`type=preference`) más recientes — no un recordatorio genérico, el texto de
  la regla en sí — con enfriamiento de 20 minutos (marcador
  `.memory/.last-preference-nudge`). Cubre el hueco entre `SessionStart` y
  `post-compact`, los únicos dos puntos donde antes se reinyectaban las
  preferencias: una sesión larga que nunca llega a compactar ya no las pierde
  de vista.
- `mem hook subagent-stop` — cuando termina un subagente, guarda su checkpoint
  de actividad. Si el mensaje final contiene una sección `## Aprendizajes clave`
  o `## Key Learnings`, también persiste sus ítems válidos como memorias
  `learning`, con deduplicación por `topic_key`.
- `mem hook plan-approved` — captura **determinista de las decisiones al aprobar
  un plan**. Un turno de plan mode es puro chat (sin ediciones ni comandos), así
  que `turn-end` lo descartaría y las decisiones del plan se perderían. Guarda el
  plan como memoria `decision`, sin gastar tokens ni depender de que el modelo
  llame `save_memory`. Es **transversal**: acepta el plan en `tool_input.plan`
  (Claude Code lo registra como `PostToolUse` matcher `ExitPlanMode`, que solo
  dispara si el usuario aprobó) o en `plan` de nivel superior (el plugin de
  OpenCode lo invoca al detectar un turno con `info.mode==="plan"`). Append-only:
  cada aprobación acumula, para no perder la evolución de las decisiones.
- `mem hook compaction-context` — entrega al compresor la memoria de la sesión
  activa y la instrucción de conservarla. OpenCode lo invoca desde
  `experimental.session.compacting`; no altera el estado de la sesión.
- `mem hook compact-summary` — persiste el resumen que entrega el cliente sin
  cerrar la sesión. Claude Code lo invoca desde `PostCompact`; OpenCode desde
  `session.compacted` cuando encuentra el mensaje marcado como resumen.
- `mem hook post-compact` — después de compactar, garantiza una sesión activa,
  reinicia los marcadores de huella y reinyecta los pasos de recuperación, la
  memoria de la sesión y el contexto de proyecto en modo índice.
- `mem hook agent-notice` — consume una sola vez el aviso opcional que pide al
  agente guardar decisiones pendientes al superar `compact_threshold`.
- `mem hook prompt` — punto de entrada **transversal** de la captura del prompt
  originante: recibe `{"prompt": …}` por stdin y lo persiste en la sesión activa
  (`sessions.last_prompt`). Al guardar cualquier memoria, `InsertMemory` adjunta
  ese prompt como `origin_prompt` (provenance: por qué se guardó esto). En Claude
  Code la captura va **inline** dentro de `user-prompt-submit`; el plugin de
  OpenCode lo invoca desde su evento `chat.message`. Agentes sin hook de mensaje
  (Cursor, Windsurf y Cline) simplemente no aportan prompt: `origin_prompt`
  queda vacío (degradación limpia).
- `mem hook session-end` — cierra la sesión como **red de seguridad**, aunque el
  modelo no haya llamado `end_session`. El resumen rico lo aporta el modelo.

> **Guardado real vs checkpoint**: el recordatorio mide el tiempo desde la última
> memoria cuyo tipo NO es `checkpoint`. Los checkpoints automáticos (actividad de
> turno) no cuentan, para que el agente sea recordado justamente cuando trabaja
> sin registrar decisiones ni hallazgos.

Decisión de diseño: el agente decide qué guardar durante el trabajo. La captura
pasiva de aprendizajes solo procesa una sección explícita mediante reglas
deterministas; no usa un LLM ni requiere API keys.

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
  external store). A preference is a FIXED RULE, not an incident log: if the
  same correction happens again, reuse a topic_key (or the same title) to
  UPDATE that memory instead of creating a new one. Do not quote examples of
  the wrong behavior in the content — repeating them can reinforce the
  pattern instead of correcting it.

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
1. If the compacted summary was not persisted by the client integration, call
   save_session_summary(summary)
2. Review the injected session memory and retrieve details only when needed
3. Only THEN continue working; do not close the session
```

## Capas de Inyección

| Capa | Dónde se inyecta | Requiere `mem install`/archivo en el repo? | Sobrevive compactación? |
|------|------------------|---------------------------------------------|------------------------|
| System Prompt (OpenCode) | Vía plugin transform | Sí (o `mem setup opencode`) | ✅ Siempre |
| Bootstrap de tools + reminder (Claude Code) | `mem hook user-prompt-submit` (1er prompt) | Sí (o `mem setup claude-code`) | ✅ Se reinyecta por sesión |
| Recordatorio de guardado (transversal) | `mem hook user-prompt-submit` (siguientes) / `mem hook nudge` (OpenCode y otros) | Sí (o `mem setup <agente>`) | ✅ Por turno, con debounce |
| Contexto previo a compactar | `mem hook compaction-context` — OpenCode: `experimental.session.compacting` | Sí (o `mem setup opencode`) | Lo consume el compresor |
| Persistencia del resumen | `mem hook compact-summary` — Claude Code: `PostCompact`; OpenCode: `session.compacted` | Sí (o `mem setup <agente>`) | ✅ No cierra la sesión |
| Recuperación post-compactación | `mem hook post-compact` — Claude Code: `SessionStart` matcher `compact`; OpenCode: recuperación pendiente tras `session.compacted` | Sí (o `mem setup <agente>`) | ✅ Se entrega después de compactar |
| `initialize.instructions` (MCP nativo) | `mem mcp` (`ServerOptions.Instructions`) | **No** — cualquier scope/agente | ✅ Una vez por conexión |
| Descripciones de tools (MCP nativo) | `mem mcp` (`Tool.Description`) | **No** — cualquier scope/agente | ✅ Siempre visibles |
| `get_context` embebido (MCP nativo) | `mem mcp` (tool + recurso `mem://context`) | **No** — cualquier scope/agente | ✅ Cada llamada |
| Instrucciones de usuario (refuerzo opcional) | Archivo global propio del agente | `mem setup-mcp --scope global` | ✅ Siempre |

## Medición del costo

El tamaño depende de la configuración, el historial y las capacidades del
cliente. `mem usage` muestra la línea base, lo emitido y el ahorro medido por
sesión y canal. No se mantienen estimaciones fijas en este documento porque se
desactualizan cuando cambia el protocolo.
