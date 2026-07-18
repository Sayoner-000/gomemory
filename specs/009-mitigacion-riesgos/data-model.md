# Data Model — Mitigación de riesgos operativos de gomemory

Ninguna de las cuatro mejoras introduce una entidad de dominio nueva persistida como "recurso de negocio" (no hay un nuevo tipo de memoria ni un nuevo puerto de repositorio). Las tres entidades derivadas del spec son estructuras auxiliares/técnicas:

## Snapshot de backup

Representa una copia exportada, en un momento dado, de las memorias y relaciones de un proyecto.

| Campo | Tipo | Descripción |
|---|---|---|
| `path` | string | Ruta del archivo JSON, `<DataHome>/backups/<project-key>/<timestamp>.json` |
| `project` | string | Clave del proyecto (misma `project-key` que resuelve `globalstore.go` para `mem.db`) |
| `createdAt` | string (RFC3339, UTC) | Igual convención que `domain.ExportBundle.ExportedAt` |
| `bundle` | `domain.ExportBundle` | Reutiliza el struct ya existente (`Version`, `ExportedAt`, `Source`, `Memories[]`, `Relations[]`) — no se agrega ningún campo nuevo al bundle |

**Reglas de validación / ciclo de vida**:
- Se crea automáticamente al finalizar una sesión (best-effort, sin bloquear el cierre de sesión si falla).
- Se retiene un máximo de `keep` snapshots por proyecto (variable de entorno con default documentado); al superar el límite, se elimina el archivo con timestamp más antiguo.
- No tiene relación con otras entidades de dominio — es un artefacto de filesystem, no una fila de base de datos.

## Patrón de secreto

Una forma de texto reconocible que corresponde a un tipo conocido de credencial. No es una entidad persistida — es una regla de transformación aplicada en memoria antes de escribir.

| Campo | Tipo | Descripción |
|---|---|---|
| `label` | string | Identificador corto del tipo (`aws-key`, `github-token`, `ai-provider-key`, `slack-token`, `jwt`, `pem-private-key`) |
| `pattern` | `*regexp.Regexp` | Expresión regular de detección |
| `placeholder` | string | Texto de reemplazo, `[REDACTED:<label>]` |

**Reglas de validación**:
- La lista de patrones es fija en código (no configurable por variable de entorno ni por el usuario) — coherente con `RedactPrivate`, que tampoco es configurable.
- Un texto que no coincide con ningún patrón se persiste sin cambios (no debe haber falsos positivos sobre contenido legítimo).
- Se aplica en los mismos call sites que `RedactPrivate`, antes de cualquier `INSERT`/`UPDATE`.

## Índice de relevancia (`memory_search`)

Tabla virtual FTS5 derivada del contenido de `memories`, usada solo para ordenar resultados de búsqueda.

| Columna | Tipo | Descripción |
|---|---|---|
| `title` | TEXT (indexada FTS5) | Copia del título de la memoria, para matching y ranking |
| `content` | TEXT (indexada FTS5) | Copia del contenido de la memoria |
| `memory_id` | UNINDEXED | Referencia a `memories.id`, no participa en el matching, solo se usa para el JOIN de vuelta a la tabla `memories` |

**Reglas de validación / consistencia**:
- Debe permanecer sincronizada 1:1 con `memories` ante `INSERT`/`UPDATE`/`DELETE` (dual-write manual, no triggers — siguiendo el patrón de `code_search`).
- Es prescindible: si el build no tiene FTS5 disponible, la tabla simplemente no existe y el dispatcher de búsqueda usa `searchMemoriesLike` sin error visible (mismo comportamiento que hoy tiene `code_search`, ver `db.go:184-187`).
- No tiene su propio ciclo de vida de "borrado" independiente — vive y muere junto con las filas de `memories` que indexa.

## Relación con entidades existentes

```
memories (existente)  ──dual-write──>  memory_search (nueva, FTS5)
memories + memory_relations (existentes) ──ExportProject──> ExportBundle (existente) ──escritura──> Snapshot de backup (nuevo, en filesystem)
```

Ninguna entidad existente (`Memory`, `Relation`, `Session`, `ExportBundle`) cambia su forma o sus campos. `RedactSecrets` opera sobre los mismos campos de texto (`Title`, `Content`, `OriginPrompt`, `LastPrompt`) que ya redacta `RedactPrivate`, sin agregar columnas.
