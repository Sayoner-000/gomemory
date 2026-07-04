# Implementation Plan: Registro global de gomemory (sin instalación por proyecto)

**Branch**: `005-global-mcp-store` | **Date**: 2026-07-03 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-global-mcp-store/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Eliminar la fricción de "instalar gomemory por proyecto" (copiar binario, crear `.memory/mem.db` local, escribir configs MCP por repo, inyectar protocolo en `AGENTS.md`/`CLAUDE.md`) manteniendo el aislamiento de memoria por proyecto que ya existe hoy. Técnicamente: el archivo `mem.db` deja de vivir en `<repo>/.memory/` y pasa a un store global del usuario, indexado por una clave derivada de la ruta absoluta del git root del proyecto (no del nombre de su carpeta); la resolución de proyecto deja de exigir un `.memory/` preinstalado (init perezoso); y el registro del servidor MCP pasa a ser global (una vez por máquina) para los agentes que lo soporten, empezando por Claude Code, con fallback por-proyecto donde no. Los datos existentes se migran de forma transparente y sin pérdida. El registro global de MCP y la limpieza de artefactos legados permanecen fuera de MCP (solo CLI), siguiendo el mismo criterio ya aplicado en `003-memory-maintenance` para operaciones que tocan configuración global o borran archivos.

## Technical Context

**Language/Version**: Go 1.25 (toolchain go1.25.11), módulo `mem`

**Primary Dependencies**: stdlib (`os`, `path/filepath`, `crypto/sha256`, `database/sql`); `modernc.org/sqlite` (sin CGO, ya en uso); `modelcontextprotocol/go-sdk` (ya en uso); `testify`

**Storage**: SQLite vía `modernc.org/sqlite`, un archivo `mem.db` por proyecto (sin cambio en ese aislamiento), pero reubicado de `<repo>/.memory/mem.db` a un store global del usuario (`$XDG_DATA_HOME/gomemory/projects/<clave>/mem.db`, con fallback a `~/.local/share/gomemory` en Linux/macOS y `%LOCALAPPDATA%\gomemory` en Windows). La clave de proyecto pasa de `filepath.Base(root)` a un slug derivado de la ruta absoluta del git root (o del cwd si no hay `.git`).

**Testing**: `testing` + `testify`; `tests/unit/` (mocks de `ports.ProjectRepository` extendido); `tests/integration/` (filesystem + SQLite reales: resolución de proyecto sin `.memory/` previo, migración legado→global, aislamiento entre proyectos con nombre de carpeta duplicado); `tests/contract/` (formato/flags de `mem migrate`, comportamiento no-op de `mem init`, `mem mcp` sin `--root` ni `.memory/` previo)

**Target Platform**: binario CLI/TUI/MCP autocontenido, multiplataforma (Linux, macOS, Windows) — sin cambios respecto a hoy

**Project Type**: proyecto único — CLI + TUI + MCP sobre la arquitectura hexagonal existente (no se introduce un segundo proyecto/servicio)

**Performance Goals**: uso local/personal, sin cambio respecto al modelo actual (mismo motor SQLite, mismo WAL + busy timeout 5s); costo adicional aceptado: un cálculo de hash de ruta + un `os.Stat`/`os.MkdirAll` por resolución de proyecto en cada invocación de comando

**Constraints**:
- Cero pérdida de datos al migrar proyectos existentes de `.memory/mem.db` a el store global
- `mem mcp` (y el resto de comandos) NUNCA deben fallar por ausencia de instalación previa en el repo — como mucho, crean el store la primera vez (init perezoso)
- La resolución de la colisión de nombre ya detectada en el registro global de Claude Code (`~/.claude.json`, clave `gomemory` apuntando hoy a otro binario) es una acción manual de una sola vez, fuera del alcance de este código — el rollout la asume resuelta antes de activar `mem setup-mcp --scope global claude`
- El registro global de MCP y la migración/limpieza de artefactos legados se exponen solo vía CLI, nunca vía MCP (mismo criterio que `003-memory-maintenance`: son operaciones que tocan configuración global o borran/mueven archivos y requieren confirmación humana)

**Scale/Scope**: N proyectos por usuario en la misma máquina, cada uno con su propio `mem.db` aislado dentro del store global (sin límite adicional al espacio en disco); 1 puerto extendido (`ProjectRepository.Key`), 1 archivo nuevo de dominio de infraestructura (`globalstore.go`), 1 comando CLI nuevo (`mem migrate`), cambios en `cmd_init.go`/`cmd_install.go`/`cmd_mcp.go`/`cmd_mcp_setup.go` y en los setups de Claude Code/Codex/OpenCode

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Evaluación | Notas |
|-----------|------------|-------|
| I. Arquitectura Hexagonal | ✅ Cumple | `globalstore.go` nuevo vive en `adapters/secondary/persistence/` (solo importa stdlib), no en dominio/aplicación. `ports.ProjectRepository` se extiende con `Key(root string) string` pero mantiene su forma de puerto; los ~25 comandos que ya dependen de la interfaz no cambian su forma de uso. |
| II. SQLite con SQL Directo | ✅ Cumple | No se introduce SQL nuevo; `Open`/`migrate` siguen igual, solo cambia la ruta del archivo que abren. Sin ORM. |
| III. Testing First | ⚠️ Gate aplica en `/speckit.tasks` | Este plan no escribe código; `tasks.md` debe ordenar tests (unit con mocks de `ProjectRepository`, integration con filesystem+SQLite reales, contract de `mem migrate`/`mem init`/`mem mcp`) ANTES de cada implementación, ciclo Red-Green-Refactor, cobertura ≥80%. |
| IV. Configuración y Entorno | ✅ Cumple | `DataHome()` resuelve `$XDG_DATA_HOME`/`%LOCALAPPDATA%` con fallback fijo documentado — son convenciones de directorio del sistema operativo, no un valor de negocio que deba venir de una variable propia de la app; no se hardcodea ninguna ruta de una máquina específica (mismo criterio que ya se aplicó a `--root` en `binref.go`, que explícitamente evita rutas absolutas de máquina). |
| V. Principios Operativos | ✅ Cumple | Simplicidad: se reutiliza la interfaz `ProjectRepository` existente en vez de crear una capa nueva. Causa raíz: se resuelve la fricción real de instalación, no se parcha con más pasos manuales. Idempotencia: `mem migrate`/init perezoso son no-op seguro si ya se migró. Fallar rápido: `FindProjectRoot` y `ProjectKey` se validan en el borde (CLI/MCP entrypoint) antes de tocar disco. |
| IX. MCP como integración primaria | ⚠️ Excepción documentada (mismo precedente que 003) | El registro global de MCP (`mem setup-mcp --scope global`) y la migración/limpieza de artefactos legados (`mem migrate`, limpieza de `.mcp.json`/binario/bloques de protocolo) NO se exponen vía MCP: implican modificar configuración global compartida (`~/.claude.json`, `~/.codex/config.toml`) o mover/borrar archivos, operaciones que ya se decidió (en `003-memory-maintenance`) mantener fuera de MCP por requerir confirmación humana explícita. |

No hay violaciones que requieran `Complexity Tracking` — la tabla queda vacía (ver sección al final).

## Project Structure

### Documentation (this feature)

```text
specs/005-global-mcp-store/
├── plan.md              # Este archivo (/speckit.plan)
├── research.md          # Fase 0 (/speckit.plan)
├── data-model.md         # Fase 1 (/speckit.plan)
├── quickstart.md         # Fase 1 (/speckit.plan)
├── contracts/
│   └── cli-contracts.md  # Fase 1 (/speckit.plan)
└── tasks.md              # Fase 2 (/speckit.tasks — NO se crea en este comando)
```

### Source Code (repository root)

**Structure Decision**: Proyecto único Go (CLI + TUI + MCP) sobre la arquitectura hexagonal ya existente (`domain/`, `application/`, `adapters/`, `infrastructure/`). Se extiende el puerto `ProjectRepository` existente y su implementación, se añade un archivo nuevo de resolución de store global, un comando CLI nuevo (`mem migrate`), y se ajustan los comandos/setups que hoy asumen instalación por proyecto. No se introduce una capa de casos de uso nueva ni un segundo proyecto/servicio.

```text
domain/                                          # SIN CAMBIOS

application/
└── ports/
    └── project_repository.go                    # EDITAR — añade Key(root string) string

adapters/
├── primary/
│   ├── cli/
│   │   ├── cmd_migrate.go                       # NUEVO — `mem migrate`
│   │   ├── cmd_init.go                          # EDITAR — no-op informativo + trigger de migración
│   │   ├── cmd_install.go                       # EDITAR — re-scope a registro global + migración, deprecar copia de archivos
│   │   ├── cmd_mcp.go                           # EDITAR — quita el fatal por `.memory/` ausente
│   │   ├── cmd_mcp_setup.go                     # EDITAR — Codex a una sola tabla global sin `cwd`; flag `--scope project|global`
│   │   └── dispatcher.go                        # EDITAR — registrar `migrate`
│   └── setup/
│       ├── claude_code_setup.go                 # EDITAR — registro en scope de usuario (`~/.claude.json`)
│       └── opencode_setup.go                    # EDITAR — evaluar registro MCP global si OpenCode lo soporta
└── secondary/
    └── persistence/
        ├── globalstore.go                        # NUEVO — FindProjectRoot, ProjectKey, DataHome, GlobalProjectDir/DbPath, migración legado→global
        ├── db.go                                  # EDITAR — FindRoot/DbPath/EnsureDir delegan al store global
        └── repositories.go                        # EDITAR — wrapper expone `Key(root)`

infrastructure/
├── container.go                                  # EDITAR — `project := deps.ProjectRepo.Key(root)` en vez de `filepath.Base(root)`
└── main.go                                        # EDITAR — simplifica/retira `rootIndependentCommands` (ya no hay comandos que fallen por falta de `.memory/`)

tests/
├── unit/
│   └── globalstore_test.go                       # NUEVO — mocks de `ports.ProjectRepository` extendido
├── integration/
│   └── global_store_migration_test.go            # NUEVO — filesystem+SQLite reales: init perezoso, migración legado→global, aislamiento entre proyectos con nombre de carpeta duplicado
└── contract/
    ├── cmd_migrate_test.go                        # NUEVO
    └── cmd_init_test.go                           # EDITAR — cubre el nuevo comportamiento no-op
```

## Complexity Tracking

> No hay violaciones de la constitución que requieran justificación en esta tabla.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
