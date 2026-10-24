# Data Model: Base de memoria optimizada, idempotente y con índices

La feature no añade tablas ni columnas. Cambia cómo se prepara el esquema, con qué configuración se abre cada conexión y qué índices existen.

## Base de memoria del proyecto (`mem.db`)

- **Ubicación**: `<DataHome>/projects/<ProjectKey>/mem.db`, en el almacén global.
- **Contenido**: memorias, sesiones, relaciones, revisiones (reviews, reviewer_results, findings, consensus_findings, fix_rounds, rejudgments), instincts del proyecto, socratica, compresión (originals, stats, tuning), entregas de contexto, uso, eventos de guarda, actividad de canales, Octopus, ADR y el grafo de código propio (code_files, code_nodes, code_edges, code_search).
- **Cambios**:
  - Índice nuevo `idx_sessions_project_created ON sessions(project, created_at)`, no único y creado con `IF NOT EXISTS` (FR-015, FR-017).
  - Sin cambios en tablas, columnas, búsquedas FTS5 ni en los demás índices (FR-008).

## Preparación (migración)

Estados de una base frente a la preparación:

```text
inexistente (user_version=0) ──(primera apertura: tx)──► preparada (user_version=schemaVersion)
desactualizada (user_version<schemaVersion, p. ej. 0) ──(apertura: tx)──► preparada
preparada (user_version=schemaVersion) ──(apertura: solo lectura de user_version)──► preparada  (FR-003, FR-018)
más nueva (user_version>schemaVersion, la abrió un binario posterior) ──(apertura: solo lectura)──► sin cambios; user_version nunca baja  (FR-018)
cualquier estado ──(fallo no tolerado)──► sin cambios, user_version intacto + error   (FR-001)
```

Reglas:

- Antes de la transacción se lee `PRAGMA user_version`. Si es mayor o igual que `schemaVersion`, no se hace nada más (R13).
- Los pasos opcionales que fallaron (por ejemplo, `idx_relations_pair` sobre duplicados) no se reintentan hasta la próxima subida de `schemaVersion`. Es un cambio aceptado en la spec (Edge Cases).
- Toda la preparación es una transacción `BEGIN IMMEDIATE … COMMIT` que termina fijando `PRAGMA user_version = schemaVersion` (FR-001, FR-002, FR-018).
- Los pasos tolerados (`ALTER` de una columna que ya existe, índices auxiliares, FTS5) fallan solo en su sentencia y la transacción continúa (FR-005, FR-006).
- Un paso no tolerado (las `Exec` del esquema base que hoy devuelven error) hace `ROLLBACK` y la apertura devuelve el error.
- Ante `SQLITE_BUSY`, `migrateWithRetry` reintenta la transacción entera (FR-007).

## Versión de esquema

- `schemaVersion` (constante en `db.go`): 2 al terminar esta feature. Pasa a 1 con la preparación transaccional (T009) y a 2 con el índice de sesiones (T025). Las bases anteriores tienen 0.
- Se guarda en `PRAGMA user_version`, en la cabecera del archivo, y es transaccional.
- Sube en 1 con cada cambio del catálogo que produce `migrateSchema` (FR-019).

## Configuración de conexión

Valores fijados en research.md R9. Es una propiedad de cada conexión del pool, no del archivo:

| Base | `busy_timeout` | Resto de pragmas de R9 |
|---|---|---|
| `mem.db` (por proyecto) | 5000 | sí |
| `mem.db` legado al migrarlo al almacén global (`globalstore.go`, `MigrateLegacy`) | 5000 | sí: es la misma base y comparte el mismo DSN |
| `instincts.db` (global) | 100 (deliberado, sin cambios) | sí; conserva además su `secure_delete` |

## Consultas frecuentes vigiladas (FR-016)

La prueba de planes cubre, como mínimo, estas consultas reales. Ninguna puede producir `SCAN <tabla>`:

| Área | Consultas |
|---|---|
| Sesiones | sesión activa, recientes, recuento desde una fecha, último prompt |
| Memorias | por proyecto, por tipo, por tópico, por sesión |
| Grafo propio | nodos por nombre, aristas desde y hacia, archivos por proyecto |
| Revisiones | resultados por revisión, hallazgos por resultado, rondas por revisión |
| Eventos | eventos de guarda por proyecto y día, uso por proyecto y sesión |
| Instincts | estadísticas por proyecto |
