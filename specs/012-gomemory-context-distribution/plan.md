# Implementation Plan: Distribuir el brazo extensor gomemory-context vía `mem install`, transversal a agentes

**Branch**: `012-gomemory-context-distribution` | **Date**: 2026-08-03 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/012-gomemory-context-distribution/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

`mem install` debe dejar el brazo extensor hacia spec-kit (spec 011) listo
de fábrica, sin depender de la CLI de terceros `specify`: cuando el
proyecto destino ya tiene `.specify/` inicializado, copia el árbol fuente
de la extensión `gomemory-context` a `.specify/extensions/gomemory-context/`
y, además, los dos artefactos ya traducidos que Claude Code y OpenCode
reconocen directamente (`.claude/skills/speckit-gomemory-context-update/SKILL.md`
y `.opencode/commands/speckit.gomemory-context.update.md`). Los tres
árboles se embeben en el binario con `go:embed` (mismo mecanismo que
`speckit-constitution-gen.md`) y se copian con la misma lógica ya
verificada en producción para el plugin de OpenCode: solo se reescribe un
archivo si su contenido difiere del embebido — así versiones futuras
propagan correcciones sin pisar en falso archivos ya idénticos.

## Technical Context

**Language/Version**: Go 1.25 (toolchain go1.25.11) — mismo módulo, sin
lenguaje nuevo.

**Primary Dependencies**: Ninguna nueva. Reutiliza `embed` (stdlib, ya
usado en `infrastructure/main.go` para `templatesFS`/`pluginFS`) y la
lógica de copia diff-aware ya implementada en
`adapters/primary/setup/setup.go` (`InstallPlugin`/`copyFileOrDir`).

**Storage**: N/A — no hay persistencia nueva; solo archivos de proyecto
(igual que la constitución embebida).

**Testing**: `testing` + `testify` (Go), en `tests/unit/` o junto al
paquete `adapters/primary/setup/` — TDD para la función de instalación de
la extensión (casos: sin `.specify/` → no-op; con `.specify/` → los 3
árboles quedan copiados; contenido idéntico → no se reescribe; el script
bash queda con permiso de ejecución).

**Target Platform**: mismo binario multiplataforma de gomemory (Linux/
macOS/Windows), ejecutándose sobre cualquier proyecto destino que tenga
spec-kit inicializado.

**Project Type**: single project (Go, hexagonal) — aditivo sobre
`adapters/primary/cli/cmd_install.go` y un helper nuevo en
`adapters/primary/setup/`.

**Performance Goals**: sin impacto perceptible en `mem install` — copiar
~7 archivos de texto pequeños (el más grande es el script bash, <2 KB) es
del orden de milisegundos, ya dominado por el resto del instalador
(escritura de AGENTS.md/CLAUDE.md, config MCP de 6 agentes).

**Constraints**: cero efecto en proyectos sin `.specify/` (FR-004); cero
dependencia de la CLI `specify` (FR-007); no debe alterar el contrato de
comportamiento del script del hook ya definido en la spec 011 (FR-008).

**Scale/Scope**: un helper nuevo (~60-80 líneas Go), tres árboles de
archivos estáticos embebidos (el ya existente `.specify/extensions/
gomemory-context/` más los dos artefactos por agente), una llamada nueva
en `cmd_install.go`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Arquitectura Hexagonal** — ✅ Cumple. El helper nuevo vive en
  `adapters/primary/setup/` (junto a `InstallOpenCode`/`InstallClaudeCode`,
  el mismo tipo de responsabilidad: adaptador primario que escribe
  artefactos de configuración en el filesystem del proyecto destino). No
  toca dominio ni aplicación. `cmd_install.go` solo lo invoca, como ya hace
  con el resto de pasos de instalación.
- **II. SQLite con SQL Directo** — N/A. No hay persistencia en base de
  datos involucrada.
- **III. Testing First** — ✅ Aplica: el helper de copia es lógica Go pura
  y testeable (dado un `embed.FS` de prueba y un directorio temporal,
  verificar cada rama). Se escribe el test primero, seguido de la
  implementación, igual que el resto del proyecto.
- **IV. Configuración y Entorno** — N/A directo: no hay variables de
  entorno nuevas: la decisión de copiar o no depende únicamente de si
  `.specify/` existe en el proyecto destino, no de configuración de
  gomemory.
- **V. Principios Operativos** — ✅ Simplicidad: se reutiliza la lógica de
  copia diff-aware ya existente (`InstallPlugin`/`copyFileOrDir`) en vez de
  escribir una nueva; ✅ Sin parches temporales: se resuelve la causa raíz
  (nada distribuye la extensión) en vez de documentar instrucciones
  manuales; ✅ Idempotencia: correr `mem install` dos veces seguidas no
  produce cambios en la segunda corrida (ver FR-006).

Sin violaciones — no se requiere `Complexity Tracking`.

## Project Structure

### Documentation (this feature)

```text
specs/012-gomemory-context-distribution/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md         # Phase 1 output (/speckit.plan command)
├── quickstart.md         # Phase 1 output (/speckit.plan command)
├── contracts/            # Phase 1 output (/speckit.plan command)
└── tasks.md              # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Plantillas embebidas nuevas (fuente de verdad versionada en el repo,
# empaquetada en el binario vía go:embed all:templates, mismo mecanismo
# que infrastructure/templates/speckit-constitution-gen.md)
infrastructure/templates/gomemory-context/
├── extension/                          # espejo exacto de .specify/extensions/gomemory-context/
│   ├── extension.yml
│   ├── README.md
│   ├── commands/speckit.gomemory-context.update.md
│   └── scripts/
│       ├── bash/update-gomemory-context.sh
│       └── powershell/update-gomemory-context.ps1
├── claude/
│   └── speckit-gomemory-context-update/SKILL.md   # artefacto ya traducido (spec-kit CLI, verificado en vivo)
└── opencode/
    └── speckit.gomemory-context.update.md          # artefacto ya traducido (spec-kit CLI, verificado en vivo)

# Cambios Go
adapters/primary/setup/speckit_extension.go          # nuevo: InstallSpeckitExtension(root string, templatesFS fs.FS) error
                                                        #   — 3 llamadas a InstallPlugin() ya existente, sin refactor
adapters/primary/setup/speckit_extension_test.go     # nuevo: tests (TDD)
adapters/primary/cli/cmd_install.go                    # +1 llamada, junto al paso 4b (copia de constitución),
                                                        #   pasando su propio TemplatesFS ya existente
# infrastructure/main.go NO cambia: gomemory-context/ ya queda cubierto por
# la directiva "go:embed all:templates" existente (ver contracts/install-speckit-extension.md)
# setup.go NO cambia: InstallPlugin()/copyFileOrDir() ya sirven tal cual
# (nil como PluginContext, sin placeholders que sustituir en estos archivos)
```

**Structure Decision**: monorepo Go existente (hexagonal, sin proyectos
nuevos). El trabajo es aditivo sobre puntos de extensión ya existentes
(`infrastructure/templates/` embebido, `adapters/primary/setup/` para
instaladores de agente, `cmd_install.go` como orquestador) — mismo patrón
ya usado por la constitución y por el plugin de OpenCode, sin introducir
mecanismos nuevos.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

Sin violaciones — tabla no aplica.
