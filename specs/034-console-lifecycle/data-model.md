# Modelo de datos: ciclo de vida de gomemory en consola

**Feature**: [spec.md](spec.md) · **Investigación**: [research.md](research.md)

Esta feature no toca el esquema SQLite. Todo el estado nuevo son archivos pequeños en el almacén global o en `.memory/`, escritos siempre de forma atómica (temporal + rename) y con permisos privados.

## Entidades de dominio (`domain/`, sin I/O)

### Version
Versión semántica de gomemory.
- `Major`, `Minor`, `Patch int`; `Pre string` (vacío en releases estables).
- `ParseVersion(s string) (Version, bool)`: acepta `v2.26.4` y `2.26.4`. Rechaza lo que no es semver.
- `Newer(a, b Version) bool`: comparación semver. Una prerrelease nunca es "más nueva" a efectos del aviso.
- **Regla**: el aviso solo sale si `Newer(latest, actual)` (FR-030).

### UpdateCheck (caché de versiones)
Archivo `<DataHome>/update-check.json`, 0600, global al usuario.

| Campo | Tipo | Regla |
|---|---|---|
| `latest` | string | Tag de la última release estable (`v2.26.5`). Vacío si nunca hubo una consulta con éxito. |
| `checked_at` | RFC 3339 | Momento de la última consulta, con éxito o 304. |
| `etag` | string | Validador de la API para `If-None-Match`. |
| `last_error` | string | Último error de consulta. Se usa en `mem doctor`, nunca en el aviso. |

- **Vencida** si `now - checked_at > 24h` o el archivo no existe o es ilegible (FR-028). El TTL es una constante de dominio, `domain.UpdateCheckTTL`.
- Un fallo de consulta **conserva** `latest` y `etag`, y solo actualiza `checked_at` y `last_error` (caso límite: se reintenta pasado el intervalo, no en cada sesión).

### UpdateNotice (marca de aviso mostrado)
Archivo `.memory/.update-notice`, 0600, por proyecto: `{"version": "v2.26.5", "session": "<id>"}`.
- **Regla**: el aviso se emite si `latest` es más nueva **y** (`version` ≠ `latest` o `session` ≠ la sesión activa). Así cumple "como mucho una vez por versión y sesión" (FR-030).

### InstallSelection (selección de instalación)
Campos nuevos en `.memory/settings.json` (ajustes existentes del proyecto):

| Campo JSON | Tipo | Regla |
|---|---|---|
| `agents` | []string | Subconjunto de nombres de `domain.KnownAgents`. Ausente = usar los detectados. |
| `agent_scope` | string | `project` (por defecto) o `global`. Cualquier otro valor se trata como `project`. |
| `update_check_disabled` | bool | Si es `true`, no hay consulta ni aviso en este proyecto (FR-031). |

`GOMEMORY_NO_UPDATE_CHECK=1` desactiva la consulta **en todo el usuario** y tiene prioridad sobre el ajuste.

### LocalCopy (copia local)
Valor calculado, no se persiste.
- `Path` (`<root>/mem` o `mem.exe`), `Version` (salida de `version`) y `IsGomemory bool`.
- **Retirable** si: existe un global (R1) ∧ `!SameFile(Path, global)` ∧ `IsGomemory` ∧ es un archivo regular (FR-003).

### ProjectRegistration (registro de proyecto)
Archivo `<DataHome>/projects/<key>/root`, 0600, con una línea: la ruta absoluta del proyecto.
- Se escribe en cada `install`, de forma idempotente (FR-035).
- La `<key>` es la misma que ya usa el almacén (`ProjectKey(root)`), así que la entidad se une sin migración al directorio existente de la base.
- **Estado**: *activo* si la ruta existe en disco; *huérfano* si no existe (el caso límite de la spec: sus datos del almacén se retiran igualmente).

### UninstallPlan (inventario de desinstalación)
Valor calculado; es la única fuente para `--dry-run` y para la ejecución (SC-004).

| Campo | Tipo | Regla |
|---|---|---|
| `Scope` | `project` \| `system` | FR-007 |
| `Memory` | `export` \| `delete` \| `keep` | FR-009 |
| `ExportDir` | string | `~/gomemory-export-AAAAMMDD-HHMMSS/` o `--export`. Nunca dentro de `DataHome` (FR-009a). |
| `Items` | []UninstallItem | Ordenados según FR-014 |
| `ScanRoot`, `ScanSkipped` | string, []string | Raíz del escaneo y rutas omitidas por permisos |

**UninstallItem**:

| Campo | Tipo |
|---|---|
| `Category` | `binary` \| `memory` \| `agent-config` \| `project-files` |
| `Path` | string |
| `Kind` | `file` \| `dir` \| `entry` (solo la entrada de gomemory dentro de un archivo compartido) |
| `Bytes` | int64 (solo en `memory`) |
| `Result` | `pending` → `ok` \| `warn` |
| `Manual` | string: el comando para quitarlo a mano cuando `Result = warn` (FR-015) |

**Transiciones**: todos los elementos nacen en `pending`. La ejecución avanza en el orden de `Items` y ningún `warn` detiene el recorrido (FR-015). En `--dry-run` los elementos se quedan en `pending` y se imprimen.

### ExportIndex (índice de exportación)
`<ExportDir>/index.json`, 0600: `[{"key": "...", "root": "/ruta" | "", "file": "<key>.json", "memories": N, "relations": M}]`. Cada `<key>.json` es un bundle de `mem export` (0600) que se puede reimportar con `mem import` (SC-011).

## Puertos nuevos (`application/ports/`)

Todos reciben `context.Context` como primer parámetro (constitución §1.2).

| Puerto | Métodos | Adaptador |
|---|---|---|
| `ReleasePort` | `Latest(ctx, etag) (tag, newEtag string, notModified bool, err error)`; `Checksum(ctx, tag, asset) (string, error)` | `adapters/secondary/release/github.go` (HTTP con timeout) |
| `UpdateCheckRepository` | `Read(ctx) (UpdateCheck, bool)`; `Write(ctx, UpdateCheck) error` | `persistence/update_check.go` |
| `ProjectRegistryRepository` | `Register(ctx, root) error`; `List(ctx) ([]ProjectRegistration, error)` | `persistence/registry.go` |
| `ClockPort` | ya existe (feature 033) | Se reutiliza para el TTL en las pruebas |

La consola (`adapters/primary/console/`) es un adaptador **primario**: la invocan los comandos `install`, `update` y `uninstall`, no se inyecta como puerto.
