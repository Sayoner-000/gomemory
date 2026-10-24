# Persistencia de gomemory

GoMemory usa SQLite mediante `modernc.org/sqlite`, sin CGO. Cada proyecto tiene una base `mem.db` aislada en el almacén global del usuario, bajo `projects/<clave>/`. La clave deriva de la ruta canónica del proyecto; el archivo `projects/<clave>/root` registra la ruta para que la desinstalación pueda encontrar las integraciones del proyecto.

`GOMEMORY_DATA_HOME` permite elegir la raíz del almacén. La caché de versiones (`update-check.json`) vive en esa raíz y no forma parte de la base SQLite. Los archivos privados de datos y exportación se crean con permisos de propietario (0600 para archivos, 0700 para directorios).

El inventario de componentes y sus dependencias está en [architecture.md](architecture.md). La evolución del esquema y las consultas viven en `adapters/secondary/persistence/`; las operaciones de aplicación acceden a ellas por los puertos de `application/ports/`.

## Texto protegido y saneamiento

Los campos de texto libre que pueden recibir contenido del usuario o del agente —memorias, `sessions.summary`, `sessions.last_prompt`, `memory_relations.reasoning`, `channel_activity.last_error` y el ledger de revisión (`reviews.target_scope`, `findings.claim`/`evidence`, `consensus_findings.claim`, `rejudgments.evidence`, `fix_rounds.verification`)— pasan por `domain.ProtectText` al escribirse y al leerse: se eliminan las regiones `<private>` (anidadas o sin cierre) y se ocultan las credenciales reconocidas. `compression_originals` rechaza guardar o devolver un original con contenido protegido. La lectura nunca reescribe filas.

Las filas guardadas antes de esa protección se sanean con `mem sanitize --apply` (ver el manual, sección 8). Las columnas JSON del ledger se protegen elemento a elemento, porque `json.Marshal` escapa `<` y un bloque privado no se ve en el texto serializado. No hay tabla ni migración nueva: el saneamiento actualiza solo el campo de texto dentro de una transacción inmediata con `secure_delete`, elimina los originales protegidos y después compacta (`VACUUM` + `wal_checkpoint(TRUNCATE)`) comprobando que el checkpoint no quedó bloqueado. Su informe depurado vive fuera de la base, en `<raíz>/maintenance/privacy/<run_id>.json`; los snapshots que reescribe están en `<raíz>/backups/<clave>/`.

## Socratica (feature 039)

La migración es aditiva (`CREATE … IF NOT EXISTS`): una base anterior se abre sin cambios previos.

- `socratica_flows`: un flujo de refinamiento por fila (`id` TEXT, UUIDv7). Guarda el original, las preguntas y respuestas (`questions_json`, `answers_json`), la propuesta, el texto aprobado, la síntesis de escrutinio (`scrutiny_json`), la fase y el estado del despacho (`dispatch_id`, `dispatch_ack`, `ack_nonce`, `dispatch_claimed_at`, `dispatch_claim_owner`, `approved_hash`, `dispatch_consumed`).
- `socratica_history`: una fila por flujo con original, propuesta, texto aprobado, adiciones y síntesis. Sobrevive a la purga del flujo.

Índices: `idx_socratica_one_pending` (único parcial: un solo flujo no terminal por `(project, client)`), `idx_socratica_dispatch` (único sobre `dispatch_id`), `idx_socratica_flows_scope` e `idx_socratica_history_scope`.

Las transiciones de despacho son sentencias condicionales: `approve` solo afecta a filas en `review` o `failed`; el reclamo exige que no exista otro; el ack exige el `dispatch_id` vigente y el propietario del reclamo; `ConsumeDispatch` compara el hash del texto y marca en la misma sentencia. Toda consulta lleva el timeout de `socratica_query_timeout_ms`, también con la base bloqueada por otro proceso. Junto a la base, `.memory/socratica/locks/<flujo>.lock` es el lock de archivo que retiene quien despacha; se borra al purgar el flujo.

Retención: historial podado a `socratica_history_limit` (20) por `(project, client)` en la misma transacción; `mem gc` borra los pendientes con más de 7 días (nunca un `approved` sin ack) y los terminados con más de 24 horas, y poda los terminados por encima del límite.

Privacidad: todas las columnas de texto y los strings dentro de los JSON pasan por `ProtectText` al escribir y al leer. `mem sanitize` las recorre por columna (categorías `socratica_flow_*` y `socratica_history_*`), respetando el proyecto. En modo privado no se escribe historial y un flujo enviado se redacta conservando solo `dispatch_id`, `approved_hash` y `dispatch_consumed`.

## Expand: estadísticas, idempotencia y registro global (040)

`instinct_stats` vive en el mem.db del proyecto. Su clave primaria `(project, slug)` identifica el patrón; conserva tipo, descripción redactada, observaciones, correcciones, confianza, alcance, estado y fechas. `instinct_folded`, con clave `(project, obs_id)`, registra los UUID ya contados en la misma transacción que actualiza las estadísticas. Se purga después de siete días.

`instinct_global` vive en `DataHome()/instincts.db`, con permisos 0600 y WAL. La clave es `slug`; guarda tipo, descripción, conjunto JSON de proyectos, confianza y fecha de actualización. Dos proyectos con confianza mínima de 0,5 permiten promoción global. Las tres tablas se crean de forma aditiva.

La cola transitoria `.memory/expand-observations.ndjson` tiene límite de 256 KiB. El plegado confirma hasta 500 observaciones y luego retira sus UUID de la cola; repetir un lote no incrementa los contadores. La reconciliación global ocurre después del commit y se reintenta por estado. `mem sanitize` informa por separado `instinct_stats`, `instinct_global` e `instinct_queue`.

## Configuración y preparación de mem.db

La apertura prepara el esquema de forma aditiva en una sola transacción inmediata. La versión se guarda en `PRAGMA user_version`: si es mayor o igual que `schemaVersion`, la apertura omite la transacción y el DDL, por lo que una base preparada no espera el bloqueo de otro escritor. Una versión mayor, escrita por un binario más nuevo, nunca se reduce. Dentro de la transacción se vuelve a leer la versión antes de crear estructuras, para cubrir aperturas concurrentes.

Cada cambio del catálogo exige aumentar `schemaVersion` y regenerar `adapters/secondary/persistence/testdata/schema.golden` en el mismo cambio. El golden incluye la versión, el catálogo y las columnas; su guarda rechaza cambios del catálogo sin aumento de versión. La versión 1 introduce la preparación transaccional y la versión 2 añade el índice no único `idx_sessions_project_created ON sessions(project, created_at)`, utilizado para contar sesiones, consultar la activa y listar las recientes. Un error no tolerado revierte toda la preparación y conserva la versión anterior. Las ampliaciones opcionales conservan sus tolerancias históricas; si fallan, no se vuelven a intentar hasta la siguiente subida de versión.

`sqliteDSN` aplica estos ajustes a cada conexión del pool, incluida la apertura del almacén legado durante su traslado:

| Ajuste | Valor | Motivo |
|---|---|---|
| `busy_timeout` | 5.000 ms; 100 ms en `instincts.db` global | Esperar bloqueos. Se aplica antes de activar WAL. |
| `journal_mode` | `WAL` | Permitir lectores mientras hay un escritor. |
| `synchronous` | `NORMAL` | Reducir las sincronizaciones de cada escritura en modo WAL. |
| `cache_size` | `-16000` | Caché de unos 16 MB, asignada bajo demanda. |
| `temp_store` | `MEMORY` | Evitar archivos temporales para ordenamientos y agregados. |
| `mmap_size` | `268435456` | Permitir hasta 256 MB de lectura mapeada donde esté disponible. |
| `journal_size_limit` | `67108864` | Limitar a 64 MB el WAL retenido tras un checkpoint. |
| `_txlock` | `immediate` | Tomar el bloqueo al comenzar las transacciones de escritura. |

Con WAL y `synchronous=NORMAL`, una caída del proceso conserva las escrituras confirmadas; un corte de energía puede perder la última escritura confirmada, sin comprometer la integridad de la base. La prueba de durabilidad comprueba la caída del proceso, no simula una pérdida de energía. `foreign_keys` continúa desactivado: activarlo requiere una auditoría de relaciones huérfanas y del orden de borrado. `instincts.db` conserva su esquema propio y `secure_delete=ON`.

Al cerrar un contenedor fuera del canal `hook`, se ejecuta `PRAGMA analysis_limit=400; PRAGMA optimize;` antes de cerrar `mem.db`. Es mantenimiento oportunista: su error no se propaga ni cambia el resultado del cierre. Los hooks lo omiten para respetar su plazo de 2 s.

Toda consulta nueva de turno, sesión o hook debe añadirse a `TestQueryPlans_HotQueriesNeverScan` con su archivo y línea de origen. La prueba usa `EXPLAIN QUERY PLAN` y rechaza recorridos completos de tablas, con la excepción de tablas virtuales FTS5.

La migración automática al abrir es una excepción documentada para esta CLI local: [ADR 0001](adr/0001-migracion-al-abrir-la-base.md). `.specify/` no se versiona; el ADR es el registro que acompaña al repositorio.
