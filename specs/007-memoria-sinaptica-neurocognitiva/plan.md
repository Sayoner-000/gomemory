# Implementation Plan: Memoria sináptica neurocognitiva

**Spec**: [spec.md](./spec.md)

## Enfoque

Dos capacidades, ambas **deterministas, sin tokens del agente y transversales** por vivir por debajo de la capa de agente (persistencia + context builder + entrypoint de hooks compartidos). La visión rectora es el modelo neurocognitivo de 4 etapas; esta entrega es la Etapa 1.

### 1. Captura de planes aprobados (`plan-approved`)

- La lógica vive en Go, en el entrypoint `mem hook`. `extractPlanFromPayload` acepta las dos formas de payload (`tool_input.plan` de Claude Code y `plan` de nivel superior genérico), igual que `mem hook prompt`/`nudge` ya hacen para OpenCode.
- Guarda una memoria `type=decision` vía `MemoryRepo.Insert`; el prompt originante se adjunta solo en `InsertMemory` (choke point de provenance).
- **Claude Code**: se registra como `PostToolUse` con matcher `ExitPlanMode`. `PostToolUse` solo dispara si el usuario aprobó → solo se capturan planes aceptados. Append-only: cada aprobación genera una `decision` nueva.
- **OpenCode**: el plugin detecta en `handleTurnEnd` los mensajes del asistente con `info.mode==="plan"` y manda su texto por stdin a `mem hook plan-approved`.

### 2. Consolidación sináptica (`formSynapse`)

- **Punto de inserción**: `InsertMemory` (`adapters/secondary/persistence/memory.go`). Es el choke point único por el que pasan las 7 vías de guardado (MCP, hooks, CLI, TUI, OpenCode) y donde ya se hereda la provenance. Poner aquí la sinapsis la hace transversal sin cablear cada agente — mismo precedente que la provenance.
- **Criterio**: arista `related` (confidence 0.5) de la memoria nueva hacia el **ancla** = la memoria no-checkpoint más reciente de la misma sesión (`SELECT id … type<>'checkpoint' ORDER BY id DESC LIMIT 1`). Teje el hilo de decisiones y enlaza cada checkpoint con la decisión que lo gobierna, sin ruido checkpoint↔checkpoint.
- **Idempotencia**: `GetRelationByPair` antes de insertar.
- **Best-effort**: cualquier error se traga; nunca hace fallar el guardado.

### 3. Visibilidad

- `build_context` clasifica las relaciones en `conflicts` (ya existía) y `synapses` (`related`/`supersedes`, nuevo) y emite la sección **🔗 Sinapsis** con títulos resueltos (`relTitle`).

## Decisiones de diseño

- **Append-only, sin deduplicar planes**: cada aprobación (incluidos planes revisados) se conserva, para no perder la evolución de las decisiones ("loop"). La deduplicación de contenido queda para la consolidación sistémica.
- **Sinapsis en persistencia, no en un usecase nuevo**: se sigue el precedente de la provenance (política en el choke point) por transversalidad y mínima superficie de cambio, en vez de enrutar los 7 call sites por un usecase envolvente.
- **`SubagentStop` añadido a la limpieza de uninstall**: bug preexistente detectado de paso (la lista de eventos a limpiar no lo incluía); se corrige junto a `PostToolUse`.

## Archivos afectados

- `adapters/primary/cli/cmd_hook.go` — `hookPlanApproved`, `extractPlanFromPayload`, `planTitle`, case `plan-approved`.
- `adapters/secondary/persistence/memory.go` — `formSynapse` + wiring en `InsertMemory`.
- `application/usecases/build_context.go` — sección Sinapsis + `relTitle`.
- `adapters/primary/setup/claude_code_setup.go` — registro `PostToolUse(ExitPlanMode)`, reconocimiento en `hookCommandIsGomemory`.
- `adapters/primary/cli/cmd_uninstall.go` — limpieza de `PostToolUse` y `SubagentStop`.
- `infrastructure/plugin/opencode/gomemory.ts` — captura de turnos en modo `plan`.
- `.claude/settings.json` — hook `PostToolUse` para este propio repo.

## Verificación

- `go build ./...` + `go test ./adapters/... ./application/...` verdes.
- Binario real: payload Claude Code (`tool_input.plan`) guarda `decision` con título derivado; JSON malformado → no-op (exit 0).
- Proyecto temporal aislado: `decision → plan-approved → bugfix` en una sesión forma la cadena de sinapsis, visible en `mem context`.
