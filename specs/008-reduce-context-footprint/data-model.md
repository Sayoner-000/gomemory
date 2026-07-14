# Data Model — Reducir la huella de contexto de gomemory (Phase 1)

Cambios de datos mínimos. La feature es sobre todo de **representación** (texto
emitido); solo el dedup por tópico y el contador de huella tocan datos.

## Entidad: Memory (extensión)

Archivo: `domain/memory.go` · Tabla: `memories`

| Campo | Tipo | Nuevo | Descripción |
|-------|------|-------|-------------|
| `TopicKey` | `string` (opcional) | **Sí** | Clave de tópico para upsert (P4). Vacío = sin agrupación por tópico (comportamiento actual). |
| _(resto de campos existentes)_ | — | No | Sin cambios. |

**Regla de validación / negocio (dedup, D6)**:
- Al insertar, si existe una memoria del mismo `project`+`type`+`title` con
  `created_at` dentro de `DedupWindowDays` (default 7) **y no es `checkpoint`**,
  se actualiza esa memoria (`content`, `updated_at`) en vez de crear una nueva.
- Si `TopicKey != ""` y ya existe una memoria con ese `TopicKey` en el proyecto,
  se actualiza la existente (revisión) en vez de crear otra.
- Idempotente: guardar N equivalentes ⇒ 1 fila consolidada (SC-007).

**Migración** — con el mecanismo EXISTENTE en `adapters/secondary/persistence/db.go` (no hay archivos numerados; el esquema vive en `migrate(db)`):
```go
// dentro de migrate(db), junto a los addColumnIfMissing actuales:
addColumnIfMissing(db, "memories", "topic_key", "TEXT")
```
```sql
-- añadir al string `schema` embebido (junto a los otros CREATE INDEX IF NOT EXISTS):
CREATE INDEX IF NOT EXISTS idx_memories_topic
  ON memories(project, topic_key) WHERE topic_key IS NOT NULL;
```
> Patrón real del proyecto (ver `origin_prompt`/`last_prompt` en db.go): `addColumnIfMissing` es idempotente (traga el error si la columna ya existe).

## Entidad: SettingsData (extensión)

Archivos: `application/ports/settings_repository.go`, `adapters/secondary/persistence/settings.go`

| Campo | Tipo | Nuevo | Default | Descripción |
|-------|------|-------|---------|-------------|
| `Budget` | `int` | **Sí** | `24000` (CARACTERES; ~6k tokens) | Techo blando de `get_context`, medido en `len(...)` para comparar directo. `≤ 0` = sin límite (opt-in). |
| `CompactThreshold` | `int` | **Sí** | `48000` (chars emitidos/sesión) | Umbral del recordatorio neutral de compactación. `≤ 0` = desactivado. |
| `DedupWindowDays` | `int` | **Sí** | `7` | Ventana de identidad para dedup por proyecto+tipo+título. `≤ 0` = sin dedup por identidad. |

- Reflejar los tres campos en `SettingsData` (puerto) y `Settings` (adaptador),
  con `json:"...,omitempty"` donde aplique y valores en `DefaultSettings()`.
- Exponerlos en la pantalla de settings de la TUI (coherencia con
  `CodeGraphDisabled`/`CodeGraphCommand`).

## Estado efímero: huella emitida por sesión (D4)

No es una entidad persistida de dominio; es un **contador por sesión activa** de
bytes que gomemory ha emitido en respuestas de tools. Se acumula en el choke
point de respuesta MCP y se consulta en `hookTurnEnd`.

| Aspecto | Definición |
|---------|-----------|
| Alcance | Sesión activa del proyecto |
| Unidad | Caracteres (bytes UTF-8) del texto de respuesta emitido |
| Uso | `hookTurnEnd` compara contra `CompactThreshold`; debounce «una vez por umbral» |
| Persistencia | Mínima: puede vivir en la sesión activa (columna/campo) o en un marcador en `.memory/`, según el patrón de sesión existente; decisión de `/tasks` |
| Reset | Al iniciar sesión o tras compactación (`post-compact`) |

## Objetos de representación (sin persistencia)

| Objeto | Dónde | Propósito |
|--------|-------|-----------|
| Extracto de memoria | helper puro en `domain` | Truncar a la primera oración/N chars + no romper palabras; usado por `Builder.Build`, `search_memories`, `list_memories` (D2/D3) |
| Salida de contexto acotada | `Builder.Build` | Aplica presupuesto y prioridad de secciones (protocolo/conflictos intactos) |

## Invariantes

- Acotar la salida **nunca** borra ni muta `content` en BD (SC-005); solo el
  dedup escribe, y lo hace consolidando, no perdiendo información.
- `get_memory <id>` siempre devuelve el `content` íntegro (capa 3).
- Todo lo anterior es idéntico para cualquier consumidor (MCP/CLI/TUI) — el
  acotado ocurre en capas compartidas, no por cliente (SC-006).
