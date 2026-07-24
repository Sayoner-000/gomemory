# Implementation Plan: Evolución de la Integración con Grafo de Código Externo

**Branch**: `010-codegraph-integration-evolution` | **Date**: 2026-07-23 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/010-codegraph-integration-evolution/spec.md`

## Summary

Evolucionar la integración opcional con un proveedor externo de grafo de
código (hoy codebase-memory-mcp) desde un enriquecimiento agregado y de un
solo sentido hacia tres capacidades dirigidas y aditivas, todas
independientes y todas desactivables: (1) anotar impacto de código al
guardar una memoria con `filepath`, leyendo solo el snapshot ya cacheado;
(2) sincronizar decisiones arquitectónicas como ADR en ambos sentidos,
registrando el origen de cada par memoria↔ADR para evitar bucles; (3)
soportar múltiples proveedores candidatos con selección automática por
disponibilidad, reutilizando la lista `[]ports.CodeGraphProvider` que
`build_context.go` ya recorre hoy (alimentada por `settings.
CodeGraphProviders` en vez de un único comando). Enfoque técnico: extender tipos y
settings existentes de forma aditiva, una tabla SQLite nueva
(`adr_sync_records`) para el registro de sincronización, y ningún cambio al
contrato de no-bloqueo ya probado (`Snapshot()` instantáneo, `MaybeRefresh()`
detached).

## Technical Context

**Language/Version**: Go 1.25 (toolchain fijado en `go.mod`)

**Primary Dependencies**: `modernc.org/sqlite` (persistencia de
`adr_sync_records`), `github.com/modelcontextprotocol/go-sdk` (sin cambios
de superficie MCP), CLI externo de `codebase-memory-mcp` (dependencia
opcional en runtime, no en build — se invoca por `exec.Command`, igual que
hoy)

**Storage**: SQLite embebido (`mem.db` en el store global del proyecto) —
una tabla nueva (`adr_sync_records`); `CodeImpactAnnotation` no se persiste
(vive adjunta al `content` de la memoria)

**Testing**: `testing` stdlib + `testify`, TDD (Constitución §III) — tests
primero para cada puerto/adaptador/caso de uso nuevo

**Target Platform**: mismo binario único multiplataforma (Linux/macOS/
Windows) ya existente — sin nuevas dependencias runtime para el usuario
final

**Project Type**: CLI + servidor MCP (stdio) — arquitectura hexagonal ya
establecida en el repo

**Performance Goals**: SC-002 — el guardado de una memoria no debe
aumentar de forma perceptible su tiempo de respuesta con 0, 1 o 2
proveedores configurados, ni con la sincronización de ADR activa o no
(mismo orden de magnitud que hoy: guardado síncrono en SQLite, todo lo
demás fire-and-forget)

**Constraints**: ninguna de las tres capacidades puede bloquear el hot path
de guardado/arranque de sesión (FR-002/FR-004); ninguna puede disparar
indexado del proveedor externo (FR-008); todas deben poder
activarse/desactivarse de forma independiente (FR-009)

**Scale/Scope**: un proyecto por `mem.db`, cientos de memorias típicas,
hasta un puñado de proveedores candidatos declarados (Historia 3 no asume
más de eso — no es un directorio de proveedores, es una lista de fallback)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio (constitución v1.0.0) | Cumplimiento |
|---|---|
| I. Arquitectura Hexagonal | ✅ Todo lo nuevo respeta las capas ya existentes: `domain/adr_sync.go` (puro), `application/ports/{code_graph_provider.go extendido, adr_sync_provider.go, adr_sync_repository.go}` (solo dominio), adaptadores concretos en `adapters/secondary/{codegraph,adrsync,persistence}/`, wiring en `infrastructure/container.go`. Ningún adaptador se importa desde `application`/`domain` |
| II. SQLite con SQL Directo | ✅ `adr_sync_records` vía SQL directo (`CREATE TABLE IF NOT EXISTS`, bind params), migración aditiva idempotente igual que `memory_relations`; WAL/timestamps UTC-5 heredados de la conexión ya existente |
| III. Testing First | ✅ Cada puerto nuevo (`ADRSyncProvider`, `ADRSyncRepository`) y cada extensión de puerto (`CodeGraphProvider.ImpactFor`) se desarrolla con test primero (mock de puerto → red → green); ver Fase 2 (`/speckit-tasks`) para el desglose Red-Green por historia |
| IV. Configuración y Entorno | ✅ Extiende `persistence.Settings` (una sola struct ya existente), sin nuevas variables de entorno — sigue el mismo mecanismo por-proyecto (`settings.json`) ya usado por `code_graph_disabled`/`code_graph_command` |
| V. Principios Operativos (fire-and-forget, idempotencia, simplicidad) | ✅ Export/import de ADR es fire-and-forget posterior al `commit` de la memoria (igual que `formSynapse()`); idempotencia vía índices únicos en `adr_sync_records` (igual que `GetRelationByPair` para sinapsis); Historia 3 reutiliza el loop de `build_context.go` que ya existe en vez de agregar un tipo nuevo — mínimo impacto real, no solo declarado |
| Stack congelado | ✅ Sin dependencias nuevas de terceros — todo sobre `modernc.org/sqlite` y `exec.Command` ya usados |
| Prohibiciones absolutas | ✅ Sin ORM, sin SQL concatenado, sin lógica de negocio en el adaptador CLI (vive en los casos de uso), sin config hardcodeada |

**Resultado**: PASS sin excepciones. No aplica `Complexity Tracking`.

## Project Structure

### Documentation (this feature)

```text
specs/010-codegraph-integration-evolution/
├── plan.md              # Este archivo
├── research.md          # Fase 0 — incógnitas técnicas resueltas
├── data-model.md         # Fase 1 — entidades y migración SQL
├── contracts/
│   └── cli-and-settings.md   # Fase 1 — superficie CLI/settings/MCP afectada
├── quickstart.md         # Fase 1 — guía de validación manual
└── tasks.md              # Fase 2 (/speckit-tasks — no generado por /speckit-plan)
```

### Source Code (repository root)

Proyecto único (CLI + MCP server en Go), arquitectura hexagonal ya
establecida — no aplica ninguna de las opciones de proyecto múltiple del
template. Estructura real afectada por esta feature:

```text
domain/
├── code_provider.go        # MODIFICA: CodeHotspot gana campo File (aditivo, omitempty)
└── adr_sync.go              # NUEVO: ADRSyncRecord, SyncOrigin, SyncStatus

application/
├── ports/
│   ├── code_graph_provider.go   # MODIFICA: agrega ImpactFor(filepath) a la interfaz
│   ├── adr_sync_provider.go     # NUEVO: puerto para exportar/importar ADR (habla con el proveedor)
│   └── adr_sync_repository.go   # NUEVO: puerto de persistencia de ADRSyncRecord
└── usecases/
    ├── build_context.go        # Sin cambios de contrato (sigue usando Snapshot(), ya itera N proveedores)
    ├── provider_selection.go    # NUEVO: Historia 3 — firstAvailable(providers) puro, sin nuevo tipo/puerto
    └── import_adrs.go            # NUEVO: Historia 2, SOLO dirección proveedor→gomemory — orquesta
                                    #   ADRSyncProvider.List() + ADRSyncRepository + MemoryRepository;
                                    #   es la única pieza de esta feature que sí amerita capa de
                                    #   usecase propia (coordina 3 puertos, no es "una cosa más que
                                    #   hace InsertMemory")

adapters/
├── secondary/
│   ├── codegraph/
│   │   └── codebasememory/provider.go   # MODIFICA: ImpactFor() (Historia 1) + GetDocument/UpdateDocument
│   │                                      #   (Historia 2, manage_adr) — MISMO Provider, mismo binario y
│   │                                      #   resolución de proyecto ya implementada; implementa ambos
│   │                                      #   puertos (CodeGraphProvider + ADRSyncProvider) en un solo tipo
│   └── persistence/
│       ├── memory.go                      # MODIFICA: InsertMemory gana 2 pasos best-effort, MISMO
│                                            #   choke point que ya usa formSynapse()/provenance:
│                                            #   (a) anotar impacto (Historia 1, via codeProvider.ImpactFor)
│                                            #   (b) exportar a ADR si type es architecture/decision
│                                            #   y adr_sync_enabled (Historia 2, sentido gomemory→proveedor)
│       ├── repositories.go                # MODIFICA: MemoryRepository gana campos opcionales
│                                            #   (codeProvider, adrSync) inyectados desde container.go —
│                                            #   nil-safe, mismo criterio que CodeProviders hoy
│       ├── adr_sync.go                     # NUEVO: CRUD SQL de adr_sync_records
│       ├── settings.go                     # MODIFICA: 3 campos nuevos (ver data-model.md)
│       └── db.go                           # MODIFICA: migración aditiva de adr_sync_records
├── primary/
│   └── cli/
│       ├── cmd_settings.go                # MODIFICA: flags --code-graph-providers/--adr-sync/--code-impact-annotation
│       └── cmd_adr_sync.go                # NUEVO: `mem adr-sync status` (solo lectura, no expuesto vía MCP)

infrastructure/
└── container.go            # MODIFICA: wiring de los puertos/adaptadores nuevos, feature-flags desde Settings

tests/
├── unit/            # mocks de ADRSyncProvider/ADRSyncRepository, casos de uso aislados
├── integration/     # adr_sync_records contra SQLite real, container.go con lista de proveedores fake (uno ausente + uno disponible)
└── contract/         # forma del JSON que produce/consume el adaptador CLI de manage_adr
```

**Structure Decision**: extensión directa de la arquitectura hexagonal ya
en uso (mismo layout de `domain/`, `application/{ports,usecases}/`,
`adapters/{primary,secondary}/`, `infrastructure/`) — no se introduce
ningún directorio ni patrón nuevo a nivel de proyecto, solo nuevos archivos
dentro de las carpetas ya existentes, siguiendo el mismo agrupamiento por
capa que `specs/004-code-graph/` y `specs/009-mitigacion-riesgos/` ya
establecieron como convención.

## Complexity Tracking

*Sin violaciones de la Constitution Check — tabla no aplica.*
