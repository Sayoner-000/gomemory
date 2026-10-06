# Tasks: 036-compression-promise-gaps

**Input**: Design documents from `/specs/036-compression-promise-gaps/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Incluidos solo donde el spec/plan los exige (contrato del estado opencode en US2; contratos de ayuda US4 ya existen y verdes).

**Organization**: Tareas agrupadas por historia de usuario. US4 ya implementada y verificada (memoria [381]) — sin tareas, solo registro.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Línea base verde antes de tocar nada

- [X] T001 Verificar línea base: `go build ./...` y `go test ./tests/contract/ ./adapters/primary/cli/` en verde

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Cifras vigentes para documentar en US1 (research R4 con fecha 2026-10-06; revalidar por si el store cambió)

**⚠️ CRITICAL**: US1 no puede redactar cifras sin esta fase

- [X] T002 Reproducir mediciones de la auditoría (`mem pack savings`, `mem pack build --json` con/sin `--no-code-graph`, hook 60 elementos) y anotar cifras vigentes con fecha en `specs/036-compression-promise-gaps/research.md`

**Checkpoint**: Cifras confirmadas — US1/US2/US3 pueden empezar

---

## Phase 3: User Story 1 - Entender qué ahorro esperar de cada vía (Priority: P1) 🎯 MVP

**Goal**: Tabla por vía con cifras fechadas y reproducción en guía, manual y ayuda (alcance B)

**Independent Test**: quickstart Q1 — localizar "cuánto ahorra" y responder 3/3 (qué ahorra cada vía, cómo reproducirlo, cuándo el hook no actúa)

### Implementation for User Story 1

- [X] T003 [P] [US1] Tabla por vía (emitido/pack/hook, cifra+fecha+reproducción) en `INSTALLATION.md` (sección de ahorro)
- [X] T004 [P] [US1] Orden de magnitud + reproducción de `pack build` en `docs/MANUAL.md` (§10)
- [X] T005 [P] [US1] `mem usage` como ahorro por emisión y distinción vs `pack savings` en `docs/MANUAL.md` (§16)
- [X] T006 [US1] Casos intactos del hook + motivo en `docs/MANUAL.md` (sección de hooks, FR-002) (depends on T003–T005 por coherencia de cifras)

**Checkpoint**: US1 verificable solo con documentación (quickstart Q1)

---

## Phase 4: User Story 2 - Diagnóstico veraz del hook en OpenCode (Priority: P2)

**Goal**: `mem doctor` reporta el hook opencode como emisión best-effort, no como fallo (contrato en `contracts/doctor-opencode-hook.md`)

**Independent Test**: quickstart Q2 — `mem doctor` con plugin instalado, cero problemas del hook opencode

### Tests for User Story 2 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T007 [P] [US2] Aserción `best-effort` para `toolOutputHookStates(dir, true)["opencode"]` en `adapters/primary/cli/cmd_doctor_compression_codex_test.go` (falla en rojo antes de T008)

### Implementation for User Story 2

- [X] T008 [US2] Estado opencode best-effort en `adapters/primary/cli/cmd_doctor_compression.go:49` (depends on T007)
- [X] T009 [US2] Verificar con binario fresco (`go build -o /tmp/mem036 ./infrastructure/` + `mem doctor`) según quickstart Q2 (depends on T008)

**Checkpoint**: US2 funciona sin US1 ni US3 (cambio de 1 línea + test)

---

## Phase 5: User Story 3 - Límite de Codex documentado y congelado (Priority: P3)

**Goal**: Límite del runtime declarado con sus dos vías de ahorro; comportamiento sin cambios (FR-006)

**Independent Test**: quickstart Q4 — hook codex no emite (salida 0) y el manual declara el límite

### Implementation for User Story 3

- [X] T010 [P] [US3] Límite Codex + vías que sí aplican en `docs/MANUAL.md` (FR-005)
- [X] T011 [P] [US3] Verificar hook codex intacto con binario fresco (`mem hook tool-output codex` sin emisión, salida 0)

**Checkpoint**: US3 completa sin tocar código (cero riesgo de regresión)

---

## Phase 6: User Story 4 - Ayuda organizada por flujos (Priority: P2) ✅ HECHA

**Estado**: Implementada y verificada el 2026-10-06 (memoria [381]). Sin tareas.

- `adapters/primary/cli/cli.go`: `Usage()` en 8 flujos, `pack`/`seed`/`adr-sync` visibles, distinción savings-vs-usage, ejemplos pack.
- `tests/contract/mcp_tool_sync_test.go`: `TestUsage_MencionaPackYFlujos` en verde.
- Evidencia: `gofmt` limpio, `go build` OK, `tests/contract` y `adapters/primary/cli` verdes, `--help` verificado con binario fresco.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Validación cruzada final

- [X] T012 Ejecutar quickstart Q5 completo (`go test ./tests/contract/ ./adapters/primary/cli/`, `gofmt -l`, `--help` con binario fresco) (depends on US1–US3)
- [X] T013 [P] Revalidar `specs/036-compression-promise-gaps/checklists/requirements.md` sigue 16/16 tras los cambios

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS US1 (cifras)
- **User Stories**: US1 depende de Phase 2; US2 y US3 solo de Setup (archivos distintos: `cmd_doctor_compression*.go` vs `*.md`) → pueden ir en paralelo con US1 tras Setup si se acepta redactar US1 con cifras de la auditoría [378] pendientes de confirmar en T002
- **US4**: Hecha, sin dependencias
- **Polish (Phase 7)**: Depends on US1–US3 complete

### Within Each User Story

- US2: test T007 en rojo ANTES de T008
- US1: T003–T005 en paralelo, T006 al final por coherencia
- US3: T010 y T011 en paralelo (archivos/artefactos distintos)

### Parallel Opportunities

- T003, T004, T005 (secciones distintas de `INSTALLATION.md`/`MANUAL.md` — coordinar para no editar el mismo archivo a la vez: T003 toca otro archivo, T004/T005 el mismo `MANUAL.md` en secciones distintas; si un solo agente, secuencial)
- T007 con cualquier tarea de US1/US3 (archivos distintos)
- T010, T011, T013 en paralelo

---

## Parallel Example: arranque tras Setup

```bash
# Tras T001, lanzar en paralelo (archivos distintos):
Task: "T002 Reproducir mediciones y anotar cifras"
Task: "T007 Aserción best-effort (en rojo)"
Task: "T010 Límite Codex en MANUAL.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002 — cifras)
3. Complete Phase 3: User Story 1 (T003–T006)
4. **STOP and VALIDATE**: quickstart Q1
5. US4 ya entregada previamente como valor visible

### Incremental Delivery

1. Setup + Foundational → cifras listas
2. US1 (docs) → validar Q1
3. US2 (1 línea + test) → validar Q2
4. US3 (docs + verificación) → validar Q4
5. Polish Q5 → feature completa

---

## Notes

- [P] tasks = different files, no dependencies
- Cada tarea cita su archivo exacto; T002 usa los comandos de la auditoría [378]
- No modificar tests de preservación del hook (FR-007/SC-005)
- Commit tras cada fase o historia
