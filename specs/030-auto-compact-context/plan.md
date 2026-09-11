# Implementation Plan: Compactación de contexto sin pérdida de memoria

**Branch**: `030-auto-compact-context` | **Date**: 2026-09-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/030-auto-compact-context/spec.md`

## Summary

La compactación la hace siempre el cliente. gomemory se engancha alrededor de
ella para que no se pierda memoria:

1. **US1**: entrega un contexto acotado a la sesión activa. Lo recibe el
   compresor cuando el cliente lo permite (C1) y el agente al reanudar (C2).
   El contexto de proyecto posterior pasa a modo índice, con tope ≤ presupuesto
   de arranque.
2. **US2**: guarda el resumen compactado en `sessions.summary` **sin cerrar la
   sesión**. Lo hace el hook directamente cuando el cliente entrega el resumen
   (C6); si no, por orden de texto al agente mediante la nueva herramienta MCP
   `save_session_summary`.
3. **US3**: extrae la sección de aprendizajes del mensaje final de cada
   subagente (C3) y la guarda sin duplicados (upsert por `topic_key`).
4. **US4**: aviso opt-in al agente al superar el umbral, por C4, reutilizando
   la decisión vigente de `computeCompactNudge`. Nunca bloquea ni prolonga
   turnos.

Las capacidades por cliente (C1–C6) se declaran en `domain.KnownAgents`, con
motivo obligatorio para las ausentes. El diseño cierra además un defecto
latente que esta feature activaría: la recuperación vigente pide `end_session`,
que cierra la sesión a mitad de conversación (research.md R4).

## Technical Context

**Language/Version**: Go 1.27 (módulo actual); TypeScript para el plugin de opencode

**Primary Dependencies**: `modelcontextprotocol/go-sdk` (herramienta MCP nueva), `@opencode-ai/plugin` 1.18.x (hooks del plugin); nada nuevo

**Storage**: SQLite (`modernc.org/sqlite`). Sin migraciones: se usan `sessions.summary`, `sessions.last_prompt`, `memories.session_id` y `memories.topic_key`, que ya existen

**Testing**: `testing` + `testify`; `tests/unit`, `tests/integration` (BD real), `tests/contract` (invariantes y textos); prueba del plugin con el runner vigente de `infrastructure/plugin/opencode`

**Target Platform**: binario `mem` autocontenido (macOS, Linux, Windows) invocado como hook por claude 2.1.268, codex 0.154.0 y opencode 1.18.30

**Project Type**: CLI + servidor MCP (stdio), arquitectura hexagonal

**Performance Goals**: cada hook termina en < 200 ms con un store de 1.000 memorias; `post-compact` y `compaction-context` hacen una sola consulta por sesión

**Constraints**: best-effort (código de salida 0 siempre); texto posterior a la compactación ≤ `Settings.Budget`; contexto de compactación ≤ 40 % de ese presupuesto; salida de `turn-end` idéntica con la opción apagada; 0 nombres de agentes, clientes o comandos de cliente en los textos

**Scale/Scope**: 3 clientes, 6 capacidades, 4 historias; ~12 archivos Go tocados, 1 archivo TS, 0 migraciones

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Evaluación | Estado |
|-----------|------------|--------|
| I. Hexagonal | Capacidades, textos y `ExtractLearnings` en `domain` (puros, sin I/O). Puertos ampliados (`ListBySession`, `UpdateSummary`). Casos de uso `BuildCompactionContext` y `CaptureLearnings` en `application`. Hooks, MCP y plugin en adaptadores. `CompactContextBuilder` se cablea en el composition root, sin aserciones de tipo en el adaptador. | ✅ |
| II. SQLite directo | Dos consultas nuevas con parámetros bind. Sin ORM ni migraciones. | ✅ |
| III. Testing first | Cada tarea de implementación va precedida de su test en rojo. **Ningún test existente se modifica**: C1 no se registra en claude (R12); `SubagentStop` se añade al final de la tabla de codex (R11); los puertos nuevos son interfaces estrechas para no romper los dobles de prueba de `MemoryRepository` y `SessionRepository`; los tests nuevos van en archivos nuevos. | ✅ |
| IV. Configuración | `CompactAgentNotice` va en la struct única `Settings`, con su default en `DefaultSettings`, sin lógica. Los topes derivan de `Settings.Budget`, sin valores de entorno nuevos. | ✅ |
| V.1 Simplicidad | Se reutiliza `sessions.summary`, `IndexMode`, el upsert por tópico, la redacción de `<private>` y `computeCompactNudge`. | ✅ |
| V.2 Causa raíz | El defecto de `end_session` en la recuperación se corrige en su origen (texto y apertura de sesión), no se tapa. | ✅ |
| V.6 Fire-and-forget | Captura pasiva y persistencia del resumen nunca bloquean el hook. | ✅ |
| V.7 Idempotencia | `UpdateSummary` con el mismo texto, captura repetida (por tópico) y consumo de marcas de un solo uso. | ✅ |
| Documentación en español | Artefactos de la spec y comentarios de código en español; identificadores en inglés (convención memoria 132). | ✅ |

**Re-check tras Phase 1**: sin violaciones. Complexity Tracking queda vacío.

## Project Structure

### Documentation (this feature)

```text
specs/030-auto-compact-context/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── hooks.md
│   └── texts.md
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
domain/
├── agents.go                      # + CompactionCapability, campos Compaction/CompactionUnavailable, valores por agente
├── compaction_texts.go            # nuevo: RecoverySteps, CompactorPersistOrder, AgentPrepareNotice, encabezados
└── learnings.go                   # nuevo: ExtractLearnings (puro)

application/
├── ports/session_memory.go        # nuevo: SessionMemoryLister (puerto estrecho; no se amplía MemoryRepository)
├── ports/session_summary.go       # nuevo: SessionSummaryUpdater (puerto estrecho; no se amplía SessionRepository)
└── usecases/
    ├── build_compaction_context.go  # nuevo
    └── capture_learnings.go         # nuevo

adapters/secondary/persistence/
├── memory.go                      # + ListBySession (SQL bind)
├── session.go                     # + UpdateSessionSummary
├── repositories.go                # + métodos de repositorio
└── settings.go                    # + CompactAgentNotice

adapters/primary/cli/
├── cmd_hook.go                    # post-compact ampliado; compaction-context, compact-summary, agent-notice nuevos; subagent-stop con captura; turn-end y user-prompt-submit con aviso
├── cmd_mcp.go                     # + save_session_summary
└── deps.go                        # + CompactContextBuilder

adapters/primary/setup/
├── claude_code_setup.go           # + PostCompact → compact-summary; hookCommandIsGomemory
└── codex_setup.go                 # + SubagentStop (al final de la tabla)

adapters/primary/tui/tui.go        # + fila «Aviso de compactación al agente»
infrastructure/container.go        # wiring de CompactContextBuilder
infrastructure/plugin/opencode/gomemory.ts  # session.compacting → compaction-context; session.compacted (C2/C6); tool.execute.after(task) (C3); agent-notice en system.transform (C4)

tests/
├── unit/        # ExtractLearnings, BuildCompactionContext, textos, decisión del aviso
├── integration/ # ListBySession, UpdateSummary, hooks con BD real
└── contract/    # INV-C1 por agente, términos prohibidos en textos
```

**Structure Decision**: proyecto único con las capas vigentes. No se crean
paquetes nuevos; los archivos nuevos siguen la convención `snake_case`.

## Árbol de tareas atómicas

Descomposición de alto nivel. `/speckit-tasks` la baja a tareas con TDD
explícito (test en rojo → implementación → verificación).

```text
🎯 Ninguna compactación, en ningún cliente, pierde la memoria de la sesión (quickstart Q0–Q7 en verde)
├─ [1] Confirmar capacidades en vivo
│  └─ [1.1] ✓ Ejecutar quickstart Q0 en los 3 clientes → tabla R1 confirmada o corregida
├─ [2] Base de dominio                                                     (dep: 1.1)
│  ├─ [2.1] ✓ Declarar CompactionCapability y C1–C6 por agente → test de contrato INV-C1 en verde
│  ├─ [2.2] ✓ Escribir los textos neutrales en domain → test de términos prohibidos en verde        ∥
│  └─ [2.3] ✓ Implementar ExtractLearnings → tests de tabla (ES/EN, cortos, sin sección) en verde   ∥
├─ [3] Persistencia                                                        (dep: 2)
│  ├─ [3.1] ✓ Añadir ListBySession (puerto + SQL) → test de integración que excluye otras sesiones
│  └─ [3.2] ✓ Añadir UpdateSummary (puerto + SQL) → test: no toca ended_at; idempotente              ∥
├─ [4] US1: contexto de compactación                                       (dep: 3.1, 3.2)
│  ├─ [4.1] ✓ Implementar BuildCompactionContext → tests de orden, tope 40 %, nota de omitidas, sesión vacía
│  ├─ [4.2] ✓ Cablear CompactContextBuilder (IndexMode) en container → test de wiring
│  ├─ [4.3] ✓ Ampliar post-compact (abre sesión si falta; recuperación + sesión + índice; tope) → test de hook    (dep: 4.1, 4.2)
│  └─ [4.4] ✓ Crear `mem hook compaction-context` → test de salida                                (dep: 4.1)
├─ [5] US2: resumen persistido                                             (dep: 3.2)
│  ├─ [5.1] ✓ Crear `mem hook compact-summary` → test: persiste, idempotente, abre sesión si falta
│  ├─ [5.2] ✓ Crear herramienta MCP save_session_summary + auto-aprobable → test de contrato MCP    ∥
│  └─ [5.3] ✓ Registrar PostCompact en claude y reconocerlo al desinstalar → test de instalación    (dep: 5.1)
├─ [6] US3: captura pasiva                                                 (dep: 2.3)
│  ├─ [6.1] ✓ Implementar CaptureLearnings (topic_key passive:, tope 10) → test: N ítems → N memorias; repetir → 0
│  ├─ [6.2] ✓ Ampliar subagent-stop (payload leído una vez; captura + checkpoint; `{}` en json) → test de hook   (dep: 6.1)
│  └─ [6.3] ✓ Añadir SubagentStop al final de codexGomemoryHooks → tests existentes intactos        (dep: 6.2)
├─ [7] US4: aviso al agente                                                (dep: 2.2)
│  ├─ [7.1] ✓ Añadir CompactAgentNotice a Settings + roundtrip → test de settings
│  ├─ [7.2] ✓ Emitir aviso en turn-end por dialecto (sin decision) → test: opción apagada = salida idéntica   (dep: 7.1)
│  ├─ [7.3] ✓ Consumir .pending-agent-notice en user-prompt-submit y `mem hook agent-notice` → test de un solo uso   (dep: 7.2)
│  └─ [7.4] ✓ Añadir fila en TUI Configuración junto al umbral → test de TUI                        (dep: 7.1)
├─ [8] Integración opencode                                                (dep: 4.4, 5.1, 6.2, 7.3)
│  └─ [8.1] ✓ Actualizar gomemory.ts (compacting, compacted, task, agent-notice) → test del plugin en verde
├─ [9] Diagnóstico
│  └─ [9.1] ✓ Mostrar C1–C6 y motivos por agente en `mem doctor` → test de salida                   (dep: 2.1)
├─ [10] Documentación
│  └─ [10.1] ✓ Actualizar MANUAL, CHANGELOG y referencia cruzada en spec 008 FR-008 → docs revisadas   (dep: 4–9)
└─ [11] Verificación
   ├─ [11.1] ✓ Ejecutar quickstart Q1–Q7 con el binario instalado en los 3 clientes → evidencia registrada   (dep: 10.1)
   └─ [11.2] ✓ Ejecutar ACR del cambio completo → veredicto APPROVED o hallazgos cerrados              (dep: 11.1)
```

25 hojas, justo en el umbral del método. La priorización la dan las historias
(P1 → [4], P2 → [5], P3 → [6], P4 → [7]), y cada rama es entregable por
separado una vez completadas [2] y [3].

Notas para `/speckit-tasks`:
- [8.1] se prueba con un test de contrato en `tests/contract/opencode_hooks_test.go`,
  que lee `gomemory.ts` y comprueba los hooks contra la superficie de tipos
  instalada, igual que los tests vigentes de ese archivo.
- [8.1] corrige también un orden defectuoso: hoy el plugin llama a
  `post-compact` dentro de `experimental.session.compacting`, es decir, **antes**
  de compactar, y reinicia la huella y los marcadores antes de tiempo. Con este
  plan, `session.compacting` pide `compaction-context` y `session.compacted`
  llama a `post-compact`.

## Complexity Tracking

Sin violaciones de la constitución que justificar.
