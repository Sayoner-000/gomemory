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

### Preparación para Spec Kit (feature 042, catálogo 3)

La versión 3 es aditiva (`addColumnIfMissing`, dentro de la misma transacción de preparación): una base de catálogo 2 o una vacía se abre sin tocar sus datos, y un fallo a medias revierte las columnas y deja la versión 2.

| Tabla | Columna | Contenido |
|---|---|---|
| `socratica_flows` | `preparation_json` (nulo) | Agregado de preparación: etapa, secciones `intent`, `scope` y `stories`, comprobaciones con la huella de sus entradas, revisión `{revision, prompt_hash}` y resultado comunicado. `NULL` en `refine`/`scrutiny` y en las filas anteriores: nunca se infiere que estuvieran preparadas. |
| `socratica_flows` | `payload_policy` (`persistent` por defecto) | Fijada al crear el flujo y nunca reescrita por una transición. `ephemeral` solo para `specify` iniciado en modo privado. |
| `socratica_history` | `preparation_json` (nulo) | Instantánea de la preparación al aprobar y al comunicar el resultado, solo en modo no privado. |

- La `revision` de la fila es la fuente única de la revisión del contenido de `specify`; cada cambio material la incrementa. El control de concurrencia de `SaveTransition` (fase, revisión y fecha) también cubre ediciones y aprobación.
- Una `preparation_json` con `schema_version` mayor que la que entiende el binario se conserva y se rechaza al intentar mutarla, sin sobrescribirla; solo se permite cancelarla, sin tocar el agregado, para liberar el hueco.
- La aprobación, el reenvío y el resultado se guardan con la transición completa. La primera confirmación (`Ack`) sigue siendo una sentencia condicional: su `RowsAffected==1` es lo único que concede ejecución, y la concesión no se guarda ni se recupera.

**Reserva frente a payload efímero.** Para un `specify` con `payload_policy='ephemeral'`, la fila contiene solo identidad, proyecto, cliente, modo, política, fases, revisión, fechas y marcas de despacho: `original_prompt`, `proposed_prompt`, `approved_prompt`, `questions_json`, `answers_json`, `last_error` y `approved_hash` quedan vacíos y `preparation_json` es `NULL`. El contenido vive en un depósito en memoria del proceso MCP que lo creó (`EphemeralSocraticaStore`): se prepara dentro de la transacción, se publica solo después del *commit* (un *rollback* no publica nada) y no se escribe en archivos temporales, locks ni historial. Si el proceso se pierde, el contenido se pierde con él: otra instancia ve la reserva como `private_state_unavailable`, solo puede consultar o cancelar y nunca reconstruye una autorización. El índice `idx_socratica_one_pending` impide otro pendiente del mismo proyecto y cliente entre procesos.

**Saneamiento y retención.** `mem sanitize` añade las categorías `socratica_flow_preparation` y `socratica_history_preparation`. Si el saneamiento cambia la preparación de un flujo sin consumir (`review` o `approved` sin reclamo ni ack), se devuelve a `captured` con la revisión incrementada y se vacían propuesta, texto aprobado, huella y `dispatch_id`; si ya se consumió o está reclamado, solo se protegen los textos y se conservan las marcas de despacho. Una preparación de versión mayor se protege hoja a hoja sin reinterpretarla. `RedactFlow` también anula `preparation_json`. Borrar un flujo (retención, purga o `mem gc`) elimina su agregado completo y descarta su payload efímero.

## HCD (feature 045, esquemas v5 y v6)

El catálogo HCD guarda solo el control de las unidades; su contenido vive en el documento `hcd/HCD-NNN-<slug>.md`. Las decisiones se registran como memorias `decision`, sin tabla propia.

| Tabla | Contenido | Claves e índices |
|---|---|---|
| `hcd_units` | Unidad: `id`, `number`, `title`, `slug`, `kind`, `lane` (`fast`/`full`), `why`, `state` (`active`/`closed`/`cancelled`), `active` y `doc_path`. | PK `(project,id)`; única `(project,number)`; índice `(project,active)`. |
| `hcd_evidence` | Evidencia: tareas, `kind` (`command`/`manual`), `expect_fail`, comando, código de salida, extracto, revisión de trabajo y huella del código. | PK `id`; índice `(project,unit_id,created_at)`. |
| `hcd_approvals` | Aprobaciones `approve`, `close` y `cancel`: revisión, huella, origen (`conversation`/`terminal`), cliente, cita protegida y, desde la v6, `constitution_rev` y `playbook_rev` (huellas de las normas vigentes al aprobar). | PK `id`. |

El store usa consultas parametrizadas y aísla cada proyecto. `Activate` deja una sola unidad activa por proyecto en una transacción.

La v4 (catálogo HCD v1, 16 tablas) solo existió en la rama 045 y nunca se publicó. Al migrar desde v4, la v5 elimina esas 16 tablas y crea las 3 actuales en la misma transacción, sin tocar ninguna otra tabla. Es la única excepción a la regla «nunca `DROP`» (ADR 0003). Una base nueva o v3 nunca ejecuta ese `DROP`. La v6 añade las dos columnas de normas con `ALTER TABLE … ADD COLUMN`, también en las bases nuevas, para que el catálogo sea el mismo por los dos caminos.
## Expand: estadísticas, idempotencia y registro global (040)

`instinct_stats` vive en el mem.db del proyecto. Su clave primaria `(project, slug)` identifica el patrón; conserva tipo, descripción redactada (la muestra de la última observación), observaciones, correcciones, confianza, alcance, estado y fechas. `last_seen` decide la ventana de 30 días tras la que un instinct deja de inyectarse. `instinct_folded`, con clave `(project, obs_id)`, registra los UUID ya contados en la misma transacción que actualiza las estadísticas. Se purga después de siete días.

`instinct_global` vive en `DataHome()/instincts.db`, con permisos 0600 y WAL. La clave es `slug`; guarda tipo, descripción, conjunto JSON de proyectos, confianza y fecha de actualización. Dos proyectos con confianza mínima de 0,5 permiten promoción global. Las tres tablas se crean de forma aditiva.

La cola transitoria `.memory/expand-observations.ndjson` tiene límite de 256 KiB. El plegado confirma hasta 500 observaciones y luego retira sus UUID de la cola; repetir un lote no incrementa los contadores. La reconciliación global ocurre después del commit y se reintenta por estado. `mem sanitize` informa por separado `instinct_stats`, `instinct_global` e `instinct_queue`.

## Configuración y preparación de mem.db

La apertura prepara el esquema de forma aditiva en una sola transacción inmediata. La versión se guarda en `PRAGMA user_version`: si es mayor o igual que `schemaVersion`, la apertura omite la transacción y el DDL, por lo que una base preparada no espera el bloqueo de otro escritor. Una versión mayor, escrita por un binario más nuevo, nunca se reduce. Dentro de la transacción se vuelve a leer la versión antes de crear estructuras, para cubrir aperturas concurrentes.

Cada cambio del catálogo exige aumentar `schemaVersion` y regenerar `adapters/secondary/persistence/testdata/schema.golden` en el mismo cambio. El golden incluye la versión, el catálogo y las columnas; su guarda rechaza cambios del catálogo sin aumento de versión. La versión 1 introduce la preparación transaccional, la versión 2 añade el índice no único `idx_sessions_project_created ON sessions(project, created_at)`, utilizado para contar sesiones, consultar la activa y listar las recientes, y la versión 3 añade las columnas de la preparación para Spec Kit (véase la sección de Socratica). La versión 5 añade el esquema HCD y retira el catálogo HCD v1 de las bases que lo tenían, y la versión 6 añade a `hcd_approvals` las huellas de la Constitución y del Playbook (véase la sección HCD). Un error no tolerado revierte toda la preparación y conserva la versión anterior. Las ampliaciones opcionales conservan sus tolerancias históricas; si fallan, no se vuelven a intentar hasta la siguiente subida de versión.

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
