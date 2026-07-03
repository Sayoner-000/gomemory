# Implementation Plan: Grafo de Código (Fase 1)

**Spec**: [spec.md](./spec.md) · **Tasks**: [tasks.md](./tasks.md)

## Arquitectura (hexagonal, mismo patrón que memorias/relaciones)

```
domain/code.go                                  — CodeNode, CodeEdge, GraphStatus (entidades puras)
application/ports/code_graph_repository.go      — puerto CodeGraphRepository
application/usecases/goparse.go                 — parseo go/ast de un archivo → nodos/imports/calls pendientes
application/usecases/index_project.go           — Indexer: walk, hash-diff, dos pasadas (nodos → aristas)
adapters/secondary/persistence/code_graph.go    — funciones libres sobre *sql.DB (estilo memory.go/relation.go)
adapters/secondary/persistence/repositories.go  — CodeGraphRepository (wrapper que implementa el puerto)
adapters/primary/cli/cmd_index.go               — `mem index [--force]`
adapters/primary/cli/cmd_mcp_code_tools.go      — 5 tools MCP (registerCodeTools)
adapters/primary/cli/cmd_hook.go                — hookTurnEnd reindexa incremental los .go tocados
infrastructure/container.go                     — wiring: CodeGraphRepo + Builder.Graph
```

## Modelo de datos

**Entidades** (`domain/code.go`):
- `CodeNodeKind`: `file | package | function | method | type`
- `CodeEdgeKind`: `defines | imports | calls`
- `CodeNode{ID, Project, Kind, Name, Package, File, Receiver, Signature, StartLine, EndLine, Exported}`
- `CodeEdge{ID, Project, FromID, ToID, Kind, Confidence}`
- `GraphStatus{Files, Nodes, Edges, LastIndexedAt, TopPackages}`

**Esquema SQL** (`db.go migrate()`, `CREATE ... IF NOT EXISTS`, idempotente):

```sql
code_files(id, project, path, hash, indexed_at, UNIQUE(project, path))
code_nodes(id, project, file_id, kind, name, package, file, receiver,
           signature, start_line, end_line, exported,
           UNIQUE(project, kind, name, file_id))
code_edges(id, project, from_id, to_id, kind, confidence, src_file_id,
           UNIQUE(project, from_id, to_id, kind))
code_search USING fts5(name, signature, package, node_id UNINDEXED)  -- best-effort, ver abajo
```

`file_id = 0` es el centinela para nodos "paquete" (no pertenecen a un
archivo específico): como `code_files`/`code_nodes` usan `AUTOINCREMENT`
desde 1, ningún archivo real tiene `id = 0`, así que `ReplaceFileNodes`
(que borra por `file_id` real al reindexar un archivo) nunca toca esas filas.

`code_search` se crea en un `db.Exec` separado del schema principal — si la
build de SQLite en uso no soporta FTS5, esa sentencia falla en silencio y
`SearchNodes` cae a `LIKE` automáticamente (intenta FTS5 primero, si el
`MATCH` falla reintenta con `LIKE`). Ningún fallo de FTS5 puede romper la
migración del resto de gomemory (memorias/sesiones/relaciones).

## Indexador — dos pasadas

`Indexer.IndexProject(force)`:
1. `filepath.WalkDir` sobre `Root`, omite `.git .memory .claude vendor node_modules dist build` y ocultos; recolecta `*.go` (incluye tests).
2. Diff por `sha256` contra `FileHashes(project)`: sin cambios → `Skipped++`; nuevo/cambiado → a la cola de indexado.
3. Archivos conocidos que ya no están en disco → `DeleteFile`.
4. **Pasada 1** (`indexFilesInternal`): por cada archivo de la cola, `parseGoFile` extrae nodos (archivo + funciones/métodos/tipos) y dos listas pendientes (`Imports`, `Calls`); `ReplaceFile` los persiste devolviendo IDs asignados.
5. **Pasada 2** (`resolveEdges`, tras que TODOS los archivos de la corrida ya están persistidos): resuelve `DEFINES` (archivo→símbolo, directo), `IMPORTS` (`UpsertPackageNode` + arista), y `CALLS` best-effort:
   - identificador simple (`Bar()`): `NodesByName` filtrado a function/method; único match → 0.9, varios y uno coincide con el paquete del llamador → 0.8, ambiguo → 0.4 (primer candidato), sin match → sin arista.
   - selector (`pkg.Bar()`): resuelve el alias vía el import map del archivo, busca un nodo con ese `Package`/nombre base → 0.7.

`Indexer.IndexFiles(relPaths)` es la misma `indexFilesInternal` sin el walk
completo — la usa `hookTurnEnd` para reindexar solo lo tocado en el turno.

## Tools MCP (`cmd_mcp_code_tools.go`)

| Tool | Input | Uso |
|---|---|---|
| `index_project` | `{force?: bool}` | indexado manual/forzado |
| `graph_status` | `{}` | conteos + paquetes top |
| `search_code` | `{query, limit?}` | búsqueda FTS5/LIKE |
| `get_symbol` | `{name}` | definición + callers/callees directos |
| `list_dependencies` | `{name, direction?, kind?, depth?}` | BFS acotado (máx. profundidad 3) |

Todas están en `setup.ClaudeAutoAllowTools` (pre-aprobadas): son de solo
lectura o escriben únicamente en `.memory/` (nunca tocan el código fuente).

## Integración con memoria existente

- `hookTurnEnd` (`cmd_hook.go`): tras insertar el checkpoint, filtra
  `activity.Files` por extensión `.go`, normaliza rutas absolutas a
  relativas al proyecto (descarta las que caen fuera), y llama
  `Indexer.IndexFiles` best-effort — nunca bloquea el `exit 0` del hook.
- `build_context.go`: `Builder.Graph` (campo opcional, `nil`-safe) agrega una
  sección `## Código indexado` a `get_context` con conteos y paquetes
  principales cuando hay grafo indexado.

## Verificación

```bash
go build ./... && go vet ./... && go test ./...
mem index --force && mem index          # 2ª corrida: todo omitido por hash
echo '{"files":["a.go"],"commands":[]}' | mem hook turn-end   # reindex incremental
```
