# Data Model: Registro global de gomemory

El esquema SQL existente (`memories`, `sessions`, `memory_relations`, todas con columna `project`) **no cambia de forma** — esta feature es de infraestructura/persistencia, no de dominio (`domain/` sin cambios). Lo que cambia es de dónde viene el valor de `project` y dónde vive físicamente el archivo que contiene esas tablas.

## Entidades

### Proyecto

Unidad de aislamiento de memoria. No es una tabla nueva — es un concepto que hoy se materializa como "un archivo `.memory/mem.db`" y pasa a materializarse como "un directorio `projects/<clave>/` dentro del store global".

| Campo | Antes | Ahora |
|---|---|---|
| Identidad | `filepath.Base(root)` (nombre de carpeta) | Ruta absoluta del git root (o cwd si no hay `.git`) |
| Clave de filesystem | N/A (la identidad y la ubicación del archivo eran la misma cosa: la carpeta del repo) | Slug legible + `sha256[:8](ruta absoluta)` — ej. `bar-a1b2c3d4` |
| Ubicación de datos | `<root>/.memory/mem.db` | `$DataHome/gomemory/projects/<clave>/mem.db` |

**Regla de derivación de clave**: determinística — la misma ruta absoluta siempre produce la misma clave; rutas distintas (aunque terminen en el mismo nombre de carpeta) nunca colisionan, porque el hash cubre la ruta completa.

### Memoria (sin cambios de estructura)

Sigue siendo cualquier fila de `memories` (`decision`, `bugfix`, `pattern`, `learning`, `discovery`, `preference`, `architecture`, `checkpoint`), asociada a un proyecto vía la columna `project`.

**Transición relevante para esta feature**: al migrar un proyecto desde el modelo legado, las filas existentes tienen `project = filepath.Base(root)` (el valor antiguo). Si tras la migración el código empieza a consultar/escribir usando la nueva clave (`ProjectKey(root)`) sin normalizar los datos, las filas migradas quedarían invisibles para las lecturas nuevas (mismatch de valor en la columna `project` dentro del mismo archivo). **Por eso la migración de datos (ver `research.md` §5) DEBE incluir un `UPDATE` parametrizado sobre `memories`, `sessions` y `memory_relations` que normalice `project` al nuevo valor de clave como parte de la misma operación de mover el archivo**, no como paso separado u opcional.

### Registro de servidor MCP (config externa, no persistida por gomemory)

No es una entidad de la base de datos de gomemory — vive en archivos de configuración de cada agente (`~/.claude.json`, `~/.codex/config.toml`, etc.). Se documenta aquí porque su estado (global vs por-proyecto) determina cómo un proyecto pasa a estar disponible.

| Campo | Descripción |
|---|---|
| Agente | Claude Code, OpenCode, Cursor, Windsurf, Cline, Codex |
| Scope | `global` (una entrada, todos los proyectos) o `project` (una entrada por repo, fallback) |
| Comando registrado | Siempre `mem mcp` sin `--root` — ya es agnóstico de proyecto (`binRefFor`), la resolución ocurre en runtime vía `FindProjectRoot()` |

## Invariantes

- Un `mem.db` en el store global contiene memorias de exactamente un proyecto (mismo invariante que hoy, solo cambia la ubicación del archivo).
- La clave de proyecto es estable mientras la ruta absoluta del git root no cambie; mover o renombrar el repo produce una clave nueva (proyecto nuevo desde la perspectiva de gomemory — ver Edge Cases del spec).
- Toda fila en `memories`/`sessions`/`memory_relations` de un `mem.db` dado comparte el mismo valor de `project`, sea el proyecto legado migrado o uno creado directamente bajo el modelo nuevo (garantizado por la normalización de migración descrita arriba).
