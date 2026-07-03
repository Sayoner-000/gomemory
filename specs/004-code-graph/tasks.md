# Tasks: Grafo de Código (Fase 1)

Todas las tareas están **[X] completas** — implementadas y testeadas en esta
sesión, siguiendo el orden Fase 0 → W1 → W3 → W2 del plan de trabajo.

## Dominio y puerto

- [X] T001 `domain/code.go` — `CodeNodeKind`, `CodeEdgeKind`, `CodeNode`, `CodeEdge`, `GraphStatus`, `PackageStat`
- [X] T002 `application/ports/code_graph_repository.go` — puerto `CodeGraphRepository`
- [X] T003 `application/ports/context_builder.go` — puerto `GraphStatusQuerier` (subset de solo lectura para `get_context`)

## Esquema y persistencia

- [X] T004 `adapters/secondary/persistence/db.go` — tablas `code_files`/`code_nodes`/`code_edges` + índices en `migrate()`
- [X] T005 `adapters/secondary/persistence/db.go` — `code_search` (FTS5) best-effort, separado del schema principal
- [X] T006 `adapters/secondary/persistence/code_graph.go` — `ReplaceFileNodes`, `DeleteCodeFile`, `InsertCodeEdges`, `SearchCodeNodes` (FTS5 + fallback LIKE), `NodesByName`, `UpsertPackageNode`, `Neighbors` (BFS), `CodeGraphStatus`
- [X] T007 `adapters/secondary/persistence/repositories.go` — `CodeGraphRepository` (wrapper que implementa el puerto)
- [X] T008 `adapters/secondary/persistence/code_graph_test.go` — CRUD, idempotencia de reindex, `DeleteCodeFile`, búsqueda, BFS por dirección/profundidad

## Indexador

- [X] T009 `application/usecases/goparse.go` — `parseGoFile`: extrae package/imports/func/method/type/calls vía `go/ast`
- [X] T010 `application/usecases/index_project.go` — `Indexer.IndexProject` (walk + hash-diff + 2 pasadas) e `IndexFiles` (incremental)
- [X] T011 `application/usecases/index_project_test.go` — extracción de símbolos, resolución same-package y selector cross-package, aristas IMPORTS, skip por hash, borrado de archivos ausentes, indexado incremental

## Wiring

- [X] T012 `adapters/primary/cli/deps.go` + `infrastructure/container.go` — `CodeGraphRepo` en `Deps`/`Container`, `Builder.Graph` seteado
- [X] T013 `application/usecases/build_context.go` — sección `## Código indexado` en `get_context` (nil-safe)

## CLI y MCP

- [X] T014 `adapters/primary/cli/cmd_index.go` — `mem index [--force]`, registrado en dispatcher + `Usage()`
- [X] T015 `adapters/primary/cli/cmd_mcp_code_tools.go` — tools `index_project`, `graph_status`, `search_code`, `get_symbol`, `list_dependencies`
- [X] T016 `adapters/primary/setup/claude_code_setup.go` — 5 tools nuevas en `ClaudeAutoAllowTools`

## Integración con hooks

- [X] T017 `adapters/primary/cli/cmd_hook.go` — `reindexTouchedGoFiles` en `hookTurnEnd`, best-effort, normaliza rutas absolutas→relativas

## Documentación

- [X] T018 `specs/004-code-graph/{spec,plan,tasks}.md` — este spec

## Notas de implementación

- Verificado manualmente que `modernc.org/sqlite` (v1.52.0, la versión
  pineada en `go.mod`) soporta `CREATE VIRTUAL TABLE ... USING fts5(...)`
  sin flags de build adicionales — no se necesitó el fallback documentado en
  el spec, pero el código lo tiene igual por robustez ante otras builds.
- La resolución de `CALLS` es intencionalmente best-effort (sin
  `go/types`) — ver "Alcance de la Fase 1" en `spec.md` para las
  limitaciones conocidas.
