# Feature Specification: Grafo de Código (Fase 1 — Go puro)

**Feature Branch**: `004-code-graph`

**Created**: 2026-07-03

**Status**: Implementado (Fase 1)

**Input**: Indexar el proyecto como grafo de código (inspirado en
[codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) y su
paper, arXiv 2603.27277) para que un agente con el MCP de gomemory conectado
pueda consultar la estructura del código — símbolos, llamadas, imports — sin
tener que releer archivos completos por grep.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Indexar el código del proyecto (Priority: P1)

Como agente AI trabajando en un proyecto Go con gomemory instalado, quiero
que exista un índice consultable de funciones, métodos, tipos e imports del
proyecto, para poder ubicar definiciones y relaciones sin leer archivo por
archivo.

**Independent Test**: Se puede probar de forma aislada corriendo `mem index`
(o la tool MCP `index_project`) sobre un proyecto Go de prueba y verificando
con `graph_status` que los conteos de archivos/símbolos/relaciones son
mayores a cero.

**Acceptance Scenarios**:

1. **Given** un proyecto Go sin indexar, **When** se corre `index_project`,
   **Then** se crean nodos para cada función, método y tipo top-level de cada
   archivo `.go`, y el reporte devuelve conteos de archivos/nodos/aristas.
2. **Given** un proyecto ya indexado sin cambios en disco, **When** se corre
   `index_project` de nuevo, **Then** los archivos sin cambios se omiten
   (diff por hash sha256) y no se reprocesan.
3. **Given** un archivo indexado que se borra del proyecto, **When** se corre
   `index_project`, **Then** el archivo y sus símbolos/aristas se eliminan
   del grafo.
4. **Given** que un agente edita archivos `.go` durante un turno, **When** el
   turno termina (hook `Stop`/`session.idle`), **Then** esos archivos se
   reindexan automáticamente sin intervención del agente.

### User Story 2 — Consultar el grafo desde el agente (Priority: P1)

Como agente AI, quiero buscar símbolos por nombre, ver la definición de un
símbolo con sus callers/callees, y recorrer dependencias, para responder
preguntas de arquitectura sin grep exhaustivo.

**Acceptance Scenarios**:

1. **Given** un proyecto indexado, **When** se llama `search_code` con un
   término, **Then** se devuelven los símbolos cuyo nombre/firma/paquete
   coincide, ordenados por relevancia.
2. **Given** una función indexada que llama a otra, **When** se llama
   `get_symbol` sobre la función llamadora, **Then** la respuesta incluye la
   función llamada bajo "Llama a".
3. **Given** una función indexada, **When** se llama `list_dependencies` con
   `direction=in` y `kind=calls`, **Then** se listan (BFS acotado por
   profundidad) los llamadores directos e indirectos hasta la profundidad
   pedida (máximo 3).

## Alcance de la Fase 1

- **Solo Go**, vía `go/parser`/`go/ast`/`go/token` de la stdlib — **sin
  cgo**, coherente con `modernc.org/sqlite` (driver puro Go) ya usado por el
  resto de gomemory. El binario sigue siendo estático y portable.
- **Sin type-checking completo**: la resolución de llamadas (`CALLS`) es
  best-effort por nombre — mismo paquete (`confidence` 0.8–0.9) o vía el
  import map del archivo para llamadas tipo `pkg.Func()` (`confidence` 0.7).
  Llamadas a paquetes no indexados (stdlib, dependencias externas) no
  generan arista `CALLS` (no hay nodo destino), aunque sí generan una arista
  `IMPORTS` hacia un nodo paquete.
- **Búsqueda**: FTS5 (`modernc.org/sqlite` lo soporta sin flags de build,
  verificado) con fallback automático a `LIKE` si no está disponible en
  alguna build.

## Fuera de alcance (Fase 2 futura)

- Soporte multi-lenguaje vía tree-sitter (requeriría cgo o WASM — rompe la
  portabilidad del binario actual; se evalúa por separado).
- Resolución de llamadas con type-checking completo (`go/types`).
- FTS5 para la búsqueda de memorias (`SearchMemories` sigue usando `LIKE`
  intencionalmente en esta fase, para no acoplar cambios).
- `.gitignore` real (la Fase 1 usa una lista fija de directorios a omitir:
  `.git`, `.memory`, `.claude`, `vendor`, `node_modules`, `dist`, `build`,
  ocultos).
