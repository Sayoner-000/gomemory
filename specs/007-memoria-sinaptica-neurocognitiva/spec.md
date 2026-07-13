# Feature Specification: Memoria sináptica neurocognitiva (captura de planes + auto-enlace)

**Feature Branch**: `007-memoria-sinaptica-neurocognitiva`

**Created**: 2026-07-13

**Status**: Implemented (Etapa 1)

**Input**: User description: "La memoria no se accionaba automáticamente al aprobar un plan: el arranque inyectaba contexto, pero al pasar un plan (p. ej. `/plan`) no se guardaba ninguna de las decisiones que se deben accionar en el flujo de trabajo. Además, el hook debe poder acumular decisiones dinámicamente (loop) para que no se olviden y no haya que empezar de cero, y esto debe aplicar a todo el ciclo de vida de la memoria, en cada decisión y turno — memoria dinámica para varios agentes: «siempre recuerda, siempre sinapsis». El norte de diseño es un modelo neurocognitivo: codificación (sinapsis al codificar), consolidación sináptica (minutos–horas) y sistémica (días–años), reconsolidación y neurogénesis."

## Contexto y motivación

El sistema tenía tres mecanismos de captura y **ninguno cubría un turno de planificación**:

1. `SessionStart` → solo inyecta contexto (funciona: "lo hizo al iniciar").
2. `UserPromptSubmit` (nudge) → recordatorio blando al modelo, solo tras >15 min sin guardar y con debounce; en un flujo de plan rápido no dispara, y aun disparando depende de que el modelo llame `save_memory`.
3. `Stop`/`turn-end` → única captura determinista, pero **descarta turnos sin ediciones ni comandos** (`activity.empty()`).

Un turno de **plan mode** es puro chat (el modelo escribe el plan y llama `ExitPlanMode`, sin `Edit`/`Write`/`Bash`), así que `turn-end` lo ignora y las decisiones del plan se perdían. No existía señal determinista para "se aprobó un plan".

Como visión de fondo, el usuario define un **modelo neurocognitivo de memoria en 4 etapas** (ver `docs/architecture.md` y memoria de arquitectura del proyecto). Esta feature implementa la **Etapa 1 (codificación + consolidación sináptica)**.

**Restricción transversal (obligatoria)**: gomemory es multiagente. Toda mejora debe implementarse una sola vez (lógica en Go) y activarse en todos los agentes, no atarse a Claude Code.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Las decisiones de un plan aprobado se guardan solas (Priority: P1)

Como usuario que aprueba un plan, quiero que sus decisiones queden en memoria automáticamente, para que no se pierdan ni haya que empezar de cero en la próxima sesión.

**Independent Test**: Aprobar un plan y comprobar que aparece una memoria `decision` con su contenido, sin que el usuario ni el modelo la guarden a mano.

**Acceptance Scenarios**:

1. **Given** un turno de plan mode en Claude Code, **When** el usuario aprueba el plan, **Then** el hook `PostToolUse(ExitPlanMode)` guarda una memoria `type=decision` con el plan como contenido y el prompt originante adjunto.
2. **Given** un plan revisado y vuelto a aprobar, **When** se aprueba de nuevo, **Then** se crea **otra** memoria `decision` (append-only); no se sobrescribe la anterior, preservando la evolución.
3. **Given** un plan rechazado, **When** no se ejecuta la tool, **Then** no se guarda nada (PostToolUse no dispara).

### User Story 2 - Las memorias se enlazan solas (sinapsis) (Priority: P1)

Como agente que retoma un proyecto, quiero que las memorias estén enlazadas entre sí, para navegar el hilo de decisiones y su implementación sin reconstruirlo a mano.

**Independent Test**: Guardar varias memorias en una sesión y comprobar que `get_context` muestra una sección de sinapsis que las encadena.

**Acceptance Scenarios**:

1. **Given** una sesión con una `decision` ya guardada, **When** se guarda otra memoria (decision/bugfix/checkpoint…), **Then** se crea una arista `related` de la nueva hacia el engrama sustantivo (no checkpoint) más reciente de la sesión.
2. **Given** varias sinapsis creadas, **When** se pide `get_context`, **Then** aparece la sección **🔗 Sinapsis** listando los pares enlazados con sus títulos.
3. **Given** dos memorias ya enlazadas, **When** se reintenta el enlace, **Then** no se duplica la arista (idempotencia).

### User Story 3 - Transversalidad multiagente (Priority: P1)

**Acceptance Scenarios**:

1. **Given** OpenCode, **When** un turno corre en modo `plan`, **Then** el plugin invoca `mem hook plan-approved` con `{"plan":"…"}` y se guarda la `decision`.
2. **Given** cualquier vía de guardado (MCP `save_memory`, hook, CLI, TUI), **When** inserta una memoria, **Then** la sinapsis se forma igual, porque la lógica vive en el choke point `InsertMemory`.

## Edge Cases

- Payload malformado o sin plan → no-op seguro (exit 0), nunca rompe el turno.
- Memoria sin sesión activa → no se forma sinapsis (no hay co-activación que enlazar).
- Sesión con solo checkpoints previos → la nueva memoria no encuentra ancla sustantiva → no enlaza (evita ruido checkpoint↔checkpoint).

## Requirements *(mandatory)*

- **FR-001**: Un hook determinista DEBE guardar el plan aprobado como `decision` sin depender del modelo.
- **FR-002**: La captura DEBE ser append-only (cada aprobación acumula, no sobrescribe).
- **FR-003**: La captura DEBE ser transversal: aceptar `tool_input.plan` (Claude Code) y `plan` de nivel superior (OpenCode/otros).
- **FR-004**: Cada `InsertMemory` DEBE formar una sinapsis determinista hacia el ancla sustantiva de la sesión, idempotente y best-effort.
- **FR-005**: `get_context` DEBE exponer el grafo de sinapsis (`related`/`supersedes`), no solo los conflictos.
- **FR-006**: Ningún hook ni la formación de sinapsis pueden hacer fallar el guardado ni el turno del agente.

## Success Criteria *(mandatory)*

- **SC-001**: Aprobar un plan en Claude Code produce una memoria `decision` con el plan, sin intervención manual.
- **SC-002**: Guardar N memorias en una sesión produce una cadena de N-1 sinapsis visible en `get_context`.
- **SC-003**: El mismo comportamiento se observa desde Claude Code y OpenCode.

## Alcance

**Implementado (Etapa 1 — codificación + consolidación sináptica):**
- Hook `plan-approved` (PostToolUse `ExitPlanMode` en Claude Code; `info.mode==="plan"` en el plugin OpenCode). — FR-001..FR-003
- `formSynapse()` en el choke point `InsertMemory`. — FR-004
- Sección **🔗 Sinapsis** en `build_context`. — FR-005

**Pendiente (fases siguientes del circuito neurocognitivo):**
- **Consolidación sistémica** (días–años): barrido en `session-end` que refuerza engramas reactivados (sube confidence) y poda sinapsis débiles, desplazando de "hipocampo" (checkpoints recientes) a "corteza" (decisions/architecture).
- **Reconsolidación**: integrar/atenuar al recordar (parcialmente cubierto por `judge_memories` + `supersedes`/`conflicts_with`).
- **Neurogénesis**: reestructuración periódica de memorias antiguas (re-resumen, re-enlace, decaimiento).
