# Implementation Plan: Evaluación de la base Go para vertical slices en arquitectura hexagonal

**Branch**: `043-vertical-slices-assessment` (sin rama propia; se trabaja sobre `main`) | **Date**: 2026-10-25 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/043-vertical-slices-assessment/spec.md`

## Summary

Producir un informe de evaluación, de solo lectura, que diga si la base Go necesita refactorizarse para desarrollar por vertical slices dentro de la arquitectura hexagonal declarada. El enfoque es empírico: primero se fija una línea base de git y se declaran los criterios de evaluación ([contracts/evaluation-criteria.md](contracts/evaluation-criteria.md)); después se obtiene el grafo real de importaciones entre capas con `go list`, se recorren tres slices representativos (Socratica, Review consensuado e Instincts) desde sus puntos de entrada hasta persistencia, y cada observación se clasifica como hallazgo, preferencia o práctica a conservar con evidencia `archivo:línea` o un comando reproducible. La conclusión elige una de tres opciones y, solo si recomienda cambios, incluye un roadmap priorizado. El informe sigue el contrato [contracts/report-contract.md](contracts/report-contract.md) y se entrega en la conversación o fuera del repositorio.

## Technical Context

**Language/Version**: Go 1.27 (`go.mod`: `go 1.27.0`, toolchain `go1.27.3`), módulo `mem`.

**Primary Dependencies**: solo herramientas de lectura y análisis ya disponibles: `go list`, `go vet`, `go test`, `git`, CodeGraph (`codegraph_explore`) y los grafos de gomemory/codebase-memory. No se añade ninguna dependencia.

**Storage**: N/A para la evaluación. El objeto evaluado usa SQLite (`adapters/secondary/persistence`).

**Testing**: la evaluación no escribe pruebas. Usa `go test ./...` y `go vet ./...` como evidencia de testabilidad y comprueba que el estado de git no cambia (FR-010).

**Target Platform**: repositorio local en macOS (darwin/arm64). Los comandos de evidencia son portables a Linux, donde corre la CI.

**Project Type**: CLI con servidor MCP y TUI (binario `mem`, composition root en `infrastructure/`).

**Performance Goals**: N/A (fuera de alcance según la spec).

**Constraints**: solo lectura (git status idéntico al terminar); toda salida intermedia va al scratchpad de la sesión, fuera del repositorio; cada afirmación verificada o marcada como no verificada (FR-011); sin red obligatoria (si una comprobación la necesita y falla, se marca como no verificada).

**Scale/Scope**: medido el 2026-10-25: 30 paquetes Go, 898 archivos `.go` (528 de prueba). Tamaño de producción por capa: `domain` 75 archivos / 11.417 líneas; `application/ports` 45 / 1.427; `application/usecases` 48 / 7.931; `adapters/primary/cli` 93 / 20.154 (57 archivos `cmd_*`); `adapters/primary/tui` 5 / 4.464; `adapters/secondary/persistence` 32 / 9.491; `infrastructure` 2 / 767. Hay 18 paquetes en `adapters/secondary` y 4 en `adapters/primary`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Regla de la constitución | Aplica a esta feature | Estado |
|---|---|---|
| §3 Arquitectura hexagonal obligatoria (tabla de dependencias por capa) | Es el **criterio de referencia** de la evaluación (FR-012), no algo que esta feature implemente | PASS: se usa como regla L1–L4 en [contracts/evaluation-criteria.md](contracts/evaluation-criteria.md) |
| §3 "Reglas verificadas en CI con reglas de paquetes en Go" | Se comprueba en el objeto evaluado como hallazgo potencial | PASS para la feature; resultado de la comprobación en [research.md](research.md) R5 |
| §4 Composition root único | Criterio M3 | PASS (criterio) |
| §10 Pruebas | Criterios T1–T3; la feature no añade código ni pruebas | PASS |
| §16 Calidad y commits | La evaluación no hace commits; los artefactos de `specs/043` se commitean solo con aprobación | PASS |
| §18 Documentación y especificaciones | La spec y el plan viven en `specs/043`; el informe no modifica `docs/ARQUITECTURA.md` | PASS |
| Reglas de trabajo del proyecto (memoria fijada): "reproduce la realidad", "tests verdes no bastan", "cierra los hallazgos" | Exigen evidencia ejecutada y propuesta de cierre por hallazgo | PASS: incorporadas en FR-011 y en el campo `cierre` del hallazgo ([data-model.md](data-model.md)) |

No hay violaciones que justificar. **Re-check post-diseño (tras la Fase 1)**: PASS, porque el diseño no introduce código, dependencias ni cambios en el repositorio fuera de `specs/043`.

## Project Structure

### Documentation (this feature)

```text
specs/043-vertical-slices-assessment/
├── spec.md                       # Especificación (/speckit-specify)
├── plan.md                       # Este archivo
├── research.md                   # Fase 0: decisiones de método y verificaciones previas
├── data-model.md                 # Fase 1: entidades del informe
├── quickstart.md                 # Fase 1: guía para ejecutar y validar la evaluación
├── contracts/
│   ├── evaluation-criteria.md    # Criterios declarados antes de aplicarse (FR-006)
│   └── report-contract.md        # Estructura obligatoria del informe
├── checklists/
│   └── requirements.md           # Calidad de la spec
└── tasks.md                      # Fase 2 (/speckit-tasks; no lo crea este comando)
```

### Source Code (repository root)

Esta feature **no modifica código**. Estas son las rutas reales que se evalúan:

```text
domain/                         # Entidades y reglas puras
application/
├── ports/                      # Interfaces hacia el exterior (45 archivos)
└── usecases/                   # Casos de uso (48 archivos)
adapters/
├── primary/
│   ├── cli/                    # Puntos de entrada: comandos mem, hooks, servidor MCP
│   ├── console/                # Presentación de terminal
│   ├── setup/                  # Instalación e integración con agentes
│   └── tui/                    # Interfaz Bubble Tea
└── secondary/                  # 18 adaptadores: persistence, codegraph, generator, launcher, speckit, usage…
infrastructure/
├── main.go                     # Binario mem
└── container.go                # Composition root
tests/
├── integration/
└── contract/
```

**Structure Decision**: estructura de proyecto único, ya existente. La evaluación la recorre sin alterarla. Los slices de muestra (que resuelven el pendiente P1) se fijan en [research.md](research.md) R2.

## Complexity Tracking

Sin violaciones de la constitución; no aplica.
