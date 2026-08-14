# Plan de Implementación: Context Optimization & Budgeting

**Branch**: `main` (sin rama dedicada — no se registró hook `before_specify` de creación
de rama para esta feature) | **Fecha**: 2026-08-14 | **Spec**: [spec.md](./spec.md)

**Input**: Especificación de la feature en `specs/015-context-optimization/spec.md`

## Resumen

Extender gomemory con un motor de optimización de contexto: dado una tarea y un
presupuesto de tokens, recupera memorias relevantes del proyecto (y, opcionalmente,
artefactos de la feature activa de Spec Kit), elimina duplicados, clasifica cada ítem
en crítico/relevante/opcional, comprime de forma determinista lo no crítico, y arma un
`ContextPack` que nunca excede el presupuesto ni descarta contenido crítico en
silencio. Se expone vía un caso de uso Go puro, un comando CLI nuevo (`mem pack`) y
tools MCP nuevas — reusando al máximo lo que ya existe en el repo (`Search` FTS5,
`DetectDuplicateGroups`, `SettingsData`) en vez de introducir mecanismos paralelos.
Enfoque técnico completo en [research.md](./research.md).

## Contexto Técnico

**Lenguaje/Versión**: Go >=1.22 (stack ya congelado, sin cambios)

**Dependencias Principales**: Ninguna dependencia externa nueva obligatoria — stdlib +
reuso de `MemoryRepository.Search` (FTS5), `DetectDuplicateGroups`, `SettingsData` ya
existentes en el repo

**Almacenamiento**: SQLite existente (`modernc.org/sqlite`, `.memory/memory.db`), sin
migraciones nuevas — `ContextPack` es efímero, no se persiste (research.md §6)

**Testing**: `testing` stdlib + `testify`, TDD Red-Green-Refactor obligatorio,
`tests/unit` + `tests/integration` + `tests/contract`

**Plataforma Objetivo**: binario Go multiplataforma existente (Linux/macOS/Windows),
CLI + servidor MCP sobre stdio — agnóstico de agente y de proveedor de LLM (FR-014,
ver contracts/mcp-tools.md "Regla de oro")

**Tipo de Proyecto**: extensión del módulo hexagonal único ya existente (no un
proyecto/módulo nuevo, no una reestructuración)

**Objetivos de Rendimiento**: SC-007 — menos de 100ms de procesamiento añadido sobre
las llamadas de retrieval ya existentes, para proyectos de tamaño típico

**Restricciones**: determinismo de la compresión estructural (SC-006); nunca soltar
contenido crítico sin error explícito (FR-008); aditivo/opt-in, cero cambio de
comportamiento existente si no se invoca (FR-018/SC-005); cero dependencias de IA
obligatorias; documentación en español latino (Constitución); 120 columnas
`gofumpt`; cobertura ≥80% (Constitución, Principio III)

**Escala/Alcance**: escala de un proyecto gomemory típico (cientos a pocos miles de
memorias en SQLite local); 5 historias de usuario P1→P5, MVP = Historia 1 sola

## Constitution Check

*GATE: debe pasar antes de la Fase 0. Se re-verifica después del diseño de Fase 1.*

| Principio | Verificación | Estado |
|---|---|---|
| I. Arquitectura Hexagonal | Dominio nuevo puro (`domain/context_pack.go`), puertos nuevos (`Compressor`, `TokenCounter`, `SpecKitReader`) con adaptadores en `adapters/secondary/`, caso de uso en `application/usecases/` que solo importa dominio+puertos. Sin imports de infraestructura en dominio/aplicación. | PASS |
| II. SQLite con SQL Directo | Sin tablas nuevas, sin migraciones (research.md §6). El único acceso a SQLite es vía el `MemoryRepository.Search` ya existente, ya escrito con SQL directo y bind params. | PASS |
| III. Testing First (TDD) | Todo el pipeline (`BuildContextPack`, `DetectDuplicateGroups` reusado, compresión estructural, conteo de tokens) es lógica pura, testeable con tests de tabla sin mocks pesados; se exige Red-Green-Refactor en tasks.md. | PASS (a verificar en implementación) |
| IV. Configuración y Entorno | Los ajustes configurables (FR-019) se agregan a la única `SettingsData` existente (`.memory/settings.json`), mismo patrón opt-out `...Disabled` que `SpeckitContextDisabled`/`AtomicPlanDisabled`. No se introduce un archivo de configuración paralelo (research.md §7, rechaza el YAML `context:` del brief original). | PASS |
| V. Principios Operativos | #1 Simplicidad: se reusan 3 mecanismos ya existentes (Search, dedup, settings) en vez de reinventarlos. #5 Fallar rápido: `ErrInvalidContextRequest` se valida en el borde antes de tocar puertos. #7 Idempotencia: `BuildContextPack` es una función pura de su input, mismo request → mismo pack (sin efectos secundarios de escritura). #9 MCP como integración primaria: tools MCP nuevas son ciudadanas de primera clase, no un añadido opcional (contracts/mcp-tools.md). | PASS |
| Documentación en español latino | Todos los artefactos de esta carpeta (`spec.md`, `plan.md`, `research.md`, `data-model.md`, `quickstart.md`, `contracts/*.md`) están en español latino, sin voseo, términos técnicos en inglés donde no hay traducción clara. | PASS |

Ninguna violación que requiera `Complexity Tracking`.

## Estructura del Proyecto

### Documentación (esta feature)

```text
specs/015-context-optimization/
├── plan.md              # Este archivo (/speckit-plan)
├── research.md          # Fase 0 (/speckit-plan)
├── data-model.md         # Fase 1 (/speckit-plan)
├── quickstart.md         # Fase 1 (/speckit-plan)
├── contracts/            # Fase 1 (/speckit-plan)
│   ├── cli.md
│   ├── go-api.md
│   └── mcp-tools.md
├── checklists/
│   └── requirements.md   # Ya generado por /speckit-specify
└── tasks.md              # Fase 2 (/speckit-tasks — NO se crea en este comando)
```

### Código fuente (raíz del repositorio)

Extensión del layout hexagonal ya existente (módulo `mem`) — no se introduce ningún
directorio de primer nivel nuevo:

```text
domain/
└── context_pack.go          # ContextItem, ContextPack, ContextStats, Priority,
                              # ErrCriticalContextOverflow, ErrInvalidContextRequest

application/
├── ports/
│   ├── compressor.go         # Compressor, CompressionOptions, CompressionResult
│   ├── token_counter.go      # TokenCounter
│   └── speckit_reader.go     # SpecKitReader
└── usecases/
    ├── build_context_pack.go       # BuildContextPack (orquesta el pipeline completo)
    ├── build_context_pack_test.go
    ├── optimize_tool.go             # OptimizeToolDescription (Historia 5)
    └── optimize_tool_test.go

adapters/
├── secondary/
│   ├── compression/
│   │   ├── noop.go
│   │   ├── structural.go
│   │   └── structural_test.go
│   ├── tokens/
│   │   ├── approximate.go
│   │   └── approximate_test.go
│   └── speckit/
│       ├── reader.go
│       └── reader_test.go
└── primary/
    ├── cli/
    │   ├── cmd_pack.go              # mem pack build|show|compress|stats
    │   └── cmd_pack_test.go
    └── (registro de tools MCP en los archivos cmd_mcp*.go ya existentes)

domain/mcp_tools.go           # agrega ToolPackBuild/Show/Compress/Stats + MCPContextPackTools

tests/
├── unit/                     # tests de tabla para compresión/tokens/clasificación
├── integration/              # BuildContextPack contra SQLite real + fixture de memorias
└── contract/                 # MCPAllTools() vs tools/list real; JSON de pack_build vs contracts/mcp-tools.md
```

**Decisión de estructura**: Opción "proyecto único" (no aplica la opción de app web ni
mobile+API del template genérico) — esta feature vive enteramente dentro del módulo Go
hexagonal ya existente, siguiendo el mismo patrón de capas que `build_context.go` /
`build_plan_context.go` (Fase 0, research.md §1).

## Complexity Tracking

*Sin violaciones de la Constitución — tabla vacía a propósito.*

| Violación | Por qué se necesita | Alternativa más simple descartada porque |
|---|---|---|
| — | — | — |
