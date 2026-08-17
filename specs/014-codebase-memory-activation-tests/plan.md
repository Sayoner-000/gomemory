# Implementation Plan: codebase-memory-mcp activation regression tests

**Branch**: `014-codebase-memory-activation-tests` | **Date**: 2026-08-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/014-codebase-memory-activation-tests/spec.md`

## Summary

Script shell de regresión que valida la activación de codebase-memory-mcp en los 4 canales de distribución (Claude Code hook, OpenCode plugin, integración AGENTS.md, subagentes). Implementa 50 checks que cubren los 3 bugs históricos (v2.3.0 tools faltantes, v2.3.1 prefijos, v2.3.3 systemMessage vs additionalContext) como barrera permanente contra regressions.

## Technical Context

**Language/Version**: Bash 5+ (script shell, no Go)

**Primary Dependencies**: `go build` (compilar binario), `python3 -m json.tool` (validación JSON), `grep` (búsqueda de strings)

**Storage**: N/A — DBs temporales creadas por el script y eliminadas al terminar

**Testing**: El script ES el test — ejecución directa con exit code 0/1

**Target Platform**: macOS / Linux (cualquier shell compatible con `set -euo pipefail`)

**Project Type**: Shell script de regresión (no es feature de negocio, es herramienta de calidad)

**Performance Goals**: <60 segundos (compilación + 50 checks)

**Constraints**: Sin dependencias externas más allá de Go, Python3 y herramientas POSIX estándar

**Scale/Scope**: 1 script, 7 secciones de test, 50 checks, 0 configuración requerida

## Constitution Check

*No constitution file exists. No gates to evaluate.*

## Project Structure

### Documentation (this feature)

```text
specs/014-codebase-memory-activation-tests/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── hook-schemas.md  # JSON schemas de los hooks
└── checklists/
    └── requirements.md  # Quality checklist
```

### Source Code (repository root)

```text
scripts/
└── test-codebase-memory-activation.sh   # Script de regresión (ya existe como implementación actual)

adapters/primary/cli/
├── cmd_hook.go              # Hook handler (valida canal Claude Code)
├── cmd_install.go           # Generador AGENTS.md (valida canal integración)
└── cmd_mcp.go               # MCP instructions (valida canal MCP)

infrastructure/plugin/opencode/
└── gomemory.ts              # Plugin OpenCode (valida canal OpenCode)

domain/
└── mcp_tools.go             # Definición de tools (CodebaseMemoryMCPDiscoveryTools)
```

**Structure Decision**: Shell script único en `scripts/`. No requiere reestructuración del proyecto existente.

## Complexity Tracking

No constitution violations. Feature is a standalone regression script with no architectural impact.
