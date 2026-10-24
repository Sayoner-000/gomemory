# Research: Base de memoria optimizada, idempotente y con índices

**Feature**: 041-migracion-transaccional · **Fecha**: 2026-10-23

Cada decisión se contrastó con el código actual (`adapters/secondary/persistence/db.go`) y con un prototipo ejecutado contra el driver real `modernc.org/sqlite v1.58.0`. El prototipo se retiró después de usarlo.

## R1. ¿Dónde se va el tiempo de la primera apertura?

- **Decision**: la causa es el número de confirmaciones durante `migrate`, no el volumen del esquema.
- **Rationale**: `migrate(db *sql.DB)` hace 8 `db.Exec` (dos de ellos con el esquema base en una sola cadena multisentencia) y 13 `addColumnIfMissing`. En total son unas 53 sentencias `CREATE` y 13 `ALTER`. Cada sentencia corre en autocommit, y en modo WAL cada commit sincroniza el WAL a disco. Según la medición de Windows CI (run 39941668694), la base nueva tarda 554 ms de mediana y 1.760 ms como máximo, y la ya creada 6 ms. Con el mismo proceso y el mismo esquema, la diferencia solo se explica por la escritura y la sincronización. En macOS la base nueva tarda 12 ms, porque allí sincronizar es barato: el coste depende de la plataforma.
- **Alternatives considered**:
  - Subir los plazos de los hooks: lo descarta la spec (FR-010) y solo movería el umbral de fallo.
  - `PRAGMA synchronous=OFF` durante la migración: sacrifica durabilidad sin hacer atómica la preparación.
  - Crear la base durante `mem install`: no cubre los hooks globales en proyectos sin instalar. Queda fuera de alcance.

## R2. ¿Admite SQLite DDL dentro de una transacción, incluidas las tablas virtuales FTS5?

- **Decision**: sí. Toda la preparación va en una sola transacción.
- **Rationale**: el prototipo creó tablas con una cadena multisentencia, añadió una columna con `ALTER` y creó `fts5` dentro de un `BEGIN IMMEDIATE`, y confirmó sin error. Después del commit estaban los tres objetos y las dos columnas.
- **Alternatives considered**: agrupar solo los `CREATE` y dejar los `ALTER` en autocommit. Se descarta porque mantiene 13 sincronizaciones y pierde la atomicidad (FR-001).

## R3. ¿Un fallo tolerado invalida la transacción?

- **Decision**: no. Los fallos que hoy se ignoran se pueden seguir ignorando dentro de la transacción.
- **Rationale**: en el prototipo, `ALTER TABLE a ADD COLUMN x` sobre una columna ya existente devolvió `duplicate column name: x (1)`, y `CREATE VIRTUAL TABLE … USING nonexistent_module` devolvió `no such module (1)`. Después de ambos errores, la misma transacción aceptó otro `ALTER` y un `CREATE VIRTUAL TABLE … fts5`, y confirmó. SQLite revierte solo la sentencia que falla.
- **Alternatives considered**: comprobar antes con `PRAGMA table_info` si la columna existe. Añade consultas sin ganar nada sobre el comportamiento actual (FR-005).

## R4. Un fallo no tolerado, ¿deja la base a medias?

- **Decision**: ante un error que hoy aborta `migrate` (las `Exec` del esquema que devuelven `err`), se hace `Rollback` y se devuelve el error. No queda ningún cambio (FR-001).
- **Rationale**: hoy un fallo en mitad del esquema base deja confirmadas las tablas de instincts y socratica, que ya se ejecutaron en autocommit. Con la transacción, la siguiente apertura parte de cero y prepara todo.
- **Cómo se prueba**: se crea antes una vista llamada `memories`. `CREATE TABLE IF NOT EXISTS memories` no hace nada porque el nombre ya existe, y el `CREATE INDEX … ON memories` que viene después falla porque una vista no admite índices. La prueba comprueba que `Open` falla y que tampoco existen las tablas de instincts ni de socratica, que en el código actual sí quedarían creadas.

## R5. Concurrencia: dos procesos preparan a la vez una base nueva

- **Decision**: se mantienen `busy_timeout(5000)`, `_txlock=immediate` y `migrateWithRetry`. La transacción se abre con `db.BeginTx`, que con `_txlock=immediate` emite `BEGIN IMMEDIATE`, pero solo cuando `user_version` indica que la base no está al día (R13). Dentro de la transacción se vuelve a leer `user_version`: si otro proceso ya la preparó, no se ejecuta nada.
- **Rationale**: `BEGIN IMMEDIATE` toma el bloqueo de escritura al empezar y respeta `busy_timeout`. El segundo proceso espera a que el primero confirme y después ejecuta una migración que no cambia nada, porque todo es `IF NOT EXISTS` o `ALTER` tolerado. Hoy, cada una de las 66 sentencias compite por el bloqueo por separado. `TestOpenDBFile_AperturaConcurrenteDeUnaBaseNueva` cubre este caso y debe seguir pasando sin cambios.
- **Alternatives considered**: un candado de archivo propio para la migración. Duplica lo que SQLite ya garantiza.

## R6. Alcance del cambio de firma

- **Decision**: `migrate(db *sql.DB) error` conserva su firma, porque 5 pruebas existentes la llaman. Pasa a ser el envoltorio transaccional: lee `user_version` (R13), abre `db.BeginTx`, llama a `migrateSchema(tx)`, fija `user_version` y confirma. El cuerpo actual se mueve sin cambios de SQL a `migrateSchema(ex schemaExecer)`. `addColumnIfMissing` recibe el mismo `schemaExecer`. `migrateWithRetry` no cambia: reintenta `migrate` entero ante `SQLITE_BUSY`.
- **Rationale**: es el cambio mínimo, no toca a ningún llamador de `Open` y no modifica ninguna prueba existente (constitución §10).
- **Fuera de alcance**: el esquema propio de `OpenInstinctGlobal` (`instincts.db`). En la medición tardó como máximo 149 ms y no está en la ruta del fallo.

## R7. Equivalencia del esquema y su vigilancia (FR-008, FR-019)

- **Decision**: un único golden, `adapters/secondary/persistence/testdata/schema.golden`. Su primera línea es `user_version=<N>`, y después vienen las filas de `sqlite_master` (type, name, tbl_name, sql) ordenadas y el `pragma_table_info` de cada tabla. `TestSchemaGolden` lo compara siempre y nunca hace `t.Skip`. Con el flag `-update-schema-golden` (`go test … -args -update-schema-golden`) lo reescribe, pero se niega si el catálogo cambió y `schemaVersion` sigue igual al del golden (FR-019).
- **Flujo de la feature**:
  1. Se genera el golden con el código anterior, con `user_version=0`.
  2. Tras el cambio, el diff contra ese golden solo puede ser `idx_sessions_project_created`, y ese diff se guarda en `evidence/schema-diff.txt` (FR-008).
  3. Se sube `schemaVersion` a 1 y se regenera el golden.
- **Rationale**: es la validación del estado resultante que exige la constitución (§6), y además evita el riesgo propio de R13: cambiar el esquema sin subir la versión dejaría sin migrar las bases existentes.
- **Alternatives considered**: un generador que hace `t.Skip` en CI. Se descarta porque la constitución (§10) no admite tests ignorados sin ticket.

## R8. Cómo se verifica el resultado en Windows (FR-009)

- **Decision**: se reaplica el commit de instrumentación `3e81733` (rama `diag/windows-timing`) sobre una rama de diagnóstico que parte de `041-migracion-transaccional`, y se ejecuta el workflow `diag-windows-timing`. Los resultados se anotan junto a la línea base en `evidence/`.
- **Rationale**: misma instrumentación y mismo runner que la línea base, así que los números son comparables. La instrumentación no se integra en `main`.

## R9. Configuración de conexión (FR-011, FR-012)

- **Decision**: cada conexión se abre con estos pragmas, en este orden, en el DSN (`_pragma=…`). El driver los aplica a cada conexión del pool, no solo a la primera.

  | Pragma | Valor | Motivo |
  |---|---|---|
  | `busy_timeout` | 5000 en `mem.db` y el almacén global; 100 en `instincts.db` (sin cambios) | Va primero: los pragmas siguientes pueden tocar el archivo y deben esperar ante un bloqueo (feature 035). |
  | `journal_mode` | `WAL` (sin cambios) | Los lectores no bloquean al escritor. |
  | `synchronous` | `NORMAL` | Con WAL, sincroniza solo en los checkpoints y no en cada commit. Es seguro frente a caídas del proceso; ante un corte de energía puede perderse el último commit, sin corrupción. Elimina la mayor parte del coste medido en R1, también fuera de la primera apertura. |
  | `cache_size` | `-16000` (16 MB) | Las páginas se reservan bajo demanda: un hook corto no gasta memoria que no usa. Una memoria típica cabe entera. |
  | `temp_store` | `MEMORY` | Los ordenamientos y agregados temporales (`GROUP BY`, `ORDER BY` sin índice) no escriben en disco. |
  | `mmap_size` | `268435456` (256 MB) | Lectura sin copia. Solo reserva espacio de direcciones; si el sistema de archivos no lo admite, SQLite vuelve a la lectura normal. |
  | `journal_size_limit` | `67108864` (64 MB) | El WAL no queda indefinidamente en su tamaño máximo tras un checkpoint. |

- **Rationale**: es la configuración recomendada por la documentación de SQLite para WAL en aplicaciones locales con un escritor y lecturas concurrentes. Aplicarla en el DSN evita que una conexión nueva del pool salga sin ajustar.
- **Alternatives considered**:
  - `synchronous=FULL` con la migración en una sola transacción: resuelve la primera apertura, pero cada escritura de un turno sigue sincronizando a disco. Se descarta porque la persona pide optimizar al máximo y la contrapartida de `NORMAL` con WAL es la estándar.
  - `wal_autocheckpoint=0` con checkpoints en segundo plano: está pensado para cargas masivas, y la memoria escribe poco y en transacciones cortas.

## R10. Índices (FR-015, FR-016, FR-017)

- **Decision**: se añade `CREATE INDEX IF NOT EXISTS idx_sessions_project_created ON sessions(project, created_at)`. Una prueba de planes de consulta recorre las consultas frecuentes y falla ante un `SCAN <tabla>` sin índice.
- **Evidencia**: en una base nueva, `EXPLAIN QUERY PLAN` de las consultas reales de `session.go:102`, `session.go:126`, `memory.go:508` y `channel_activity.go:102` daba `SCAN sessions` más `USE TEMP B-TREE FOR ORDER BY`. Con el índice aplicado a una copia, las cuatro pasan a `SEARCH sessions USING INDEX idx_sessions_project_created (project=?)` y la de recuento a `COVERING INDEX`. El `ORDER BY created_at DESC, rowid DESC` lo resuelve el propio índice, porque el rowid es su cola implícita.
- **Ya cubiertas** (sin cambios): findings, fix_rounds, reviewer_results, context_deliveries, delivered_blocks, usage_records, hook_guard_events e instinct_stats usan índices existentes, algunos de ellos autoindex de su clave.
- **Alternatives considered**: un índice parcial `WHERE ended_at IS NULL` solo para la sesión activa. No sirve para «recientes» ni para el recuento, y el compuesto ya resuelve las cuatro consultas.
- **Por qué no único**: un índice único podría fallar sobre datos existentes (FR-017). El compuesto no tiene esa limitación.

## R11. Estadísticas de consulta al cerrar (FR-013)

- **Decision**: `Container.Close` ejecuta `PRAGMA analysis_limit=400; PRAGMA optimize;` antes de cerrar la base, ignorando los errores, solo si el canal del contenedor no es `"hook"`. Tope: el cierre crece como máximo 100 ms en el peor caso en Windows CI (FR-013), medido con una marca `close` en la instrumentación.
- **Rationale**: `PRAGMA optimize` solo analiza las tablas cuyas estadísticas lo necesitan, y `analysis_limit` acota el trabajo por índice. Es la práctica que recomienda SQLite para conexiones que se cierran.
- **Alcance (corregido tras el análisis G1)**: los hooks SÍ pasan por `Close`. `infrastructure/main.go:169` hace `defer closeContainer(container)`, y los hooks protegidos contra reentrada (`session-start`, `user-prompt-submit`, `turn-end`, `subagent-start`, `subagent-stop`) vuelven a `main`. Por eso el contenedor guarda su `channel` (el que ya recibe `NewContainer` desde `channelForCommand`) y `Close` solo ejecuta `optimize` cuando el canal no es `"hook"`. Lo ejecutan el servidor MCP, la TUI y los comandos de la CLI.
- **Verificación**:
  - Una prueba determinista comprueba que `Close` devuelve nil y que la sentencia es válida sobre una base preparada. No se exige `sqlite_stat1`, porque `optimize` decide con heurísticas si analiza una base pequeña.
  - El coste se mide en Windows.

## R12. Integridad referencial (FR-014)

- **Decision**: `foreign_keys` sigue desactivado.
- **Rationale**: el esquema declara 9 `FOREIGN KEY` (relaciones, revisiones, hallazgos, rondas) que nunca se aplicaron. Hay limpieza de relaciones huérfanas (`TestMaintenancePurgeCleansOrphanRelations`), así que puede haber datos que no la cumplen. Activarla haría fallar borrados e inserciones que hoy funcionan. Se propone como feature aparte, con auditoría de huérfanos y orden de borrado.

## R13. Omitir la preparación de una base al día (FR-018)

- **Decision**: constante `schemaVersion = 1` en `db.go`. El flujo de `migrate(db)` es:
  1. `SELECT` de `PRAGMA user_version` con la conexión del pool, sin transacción. Si es mayor o igual que `schemaVersion`, termina. Una versión mayor viene de un binario más nuevo y nunca se baja (análisis I1).
  2. Si no, `BeginTx` (`BEGIN IMMEDIATE`).
  3. Vuelve a leer `user_version` dentro de la transacción. Si ya es mayor o igual, otro proceso preparó la base: `Rollback` y termina.
  4. `migrateSchema(tx)`.
  5. `PRAGMA user_version = <schemaVersion>` dentro de la transacción. Se construye con `fmt.Sprintf`, porque PRAGMA no admite parámetros y el valor es una constante del código, no una entrada (§5, análisis I5).
  6. `Commit`.
- **Evidencia** (prototipo con modernc v1.58.0, retirado después):
  - Con otra conexión manteniendo un `BEGIN IMMEDIATE` con un `INSERT` sin confirmar, leer `PRAGMA user_version` tardó 0,17 ms.
  - `BEGIN IMMEDIATE` desde esa conexión esperó 337 ms, hasta que el escritor hizo rollback.
  - `PRAGMA user_version = 7` dentro de una transacción con rollback dejó 0; con commit, otra conexión leyó 7.
- **Rationale**: sin el atajo, cada apertura de una base preparada pediría el bloqueo de escritura y podría esperar hasta 5 s detrás de otro escritor (el MCP, el grafo, el mantenimiento). Superaría el plazo de 2 s de los hooks y reproduciría la pérdida de US1 en bases antiguas (análisis U1).
- **Bases y binarios anteriores**:
  - Las bases existentes tienen `user_version=0`. Se preparan una vez y quedan en 1.
  - Un binario anterior a esta feature que abre una base en 1 o 2 no lee `user_version` y aplica su propia migración aditiva, que es un subconjunto. No toca `user_version`, así que al volver a la versión nueva la base sigue al día.
  - Un binario de esta feature o posterior con un `schemaVersion` menor que el de la base la considera al día y no la toca (paso 1 con `>=`). Así se evita que dos versiones se alternen migrando la misma base.
- **Riesgo**: cambiar el esquema sin subir `schemaVersion` dejaría las bases existentes sin las estructuras nuevas. Lo impide la prueba de R7 (FR-019).
- **Alternatives considered**: `BEGIN DEFERRED`, que solo toma el bloqueo al escribir. Se descarta porque el `_txlock=immediate` del DSN se aplica a todas las transacciones del pool, y porque el primer `CREATE` de una base nueva promovería de lectura a escritura, la carrera que la feature 035 resolvió con `immediate`.

## R14. Una sola forma de abrir SQLite (FR-012)

- **Decision**: `sqliteDSN(path, busyMs)` es el único constructor de DSN. Una prueba de guarda (`TestSQLiteOpen_OnlyThroughSqliteDSN`) recorre los `.go` de `adapters/secondary/persistence/` que no son de prueba y falla si alguna llamada `sql.Open("sqlite", …)` no recibe `sqliteDSN(…)`.
- **Rationale**: los pragmas son por conexión y no se pueden observar después de que `MigrateLegacy` cierre la suya. Una guarda estática cubre `MigrateLegacy` y cualquier apertura futura (análisis G3).
- **Cambio intencionado en `MigrateLegacy` (análisis I4)**: hoy su DSN pone `journal_mode` antes de `busy_timeout` y no usa `_txlock=immediate`. El DSN común corrige ambos, de forma coherente con la feature 035: `busy_timeout` primero, para que pasar a WAL espere a un bloqueo, y transacciones inmediatas sin promoción de lectura a escritura.

## R15. Resultado de la primera medición transaccional (2026-10-23)

- **Evidencia:** run 39954797737, commit diagnóstico 1b44900, 20 repeticiones; detalles y agregados en `evidence/windows-timing.md` y `windows-transaction-spans.json`.
- **Resultado:** ambas pruebas US1 pasan 20/20 y fold máximo es 335 ms. Sin embargo, una base nueva de tool-output Claude abre en 964 ms y no cumple SC-002. `TestHookToolOutput` pasa 19/20 y falla una vez por salida vacía de `claude-mcp.json`.
- **Decisión de la puerta:** detener el avance de implementación tras T014, conservando el trabajo independiente de US2 ya validado mientras corría CI. No elevar presupuestos ni atribuir causalidad sin evidencia.
- **Cierre propuesto:** aislar en un diagnóstico la primera apertura de tool-output, separando apertura de conexión/WAL, lectura de versión, inicio de transacción, DDL y commit, y comparar con el mismo muestreo sin competir con otros tests. R9 está diseñado pero aún no aplicado; cualquier adelanto de esa fase debe quedar explícito en el plan. Volver a ejecutar la medición de aceptación de 20 repeticiones y exigir SC-002 antes de continuar.

### Preparación de R15: medición por fase

Instrumentación adicional preparada solo en el worktree de diagnóstico; no cambia SQL, pragmas ni plazos. `migration.connect` incluye la primera conexión real y los pragmas del DSN, porque `sql.Open` es perezoso. `migration.lock` mide `BeginTx`; `migration.schema`, DDL; `migration.commit`, la confirmación. `dbfile.create`, `dbfile.chmod` y el total completan el desglose. La marca `dbfile.created` permite identificar bases nuevas sin deducirlo de su posición en la tanda ni registrar rutas privadas.

El workflow mantiene la tanda de aceptación conjunta y añade después 20 ejecuciones aisladas de tool-output, incluso si la primera tanda falla, con registros separados. Comparar ambas permite observar el efecto de la competencia entre paquetes sin eliminar la prueba original. Validación local completada; falta publicar y ejecutar en Windows para atribuir la demora.

### Resultado del diagnóstico por fase (T039)

Run 40043142611: ambas pruebas US1 20/20; fold máximo 282 ms. Conjunta: ToolOutput 19/20, base nueva máxima 1.435 ms, dividida en primera conexión/pragmas 1.035 ms y commit 383 ms; DDL 16 ms y BEGIN 0 ms. Otra apertura tarda 850 ms, con commit de 664 ms. Aislada: ToolOutput 20/20, base nueva máxima 82 ms; bases existentes máximas 6 ms. Las marcas identifican archivos nuevos explícitamente. Detalles y recuentos en evidence.

**Siguiente experimento:** R9 `synchronous=NORMAL` únicamente en la rama diag, activado con `GOMEMORY_DIAG_NORMAL=1`, conservando un control FULL y añadiendo una tanda conjunta NORMAL de 20 repeticiones en el mismo run. El interruptor experimental no se integra. La rama de feature conserva la configuración anterior hasta que la aceptación permita continuar. No se atribuye todavía la pausa de primera conexión a fsync, antivirus o scheduler: esa causa no está medida.
