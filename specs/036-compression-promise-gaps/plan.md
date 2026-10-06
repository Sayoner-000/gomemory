# Implementation Plan: 036-compression-promise-gaps

**Branch**: `036-compression-promise-gaps` | **Date**: 2026-10-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/036-compression-promise-gaps/spec.md`

## Summary

Cerrar las brechas entre la promesa de ahorro y lo medido (auditoría [378], memoria del proyecto): documentar el ahorro por vía con cifras fechadas y reproducibles (US1), hacer veraz el estado del hook de OpenCode en el diagnóstico (US2), documentar el límite de Codex sin tocar su comportamiento (US3) y ayuda organizada por flujos con `pack` visible (US4 — **ya implementada y verde**, se registra como hecha).

Enfoque técnico: solo textos (documentación + cadenas del diagnóstico y la ayuda). Sin cambios de motor, umbrales, esquemas JSON ni comportamiento de hooks. Cada cambio de cadena lleva su prueba de contrato.

## Technical Context

**Language/Version**: Go 1.27.x (línea base del repo)

**Primary Dependencies**: Ninguna nueva. Se tocan `adapters/primary/cli` (diagnóstico), `docs/MANUAL.md`, `INSTALLATION.md`.

**Storage**: N/A (sin cambios de esquema ni migraciones).

**Testing**: `go test` — paquetes `adapters/primary/cli` y `tests/contract` (patrón existente: `TestUsage_MencionaReview`, `TestToolOutputHookStates_CodexSoloEnOrigen`).

**Target Platform**: CLI multiplataforma existente (darwin/linux); sin cambios de plataforma.

**Project Type**: CLI + plugin OpenCode (solo se cambia una cadena que el plugin no lee; sin cambios en `infrastructure/plugin/opencode/gomemory.ts`).

**Performance Goals**: N/A (cambios de texto; el diagnóstico no añade E/S).

**Constraints**: Compatibilidad de `mem doctor --json`: la clave `compression.tool_output_hooks.opencode` mantiene clave y tipo (string); solo cambia el valor legible. Los tests de preservación del hook (código, lecturas exactas, diagnósticos) no se tocan (FR-007).

**Scale/Scope**: 4 historias; US4 ya implementada. Archivos a tocar: `cmd_doctor_compression.go` (1 línea + test), `docs/MANUAL.md` (§10, §16, sección de hooks), `INSTALLATION.md` (sección de ahorro).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Sin dependencias nuevas ni versiones `latest`: PASS (no aplica; cero dependencias).
- Sin secretos ni credenciales en textos: PASS (las cifras son agregadas, sin contenido de memorias).
- Cambio de comportamiento cubierto por pruebas: PASS — US2 añade aserción al test de estados existente; US4 ya trae `TestUsage_MencionaPackYFlujos`; US1/US3 son documentación verificada por revisión del quickstart.
- Compatibilidad de salidas: PASS con nota — el valor string de `tool_output_hooks.opencode` en `--json` cambia (de estado conservador a estado veraz); clave y tipo intactos. Se documenta en research R1.
- Regla operativa del proyecto "tests verdes no bastan": la validación incluye binario fresco (`go build` + `mem doctor` + `mem --help` reales), registrado en quickstart.

Re-check post-Phase 1: sin violaciones; no se añade nada a Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/036-compression-promise-gaps/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── doctor-opencode-hook.md
├── checklists/
│   └── requirements.md  # Spec quality checklist (done, 16/16)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
adapters/primary/cli/
├── cli.go                        # US4 HECHO — Usage() por flujos (no tocar salvo regresión)
├── cmd_doctor_compression.go      # US2 — 1 línea en toolOutputHookStates + test
├── cmd_doctor_compression_codex_test.go  # US2 — extender con aserción opencode
docs/
├── MANUAL.md                     # US1/US3 — §10 pack, §16 usage, hooks, tabla por vía
INSTALLATION.md                    # US1 — sección de ahorro: tabla por vía + reproducción
tests/contract/
├── mcp_tool_sync_test.go         # US4 HECHO — TestUsage_MencionaPackYFlujos (no tocar)
```

**Structure Decision**: Se reutiliza la estructura existente (single project Go). Sin directorios nuevos en código; solo artefactos de spec.

## Complexity Tracking

> No hay violaciones del Constitution Check que justificar.
