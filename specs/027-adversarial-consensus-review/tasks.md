---

description: "Tareas de implementación — Revisión Adversarial por Consenso"
---

# Tasks: Revisión Adversarial por Consenso

**Input**: Documentos de diseño en `/specs/027-adversarial-consensus-review/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: OBLIGATORIOS. No es una elección de esta feature: el Principio III de la constitución
(«Testing First — NO NEGOCIABLE») exige escribir el test primero, verlo fallar y solo entonces
implementar. Además, la sección 42 de la entrada original enumera la cobertura mínima exigida. Cada
tarea de implementación va precedida por su tarea de test. Los tests existentes son intocables.

**Principio rector (§44 de la entrada, [research.md](./research.md) D1)**: GoMemory **no ejecuta
modelos ni juzga equivalencia semántica**. El agente orquestador propone; GoMemory valida y persiste.
Ninguna tarea de esta lista debe añadir a GoMemory una llamada a un proveedor de IA. Toda invariante
(INV-001..INV-012) se cierra con código verificable, nunca con texto de prompt.

**Puerta de checkpoint (añadida en revisión, 2026-08-29)**: ninguna fase se marca completa con
`go test ./...` en rojo, aunque los tests propios de sus tareas pasen. La Fase 3 llegó a tener sus 18
tareas en `[X]` con la suite roja: registrar cinco tools MCP rompió dos contratos que ningún test de
la fase ejercía (el plugin de OpenCode y el recuento de esquemas publicados). Los tests de una tarea
cubren lo que esa tarea construye; solo la suite entera cubre lo que esa tarea **rompe**.

**Organization**: agrupadas por historia de usuario, para poder implementar, probar y entregar cada
una por separado.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizable (archivos distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1..US4)
- Cada tarea lleva su ruta de archivo exacta

## Path Conventions

Arquitectura hexagonal ya vigente en el repositorio: `domain/`, `application/{ports,usecases}/`,
`adapters/primary/{cli,setup,tui}/`, `adapters/secondary/persistence/`, `infrastructure/`,
`tests/{contract,integration}/`. Los tests unitarios van junto a su paquete (`{paquete}_test.go`).

---

## Phase 1: Setup — verificar los supuestos contra el código real (BLOQUEANTE)

**Purpose**: el plan se apoya en tres mecanismos que **ya existen** y que aquí se dan por buenos
(dedup por `topic_key`, `addColumnIfMissing`, puntos de extensión de MCP/CLI). La regla de campo n.º 2
del proyecto dice que un supuesto no verificado contra el sistema en ejecución miente con cara de
éxito. Se comprueban antes de escribir una línea de código nuevo.

- [X] T001 Verificar en vivo el dedup por `topic_key` que asume [research.md](./research.md) D4: sobre una BD temporal, guardar dos memorias equivalentes con el mismo `topic_key` vía el binario compilado y confirmar que queda **una sola fila**; anotar el resultado en la sección «Estado de la evidencia» de `specs/027-adversarial-consensus-review/research.md`
- [X] T002 Verificar que `addColumnIfMissing` agrega una columna nullable sobre una BD **preexistente** con datos sin pérdida ni error, usando `adapters/secondary/persistence/db.go` contra una copia de `mem.db`; anotar el resultado en `specs/027-adversarial-consensus-review/research.md`
- [X] T003 Confirmar y anotar en `specs/027-adversarial-consensus-review/research.md` los tres puntos de extensión exactos: `registerTools` en `adapters/primary/cli/cmd_mcp.go:118`, el `switch` de `adapters/primary/cli/dispatcher.go`, y el patrón de instalación de skills de `adapters/primary/setup/constitution_setup.go` (`.claude/skills/<nombre>/SKILL.md` desde `infrastructure/templates/`)

**⚠️ Puerta de decisión**: si T001 resulta negativa, **detenerse y replantear la Historia 3** — la
promoción de memoria dejaría de ser gratis y necesitaría dedup propio, lo que cambia su alcance. Si
T002 resulta negativa, la columna `source_review_id` debe sustituirse por una tabla de enlace aparte.

**Checkpoint**: supuestos confirmados; ninguna historia construye sobre aire.

---

## Phase 2: Foundational (Prerequisitos bloqueantes)

**Purpose**: tipos de dominio puros, puertos, esquema y adaptador de persistencia que **todas** las
historias necesitan.

**⚠️ CRITICAL**: ninguna historia de usuario puede empezar hasta cerrar esta fase.

- [X] T004 [P] Escribir el test de los tipos de revisión en `domain/review_test.go`: `Target` con digest vacío se rechaza, digest congelado es inmutable, y los valores de `ReviewStatus` cubren las transiciones de [data-model.md](./data-model.md)
- [X] T005 [P] Escribir el test de los tipos de hallazgo en `domain/finding_test.go`: solo las cinco severidades y las siete clases de evidencia del spec son válidas, y un `Finding` con `Evidence` vacío queda marcado como no confirmable (FR-013)
- [X] T006 Implementar `Target`, `Review` y `ReviewStatus` en `domain/review.go` hasta que pase T004
- [X] T007 [P] Implementar `Finding`, `Severity` y `EvidenceClass` en `domain/finding.go` hasta que pase T005
- [X] T008 [P] Implementar `ConsensusFinding` y `ConsensusStatus` (CONFIRMED/SUSPECT/CONTRADICTION/INFO) en `domain/consensus.go`
- [X] T009 [P] Implementar los tipos `Verdict` (APPROVED/ESCALATED/INCOMPLETE) y `ReJudgmentState` (RESOLVED/UNRESOLVED/REGRESSED) en `domain/verdict.go` — solo los tipos; `DeriveVerdict` llega en T029
- [X] T010 [P] Implementar `FixDelta` y `ReviewerResult` en `domain/fix.go`
- [X] T011 Escribir el test del esquema en `adapters/secondary/persistence/review_test.go`: `migrate()` crea las cinco tablas de [data-model.md](./data-model.md) y una BD previa con memorias se abre sin pérdida
- [X] T012 Añadir las tablas `reviews`, `reviewer_results`, `findings`, `consensus_findings` y `fix_rounds` con sus índices al `migrate()` de `adapters/secondary/persistence/db.go`, en el mismo bloque aditivo `CREATE TABLE IF NOT EXISTS` existente
- [X] T013 Añadir la columna aditiva `memories.source_review_id` vía `addColumnIfMissing` en `adapters/secondary/persistence/db.go`
- [X] T014 [P] Declarar el puerto `ReviewRepository` (revisión, target, resultados de revisor, hallazgos) en `application/ports/review_repository.go`
- [X] T015 [P] Declarar el puerto `ConsensusRepository` (hallazgos de consenso, rondas de corrección) en `application/ports/consensus_repository.go`
- [X] T016 Escribir el test del adaptador en `adapters/secondary/persistence/review_test.go`: alta y lectura de cada entidad, y **reenvío idempotente** del mismo `(review, reviewer, round, local_id)` que actualiza en vez de duplicar (FR-039)
- [X] T017 Implementar los dos puertos sobre SQLite en `adapters/secondary/persistence/review.go` con parámetros bind y los índices `UNIQUE` como garantía de idempotencia, hasta que pase T016
- [X] T018 Registrar el repositorio nuevo en el composition root `infrastructure/container.go`

**Checkpoint**: dominio, esquema y persistencia listos — las historias pueden empezar.

---

## Phase 3: User Story 1 - Validar un cambio antes de darlo por terminado (Priority: P1) 🎯 MVP

**Goal**: congelar un target, recibir dos evaluaciones de revisor independientes, clasificarlas por
consenso y derivar un veredicto respaldado por el ledger.

**Independent Test**: iniciar una revisión sobre un diff con un defecto deliberado, enviar los dos
resultados de revisor y obtener un `CONFIRMED` que referencia ambas fuentes, o `INCOMPLETE` si uno
falla — sin que exista todavía ninguna capacidad de corrección.

### Tests de la Historia 1 ⚠️

> Escribir estos tests PRIMERO y verlos fallar antes de implementar.

- [X] T019 [P] [US1] Escribir el test de congelamiento en `application/usecases/start_review_test.go`: digest vacío se rechaza sin crear fila, el target queda inmutable, y mismo proveedor+modelo en A y B produce `independence.level = degraded` con motivo `same-model` (FR-009)
- [X] T020 [P] [US1] Escribir el test de envío en `application/usecases/submit_reviewer_result_test.go`: un `target_digest` distinto al congelado se rechaza con «target changed» (AC-003), el reenvío del mismo hallazgo es idempotente (FR-039) y `status: failure` bloquea el camino a APPROVED (FR-030)
- [X] T021 [P] [US1] Escribir el test de consenso en `application/usecases/build_consensus_test.go`: dos hallazgos equivalentes de revisores distintos producen **un** `CONFIRMED` con ambas fuentes (AC-001), un hallazgo único queda `SUSPECT` (AC-002), un par cuyas fuentes vienen del mismo revisor se **rechaza** (INV-004), y un hallazgo sin evidencia no puede quedar `CONFIRMED` (FR-013)
- [X] T022 [P] [US1] Escribir el test de derivación de veredicto en `domain/verdict_test.go` como tabla: sin `CONFIRMED` CRITICAL/HIGH ⇒ `APPROVED`; con un resultado de revisor en fallo ⇒ `INCOMPLETE` y nunca `APPROVED` (AC-007, INV-010)
- [X] T023 [P] [US1] Escribir el test de finalización en `application/usecases/finalize_review_test.go`: el veredicto se calcula del estado persistido y **no se acepta como parámetro de entrada**, y un `APPROVED` no dispara ninguna operación de entrega (AC-010, INV-011)
- [X] T024 [P] [US1] Escribir el test de contrato de las tools MCP en `tests/contract/review_protocol_test.go` según [contracts/mcp-tools.md](./contracts/mcp-tools.md): nombres, parámetros y ausencia de un parámetro `verdict` de entrada en `review_finalize`
- [X] T025 [P] [US1] Escribir el test de integración del flujo aprobado en `tests/integration/review_approved_flow_test.go` siguiendo el Escenario 1 de [quickstart.md](./quickstart.md) — permanece en rojo hasta cerrar la fase

### Implementación de la Historia 1

- [X] T026 [US1] Implementar el congelamiento del target y el cálculo del nivel de independencia en `application/usecases/start_review.go` hasta que pase T019
- [X] T027 [US1] Implementar el envío validado e idempotente de resultados de revisor en `application/usecases/submit_reviewer_result.go` hasta que pase T020
- [X] T028 [US1] Implementar la validación estructural y la persistencia de la clasificación de consenso en `application/usecases/build_consensus.go` hasta que pase T021 — sin comparación textual ni semántica propia ([research.md](./research.md) D1)
- [X] T029 [US1] Implementar `DeriveVerdict` como función pura (ramas APPROVED e INCOMPLETE) en `domain/verdict.go` hasta que pase T022
- [X] T030 [US1] Implementar la finalización sobre el estado persistido en `application/usecases/finalize_review.go` hasta que pase T023
- [X] T031 [US1] Registrar la tool `review_start` en `adapters/primary/cli/cmd_mcp_review_tools.go` según [contracts/mcp-tools.md](./contracts/mcp-tools.md)
- [X] T032 [US1] Registrar las tools `review_submit` y `review_consensus` en `adapters/primary/cli/cmd_mcp_review_tools.go`
- [X] T033 [US1] Registrar las tools `review_status` y `review_finalize` en `adapters/primary/cli/cmd_mcp_review_tools.go`
- [X] T034 [US1] Llamar a `registerReviewTools` desde `registerTools` en `adapters/primary/cli/cmd_mcp.go` hasta que pase T024
- [X] T035 [US1] Implementar el inicio de revisión por CLI (`--diff`, `--commit`, `--file`) con resolución local del digest en `adapters/primary/cli/cmd_review.go`, según [contracts/cli-contracts.md](./contracts/cli-contracts.md)
- [X] T036 [US1] Añadir el caso `review` al `switch` de `adapters/primary/cli/dispatcher.go` hasta que pase T025

### Propagación de las tools nuevas (añadidas en revisión, 2026-08-29)

> Estas dos tareas **no estaban en el plan original y debieron estarlo**. Se
> descubrieron al revisar la fase: T019–T036 figuraban completas con la suite en
> ROJO. Registrar una tool MCP no termina en su archivo de registro — hay dos
> consumidores más que el repositorio ya protege con tests de contrato.

- [X] T075 [US1] Declarar las cinco tools de revisión con su prefijo real de OpenCode (`gomemory_review_*`) en `infrastructure/plugin/opencode/gomemory.ts` y describirlas en su bloque de protocolo, hasta que pase `TestOpenCodeProtocolNombraTodasLasToolsPrefijadas`. **No es cosmético**: sin el prefijo declarado, un agente de OpenCode intenta invocar una tool inexistente
- [X] T076 [US1] Actualizar el recuento de operaciones publicadas de 19 a 24 en `adapters/primary/cli/cmd_mcp_schemas_test.go`, con el desglose por archivo de origen en el comentario, hasta que pase `TestMeasurePublishedSchemas_CountsRealServer`

**Checkpoint**: la Historia 1 detecta y clasifica defectos de forma independiente y emite veredicto,
sin capacidad de corregir nada — funcional y demostrable por sí sola (MVP).
**Suite completa en verde** (verificado 2026-08-29 tras T075/T076).

---

## Phase 4: User Story 2 - Corregir solo lo confirmado y volver a verificar (Priority: P2)

**Goal**: autorizar correcciones únicamente sobre hallazgos confirmados de severidad admitida,
registrar el cambio exacto, revalidarlo con una ronda acotada y escalar al agotar el presupuesto.

**Independent Test**: tomar un `CONFIRMED` HIGH de la Historia 1, registrar una corrección, ejecutar
la re-revisión y ver el hallazgo pasar a `RESOLVED`; con dos correcciones insuficientes, obtener
`ESCALATED` sin que se añada una tercera ronda.

### Tests de la Historia 2 ⚠️

- [X] T037 [P] [US2] Escribir el test de autorización de corrección en `application/usecases/record_fix_test.go`: un `SUSPECT` se rechaza (INV-005), una severidad fuera de `auto_fix_severities` se rechaza sin `explicit_authorization` (FR-019), un fix sin hallazgo confirmado se rechaza (INV-006) y una ronda por encima de `max_fix_rounds` se rechaza (INV-009)
- [X] T038 [P] [US2] Escribir el test de re-revisión en `application/usecases/rejudge_review_test.go`: cada hallazgo confirmado termina en `RESOLVED`, `UNRESOLVED` o `REGRESSED`, y la ronda referencia el hallazgo original y el fix delta (INV-008, AC-005)
- [X] T039 [P] [US2] Ampliar la tabla de `domain/verdict_test.go` con las ramas `ESCALATED`: presupuesto de rondas agotado con hallazgo sin resolver (AC-006) y contradicción severa sin resolver
- [X] T040 [P] [US2] Escribir el test de la política configurable en `adapters/secondary/persistence/settings_test.go`: sin configuración, `max_fix_rounds = 2` y `auto_fix_severities = [CRITICAL, HIGH]`; con configuración, se respetan los valores del proyecto
- [X] T041 [P] [US2] Escribir el test de integración del flujo escalado en `tests/integration/review_escalated_flow_test.go` siguiendo el Escenario 2 de [quickstart.md](./quickstart.md)

### Implementación de la Historia 2

- [X] T042 [US2] Añadir la política de revisión (`max_fix_rounds`, `auto_fix_severities`) con sus defaults a `adapters/secondary/persistence/settings.go` hasta que pase T040
- [X] T043 [US2] Implementar la elegibilidad de corrección como función pura en `domain/fix.go` (confirmado + severidad admitida + ronda dentro de presupuesto)
- [X] T044 [US2] Implementar el registro validado del fix delta en `application/usecases/record_fix.go` hasta que pase T037
- [X] T045 [US2] Implementar la ronda de re-revisión acotada en `application/usecases/rejudge_review.go` hasta que pase T038
- [X] T046 [US2] Ampliar `DeriveVerdict` en `domain/verdict.go` con las ramas `ESCALATED` hasta que pase T039
- [X] T047 [US2] Integrar el estado de re-revisión y el conteo de rondas en `application/usecases/finalize_review.go`
- [X] T048 [US2] Registrar la tool `review_fix_record` en `adapters/primary/cli/cmd_mcp_review_tools.go` según [contracts/mcp-tools.md](./contracts/mcp-tools.md)
- [X] T049 [US2] Extender `application/usecases/submit_reviewer_result.go` para aceptar envíos de rondas posteriores (`round > 0`) manteniendo la idempotencia por `(review, reviewer, round)`
- [X] T050 [US2] Extender `application/usecases/build_consensus.go` para registrar el `rejudgment_state` de cada hallazgo confirmado hasta que pase T041

### Hallazgos de la Fase 4 (2026-08-29)

- **T047 destapó un defecto de US1 que aprobaba revisiones con su fallo intacto.**
  `FinalizeReview` listaba los hallazgos de consenso filtrando por `review.Round`, pero un hallazgo
  nace en la ronda del consenso y su resolución llega en una posterior: tras la primera corrección no
  veía ninguno y devolvía `APPROVED` con el defecto severo sin resolver. Se añadió
  `ListAllConsensusFindings` al puerto y al adaptador. El primer test de re-revisión pasaba **por el
  motivo equivocado**; lo que lo destapó fue añadir su contraste (`SinResolverNoAprueba`), no el
  camino feliz.
- **T049 no requirió código**: `SubmitReviewerResult` ya opera sobre `review.Round`, así que admite
  rondas posteriores por diseño. Lo que faltaba era quién avanza la ronda, y eso es `RecordFix`.
- **T050 se resolvió en `rejudge_review.go`, no en `build_consensus.go`.** El estado de re-revisión
  lo escribe quien revalida, no quien construye el consenso; ponerlo en el segundo habría mezclado
  dos momentos distintos del protocolo.
- **Superficie MCP**: además de `review_fix_record` (T048) se añadió `review_rejudge`. La
  revalidación es un paso propio del protocolo con reglas propias, y colgarla de `review_consensus`
  habría dado a esa tool dos significados según el momento.

**Checkpoint**: Historias 1 y 2 funcionan de forma independiente; el ciclo detectar → corregir →
reverificar → veredicto está cerrado y acotado.
**Suite completa en verde** (verificado 2026-08-29, incluye el flujo escalado contra SQLite real).

---

## Phase 5: User Story 3 - Reutilizar el conocimiento de revisiones pasadas (Priority: P3)

**Goal**: convertir un defecto confirmado y resuelto en conocimiento reutilizable, deduplicado y
recuperable por el mecanismo normal de contexto.

**Independent Test**: aprobar una revisión con un defecto resuelto y comprobar que aparece una
memoria `review_learning` con problema/causa raíz/resolución/verificación, sin transcript ni
razonamiento, recuperable desde `get_context()` en una sesión nueva.

### Tests de la Historia 3 ⚠️

- [X] T051 [P] [US3] Escribir el test de promoción en `application/usecases/promote_review_memory_test.go`: solo `CONFIRMED + RESOLVED` se promueve, la memoria contiene problema, causa raíz, resolución y verificación, y **no** contiene transcript ni cadena de razonamiento (AC-008, FR-031)
- [X] T052 [P] [US3] Ampliar `application/usecases/promote_review_memory_test.go` con el caso de deduplicación: dos revisiones del mismo patrón de fallo actualizan la memoria existente vía `topic_key` en vez de crear una segunda (AC-009, FR-034)
- [X] T053 [P] [US3] Escribir el test de integración de recuperación en `tests/integration/review_memory_context_test.go`: tras promover, el aprendizaje aparece en la salida de `get_context()` sin consulta especial al historial de revisiones (FR-035)

### Implementación de la Historia 3

- [X] T054 [US3] Implementar el constructor de la memoria `review_learning` (estructura y `topic_key` derivado de categoría+componente) en `domain/review.go`, aplicando el helper de redacción existente
- [X] T055 [US3] Implementar la extracción y promoción reutilizando `SaveMemory` y su dedup por `topic_key` en `application/usecases/promote_review_memory.go` hasta que pasen T051 y T052
- [X] T056 [US3] Persistir `source_review_id` al insertar la memoria promovida en `adapters/secondary/persistence/memory.go` (trazabilidad de linaje, FR-038)
- [X] T057 [US3] Disparar la promoción desde `application/usecases/finalize_review.go` al alcanzar un veredicto `APPROVED`, y opcionalmente en `ESCALATED` de alto valor (FR-033)
- [X] T058 [US3] Incluir las memorias `review_learning` en el contexto normal del proyecto en `application/usecases/build_context.go` hasta que pase T053

### Hallazgos de la Fase 5 (2026-08-29)

- **T057 no podía cumplirse tal como estaba redactada.** «Disparar la promoción desde finalize» exige
  que gomemory redacte el aprendizaje, y no puede: el problema, la causa raíz y la resolución los
  escribe quien revisó. Acoplar ambas cosas habría dado a `finalize` dos responsabilidades y habría
  desperdiciado la entrada cuando el veredicto sale `ESCALATED`. Resuelto separando la decisión del
  contenido: `domain.PromotableFindings` dice **cuáles** tienen derecho a promoverse —regla del
  sistema, no del prompt— y `review_finalize` lo informa; promover sigue siendo un acto aparte
  (`review_promote_memory`).
- **T058 no requirió código, y eso era la prueba de la decisión de diseño.** `build_context` ya emite
  las memorias de tipo `Learning`, así que el aprendizaje reaparece por `get_context` sin que la ruta
  de contexto sepa que existen las revisiones. No suponerlo fue lo importante: T053 lo verifica
  contra SQLite real.
- **La exclusión de transcripts es estructural, no una comprobación.** `ReviewLearning` no tiene
  campo donde quepa una cadena de razonamiento (FR-031, AC-008), así que no depende de que quien
  promueve se acuerde de filtrar.
- **`ReviewMemoryWriter`**: la promoción declara su propio puerto de una sola operación en vez de
  pedir las doce de `ports.MemoryRepository`. No lee, no borra, no busca — y su firma lo dice.

**Checkpoint**: las tres historias funcionan de forma independiente; las revisiones pasadas se
convierten en conocimiento preventivo.
**Suite completa en verde** (verificado 2026-08-29, incluye la recuperación por contexto contra SQLite real).

---

## Phase 6: User Story 4 - Consultar el estado y el historial de una revisión (Priority: P4)

**Goal**: auditar qué se revisó, qué se detectó, qué corrección se intentó y cuál fue el resultado,
sin acceder al almacenamiento interno.

**Independent Test**: consultar el estado de una revisión abierta, listar el historial con sus
veredictos y ver el linaje completo de una revisión concreta.

### Tests de la Historia 4 ⚠️

- [X] T059 [P] [US4] Escribir el test de contrato de los subcomandos en `tests/contract/review_cli_test.go` según [contracts/cli-contracts.md](./contracts/cli-contracts.md): `status` muestra etapa y no veredicto en una revisión abierta, `history` lista con veredicto, `show` reconstruye el linaje, y un `review-id` inexistente es error y no una revisión vacía

### Implementación de la Historia 4

- [X] T060 [US4] Añadir las consultas de listado y de detalle con linaje a `adapters/secondary/persistence/review.go` y a los puertos de `application/ports/review_repository.go`
- [X] T061 [US4] Implementar `mem review status [<review-id>]` en `adapters/primary/cli/cmd_review.go`
- [X] T062 [US4] Implementar `mem review history [--limit N]` en `adapters/primary/cli/cmd_review.go`
- [X] T063 [US4] Implementar `mem review show <review-id>` con el linaje completo de [data-model.md](./data-model.md) en `adapters/primary/cli/cmd_review.go`
- [X] T064 [US4] Enrutar los tres subcomandos desde `CmdReview` en `adapters/primary/cli/cmd_review.go` hasta que pase T059

**Checkpoint**: las cuatro historias son funcionales e independientes.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: observabilidad, seguridad, distribución de la skill y documentación — transversales a
todas las historias.

> **Adelanto (2026-08-29)**: T069 y T070 se entregaron **antes** que el resto de fases, y con razón:
> la guía es autosuficiente —no depende de ninguna tool de gomemory ni de que exista memoria
> persistente—, así que aporta el protocolo desde ya sin prometer nada que el binario no pueda
> cumplir. Lo que las fases 2–6 añaden después es hacer sus invariantes **incumplibles** en vez de
> meramente enunciadas. T071 (independencia del prompt) sigue pendiente: hoy las invariantes de la
> guía son solo texto, que es exactamente lo que advierten el §44 de la entrada y FR-045.

- [X] T065 [P] Escribir el test de métricas en `application/usecases/finalize_review_test.go`: la finalización emite duración, hallazgos totales/confirmados/sospechosos, contradicciones, rondas y veredicto (FR-042)
- [X] T066 Emitir las métricas del protocolo reutilizando el registrador de uso existente en `application/usecases/finalize_review.go` hasta que pase T065
- [X] T067 [P] Escribir el test de redacción en `adapters/secondary/persistence/review_test.go`: un secreto en un campo de texto libre (`claim`, `verification`) no se persiste en claro (FR-041, FR-031)
- [X] T068 Aplicar el helper de redacción existente a los campos de texto libre en `adapters/secondary/persistence/review.go` hasta que pase T067
- [X] T069 [P] Incorporar la guía de participación agnóstica de proveedor a `infrastructure/templates/adversarial-consensus-review/SKILL.md` — FR-044, FR-045. **Ya existía** (1426 líneas, 47 secciones, Apache-2.0, `portability: vendor-neutral`, sin acoplamiento a gomemory): se importó verbatim desde la copia del usuario en vez de redactarla. No lleva `references/`: es un documento único y autosuficiente
- [X] T070 Instalar la guía en el ámbito de usuario de los tres agentes con directorio de habilidades (`~/.claude`, `~/.codex`, `~/.opencode`) en `adapters/primary/setup/adversarial_review_setup.go`, invocada desde `adapters/primary/cli/cmd_mcp_setup.go`, con tests de cobertura, no-invasión, idempotencia, actualización de versión previa, degradación sin plantilla y agnosticismo (FR-044)
- [X] T071 Escribir el test de independencia del prompt en `tests/contract/review_protocol_test.go`: con la guía ausente, los intentos de exceder el presupuesto de rondas, corregir un hallazgo no confirmado o declarar un veredicto por parámetro siguen siendo rechazados (SC-008, §44 de la entrada)
- [X] T072 [P] Documentar `mem review` y el protocolo en `docs/MANUAL.md` y en la sección correspondiente de `README.md`, en español
- [X] T073 Ejecutar los cinco escenarios de [quickstart.md](./quickstart.md) contra el binario recién compilado (no solo los tests) y anotar el resultado en `tasks/todo.md`
- [X] T074 Dejar `golangci-lint run` y `go test ./...` en verde antes de dar la feature por terminada

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias — empieza de inmediato y **bloquea todo lo demás** (sus puertas de decisión pueden cambiar el alcance de la Historia 3)
- **Foundational (Phase 2)**: depende de Phase 1 — **BLOQUEA todas las historias**
- **US1 (Phase 3)**: depende de Phase 2
- **US2 (Phase 4)**: depende de Phase 3 — extiende `DeriveVerdict`, `submit_reviewer_result`, `build_consensus` y `finalize_review` creados allí
- **US3 (Phase 5)**: depende de Phase 4 — la promoción exige un hallazgo confirmado **y resuelto**
- **US4 (Phase 6)**: depende de Phase 2; el `show` completo gana valor tras Phase 4, pero es testeable antes
- **Polish (Phase 7)**: depende de las historias que se decidan entregar

### Dependencias entre historias

Esta feature es un protocolo secuencial, así que las historias **no** son mutuamente independientes
en implementación (sí en prueba y demostración):

- **US1 (P1)**: independiente tras Foundational — es el MVP real
- **US2 (P2)**: necesita el consenso de US1 para tener qué corregir
- **US3 (P3)**: necesita la resolución de US2 para tener qué promover
- **US4 (P4)**: solo necesita el repositorio de Foundational; puede adelantarse si conviene

### Dentro de cada historia

- El test se escribe y **falla** antes de la implementación
- Dominio antes que casos de uso; casos de uso antes que adaptadores (MCP/CLI)
- El test de integración de la historia cierra la fase

### Parallel Opportunities

- T004–T005 en paralelo; luego T007–T010 en paralelo (archivos de dominio distintos)
- T014–T015 en paralelo (puertos distintos)
- T019–T025 en paralelo (todos los tests de US1, archivos distintos)
- T037–T041 en paralelo (todos los tests de US2)
- T051–T053 en paralelo (todos los tests de US3)
- T065, T067, T069 y T072 en paralelo (métricas, redacción, guía de participación y documentación no se tocan entre sí)

---

## Parallel Example: Historia 1

```bash
# Lanzar juntos todos los tests de la Historia 1 (deben fallar):
go test ./application/usecases/ -run 'TestStartReview|TestSubmitReviewerResult|TestBuildConsensus|TestFinalizeReview'
go test ./domain/ -run TestDeriveVerdict
go test ./tests/contract/ -run TestReviewProtocol
go test ./tests/integration/ -run TestReviewApprovedFlow
```

---

## Implementation Strategy

### MVP primero (solo Historia 1)

1. Cerrar Phase 1: verificación de supuestos
2. Cerrar Phase 2: Foundational (CRÍTICO — bloquea todo)
3. Cerrar Phase 3: Historia 1
4. **DETENERSE Y VALIDAR**: probar la Historia 1 de forma independiente con el Escenario 1 de [quickstart.md](./quickstart.md), hasta el punto de consenso
5. En este punto ya existe valor real: dos revisores independientes sobre un target congelado, con hallazgos confirmados por corroboración y un veredicto — sin corrección automática

### Entrega incremental

1. Setup + Foundational → base lista
2. + Historia 1 → detectar y confirmar defectos (MVP)
3. + Historia 2 → corregir lo confirmado y reverificar, con presupuesto
4. + Historia 3 → convertir lo aprendido en conocimiento preventivo
5. + Historia 4 → auditoría e historial
6. + Polish → métricas, redacción, skill y documentación

---

## Notes

- `[P]` = archivos distintos, sin dependencias pendientes
- Ninguna tarea puede añadir a GoMemory una llamada a un proveedor de IA ([research.md](./research.md) D1)
- El veredicto nunca entra como parámetro: siempre se deriva del estado persistido (INV-010)
- El esquema se amplía **solo** de forma aditiva: sin `DROP`, sin `RENAME`, sin cambio de tipo
- Verificar que cada test falla antes de implementar
- Hacer commit tras cada tarea o grupo lógico
- Detenerse en cualquier checkpoint para validar la historia por separado

---

## Cierre de la feature (2026-08-29)

**76/76 tareas.** Suite completa en verde, `go vet` limpio, sin líneas sobre 120.

### Hallazgos de las Fases 6 y 7

- **La redacción de secretos NO existía en el ledger de revisión.** Las memorias
  se depuran al insertar desde hace varias features; las tablas `findings`,
  `consensus_findings` y `fix_rounds` guardaban `claim`, `evidence` y
  `verification` en claro. Un revisor CITA el código que analiza: si esa línea
  trae una credencial, la cita se persistía tal cual y luego se servía por
  `mem review show`. Reproducido con un token real en las cuatro columnas antes
  de arreglarlo (T067), y cerrado reutilizando el mismo helper que ya protege a
  las memorias (T068). El agravante era la duración: el ledger está pensado para
  quedarse.
- **Control de errores verificado sobre el binario, no sobre tests**: los cuatro
  caminos de fallo de la CLI (`show`/`status` con id inexistente, `--limit` no
  numérico, subcomando desconocido) salen con código 1 y mensaje accionable. Un
  identificador inexistente **nunca** devuelve una revisión vacía: quien
  consulta leería «sin hallazgos» y concluiría que no hay defectos.
- **`status` de una revisión abierta muestra etapa, no veredicto.** Confundir
  «va por la mitad» con «terminó sin defectos» es el error que la funcionalidad
  existe para impedir, así que es contrato, no detalle de presentación.
- **T065/T066**: `FinalizeReviewWithMetrics` deriva las métricas de lo
  persistido en el momento de finalizar, no las acumula por el camino — así no
  pueden desincronizarse del ledger. `FinalizeReview` conserva su firma para los
  llamadores que no las necesitan.
- **T071 es la tesis de la feature puesta a prueba**: sin ningún texto de prompt
  presente, los tres atajos posibles (corregir sin corroborar, exceder el
  presupuesto, declarar veredicto) siguen rechazados. Si ese test pasara con las
  invariantes solo en `SKILL.md`, pasaría también sin ellas.

### Deuda declarada, no cerrada

`TestOpenCodeProtocolNombraTodasLasToolsPrefijadas` comprueba que la cadena
prefijada **aparezca** en el plugin, no que la tool se describa en el bloque de
protocolo. Declarar la constante sin explicarla haría pasar el test dejando al
agente sin saber que existe. Cerrarlo exige definir qué cuenta como «descrita»,
y eso admite varias lecturas razonables.
