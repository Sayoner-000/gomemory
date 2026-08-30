---
description: "Lista de tareas para el fortalecimiento de la revisión ACR"
---

# Tareas: Fortalecimiento de la revisión ACR

**Entrada**: documentos de diseño en `/specs/028-harden-acr-review/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: OBLIGATORIOS. La constitución declara el principio III (*Testing First*) como
NO NEGOCIABLE: el test se escribe primero, se ve fallar, y solo entonces se implementa.
Cada tarea de implementación de esta lista tiene su test escrito antes.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se
pueda implementar, probar y entregar de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: puede ejecutarse en paralelo (archivos distintos, sin dependencias pendientes)
- **[Historia]**: a qué historia de usuario pertenece (US1…US5)
- Toda tarea incluye la ruta exacta del archivo

## Convención de rutas

Proyecto único con arquitectura hexagonal, la que ya usa el repositorio:
`domain/`, `application/ports/`, `application/usecases/`, `adapters/secondary/persistence/`,
`adapters/primary/cli/`, `tests/{unit,integration,contract}/`.

---

## ⚠️ Excepción constitucional declarada: tests existentes que se modifican

El principio III prohíbe modificar tests existentes sin autorización explícita. La
sección *Assumptions* de [spec.md](./spec.md) la concede **solo** para los tests que hoy
exigen el comportamiento defectuoso. Esta es la lista cerrada; ningún otro test se toca.

| Test | Archivo | Por qué cambia |
|---|---|---|
| `TestBuildConsensusValidatesIndependentSourcesAndEvidence` | `application/usecases/build_consensus_test.go` | Clasifica 2 de los 5 hallazgos y hoy pasa. Bajo FR-001 eso es un rechazo: el test debe clasificar los cinco |
| `TestFinalizeReview_EmiteMetricasDelProtocolo` | `application/usecases/finalize_review_test.go` | Usa el mapa plano de re-juicio y no verifica `duration` ni las métricas de promoción |
| `TestRejudgeReview_RegistraLosTresEstados` | `application/usecases/rejudge_review_test.go` | El re-juicio pasa a ser por revisor; el mapa plano deja de existir |
| `TestRejudgeReview_ExigeUnaCorreccionPrevia` | `application/usecases/rejudge_review_test.go` | Misma firma de entrada |
| `TestRejudgeReview_SoloConfirmados` | `application/usecases/rejudge_review_test.go` | Misma firma de entrada |
| `TestRejudgeReview_RechazaEstadoInvalido` | `application/usecases/rejudge_review_test.go` | Misma firma de entrada |
| `TestRejudgeReview_ResueltoPermiteAprobar` | `application/usecases/rejudge_review_test.go` | `RESOLVED` pasa a exigir dos revisores; con uno ya no aprueba |
| `TestRejudgeReview_SinResolverNoAprueba` | `application/usecases/rejudge_review_test.go` | Misma firma de entrada |
| `escenarioReRevisable`, `entradaDeReRevision` | `application/usecases/rejudge_review_test.go` | Helpers compartidos de los anteriores |
| `TestRecordFix_*` (6 tests) | `application/usecases/record_fix_test.go` | `base_target_digest` pasa a validarse contra el target vigente; los digests inventados dejan de valer |
| `TestPromoteReviewMemory_SoloConfirmadoYResuelto`, `_RechazaSinResolver` | `application/usecases/promote_review_memory_test.go` | Usan el re-juicio con firma vieja y una revisión sin veredicto `APPROVED` |
| `TestReviewMCPToolContracts` | `tests/contract/review_protocol_test.go` | Valida la forma de las respuestas, que cambia en `review_status`, `review_finalize` y `review_rejudge` |
| `TestReviewCLI_ContratoDeConsulta` | `tests/contract/review_cli_test.go` | `mem review show` y `status` amplían su salida |
| `TestReviewApprovedFlow`, `TestReviewEscalatedFlow` | `tests/integration/` | Recorren el ciclo completo, cuya firma de re-juicio cambia |

Tests que **NO** se tocan y deben seguir en verde sin edición: todo `domain/*_test.go`
existente, `start_review_test.go`, y el resto de la suite del proyecto ajena a la
revisión.

#### Ampliación de la lista durante la implementación

La lista de arriba era una predicción hecha leyendo el código. Al implementar FR-007
aparecieron dos casos que no había previsto. Se declaran aquí en vez de editarlos en
silencio, que es justo lo que la lista existe para impedir:

| Test | Archivo | Qué se cambió y por qué |
|---|---|---|
| `TestSubmitReviewerResultValidatesDigestIdempotencyAndFailure` | `application/usecases/submit_reviewer_result_test.go` | **Solo el fixture**: sus hallazgos omitían `location`, `category` y `confidence`, que FR-007 vuelve obligatorios. Ninguna aserción cambió — sigue verificando digest, idempotencia y finalidad del fallo exactamente igual |
| `TestReviewApprovedFlow` | `tests/integration/review_approved_flow_test.go` | Igual: fixture completado con los tres campos. Ya estaba en la lista original por la firma de re-juicio |

La diferencia importa: completar un fixture incompleto no es lo mismo que relajar una
aserción. Estos dos tests siguen exigiendo lo mismo que exigían antes.

---

## Fase 1: Preparación

**Propósito**: dejar constancia del punto de partida para poder demostrar el cambio.

- [X] T001 Capturar la línea base ejecutando `go test ./... > /tmp/028-baseline.txt 2>&1` y registrar el resultado en `specs/028-harden-acr-review/tasks.md` como nota al pie
- [X] T002 [P] Verificar que `gofumpt -l .` y `golangci-lint run` salen limpios ANTES de tocar nada, para que cualquier hallazgo posterior sea atribuible a esta funcionalidad
- [X] T003 [P] Copiar una `mem.db` real a `/tmp/mem-antes-028.db` y anotar los recuentos de `reviews`, `findings`, `consensus_findings` y `fix_rounds` para el escenario 0 de `quickstart.md`

### Línea base registrada (T001–T003)

- **T001**: `go test ./...` sale en verde. 13 paquetes `ok`, 0 `FAIL`, código de salida 0.
- **T002**: `gofumpt` y `golangci-lint` **no están instalados** en este entorno. Se usó
  la alternativa disponible: `go vet ./...` limpio y `gofmt -l .` con **13 archivos
  preexistentes** sin formatear, ninguno de ellos entre los que toca esta funcionalidad
  (lista guardada en `/tmp/028-gofmt-baseline.txt`). Cualquier archivo del ledger de
  revisión que aparezca en `gofmt -l` al terminar es atribuible a este trabajo.
- **T003**: base de `gomemory` respaldada en `/tmp/mem-antes-028.db` con
  `reviews=2`, `findings=15`, `consensus_findings=13`, `fix_rounds=0`. La revisión
  `acr_96710834-8273-49f3-bd11-42764b2f11d4` está en `consensus_ready` sin veredicto:
  es el bloqueo que FR-019 debe desatascar.
- **Hallazgo del paso**: copiar una base SQLite en modo WAL con `cp` **pierde** lo que
  aún está en el `-wal`; la copia parecía no tener ni la tabla `reviews`. El respaldo
  hay que hacerlo con `sqlite3 origen ".backup destino"`. Aplica también al test de
  migración de T004.

---

## Fase 2: Fundacional (prerrequisitos bloqueantes)

**Propósito**: esquema, entidades y puertos que TODAS las historias necesitan.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

### Tests de la fase fundacional

- [X] T004 [P] Escribir test de migración aditiva en `adapters/secondary/persistence/review_test.go` que abra una base con el esquema anterior a 028, ejecute la migración y verifique que las columnas nuevas existen con `notnull = 0` y que los recuentos de filas no cambian
- [X] T005 [P] Escribir test de `Review.TransitionTo` en `domain/review_test.go` que verifique que un estado terminal rechaza cualquier transición y que una transición no permitida por `CanTransitionTo` devuelve error
- [X] T006 [P] Escribir test de agregación de re-juicios en `domain/rejudgment_test.go` cubriendo la tabla completa de FR-014: cualquier `REGRESSED` gana, `RESOLVED` exige a los dos, un solo revisor deja `UNRESOLVED`
- [X] T007 [P] Escribir test de orden de severidad en `domain/finding_test.go` que verifique `CRITICAL > HIGH > MEDIUM > LOW > INFO` y el máximo de un conjunto

### Implementación de la fase fundacional

- [X] T008 Añadir las columnas nuevas de `reviews` (`current_target_digest`, `fix_authorized`, `reviewer_a_provider`, `reviewer_a_model`, `reviewer_b_provider`, `reviewer_b_model`) con `addColumnIfMissing` en `adapters/secondary/persistence/db.go`
- [X] T009 Añadir la columna `round_fingerprint` a `consensus_findings` con `addColumnIfMissing` en `adapters/secondary/persistence/db.go`
- [X] T010 Crear la tabla `rejudgments` y su índice `idx_rejudgments_finding` con `CREATE TABLE IF NOT EXISTS` en `adapters/secondary/persistence/db.go` según el DDL de `data-model.md`
- [X] T011 [P] Añadir `CurrentTargetDigest`, `FixAuthorized` y las identidades esperadas de revisor a la struct `Review` en `domain/review.go`
- [X] T012 [P] Implementar `Review.TransitionTo(next ReviewStatus) error` en `domain/review.go` apoyándose en `Terminal()` y `CanTransitionTo()`, que ya existen
- [X] T013 [P] Crear `domain/rejudgment.go` con la entidad `ReJudgment` (revisor, ronda, hallazgo, estado, evidencia) y la función pura `AggregateReJudgment([]ReJudgment) ReJudgmentState`
- [X] T014 [P] Añadir `Severity.Rank() int` y `MaxSeverity(...Severity) Severity` en `domain/finding.go`
- [X] T015 Ampliar `ports.ConsensusRepository` en `application/ports/consensus_repository.go` con `UpsertReJudgment`, `ListReJudgments(project, reviewID, consensusLocalID)` y `RecordFixAtomically`
- [X] T016 Persistir y leer las columnas nuevas de `reviews` y `consensus_findings` en `adapters/secondary/persistence/review.go`, con `current_target_digest` vacío interpretado como el target original y `fix_authorized` nulo como `1`
- [X] T017 Implementar la persistencia de `rejudgments` en `adapters/secondary/persistence/review.go`, aplicando `redactarLista` a la evidencia (FR-027)
- [X] T018 Actualizar los repositorios en memoria de `application/usecases/review_test_helpers_test.go` para cubrir los métodos nuevos del puerto

**Punto de control**: el esquema migra sin pérdida, el dominio expone las invariantes nuevas y la suite sigue compilando.

---

## Fase 3: Historia de Usuario 1 — Impedir aprobaciones falsas (Prioridad: P1) 🎯 MVP

**Objetivo**: ninguna clasificación parcial, duplicada o con severidad degradada puede terminar en `APPROVED`.

**Prueba independiente**: presentar resultados con varios hallazgos graves e intentar omitirlos, repetirlos o bajarles la severidad durante el consenso; ningún caso debe terminar aprobado.

### Tests de la Historia 1

> Escribir primero, verlos fallar, y solo entonces implementar.

- [X] T019 [P] [US1] Escribir test de cobertura total en `domain/consensus_coverage_test.go`: un hallazgo de la ronda sin clasificar devuelve error nombrando su `local_id` (FR-001)
- [X] T020 [P] [US1] Escribir test de unicidad en `domain/consensus_coverage_test.go`: un ID repetido, o presente a la vez en emparejados y no emparejados, devuelve error (FR-002)
- [X] T021 [P] [US1] Escribir test de ronda cruzada en `domain/consensus_coverage_test.go`: un ID de otra ronda o de otra revisión se rechaza (FR-002)
- [X] T022 [P] [US1] Escribir test de severidad derivada en `domain/consensus_coverage_test.go`: dos fuentes `HIGH` producen `HIGH` aunque la entrada declare `LOW` (FR-003)
- [X] T023 [P] [US1] Escribir test en `domain/verdict_test.go` que verifique que `DeriveVerdict` NO aprueba cuando existe un hallazgo fuente sin clasificar, incluso habiendo filas de consenso (FR-004)
- [X] T024 [P] [US1] Escribir test de idempotencia en `application/usecases/build_consensus_test.go`: reenviar la misma clasificación en otro orden devuelve lo persistido con los mismos `consensus_local_id` y sin escribir (FR-005)
- [X] T025 [P] [US1] Escribir test de reemplazo rechazado en `application/usecases/build_consensus_test.go`: una clasificación divergente para la misma ronda se rechaza y conserva la original (FR-005)
- [X] T026 [P] [US1] Escribir test de identidad de revisor en `application/usecases/submit_reviewer_result_test.go`: un resultado con `provider`/`model` distintos del asignado se rechaza (FR-006)
- [X] T027 [P] [US1] Escribir test de campos obligatorios en `application/usecases/submit_reviewer_result_test.go`: un hallazgo sin `location`, `category`, `confidence` o sin evidencia no vacía se rechaza nombrando el campo (FR-007)
- [X] T028 [US1] Actualizar `TestBuildConsensusValidatesIndependentSourcesAndEvidence` en `application/usecases/build_consensus_test.go` para clasificar los cinco hallazgos del escenario (excepción constitucional declarada arriba)

### Implementación de la Historia 1

- [X] T029 [US1] Crear `domain/consensus_coverage.go` con `ValidateCoverage(sources []Finding, matches, unmatched)` que exija cobertura total, unicidad y pertenencia a la ronda (FR-001, FR-002)
- [X] T030 [US1] Añadir a `domain/consensus_coverage.go` la derivación de severidad como máximo de las fuentes y el rechazo de una severidad declarada divergente (FR-003)
- [X] T031 [US1] Añadir a `domain/consensus_coverage.go` el cálculo de la huella determinista de la ronda, ordenada por el menor ID de hallazgo fuente (FR-005)
- [X] T032 [US1] Reescribir `BuildConsensus` en `application/usecases/build_consensus.go` para delegar la validación en el dominio, asignar `consensus_local_id` tras ordenar por fuente y no aceptar severidad del llamador
- [X] T033 [US1] Implementar en `application/usecases/build_consensus.go` la ruta idempotente: huella igual devuelve lo persistido sin escribir; huella distinta rechaza con `la ronda <n> ya tiene un consenso registrado y no admite reemplazo` (FR-005)
- [X] T034 [US1] Extender `DeriveVerdict` en `domain/verdict.go` para no aprobar mientras exista cualquier hallazgo fuente sin clasificar, sustituyendo la comprobación actual de `len(consensus) == 0` (FR-004)
- [X] T035 [US1] Persistir la identidad esperada de los revisores en `StartReview` (`application/usecases/start_review.go`) y validarla en `SubmitReviewerResult` (`application/usecases/submit_reviewer_result.go`) (FR-006)
- [X] T036 [US1] Validar los campos obligatorios del hallazgo estructurado en `application/usecases/submit_reviewer_result.go`, en el borde del sistema (FR-007)
- [X] T037 [US1] Actualizar los DTO y mensajes de error de `review_consensus` y `review_submit` en `adapters/primary/cli/cmd_mcp_review_tools.go` según `contracts/mcp-tools.md`, con `severity` ya opcional e informativo

**Punto de control**: la Historia 1 cierra el defecto HIGH `C-001` de la revisión original. `go test ./domain/... ./application/...` en verde y los escenarios 1, 2 y 3 de `quickstart.md` pasan.

---

## Fase 4: Historia de Usuario 2 — Corregir y revalidar con trazabilidad (Prioridad: P1)

**Objetivo**: cada corrección mantiene una cadena verificable entre target original, target corregido, hallazgos abordados y dos re-juicios independientes.

**Prueba independiente**: registrar dos defectos confirmados, corregir solo uno e intentar resolver ambos; únicamente el incluido en la corrección y validado por los dos revisores puede quedar resuelto.

### Tests de la Historia 2

- [X] T038 [P] [US2] Escribir test de target vigente en `application/usecases/record_fix_test.go`: la primera corrección debe partir del digest original y se rechaza cualquier otro (FR-008, FR-009)
- [X] T039 [P] [US2] Escribir test de cadena en `application/usecases/record_fix_test.go`: la ronda 2 que no parte del `fixed_target_digest` de la ronda 1 se rechaza (FR-009)
- [X] T040 [P] [US2] Escribir test de resultado post-corrección en `application/usecases/submit_reviewer_result_test.go`: tras una corrección, el `target_digest` se valida contra el corregido, no contra el original (FR-011)
- [X] T041 [P] [US2] Escribir test de re-juicio por revisor en `application/usecases/rejudge_review_test.go`: se persiste una fila por revisor con su evidencia (FR-012)
- [X] T042 [P] [US2] Escribir test de unanimidad en `application/usecases/rejudge_review_test.go`: con un solo `RESOLVED` el agregado es `UNRESOLVED`; con los dos, `RESOLVED` (FR-013)
- [X] T043 [P] [US2] Escribir test de hallazgo fuera de la corrección en `application/usecases/rejudge_review_test.go`: re-juzgar un hallazgo que la corrección vigente no incluye se rechaza (FR-013)
- [X] T044 [P] [US2] Escribir test de `REGRESSED` en `application/usecases/rejudge_review_test.go`: un `REGRESSED` de cualquier revisor manda sobre el `RESOLVED` del otro (FR-014)
- [X] T045 [P] [US2] Escribir test de concurrencia en `tests/integration/review_concurrent_fix_test.go`: 100 registros concurrentes de la misma ronda dejan una única fila en `fix_rounds` y ninguna corrección sobrescrita (SC-003, FR-010)
- [X] T046 [US2] Actualizar `escenarioReRevisable`, `entradaDeReRevision` y los seis `TestRejudgeReview_*` en `application/usecases/rejudge_review_test.go` a la entrada por revisor (excepción constitucional declarada arriba)
- [X] T047 [US2] Actualizar los seis `TestRecordFix_*` en `application/usecases/record_fix_test.go` para usar digests coherentes con la cadena de targets (excepción constitucional declarada arriba)

### Implementación de la Historia 2

- [X] T048 [US2] Inicializar `CurrentTargetDigest` con el digest original en `StartReview` (`application/usecases/start_review.go`) y exponerlo (FR-008)
- [X] T049 [US2] Validar en `RecordFix` (`application/usecases/record_fix.go`) que `base_target_digest` es el target vigente, con el mensaje del contrato (FR-009)
- [X] T050 [US2] Implementar `RecordFixAtomically` en `adapters/secondary/persistence/review.go` con `BEGIN IMMEDIATE`, cubriendo la lectura de rondas, la inserción del delta y la actualización de `round`, `status` y `current_target_digest` (FR-010)
- [X] T051 [US2] Reescribir `RecordFix` en `application/usecases/record_fix.go` para usar la transacción del puerto en vez de las cuatro operaciones sueltas actuales (FR-010)
- [X] T052 [US2] Validar el `target_digest` de los resultados contra el target vigente en `application/usecases/submit_reviewer_result.go` (FR-011)
- [X] T053 [US2] Reescribir `RejudgeReview` en `application/usecases/rejudge_review.go` con entrada por revisor (`reviewer` + `judgments`), persistiendo una fila en `rejudgments` por hallazgo y revisor (FR-012)
- [X] T054 [US2] Exigir en `application/usecases/rejudge_review.go` que el hallazgo esté en `addressed_consensus_ids` de la corrección vigente (FR-013)
- [X] T055 [US2] Recalcular `consensus_findings.rejudgment_state` con `AggregateReJudgment` dentro de la misma transacción del re-juicio en `adapters/secondary/persistence/review.go` (FR-014)
- [X] T056 [US2] Actualizar el DTO de `review_rejudge` en `adapters/primary/cli/cmd_mcp_review_tools.go` a la entrada y salida de `contracts/mcp-tools.md`, con `reviewer_states` y `aggregate_state`

**Punto de control**: la cadena de corrección es verificable de punta a punta. Escenarios 4, 5 y 6 de `quickstart.md` pasan.

---

## Fase 5: Historia de Usuario 3 — Ciclo de vida y política (Prioridad: P2)

**Objetivo**: distinguir revisión de solo lectura de revisión autorizada a corregir, respetar la política del proyecto y mantener inmutables los estados terminales.

**Prueba independiente**: una revisión de solo lectura con un defecto grave confirmado termina escalada; enviar resultados después de un estado terminal se rechaza.

### Tests de la Historia 3

- [X] T057 [P] [US3] Escribir test de guarda terminal en `application/usecases/submit_reviewer_result_test.go`, `build_consensus_test.go`, `record_fix_test.go` y `rejudge_review_test.go`: sobre una revisión `approved`, `escalated` o `incomplete`, las cuatro operaciones se rechazan sin tocar el ledger (FR-015, FR-016)
- [X] T058 [P] [US3] Escribir test de solo lectura en `domain/verdict_test.go`: con `FixAuthorized = false` y un `CONFIRMED` severo sin resolver, `DeriveVerdict` devuelve `ESCALATED` (FR-019)
- [X] T059 [P] [US3] Escribir test de presupuesto en `domain/verdict_test.go`: con `FixAuthorized = true` y rondas disponibles, la revisión sigue abierta; agotado el presupuesto, `ESCALATED` (FR-020)
- [X] T060 [P] [US3] Escribir test de política en `application/usecases/start_review_test.go`: sin valores explícitos, la revisión toma `max_fix_rounds` y `auto_fix_severities` del proyecto y los persiste (FR-017)
- [X] T061 [P] [US3] Escribir test de persistencia de política en `adapters/secondary/persistence/settings_test.go`: `review_fix_authorized` sobrevive a una escritura conjunta con otras preferencias (FR-017)
- [X] T062 [P] [US3] Escribir test de promoción en `application/usecases/promote_review_memory_test.go`: un hallazgo confirmado y resuelto en una revisión no aprobada no se promueve (FR-021)
- [X] T063 [US3] Actualizar `TestPromoteReviewMemory_SoloConfirmadoYResuelto` y `_RechazaSinResolver` en `application/usecases/promote_review_memory_test.go` para que la revisión llegue a `APPROVED` (excepción constitucional declarada arriba)

### Implementación de la Historia 3

- [X] T064 [US3] Crear `domain/review_policy.go` con `ReviewPolicy{FixAuthorized, MaxFixRounds, AutoFixSeverities}` y su resolución por precedencia: explícito → proyecto → defecto del dominio (FR-017, FR-018)
- [X] T065 [US3] Añadir `ReviewFixAuthorized` a `Settings` y a `applyReviewDefaults` en `adapters/secondary/persistence/settings.go`, con defecto `true` (FR-017)
- [X] T066 [US3] Leer la política del proyecto y pasarla a `StartReview` desde `adapters/primary/cli/cmd_review.go` y desde la tool `review_start` en `adapters/primary/cli/cmd_mcp_review_tools.go` (FR-017)
- [X] T067 [US3] Eliminar los defectos hardcodeados de `application/usecases/start_review.go` y sustituirlos por la política recibida y `domain.DefaultMaxFixRounds` (FR-017)
- [X] T068 [US3] Derivar `ESCALATED` en `domain/verdict.go` cuando exista un `CONFIRMED` severo sin resolver y `FixAuthorized` sea falso, en vez de devolver la cadena vacía (FR-019, FR-020)
- [X] T069 [US3] Sustituir toda asignación directa de `review.Status` por `Review.TransitionTo` en `application/usecases/submit_reviewer_result.go`, `record_fix.go` y `finalize_review.go` (FR-015)
- [X] T070 [US3] Añadir la guarda de estado terminal al inicio de `SubmitReviewerResult`, `BuildConsensus`, `RecordFix`, `RejudgeReview` y `PromoteReviewMemory` en `application/usecases/` (FR-016)
- [X] T071 [US3] Exigir veredicto `APPROVED` en `application/usecases/promote_review_memory.go` (FR-021)
- [X] T072 [US3] Añadir `--read-only` a `mem review` y exponer `fix_authorized` en la salida, en `adapters/primary/cli/cmd_review.go`, según `contracts/cli-contracts.md`

**Punto de control**: la revisión original `acr_96710834-…` deja de estar bloqueada. Escenarios 7, 8, 12 y 13 de `quickstart.md` pasan.

---

## Fase 6: Historia de Usuario 4 — Auditoría y target completo (Prioridad: P2)

**Objetivo**: resumen completo, métricas coherentes con el contrato, linaje de cada hallazgo y congelado de todos los cambios pendientes.

**Prueba independiente**: iniciar una revisión con cambios preparados, sin preparar y archivos nuevos, y comprobar en el ledger que el target y la cadena de evidencia son visibles y coherentes.

### Tests de la Historia 4

- [X] T073 [P] [US4] Escribir test de digest de cambios pendientes en `tests/contract/review_cli_test.go`: preparados, sin preparar y archivos nuevos producen un digest reproducible que cambia al modificar cualquiera (FR-025, SC-004)
- [X] T074 [P] [US4] Escribir test de nombres con espacios y archivos borrados en `tests/contract/review_cli_test.go` (casos límite de la spec)
- [X] T075 [P] [US4] Escribir test de árbol limpio en `tests/contract/review_cli_test.go`: `mem review --pending` sin cambios falla con diagnóstico y código 1 (FR-026)
- [X] T076 [P] [US4] Escribir test de recuentos en `tests/contract/review_protocol_test.go`: `review_status` devuelve `counts.by_status`, `by_severity` y `by_rejudgment` (FR-022)
- [X] T077 [P] [US4] Escribir test de linaje en `tests/contract/review_protocol_test.go`: `review_status` permite recorrer fuentes, consenso, correcciones y re-juicios de un hallazgo en una sola consulta (FR-023, SC-006)
- [X] T078 [P] [US4] Escribir test de contrato de métricas en `tests/contract/review_protocol_test.go` que compare las claves serializadas de `metrics` con las ocho exactas del contrato, en `snake_case` (FR-024, SC-007)
- [X] T079 [P] [US4] Escribir test de redacción en `adapters/secondary/persistence/review_test.go`: un secreto en la evidencia de re-juicio no llega al ledger en claro (FR-027)
- [X] T080 [P] [US4] Escribir test de rendimiento en `tests/integration/review_status_perf_test.go`: `review_status` y `review_finalize` responden en menos de 2 s con 1.000 hallazgos (SC-008)
- [X] T081 [US4] Actualizar `TestReviewMCPToolContracts` en `tests/contract/review_protocol_test.go` y `TestReviewCLI_ContratoDeConsulta` en `tests/contract/review_cli_test.go` a las salidas ampliadas (excepción constitucional declarada arriba)

### Implementación de la Historia 4

- [X] T082 [US4] Añadir `Duration`, `MemoryPromoted` y `MemoryDeduplicated` a `ReviewMetrics` en `application/usecases/finalize_review.go`, con `duration` derivada de `created_at`/`updated_at` (FR-024)
- [X] T083 [US4] Contar promociones y deduplicaciones por `topic_key` en `application/usecases/promote_review_memory.go` y exponerlas para la métrica (FR-024)
- [X] T084 [US4] Crear el DTO de métricas con etiquetas JSON en `snake_case` en `adapters/primary/cli/cmd_mcp_review_tools.go` y serializar desde él, no desde el struct de aplicación (FR-024)
- [X] T085 [US4] Ampliar la salida de `review_status` en `adapters/primary/cli/cmd_mcp_review_tools.go` con target, política, revisores, recuentos, hallazgos con linaje y rondas de corrección (FR-022, FR-023)
- [X] T086 [US4] Implementar `resolvePendingTarget` en `adapters/primary/cli/cmd_review.go` con `git status --porcelain=v1 -z --untracked-files=all`, rutas ordenadas y hash con separadores nulos (FR-025)
- [X] T087 [US4] Añadir el modo `--pending` y su rechazo con target vacío a `CmdReview` en `adapters/primary/cli/cmd_review.go` (FR-025, FR-026)
- [X] T088 [US4] Ampliar `mem review show` y `mem review status` en `adapters/primary/cli/cmd_review.go` según `contracts/cli-contracts.md` (FR-022, FR-023)
- [X] T089 [US4] Aplicar `redactarTexto` y `redactarLista` a todos los campos nuevos de evidencia y re-juicio en `adapters/secondary/persistence/review.go` (FR-027)

**Punto de control**: un auditor reconstruye cualquier hallazgo con una sola consulta y las métricas coinciden con el contrato. Escenarios 9, 10, 11 y 14 de `quickstart.md` pasan.

---

## Fase 7: Historia de Usuario 5 — Cerrar los hallazgos sin borrar el historial (Prioridad: P3)

**Objetivo**: cerrar la revisión que descubrió estos defectos y validar el resultado con una revisión adversarial nueva, sin destruir evidencia.

**Prueba independiente**: registrar el delta de corrección en la revisión original, obtener dos re-juicios y revisar el target corregido desde cero; la original conserva sus hallazgos resueltos y la nueva no confirma defectos graves.

**⚠️ Depende de que las Historias 1 a 4 estén implementadas**: esta fase no aporta código de producto, ejecuta el protocolo sobre el resultado.

### ⛔ Fase NO ejecutada — decisión tomada el 2026-08-29

Las ocho tareas de esta fase quedan **pendientes a propósito**, por dos motivos que se
plantearon antes de tocar nada:

1. **Es irreversible sobre datos reales.** Registrar la corrección, los re-juicios y
   finalizar `acr_96710834` deja la revisión en un estado terminal inmutable en la base
   del proyecto. No hay marcha atrás por diseño (FR-016).
2. **La independencia sería falsa.** El protocolo exige dos revisores independientes.
   Un solo agente haciendo los dos papeles produce una corroboración nominal: el propio
   sistema la marcaría `degraded`, y un `RESOLVED` obtenido así no significa lo que
   dice significar. Ejecutarla habría producido un ledger con cara de validado y sin la
   propiedad que lo hace valer.

Lo que SÍ se demostró, sobre una **copia** de la base real: la revisión `acr_96710834`,
que llevaba atascada en `consensus_ready` desde que se abrió, ahora finaliza
`ESCALATED` en una sola llamada cuando se declara de solo lectura, y sigue abierta
cuando la corrección está autorizada y quedan rondas. Es exactamente el defecto que
motivó la funcionalidad.

Estas tareas se ejecutan cuando haya dos revisores reales —dos modelos o sesiones
distintas—, que es lo que el protocolo pide.

- [ ] T090 [US5] Verificar que la revisión `acr_96710834-8273-49f3-bd11-42764b2f11d4` conserva sus hallazgos tras la migración, con `mem review show`, antes de tocarla (FR-028)
- [ ] T091 [US5] Registrar el delta de corrección de los hallazgos confirmados de esa revisión con `review_fix_record`, declarando rutas modificadas y verificación (FR-029)
- [ ] T092 [US5] Obtener el re-juicio del revisor A sobre los hallazgos corregidos con `review_rejudge`, con evidencia verificable (FR-013)
- [ ] T093 [US5] Obtener el re-juicio independiente del revisor B sobre los mismos hallazgos con `review_rejudge` (FR-013)
- [ ] T094 [US5] Finalizar la revisión original con `review_finalize` y comprobar con `mem review show` que conserva todos sus hallazgos con los confirmados en `RESOLVED` (SC-009, FR-028)
- [ ] T095 [US5] Congelar el target corregido con `mem review --pending` e iniciar una revisión adversarial nueva (FR-030)
- [ ] T096 [US5] Ejecutar la revisión nueva completa con dos revisores independientes y registrar su consenso con `review_submit` y `review_consensus` (FR-030)
- [ ] T097 [US5] Finalizar la revisión nueva con `review_finalize` y verificar `APPROVED` sin defectos graves confirmados ni contradicciones severas (SC-010)

**Punto de control**: si la revisión nueva confirma un defecto severo, la funcionalidad NO está terminada. Ese es el propósito de esta fase.

---

## Fase 8: Pulido y asuntos transversales

- [X] T098 [P] Actualizar `docs/architecture.md` con la tabla `rejudgments`, la política de revisión y la máquina de estados obligatoria
- [X] T099 [P] Actualizar `docs/MANUAL.md` con `mem review --pending`, `--read-only` y la salida ampliada de `show`
- [X] T100 [P] Añadir la entrada de la versión al `CHANGELOG.md` describiendo los tres defectos cerrados
- [X] T101 [P] Actualizar `README.md` si el recuento de tools MCP o la descripción del grupo de revisión cambian
- [X] T102 Ejecutar `gofumpt -l .`, `golangci-lint run`, `go vet ./...` y `go test ./...`, y comparar con la línea base de T001 (SC-011)
- [X] T103 Ejecutar la guía completa de `specs/028-harden-acr-review/quickstart.md`, escenarios 0 a 15, contra el binario compilado
- [X] T104 Guardar en gomemory la causa raíz de cada defecto cerrado con `save_memory`, y promover con `review_promote_memory` los aprendizajes de la revisión aprobada

---

## Dependencias y orden de ejecución

### Dependencias entre fases

- **Preparación (Fase 1)**: sin dependencias, puede empezar de inmediato
- **Fundacional (Fase 2)**: depende de la Fase 1 — BLOQUEA todas las historias
- **Historias (Fases 3-6)**: todas dependen de la Fase 2
- **Historia 5 (Fase 7)**: depende de que las Historias 1 a 4 estén implementadas; es verificación en vivo, no código de producto
- **Pulido (Fase 8)**: depende de todas las anteriores

### Dependencias entre historias

- **US1 (P1)**: puede empezar en cuanto termine la Fase 2. Sin dependencias de otras historias
- **US2 (P1)**: puede empezar en cuanto termine la Fase 2. Comparte con US1 el patrón de validación-antes-de-escribir, pero no depende de su código
- **US3 (P2)**: puede empezar en cuanto termine la Fase 2. Toca `DeriveVerdict`, que US1 también modifica: si se trabaja en paralelo, coordinar `domain/verdict.go`
- **US4 (P2)**: puede empezar en cuanto termine la Fase 2. Su parte de re-juicio en `review_status` se completa cuando US2 esté lista
- **US5 (P3)**: requiere US1 a US4 completas

### Dentro de cada historia

- Los tests se escriben y se ven fallar ANTES de la implementación (principio III)
- Dominio antes que casos de uso; casos de uso antes que adaptadores
- Historia completa y verde antes de pasar a la siguiente prioridad

### Puntos de conflicto entre historias

Tres archivos los tocan varias historias. Si se trabaja en paralelo, es donde hay que coordinar:

| Archivo | Historias | Nota |
|---|---|---|
| `domain/verdict.go` | US1 (T034), US3 (T068) | Dos cambios distintos a `DeriveVerdict` |
| `application/usecases/submit_reviewer_result.go` | US1 (T035, T036), US2 (T052), US3 (T069, T070) | |
| `adapters/primary/cli/cmd_mcp_review_tools.go` | US1 (T037), US2 (T056), US3 (T066), US4 (T084, T085) | Secciones distintas del mismo archivo |

### Oportunidades de paralelismo

- T002 y T003 en paralelo
- T004 a T007 (tests de la fase fundacional) en paralelo
- T011 a T014 (dominio puro, archivos distintos) en paralelo
- T019 a T027 (tests de US1) en paralelo
- T038 a T045 (tests de US2) en paralelo
- T057 a T062 (tests de US3) en paralelo
- T073 a T080 (tests de US4) en paralelo
- T098 a T101 (documentación) en paralelo

---

## Ejemplo de paralelismo: Historia 1

```bash
# Los nueve tests de US1, todos en archivos o funciones distintas:
Tarea: "Test de cobertura total en domain/consensus_coverage_test.go"
Tarea: "Test de unicidad en domain/consensus_coverage_test.go"
Tarea: "Test de ronda cruzada en domain/consensus_coverage_test.go"
Tarea: "Test de severidad derivada en domain/consensus_coverage_test.go"
Tarea: "Test de no aprobación sin clasificar en domain/verdict_test.go"
Tarea: "Test de idempotencia en application/usecases/build_consensus_test.go"
Tarea: "Test de reemplazo rechazado en application/usecases/build_consensus_test.go"
Tarea: "Test de identidad de revisor en application/usecases/submit_reviewer_result_test.go"
Tarea: "Test de campos obligatorios en application/usecases/submit_reviewer_result_test.go"
```

---

## Estrategia de implementación

### MVP primero (solo Historia 1)

1. Completar la Fase 1: preparación
2. Completar la Fase 2: fundacional (CRÍTICO — bloquea todo)
3. Completar la Fase 3: Historia 1
4. **PARAR Y VALIDAR**: escenarios 1, 2 y 3 de `quickstart.md`
5. La Historia 1 sola ya cierra el defecto HIGH que motivó esta funcionalidad: una revisión con un hallazgo grave omitido deja de poder aprobarse

### Entrega incremental

1. Preparación + Fundacional → base lista
2. + US1 → ninguna aprobación falsa (MVP)
3. + US2 → la corrección es trazable y no se pierde por concurrencia
4. + US3 → el protocolo siempre termina; la política del proyecto por fin se aplica
5. + US4 → auditoría completa y contrato cumplido
6. + US5 → los hallazgos originales cerrados y validados por una revisión nueva

### Estrategia con varias personas

Tras la Fase 2, US1 y US2 pueden ir en paralelo con personas distintas. US3 y US4 tocan
`domain/verdict.go` y `cmd_mcp_review_tools.go` junto a US1 y US2: conviene arrancarlas
cuando US1 haya integrado su cambio en `DeriveVerdict`.

---

## Notas

- `[P]` = archivos distintos, sin dependencias pendientes
- La etiqueta de historia da trazabilidad de la tarea a su historia de usuario
- Cada historia debe poder completarse y probarse por separado
- Verificar que el test falla ANTES de implementar
- Un commit por tarea o por grupo lógico
- Los hallazgos históricos **nunca** se borran: "quitar los hallazgos" significa
  resolver su causa, registrar la corrección y superar el re-juicio (FR-028)


---

## Registro de ejecución (T102–T103)

**T102 — validación integral.** `go vet ./...` limpio. `go test ./...` en verde: 13
paquetes `ok`, 0 `FAIL`. `gofmt -l .` **idéntico a la línea base** de T002 — los mismos
13 archivos preexistentes, ninguno nuevo. Durante el trabajo un `gofmt -w` sobre
`application/usecases/` reformateó dos archivos ajenos a la funcionalidad
(`import_adrs_test.go`, `index_project.go`); se revirtieron con `git checkout`, porque
no forman parte de este alcance.

**T103 — escenarios de quickstart.md.** Qué se ejecutó y cómo:

| Escenario | Cómo se verificó | Resultado |
|---|---|---|
| 0 — migración sobre base real | Copia de `mem.db` real, `mem review history` sobre ella | Migra, columnas nuevas con `notnull=0`, tabla `rejudgments` creada, `reviews=2 findings=15 consensus=13` intactos |
| 1, 2, 3 — consenso parcial, severidad, idempotencia | `TestBuildConsensus_*` + `domain/consensus_coverage_test.go` | Verde |
| 4 — un revisor no resuelve | `TestRejudgeReview_UnSoloRevisorNoResuelve` | Verde |
| 5 — cadena de targets | `TestRecordFix_ExigeElTargetVigente` | Verde |
| 6 — 100 correcciones concurrentes | `TestRecordFixConcurrenteConservaUnaSolaRonda` contra SQLite real, 5 ejecuciones seguidas | Una sola ronda gana, siempre |
| 7 — solo lectura escala | **Sobre copia de la base real**, revisión `acr_96710834` | `verdict=ESCALATED status=escalated` en una sola llamada. Con `fix_authorized=1`: `review is not ready to finalize` y sigue en `consensus_ready` — el contraste que hace válido el caso |
| 8 — estado terminal inmutable | `Test*_RechazaRevisionTerminal` en los cuatro casos de uso | Verde |
| 9 — métricas del contrato | `TestReviewMetricsDTO_CoincideConElContrato` (serialización real) y sobre la base real: `duration=17768 total=13 confirmed=2 suspect=11 contradictions=0 fix_rounds=0` | Las ocho claves exactas en `snake_case` |
| 10 — target pendiente | Repositorio git temporal con preparados, sin preparar, archivo con espacios y `.gitignore`; y `TestReviewCLI_Pending*` | `target_files: 3`, digest reproducible, cambia al modificar y al borrar, ignora lo de `.gitignore`; árbol limpio → error y salida 1 |
| 11 — linaje en una consulta | `TestReviewStatusExponeElLinaje` + salida ampliada de `mem review show` | Verde |
| 12 — promoción exige aprobada | `TestPromoteReviewMemory_ExigeVeredictoAprobado` | Verde |
| 13 — política del proyecto | `TestStartReview_AplicaLaPoliticaDelProyecto` y `TestSettingsData_ConservaLaPoliticaDeRevision` | Verde |
| 14 — 1.000 hallazgos en <2 s | `TestReviewStatusYFinalizeConMilHallazgos` | 0,14 s |
| 15 — cierre real de la revisión original | **NO ejecutado** | Ver la nota de la Fase 7 |

Todas las verificaciones sobre datos reales se hicieron sobre **copias**
(`/tmp/mem-antes-028.db`). La base del proyecto no se ha modificado.