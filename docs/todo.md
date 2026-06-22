# Project Tasks

<!-- Plan, track, and review project tasks here. -->

## Current

_No tasks in progress._

## Completed

### 2026-06-22 — Fix instalación multi-agente + Memory Protocol en AGENTS.md/CLAUDE.md + v1.3.0

- **Bug**: `session-start.sh` y `plugin/opencode/plugin.ts` invocaban el binario como `mem` literal (sin `PATH`) en vez de usar `{{BIN_PATH}}` → `command not found` al disparar el hook
- **Bug**: hooks de Claude Code registrados como `PostStartup`/`PreShutdown` (eventos inexistentes) en `claude_code_setup.go` y `hooks/hooks.json` → corregido a `SessionStart`/`SessionEnd`
- **Bug**: `//go:embed plugin` sin prefijo `all:` excluía `.claude-plugin/plugin.json` y el `.mcp.json` del template — nunca se instalaban a pesar de estar documentados
- **Bug de uso**: `mem setup <agent> --port N` no respeta `--port`/`--target` si el agente (posicional) va antes de los flags — limitación del paquete `flag` de Go; documentado el orden correcto (`mem setup --port N <agent>`) en `cli.go`, `cmd_setup.go` y todos los docs
- **Feature**: agregado bloque "Memory Protocol" (triggers de `save_memory`/`search_memories`/`end_session`) a `AGENTS.md` y `CLAUDE.md` de este repo
- **Docs**: `AGENTS.md` tenía la arquitectura de OTRO proyecto (Python/FastAPI/Jinja2) y una sección `graphify` sin uso — reemplazado por el resumen real de gomemory (Go hexagonal)
- **Docs**: corregidas referencias a `.memory/plugins/claude-code/` (ruta real: `.claude/plugins/gomemory/`) en README.md y docs/MANUAL.md
- Versión bump: `1.0.0` → `1.3.0` (README, INSTALLATION, plugin.json, MCP server Implementation)

### 2026-06-18 — Nuevas funcionalidades: Capture, Compare, Project + Documentación completa

**Implementación:**

- **`types/types.go`**: Agregados `RelationType` con 6 constantes (`related`, `compatible`, `scoped`, `conflicts_with`, `supersedes`, `not_conflict`), `ValidRelationType()`, y struct `Relation`
- **`store/db.go`**: Migración para tabla `memory_relations` con FK a `memories(id)` e índices
- **`store/relation.go`**: CRUD completo — `InsertRelation`, `UpdateRelation`, `GetRelation`, `GetRelationByPair` (búsqueda bidireccional), `ListRelations`
- **`cmd_capture.go`**: `mem capture` con flags `--what/-w`, `--why/-y`, `--where/-f`, `--learned/-l`, `--type/-t`, `--interactive/-i`. Formato estructurado `**What**/**Why**/**Where**/**Learned**`
- **`cmd_compare.go`**: `mem compare <id1> <id2> -r <rel> -c <conf> -m <razón>` con persistencia idempotente + `mem compare list [-n N]`
- **`cmd_project.go`**: `mem project` read-only — muestra nombre, raíz, ruta BD, conteo de memorias, sesión activa
- **`main.go`**: Routers para `capture`, `compare`, `project` + usage actualizada

**Documentación completa:**

- **README.md**: Agregados capture, compare, project a tabla de comandos + secciones explicativas
- **docs/architecture.md**: Componentes 7-9 (Capture, Compare, Project), tabla `memory_relations`, file tree actualizado, 3 nuevas decisiones técnicas
- **AGENTS.md**: File tree + comandos + flujo de trabajo actualizados
- **CLAUDE.md**: Sincronizado con AGENTS.md

## Guidelines

- Each task should be a single verifiable unit of work
- Mark [x] when done, include test/demo evidence
- Review at end of session
