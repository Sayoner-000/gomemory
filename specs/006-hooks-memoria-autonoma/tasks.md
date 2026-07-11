# Tasks: Memoria autónoma transversal

**Spec**: [spec.md](./spec.md) · **Plan**: [plan.md](./plan.md)

Leyenda: `[X]` hecho · `[ ]` pendiente

## Fase 1 — Fix de carga de tools diferidas (FR-001)

- [X] T001 Eliminar `{"tools": true}` de `hookUserPromptSubmit` (no-op en Claude Code) y emitir `systemMessage` con `ToolSearch select:` + nombres reales de las tools de gomemory. `adapters/primary/cli/cmd_hook.go`
- [X] T002 Añadir constante `memoryToolBootstrap` con la instrucción de ToolSearch y documentar por qué es Claude-Code-only. `adapters/primary/cli/cmd_hook.go`
- [X] T003 Migrar el recordatorio de protocolo a `hookSpecificOutput.additionalContext` (forma documentada). `adapters/primary/cli/cmd_hook.go`
- [X] T004 Verificación end-to-end contra binario real: primer prompt trae systemMessage+ToolSearch y additionalContext; segundo pasivo.

## Fase 2 — Recordatorio de guardado (FR-002, FR-003)

- [X] T005 `SecondsSinceLastSave(project)` en persistencia: segundos desde la última memoria `type != 'checkpoint'`, o inexistencia. `adapters/secondary/persistence/memory.go`
- [X] T006 Exponer el método en el puerto `MemoryRepository` e implementarlo. `application/ports/memory_repository.go`, `adapters/secondary/persistence/repositories.go`
- [X] T007 `computeSaveNudge` + `ageSeconds` + umbrales + texto + marcador de debounce `.memory/.last-nudge`. `adapters/primary/cli/nudge.go`
- [X] T008 Test unitario `TestSecondsSinceLastSave` (checkpoints excluidos, caso vacío, edad, checkpoint no reinicia reloj). `adapters/secondary/persistence/memory_test.go`

## Fase 3 — Cableado transversal (FR-004, FR-005)

- [X] T009 Branch de prompt subsiguiente de `hookUserPromptSubmit`: emite `{"systemMessage": <nudge>}` cuando corresponde, `{}` si no. `adapters/primary/cli/cmd_hook.go`
- [X] T010 Evento `mem hook nudge` + `hookNudge` (salida en texto plano) para integraciones que no consumen el JSON de Claude Code. `adapters/primary/cli/cmd_hook.go`
- [X] T011 Plugin OpenCode: invocar `mem hook nudge` en `experimental.chat.system.transform` y empujar al system prompt si no está vacío. `infrastructure/plugin/opencode/gomemory.ts`
- [X] T012 Verificación end-to-end del nudge con timestamps retrasados en SQLite (dispara / debounce / guardado reciente / checkpoint no cuenta) en las vías `mem hook nudge` y Claude `user-prompt-submit`.

## Fase 4 — Documentación y release

- [X] T013 Actualizar documentación (protocolo de memoria, arquitectura) con el bootstrap de tools y el recordatorio transversal.
- [X] T014 Bump de versión a v1.13.0. `version/version.go`
- [ ] T015 Commit + tag + release.

## Pendiente para iteraciones futuras (fuera de alcance de v1.13.0)

- [X] T016 Captura pasiva de subagentes: hook `SubagentStop` → checkpoint desde el transcript del subagente (`recordActivityCheckpoint` reutilizado por `turn-end` y `subagent-stop`). Registrado en el instalador (`claudeHookEvents`). v1.14.0. (feat #3)
- [X] T017 Re-inyección post-compactación: `SessionStart` matcher `compact` → `post-compact` (re-inyecta recuperación + contexto y borra el marcador para re-materializar las tools diferidas). Reemplaza a `PreCompact`. Transversal: OpenCode reusa el mismo texto vía `experimental.session.compacting` → `mem hook post-compact`. v1.15.0. (feat #4)
- [X] T018 Persistir el prompt originante junto al guardado: `sessions.last_prompt` (capturado por turno) + `memories.origin_prompt` (backfill en `InsertMemory`, choke point único). Captura transversal: Claude Code inline en `user-prompt-submit`, OpenCode vía `chat.message` → `mem hook prompt`; agentes sin hook de mensaje degradan a `origin_prompt` vacío. v1.16.0. (feat #5)
