# Project Tasks

<!-- Plan, track, and review project tasks here. -->

## Current

_No tasks in progress._

## Completed

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
