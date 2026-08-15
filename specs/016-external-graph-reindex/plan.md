# Implementation Plan: Reindexado dual de grafos de código + edición de huella de contexto en TUI

**Branch**: `016-external-graph-reindex` | **Date**: 2026-08-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/016-external-graph-reindex/spec.md`

## Summary

`mem index` hoy solo refresca el grafo de código propio (Go). El grafo externo multi-lenguaje
(proveedor opcional `codebase-memory-mcp`) requiere invocarse a mano por separado, y en la TUI
no hay ninguna acción equivalente. Este plan agrega una interfaz `ports.CodeGraphIndexer`
(separada de `ports.CodeGraphProvider`, que mantiene su contrato estricto de no-bloqueo) con un
único método `IndexRepository(ctx, mode) (nodes, edges int, err error)`, implementada por el
adaptador ya existente `codebasememory.Provider`. `mem index` la invoca tras el indexado nativo
(con opción `--skip-graph` para omitirla); la TUI la invoca como el primer `tea.Cmd` asíncrono
real del programa, sin bloquear la interfaz. La misma pantalla de configuración gana además tres
filas nuevas para editar Budget/CompactThreshold/DedupWindowDays vía una pantalla `screenEditSetting`
reutilizable (un solo `textinput`, parametrizado por campo). Cero cambios en `MaybeRefresh`,
`Refresh`, `Snapshot`, `probeTimeout`, `container.go` ni `deps.go`.

## Technical Context

**Language/Version**: Go >=1.22 (stack congelado del proyecto)

**Primary Dependencies**: `charmbracelet/bubbletea` (TUI), `flag` stdlib (CLI), `modernc.org/sqlite`
(persistencia de settings, sin CGO), binario externo `codebase-memory-mcp` invocado vía `exec.CommandContext`

**Storage**: `.memory/settings.json` por proyecto (lectura/escritura ya existente en
`adapters/secondary/persistence/settings.go`); sin nuevas tablas SQLite

**Testing**: `testing` stdlib + `testify`, siguiendo el patrón fake/stub ya presente en
`provider_test.go` y `tui_test.go`

**Target Platform**: CLI y TUI multiplataforma (Linux/macOS/Windows), binario autocontenido

**Project Type**: CLI + TUI (single project, arquitectura hexagonal ya establecida)

**Performance Goals**: N/A (el reindexado externo es intrínsecamente lento — minutos — el
objetivo no es velocidad sino no bloquear la TUI mientras corre)

**Constraints**: el hot path (hooks por turno, `MaybeRefresh`, `probeTimeout` de 2s) NO debe
verse afectado; el nuevo timeout (`indexTimeout` = 10 min) es exclusivo de la ruta explícita
`mem index` / acción de TUI

**Scale/Scope**: 1 interfaz nueva, 1 método nuevo en el adaptador existente, 1 flag de CLI,
4 filas nuevas en un menú de TUI ya existente, 1 pantalla de TUI nueva (reutiliza molde existente)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Arquitectura Hexagonal** — ✅ PASA. `ports.CodeGraphIndexer` es un puerto nuevo en la capa
  de aplicación; `codebasememory.Provider` (adaptador secundario existente) lo implementa. CLI y
  TUI (adaptadores primarios) dependen solo del puerto, nunca del tipo concreto. Sin wiring nuevo:
  `Deps.CodeProviders`/`Deps.TUIProvider` ya inyectan el mismo `*codebasememory.Provider`.
- **II. SQLite con SQL Directo** — ✅ N/A. No se toca SQL; la escritura de settings usa el
  repositorio JSON ya existente (`settings.go`), sin cambios de esquema.
- **III. Testing First** — ✅ PASA (a verificar en implementación). Plan explícito de tests
  unitarios para el parser de respuesta, el sentinel `ErrIndexerNotInstalled`, y los casos de TUI
  (con/sin soporte de interfaz, cancelación, validación). Ningún test existente se modifica —
  las filas de menú se agregan al final para no invalidar `tui_test.go:538,549`.
- **IV. Configuración y Entorno** — ✅ N/A. No se agregan variables de entorno nuevas.
- **V. Principios Operativos** — ✅ PASA. "Fire-and-forget" se respeta en el sentido inverso
  correcto: el hot path sigue sin bloquear; el reindexado SÍ bloquea pero solo en comandos
  explícitos que el usuario ya espera que tarden. Causa raíz atendida (el hueco real es la falta
  de un solo comando/acción), sin parches temporales.

Sin violaciones. No se requiere `Complexity Tracking`.

## Project Structure

### Documentation (this feature)

```text
specs/016-external-graph-reindex/
├── plan.md              # Este archivo
├── research.md          # Fase 0
├── data-model.md         # Fase 1
├── quickstart.md         # Fase 1
├── contracts/
│   └── code-graph-indexer.md   # Fase 1 — contrato del puerto nuevo
└── tasks.md              # Fase 2 (/speckit-tasks, no generado por /speckit-plan)
```

### Source Code (repository root)

**Structure Decision**: Proyecto único (CLI + TUI) con arquitectura hexagonal ya establecida.
No se crean paquetes nuevos; se extiende `application/ports`, el adaptador secundario existente
`adapters/secondary/codegraph/codebasememory`, y los dos adaptadores primarios `adapters/primary/cli`
y `adapters/primary/tui`.

```text
application/
└── ports/
    └── code_graph_indexer.go        # NUEVO — interfaz CodeGraphIndexer + ErrIndexerNotInstalled

adapters/
├── secondary/
│   └── codegraph/
│       └── codebasememory/
│           ├── provider.go          # MODIFICAR — método IndexRepository, runCLIWithTimeout
│           ├── provider_test.go     # MODIFICAR — tests de parser/sentinel
│           └── testdata/
│               └── index_repository.json   # NUEVO — fixture de respuesta real
└── primary/
    ├── cli/
    │   └── cmd_index.go             # MODIFICAR — flag --skip-graph, indexExternalGraph()
    └── tui/
        ├── tui.go                   # MODIFICAR — acción reindex + pantalla screenEditSetting
        └── tui_test.go              # MODIFICAR — fakes + tests nuevos
```

## Complexity Tracking

*Sin violaciones a la constitución — sección no aplica.*
