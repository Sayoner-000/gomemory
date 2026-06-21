# Implementation Plan: Reorganización a Arquitectura Hexagonal

**Branch**: `002-hexagonal-architecture` | **Date**: 2026-06-21 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/002-hexagonal-architecture/spec.md`

## Summary

Reorganizar los 16 archivos `cmd_*.go` en la raíz y los paquetes `store/`, `types/`, `context/`, `tui/`, `internal/server/`, `internal/setup/` en una arquitectura hexagonal de 4 capas: `domain/`, `application/`, `adapters/`, `infrastructure/`. La migración es puramente estructural: preserva toda funcionalidad, tests, y el módulo Go `mem`. Usa `git mv` para preservar historial.

## Technical Context

**Language/Version**: Go 1.25.0

**Primary Dependencies**: `modernc.org/sqlite` (driver), `charmbracelet/bubbletea` (TUI), `charmbracelet/bubbles`, `charmbracelet/lipgloss`, `modelcontextprotocol/go-sdk` (MCP)

**Storage**: SQLite embebido via `modernc.org/sqlite` (sin CGO), WAL mode

**Testing**: `testing` stdlib + `testify`, organización: `tests/unit/`, `tests/integration/`, `tests/contract/`

**Target Platform**: Linux/macOS/Windows, binario autocontenido ~16MB

**Project Type**: CLI + TUI + MCP server (multicapa)

**Performance Goals**: Sin cambios de performance esperados (migración estructural)

**Constraints**: Preservar todos los tests existentes sin modificaciones. Usar `git mv` para historial. No cambiar `go.mod`. No introducir nuevas dependencias. No frameworks de DI.

**Scale/Scope**: ~16 archivos cmd_*.go, 5 paquetes internos, ~7k LOC aprox

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Gate 1 — Arquitectura Hexagonal (Principio I)
- **Veredicto**: ✅ CUMPLE. La migración implementa el Principio I que el proyecto ya exige: dominio sin I/O, puertos en aplicación, adaptadores concretos en capa externa, composition root único.
- **Justificación**: El código actual viola este principio (store llamado directamente desde cmd_*.go). La reorganización corrige la violación.

### Gate 2 — SQLite con SQL Directo (Principio II)
- **Veredicto**: ✅ CUMPLE. No se cambia la estrategia de persistencia. store/ se mueve a adapters/secondary/persistence/ pero conserva SQL directo, parámetros bind, WAL mode, timestamps UTC-5.

### Gate 3 — Testing First (Principio III)
- **Veredicto**: ⚠️ EXCEPCIÓN CONTROLADA APROBADA. No se modifican tests existentes (FR-016). Los tests actuales importan directamente `mem/store` y `mem/types`. Al mover estos paquetes, los imports en tests se romperán. 
- **Excepción**: Los tests existentes actualizan SOLAMENTE sus import paths a las nuevas rutas (ej. `mem/store` → `mem/adapters/secondary/persistence`). Esto NO es modificar lógica de tests, solo actualizar import paths. Documentado en `research.md` y `contracts/layer-contracts.md` como excepción controlada.
- **Post-design check**: El diseño no introduce nuevas violaciones. Las interfaces (`ports`) permiten que los tests futuros usen mocks sin acoplamiento a implementaciones concretas.

### Gate 4 — Configuración y Entorno (Principio IV)
- **Veredicto**: ✅ CUMPLE. Sin cambios en configuración.

### Gate 5 — Principios Operativos (Principio V)
- **Veredicto**: ✅ CUMPLE. Simplicidad (migración pura, sin cambios de lógica), sin parches temporales, idempotencia preservada.

### Gate 6 — Stack Congelado
- **Veredicto**: ✅ CUMPLE. No se agregan ni cambian dependencias.

### Gate 7 — Documentación en Español
- **Veredicto**: ✅ CUMPLE. Plan y artefactos en español.

## Project Structure

### Documentation (this feature)

```text
specs/002-hexagonal-architecture/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Created by /speckit.tasks
```

### Source Code — Estructura Hexagonal Destino

```text
# Raíz del proyecto (package main)
domain/                          # Capa más interna — 0 dependencias del proyecto
├── memory.go                    #   tipos Memory, MemoryType, validación
├── session.go                   #   tipos Session, SessionStatus
├── relation.go                  #   tipos Relation, RelationType, Confidence
└── errors.go                    #   errores de dominio (ErrNotFound, ErrValidation)

application/                     # Capa de aplicación — solo importa domain/
├── ports/                       #   Puertos (interfaces) que definen contratos
│   ├── memory_repository.go     #     MemoryRepository interface
│   ├── session_repository.go    #     SessionRepository interface
│   ├── relation_repository.go   #     RelationRepository interface
│   └── context_builder.go       #     ContextBuilder interface
└── usecases/                    #   Casos de uso orquestadores
    ├── save_memory.go           #     SaveMemoryUseCase
    ├── search_memories.go       #     SearchMemoriesUseCase
    ├── manage_session.go        #     ManageSessionUseCase
    └── build_context.go         #     BuildContextUseCase

adapters/                        # Capa de adaptadores — implementan puertos
├── primary/                     #   Adaptadores primarios (driving)
│   ├── cli/                     #     Comandos CLI (cmd_*.go)
│   │   ├── cli.go               #       Dispatcher CLI
│   │   ├── cmd_init.go          #       mem init
│   │   ├── cmd_save.go          #       mem save
│   │   ├── cmd_capture.go       #       mem capture
│   │   ├── cmd_compare.go       #       mem compare
│   │   ├── cmd_list.go          #       mem list / log
│   │   ├── cmd_search.go        #       mem search
│   │   ├── cmd_context.go       #       mem context
│   │   ├── cmd_session.go       #       mem session
│   │   ├── cmd_install.go       #       mem install
│   │   ├── cmd_project.go       #       mem project
│   │   ├── cmd_wrap.go          #       mem wrap
│   │   ├── cmd_mcp.go           #       mem mcp
│   │   ├── cmd_mcp_setup.go     #       mem setup-mcp
│   │   ├── cmd_serve.go         #       mem serve
│   │   ├── cmd_setup.go         #       mem setup
│   │   └── cmd_settings.go      #       mem settings
│   ├── tui/                     #     TUI (Bubbletea)
│   │   └── tui.go
│   ├── mcp/                     #     Servidor MCP (antes internal/server/)
│   │   └── server.go
│   └── setup/                   #     Setup de plugins (antes internal/setup/)
│       ├── setup.go
│       ├── opencode_setup.go
│       └── claude_code_setup.go
└── secondary/                   #   Adaptadores secundarios (driven)
    └── persistence/             #     Persistencia SQLite (antes store/)
        ├── db.go                #       Conexión, migraciones, FindRoot
        ├── memory.go            #       CRUD memorias
        ├── session.go           #       CRUD sesiones
        ├── relation.go          #       CRUD relaciones
        └── settings.go          #       Configuración local

infrastructure/                  # Capa de infraestructura — composition root
├── main.go                      #   Composition root: flags, wiring, dispatch
├── config.go                    #   Config (variables de entorno)
└── container.go                 #   Wiring de dependencias (DI manual)

tests/                           # Tests existentes (sin cambios estructurales)
├── contract/
│   └── memory_protocol_test.go
├── integration/
│   └── plugin_integration_test.go
└── unit/                        #   (nuevos tests unitarios aquí en el futuro)

tui/                             # [ELIMINAR] migrado a adapters/primary/tui/
store/                           # [ELIMINAR] migrado a adapters/secondary/persistence/
types/                           # [ELIMINAR] migrado a domain/
context/                         # [ELIMINAR] migrado a application/usecases/ o adapters/
internal/                        # [ELIMINAR] migrado a adapters/primary/{mcp,setup}/
cmd_*.go                         # [ELIMINAR] migrados a adapters/primary/cli/
main.go                          # [ELIMINAR] migrado a infrastructure/main.go
```

**Structure Decision**: Arquitectura hexagonal pura de 4 capas según el Principio I de la constitución. Los nombres de directorios siguen la convención estándar hexagonal: `domain/`, `application/`, `adapters/`, `infrastructure/`. Los adaptadores se subdividen en `primary/` (driving) y `secondary/` (driven). No se usa `internal/` para evitar anidamiento innecesario.

## Complexity Tracking

> Sin violaciones constitucionales que justificar.
