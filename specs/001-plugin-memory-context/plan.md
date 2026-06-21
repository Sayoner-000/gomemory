# Implementation Plan: Plugin de Memoria con Contexto Automático

**Branch**: `001-plugin-memory-context` | **Date**: 2026-06-21 | **Spec**: specs/001-plugin-memory-context/spec.md

**Input**: Feature specification from `specs/001-plugin-memory-context/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Transformar gomemory de un simple servidor MCP a un sistema de plugins que se
invoque en cada inferencia del agente. Tres pilares: (1) plugin para OpenCode
que inyecta el Memory Protocol en el system prompt y gestiona sesiones,
(2) plugin para Claude Code con hooks nativos y skill de memoria,
(3) protocolo de memoria con reglas estrictas de cuándo guardar, buscar y
cerrar, más revelación progresiva de 3 capas para minimizar tokens.

## Technical Context

**Language/Version**: Go 1.22+

**Primary Dependencies**: `modernc.org/sqlite`, `charmbracelet/bubbletea`,
`modelcontextprotocol/go-sdk`, más `net/http` (stdlib) para servidor HTTP de
background

**Storage**: SQLite via `modernc.org/sqlite` (WAL mode, sin CGO)

**Testing**: `testing` stdlib + `testify`

**Target Platform**: Linux, macOS, Windows (CLI binario autocontenido)

**Project Type**: CLI + TUI + MCP server + plugin system multi-agente

**Performance Goals**:
- Inyección de contexto: <200 tokens adicionales por inferencia
- Servidor HTTP background: <10MB RAM, <0.5% CPU en idle
- Recuperación post-compactación: <2 interacciones
- Reducción de tokens de contexto: ≥60% vs volcado completo de memoria

**Constraints**:
- Zero dependencias runtime (binario autocontenido ~16MB)
- Sin CGO (SQLite puro Go)
- Sin frameworks de DI
- Documentación en español latino
- Timestamps UTC-5 (Bogotá, sin DST)

**Scale/Scope**: Single-user por proyecto, hasta 6 agentes por proyecto
(OpenCode, Claude, Cursor, Windsurf, Cline, Codex)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principios Aplicables

| Principio | Requisito | Estado |
|-----------|-----------|--------|
| I. Arquitectura Hexagonal | Plugins como adaptadores externos; la lógica de plugin vive en capa de adaptadores/infraestructura, no contamina dominio | ✅ Cumple |
| II. SQLite con SQL Directo | Nuevos tools MCP usan SQL directo, sin ORM | ✅ Cumple |
| III. Testing First | TDD obligatorio para cada plugin, mock de servidor HTTP | ✅ Cumple |
| IV. Configuración y Entorno | Flags de setup, variables para config de plugins | ✅ Cumple |
| V. Principios Operativos | Simplicidad: plugins pequeños y enfocados. Idempotencia en setup. Fire-and-forget para captura pasiva | ✅ Cumple |
| Documentación en español | Spec, plan y artefactos en español latino | ✅ Cumple |

### Violaciones

Ninguna. El feature respeta todos los principios constitucionales. Plugins
son extensiones de infraestructura, no modifican dominio ni capas internas.

## Project Structure

### Documentation (this feature)

```text
specs/001-plugin-memory-context/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── plugin-opencode.md
│   └── plugin-claude-code.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
gomemory/
├── main.go                   # Dispatcher CLI + launchTUI() + usage()
├── cmd_init.go               # mem init [--force]
├── cmd_save.go               # mem save -t "tit" -y tipo "cuerpo"
├── cmd_mcp.go                # mem mcp — servidor MCP (tools + resources)
├── cmd_mcp_setup.go          # mem setup-mcp — configura MCP multi-agente
├── cmd_setup.go              # [NEW] mem setup [agent] — instala plugins
├── cmd_serve.go              # [NEW] mem serve [port] — servidor HTTP background
├── store/
│   └── db.go, memory.go, session.go, relation.go
├── plugin/                   # [NEW] plugins por agente
│   ├── opencode/
│   │   └── plugin.ts         # Plugin TypeScript para OpenCode
│   └── claude-code/
│       ├── .claude-plugin/plugin.json
│       ├── .mcp.json
│       ├── hooks/hooks.json
│       ├── scripts/
│       │   ├── session-start.sh
│       │   ├── session-stop.sh
│       │   ├── user-prompt-submit.sh
│       │   └── post-compaction.sh
│       └── skills/memory/SKILL.md
├── internal/
│   ├── server/               # [NEW] Servidor HTTP background
│   │   └── server.go
│   └── setup/                # [NEW] Instalador de plugins (go:embed)
│       ├── setup.go
│       ├── opencode_setup.go
│       └── claude_code_setup.go
├── docs/
│   ├── architecture.md
│   ├── PLUGINS.md            # [NEW] Documentación del sistema de plugins
│   └── MEMORY-PROTOCOL.md    # [NEW] Protocolo de memoria para agentes
├── tests/
│   ├── unit/
│   │   ├── setup_test.go
│   │   └── server_test.go
│   ├── integration/
│   │   └── plugin_integration_test.go
│   └── contract/
│       └── memory_protocol_test.go
├── AGENTS.md
├── CLAUDE.md
├── README.md
└── go.mod / go.sum
```

**Structure Decision**: Single Go project. Plugins viven en `plugin/` con
go:embed para embeberlos en el binario. Servidor HTTP en `internal/server/`.
Setup multi-agente en `internal/setup/`.

## Complexity Tracking

> Sin violaciones constitucionales. No se requiere justificación de
> complejidad adicional.
