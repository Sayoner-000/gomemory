---

description: "Task list for: Mantenimiento de Memoria (Purga, Compactación y Garbage Collector)"

---

# Tasks: Mantenimiento de Memoria (Purga, Compactación y Garbage Collector)

**Input**: Design documents from `/specs/003-memory-maintenance/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/cli-tui-contracts.md](./contracts/cli-tui-contracts.md), [quickstart.md](./quickstart.md)

**Tests**: OBLIGATORIOS — el Principio III de la constitución ("Testing First — NO NEGOCIABLE") exige TDD estricto: cada tarea de test se escribe y debe **fallar** antes de implementar la tarea correspondiente.

**Organization**: Tareas agrupadas por historia de usuario (P1–P4 de `spec.md`) para que cada una sea implementable y verificable de forma independiente.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Puede ejecutarse en paralelo (archivo distinto, sin dependencia de tareas incompletas)
- **[Story]**: Historia de usuario a la que pertenece (US1–US4)
- Cada tarea incluye la ruta exacta de archivo

## Path Conventions

Proyecto único Go (CLI + TUI), arquitectura hexagonal existente — rutas reales del repo:

- `application/ports/` — puertos
- `adapters/secondary/persistence/` — adaptador SQLite
- `adapters/primary/cli/` — comandos CLI
- `adapters/primary/tui/` — TUI (bubbletea)
- `infrastructure/` — composition root
- `tests/integration/`, `tests/contract/` — tests con SQLite real / contratos CLI

---

## Phase 1: Setup

**Purpose**: Preparar el contrato (puerto) y la dependencia de formateo antes de tocar cualquier capa de implementación.

- [X] T001 Promover `github.com/dustin/go-humanize` (ya indirecta en `go.mod`) a dependencia directa ejecutando `go get github.com/dustin/go-humanize` desde la raíz del repo; verificar que `go.mod`/`go.sum` reflejan el cambio
- [X] T002 [P] Crear `application/ports/maintenance_repository.go` con los tipos `StorageStats`, `PurgeFilter` y la interfaz `MaintenanceRepository` (`Stats`, `Purge`, `Compact`) exactamente como se definen en [data-model.md](./data-model.md#puerto-maintenancerepository)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Infraestructura compartida por las 4 historias — adaptador base, wiring en el composition root y el primer método (`Stats`) que todas las historias necesitan para reportar antes/después.

**⚠️ CRITICAL**: Ninguna historia de usuario puede empezar hasta que esta fase esté completa.

- [X] T003 [P] Test de integración (SQLite real) para `Stats`: crear memorias en 2 proyectos distintos dentro del mismo `.memory/mem.db`, verificar `ProjectMemoryCount` vs `TotalMemoryCount`, y que `FileSizeBytes` coincide con `os.Stat` del archivo — en `tests/integration/maintenance_integration_test.go` (escribir primero; debe fallar porque `NewMaintenanceRepository` no existe aún)
- [X] T004 Crear `adapters/secondary/persistence/maintenance.go`: struct `maintenanceRepository{db *sql.DB; dbPath string}`, constructor `NewMaintenanceRepository(db *sql.DB, dbPath string) ports.MaintenanceRepository`, e implementar `Stats(project string) (ports.StorageStats, error)` (COUNT por proyecto + COUNT total + `os.Stat(dbPath)`) — hace pasar T003
- [X] T005 [P] Añadir campo `MaintenanceRepo ports.MaintenanceRepository` a la struct `Deps` en `adapters/primary/cli/deps.go`
- [X] T006 Wiring de `MaintenanceRepo` en `infrastructure/container.go`: campo en `Container`, construcción con `persistence.NewMaintenanceRepository(db, persistence.DbPath(root))` en `NewContainer`, y exponerlo en `ToDeps()` (depende de T004)

**Checkpoint**: Adaptador base + `Stats` funcionando end-to-end; las 4 historias pueden empezar.

---

## Phase 3: User Story 1 — Purgar memoria cuando crece demasiado (Priority: P1) 🎯 MVP

**Goal**: Permitir borrar memorias deliberadamente (todas, o filtradas por proyecto/tipo/antigüedad) desde CLI y TUI, con confirmación explícita y sin afectar otros proyectos ni dejar relaciones huérfanas.

**Independent Test**: Guardar varias memorias de prueba, ejecutar `mem purge` con un filtro, confirmar, y verificar que solo las memorias dentro del alcance fueron eliminadas y las demás permanecen intactas (spec.md, Acceptance Scenarios US1.1–US1.4).

### Tests for User Story 1 (escribir primero — deben fallar)

- [X] T007 [P] [US1] Test de integración: `Purge` con filtro por proyecto y por tipo elimina solo lo esperado, deja otros proyectos intactos, y limpia `memory_relations` huérfanas en la misma transacción — añadir casos a `tests/integration/maintenance_integration_test.go`
- [X] T008 [P] [US1] Test de contrato: `mem purge` exige confirmación (cancela sin `--yes`/respuesta "si"), respeta `--project`/`--all`/`--type`/`--older-than-days`, y reporta cantidad eliminada — en `tests/contract/maintenance_cli_test.go`

### Implementation for User Story 1

- [X] T009 [US1] Implementar `Purge(filter ports.PurgeFilter) (int64, error)` en `adapters/secondary/persistence/maintenance.go`: `DELETE FROM memories` parametrizado según `PurgeFilter` (proyecto/`All`/tipo/antigüedad) + `DELETE FROM memory_relations WHERE memory_id_a NOT IN (SELECT id FROM memories) OR memory_id_b NOT IN (SELECT id FROM memories)` en la misma transacción — hace pasar T007
- [X] T010 [US1] Crear `adapters/primary/cli/cmd_purge.go`: flags `--project`, `--all`, `--type`, `--older-than-days`, `--yes`; prompt de confirmación interactivo si no se pasa `--yes`; imprime resumen de memorias eliminadas (contrato completo en [contracts/cli-tui-contracts.md](./contracts/cli-tui-contracts.md#mem-purge))
- [X] T011 [US1] Registrar el caso `"purge"` en `adapters/primary/cli/dispatcher.go` y documentar `mem purge` en `Usage()` dentro de `adapters/primary/cli/cli.go`
- [X] T012 [US1] Agregar pantalla de mantenimiento (`screenMaintenance`) a `adapters/primary/tui/tui.go` con la opción "Purgar" — sub-pantalla de confirmación que exige escribir el nombre del proyecto (o "TODOS") antes de ejecutar, tecla `m` desde `screenList` para abrirla

**Checkpoint**: US1 funcional de forma independiente — purgar memoria desde CLI y desde TUI funciona end-to-end.

---

## Phase 4: User Story 2 — Compactar el almacenamiento (Priority: P2)

**Goal**: Recuperar espacio en disco liberado por borrados previos sin eliminar ninguna memoria, reportando tamaño antes/después.

**Independent Test**: Guardar memorias, borrar una parte significativa, ejecutar `mem compact`, y comparar el tamaño del archivo `.memory/mem.db` antes y después (spec.md, Acceptance Scenarios US2.1–US2.3).

### Tests for User Story 2 (escribir primero — deben fallar)

- [X] T013 [P] [US2] Test de integración: `Compact` reduce el tamaño en disco tras borrados previos, no pierde ni modifica filas sobrevivientes, y sobre una BD ya compacta no falla (reporta "nada que liberar") — añadir casos a `tests/integration/maintenance_integration_test.go`
- [X] T014 [P] [US2] Test de contrato: salida de `mem compact` incluye tamaño antes/después y el caso "sin espacio que liberar" no es tratado como error — en `tests/contract/maintenance_cli_test.go`

### Implementation for User Story 2

- [X] T015 [US2] Implementar `Compact() (beforeBytes, afterBytes int64, err error)` en `adapters/secondary/persistence/maintenance.go`: mide tamaño con `os.Stat(dbPath)`, ejecuta `VACUUM;`, mide tamaño de nuevo — hace pasar T013
- [X] T016 [US2] Crear `adapters/primary/cli/cmd_compact.go`: ejecuta `Compact()` sin necesidad de confirmación (operación no destructiva) e imprime "antes → después (liberado: X)" usando `humanize.Bytes`
- [X] T017 [US2] Registrar el caso `"compact"` en `adapters/primary/cli/dispatcher.go` y documentar `mem compact` en `Usage()` (`adapters/primary/cli/cli.go`)
- [X] T018 [US2] Agregar la opción "Compactar" en `screenMaintenance` de `adapters/primary/tui/tui.go` (ejecución directa, resultado vía `statusMsg`) y extender el header de `listView()` para mostrar el tamaño en disco junto al conteo de memorias

**Checkpoint**: US1 y US2 funcionan de forma independiente y combinada.

---

## Phase 5: User Story 3 — Garbage collection de memorias antiguas (Priority: P3)

**Goal**: Limpiar memorias más antiguas que un umbral de retención mediante un comando/acción explícita (nunca automático), reutilizando la lógica de purga de US1.

**Independent Test**: Guardar memorias con distintas fechas de creación, ejecutar el garbage collector con un umbral determinado, y verificar que solo las memorias más viejas que el umbral fueron eliminadas (spec.md, Acceptance Scenarios US3.1–US3.3).

### Tests for User Story 3 (escribir primero — deben fallar)

- [X] T019 [P] [US3] Test de integración: GC con `OlderThanDays` elimina solo memorias más viejas que el umbral, deja las recientes intactas, e informa "nada que limpiar" cuando ninguna supera el umbral — añadir casos a `tests/integration/maintenance_integration_test.go`
- [X] T020 [P] [US3] Test de contrato: `mem gc` usa default de 90 días si no se especifica `--older-than-days`, exige confirmación salvo `--yes`, y reporta cantidad eliminada por proyecto — en `tests/contract/maintenance_cli_test.go`

### Implementation for User Story 3

- [X] T021 [US3] Crear `adapters/primary/cli/cmd_gc.go`: flags `--project`, `--all`, `--older-than-days` (default 90), `--yes`; construye un `ports.PurgeFilter` y reutiliza `deps.MaintenanceRepo.Purge(...)` (sin tocar `maintenance.go` — ya implementado en T009); texto de confirmación distinto al de purga ("garbage collection" vs "purga total")
- [X] T022 [US3] Registrar el caso `"gc"` en `adapters/primary/cli/dispatcher.go` y documentar `mem gc` en `Usage()` (`adapters/primary/cli/cli.go`)
- [X] T023 [US3] Agregar la opción "GC" en `screenMaintenance` de `adapters/primary/tui/tui.go`, reutilizando la misma sub-pantalla de confirmación que "Purgar" (T012)

**Checkpoint**: US1, US2 y US3 funcionan de forma independiente y combinada.

---

## Phase 6: User Story 4 — Desinstalación completa de la herramienta (Priority: P4)

**Goal**: Reverso exacto de `mem install`: además de los datos, remueve el binario `mem`, los hooks, las entradas en AGENTS.md/CLAUDE.md y el registro MCP, como acción separada de la purga.

**Independent Test**: Instalar gomemory en un directorio de prueba (`mem install`), guardar memorias, ejecutar `mem uninstall`, y verificar que binario, hooks, entradas en AGENTS.md/CLAUDE.md, registro MCP y `.memory/` ya no existen (spec.md, Acceptance Scenarios US4.1–US4.3).

### Tests for User Story 4 (escribir primero — deben fallar)

- [X] T024 [P] [US4] Test de integración: instalar con `mem install` sobre un `t.TempDir()`, ejecutar `mem uninstall --yes`, y verificar ausencia de binario, `.claude/plugins/gomemory/`, entradas `gomemory` en `.mcp.json`, bloque delimitado por `integrationMarker` en `AGENTS.md`/`CLAUDE.md`, y `.memory/` — en `tests/integration/uninstall_integration_test.go` (archivo nuevo, usa filesystem real en vez de solo SQLite)
- [X] T025 [P] [US4] Test de contrato: `mem uninstall` cancela sin confirmar, reporta componentes no encontrados sin fallar (instalación parcial), y respeta `--yes` — en `tests/contract/maintenance_cli_test.go`

### Implementation for User Story 4

- [X] T026 [US4] Crear `adapters/primary/cli/cmd_uninstall.go`: resolver `target` igual que `CmdInstall`; prompt de confirmación salvo `--yes`; eliminar el bloque `integrationMarker`/`integrationVersionMarker` de `AGENTS.md`/`CLAUDE.md`/`CLAUDE.txt`/`.cursorrules`/`.windsurfrules` (borrar el archivo completo solo si quedó vacío tras quitar el bloque); eliminar entradas `"gomemory"` de `.mcp.json`, `.cursor/mcp.json`, `.windsurf/mcp_config.json`, `.cline/mcp_settings.json`; eliminar `.claude/plugins/gomemory/` y las entradas de hooks correspondientes en `.claude/settings.json`; eliminar `.memory/`; eliminar el binario `mem` con `os.SameFile` para detectar auto-eliminación (ver [research.md](./research.md#5-auto-eliminación-segura-del-binario-en-mem-uninstall)); imprimir resumen de qué se eliminó y qué no se encontró — hace pasar T024
- [X] T027 [US4] Registrar el caso `"uninstall"` en `adapters/primary/cli/dispatcher.go` y documentar `mem uninstall [dir]` en `Usage()` (`adapters/primary/cli/cli.go`)

**Checkpoint**: Las 4 historias de usuario funcionan de forma independiente.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Calidad transversal tras completar las historias deseadas.

- [X] T028 [P] Ejecutar `golangci-lint run ./...` y corregir cualquier hallazgo en los archivos nuevos/editados
- [X] T029 [P] Ejecutar `go build ./... && go test ./...`; verificar cobertura ≥80% en `application/ports/maintenance_repository.go`, `adapters/secondary/persistence/maintenance.go` y los comandos CLI nuevos (Principio III)
- [X] T030 [P] Documentar `mem purge`, `mem compact`, `mem gc` y `mem uninstall` en `docs/MANUAL.md`
- [X] T031 Ejecutar manualmente la guía [quickstart.md](./quickstart.md) de punta a punta (las 7 secciones) y confirmar cada resultado esperado

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias — puede iniciar de inmediato
- **Foundational (Phase 2)**: depende de Setup — BLOQUEA las 4 historias de usuario
- **US1 (Phase 3)**: depende de Foundational; es la base de la que depende US3
- **US2 (Phase 4)**: depende de Foundational; independiente de US1/US3
- **US3 (Phase 5)**: depende de Foundational **y** de la implementación de `Purge` hecha en US1 (T009) — reutiliza ese método, no duplica SQL
- **US4 (Phase 6)**: depende solo de Foundational (no usa `MaintenanceRepository`; opera sobre filesystem, simétrico a `cmd_install.go`)
- **Polish (Phase 7)**: depende de todas las historias que se hayan completado

### Within Each User Story

- Tests escritos y en estado FALLIDO antes de cada tarea de implementación correspondiente (TDD, Principio III)
- Adaptador (SQL) antes de comando CLI
- Comando CLI antes de registrarlo en `dispatcher.go`/`Usage()`
- Wiring CLI antes de la acción equivalente en TUI

### Parallel Opportunities

- T001 y T002 (Setup) — archivos distintos
- T003 y T005 (Foundational) — archivos distintos, sin dependencia entre sí
- Dentro de cada historia: la tarea de test de integración y la de test de contrato son `[P]` entre sí (archivos distintos); las tareas de implementación son secuenciales por construir sobre el mismo archivo o por depender del test correspondiente
- US2 puede implementarse en paralelo con US1 una vez completada la fase Foundational (no comparten archivo de implementación, solo el header de la TUI al final — T018 debe aplicarse después de T012 si ambas tocan `tui.go` en la misma sesión de trabajo)
- US4 puede implementarse en paralelo con US1/US2/US3 en cualquier momento tras Foundational — no comparte ningún archivo de implementación con ellas

---

## Parallel Example: User Story 1

```bash
# Lanzar juntos los dos tests de US1 (archivos distintos):
Task: "Test de integración Purge en tests/integration/maintenance_integration_test.go"
Task: "Test de contrato mem purge en tests/contract/maintenance_cli_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Completar Fase 1: Setup
2. Completar Fase 2: Foundational (CRÍTICO — bloquea todas las historias)
3. Completar Fase 3: User Story 1 (purgar)
4. **DETENERSE Y VALIDAR**: correr las secciones 1–3 de `quickstart.md` de forma independiente
5. Entregar/demostrar si está listo

### Incremental Delivery

1. Setup + Foundational → base lista
2. + US1 (purgar) → validar independientemente → MVP entregable
3. + US2 (compactar) → validar independientemente
4. + US3 (GC, reutiliza Purge de US1) → validar independientemente
5. + US4 (desinstalación completa) → validar independientemente
6. Cada historia agrega valor sin romper las anteriores

### Parallel Team Strategy

Con varios desarrolladores, tras completar Foundational:

- Desarrollador A: US1 (purgar) — luego puede continuar con US3 (GC), que depende de su propio trabajo
- Desarrollador B: US2 (compactar) — independiente de A
- Desarrollador C: US4 (desinstalación completa) — independiente de A y B, sin tocar `maintenance.go`

---

## Notes

- `[P]` = archivos distintos, sin dependencias pendientes entre sí
- `[Story]` mapea cada tarea a su historia de usuario para trazabilidad
- Tests del Principio III: escribir y ver fallar ANTES de implementar — no es opcional en este proyecto
- Commitear tras cada tarea o grupo lógico de tareas
- Detenerse en cualquier checkpoint para validar la historia de forma independiente
- Evitar: tareas vagas, conflictos de archivo simultáneos, dependencias cruzadas entre historias que rompan su independencia (US3 depende de US1 a propósito — está documentado arriba, no es una dependencia oculta)
