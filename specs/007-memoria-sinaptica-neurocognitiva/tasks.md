# Tasks: Memoria sináptica neurocognitiva

**Spec**: [spec.md](./spec.md) · **Plan**: [plan.md](./plan.md)

Leyenda: `[X]` hecho · `[ ]` pendiente

## Fase 1 — Captura de planes aprobados (FR-001, FR-002, FR-003)

- [X] T001 `hookPlanApproved` + case `plan-approved` en el switch de `CmdHook`. `adapters/primary/cli/cmd_hook.go`
- [X] T002 `extractPlanFromPayload` (acepta `tool_input.plan` y `plan` de nivel superior) + `planTitle` (título desde la 1ª línea sin marcadores markdown). `adapters/primary/cli/cmd_hook.go`
- [X] T003 Registrar `PostToolUse` matcher `ExitPlanMode` → `plan-approved` en `claudeHookEvents`. `adapters/primary/setup/claude_code_setup.go`
- [X] T004 Reconocer `hook plan-approved` en `hookCommandIsGomemory` (para setup idempotente y limpieza). `adapters/primary/setup/claude_code_setup.go`
- [X] T005 Añadir `PostToolUse` y `SubagentStop` (faltaba) a la lista de eventos a limpiar en uninstall. `adapters/primary/cli/cmd_uninstall.go`
- [X] T006 Registrar el hook `PostToolUse(ExitPlanMode)` en el `.claude/settings.json` de este repo.
- [X] T007 OpenCode: capturar en `handleTurnEnd` los mensajes con `info.mode==="plan"` y enviarlos a `mem hook plan-approved`. `infrastructure/plugin/opencode/gomemory.ts`

## Fase 2 — Consolidación sináptica (FR-004)

- [X] T008 `formSynapse(db, project, sessionID, newID)`: enlaza la memoria nueva con el ancla no-checkpoint más reciente de su sesión (`related`, 0.5), idempotente vía `GetRelationByPair`, best-effort. `adapters/secondary/persistence/memory.go`
- [X] T009 Invocar `formSynapse` tras el INSERT en `InsertMemory` sin romper la firma ni la provenance. `adapters/secondary/persistence/memory.go`

## Fase 3 — Visibilidad (FR-005)

- [X] T010 Clasificar relaciones en `conflicts` + `synapses` y emitir la sección **🔗 Sinapsis** en el contexto. `application/usecases/build_context.go`
- [X] T011 Helper `relTitle` para resolver el título de cada extremo (o marcador si quedó fuera de la ventana). `application/usecases/build_context.go`

## Fase 4 — Verificación (FR-006, SC-001..SC-003)

- [X] T012 `go build ./...` + `go test ./adapters/... ./application/...` verdes.
- [X] T013 Verificación end-to-end con binario real: plan Claude Code guarda `decision`; JSON malformado → no-op seguro.
- [X] T014 Verificación end-to-end en proyecto temporal aislado: cadena de sinapsis `decision→plan→bugfix` visible en `mem context`.

## Fase 5 — Documentación

- [X] T015 `docs/architecture.md`: subcomando `plan-approved`, fila `PostToolUse` en la tabla de hooks, `formSynapse` en el árbol de persistencia, sección Sinapsis en el contexto y bloque "Consolidación sináptica".
- [X] T016 `README.md`: características de captura de planes y consolidación sináptica.
- [X] T017 Este set de spec (`spec.md`, `plan.md`, `tasks.md`) en `specs/007-memoria-sinaptica-neurocognitiva/`.

## Pendiente (fases siguientes del circuito — fuera de esta entrega)

- [ ] Consolidación sistémica: barrido en `session-end` que refuerza engramas reactivados y poda sinapsis débiles.
- [ ] Reconsolidación al recordar (integrar/atenuar); apoyarse en `judge_memories`/`supersedes`.
- [ ] Neurogénesis: reestructuración periódica de memorias antiguas (re-resumen, re-enlace, decaimiento).
