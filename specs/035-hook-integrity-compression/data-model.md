# Data Model: Feature 035

## Conversación (`domain.Conversation`, archivo `.memory/.conversation`)

| Campo | Tipo | Regla |
|---|---|---|
| `id` | string | `session_id` del host; si falta, `local-<epoch>-<rand6>` al abrir una conversación nueva |
| `started_at` | int64 (epoch s) | se fija al iniciar la conversación |

- **Formato**: JSON de una línea, permisos 0600, escrito con `writeFileAtomic`.
- **Transiciones**:
  - `session-start` con un `id` distinto del guardado (o sin archivo): **nueva** → `BeginConversation`, que reinicia el estado por turno y valora la rotación de sesión.
  - `session-start` con el mismo `id` (reanudación): **continúa** → sin reinicio.
  - `session-start` sin `id`: **nueva** con id local, salvo `source=resume`; este último conserva la registrada mientras no haya superado el umbral de inactividad.
  - `hook session-end` y `mem session end`: **cierre** → borran `.conversation`; el siguiente inicio sin id crea otra conversación.
  - `post-compact`: **continúa** → se mantienen los reinicios propios de la compactación (marcadores de entrega y huella); el `id` no cambia.
  - Autoarranque MCP / `mem session start` sin conversación registrada: **nueva**, con `id` local.

## Estado por turno (pertenece a la conversación)

| Archivo `.memory/` | Contenido | Se reinicia en |
|---|---|---|
| `.footprint` | entero ≥ 0 | conversación nueva, post-compact |
| `.last-nudge`, `.last-compact-nudge`, `.last-preference-nudge` | epoch | conversación nueva (los dos últimos también en post-compact) |
| `.pending-agent-notice` | JSON `{conv_id, text}` | conversación nueva, post-compact, al consumirse |
| `.session-tools-injected`, `.plan-entered-emitted` | marcador | conversación nueva, post-compact |
| `.plan-reminder-emitted` (nuevo) | marcador | conversación nueva, post-compact |
| `.octopus-emitted` (nuevo) | `"true"`/`"false"` (último valor emitido) | conversación nueva, post-compact |
| `.markers-note-emitted` (nuevo) | marcador | conversación nueva, post-compact |
| `.hook-lock-<evento>-<hash>` (nuevo) | archivo de lock, sin contenido | duplicado mientras otro proceso conserva el lock del SO; se purga a los 60 s solo si está libre |
| `.hook-lock-mutex` | archivo de lock, sin contenido | serializa la creación y limpieza de los locks por evento |
| `.hook-done-user-prompt-submit-<hash>` | recibo vacío, modo 0600; hash de stdin con `prompt_id` | copia tardía del mismo prompt tras completar el handler; se purga tras 1 h |

- **Validación**: un archivo ilegible o corrupto equivale a «ausente» y se borra.
- **Compatibilidad**: un `.pending-agent-notice` en texto plano (formato anterior) se trata como de conversación desconocida. Se entrega solo si tampoco hay `.conversation`; así se conserva el flujo de los tests de integración vigentes.

## Sesión de memoria (`sessions`, sin cambios de esquema)

- **Actividad** = `MAX(memories.created_at WHERE session_id = ?)`, o `sessions.created_at` si no hay memorias.
- **Regla de rotación**: actividad con más de `StaleSessionSecs` (14 400 s) → `End(id, "cerrada automáticamente por inactividad")` + snapshot + `Start`.

## Sección de salida de hook (`domain.HookSection`)

| Campo | Tipo | Regla |
|---|---|---|
| `Name` | string | `bootstrap`, `protocol`, `octopus`, `plan`, `memory`, `notice` |
| `Priority` | int | 100 bootstrap, 90 protocol, 80 octopus, 70 plan, 50 memory, 40 notice |
| `Text` | string | contenido |
| `Trimmable` | bool | `bootstrap` y `protocol`: false |

- **Invariantes**:
  - salida ≤ `HookInlineContextMaxChars` (runas);
  - el orden de emisión es el de prioridad;
  - Σ de las no recortables ≤ 4 000 runas (test);
  - el texto recortado termina con `domain.HookTrimNotice`.

## Evento de guarda (`hook_guard_events`, tabla nueva)

```sql
CREATE TABLE IF NOT EXISTS hook_guard_events (
  project TEXT NOT NULL, agent TEXT NOT NULL, kind TEXT NOT NULL,
  day TEXT NOT NULL,              -- 'YYYY-MM-DD' en UTC-5
  count INTEGER NOT NULL DEFAULT 0,
  last_detail TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, agent, kind, day)
);
```

- `kind` ∈ {`duplicate_dropped`, `budget_trimmed`, `tool_output_excluded`}.
- `last_detail` no contiene nunca contenido del usuario: solo cifras o el nombre de la herramienta.
- Consulta de doctor: `SUM(count) … WHERE day >= date(hoy, '-6 days') GROUP BY agent, kind`.

## Tipo de contenido `domain.ContentListing`

- **Detección**: ≥ 30 líneas no vacías y ≥ 70 % con forma `ruta:línea[:col]:` o ruta sola.
- **Salida**: las 15 primeras y las 5 últimas literales, más las líneas con `error|FAIL|panic|fatal`, y una marca ⟦mem⟧ con la ref y el conteo por archivo o directorio (los 5 primeros y «+N otros»).

## Opciones de compresión (`ports.CompressionOptions`)

- Campo nuevo `ToolOutput bool`: prosa intacta, `JSONMinItems` = 51 y sin limpieza estructural.
- Con `false` (todos los usos actuales salvo el hook de salidas), el comportamiento es idéntico al vigente.
