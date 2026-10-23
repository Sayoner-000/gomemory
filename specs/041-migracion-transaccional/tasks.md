# Tasks: Base de memoria optimizada, idempotente y con índices

**Input**: Design documents from `specs/041-migracion-transaccional/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/apertura-de-base.md, quickstart.md

**Tests**: SÍ. La constitución (§10) exige pruebas y el proyecto trabaja con TDD: cada prueba se escribe primero y se comprueba que falla (rojo) antes de implementar. Las pruebas de compatibilidad y tolerancia (T005, T006 y T020) protegen un comportamiento que ya existe, así que deben pasar antes y después del cambio. Ninguna prueba compara tiempos de reloj real (§10); las cifras se miden en Windows CI con la instrumentación de diagnóstico.

**Idioma (§16, análisis D3)**: los nombres de las pruebas, los identificadores nuevos, los mensajes de fallo de las pruebas (`t.Errorf`/`t.Fatalf`) y los mensajes de commit van en inglés. Los comentarios siguen el idioma del archivo que los rodea. La documentación, la spec y las tareas van en español.

**Excepción §6**: registrada el 2026-10-23 (constitución §25.1 y `docs/adr/0001-migracion-al-abrir-la-base.md`). `.specify/` no se versiona: el registro que viaja con el repo es el ADR.

**Revisión 3 (2026-10-23)**: incorpora el 2.º `/speckit-analyze`:
- **D1:** la prueba de contención no depende del reloj.
- **D2:** la excepción está registrada.
- **D3:** pruebas y commits en inglés.
- **G1:** `optimize` solo fuera del canal `hook`, con su prueba.
- **I1:** atajo con `>=` y prueba de que la versión no baja.
- **O1:** `schemaVersion = 0` declarado en T008.
- **I2, I3, I5, F2, T1:** ajustes menores de referencias, marcas `[P]`, comentario del PRAGMA, conflicto esperado y nombre de prueba.

La revisión 2 incorporó el 1.er análisis (U1–U3, C2, G1–G4, I1–I4, I7, I8 y A2).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizable (archivos distintos, sin dependencias pendientes)
- **[Story]**: US1–US4 según spec.md

## Path Conventions

Monolito Go hexagonal en la raíz. El código de producción cambia en `adapters/secondary/persistence/` y `infrastructure/container.go`. Las pruebas unitarias van junto al código, porque usan funciones no exportadas (`migrate`, `openDBFile`). Las referencias R*, FR-* y SC-* remiten a `research.md` y `spec.md`.

**Regla de firmas**: `migrate(db *sql.DB) error` y `openDBFile(path string)` conservan su firma, porque 5 pruebas existentes llaman a `migrate(db)`: `db_compression_migration_test.go:34`, `instincts_test.go:68` y `socratica_test.go:24`, `:311` y `:502`. Las aserciones existentes se conservan; la excepción autorizada para tres fixtures se registra en el estado de ejecución.

**Regla de commits**: todo commit o push espera la aprobación explícita de la persona, con la lista de archivos y el mensaje en inglés (Conventional Commits). Nunca se incluyen CLAUDE.md, `.env*` ni certificados, y no se añade trailer de IA.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: dejar la referencia del estado anterior antes de tocar código.

- [X] T001 [P] Crear `specs/041-migracion-transaccional/evidence/windows-timing.md` con la tabla de línea base de spec.md («Contexto y evidencia», run 39941668694) y las columnas «Después (transacción)» y «Después (con R9 e índice)» vacías. Añadir las filas `close` (sin línea base) y «apertura de base al día con escritor abierto» (SC-010, sin línea base).
- [X] T002 [P] Crear `adapters/secondary/persistence/db_schema_golden_test.go` con:
  - `var updateSchemaGolden = flag.Bool("update-schema-golden", false, "rewrite testdata/schema.golden")`.
  - `schemaCatalog(t, db) string`. Primera línea: `user_version=<N>`. Después, ordenadas por `type, name`, las líneas `type|name|tbl_name|sql` de `sqlite_master`; los autoindex, cuyo `sql` es NULL, se listan con el sql vacío. Para cada `type='table'`, las líneas `col|<table>|cid|name|type|notnull|dflt_value|pk` de `pragma_table_info`.
  - `TestSchemaGolden`: abre una base nueva con `openDBFile(filepath.Join(t.TempDir(),"mem.db"))` y calcula `schemaCatalog`. Sin el flag, la compara con `testdata/schema.golden` y falla con el diff (mensaje en inglés). Con el flag, escribe el archivo. Nunca hace `t.Skip`.
- [X] T003 Generar el golden con el código SIN cambios (`go test ./adapters/secondary/persistence/ -run '^TestSchemaGolden$' -count=1 -args -update-schema-golden`). Confirmar que `testdata/schema.golden` empieza por `user_version=0` y que, sin el flag, `TestSchemaGolden` pasa (dep: T002).

**Checkpoint**: la referencia de FR-008 está fijada antes de cualquier cambio de producción.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: preparación en una sola transacción, versionada con `user_version` (FR-001, FR-002, FR-005, FR-006, FR-018, FR-019). Sostiene US1, US2 y US3.

**⚠️ CRITICAL**: ninguna historia empieza hasta que esta fase pase.

### Tests (rojo primero salvo T005 y T006)

- [X] T004 Escribir `TestMigrate_FailureMidwayLeavesNothing` en `adapters/secondary/persistence/db_migrate_atomic_test.go`:
  1. Crear un SQLite vacío con `sql.Open("sqlite", path)` y ejecutar `CREATE VIEW memories AS SELECT 1 AS id`.
  2. Llamar a `openDBFile(path)`: debe devolver error. El `CREATE INDEX … ON memories` del esquema base falla sobre una vista (R4).
  3. Reabrir con `sql.Open` y comprobar que NO existen `instinct_stats` ni `socratica_flows` en `sqlite_master` y que `PRAGMA user_version` sigue en 0.

  Rojo hoy: esas tablas quedan creadas.
- [X] T005 Escribir `TestMigrate_ToleratesDuplicateColumn` en `adapters/secondary/persistence/db_migrate_atomic_test.go`: abrir dos veces la misma base con `openDBFile`, cerrando entre una y otra, y comprobar que la segunda no devuelve error y que existe `memories.topic_key` (FR-005). Verde antes y después (dep: T004, mismo archivo).
- [X] T006 Escribir `TestMigrate_ToleratesUniqueIndexOverDuplicates` en `adapters/secondary/persistence/db_migrate_atomic_test.go`:
  1. Crear una base con el `CREATE TABLE memory_relations` original de `db.go`, sin las columnas añadidas después.
  2. Insertar dos filas idénticas con `project='p'`, `memory_id_a=1` y `memory_id_b=2`.
  3. Llamar a `openDBFile`: no debe devolver error. Las 2 filas siguen, `idx_relations_pair` no existe y el resto del esquema sí, por ejemplo `memories` y `sessions` (FR-006).

  Verde antes y después (dep: T005, mismo archivo).
- [X] T007 [P] Escribir `TestSchemaGolden_VersionBumpsWithCatalog` en `adapters/secondary/persistence/db_schema_golden_test.go` (FR-019). Si el catálogo actual (sin la línea `user_version`) difiere del de `testdata/schema.golden` y `schemaVersion` es igual a la versión del golden, falla con «schema changed without bumping schemaVersion». Además, con `-update-schema-golden`, `TestSchemaGolden` se niega a escribir en ese mismo caso. Compila desde T008 (O1).

### Implementation

- [X] T008 En `adapters/secondary/persistence/db.go`:
  - Declarar `const schemaVersion = 0`, el estado actual (O1). Con eso, T007 y las demás pruebas compilan y fallan por su aserción, no por la compilación.
  - Declarar `type schemaExecer interface { Exec(query string, args ...any) (sql.Result, error) }`.
  - Renombrar el cuerpo actual de `migrate` a `migrateSchema(ex schemaExecer) error`, cambiando cada `db.Exec(` por `ex.Exec(`.
  - Hacer que `addColumnIfMissing` reciba `ex schemaExecer`.

  Sin otros cambios de SQL. Ejecutar T004 a T007: T005 y T006 en verde, T004 en rojo y T007 en verde (el catálogo aún no cambia).
- [X] T009 En `adapters/secondary/persistence/db.go`, subir `schemaVersion` a 1 y reescribir `migrate(db *sql.DB) error` (R13) (dep: T008):
  1. Leer `PRAGMA user_version` con `db.QueryRow`. Si es `>= schemaVersion`, `return nil` (I1: una versión mayor la dejó un binario más nuevo y nunca se baja).
  2. `tx, err := db.BeginTx(context.Background(), nil)` (`BEGIN IMMEDIATE`).
  3. Leer `PRAGMA user_version` con `tx.QueryRow`. Si ya es `>=`, `tx.Rollback()` y `return nil`.
  4. `migrateSchema(tx)`. Si falla, `Rollback` y devolver el error.
  5. `tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))`, con un comentario de una línea: PRAGMA no admite parámetros y el valor es una constante (I5).
  6. `tx.Commit()`.

  Comentario de cabecera: el esquema sigue siendo solo aditivo; la transacción única tiene el motivo medido (R1); el atajo existe para no pedir el bloqueo en bases al día (U1); cada cambio del catálogo obliga a subir `schemaVersion`; y la excepción §6 está en el ADR 0001. `migrateWithRetry` no cambia.
- [X] T010 Ejecutar `go test ./adapters/secondary/persistence/ -count=1` (dep: T009). Deben pasar:
  - T004 a T007;
  - las existentes `TestOpenDBFile_AperturaConcurrenteDeUnaBaseNueva`, `TestMigrate_IsIdempotent`, `TestMigrateCompression_BaseVaciaYAnterior`, `TestReviewSchemaMigrationIsAdditive` y `TestMigracion028EsAditivaSobreEsquemaPrevio`.

  `TestSchemaGolden` debe fallar SOLO por `user_version=0` frente a 1. Guardar ese diff en `specs/041-migracion-transaccional/evidence/schema-diff.txt` y regenerar el golden con `-update-schema-golden`.

**Checkpoint**: la preparación es todo o nada, versionada y tolerante.

---

## Phase 3: User Story 1 - El primer turno en un proyecto nuevo no pierde trabajo (Priority: P1) 🎯 MVP

**Goal**: el primer hook en un proyecto nuevo registra y consolida sin perder nada en Windows (SC-001, SC-002, SC-003).

**Independent Test**: en Windows CI, 20 ejecuciones seguidas de `TestHookExpandInstinctsTwoProjects` y `TestHookExpand_SuggestionOncePerEpisodeAndRecorded` pasan 20 de 20, y `db.open` en una base nueva baja de 400 ms en el peor caso.

**Nota**: US1 no tiene código propio, lo entrega la fase Foundational. Esta fase lo demuestra en el entorno donde fallaba (regla de trabajo 2).

- [X] T011 [US1] Proponer a la persona el commit de las fases 1 y 2 en `041-migracion-transaccional`. Lista de archivos: `db.go`, las pruebas nuevas, `testdata/schema.golden`, `specs/041-migracion-transaccional/` y `docs/adr/0001-migracion-al-abrir-la-base.md`. Mensaje: `perf(persistence): prepare the memory database in a single versioned transaction`. Hacer commit solo tras su aprobación (dep: T010).
- [ ] T012 [US1] Crear la rama `diag/windows-timing-041` a partir de `041-migracion-transaccional` y aplicar `git cherry-pick 3e81733`. En `infrastructure/container.go`, resolver los conflictos conservando las marcas `db.open.*`. Añadir una marca `close` alrededor del cuerpo de `Container.Close` (dep: T011).
- [ ] T013 [US1] En `.github/workflows/diag-windows-timing.yml` de esa rama, cambiar `on.push.branches` a `["diag/windows-timing-041"]` y `-count=5` por `-count=20`. Añadir `TestMigrate_UpToDateDBDoesNotWaitForOpenWriter` al patrón `-run` cuando exista (T017), con una marca `contention.open` en esa prueba solo en esta rama. Proponer el push a la persona con la URL remota (`https://github.com/Sayoner-000/gomemory.git`). Tras su aprobación, hacer push y esperar el run con `gh run watch` (dep: T012).
- [ ] T014 [US1] Agregar el registro del run con el script de la línea base (spans `db.open`, `fold`, `flush`, `toolout` y `close` por comando). Completar la columna «Después (transacción)» de `evidence/windows-timing.md` con los recuentos PASS/FAIL por test (SC-001), el `db.open` máximo en una base nueva (SC-002, < 400 ms) y el `fold` máximo (SC-003, < 500 ms) (dep: T013).

**Checkpoint**: si SC-001, SC-002 o SC-003 no se cumplen, se detiene la feature y se vuelve a research con los datos. No se suben plazos (FR-010).

---

## Phase 4: User Story 2 - La preparación es idempotente y de todo o nada (Priority: P1)

**Goal**: repetir la preparación no cambia nada; un fallo no deja nada; varios pools a la vez terminan bien; una base al día no espera a otros escritores y su versión nunca baja (FR-001, FR-003, FR-007, FR-018, SC-009, SC-010).

**Independent Test**: `go test ./adapters/secondary/persistence/ -run 'ThreeOpens|ConcurrentPools|FailureMidway|OpenWriter|NotDowngraded' -count=3`.

- [ ] T015 [P] [US2] Escribir `TestMigrate_ThreeOpensSameCatalog` en `adapters/secondary/persistence/db_schema_golden_test.go`: abrir la misma base 3 veces con `openDBFile`, cerrando entre una y otra, y comprobar que `schemaCatalog` da el mismo texto en las 3 (SC-009) (dep: T007, mismo archivo).
- [ ] T016 [P] [US2] Escribir `TestMigrate_ConcurrentPoolsNewDBWithWrites` en `adapters/secondary/persistence/db_migrate_concurrency_test.go` (T1: son pools `*sql.DB` independientes, que reproducen el bloqueo entre procesos; el caso de procesos reales lo cubre la prueba existente). 8 goroutines abren a la vez, cada una con su propio `openDBFile`, la misma ruta nueva, y cada una inserta una memoria con `InsertMemory`. Ninguna falla, hay 8 filas y `user_version = schemaVersion` (FR-007).
- [ ] T017 [P] [US2] Escribir `TestMigrate_UpToDateDBDoesNotWaitForOpenWriter` en `adapters/secondary/persistence/db_migrate_contention_test.go` (U1 y D1, FR-018). Sin comparar tiempos de reloj (§10):
  1. Preparar una base con `openDBFile` y cerrarla.
  2. Abrir una conexión `w` con el mismo DSN que `openDBFile`; tras T026, `sqliteDSN(path, 5000)` (I2). Hacer `tx, _ := w.BeginTx` y un `INSERT` en `memories` sin confirmar.
  3. En una goroutine, `openDBFile(path)` y enviar el resultado a `done`.
  4. Esperar en `select` a `done` o a `time.After(3*time.Second)`, menor que el `busy_timeout` de 5 s. Este tope solo evita que el test se cuelgue; no mide rendimiento. Si llega `done` mientras `tx` sigue abierta, es verde, con `err == nil`. Si vence el tope, es rojo: «opening an up-to-date database waited for the open writer».
  5. `tx.Rollback()` siempre al final.

  Verificar que es rojo con un `BEGIN IMMEDIATE` incondicional.
- [ ] T018 [P] [US2] Escribir `TestMigrate_NewerUserVersionIsNotDowngraded` en `adapters/secondary/persistence/db_migrate_contention_test.go` (I1): preparar una base, fijar `PRAGMA user_version = schemaVersion + 5`, cerrar, abrir con `openDBFile` y comprobar que `user_version` sigue en `schemaVersion + 5` (dep: T017, mismo archivo).
- [ ] T019 [US2] Ejecutar el Independent Test de US2 con `-count=3`: verde las 3 veces (dep: T015, T016, T017, T018).

**Checkpoint**: US2 verificada de forma independiente y sin depender del reloj.

---

## Phase 5: User Story 3 - Las memorias existentes se abren igual que antes (Priority: P1)

**Goal**: el esquema resultante es el de la versión anterior más los índices permitidos, y las bases antiguas conservan sus datos (FR-004, FR-008, SC-005).

**Independent Test**: `go test ./adapters/secondary/persistence/ -run 'SchemaGolden|LegacyDB|Aditiv|Migracion028|Compression_BaseVacia' -count=1`.

- [ ] T020 [P] [US3] Escribir `TestMigrate_LegacyDBKeepsData` en `adapters/secondary/persistence/db_migrate_legacy_test.go`:
  1. Crear una base con las columnas originales, tomadas del `CREATE TABLE` real sin las columnas añadidas después: `CREATE TABLE memories(id INTEGER PRIMARY KEY AUTOINCREMENT, project TEXT NOT NULL, session_id TEXT, type TEXT NOT NULL DEFAULT 'learning', title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL, filepath TEXT, created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')` y `CREATE TABLE sessions(id TEXT PRIMARY KEY, project TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', ended_at TEXT)`.
  2. Insertar 1 memoria y 1 sesión.
  3. Abrir con `openDBFile` y comprobar que las 2 filas siguen iguales, que existen `memories.topic_key` y `sessions.last_prompt` y que `user_version = schemaVersion` (FR-004, SC-005).

  Verde antes y después, salvo la aserción de `user_version`, que entra con T009.
- [ ] T021 [US3] Ejecutar el Independent Test de US3: todo en verde (dep: T020).

**Checkpoint**: compatibilidad garantizada.

---

## Phase 6: User Story 4 - Cada turno consulta y escribe con el menor coste posible (Priority: P2)

**Goal**: configuración de R9 en todas las aperturas, índice de sesiones, prueba de planes, `optimize` al cerrar fuera de los hooks y durabilidad (FR-011 a FR-017, SC-007, SC-008).

**Independent Test**: `go test ./adapters/secondary/persistence/ ./infrastructure/ -run 'Pragmas|QueryPlans|OnlyThroughSqliteDSN|Durability|ContainerClose|SchemaGolden|SessionsIndex' -count=1`.

### Tests (rojo primero)

- [ ] T022 [P] [US4] Escribir `TestOpenDBFile_PragmasOnEveryConnection` en `adapters/secondary/persistence/db_pragmas_test.go`:
  1. Abrir con `openDBFile` y tomar 3 conexiones a la vez con `db.Conn(ctx)`, sin soltarlas, para forzar conexiones nuevas del pool.
  2. En cada una, comprobar `busy_timeout`=5000, `journal_mode`=wal, `synchronous`=1, `cache_size`=-16000, `temp_store`=2, `mmap_size`=268435456, `journal_size_limit`=67108864 y `foreign_keys`=0 (FR-011, FR-014, SC-007).
  3. Para `OpenInstinctGlobal(t.TempDir())`, que tiene `SetMaxOpenConns(1)`, comprobar los mismos pragmas en UNA sola conexión, con `busy_timeout`=100 (FR-012).

  Rojo hasta T026.
- [ ] T023 [P] [US4] Escribir `TestSQLiteOpen_OnlyThroughSqliteDSN` en `adapters/secondary/persistence/sqlite_open_guard_test.go` (R14):
  1. Recorrer con `go/parser` los `.go` del paquete que no terminan en `_test.go`.
  2. Para cada `sql.Open` con primer argumento `"sqlite"`, exigir que el segundo sea una llamada a `sqliteDSN`.

  Rojo hasta T026 (hoy hay tres aperturas con DSN literal).
- [ ] T024 [P] [US4] Escribir `TestQueryPlans_HotQueriesNeverScan` en `adapters/secondary/persistence/db_query_plans_test.go`. Sobre una base nueva con `openDBFile`, ejecutar `EXPLAIN QUERY PLAN` de cada consulta de la lista vigilada y fallar si alguna fila de `detail` empieza por `SCAN ` seguido de una tabla que no sea virtual FTS5 (`code_search`, `memory_search`). Cada entrada lleva un comentario `// archivo:línea` (FR-016). La lista:
  - `SELECT COUNT(*) FROM sessions WHERE project = ? AND created_at >= ?` (channel_activity.go:102)
  - `SELECT id, project, COALESCE(summary,''), created_at, ended_at FROM sessions WHERE project = ? AND ended_at IS NULL ORDER BY created_at DESC, rowid DESC LIMIT 1` (session.go:102)
  - `SELECT id, project, COALESCE(summary,''), created_at, COALESCE(ended_at,'') FROM sessions WHERE project = ? ORDER BY created_at DESC, rowid DESC LIMIT ?` (session.go:126)
  - `SELECT last_prompt FROM sessions WHERE project = ? AND ended_at IS NULL ORDER BY created_at DESC, rowid DESC LIMIT 1` (memory.go:508)
  - `SELECT id FROM memories WHERE project = ? AND topic_key = ? ORDER BY id DESC LIMIT 1` (memory.go:381)
  - `SELECT id FROM memories WHERE project = ? AND type = ? AND title = ?` (memory.go:418; copiar las columnas del SELECT real)
  - `SELECT id FROM memories WHERE project = ? AND session_id = ? AND id <> ? AND type <> ?` (memory.go:462)
  - `SELECT id FROM memories WHERE project = ? ORDER BY created_at DESC, id DESC LIMIT ?` (memory.go:615)
  - `SELECT path, hash FROM code_files WHERE project = ?` (code_graph.go:12)
  - `SELECT id, project, kind, name, package, file, receiver, signature, start_line, end_line, exported FROM code_nodes WHERE project = ? AND name = ?` (code_graph.go:307)
  - `DELETE FROM code_edges WHERE src_file_id = ?` (code_graph.go:70)
  - `SELECT id FROM findings WHERE reviewer_result_id = ? ORDER BY id`
  - `SELECT COUNT(*) FROM fix_rounds WHERE review_id = ?`
  - `SELECT status FROM reviewer_results WHERE review_id = ? AND reviewer = ? AND round = ?`
  - `SELECT content_hash FROM context_deliveries WHERE session_id = ? AND kind = ?`
  - `SELECT block_hash FROM delivered_blocks WHERE session_id = ? AND block_hash IN (?)`
  - `SELECT session_id FROM usage_records WHERE project = ? ORDER BY id ASC`
  - la consulta completa de `GuardEventsSince` (channel_activity.go:179), copiada literal
  - `SELECT slug FROM instinct_stats WHERE project = ? AND status != 'rejected' ORDER BY confidence DESC, slug`

  En las cuatro últimas sin `archivo:línea` (findings, fix_rounds, reviewer_results e instinct_stats), localizar la línea real con `grep` y anotarla. Rojo hasta T027, por las cuatro consultas de `sessions`.
- [ ] T025 [P] [US4] Escribir `TestContainerClose_HookChannelSkipsOptimize` en `infrastructure/container_close_test.go` (G1):
  1. Con `GOMEMORY_DATA_HOME` aislado, `NewContainer(root, "hook")`, insertar 3 memorias y llamar a `Close()`.
  2. Reabrir la base con `sql.Open` y comprobar que NO existe `sqlite_stat1`. Una base nueva no lo tiene hasta que corre `ANALYZE` u `optimize`, así que su ausencia demuestra que el cierre con el canal `hook` no ejecutó `optimize`.

  Rojo si T028 ejecuta `optimize` sin mirar el canal.

### Implementation

- [ ] T026 [US4] Crear `adapters/secondary/persistence/sqlite_dsn.go` con `func sqliteDSN(path string, busyMs int) string`. Devuelve `path + "?_pragma=busy_timeout(<busyMs>)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-16000)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(268435456)&_pragma=journal_size_limit(67108864)&_txlock=immediate"`, con `busy_timeout` siempre primero (R9). El comentario de cabecera explica el motivo de cada pragma en una línea y la contrapartida de `synchronous=NORMAL`. Usarlo en las tres aperturas:
  - `openDBFile` en `db.go` (5000; conservar el comentario sobre el orden y `_txlock`);
  - `MigrateLegacy` en `globalstore.go:316` (5000; cambio intencionado del orden de pragmas y de `_txlock`, R14/I4);
  - `OpenInstinctGlobal` en `instincts.go:194` (100).

  T022 y T023 deben pasar a verde.
- [ ] T027 [US4] En `migrateSchema` (`adapters/secondary/persistence/db.go`), junto a los demás `CREATE INDEX` de sesiones y memorias, añadir `CREATE INDEX IF NOT EXISTS idx_sessions_project_created ON sessions(project, created_at);` con un comentario que cite el `SCAN sessions` de R10. El índice NO es único (FR-017). Subir `schemaVersion` a 2 y regenerar `testdata/schema.golden` con `-update-schema-golden`. Comprobar con `git diff` que el diff del golden es solo la línea del índice y `user_version=2`, y añadirlo a `evidence/schema-diff.txt` (FR-008, FR-019). T024 debe pasar a verde (dep: T009).
- [ ] T028 [US4] En `infrastructure/container.go` (G1):
  - Añadir el campo `channel string` a `Container` y asignarlo en `NewContainer(root, channel)`.
  - En `Close`, antes de `c.db.Close()`, `if c.channel != "hook" { _, _ = c.db.Exec("PRAGMA analysis_limit=400; PRAGMA optimize;") }`, con un comentario de una línea: los hooks llegan a `Close` por el `defer closeContainer` de `main` y tienen un plazo de 2 s (R11).

  El error se ignora a propósito y el valor de retorno de `Close` no cambia (FR-013). T025 debe seguir en verde.

### Tests (tras implementar)

- [ ] T029 [P] [US4] Escribir `TestMigrate_LegacyDBGainsSessionsIndex` en `adapters/secondary/persistence/db_migrate_legacy_test.go`: misma base anterior de T020; tras `openDBFile`, existe `idx_sessions_project_created` y `user_version = schemaVersion` (FR-004) (dep: T027, T020).
- [ ] T030 [P] [US4] Escribir `TestContainerClose_NoErrorAndValidStatement` en `infrastructure/container_close_test.go`:
  1. `NewContainer(root, "cli")` con `GOMEMORY_DATA_HOME` aislado, 3 memorias y `Close()` devuelve nil.
  2. Abrir otra base con `persistence.Open` y comprobar que `db.Exec("PRAGMA analysis_limit=400; PRAGMA optimize;")` devuelve nil.

  No se comprueba `sqlite_stat1` con el canal `cli`, porque `optimize` decide con heurísticas (R11) (dep: T028, T025, mismo archivo).
- [ ] T031 [P] [US4] Escribir `TestDurability_CommitSurvivesAbruptExit` en `adapters/secondary/persistence/db_durability_test.go` (US4-AC4):
  1. El test se relanza a sí mismo con `exec.Command(os.Args[0], "-test.run=^TestDurability_CommitSurvivesAbruptExit$")` y `GOMEMORY_DURABILITY_CHILD=<path>`.
  2. En el hijo: `openDBFile`, `InsertMemory` y `os.Exit(0)` sin `Close`.
  3. El padre espera al hijo, reabre con `openDBFile` y comprueba que la memoria está y que `PRAGMA integrity_check` devuelve `ok` (dep: T026).

**Checkpoint**: todas las aperturas con R9, sin `SCAN` en la lista vigilada, índice versionado, `optimize` fuera de los hooks y durabilidad comprobada.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T032 Ejecutar `gofmt -l .`, `go vet ./...`, `GOOS=windows go vet ./...` y `go test ./... -count=1`: todo en verde (SC-006).
- [ ] T033 Proponer a la persona el commit de US2 a US4 (archivos y mensaje `perf(persistence): tune SQLite pragmas, index sessions and refresh stats on close`). Hacer commit solo tras su aprobación. Después, en `diag/windows-timing-041`, rebasar sobre `041-migracion-transaccional`. Habrá un conflicto esperado en `Container.Close` entre la marca `close` (T012) y el `optimize` (T028) (F2): conservar las dos. Proponer el push (dep: T032).
- [ ] T034 Tras el run, completar la columna «Después (con R9 e índice)» de `evidence/windows-timing.md` con estos objetivos:
  - `db.open` en una base nueva (SC-002, < 400 ms);
  - `db.open` en una base preparada (SC-004, ≤ 10 ms p50);
  - `contention.open` (SC-010, < 50 ms);
  - `close` con canal distinto de `hook` (FR-013, ≤ 100 ms máx), y que en los hooks no crece;
  - `fold` máximo (SC-003, < 500 ms);
  - `flush` en pre-tool, como referencia, frente a 42 ms p50 de la línea base (dep: T033).
- [ ] T035 [P] Añadir a `docs/DATABASE.md` una sección «Configuración y preparación de mem.db» con:
  - los pragmas de R9 y su motivo;
  - `user_version` con `>=` y la regla de subir `schemaVersion` con cada cambio de esquema;
  - la preparación en una sola transacción;
  - el índice de sesiones;
  - la regla de revisión de FR-016: añadir a `TestQueryPlans_HotQueriesNeverScan` cada consulta nueva de turno, sesión o hook;
  - `optimize` solo fuera de los hooks;
  - la contrapartida de `synchronous=NORMAL`;
  - un enlace al ADR 0001.
- [ ] T036 [P] Añadir en `CHANGELOG.md`, bajo «Unreleased», una entrada `### Changed` con la preparación atómica y versionada, los pragmas, el índice y las cifras de T034.
- [ ] T037 Guardar en gomemory (`save_memory`, `type=decision`, `topic_key=spec-041-db-optimizada`) el resultado final medido, la regla de `schemaVersion` y lo pendiente: R12 (FK) y que `.specify/` no se versiona, así que la excepción §25.1 vive en el ADR (dep: T034).
- [ ] T038 Ejecutar el CI completo (`test.yml`) en `041-migracion-transaccional` tras el push aprobado y confirmar los tres jobs en verde antes de proponer el merge. La rama `diag/windows-timing-041` no se integra (dep: T032, T034).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Fase 1)**: T001 ∥ T002 → T003. Antes de cualquier cambio de producción.
- **Foundational (Fase 2)**: depende de Fase 1. T004 → T005 → T006, en el mismo archivo y en secuencia; T007 en paralelo con ellas. Después T008 → T009 → T010.
- **US1 (Fase 3)**: depende de Fase 2. T011 (commit con aprobación) → T012 → T013 → T014. Mientras corre CI se avanza en local en US2 a US4.
- **US2 (Fase 4)** y **US3 (Fase 5)**: dependen de Fase 2 y son independientes entre sí.
- **US4 (Fase 6)**: depende de Fase 2. T022 ∥ T023 ∥ T024 ∥ T025 → T026 → T027 → T028 → T029 ∥ T030 ∥ T031.
- **Polish (Fase 7)**: depende de todas.

### User Story Dependencies

- **US1, US2, US3**: solo Foundational.
- **US4**: Foundational. T017 (US2) usa `sqliteDSN` cuando T026 ya existe; si no, el DSN de `openDBFile`.

### Within Each User Story

- Pruebas primero, en rojo; después la implementación; al final la batería en verde.
- Cada cambio de catálogo (T009 y T027) sube `schemaVersion` y regenera el golden en la misma tarea.

### Parallel Opportunities

- T001 ∥ T002.
- T007 ∥ (T004 → T005 → T006).
- T015 ∥ T016 ∥ T017 ∥ T020 ∥ T022 ∥ T023 ∥ T024 ∥ T025, todas en archivos distintos.
- T029 ∥ T031. T030 va tras T025, porque comparten archivo.
- T035 ∥ T036.
- La medición de US1 en CI corre en paralelo con el trabajo local.

---

## Parallel Example: User Story 4

```bash
# Pruebas de US4 en paralelo (archivos distintos):
Task: "T022 TestOpenDBFile_PragmasOnEveryConnection en db_pragmas_test.go"
Task: "T023 TestSQLiteOpen_OnlyThroughSqliteDSN en sqlite_open_guard_test.go"
Task: "T024 TestQueryPlans_HotQueriesNeverScan en db_query_plans_test.go"
Task: "T025 TestContainerClose_HookChannelSkipsOptimize en infrastructure/container_close_test.go"

# Tras implementar, en paralelo:
Task: "T029 TestMigrate_LegacyDBGainsSessionsIndex"
Task: "T031 TestDurability_CommitSurvivesAbruptExit (subproceso)"
```

---

## Implementation Strategy

### MVP First (Foundational + User Story 1)

1. Fase 1: golden del esquema actual.
2. Fase 2: migración transaccional con `user_version` (`>=`, sin bloqueo para bases al día).
3. Fase 3: commit aprobado y medición en Windows CI con 20 repeticiones.
4. **STOP y VALIDAR**: si SC-001 a SC-003 se cumplen, el problema que bloqueaba el CI de Windows está resuelto.

### Incremental Delivery

1. MVP (Fases 1-3).
2. US2 y US3: idempotencia, contención sin reloj, versión que no baja, atomicidad y compatibilidad.
3. US4: optimización completa (pragmas, índice, planes, `optimize` fuera de los hooks y durabilidad).
4. Polish: segunda medición, documentación, changelog y CI completo.

---

## Notes

- Total: 38 tareas (T001–T038 más T025). Setup 3, Foundational 7, US1 4, US2 5, US3 2, US4 10, Polish 7.
- Excepción autorizada: tres fixtures antiguos restablecen `user_version=0`; sus aserciones se conservan. `migrate(db)` conserva su firma.
- Pendiente fuera de alcance, registrado en T037: R12 (FK).
- Las ramas `diag/*` no se integran en `main`.

## Estado de ejecución (2026-10-23)

T001–T011 completadas y marcadas. Commit de fases 1 y 2 autorizado expresamente por la persona; sin push.

- Golden original generado con producción sin cambios (`user_version=0`); tras T009 el catálogo es idéntico salvo `user_version=1`. Diff en `evidence/schema-diff.txt`.
- T004 falló antes con dos tablas residuales; después pasa. T005/T006 pasaron antes y después. T007 pasa con versión inicial 0 y después con versión 1.
- El primer pase de la batería completa detectó tres fixtures que simulaban bases antiguas borrando tablas sin restablecer la versión. La persona autorizó expresamente corregirlos: `TestMigrateCompression_BaseVaciaYAnterior`, `TestMigracion028EsAditivaSobreEsquemaPrevio` y `TestSocraticaRepository_Complementos/older_database_migrates_additively` ahora ejecutan `PRAGMA user_version=0` tras borrar tablas. Se conservan todas sus aserciones. Esta es la excepción acotada a la regla de no modificar pruebas existentes; no se cambia el contrato FR-018.
- Validación posterior: `go test ./adapters/secondary/persistence/ -count=1` en verde (14,795 s); `git diff --check` sin errores.
- T011: commit de fases 1 y 2 autorizado expresamente por la persona con el mensaje acordado. Siguiente tarea: T012, rama de diagnóstico Windows.
