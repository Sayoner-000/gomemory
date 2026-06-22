# Implementation Plan: Mantenimiento de Memoria (Purga, Compactación y Garbage Collector)

**Branch**: `003-memory-maintenance` | **Date**: 2026-06-22 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-memory-maintenance/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Agregar mantenimiento de almacenamiento a gomemory: **purgar** memorias (con alcance por proyecto/tipo/antigüedad), **compactar** el archivo SQLite para recuperar espacio en disco, **garbage collection** a demanda para limpiar memorias viejas, y **desinstalación completa** (reverso de `mem install`) como acción separada. Técnicamente: un nuevo puerto `MaintenanceRepository` con un único método de borrado parametrizado (`Purge`) reutilizado tanto por `mem purge` como por `mem gc`, un método `Compact` que ejecuta `VACUUM`, y un método `Stats` para tamaño/conteo; expuestos vía 4 comandos CLI nuevos y acciones nuevas en la TUI existente. Deliberadamente **no se expone vía MCP** por ser operaciones destructivas que requieren confirmación humana.

## Technical Context

**Language/Version**: Go 1.25 (toolchain go1.25.11), módulo `mem`

**Primary Dependencies**: `flag` + `database/sql` + `os` (stdlib); `charmbracelet/bubbletea` + `bubbles/textinput` + `lipgloss` (TUI); `modernc.org/sqlite` (sin CGO); `dustin/go-humanize` (ya dependencia indirecta, se promueve a directa para formatear bytes legibles en los reportes)

**Storage**: SQLite vía `modernc.org/sqlite`, archivo único `.memory/mem.db` en modo WAL; el esquema ya soporta múltiples proyectos en un mismo archivo (columna `project` en `memories`, `sessions`, `memory_relations`)

**Testing**: `testing` + `testify`; `tests/unit/` (mocks de `ports.MaintenanceRepository`), `tests/integration/` (SQLite real: purga, compactación, GC, relaciones huérfanas), `tests/contract/` (formato y flags de los comandos CLI nuevos)

**Target Platform**: binario CLI/TUI autocontenido, multiplataforma (Linux, macOS, Windows)

**Project Type**: proyecto único — CLI + TUI sobre la arquitectura hexagonal ya existente (no se introduce un segundo proyecto/servicio)

**Performance Goals**: uso local/personal — cientos a pocos miles de filas por archivo `.memory/mem.db`; sin meta de throughput, sí de no bloquear la CLI/TUI más de unos segundos por operación de mantenimiento

**Constraints**:
- No romper el modelo de concurrencia existente (WAL + busy timeout 5s)
- Toda operación destructiva (purga, GC, desinstalación) DEBE pasar por una confirmación explícita (FR-002, FR-013) antes de ejecutarse
- `mem uninstall` debe poder borrar su propio binario en ejecución sin fallar en Linux/macOS (unlink de ejecutable en uso es válido en POSIX); en Windows, si el archivo está bloqueado, debe completar el resto de la limpieza y reportar al usuario que debe borrar el binario manualmente al cerrar el proceso
- Las nuevas capacidades se exponen solo vía CLI y TUI — explícitamente fuera del servidor MCP

**Scale/Scope**: 4 capacidades nuevas (purgar, compactar, GC, desinstalar) sobre 4 historias de usuario; 1 puerto nuevo, 1 adaptador de persistencia nuevo, 4 comandos CLI nuevos, nuevas acciones/pantallas en la TUI existente, wiring en el composition root

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Evaluación | Notas |
|-----------|------------|-------|
| I. Arquitectura Hexagonal | ✅ Cumple | Nuevo puerto en `application/ports/maintenance_repository.go` (solo importa stdlib, sin domain ni adaptadores); adaptador concreto en `adapters/secondary/persistence/maintenance.go`; CLI/TUI (primarios) dependen del puerto, no del adaptador concreto. Sin nueva capa de casos de uso: se sigue el patrón ya usado por `cmd_save.go` (CLI llama directo al repo), evitando abstracción prematura. |
| II. SQLite con SQL Directo | ✅ Cumple | `Purge` usa `DELETE` parametrizado con bind args; `Compact` usa `VACUUM` nativo; limpieza de relaciones huérfanas con `DELETE ... WHERE id NOT IN (SELECT ...)` parametrizado. Sin ORM. |
| III. Testing First | ⚠️ Gate aplica en `/speckit.tasks` | Plan no escribe código; `tasks.md` deberá ordenar tests (unit con mocks, integration con SQLite real) ANTES de cada implementación, ciclo Red-Green-Refactor, cobertura ≥80%. |
| IV. Configuración y Entorno | ✅ Cumple | El umbral de retención (`OlderThanDays`) es un parámetro de invocación (flag CLI / input TUI) con default documentado (90 días), no una variable de entorno — no aplica la regla de "todo valor que cambie entre entornos debe venir de env vars" porque no es un valor de entorno, es un parámetro de usuario en tiempo de ejecución, igual que `--limit` en `mem list`. |
| V. Principios Operativos | ✅ Cumple | Simplicidad: un solo método `Purge` reutilizado por purga y GC, sin duplicar SQL. Causa raíz: se aborda el problema real (archivo inflado) con `VACUUM`, no un workaround. Idempotencia: repetir `Purge`/`Compact` sobre un estado ya limpio es un no-op seguro. Fallar rápido: validar alcance/flags en el borde (CLI) antes de tocar la BD. |
| IX. MCP como integración primaria | ⚠️ Excepción documentada | Las 4 capacidades NO se exponen como tools MCP. Razón: son operaciones destructivas que exigen confirmación humana explícita (FR-002, FR-013); exponerlas vía MCP permitiría que un agente AI las dispare sin supervisión (incluso con `autoApprove` activo), contradiciendo el propósito de la confirmación. Se documenta como decisión de alcance, no como deuda técnica — ver `research.md` punto 6. |

No hay violaciones que requieran `Complexity Tracking` — la tabla queda vacía (ver sección al final).

## Project Structure

### Documentation (this feature)

```text
specs/003-memory-maintenance/
├── plan.md              # Este archivo (/speckit.plan)
├── research.md          # Fase 0 (/speckit.plan)
├── data-model.md         # Fase 1 (/speckit.plan)
├── quickstart.md         # Fase 1 (/speckit.plan)
├── contracts/
│   └── cli-tui-contracts.md   # Fase 1 (/speckit.plan)
└── tasks.md              # Fase 2 (/speckit.tasks — NO se crea en este comando)
```

### Source Code (repository root)

**Structure Decision**: Proyecto único Go (CLI + TUI) sobre la arquitectura hexagonal ya existente en este repo (`domain/`, `application/`, `adapters/`, `infrastructure/`). Se añade un puerto nuevo, su adaptador SQLite, 4 comandos CLI y nuevas acciones en la TUI existente. No se introduce una capa de casos de uso nueva: las operaciones son CRUD/administrativas directas, consistente con cómo ya funcionan `cmd_save.go`, `cmd_list.go`, etc.

```text
domain/                                          # SIN CAMBIOS — no se necesitan nuevas entidades de dominio

application/
└── ports/
    └── maintenance_repository.go                # NUEVO — StorageStats, PurgeFilter, MaintenanceRepository

adapters/
├── primary/
│   ├── cli/
│   │   ├── cmd_purge.go                         # NUEVO — `mem purge`
│   │   ├── cmd_compact.go                       # NUEVO — `mem compact`
│   │   ├── cmd_gc.go                            # NUEVO — `mem gc`
│   │   ├── cmd_uninstall.go                     # NUEVO — `mem uninstall [dir]`
│   │   ├── dispatcher.go                        # EDITAR — registrar los 4 comandos nuevos
│   │   └── cli.go                               # EDITAR — actualizar Usage()
│   └── tui/
│       └── tui.go                                # EDITAR — pantalla de mantenimiento + stats en header
└── secondary/
    └── persistence/
        └── maintenance.go                        # NUEVO — Stats/Purge/Compact con SQL directo

infrastructure/
└── container.go                                  # EDITAR — wiring de MaintenanceRepository

tests/
├── unit/
│   └── maintenance_test.go                       # NUEVO — mocks de ports.MaintenanceRepository
├── integration/
│   └── maintenance_integration_test.go           # NUEVO — SQLite real: purge/compact/relaciones huérfanas
└── contract/
    └── maintenance_cli_test.go                    # NUEVO — formato/flags de los 4 comandos nuevos
```

## Complexity Tracking

> No hay violaciones de la constitución que requieran justificación en esta tabla.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
