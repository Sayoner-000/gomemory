# Implementation Plan: Integridad de los hooks, avisos por conversación y compresión que no destruye

**Branch**: `035-hook-integrity-compression` (sin rama git creada; se trabaja sobre `main`) | **Date**: 2026-10-04 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/035-hook-integrity-compression/spec.md`

## Summary

En Claude Code, los hooks de gomemory se degradan por tres causas medidas en vivo:

1. **Doble registro** usuario + proyecto: cada evento corre dos veces.
2. **Salidas de 12–18 mil caracteres** que el host trunca a una vista previa de 2 KB.
3. **Un hook de salidas que destruye** código, resultados y diagnósticos (reproducido cuatro veces durante esta misma planificación).

Además, el estado por turno vive por proyecto y no se reinicia por conversación (el «checkpoint pegado» en Codex), y la compresión casi no ahorra fuera de JSON.

Enfoque técnico:
- un **presupuesto duro por secciones** en dominio (`FitHookOutput`);
- **limpieza de ámbito** en `install`/`update`, más una guarda de reentrada `O_EXCL`;
- **modo «salida de herramienta»** en el motor de compresión: prosa intacta, arrays ≤ 50 completos, Go e indentación reconocidos, sin colapso de espacios;
- un **único `BeginConversation`** con identidad del host y rotación por inactividad;
- **escritura atómica** de los archivos de estado;
- un **compresor de listados**;
- **eventos de guarda** visibles en `mem doctor`.

Todo vive en el binario; los hooks instalados no cambian de forma, salvo la limpieza de duplicados y el campo `command` del plugin de OpenCode.

## Technical Context

**Language/Version**: Go ≥ 1.22 (stack congelado)

**Primary Dependencies**: stdlib, `modelcontextprotocol/go-sdk`, `modernc.org/sqlite`, `testify` (no se añade ninguna)

**Storage**: SQLite (tabla nueva `hook_guard_events`, migración idempotente) + archivos de estado en `.memory/`

**Testing**: `testing` + `testify`; `tests/unit`, `tests/integration` (binario real), `tests/contract` (fixtures reales); `-race` para la huella concurrente

**Target Platform**: macOS, Linux, Windows (binario autocontenido)

**Project Type**: CLI + servidor MCP stdio + hooks de agentes (Claude Code, Codex, OpenCode)

**Performance Goals**: hook de salidas < `domain.HookBudget` (150 ms), incluido el compresor de listados; guarda de reentrada < 2 ms

**Constraints**:
- salida inyectada ≤ 10 000 runas;
- ningún hook rompe el turno (salida neutra y código 0 ante cualquier fallo);
- la configuración del host no se escribe durante `session-start`.

**Scale/Scope**: unos 12 archivos de producción tocados, 3 nuevos (`conversation.go`, `hook_guard.go`, `native/listing.go`) más uno de dominio (`hook_budget.go`); 5 historias de usuario y 29 FR

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Evaluación | Estado |
|---|---|---|
| I. Hexagonal | `FitHookOutput`, `HookSection`, `Conversation`, `ContentListing` y la decisión de rotación son puros en `domain/`. Los puertos nuevos (`SessionActivityReader`, `HookGuardRecorder`) se declaran en `application/ports`. La persistencia, en `adapters/secondary/persistence`. Los hooks orquestan en `adapters/primary/cli`. El wiring, en el composition root existente. | ✅ |
| II. SQLite directo | Tabla nueva con `CREATE TABLE IF NOT EXISTS`, *upsert* con parámetros bind y transacción explícita. | ✅ |
| III. Testing First | Cada tarea empieza con un test rojo: fixtures reales de esta sesión, binario real en integración. El único test existente que contradecía una decisión aclarada se reescribió con autorización explícita (ver Complexity Tracking). Cobertura: se mide con `scripts/coverage.sh`; la brecha heredada de ≥ 80 % (memoria 305) no debe empeorar. | ✅ (autorizado) |
| IV. Configuración | Los umbrales nuevos (4 h, 2 000 tokens, 51 elementos, 15+5 líneas y purga de locks a 60 s) se declaran como constantes de política en `domain/compression_policy.go` y `domain/conversation_policy.go`. La ventana de 5 s del diseño inicial se retiró: un duplicado se reconoce por el lock del SO durante la ejecución. | ✅ (patrón existente) |
| V. Operativos | Simplicidad: reutiliza `filterOutGomemoryHooks`, `globalRemover`, `renderSessionStart(avisos)`, `checkLiteral`, `refOf`, `DeliveryLog`, `backupSessionSnapshot`. Causa raíz: se corrige el registro, el tope y la clasificación, sin parches en la salida. Fire-and-forget en los eventos de guarda. Idempotencia: la limpieza de ámbito y la migración son repetibles. | ✅ |
| Documentación en español | Todos los artefactos en español latino. | ✅ |
| Prohibiciones | Ninguna violada (sin ORM, sin SQL concatenado, sin config hardcodeada por entorno, sin caché de valores en caliente). | ✅ |

**Re-check post-diseño**: todas las puertas pasan; la excepción de la constitución III está autorizada.

## Project Structure

### Documentation (this feature)

```text
specs/035-hook-integrity-compression/
├── spec.md
├── plan.md              # este archivo
├── research.md          # R1–R10
├── data-model.md
├── quickstart.md        # validación contra el binario real
├── contracts/
│   └── hooks.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit-tasks (no lo crea este comando)
```

### Source Code (repository root)

```text
domain/
├── hook_budget.go              # NUEVO: HookSection, FitHookOutput, HookTrimNotice (puro)
├── conversation.go             # NUEVO: Conversation, IsNewConversation, ShouldRotate (puro)
├── conversation_policy.go      # NUEVO: StaleSessionSecs, HookReentryWindow
├── compression_policy.go       # ToolOutputMinTokens 400→2000, ToolOutputJSONMinItems=51
└── compression.go              # ContentListing

application/ports/
├── compressor.go               # CompressionOptions.ToolOutput
├── session_activity.go         # NUEVO: SessionActivityReader
└── hook_guard.go               # NUEVO: HookGuardRecorder

adapters/primary/cli/
├── conversation.go             # NUEVO: beginConversation, readConversation, writeFileAtomic
├── hook_guard.go               # NUEVO: guarda de reentrada O_EXCL + registro de eventos
├── cmd_hook.go                 # session-start, user-prompt-submit, turn-end, post-compact, subagent-*
├── nudge.go                    # computeSaveNudge desde started_at; gating del recordatorio de plan
├── footprint.go                # escritura atómica, aviso con conv_id, lectura que se autorrepara
├── context_output.go           # entrega con FitHookOutput
├── cmd_mcp.go / cmd_session.go # llaman a beginConversation
├── hook_tool_output.go         # exclusiones por comando, ToolOutput=true, nota de marcas una vez
├── cmd_doctor*.go              # duplicados, protecciones de 7 días, texto de Codex
├── cmd_install.go / cmd_update.go # limpieza del ámbito contrario
└── cmd_pack*.go                # columnas «sin ganancia» / «degradaciones»

adapters/primary/setup/
├── claude_code_setup.go        # retirar hooks gomemory del ámbito contrario (reutiliza filterOutGomemoryHooks)
├── codex_setup.go              # ídem para ~/.codex/config.toml / proyecto
├── opencode_setup.go           # limpieza de ámbito del plugin (global vs .opencode)
└── activation_inspect.go       # detectar duplicación (solo lectura) para doctor y session-start

infrastructure/plugin/opencode/
└── gomemory.ts                 # enviar command a tool-output y --conversation en session.created

adapters/secondary/compression/native/
├── router.go                   # patrones de Go, heurística de indentación, detección de listados
├── engine.go                   # modo ToolOutput: prosa intacta, sin limpieza estructural, JSONMinItems 51
└── listing.go                  # NUEVO: compresor de listados

adapters/secondary/persistence/
├── db.go                       # CREATE TABLE IF NOT EXISTS hook_guard_events
├── channel_activity.go         # RecordGuardEvent / GuardEventsSince
├── session.go                  # LastActivity(sessionID)
└── compression_stats.go        # no_gain fuera de fallbacks (columna/conteo propio)

tests/
├── contract/testdata/tool_output/   # NUEVOS fixtures: claude-bash-sed-go.json, claude-mcp-search30.json,
│                                    #   claude-bash-doctor.json, claude-grep-400.json (listado), go-test-v-long
├── contract/hook_budget_test.go     # tope sobre la memoria real de este repo (snapshot)
├── integration/conversation_hook_integration_test.go
├── integration/hook_reentry_integration_test.go
└── unit (paquetes)                  # FitHookOutput, ShouldRotate, listing, router, footprint -race
```

**Structure Decision**: proyecto único y hexagonal, ya vigente. Las piezas puras van a `domain/`; la orquestación de hooks queda en `adapters/primary/cli`, donde ya vive. No se crean paquetes nuevos.

## Diseño por historia (orden de implementación)

1. **US1 — Integridad de hooks (P1)**
   1. `FitHookOutput` (dominio) y su uso en session-start, el primer prompt, post-compact y subagent-start. Se anota como entregado solo lo emitido, y el plan-doc no repite la memoria (DeliveryLog).
   2. Guarda de reentrada (`hook_guard.go`) al inicio de cada handler de evento, **antes** de leer stdin para efectos (lee stdin una vez y calcula el hash).
   3. Detección de duplicación (lectura) → aviso en `systemMessage` de session-start + ⚠ en doctor.
   4. Limpieza del ámbito contrario en `install` y `update`, para Claude, Codex y OpenCode.
   5. Gating de recordatorios: plan (solo claude) y Octopus (por cambio de estado).
2. **US2 — Compresión segura (P1)**: `ToolOutput` en las opciones; router con Go e indentación; engine sin limpieza estructural ni prosa en ese modo; arrays ≤ 50 completos; exclusiones por comando; umbral de 2 000 tokens; nota de marcas una vez por conversación.
3. **US3 — Conversación (P2)**: `writeFileAtomic`, `.conversation`, `beginConversation` (llamado desde session-start, `mem session start` y `CmdMCP`), aviso con `conv_id` consumido también en el primer prompt, `computeSaveNudge` desde `started_at`, rotación con `SessionActivityReader`.
4. **US4 — Listados y métrica (P3)**: `ContentListing` + `listing.go`; `no_gain` contado aparte; columnas en `mem pack savings`.
5. **US5 — Codex/OpenCode (P3)**: línea de protocolo para Codex, texto de doctor, `command` en el plugin.
6. **Observabilidad (FR-028/029)**: tabla, puerto y registro en los tres puntos (reentrada, recorte, exclusión); sección de doctor.

Cada historia termina con la validación de su bloque en [quickstart.md](quickstart.md) sobre el binario instalado (regla 1 del proyecto) y un commit verificable, previa confirmación de la persona.

## Riesgos y cierre

| Riesgo | Cierre |
|---|---|
| La limpieza de ámbito borra hooks ajenos | Se filtra solo con `IsGomemoryHookEntry`/`hookCommandIsGomemory` (ya probados en la 034); test con cbm-* y herdr presentes |
| La guarda de reentrada descarta un evento legítimo | Solo descarta si el dueño del bloqueo sigue vivo (concurrencia real); test con dos invocaciones secuenciales idénticas, que deben procesarse ambas |
| El recorte se come instrucciones críticas | Las secciones no recortables tienen un tamaño acotado por test (≤ 4 000 runas) |
| Detección de listados con falsos positivos | Las líneas de error siempre se conservan; ref recuperable; `checkLiteral` |
| Agentes simultáneos y un solo `.conversation` | Aceptado (spec, Assumptions); la rotación es solo por inactividad |
| T057 (contrato de salidas) deja de reducir algún fixture | Medido en research R5: los tres siguen siendo comprimibles; se ejecuta en la primera tarea de la US2 |

## Complexity Tracking

| Violación | Por qué se necesita | Alternativa más simple rechazada porque |
|---|---|---|
| **Modificar un test existente** (AUTORIZADO por la persona el 2026-10-04; reescrito como `TestHookOctopus_ActivarAMitadDeSesionLlegaEnElTurnoSiguiente`, hoy en rojo según TDD): `tests/integration/hook_marker_integration_test.go::TestHookOctopusEncendido_SiguePresenteEnTurnosPosteriores` exige la regla Octopus en el 2.º turno sin cambio de estado (constitución III: requiere **autorización explícita**) | FR-008a (aclaración Q2) decide que Octopus se emita solo al inicio, tras compactar o al cambiar su activación. El test se reescribiría para cubrir su objetivo original (ACR 029 C-002: activar Octopus a mitad de sesión llega al agente en el turno siguiente) | Mantener Octopus en cada turno contradice la decisión de la persona y deja el ruido que la US1 elimina |
| Tabla nueva `hook_guard_events` en vez de usar solo `channel_activity` | `channel_activity` guarda una sola marca por canal y no puede contar eventos de 7 días (FR-029) | Añadir columnas de conteo a `channel_activity` mezclaría la semántica «último disparo» con «conteo diario» y obligaría a migrar su clave |
