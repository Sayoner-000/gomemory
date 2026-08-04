# Implementation Plan: gomemory como brazo extensor de contexto histórico para /speckit

**Branch**: `011-gomemory-spec-context` | **Date**: 2026-08-03 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/011-gomemory-spec-context/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Conectar el resumen de historial que gomemory ya construye (`get_context` /
`mem context`: memorias por tipo, decisiones, y — si hay proveedor externo —
un resumen aparte del grafo de código) al flujo de `/speckit-specify` (y
opcionalmente `/speckit-plan`/`/speckit-clarify`), mediante una extensión
spec-kit bundleada (`gomemory-context`, calcada de la ya existente
`agent-context`) con un hook `before_specify` **mandatorio**. El hook
ejecuta un script que llama a `./mem context` (o hace fallback a la tool MCP
`get_context`), respetando un interruptor nuevo del lado de gomemory
(`SpeckitContextDisabled` en `Settings`, expuesto en la TUI) que permite
apagar la integración sin tocar la configuración de spec-kit. Sin datos, sin
gomemory disponible, o con el interruptor apagado, el script no produce
salida y el flujo de especificación continúa exactamente igual que hoy.

## Technical Context

**Language/Version**: Go 1.25 (toolchain go1.25.11) para el cambio en
gomemory (settings + TUI); Bash/PowerShell + YAML/Markdown para la
extensión spec-kit (sin lenguaje de aplicación, mismo formato que
`agent-context`).

**Primary Dependencies**: Ninguna dependencia Go nueva — se reutiliza
`charmbracelet/bubbletea`/`lipgloss` (TUI ya en `go.mod`) y
`modelcontextprotocol/go-sdk` (MCP ya en `go.mod`). La extensión spec-kit no
añade dependencias: reutiliza el mecanismo de hooks/extensions ya presente
en `.specify/` (visto en `agent-context`).

**Storage**: `.memory/settings.json` (archivo JSON existente, gestionado por
`adapters/secondary/persistence/settings.go`) — un campo booleano nuevo, sin
tabla SQLite nueva.

**Testing**: `testing` + `testify` (Go) para el campo de settings y el
toggle de TUI, siguiendo `tests/unit/` del proyecto. El script bash/
PowerShell del hook se valida con el flujo de `quickstart.md` (no hay
framework de test de spec-kit para scripts de extensión, mismo criterio que
`agent-context`, que tampoco trae tests Go).

**Target Platform**: mismo binario multiplataforma de gomemory (Linux/
macOS/Windows) + cualquier agente/IDE compatible con spec-kit que soporte
hooks de extensión (Claude Code, y en general cualquier integración listada
en `.specify/integrations/`).

**Project Type**: single project (CLI/TUI/MCP en Go, arquitectura
hexagonal) + un directorio de extensión spec-kit bundleada (config/
markdown, no es un proyecto de código separado) — mismo patrón que
`.specify/extensions/agent-context/`.

**Performance Goals**: sin demora perceptible en `/speckit-specify` (SC-002)
— una sola invocación de `mem context`, ya acotada por el `Budget` existente
(24000 caracteres por defecto), sin I/O adicional más allá de la lectura de
settings y memorias ya cacheadas.

**Constraints**: solo lectura del grafo de código externo (FR-008); salida
acotada por el `Budget` ya existente (FR-005); cero efecto perceptible si no
hay spec-kit instalado (SC-006) — el hook simplemente no existe/no se
dispara en ese caso, sin que gomemory tenga que detectarlo activamente.

**Scale/Scope**: un campo de settings nuevo, una fila nueva en la pantalla
de configuración de la TUI, un flag nuevo en `mem settings`, y un directorio
de extensión spec-kit bundleada con un hook y un comando.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Arquitectura Hexagonal** — ✅ Cumple. El campo nuevo
  (`SpeckitContextDisabled`) se agrega al puerto ya existente
  (`application/ports/settings_repository.go`) y a su adaptador
  (`adapters/secondary/persistence/settings.go`), sin nuevas capas ni
  imports cruzados. La TUI (`adapters/primary/tui`) solo lee/escribe vía el
  puerto `SettingsRepository`, igual que `CodeGraphDisabled`.
- **II. SQLite con SQL Directo** — N/A. No se toca la base de datos; el
  toggle vive en `.memory/settings.json`, mismo lugar que los toggles
  equivalentes ya existentes (`CodeGraphDisabled`, `SynapseDisabled`).
- **III. Testing First** — ✅ Aplica al código Go (settings + wiring de TUI):
  test unitario primero para `ReadSettings`/`WriteSettings` con el campo
  nuevo, y para la rama de `updateConfig` que lo alterna. El script del hook
  de spec-kit (bash/PowerShell) no tiene framework de test en este
  repositorio — se valida con `quickstart.md`, mismo criterio ya aceptado
  para `agent-context/scripts/`.
- **IV. Configuración y Entorno** — El toggle es una preferencia por
  proyecto en `settings.json`, no una variable de entorno de despliegue;
  esta distinción ya es precedente aceptado en el proyecto para
  `CodeGraphDisabled`/`AdrSyncEnabled`/`SynapseDisabled` — no aplica la
  regla de "toda config por env var" a preferencias de usuario por
  proyecto.
- **V. Principios Operativos** — ✅ Simplicidad (reutiliza `get_context`/
  `mem context` sin construir un mecanismo nuevo de resumen); sin parches
  temporales; degradación transparente ya es el comportamiento por defecto
  de los proveedores opcionales existentes.

Sin violaciones — no se requiere `Complexity Tracking`.

## Project Structure

### Documentation (this feature)

```text
specs/011-gomemory-spec-context/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Extensión spec-kit bundleada (nueva), calcada de .specify/extensions/agent-context/
.specify/extensions/gomemory-context/
├── extension.yml                              # hooks.before_specify (optional: false)
├── README.md                                  # opt-out, config, requisitos
├── commands/
│   └── speckit.gomemory-context.update.md     # instrucciones del comando del hook
└── scripts/
    ├── bash/update-gomemory-context.sh         # gate settings + `./mem context`
    └── powershell/update-gomemory-context.ps1  # paridad Windows

# Cambios Go (gomemory), siguiendo el patrón ya usado por CodeGraphDisabled
application/ports/settings_repository.go        # +SpeckitContextDisabled bool
adapters/secondary/persistence/settings.go       # +campo, +default
adapters/secondary/persistence/repositories.go   # +mapping puerto↔adaptador
adapters/primary/tui/tui.go                      # +1 fila en pantalla de configuración
adapters/primary/cli/cmd_settings.go             # +flag --speckit-context
```

**Structure Decision**: monorepo Go existente (hexagonal, sin proyectos
nuevos) más un directorio de extensión spec-kit bundleada — mismo patrón ya
usado por `agent-context`, así que no hay una capa/estructura nueva que
introducir; el trabajo es aditivo sobre puntos de extensión existentes en
ambos lados (settings/TUI de gomemory, extensions/hooks de spec-kit).

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

Sin violaciones — tabla no aplica.
