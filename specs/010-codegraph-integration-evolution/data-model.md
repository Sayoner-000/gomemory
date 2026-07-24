# Data Model: Evolución de la Integración con Grafo de Código Externo

**Feature**: `010-codegraph-integration-evolution`

## Entidades

### CodeImpactAnnotation (transitoria, no persistida como fila propia)

Resultado de consultar el snapshot cacheado por `filepath` en el momento de
guardar una memoria (Historia 1). Vive adjunta al `content` de la memoria
(texto anexado, p. ej. `"\n\n[impacto: hotspot, 12 llamadores directos]"`),
no como tabla ni columna nueva — consistente con la Assumption del spec.

| Campo | Tipo | Descripción |
|---|---|---|
| `Hotspot` | bool | Si el `filepath` casó con un símbolo marcado como hotspot en el snapshot vigente |
| `Symbol` | string | Nombre del símbolo casado (para el texto de la anotación) |
| `FanIn` | int | Llamadores directos reportados por el proveedor |

Fuente: `domain.CodeArchitecture.Hotspots` (extendido con `File`, ver más
abajo) — se resuelve en memoria, sin I/O adicional.

### ADRDocument / ADRSection / ADRBlock (dominio puro, no persistidas)

`manage_adr` maneja un documento único por proyecto con 6 secciones fijas
(verificado en vivo, ver `research.md` §2) — no un CRUD de múltiples ADR con
ID. El parseo/render de ese documento es dominio puro:

```go
type ADRDocument struct{ Sections []ADRSection }
type ADRSection struct {
    Name   string // uno de: PURPOSE, STACK, ARCHITECTURE, PATTERNS, TRADEOFFS, PHILOSOPHY
    Blocks []ADRBlock
}
type ADRBlock struct {
    MemoryID *int64 // no-nil ⇒ origen gomemory (via marcador <!-- gomemory:id=N -->)
    Heading  string
    Body     string
}
```

`ParseADRDocument(content string) ADRDocument` / `(d ADRDocument)
Render() string` son funciones puras (sin I/O), testeables con fixtures de
texto plano.

### ADRSyncRecord (persistida — tabla `adr_sync_records`)

Relación entre una memoria de gomemory (`architecture`/`decision`) y el
bloque que le corresponde dentro del documento único del proveedor. Uno de
los dos DEBE existir al crear el registro (`memory_id` si el origen es
gomemory; `block_key` siempre presente).

| Columna | Tipo | Descripción |
|---|---|---|
| `id` | INTEGER PK | Auto-incremental |
| `project` | TEXT | Identidad de proyecto (mismo criterio que el resto de tablas) |
| `memory_id` | INTEGER NULL | FK → `memories(id)`; NULL solo transitoriamente entre "bloque importado" y "memoria creada" en la misma operación |
| `provider` | TEXT | Nombre del proveedor (`CodeGraphProvider.Name()`) dueño del documento |
| `section` | TEXT | Sección del documento donde vive el bloque (`ARCHITECTURE`/`TRADEOFFS`) |
| `block_key` | TEXT | Identidad estable del bloque: `id=<memory_id>` si origen gomemory, o hash de `section+heading` si origen proveedor (no hay ID real que usar) |
| `origin` | TEXT | `gomemory` (se exportó desde una memoria existente) o `provider` (se importó desde un bloque sin marcador) — decide qué dirección de sync se salta para evitar bucles |
| `status` | TEXT | `ok`, `pending` (proveedor no disponible en el último intento), `failed`, `conflict_resolved` |
| `content_hash` | TEXT | Hash del último `Body` sincronizado — permite detectar cambios en cualquiera de los dos lados sin depender de timestamps que la API no expone |
| `last_synced_at` | TEXT | Timestamp UTC-5 del último intento (exitoso o no) |
| `created_at` | TEXT | Timestamp UTC-5 de creación del registro |

**Índices**: `(project, memory_id)` único (una memoria, un bloque — cumple
FR-005/FR-005c evitando duplicados por reexportación); `(project, provider,
block_key)` único (un bloque del proveedor, una memoria — evita reimportar
el mismo bloque dos veces).

**Migración** (`persistence/migrate()`, aditiva, `CREATE TABLE IF NOT
EXISTS`, mismo patrón que `memory_relations`):

```sql
CREATE TABLE IF NOT EXISTS adr_sync_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    memory_id INTEGER,
    provider TEXT NOT NULL,
    section TEXT NOT NULL,
    block_key TEXT NOT NULL,
    origin TEXT NOT NULL CHECK (origin IN ('gomemory', 'provider')),
    status TEXT NOT NULL CHECK (status IN ('ok', 'pending', 'failed', 'conflict_resolved')),
    content_hash TEXT NOT NULL,
    last_synced_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (memory_id) REFERENCES memories(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_adr_sync_memory
    ON adr_sync_records (project, memory_id) WHERE memory_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_adr_sync_block
    ON adr_sync_records (project, provider, block_key);
```

### ProviderCandidateList (configuración, no persistida en SQLite)

Vive en `.memory/settings.json` (mismo archivo ya usado hoy para
`code_graph_disabled`/`code_graph_command`), no en base de datos — es
configuración, no dato de dominio.

| Campo (JSON) | Tipo | Descripción |
|---|---|---|
| `code_graph_providers` | `[]string` | Comandos candidatos, en orden de prioridad. Vacío → fallback a `code_graph_command` (compatibilidad) → fallback a autodetección en PATH |
| `adr_sync_enabled` | bool | Activa la Historia 2 (default `false`) |
| `code_impact_annotation_enabled` | bool | Activa la Historia 1 (default `true`) |

## Cambios a tipos existentes

### `domain.CodeHotspot` (extensión aditiva)

```go
type CodeHotspot struct {
    Name  string `json:"name"`
    FanIn int    `json:"fan_in"`
    File  string `json:"file,omitempty"` // nuevo: habilita el match por filepath (Historia 1)
}
```

Campo `omitempty`: un snapshot ya cacheado de una versión anterior (sin
`File`) sigue siendo válido JSON — `File` simplemente queda vacío y esos
hotspots no participan del match por archivo hasta el próximo refresco.

## Relaciones

```
memories (existente) 1───0..1 adr_sync_records   (una memoria architecture/decision, a lo sumo un ADR vinculado)
CodeProviderSnapshot (existente, en disco) ──lectura──> CodeImpactAnnotation (calculado, no persistido)
settings.json (existente) ──configura──> ProviderCandidateList + adr_sync_enabled + code_impact_annotation_enabled
```

Ninguna relación nueva toca `memory_relations` (esa tabla sigue siendo
exclusiva de `mem compare`/sinapsis entre memorias del propio gomemory).
