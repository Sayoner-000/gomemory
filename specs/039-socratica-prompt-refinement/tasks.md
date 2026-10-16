# Tasks: Socratica — refinamiento de prompts y escrutinio socrático

**Input**: Design documents from `specs/039-socratica-prompt-refinement/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: SÍ. El borrador (§8) y la constitución (§10) exigen pruebas. Se usa TDD: cada prueba se escribe primero y debe fallar antes de implementar.

**Organization**: Tareas agrupadas por historia de usuario de `spec.md`. Revisión 2 (2026-10-15): incorpora las correcciones de `/speckit-analyze` (C1, F1–F6, U1–U3, D1, G1–G7, A1–A4, T1–T2, P1–P2) y el cuerpo de habilidad basado en el texto aportado por la persona (`contracts/skill.md`). Revisión 6 (2026-10-16): cierra I10 e I11 del 7.º análisis (garantía de despacho: autorización consumida, entrega repetible con `ack_nonce` y ejecución nunca afirmada; `force_resend` desde dispatched; `next` por estado). Revisión 5 (2026-10-16): cierra U2, I8 e I9 del 6.º análisis (lock de archivo por flujo durante todo el despacho, `ack` como compuerta previa a ejecutar desde MCP y `resume` condicionado al reclamo). Revisión 4 (2026-10-16): cierra U2, U3, I6 e I7 del 5.º análisis (propiedad del reclamo, envío incierto con conciliación explícita, propuesta en el historial y skip de la última pregunta). Revisión 3 (2026-10-16): cierra C1–C2, I1–I5 y U1 del 4.º análisis (ajustes inválidos fallan rápido, timeout por consulta, redacción tras ack privado, approve desde failed, generación solo fuera de MCP, contrato bloqueante del lanzador, reclamo exclusivo del despacho y validación MVP separada).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizable (archivos distintos, sin dependencias pendientes)
- **[Story]**: US1–US6 según spec.md

## Path Conventions

Monolito Go hexagonal en la raíz: `domain/`, `application/`, `adapters/`, `infrastructure/`, `tests/`. Unitarias junto al código (`*_test.go`). Composition root: `infrastructure/container.go`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: guardia de fuente única y carpeta de evidencias.

- [X] T001 Crear la prueba de contrato `TestSkillSourcesInSync` en `tests/contract/skill_sources_sync_test.go`. Por cada directorio de `infrastructure/templates/` con `SKILL.md` que tenga copia en `skills/<name>/SKILL.md`, debe exigir igualdad byte a byte; además, debe exigir la existencia de `skills/socratica/SKILL.md`. Debe fallar ahora (socratica ausente) y pasar ya para `adversarial-consensus-review` y `defensive-qa-audit` (research R1).
- [X] T002 [P] Crear `specs/039-socratica-prompt-refinement/evidence/README.md` con el formato de registro por agente (orden ejecutada, versión del binario, salida literal, veredicto) y el conjunto fijo de 5 prompts de referencia con datos esenciales ausentes para SC-005 (por ejemplo «envía el informe al cliente antes del cierre»), cada uno con la lista de datos que deben quedar como `[marcador]`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: dominio puro, persistencia, ajustes, caso de uso y la herramienta MCP `socratica_flow`, de los que dependen todas las historias.

**⚠️ CRITICAL**: ninguna historia empieza antes de cerrar esta fase.

### Pruebas (escribir primero, deben fallar)

- [X] T003 [P] Escribir la tabla de pruebas `TestReduce` en `domain/socratica_test.go` con **todas** las filas de la tabla de transiciones de `data-model.md`: questions_generated con 1–3 y con 0; answer/skip/skip_rest, incluido `skip` de la **última** pregunta → rewriting (I7); rewrite_ready (`last_stable_phase=review`, `EffectRecordHistory`); edit (revision+1); approve (asigna `dispatch_id`); approve sobre approved (`Duplicate=true`, `Acked=false`, mismo id) y sobre dispatched (`AlreadyDispatched=true`); `force_resend` desde dispatched → approved con `dispatch_id` nuevo y ack vacío, y error si el flujo está redactado (I10); ack coincidente y no coincidente; model_error desde captured y desde rewriting (conserva original y `last_stable_phase`); retry desde failed hacia captured si `last_stable_phase=captured` y hacia rewriting si es questions, y error en cualquier otro caso; send_original (con `EffectRecordHistory`); cancel desde cada fase no terminal (`EffectRestoreOriginal`); resume de approved sin ack ni reclamo (→ review, `recovered=true`, sin reenviar); resume de approved sin ack **con** reclamo → approved con `in_flight=true` si `LockHeld` y con `uncertain=true` si no; con `LockHeld`, `mark_dispatched`/`force_resend`/`cancel` dan «envío en curso» sin cambios; desde `uncertain`, `mark_dispatched` → dispatched, `force_resend` → approved con `dispatch_id` nuevo y reclamo vacío, `cancel` → cancelled, y approve/edit dan `ErrInvalidTransition` (U2, U3); resume de captured (`EffectGenerateQuestions`) y de rewriting (`EffectGenerateRewrite`); resume de questions/review sin efecto; ack con `private` (`EffectRedactFlow`, conserva `dispatch_id`, `approved_hash` y `dispatch_consumed`) y cancel con `private` (`EffectDeleteFlow`); transiciones no listadas (`ErrInvalidTransition`). Reloj y `newID` se inyectan.
- [X] T004 [P] Escribir `TestParseGeneration` en `domain/socratica_generation_test.go`: 0 preguntas; 1–3 válidas; más de 3 (recorta a 3 en orden); opción única (descarta la pregunta); más de 4 opciones (recorta a 4); JSON envuelto en texto (extrae el primer objeto balanceado); JSON irreparable (error); bloque `scrutiny` con `assumptions[].kind` ∈ {fact, hypothesis} y más de 3 supuestos recortados a 3. Añadir también `TestWordCount`.
- [X] T005 [P] Escribir `TestSocraticaRepository` en `adapters/secondary/persistence/socratica_test.go` contra SQLite real: crear y leer un flujo con `scrutiny_json`; `ActiveFlow` por (project, client); el índice único parcial impide un segundo flujo no terminal del mismo (project, client) pero permite otro cliente; `Approve` concurrente desde 2 goroutines (una sola asigna `dispatch_id`, la otra recibe `duplicate=true`); `Approve` desde `failed` (recorrido fallo → `send_original`) asigna `dispatch_id` y fija `approved_prompt` = original, y desde cualquier otra fase no afecta filas (I2); `ClaimDispatch` concurrente desde 2 goroutines sobre el mismo flujo approved sin ack: exactamente una obtiene `claimed=true` (U1); **propiedad (U2)**: `ReleaseDispatch` con un `owner` ajeno no afecta filas; un reclamo sin ack nunca lo toma otro `owner`, por antiguo que sea; `Ack` con `owner` NULL (MCP) se rechaza si hay reclamo local, y dos `Ack` concurrentes con `owner` NULL y nonces distintos sobre el mismo `dispatch_id` dejan exactamente uno aceptado (I8); un `Ack` repetido con el **mismo** nonce devuelve `acked=true` sin cambiar nada, y con otro nonce `alreadyDispatched=true` (I10); `force_resend` desde dispatched falla si ya existe otro pendiente del mismo `(project, client)` por el índice único parcial; `SocraticaDispatchLock`: un segundo `TryAcquire` del mismo flujo con otro descriptor devuelve `acquired=false`, se puede volver a tomar tras `release` o tras cerrar el descriptor, y `PurgeTerminal` borra los archivos de lock de los flujos purgados; `Ack` con `owner` o `dispatch_id` ajenos devuelve `ErrAckRejected`; tras `ForceResend`, el `Ack` tardío del propietario anterior (dispatch_id viejo) se rechaza y el nuevo reclamo funciona; `MarkDispatched` marca el ack; `RedactFlow` vacía `original_prompt`, `proposed_prompt`, `approved_prompt`, `questions_json`, `answers_json` y `scrutiny_json` pero conserva `dispatch_id`, `approved_hash` y `dispatch_consumed`, y `ConsumeDispatch` sigue reconociendo el texto aprobado después de redactar (I1); `Ack` con id ajeno da error; `FindByDispatchID`; `DeleteFlow`; historial podado a `socratica_history_limit` (20) por (project, client) sin tocar otro cliente; el historial guarda `proposed_prompt` al llegar a review y lo actualiza en `edit`; tras cancelar el flujo y `PurgeTerminal`, `ListHistory` sigue devolviendo original y propuesta (I6); `<private>` ausente tanto al escribir como al leer una fila insertada en crudo (lección H-01, memoria 510); `PurgeStale` borra solo flujos en captured/questions/rewriting/review/failed con más de `PendingMaxAge` y **conserva** `approved` sin ack; `ConsumeDispatch` devuelve `matched=true` una sola vez y solo si el texto coincide con `approved_prompt`; `SocraticaUnitOfWork.WithTransaction` hace rollback de la transición si falla `UpsertHistory`; `BenchmarkSocraticaApprove` registra el tiempo de la operación de estado (objetivo < 50 ms, registrado en evidencia); una base anterior abre sin error (migración aditiva). Timeout por consulta (C2, constitución §5): con `socratica_query_timeout_ms` = 100 y una transacción de escritura retenida desde otra conexión, `Approve` y `SaveTransition` devuelven `context.DeadlineExceeded` en menos de 1 s y no dejan cambios parciales. Además (G2): al entrar un flujo en dispatched o cancelled se podan los terminales por encima de `socratica_history_limit` para ese (project, client), y `PurgeTerminal(project, TerminalRetention)` borra los terminales con más de 24 h.
- [X] T006 [P] Escribir `TestSettings_SocraticaRoundTrip` en `adapters/secondary/persistence/settings_test.go`: escribir `socratica_interception`, `socratica_interception_unverified`, `socratica_history_limit`, `socratica_private`, `socratica_generators`, `socratica_generator_timeout_seconds` y `socratica_query_timeout_ms`; guardar otra preferencia ajena; releer y comprobar que no se perdió ningún campo (research R8). Añadir `TestValidateSocraticaSettings` (tabla, constitución §9, falla rápido): **claves ausentes** → defaults documentados (20, 120 s, 5000 ms, `["claude","codex"]`, mapas vacíos) sin error; **valores inválidos** → `ErrInvalidSocraticaSettings` con la clave y el valor, nunca sustitución silenciosa: `history_limit` < 1 o > 1000, `generator_timeout_seconds` < 1, `query_timeout_ms` < 100, generador desconocido en la lista, agente desconocido en `interception`/`interception_unverified`.
- [X] T007 [P] Escribir `TestSanitize_CubreSocratica` en `adapters/secondary/persistence/socratica_sanitize_test.go`: secretos en `original_prompt`, `questions_json` (string anidado en un objeto), `answers_json`, `scrutiny_json` y `last_error` de `socratica_flows`, y en `original_prompt`/`proposed_prompt`/`additions_json` de `socratica_history`, deben aparecer como candidatos con `RecordID` TEXT y su categoría por columna (research R9); con `--apply` quedan depurados respetando `project`; otro proyecto no se toca.
- [X] T008 [P] Escribir `TestSocraticaFlowUseCase` en `application/usecases/socratica_flow_test.go` con repositorio, unidad de trabajo, generador y lanzador falsos definidos en `application/usecases/socratica_fakes_test.go` (convención del repositorio, constitución §11). Casos: un fallo simulado de `UpsertHistory` dentro de la transacción deja el flujo en su fase anterior (C1); `Execute` hace approve (tx) → `ClaimDispatch` → `Launch(ctx, req, onStarted)` y escribe el ack **dentro de `onStarted`**: el lanzador falso registra que el ack ya existe al volver de `onStarted` y antes de que `Launch` retorne (I4); si `Launch` falla sin llamar a `onStarted` (`ErrLaunchFailed`), no hay ack, se llama a `ReleaseDispatch` con el propio `owner` y el flujo queda approved y recuperable; **confirmación fallida (U3)**: si el agente arranca pero el ack falla en `onStarted` y en el reintento posterior a `Wait`, `Execute` devuelve `ErrDispatchUnconfirmed`, no libera el reclamo, un nuevo `Execute` no llama al lanzador y `resume` deja el flujo `uncertain`; si el reintento posterior a `Wait` tiene éxito, el flujo queda dispatched; **propietario pausado (U2)**: el `Execute` A toma el lock, reclama y se bloquea después de comprobar la propiedad y antes de `Start` (el lanzador falso espera una señal antes de arrancar); mientras tanto `force_resend`, `mark_dispatched` y `cancel` responden «envío en curso» sin cambios y un `Execute` B tampoco lanza; A continúa, arranca y confirma; el lanzador registra **una** sola llamada. Con A muerto (la prueba cierra su lock sin ack), `resume` da `uncertain`; tras `force_resend`, un `Execute` nuevo lanza una vez y el ack tardío con el `owner` y el `dispatch_id` de A se rechaza; ante un duplicado ya confirmado devuelve «ya enviado» sin llamar al lanzador (C2); **dos `Execute` concurrentes** sobre el mismo flujo (lanzador falso que bloquea en `Launch` hasta una señal) producen exactamente **una** llamada al lanzador y la otra devuelve «envío en curso» sin lanzar (U1); `Execute` sobre un flujo en `failed` con `send_original` aprueba el original y lanza (I2); `Apply` con origen `OriginMCP` ante `EffectGenerateQuestions`/`EffectGenerateRewrite` devuelve `next` para el agente activo y el generador falso registra **cero** llamadas, mientras que con `OriginLocal` (CLI/TUI) lo llama una vez (I3); un ajuste inválido hace que el caso de uso devuelva `ErrInvalidSocraticaSettings` sin tocar el repositorio (C1); `Execute` pasa `GOMEMORY_SOCRATICA_DISPATCH` en el entorno; start con un pendiente del mismo cliente lo retoma (`recovered=true`) sin crear otro; review registra historial; edit lo actualiza; approved lo actualiza; modo privado no escribe historial, **redacta** la fila en ack (`RedactFlow`: sin textos, conserva `dispatch_id`, `approved_hash` y `dispatch_consumed`) y la borra en cancel (I1); resume desde captured y rewriting relanza la generación una sola vez (idempotente); resume de approved sin ack **ni reclamo** devuelve review recuperable; con reclamo y lock libre devuelve approved `uncertain`, y con lock retenido approved `in_flight` (I9); `PurgeStale` delega con `PendingMaxAge`.
- [X] T009 [P] Escribir `TestMCPSocraticaFlow` en `adapters/primary/cli/cmd_mcp_socratica_tool_test.go`. Casos: esquema registrado; start → questions → rewrite → approve → ack; un segundo approve **antes** del ack devuelve `duplicate=true`, `acked=false` y un `next` que dirige al ack; **después** del ack devuelve `already_dispatched=true` y un `next` que prohíbe ejecutar; recorrido de despacho (I8): approve → ack(`dispatch_id`) responde `acked=true` antes de ejecutar, un segundo ack responde `already_dispatched=true` y dos ack concurrentes dejan un solo `acked=true`; recorrido de reenvío (I8): flujo `uncertain` → `force_resend(dispatch_id)` → approve (`duplicate`, `acked=false`) → ack con el `dispatch_id` nuevo da `acked=true`, mientras que el ack con el viejo se rechaza; `force_resend` con el lock retenido responde `in_flight=true` sin cambios; **pérdida de respuesta (I10)**: el ack se aplica pero el cliente reintenta con el mismo `ack_nonce` y recibe `acked=true` sin consumir otra autorización; **caída tras el ack (I10)**: una sesión nueva recibe `already_dispatched=true` y un `next` que prohíbe ejecutar y ofrece reenviar; `force_resend` desde dispatched → approve → ack con el `dispatch_id` nuevo da `acked=true` y el antiguo se rechaza; **`next` por estado (I11)**: tabla que comprueba el `next` exacto de revisión recuperable, listo para confirmar, envío en curso, envío incierto y autorización consumida, y que ni `in_flight` ni `uncertain` ofrecen Ejecutar; ack con id ajeno da error; `settings` es de solo lectura (no existe acción que cambie la intercepción); start(scrutiny) persiste y devuelve `scrutiny`; sin memoria inicializada responde `{"phase":"unavailable"}`; respuestas sin `<private>`; `get` sobre approved sin ack **ni reclamo** no cambia el estado y devuelve `recoverable=true`; `resume` lo lleva a review con `recovered=true` (M3); con reclamo, `get`/`resume` devuelven `uncertain` o `in_flight` y nunca review (I9); `resume` de un flujo en captured o rewriting devuelve un `next` que pide al agente activo regenerar y el generador falso registra cero llamadas (I3); ack con `socratica_private=true` deja la fila redactada (no borrada) y un `get` posterior no devuelve textos (I1); un ajuste inválido devuelve error MCP con la clave (C1); un flujo approved con reclamo sin ack devuelve `uncertain=true`, un `next` que prohíbe ejecutar, y acepta solo `mark_dispatched`, `force_resend` o `cancel` (U3). Validación del payload del modelo (G4, SC-004): 4 preguntas → se recortan a 3; una pregunta con 1 opción se descarta; con 5 opciones se recortan a 4; JSON de preguntas irreparable → error y flujo en `failed` con el original intacto.

### Implementación

- [X] T010 [P] Implementar tipos, `SocraticaPhase` (con `IsStable`), `SocraticaMode` (`refine`/`scrutiny`), `RefinementFlow`, `Question`, `Answer`, `ScrutinySynthesis`, `RefinementHistory`, `Event`, `Effect`, `ErrInvalidTransition` y `Reduce` en `domain/socratica.go`, hasta que T003 pase.
- [X] T011 [P] Implementar `ParseGeneration`, la reparación y `WordCount` en `domain/socratica_generation.go`, y copiar el esquema a `domain/socratica_generation.schema.json` (embebido) desde `contracts/generation.schema.json`, hasta que T004 pase.
- [X] T012 [P] Declarar en `domain/socratica_capabilities.go` `SocraticaCapabilities` con todas las capacidades a `false` (incluido `Pane`), y las constantes `DefaultHistoryLimit=20`, `PendingMaxAge=7*24h`, `TerminalRetention=24h`, `ContinuationWindow=30m`, `LowInfoMaxWords=3`, `DefaultGeneratorTimeout=120s`, y `DefaultQueryTimeout=5s`, con una prueba que las fije en `domain/socratica_capabilities_test.go`.
- [X] T013 Definir los puertos de `data-model.md`: `SocraticaRepository` en `application/ports/socratica_repository.go`, `SocraticaUnitOfWork` en `application/ports/socratica_unit_of_work.go`, `SocraticGenerator` (`Name`, `Available`, `GenerateQuestions`, `Rewrite`) en `application/ports/socratic_generator.go` y `AgentLauncher` (`Launch(ctx, req LaunchRequest, onStarted func() error) error`, bloqueante: arranca, llama a `onStarted` y espera a que el proceso termine; si el arranque falla devuelve error sin llamar a `onStarted`) en `application/ports/agent_launcher.go`; `SocraticaDispatchLock` (`TryAcquire(flowID) (release, acquired, err)`) en `application/ports/socratica_dispatch_lock.go`; `SocraticaRepository` incluye `ClaimDispatch` y `ReleaseDispatch` con `owner`, `Ack` con `owner`, `ForceResend`, `MarkDispatched` y `RedactFlow`; errores `ErrLaunchFailed`, `ErrConfirmFailed`, `ErrAckRejected`, `ErrDispatchUncertain` y `ErrDispatchUnconfirmed` (dep: T010).
- [X] T014 Añadir las tablas `socratica_flows` y `socratica_history`, sus índices el índice único parcial `idx_socratica_one_pending` las columnas `dispatch_claimed_at`, `dispatch_claim_owner`, `ack_nonce` y `socratica_history.proposed_prompt` (DDL de `data-model.md`) a `migrate()` en `adapters/secondary/persistence/db.go`, solo con `CREATE … IF NOT EXISTS` (dep: T010).
- [X] T015 Implementar el repositorio SQLite y `SocraticaUnitOfWork` (patrón de `PrivacyMaintenanceUnitOfWork`: confirma solo sin error, rollback completo en otro caso) en `adapters/secondary/persistence/socratica.go`: SQL parametrizado, sin `SELECT *`, `ProtectText` al escribir y al leer (incluidos los strings dentro de los JSON), `Approve` con `UPDATE … WHERE id=? AND phase IN ('review','failed')` (`failed` solo por `send_original`, ya validado por `Reduce`), `ClaimDispatch` con `UPDATE … SET dispatch_claimed_at=?, dispatch_claim_owner=? WHERE id=? AND dispatch_id=? AND phase='approved' AND dispatch_ack=0 AND dispatch_claim_owner IS NULL` (gana quien afecta 1 fila; nunca se toma un reclamo existente, por antiguo que sea), `ReleaseDispatch` y `Ack` condicionados al `owner`, `ForceResend`, `MarkDispatched`, el adaptador `SocraticaDispatchLock` en `adapters/secondary/persistence/socratica_dispatch_lock{,_unix,_windows}.go` (mismo patrón que `adapters/secondary/codegraph/refresh_lock_*`, sin dependencias nuevas), `RedactFlow`, toda consulta envuelta en `context.WithTimeout` con `socratica_query_timeout_ms`, la poda del historial en la misma transacción, `Ack`, `DeleteFlow`, `FindByDispatchID`, `ConsumeDispatch` (comparación y marca en la misma sentencia) `PurgeStale` (excluye approved), `PurgeTerminal` y la poda de terminales por encima del límite. Debe hacer pasar T005 (dep: T013, T014).
- [X] T016 [P] Añadir las claves `socratica_interception`, `socratica_interception_unverified`, `socratica_history_limit`, `socratica_private`, `socratica_generators`, `socratica_generator_timeout_seconds` y `socratica_query_timeout_ms` a `ports.SettingsData` en `application/ports/settings_repository.go` y al `Settings` de `adapters/secondary/persistence/settings.go`. Implementar `ValidateSocraticaSettings` en `domain/socratica_settings.go` con `ErrInvalidSocraticaSettings` (clave y valor) y defaults solo para claves ausentes, invocada al cargar los ajustes. CLI y TUI salen con código 1 y el mensaje, MCP devuelve error, y los hooks tratan la intercepción como apagada y escriben el error por stderr sin bloquear el turno. Debe hacer pasar T006.
- [X] T017 Añadir en `domain/privacy_maintenance.go` las categorías por columna `socratica_flow_{original,proposed,approved,questions,answers,error,scrutiny}` y `socratica_history_{original,proposed,approved,additions,scrutiny}` con su campo en `sanitizeFields`. Implementar `readSocraticaCandidates` y su rama de `Apply` (`UPDATE … WHERE id=? AND <col>=? AND project=?`) en `adapters/secondary/persistence/socratica_sanitize.go`, e invocarla desde `Candidates` y `Apply` de `adapters/secondary/persistence/privacy_maintenance.go` sin tocar `ledgerColumns`, hasta que T007 pase (dep: T014).
- [X] T018 Implementar `usecases.SocraticaFlow` en `application/usecases/socratica_flow.go`: `Apply(ctx, flowID, Event, origin)` aplica `Reduce` y persiste dentro de **una** `WithTransaction` por evento junto con los efectos de persistencia (`EffectRecordHistory`, `EffectDeleteFlow`, `EffectRedactFlow`, `EffectRestoreOriginal`); los efectos `EffectGenerateQuestions`/`EffectGenerateRewrite` dependen del origen: con `OriginMCP` se traducen en `next` para el agente activo y **nunca** llaman a `SocraticGenerator`; con `OriginLocal` (CLI/TUI) llaman al generador inyectado fuera de cualquier transacción; `Execute` orquesta approve (tx) → `ClaimDispatch` (si no se obtiene, devuelve «envío en curso» sin lanzar) → `AgentLauncher.Launch(ctx, req, onStarted)` fuera de tx, con el ack (tx, condicionado al `owner`) **dentro de `onStarted`**; retiene el `SocraticaDispatchLock` del flujo durante todo `Execute` (si no lo obtiene, «envío en curso» sin lanzar) y exige el mismo lock para `mark_dispatched`, `force_resend` y `cancel` sobre un flujo reclamado, pasando `LockHeld` al evento; ante `ErrLaunchFailed` llama a `ReleaseDispatch(owner)`; ante `ErrConfirmFailed` reintenta el ack una vez tras `Wait` y, si falla, devuelve `ErrDispatchUnconfirmed` sin liberar el reclamo; un reclamo existente con el lock libre da `ErrDispatchUncertain`; los eventos `mark_dispatched` y `force_resend` pasan por `Apply`; `Resume`; retoma el pendiente en start; respeta el modo privado; expone `PurgeStale`. Debe hacer pasar T008 (dep: T013, T016).
- [X] T019 Hacer el wiring de `SocraticaFlow`, del repositorio, de la unidad de trabajo y de `SocraticaDispatchLock` en el composition root `infrastructure/container.go` (dep: T015, T018).
- [X] T020 Añadir el esquema de `socratica_flow` en `adapters/primary/cli/cmd_mcp_schemas.go` y su handler en `adapters/primary/cli/cmd_mcp_socratica_tool.go`, según `contracts/mcp-tool.md`, hasta que T009 pase (dep: T019).
- [X] T021 Conectar `SocraticaFlow.PurgeStale` a `mem gc` en `adapters/primary/cli/` (handler del `case "gc"` de `dispatcher.go`) y añadir `TestGC_PurgaSocraticaPendientes` en `adapters/primary/cli/cmd_gc_socratica_test.go`: un flujo en questions con más de 7 días se purga; uno reciente y uno approved sin ack de más de 7 días se conservan; uno dispatched de más de 24 h se purga (G2) (dep: T019).

**Checkpoint**: `go test ./domain/ ./application/... ./adapters/secondary/persistence/ ./adapters/primary/cli/ -run 'Socratica|Reduce|ParseGeneration|Sanitize|GC' -count=1` en verde.

---

## Phase 3: User Story 1 - Refinar un prompt de forma explícita desde el agente (Priority: P1) 🎯 MVP

**Goal**: habilidad `socratica` con las secciones Contrato, Seguridad y precedencia, Flujo /socratica, Eficiencia y Portabilidad (`contracts/skill.md`), compuerta de aprobación y alternativa en texto.

**Independent Test**: prueba de contenido de la habilidad, más el recorrido MCP start→approve→ack con generador simulado. La aceptación en agentes reales se hace en T061–T063.

### Tests for User Story 1

- [X] T022 [P] [US1] Escribir `TestSocraticaSkill_Contenido` en `adapters/primary/setup/socratica_skill_test.go`, leyendo el `SKILL.md` embebido. Comprueba:
  - el frontmatter `name: socratica`, igual al directorio, y que `description` exista;
  - los encabezados `## Contrato`, `## Seguridad y precedencia`, `## Flujo /socratica`, `## Eficiencia` y `## Portabilidad y alternativa`;
  - las frases clave de `contracts/skill.md` §1–3, §5 y §6: precedencia del sistema, pregunta sin responder = omitida, lectura de solo lectura permitida, sin ampliar alcance, sin convertir opcionales en obligatorios, «no afirmar que existen chips nativos», «nunca afirmar que la tarea se ejecutó», `socratica_flow(action=approve)` seguido de `socratica_flow(action=ack, dispatch_id=…, ack_nonce=…)` **antes** de ejecutar, repetir el ack con el mismo nonce si se pierde la respuesta, ejecutar solo con `acked=true`, no ejecutar con `already_dispatched=true`, y ante `uncertain=true` preguntar a la persona sin elegir por ella, `mem socratica on|off --client`, prohibición de `save_memory` de prompts, `octopus_route_task` como condicional (G7), «no usar subagentes solo para mejorar el estilo» y el caso «`/socratica` sin argumento» (M6);
  - que no se mencionen herramientas MCP inexistentes, contrastando con la lista de herramientas registradas.
- [X] T023 [P] [US1] Escribir `TestSocraticaFlow_RecorridoRefine` en `tests/integration/socratica_refine_test.go`, contra el servidor MCP en proceso: start(refine) con preguntas; answer + skip_rest; rewrite con `[marcador]`; edit; approve; ack. Después, un segundo approve devuelve duplicate; cancel en otro flujo devuelve el original literal; un fallo (`fail`) seguido de `send_original` exige una acción explícita; start con un pendiente lo retoma; con la intercepción desactivada para ese cliente, la invocación explícita funciona igual (FR-022); tras el recorrido, los directorios de habilidades instaladas no cambian (FR-025). Un `start` con 5 preguntas en el payload del modelo muestra como máximo 3 (G4).

- [X] T024 [P] [US1] Escribir `TestUserPromptSubmit_ProcedenciaSocratica` en `adapters/primary/cli/hook_provenance_socratica_test.go` y el caso equivalente de `mem hook prompt` (OpenCode). Con la intercepción apagada, `SetLastPrompt` **no** se llama para `/socratica …`, `/socratic-q …`, `$socratica …` ni para respuestas mientras hay un flujo pendiente vigente (`ContinuationWindow`) del mismo agente; un prompt normal sí se guarda. Una memoria guardada en ese turno no lleva el prompt de Socratica como origen (G1, FR-026).

### Implementation for User Story 1

- [X] T025 [US1] Redactar la fuente canónica `infrastructure/templates/socratica/SKILL.md` traduciendo al español el texto base de la persona con las secciones §1–3, §5 y §6 de `contracts/skill.md` y sus añadidos ⊕, hasta que T022 pase.
- [X] T026 [US1] Copiar el contenido exacto a `skills/socratica/SKILL.md` y crear `skills/socratica/README.md` (propósito, modos e instalación, al estilo de `skills/defensive-qa-audit/README.md`), hasta que T001 pase (dep: T025).
- [X] T027 [US1] Ajustar las respuestas `next` del handler en `adapters/primary/cli/cmd_mcp_socratica_tool.go` para que guíen cada paso permitido del modo refine (preguntar, reescribir, revisar, esperar aprobación), hasta que T023 pase (dep: T020, T025).

- [X] T028 [US1] Añadir `domain.IsSocraticaInvocation(text)` en `domain/socratica_interception.go` (con prueba de tabla en `domain/socratica_interception_test.go`) y `SocraticaFlow.ClassifyPrompt(ctx, project, client, text, env) PromptClassification{SkipProvenance, Capture, Forward, Directive}` en `application/usecases/socratica_flow.go`, que por ahora solo resuelve `SkipProvenance`. Llamarlo en `hookUserPromptSubmit` (`adapters/primary/cli/cmd_hook.go`) **antes** de `SetLastPrompt` y en `hookPrompt`, hasta que T024 pase (dep: T018).

**Checkpoint**: US1 funcional por MCP y con la habilidad lista para distribuir.

---

## Phase 4: User Story 2 - Escrutar una propuesta con preguntas socráticas (Priority: P1)

**Goal**: modo `scrutiny` (`/socratic-q`) con síntesis equilibrada, hipótesis rotuladas, regla de parada, compuerta y conversión opt-in a entrada de especificación.

**Independent Test**: prueba de contenido de `## Flujo /socratic-q`, más el recorrido MCP en modo scrutiny que conserva el bloque `scrutiny` y exige approve.

### Tests for User Story 2

- [X] T029 [P] [US2] Ampliar `adapters/primary/setup/socratica_skill_test.go` con `TestSocraticaSkill_Escrutinio`. Comprueba:
  - el encabezado `## Flujo /socratic-q` con los pasos 1–5 de `contracts/skill.md` §4: reformulación neutral, hasta tres supuestos separando hechos de hipótesis, mejor argumento a favor y objeción más fuerte, «Hipótesis:», «Examinar no autoriza a ejecutar», no forzar postura adversarial y parar cuando las preguntas aporten poco;
  - que la acción «Convertir en entrada de especificación» es opcional y no invoca `/speckit-specify`.
- [X] T030 [P] [US2] Escribir `TestSocraticaFlow_RecorridoScrutiny` en `tests/integration/socratica_scrutiny_test.go`: start(scrutiny) con a lo sumo 3 preguntas; rewrite con bloque `scrutiny` persistido en `scrutiny_json`; `get` devuelve la síntesis; sin approve no hay `dispatch_id`; el historial guarda la síntesis depurada.

### Implementation for User Story 2

- [X] T031 [US2] Añadir la sección `## Flujo /socratic-q` (§4) y la acción de conversión a especificación en `infrastructure/templates/socratica/SKILL.md`, y sincronizar `skills/socratica/SKILL.md`, hasta que T029 y T001 pasen.
- [X] T032 [US2] Persistir y devolver `scrutiny` en `flow.Scrutiny`/`scrutiny_json` (flujo e historial) en `application/usecases/socratica_flow.go` y `adapters/primary/cli/cmd_mcp_socratica_tool.go`, hasta que T030 pase.

**Checkpoint**: ambos modos operativos por MCP.

---

## Phase 5: User Story 3 - Instalar y actualizar la habilidad en todos los agentes (Priority: P1)

**Goal**: distribución idempotente a Claude Code, Codex y OpenCode con envoltorios finos y desinstalación sin rastros.

**Independent Test**: dos instalaciones seguidas en un HOME **y PATH** aislados; la segunda escribe 0 archivos; la desinstalación no deja rastros.

### Tests for User Story 3

- [X] T033 [P] [US3] Escribir `TestInstallSocraticaSkill` en `adapters/primary/setup/agent_skills_setup_test.go`. Casos: escribe `SKILL.md` en `~/.claude/skills/socratica`, `~/.codex/skills/socratica` y `~/.opencode/skills/socratica` solo si existe el directorio del agente; escribe los envoltorios `~/.claude/commands/socratic-q.md`, `~/.config/opencode/commands/socratica.md` y `~/.config/opencode/commands/socratic-q.md`; la segunda ejecución da 0 escrituras; los envoltorios no contienen `## Seguridad y precedencia`; el instalador no escribe en esos directorios nada más que `SKILL.md` y los envoltorios (FR-025).
- [X] T034 [P] [US3] Ampliar `tests/contract/lifecycle_uninstall_test.go` (`TestUninstall_SistemaSinRastros`) con la expectativa de que, tras la desinstalación, no quedan `socratica` ni los tres envoltorios (memoria 501).

### Implementation for User Story 3

- [X] T035 [P] [US3] Crear los envoltorios `infrastructure/templates/socratica/commands/socratica.md` y `infrastructure/templates/socratica/commands/socratic-q.md` (frontmatter `description` y una línea «Aplica la habilidad `socratica` en modo <modo> a: $ARGUMENTS»).
- [X] T036 [US3] Implementar `InstallSocraticaSkill(body string, wrappers map[string]string)` en `adapters/primary/setup/agent_skills_setup.go`. Usa `InstallAgentSkill("socratica", body)` y una tabla de destinos de envoltorio por agente que solo escribe si existe el directorio base, de forma idempotente. Debe hacer pasar T033 (dep: T035).
- [X] T037 [US3] Añadir `socratica` a `globalSkillNames` y las rutas de los envoltorios a `GlobalGeneratedArtifacts` en `adapters/primary/setup/global_artifacts.go`, hasta que T034 pase (dep: T036).
- [X] T038 [US3] Llamar a `setup.InstallSocraticaSkill(embeddedTemplate("socratica/SKILL.md"), …)` en `runGlobalScopeSetup` de `adapters/primary/cli/cmd_mcp_setup.go`, con un mensaje «✅ socratica: <ruta>» por escritura y un aviso no fatal ante errores, igual que `InstallDefensiveQASkill` (dep: T036).
- [X] T039 [US3] Añadir `TestUpdate_ReinstalaSocratica` en `adapters/primary/cli/cmd_update_test.go`, con PATH aislado (lección memoria 438), comprobando que `mem update` reinstala por la misma vía (dep: T038). **Implementada** en `tests/contract/lifecycle_update_test.go` (banco con servidor de releases y PATH aislado; publica el binario real como release).

**Checkpoint**: MVP completo (US1 + US2 + US3), listo para la aceptación por agente.

---

## Phase 6: User Story 4 - Gestionar Socratica desde la línea de comandos (Priority: P2)

**Goal**: `mem socratica refine|history|on|off|status|pending|private` con generador delegado, Ejecutar en tres pasos y códigos de salida de `contracts/cli.md`.

**Independent Test**: e2e con ejecutables falsos de agente en un PATH aislado; sin agentes, código 2 y el original intacto.

### Tests for User Story 4

- [X] T040 [P] [US4] Escribir `TestGenerators` en `adapters/secondary/generator/generator_test.go`, con scripts falsos `claude` y `codex` en un PATH temporal. Comprueba:
  - los argumentos exactos de Claude: `-p --output-format json --json-schema <schema> --tools ""`;
  - los de Codex: `exec --ephemeral --sandbox read-only --skip-git-repo-check --output-schema <file> -`;
  - el parseo de la salida, el timeout tomado de `socratica_generator_timeout_seconds` y que `Available()` es falso si falta el binario;
  - que `FirstAvailable` recorre la lista recibida en orden y devuelve `ErrNoGenerator` cuando ninguno está disponible.
- [X] T041 [P] [US4] Escribir `TestAgentLauncher` en `adapters/secondary/launcher/launcher_test.go`: `ExecLauncher` arranca un ejecutable falso en un PATH temporal con el prompt como argumento y la variable de entorno recibida; `onStarted` se llama exactamente una vez, después de `Start` y **antes** de que el proceso termine (el ejecutable falso espera un archivo-señal que la prueba crea dentro de `onStarted`), y `Launch` no retorna hasta que el proceso sale (`Wait`); si el binario no existe o no es ejecutable devuelve error **sin** llamar a `onStarted`; si `onStarted` devuelve error, `Launch` **no** mata el proceso, espera a que termine y devuelve un error que envuelve `ErrConfirmFailed`, distinto de `ErrLaunchFailed` (U3). `PrintLauncher` escribe el prompt exacto en el `io.Writer` y después llama a `onStarted` (I4).
- [X] T042 [P] [US4] Escribir `TestCmdSocratica` en `adapters/primary/cli/cmd_socratica_test.go`. Casos:
  - **refine:** imprime «Generando preguntas con <agente>…» antes de llamar al generador falso (FR-017); `--print` emite el prompt una vez y una segunda ejecución informa «ya enviado»; Editar→Ejecutar funciona; Cancelar repite el original literal; sin generador sale con código 2 y el original; sin TTY nunca ejecuta.
  - **Ejecutar con agente falso:** el agente recibe el prompt y `GOMEMORY_SOCRATICA_DISPATCH=<id>`, y el `ack` ya está escrito mientras el agente falso sigue en ejecución (lo comprueba leyendo la base antes de salir); la CLI espera a que el agente termine. Si el arranque falla (binario no ejecutable), sale con código 1, no hay `ack` y `pending --resume` muestra la revisión recuperable. Si el agente falso arranca pero el ack falla (base bloqueada durante `onStarted` y el reintento), sale con código 1, imprime «el agente arrancó pero no se pudo confirmar el envío; no reintentes» y `pending --resume` ofrece `[m]arcar enviado [r]eenviar [x]cancelar`, sin relanzar nada (U3). `history --show` sobre un flujo dispatched muestra «ya enviado» y `[r]eenviar` solo con TTY y confirmación; sin TTY no ofrece reenviar (I10). Mientras un primer `mem socratica` sigue con el agente falso en ejecución, un segundo proceso sobre el mismo flujo (Ejecutar o `pending --resume` → reenviar) informa «envío en curso» y no cambia nada (U2).
  - **fallo del generador (G3, FR-016):** un timeout o un JSON irreparable conserva el original byte a byte y ofrece «Reintentar | Enviar mi original | Cancelar». «Enviar mi original» exige elección explícita y pasa por `Execute` con `send_original`.
  - **`--print` (G5):** emite `>` seguido del prompt aprobado, para que al pegarlo en un agente con intercepción activa no se recapture; `--print --raw` emite el texto sin prefijo.
  - **cliente del flujo:** el flujo se guarda con `client` igual al agente destino.
  - **history, on/off y pending:** `history` lista como máximo 20 y admite `--show`, que muestra original y propuesta aunque el flujo se haya cancelado y purgado (I6); `on --client codex` con capacidad false sale con código 1 sin persistir; `on --client codex --unverified` persiste, avisa y `status` muestra «sin verificar»; `on`/`off` de un agente no altera a otro; `pending --cancel` imprime el original.
  - **privado y errores:** `private on` desactiva el historial; un proyecto no inicializado sale con código 3.

### Implementation for User Story 4

- [X] T043 [P] [US4] Implementar `adapters/secondary/generator/claude.go`, `adapters/secondary/generator/codex.go` y el compuesto `adapters/secondary/generator/first_available.go` (recibe `[]ports.SocraticGenerator` ya ordenado; no importa los adaptadores concretos), con el esquema temporal escrito desde el embebido, hasta que T040 pase.
- [X] T044 [P] [US4] Implementar `ExecLauncher` (argumentos por agente de research R4: `claude "<p>"`, `codex "<p>"`, `opencode --prompt "<p>"`, con `cmd.Start()` → `onStarted` → `cmd.Wait()` y stdio heredado) y `PrintLauncher` (escribe → `onStarted`) en `adapters/secondary/launcher/exec.go` y `adapters/secondary/launcher/print.go`, hasta que T041 pase.
- [X] T045 [US4] Construir en `infrastructure/container.go` la lista ordenada de generadores concretos según `socratica_generators` (ignorando `opencode`, research R3) y el timeout configurado, e inyectarla en `FirstAvailable`, y registrar `ExecLauncher`/`PrintLauncher` (constitución §4) (dep: T043, T044).
- [X] T046 [US4] Implementar `CmdSocratica` en `adapters/primary/cli/cmd_socratica.go` según `contracts/cli.md`. Incluye:
  - acuse inmediato antes de generar;
  - preguntas interactivas y revisión en texto;
  - Ejecutar **delegado en `SocraticaFlow.Execute`** con el lanzador inyectado (la CLI no orquesta approve/launch/ack);
  - `--print`, y Copiar por OSC 52 con alternativa en bloque delimitado;
  - `pending --resume/--cancel`;
  - `on --unverified` con aviso;
  - `client` = agente destino.

  (dep: T045)
- [X] T047 [US4] Registrar `socratica` en `adapters/primary/cli/dispatcher.go` (lista de subcomandos de proyecto y `case`) y en la ayuda de `mem help`, hasta que T042 pase (dep: T046).

**Checkpoint**: CLI completa e independiente de los agentes.

---

## Phase 7: User Story 5 - Gestionar Socratica desde la TUI de gomemory (Priority: P2)

**Goal**: tres vistas Socratica (lista, detalle/revisión y preguntas) con el contrato visual, acuse inmediato y render puro.

**Independent Test**: render en 59, 80 y 120 columnas; `View()` repetido no muta nada.

### Tests for User Story 5

- [X] T048 [P] [US5] Escribir `TestSocraticaScreens` en `adapters/primary/tui/tui_socratica_test.go`. Casos:
  - lista con seguimiento de 4 etapas e historial truncado con elipsis; el detalle de una entrada del historial muestra original y propuesta aunque el flujo se haya cancelado y purgado (I6);
  - acciones por estado (I11), alineadas con el `next` de `contracts/mcp-tool.md`: revisión recuperable → Ejecutar · Editar · Cancelar; `in_flight` → «Envío en curso» sin acciones salvo volver; `uncertain` → «Puede que ya se haya enviado» con Marcar enviado · Reenviar · Cancelar, sin Ejecutar (U3); `dispatched` → «Ya enviado» con Reenviar solo como elección explícita (I10);
  - acuse «Generando preguntas con <agente>…» antes del `tea.Cmd`;
  - detalle lado a lado en 120 columnas, apilado en 80 y aviso de una línea en 59 con el original íntegro;
  - detalle en modo scrutiny que muestra la síntesis;
  - preguntas con marco de trazo grueso, contador y ✓;
  - alternar `i` respeta la capacidad;
  - Ejecutar/Cancelar se despachan como `tea.Cmd`, y Ejecutar llama a `SocraticaFlow.Execute` del caso de uso; con `ExecLauncher`, Ejecutar devuelve un `tea.ExecProcess` que cede la terminal mientras el agente corre, y al volver la vista muestra el flujo `dispatched` (I4);
  - un fallo del generador muestra «Reintentar · Enviar mi original · Cancelar» con el original intacto (G3);
  - la TUI no importa ni llama a `domain.Reduce`: todo cambio de estado pasa por `SocraticaFlow.Apply/Resume/Execute` en un `tea.Cmd` (K1);
  - la lista muestra el flujo pendiente de cada agente con su antigüedad; `p` alterna el modo privado y `r` abre el refinamiento con el acuse (M9).
  Las fakes van en `adapters/primary/tui/socratica_fakes_test.go`.
- [X] T049 [P] [US5] Escribir `TestSocraticaView_Pura` en `adapters/primary/tui/tui_socratica_test.go`: 100 llamadas a `View()` no cambian el modelo ni el repositorio o generador falsos (cero llamadas registradas).
- [X] T050 [P] [US5] Ampliar `adapters/primary/tui/visual_identity_test.go` con los tokens `soc*` de `contracts/tui.md` (valores hexadecimales exactos) y la ausencia de emojis en el render de Socratica.

### Implementation for User Story 5

- [X] T051 [P] [US5] Añadir los tokens `socDone`, `socCurrent`, `socUpcoming`, `socFg`, `socLabel`, `socControl` y `socWarn` en `adapters/primary/tui/styles.go`, hasta que T050 pase.
- [X] T052 [US5] Implementar `screenSocratica`, `screenSocraticaDetail` y la vista de preguntas en `adapters/primary/tui/socratica.go`, con todo cambio de estado vía `SocraticaFlow.Apply/Resume/Execute` en `tea.Cmd`; `Update` solo guarda el flujo devuelto (K1, constitución §3/§5), reutilizando el generador inyectado (`FirstAvailable`) y `SocraticaFlow.Execute` con el lanzador inyectado envuelto en un `tea.ExecCommand` y ejecutado con `tea.ExecProcess` (cede la terminal, `onStarted` escribe el ack y la vista se refresca al volver), hasta que T048 y T049 pasen (dep: T051, T045).
- [X] T053 [US5] Añadir la entrada «Socratica» al menú principal y la navegación (`esc`) en `adapters/primary/tui/tui.go`, y su ayuda en `adapters/primary/tui/tui_docs.go` (dep: T052). **Nota**: `tui_docs.go` administra documentos fijados y no tiene catálogo de atajos; la ayuda («p socratica») va en el pie de la lista principal (`listView`), donde viven los demás atajos.

**Checkpoint**: TUI operativa.

---

## Phase 8: User Story 6 - Intercepción automática opcional antes del envío (Priority: P3)

**Goal**: decisión pura de captura, marca de origen al lanzar desde CLI/TUI y su aplicación en los hooks de Claude/Codex y en el plugin de OpenCode, desactivada por defecto.

**Independent Test**: tabla de decisión, más pruebas de hook por dialecto y del plugin. Con todo desactivado, el hook no abre la base.

### Tests for User Story 6

- [X] T054 [P] [US6] Escribir `TestDecideInterception` en `domain/socratica_interception_test.go`. Casos: captura con intercepción activa y agente en reposo; deja pasar con `/`, adjunto, imagen, agente ocupado, ≤ `LowInfoMaxWords` palabras, flujo pendiente en questions/review y `Origin="socratica-approved"`; `>hola` → `Forward="hola"` y `>>hola` → `">hola"` (exactamente un `>`); deshabilitada → nunca captura; un flujo pendiente actualizado hace más de 30 min (`ContinuationWindow`) ya no cuenta como continuación (M1).
- [X] T055 [P] [US6] Escribir `TestUserPromptSubmit_Socratica` en `adapters/primary/cli/hook_socratica_test.go`. Casos: con captura, los dialectos Claude y JSON emiten `additionalContext` con la directiva; con `>`, la directiva de quitar el prefijo; con `GOMEMORY_SOCRATICA_DISPATCH` y texto igual a `approved_prompt`, no se captura y la marca se consume (U1); un **segundo** prompt en la misma sesión con la variable aún presente sí se captura (H2); con un `dispatch_id` desconocido, se evalúa normalmente; con `interception=true` pero capacidad false y sin `interception_unverified`, no se captura; con `interception_unverified=true`, sí (M5); `BenchmarkUserPromptSubmit_Off` y `_On` registran la latencia del hook (objetivo < 20 ms, registrado en evidencia, M8); con todo apagado, no se abre la base (repositorio falso sin llamadas) — **resuelto así**: T024/FR-026 exigen reconocer la respuesta a un flujo pendiente con la intercepción apagada, lo que requiere una lectura `ActiveFlow`; con la intercepción apagada esa es la única consulta (nunca `ConsumeDispatch` ni lógica de captura), y la invocación explícita no toca la base; dialectos neutrales reciben texto plano. Un prompt que pasa sin captura no crea flujo ni historial (G6, FR-027).
- [X] T056 [P] [US6] Ampliar `infrastructure/plugin/opencode/gomemory.test.mjs` con `chat.message` usando un `Exec` falso de `mem hook socratica-decide`: el plugin aplica la decisión JSON recibida y no contiene reglas propias (M4); captura y antepone la parte de directiva; con `>`, muta la parte quitando un `>`; un comando con barra pasa intacto; con `GOMEMORY_SOCRATICA_DISPATCH` no captura; apagado no hace nada.

### Implementation for User Story 6

- [X] T057 [US6] Implementar `DecideInterception` en `domain/socratica_interception.go` y ampliar `ClassifyPrompt` para que también resuelva `Capture/Forward/Directive` y marque `SkipProvenance` en todo prompt capturado, hasta que T054 pase.
- [X] T058 [US6] Añadir la rama Socratica en `user-prompt-submit` de `adapters/primary/cli/cmd_hook.go`: leer el ajuste antes de abrir la base; delegar toda la decisión en `SocraticaFlow.ClassifyPrompt` (ampliado con `DecideInterception` y `ConsumeDispatch`; el handler no contiene reglas, I1). Emitir la directiva por `renderPromptContext` de `adapters/primary/cli/hook_dialect.go`. Añadir el subcomando `mem hook socratica-decide` (entrada JSON por stdin acotada con `readBounded`, salida `{capture, forward, directive}`), hasta que T055 pase (dep: T057).
- [X] T059 [US6] Implementar la intercepción en `chat.message` de `infrastructure/plugin/opencode/gomemory.ts` llamando a `mem hook socratica-decide` por el `Exec` existente (sin replicar reglas en TypeScript, research R5), hasta que T056 pase. Las capacidades siguen en `false` hasta T065 (dep: T058).

**Checkpoint**: intercepción implementada pero no habilitable hasta tener evidencia.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: aceptación real por agente, matriz, documentación y regresión.

- [ ] T060 Ejecutar la aceptación MVP tras US3 en los tres agentes: pasos 1–3 y 5 de `quickstart.md` §6 (sin CLI ni intercepción), y la regresión acotada al MVP: `go test ./domain/ ./application/... ./adapters/secondary/persistence/ ./adapters/primary/cli/ ./adapters/primary/setup/ ./tests/... -count=1`, `go vet ./...` y `bash scripts/coverage.sh` sin bajar la línea base. Registrar órdenes y salidas en `specs/039-socratica-prompt-refinement/evidence/mvp.md`. No depende de CLI, TUI ni intercepción (I5) (dep: T039, T032).
- [ ] T061 Ejecutar `quickstart.md` §6 en Claude Code 2.1.300 tras `go build` y `mem setup-mcp --scope global --agents claude` en un proyecto de prueba, incluidos los 5 prompts de referencia de SC-005 (T002), y registrar órdenes y salidas en `specs/039-socratica-prompt-refinement/evidence/claude.md` (dep: T039, T047, T059).
- [ ] T062 Ejecutar `quickstart.md` §6 en Codex 0.162.1 (incluido comprobar si `$socratica` o `/socratic-q` funcionan como comando con nombre, y los 5 prompts de SC-005) y registrar en `specs/039-socratica-prompt-refinement/evidence/codex.md` (dep: T039, T047, T059).
- [ ] T063 Ejecutar `quickstart.md` §6 en OpenCode 2.0.27 (incluidos los 5 prompts de SC-005) y registrar en `specs/039-socratica-prompt-refinement/evidence/opencode.md` (dep: T039, T047, T059).
- [ ] T064 Rellenar `specs/039-socratica-prompt-refinement/contracts/compatibility-matrix.md` solo con las celdas que tengan evidencia en T061–T063, distinguiendo «garantizado» de «por directiva» en la fila de `>`; el resto queda «no soportado» con su alternativa en texto (dep: T061, T062, T063).
- [ ] T065 Poner a `true` en `domain/socratica_capabilities.go` exclusivamente las capacidades evidenciadas en T064 y actualizar `domain/socratica_capabilities_test.go` citando el archivo de evidencia en un comentario (dep: T064).
- [X] T066 Verificar si OpenCode 2.0.27 admite un agente de solo lectura invocable con `opencode run --agent <nombre>` (definición en su configuración, sin herramientas de escritura ni shell). Registrar orden y salida en `specs/039-socratica-prompt-refinement/evidence/opencode.md`. Solo si queda demostrado que no puede modificar archivos, añadir `adapters/secondary/generator/opencode.go` con prueba en `generator_test.go` y quitar la exclusión de T045; si no, dejar «no soportado» en la matriz (research R3).
- [X] T067 [P] Redactar `docs/socratica.md`. Debe cubrir:
  - modos y cómo invocarlos;
  - on/off solo desde CLI o TUI, y la orden que devuelve el agente;
  - historial y su tope;
  - aprobación como compuerta y ausencia de trabajo antes de Ejecutar;
  - prevención de duplicados, con un ejemplo de segundo approve y de `GOMEMORY_SOCRATICA_DISPATCH`;
  - un solo pendiente por agente;
  - privacidad y modo privado;
  - sobrecoste en tokens: preguntas y reescritura, más la directiva por turno si la intercepción está activa;
  - limitaciones: sin paneles nativos, OpenCode no es generador, la matriz;
  - uso desde CLI y TUI.

  Actualizar también `docs/DATABASE.md` (tablas `socratica_flows` y `socratica_history`, índices, retención y sanitize) y `docs/ARQUITECTURA.md` (ports `SocraticaRepository`, `SocraticaUnitOfWork`, `SocraticGenerator`, `AgentLauncher` y el flujo Execute) (M7).

  (dep: T064)
- [X] T068 [P] Enlazar `docs/socratica.md` desde `README.md` en la sección de funcionalidades y añadir la entrada en `CHANGELOG.md` bajo «Sin publicar» (dep: T067).
- [X] T069 Ejecutar `skills-ref validate skills/socratica` solo si `command -v skills-ref` lo encuentra; si no, registrar «no ejecutado: skills-ref no instalado» en `specs/039-socratica-prompt-refinement/evidence/README.md` sin afirmar validación.
- [X] T070 Ejecutar `go test ./... -count=1`, `go vet ./...` y `bash scripts/coverage.sh` (umbral 80 %, sin bajar la línea base, H3), y `go test -bench 'Socratica|UserPromptSubmit' -run '^$'` para los objetivos de latencia, y registrar los resultados en `specs/039-socratica-prompt-refinement/evidence/README.md`. Debe quedar en verde sin regresiones en MCP, sesiones, distribución de habilidades, hooks ni Octopus (SC-009) (dep: T053, T059, T047, T039).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Fase 1)**: sin dependencias.
- **Foundational (Fase 2)**: depende de la Fase 1 y bloquea todas las historias.
- **US1 (Fase 3)**, **US2 (Fase 4)**: dependen de la Fase 2. US2 edita el mismo `SKILL.md` que US1, así que va después de T025.
- **US3 (Fase 5)**: depende de T025/T026 (necesita la habilidad redactada). Puede ir en paralelo con US2.
- **US4 (Fase 6)**: depende de la Fase 2.
- **US5 (Fase 7)**: depende de la Fase 2 y de T045 (generador y lanzadores registrados en el composition root). `Execute` vive en el caso de uso (T018), así que no depende de la CLI.
- **US6 (Fase 8)**: depende de la Fase 2 (ajustes T016, `FindByDispatchID` T015).
- **Polish (Fase 9)**: depende de las historias que se quieran entregar.

### Within Each User Story

Pruebas primero (en rojo), luego dominio/puertos, después adaptadores y wiring, y al final la integración.

## Parallel Example

```text
Fase 2 (pruebas):   T003 ∥ T004 ∥ T005 ∥ T006 ∥ T007 ∥ T008 ∥ T009
Fase 2 (impl.):     T010 ∥ T011 ∥ T012 ∥ T016
US2 ∥ US3:          (T029 ∥ T030 → T031 → T032) ∥ (T033 ∥ T034 ∥ T035 → T036 → …)
US4 → US5:          (T040 ∥ T041 ∥ T042) → (T043 ∥ T044) → T045 → (T046 → T047) ∥ (T048 ∥ T049 ∥ T050 → T051 → T052)
US6:                T054 ∥ T055 ∥ T056 → T057 → T058 → T059
Polish:             T067 ∥ T068 tras T064
```

## Implementation Strategy

### MVP First

1. Fases 1 y 2.
2. US1, luego US2, luego US3 (los tres P1): habilidad con compuerta instalada en los tres agentes.
3. **PARAR y validar**: T060 (aceptación y regresión acotada del MVP, `evidence/mvp.md`). T070 es la regresión final y solo corre tras US4–US6.

### Incremental Delivery

4. US4 (CLI) → validar.
5. US5 (TUI) → validar.
6. US6 (intercepción) → T061–T065 completos, y solo entonces habilitar `on` donde haya evidencia.
7. T066 (OpenCode como generador), documentación y regresión final.

## Notes

- `[P]` = archivos distintos sin dependencias pendientes.
- No se modifican pruebas existentes para hacer pasar una implementación (constitución §10). T034 y T050 **amplían** las existentes porque cambia el comportamiento esperado, y la spec lo respalda.
- No hacer commit sin mostrar antes la lista de archivos (preferencia global). `CLAUDE.md`, `.env*` y certificados quedan siempre fuera.
