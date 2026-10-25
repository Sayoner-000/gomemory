# Tasks: Socrática → Spec Kit — preparación guiada y validada

**Input**: Artefactos de `specs/042-socratica-specify-preparation/`.
**Fecha**: 2026-10-24. **Rama de referencia**: `main`.
**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contratos](contracts/socratica-flow.md) y [quickstart.md](quickstart.md).

**Tests**: Incluidos porque la especificación exige verificar controles, recuperación, instalación, privacidad y recorridos reales. Escribir las pruebas específicas antes de implementar su comportamiento y confirmar el fallo esperado cuando falte la capacidad. Si la base ya proporciona la condición, conservar la evidencia verde sin introducir un fallo artificial.

**Organization**: US1–US4 tienen prioridad P1; US5–US6, P2. Todas las tareas están pendientes. Este archivo no acredita implementación ni aceptación ejecutada.

## Format: `[ID] [P?] [Story] Description`

- Cada tarea tiene checkbox pendiente, ID secuencial, ruta exacta y dependencias explícitas.
- `[P]` permite paralelismo solo cuando las dependencias estén completas y las tareas elegidas no modifiquen archivos comunes. No habilita fases bloqueadas.
- `[USn]` aparece únicamente en fases de historia. Las rutas son relativas a la raíz; los archivos nuevos se crean durante la implementación.
- `evidence/*.md` identifica entregables futuros con datos sintéticos, no evidencia ya obtenida.
- Reutilizar Go, SQLite, MCP, TUI y distribución existentes. No añadir dependencias ni otro motor de flujo.
- Mantener preparación privada efímera y reserva sin texto; la concesión de ejecución solo se devuelve una vez. `ConsumeDispatch` conserva su función de procedencia del hook.

## Phase 1: Setup — línea base y soporte compartido

**Objetivo**: Preparar evidencia y datos sintéticos sin inicializar otro proyecto.

- [X] T001 Registrar pruebas previas con `go test ./... -count=1`, versión del catálogo y paridad de skills en `specs/042-socratica-specify-preparation/evidence/baseline.md`; distinguir fallos previos antes de modificar producción.
- [X] T002 Crear soporte de proceso helper y proyectos temporales en `tests/integration/socratica_specify_fixture_test.go`, cargando `specs/042-socratica-specify-preparation/contracts/example-preparation.json` y contando invocaciones sin modelos reales. (dep: T001)

**Checkpoint**: Línea base y soporte sintético disponibles; ninguna capacidad nueva se declara implementada.

## Phase 2: Foundational — contrato y almacenamiento compartidos

**Objetivo**: Disponer de tipos, render y persistencia segura antes de exponer historias. Requiere Setup; bloquea las historias.

- [X] T003 Definir `ModeSpecify`, preparación opcional, etapas, comprobaciones, revisión, resultado y política inmutable de payload en `domain/socratica_specify.go` y `domain/socratica.go`; declarar el resultado interno de confirmación en `application/ports/socratica_repository.go` sin cambiar salidas públicas previas. (dep: T001, T002)
- [X] T004 [P] Añadir casos de base vacía, migración desde catálogo v2, rollback y preparación nula de flujos antiguos en `adapters/secondary/persistence/socratica_specify_migration_test.go`, incluyendo la expectativa del catálogo nuevo. (dep: T003)
- [X] T005 [P] Añadir pruebas de primera confirmación, mismo nonce, otro nonce, concurrencia y fallo de commit en `adapters/secondary/persistence/socratica_specify_ack_test.go`; solo la primera transición comprometida concede ejecución. (dep: T003)
- [X] T006 [P] Añadir pruebas de depósito privado, reserva SQL sin texto y publicación después del commit en `adapters/secondary/persistence/socratica_ephemeral_test.go`, verificando que rollback y pérdida de payload no conceden permisos. (dep: T003)
- [X] T007 Añadir `preparation_json` a flujos/historial y `payload_policy` con default persistente en `adapters/secondary/persistence/socratica_schema.go` y `adapters/secondary/persistence/db.go`; elevar catálogo 2→3 si 2 sigue vigente y actualizar `adapters/secondary/persistence/db_schema_golden_test.go` mediante el mecanismo aditivo transaccional existente. (dep: T004)
- [X] T008 Ampliar columnas explícitas, valores, lectores e historial en `adapters/secondary/persistence/socratica.go`, conservando `NULL` legado y revisión/CAS; probar round-trip y rechazo sin sobrescritura de versiones nuevas no soportadas en `adapters/secondary/persistence/socratica_specify_migration_test.go`. (dep: T007)
- [X] T009 Ampliar el resultado interno de `Ack` en `application/ports/socratica_repository.go` y `adapters/secondary/persistence/socratica.go`, adaptando consumidores/fakes en `application/usecases/socratica_flow.go` y `application/usecases/socratica_flow_test.go`; distinguir primera concesión con `RowsAffected==1`, sin reutilizar `ConsumeDispatch` ni alterar flags públicos de modos previos. (dep: T005, T008)
- [X] T010 Implementar el adaptador coordinado en `adapters/secondary/persistence/socratica_ephemeral.go`: payload solo en memoria, reserva durable sin texto, mutex y cambios publicables después del commit; mantener pendiente único, marcas de consumo y borrado sin historial privado. (dep: T006, T009)
- [X] T011 Conectar el adaptador en `infrastructure/container.go` y fijar la política al crear el flujo en `application/usecases/socratica_flow.go`; consultar la reserva común antes de crear pendientes y no cambiar de respaldo al alternar el ajuste privado. (dep: T010)
- [X] T012 [P] Definir pruebas de render repetible, preservación de IDs, protección previa, datos esenciales retirados y huella de bytes exactos en `domain/socratica_specify_render_test.go`, distinguiéndola de la huella normalizada de procedencia. (dep: T003)
- [X] T013 Implementar renderizador y huella exacta en `domain/socratica_specify_render.go`, con orden fijo de secciones, orden declarado de listas y textos protegidos; no introducir una reescritura posterior a la aprobación. (dep: T012)

**Checkpoint**: Tipos, render, migración, mapeo, concesión interna y almacenamiento compartidos comprobables. Las historias requieren completar T011 y T013.

## Phase 3: User Story 1 — preparar con ayuda del contexto (P1)

**Objetivo**: Obtener un borrador estructurado desde el agente sin generar una especificación.

**Independent Test**: Preparar solicitudes clara, ambigua, técnica y demasiado grande mediante MCP; conservar original y faltantes, hacer como máximo tres preguntas y no ejecutar el trabajo. Se puede demostrar sin completar el despacho.

### Tests

- [X] T014 [P] [US1] Añadir captura, preguntas acumuladas, respuestas/omisiones y retorno a preparación sin generación libre en `domain/socratica_specify_questions_test.go`, incluyendo edición/reanudación sin reiniciar el contador. (dep: T011, T013)
- [X] T015 [P] [US1] Añadir pruebas de inicio, reemplazo de sección con revisión esperada, no-op idéntico y recuperación del pendiente de otro modo en `application/usecases/socratica_specify_draft_test.go`, usando las cuatro clases de solicitud. (dep: T011, T013)
- [X] T016 [P] [US1] Añadir contrato de `mode=specify` y `prepare(set)` en `adapters/primary/cli/cmd_mcp_socratica_specify_draft_test.go`: proyecto impuesto por servidor, rechazo de campos calculados, revisión obsoleta, contexto recuperado con instrucciones incrustadas que no autoriza, no completa respuestas ni altera controles, y etapa/faltantes/acciones/next devueltos. (dep: T011, T013)

### Implementation

- [X] T017 [US1] Integrar captura y preguntas de `specify` en `domain/socratica.go` y `domain/socratica_specify.go`; acumular como máximo tres, volver a preparación tras responder y bloquear rutas genéricas de aprobación/despacho mientras no existan sus guardas específicas. (dep: T014)
- [X] T018 [US1] Implementar inicio y `prepare(set)` en `application/usecases/socratica_specify.go` y `application/usecases/socratica_flow.go`: protección antes de guardar/renderizar, revisión esperada y reemplazo completo dentro del UnitOfWork, sin generador externo. (dep: T015, T017)
- [X] T019 [US1] Exponer inicio, actualización de secciones y proyección inicial en `adapters/primary/cli/cmd_mcp_socratica_tool.go`, con siguiente paso calculado y rechazo de `rewrite`, edición textual libre o `send_original` en el nuevo modo. (dep: T016, T018)
- [X] T020 [US1] Añadir preparación guiada a `infrastructure/templates/socratica/SKILL.md` y sincronizar `skills/socratica/SKILL.md`: contexto antes de preguntar, actores técnicos válidos, entregas completas, omisiones pendientes y seguimiento de `next` sin ejecutar; sin herramienta disponible, declarar la limitación y no afirmar preparación validada ni iniciar el traspaso gestionado. (dep: T019)

**Checkpoint**: Preparación utilizable como primer incremento; aprobación y entrega siguen cerradas hasta sus historias. Las pruebas sintéticas comprueban controles; la calidad de conversación real se acepta en US6.

## Phase 4: User Story 2 — comprobar la preparación (P1)

**Objetivo**: Controlar condiciones de avance y mostrar bloqueos.

**Independent Test**: Con borradores preparados, introducir campos vacíos, IDs duplicados, referencias inválidas y pendientes bloqueantes; solo el borrador corregido alcanza la comprobación final. La aprobación permanece cerrada hasta US3.

### Tests

- [X] T021 [P] [US2] Añadir tablas de mínimos, listas declaradas vacías, espacios/marcadores, actores, referencias y bloqueos en `domain/socratica_specify_validation_test.go`; incluir eliminación o reclasificación silenciosa de un bloqueo. (dep: T020)
- [X] T022 [P] [US2] Añadir saltos de etapa, huellas obsoletas y omisiones que no resuelven bloqueos en `domain/socratica_specify_stages_test.go`, conservando el límite acumulativo tras tres preguntas. (dep: T020)
- [X] T023 [P] [US2] Añadir contrato de `prepare(validate)` en `adapters/primary/cli/cmd_mcp_socratica_specify_validation_test.go`, rechazando validez/huellas aportadas por el agente y sin confundir completitud con aprobación humana. (dep: T020)

### Implementation

- [X] T024 [US2] Implementar validadores puros y códigos de faltantes en `domain/socratica_specify_validation.go`, aplicando mínimos del esquema, cobertura de historias por criterios/entregas y pendientes, sin inventar decisiones de negocio. (dep: T021)
- [X] T025 [US2] Implementar avance `intent→scope→stories→validation` y huellas de sus entradas en `domain/socratica_specify.go`; un fallo conserva el borrador y no habilita etapas posteriores ni aprobación. (dep: T022, T024)
- [X] T026 [US2] Coordinar `prepare(validate)` y sus comprobaciones/acciones/next en `application/usecases/socratica_specify.go`, guardando comprobaciones con la transición y generando el render protegido mediante el componente compartido al completar la comprobación final. (dep: T025)
- [X] T027 [US2] Completar validación por etapas en `adapters/primary/cli/cmd_mcp_socratica_tool.go`, rechazando saltos y revisiones obsoletas, mostrando faltantes tras omitir/agotar preguntas y manteniendo cerrada la autorización sin revisión vinculada. (dep: T023, T026)

**Checkpoint**: La preparación distingue faltantes y validez estructural; no sustituye la decisión humana.

## Phase 5: User Story 3 — revisar y aprobar el texto exacto (P1)

**Objetivo**: Vincular la decisión humana a la revisión vigente y permitir editar, copiar o cancelar sin despacho.

**Independent Test**: Revisar un borrador comprobado, editar alcance, rechazar aprobación obsoleta y volver a comprobar/aprobar; comparar texto exacto y cancelar recuperando el original disponible. No necesita un generador real.

### Tests

- [X] T028 [P] [US3] Añadir invalidación por sección, retirada de revisión/autorización no consumida y cancelación con original protegido en `domain/socratica_specify_review_test.go`, rechazando editar un despacho reclamado o consumido. (dep: T027)
- [X] T029 [P] [US3] Añadir identidad exacta, revisión esperada, edición concurrente con aprobación y bypass en `application/usecases/socratica_specify_review_test.go`, incluidos cambios de contenido por sanitización. (dep: T027)
- [X] T030 [P] [US3] Añadir contrato de revisión, copia de solo lectura, aprobación explícita y rechazo de `rewrite/edit/send_original` en `adapters/primary/cli/cmd_mcp_socratica_specify_review_test.go`; ninguna consulta concede ejecución. (dep: T027)

### Implementation

- [X] T031 [US3] Vincular render a `{revision,prompt_hash}` e implementar invalidación/cancelación en `domain/socratica_specify.go` y `domain/socratica.go`; preservar la huella normalizada del hook y no reescribir después de aprobar. (dep: T028)
- [X] T032 [US3] Comprobar contenido, huellas y texto vigentes al aprobar en `application/usecases/socratica_specify.go` y `application/usecases/socratica_flow.go`, dentro del UnitOfWork; una autorización vieja no aprueba una versión nueva. (dep: T029, T031)
- [X] T033 [US3] Conectar revisión y `approve(expected_revision)` en `adapters/primary/cli/cmd_mcp_socratica_tool.go`, mostrando original/propuesta completos y preservando idempotencia sin conceder ejecución desde aprobar/get/resume. (dep: T030, T032)
- [X] T034 [US3] Actualizar revisión humana en `infrastructure/templates/socratica/SKILL.md` y `skills/socratica/SKILL.md`: explicar límites de validación, ofrecer acciones reales o texto seleccionable y exigir «Aprobar y generar especificación» para la versión mostrada. (dep: T033)

**Checkpoint**: Solo la versión mostrada puede aprobarse; editar obliga a comprobar y revisar; aprobar no ejecuta.

## Phase 6: User Story 4 — entregar una única vez y comunicar resultado (P1)

**Objetivo**: Entregar a Spec Kit el contenido aprobado tras comprobar disponibilidad, diferenciando concesión, consumo y resultado.

**Independent Test**: Con una preparación aprobada, usar un ejecutor sintético contado con generador disponible/ausente, replay del nonce, fallo y pérdida de respuesta; obtener como máximo una concesión y cero éxito declarado sin resultado. La generación real se demuestra en US6.

### Tests

- [X] T035 [P] [US4] Añadir primera concesión solo tras commit, replay, respuesta perdida, `in_flight/uncertain` y reconciliación explícita en `application/usecases/socratica_specify_dispatch_test.go`, sin reutilizar la marca del hook. (dep: T034)
- [X] T036 [P] [US4] Añadir pruebas de `report` completado/fallido/incierto, rutas relativas, cobertura de criterios, duplicados y corrección explícita en `application/usecases/socratica_specify_report_test.go`; reportar nunca rehabilita permisos. (dep: T034)
- [X] T037 [P] [US4] Añadir contrato de `execution_granted`, `ack` específico y `report` en `adapters/primary/cli/cmd_mcp_socratica_specify_dispatch_test.go`, incluyendo otro nonce, despacho obsoleto y aprobación repetida. (dep: T034)
- [X] T038 [P] [US4] Añadir verificaciones del orden preflight→ack→invocación, texto sin reescritura, replay sin permiso y compatibilidad de modos previos en `adapters/primary/setup/socratica_specify_skill_test.go`, contrastando la plantilla canónica. (dep: T034)
- [X] T039 [P] [US4] Añadir recorrido contado con proceso helper en `tests/integration/socratica_specify_dispatch_test.go`: falta de generador no consume, primera concesión entrega texto exacto, fallo/replay no duplican y el reporte no sustituye inspección del artefacto sintético. (dep: T034)

### Implementation

- [X] T040 [US4] Transportar la primera concesión hasta el resultado de aplicación en `application/usecases/socratica_specify.go` y `application/usecases/socratica_flow.go`; publicarla solo tras commit, conservar aprobación vigente al retomar y devolver next de consumo/reconciliación para replays. (dep: T035)
- [X] T041 [US4] Implementar resultado comunicado y transiciones en `domain/socratica_specify.go` y `application/usecases/socratica_specify.go`, verificando despacho, rutas sin escapes, trazabilidad y correcciones, sin ejecutar rutas ni atribuir inspección semántica al backend. (dep: T036, T040)
- [X] T042 [US4] Publicar `ack/report`, concesión y resultado para `specify` en `adapters/primary/cli/cmd_mcp_socratica_tool.go`, revalidando revisión antes de confirmar y conservando las respuestas públicas de `refine/scrutiny`. (dep: T037, T041)
- [X] T043 [US4] Completar traspaso en `infrastructure/templates/socratica/SKILL.md` y `skills/socratica/SKILL.md`: comprobar capacidad de cargar `speckit-specify` antes de ack, retener entrada si falta, invocar el texto exacto solo con concesión y comunicar archivos/alcance/trazabilidad inspeccionados. (dep: T038, T042)
- [X] T044 [US4] Verificar en `tests/integration/socratica_specify_dispatch_test.go` los casos A–C de `specs/042-socratica-specify-preparation/quickstart.md`, sin modelos externos y documentando límites del ejecutor sintético. (dep: T039, T043)

**Checkpoint**: Cada autorización concede como máximo una entrega; consumo no equivale a generación completada.

## Phase 7: User Story 5 — recuperar con privacidad y consultar localmente (P2)

**Objetivo**: Retomar estado válido y consultar desde CLI/TUI sin ejecutar la ruta local genérica ni filtrar payload privado.

**Independent Test**: Cerrar/reabrir un flujo persistente y comparar decisiones/relaciones/pendientes; consultar terminal/TUI sin generación; perder la instancia privada conserva solo reserva y rechaza operaciones que requieren contenido perdido.

### Tests

- [X] T045 [P] [US5] Añadir recuperación persistente, versión no soportada, aislamiento y reserva sin payload en `application/usecases/socratica_specify_recovery_test.go`, sin perder decisiones ni resetear preguntas. (dep: T034)
- [X] T046 [P] [US5] Añadir protección de hojas nuevas/revisión, invalidación por sanitización y ausencia de residuos persistentes en `adapters/secondary/persistence/socratica_specify_sanitize_test.go`, incluyendo aprobación pendiente modificada. (dep: T034)
- [X] T047 [P] [US5] Añadir consulta/retoma/cancelación de `specify` en `adapters/primary/cli/cmd_socratica_specify_test.go`, verificando que no entra en drive, ejecución, edición textual o asistencia genéricas. (dep: T034)
- [X] T048 [P] [US5] Añadir detalle/historial/acciones y pureza de dibujado en `adapters/primary/tui/tui_socratica_specify_test.go`, con anchos distintos y reserva privada sin contenido disponible. (dep: T034)
- [X] T049 [P] [US5] Añadir prueba entre dos instancias y pérdida de instancia en `tests/integration/socratica_specify_private_test.go`: pendiente único, reserva sin texto, rechazo de reconstrucción/ack/reenvío y cancelación explícita antes de empezar otro flujo. (dep: T034)
- [X] T050 [P] [US5] Añadir política privada fijada al inicio, cero historial/payload durable tras alternar ajustes y eliminación/retención completas en `adapters/secondary/persistence/socratica_specify_retention_test.go`, sin copias efímeras activas tras borrar la reserva conocida. (dep: T034)

### Implementation

- [X] T051 [US5] Ampliar categorías, protección recursiva, RedactFlow y depuración de preparación en `domain/privacy_maintenance.go`, `adapters/secondary/persistence/socratica_sanitize.go` y `adapters/secondary/persistence/socratica.go`; invalidar revisión/autorización no consumidas si cambia contenido y conservar solo las marcas necesarias de un envío consumido. (dep: T046, T050)
- [X] T052 [US5] Completar recuperación, política fija de historial y `private_state_unavailable` en `application/usecases/socratica_specify.go`, `application/usecases/socratica_flow.go` y `adapters/secondary/persistence/socratica_ephemeral.go`; permitir solo consultar/cancelar reserva sin payload, nunca reconstruir autorización. (dep: T045, T049, T051, T044)
- [X] T053 [US5] Reconocer modo antes de rutas locales genéricas en `adapters/primary/cli/cmd_socratica.go`: mostrar etapa/revisión/pendientes, instruir retoma en el agente y permitir cancelación válida, sin generar o despachar al consultar. (dep: T047, T052)
- [X] T054 [US5] Añadir detalle y acciones seguras en `adapters/primary/tui/socratica.go`, con retorno al agente y aviso de payload no disponible; conservar dibujado puro y acciones previas de otros modos. (dep: T048, T052)

**Checkpoint**: Recuperación y consulta local conservan estado/privacidad; la pérdida de payload se declara sin recuperación ficticia.

## Phase 8: User Story 6 — distribución y aceptación en tres agentes (P2)

**Objetivo**: Instalar el mismo procedimiento/alias, conservar modos previos y demostrar el traspaso real por agente.

**Independent Test**: Instalar dos veces/desinstalar en entornos aislados; comprobar invocaciones y modos previos; después de la puerta automática, generar e inspeccionar especificación y checklist reales en un proyecto separado para cada agente.

### Tests

- [X] T055 [P] [US6] Ampliar instalación idempotente, agentes ausentes y destinos del wrapper en `adapters/primary/setup/socratica_install_test.go`, verificando también eliminación real de artefactos concretos y preservación de archivos ajenos. (dep: T044, T053, T054)
- [X] T056 [P] [US6] Añadir Socrática a la paridad plantilla/copia en `adapters/primary/setup/agent_skills_setup_test.go` y ampliar `adapters/primary/setup/socratica_skill_test.go` para conservar refinamiento/escrutinio y las entradas natural/Codex del nuevo modo. (dep: T044, T053, T054)
- [X] T057 [P] [US6] Ampliar reconocimiento de alias, entrada Codex y petición natural en `domain/socratica_interception_test.go`, comprobando exclusión de procedencia general en `adapters/primary/cli/hook_socratica_test.go` sin activar intercepción por iniciativa del agente. (dep: T044, T053, T054)

### Implementation and acceptance

- [X] T058 [US6] Crear `infrastructure/templates/socratica/commands/socratica-hu.md` como wrapper fino a la habilidad común en `mode=specify`, conservando el argumento original y sin duplicar reglas del recorrido. (dep: T055)
- [X] T059 [US6] Añadir destinos Claude/OpenCode en `adapters/primary/setup/agent_skills_setup.go` y registrar el cuerpo en `adapters/primary/cli/cmd_mcp_setup.go`; usar comparación de contenido y no crear raíces de agentes ausentes. (dep: T058, T056)
- [X] T060 [US6] Ampliar `IsSocraticaInvocation` en `domain/socratica_interception.go` para las entradas nuevas sin capturar texto ajeno y conservando reconocimiento de procedencia aprobada y exclusiones existentes. (dep: T057)
- [X] T061 [US6] Verificar inventario derivado del wrapper en `adapters/primary/setup/global_artifacts.go` mediante `adapters/primary/setup/socratica_install_test.go`, corrigiendo registro/retirada gestionada si hay omisiones y conservando contenedores compartidos y configuraciones ajenas. (dep: T059)
- [X] T062 [US6] Ejecutar build, vet, pruebas completas, race y cobertura (mínimo 80 % en código nuevo y sin bajar de la línea base registrada en T001) y registrar salidas en `specs/042-socratica-specify-preparation/evidence/automated.md`; exigir casos nuevos de 042, regresión de modos previos, migración y privacidad, sin rebajar puertas ni equiparar harness refine con traspaso. (dep: T061, T060, T056, T057)
- [ ] T063 [P] [US6] Demostrar el caso D de `specs/042-socratica-specify-preparation/quickstart.md` en Claude Code, con proyecto desechable exclusivo, y guardar `specs/042-socratica-specify-preparation/evidence/claude.md`: alias/lenguaje natural, aprobación, primera concesión, archivos reales y criterios rastreables. (dep: T062)
- [ ] T064 [P] [US6] Demostrar el caso D de `specs/042-socratica-specify-preparation/quickstart.md` en Codex, con proyecto desechable exclusivo, y guardar `specs/042-socratica-specify-preparation/evidence/codex.md`: skill/lenguaje natural, aprobación, primera concesión, archivos reales y criterios rastreables. (dep: T062)
- [ ] T065 [P] [US6] Demostrar el caso D de `specs/042-socratica-specify-preparation/quickstart.md` en OpenCode, con proyecto desechable exclusivo, y guardar `specs/042-socratica-specify-preparation/evidence/opencode.md`: alias/lenguaje natural, aprobación, primera concesión, archivos reales y criterios rastreables. (dep: T062)

**Checkpoint**: Distribución comprobada y evidencias reales separadas. Una demostración sin autenticación/capacidad o no ejecutada permanece pendiente; no se reemplaza por presencia del binario ni un test de refine.

## Phase 9: Polish — documentación y cierre transversal

**Objetivo**: Entregar documentación operativa y trazabilidad real, sin repetir puertas aprobadas salvo cambios, fallos o dudas nuevas.

- [X] T066 [P] Actualizar `docs/socratica.md` con invocaciones, etapas, límite acumulativo, revisión, preflight, concesión/replay, recuperación privada y resultado comunicado, conservando instrucciones vigentes de los otros modos. (dep: T062)
- [X] T067 [P] Actualizar `docs/DATABASE.md` con columnas/versión aditivas, registros nulos y reserva sin texto frente a payload efímero, sanitización, retención y pérdida de instancia. (dep: T062)
- [ ] T068 Ajustar `specs/042-socratica-specify-preparation/quickstart.md` a órdenes/resultados efectivamente verificados y enlazar evidencias por agente sin añadir capacidades no demostradas ni datos privados. (dep: T063, T064, T065)
- [ ] T069 Cerrar la matriz FR-001..FR-028/SC-001..SC-009 en `specs/042-socratica-specify-preparation/evidence/acceptance.md`, enlazando pruebas y artefactos reales; dejar explícitamente pendientes los casos no ejecutados y no declarar completa la feature mientras falte aceptación. (dep: T066, T067, T068)

## Dependencies & Execution Order

### Árbol de entregas

Este árbol agrupa las tareas de ejecución; las hojas detalladas son las 69 entradas anteriores, no tareas adicionales. Las 22 hojas del plan se desglosan en pruebas, componentes y aceptación por historia.

```text
🎯 Preparación validada y traspaso aprobado a Spec Kit
├─ [1] Establecer base compartida → T001–T013
├─ [2] Preparar borrador → US1 / T014–T020                    (dep: 1)
├─ [3] Comprobar etapas → US2 / T021–T027                    (dep: 2)
├─ [4] Vincular revisión/aprobación → US3 / T028–T034        (dep: 3)
├─ [5] Entregar y reportar → US4 / T035–T044                 (dep: 4)
├─ [6] Recuperar y consultar con privacidad → US5 / T045–T054
│  └─ Pruebas desde T034; integración T052 espera T044       (dep: 4, 5)
├─ [7] Distribuir y demostrar → US6 / T055–T065              (dep: 5, 6)
└─ [8] Documentar y cerrar evidencia → T066–T069             (dep: 7)
```

### Dependencias de historias y archivos compartidos

- Setup precede a la base. La base debe estar completa antes de las implementaciones de historias.
- US1→US2→US3→US4 representa el recorrido de negocio; no se presenta como seis historias sin dependencias.
- US5 se prueba sobre flujos sintéticos desde US3. T052 espera US4 para no modificar simultáneamente los mismos casos de uso ni integrar una recuperación incompatible con la entrega ya definida.
- US6 espera US4 y US5: no distribuir el modo como completo antes de sus controles de recuperación y privacidad.
- T063/T064/T065 usan proyectos y archivos de evidencia distintos; no comparten `.specify/feature.json`, almacenes ni preparación.
- `domain/socratica.go`, `domain/socratica_specify.go`, `application/usecases/socratica_flow.go`, `application/usecases/socratica_specify.go`, `adapters/primary/cli/cmd_mcp_socratica_tool.go` y las dos copias de la skill tienen ediciones sucesivas: serializarlas aunque otras pruebas puedan avanzar.
- T066/T067 pueden ejecutarse después de T062 mientras se realizan las demostraciones. T069 espera todas las evidencias y documentación. No se realiza commit, push o release como parte de esta lista sin una solicitud posterior.

### Parallel execution examples per story

| Historia | Grupo paralelo con dependencias ya completas | Restricción |
| --- | --- | --- |
| US1 | T014 ∥ T015 ∥ T016 | Tras base; pruebas en archivos distintos. |
| US2 | T021 ∥ T022 ∥ T023 | Tras T020; no implementar todavía los archivos comunes. |
| US3 | T028 ∥ T029 ∥ T030 | Tras T027; no editar reductor/caso de uso simultáneamente. |
| US4 | T035 ∥ T036 ∥ T037 ∥ T038 ∥ T039 | Tras T034; cada contrato usa su archivo de prueba. |
| US5 | T045 ∥ T046 ∥ T047 ∥ T048 ∥ T049 ∥ T050 | Tras T034; integración productiva espera T044. |
| US6 | T055 ∥ T056 ∥ T057; luego T063 ∥ T064 ∥ T065 | Primer grupo tras US4/US5; demostraciones solo tras T062 y con proyectos separados. |

En la base, T004/T005/T006/T012 pueden escribirse en paralelo tras T003. En cierre, T066/T067 modifican documentos distintos. El paralelismo es una oportunidad de organización; no requiere delegar ni autoriza subagentes por sí mismo.

## Implementation Strategy

### MVP e incrementos

1. Completar Setup y base. No añadir instalaciones, dependencias o servicios nuevos para empezar.
2. Entregar US1 como incremento de preparación sin despacho: permite inspeccionar un borrador y sus faltantes. US1 no equivale a la aceptación completa del traspaso.
3. Completar US2 y US3 para una preparación comprobada y revisada por la persona, todavía sin necesidad de generador real para probar sus controles.
4. Completar US4 para el MVP funcional del traspaso desde el agente existente. Verificar disponibilidad antes de consumir y el texto exacto después de conceder.
5. Completar US5 y US6 antes de publicar la ampliación como soportada: privacidad/recuperación, distribución y demostraciones reales son requisitos de esta primera versión, no mejoras opcionales.
6. Cerrar documentación/evidencia. Si falta una ejecución real, mantener su checkbox pendiente y comunicar causa/siguiente paso sin declarar la feature terminada.

La prueba independiente de una historia usa preparaciones sintéticas en su estado de entrada, sin tener que repetir conversaciones o invocar modelos. Esa independencia de verificación no elimina las dependencias de implementación del recorrido.

### Cobertura de requisitos y resultados

| Requisitos | Tareas principales | Resultados |
| --- | --- | --- |
| FR-001–FR-006 | T014–T020, T021–T026, T038, T043 | SC-001, SC-002 |
| FR-007–FR-010 | T021–T027 | SC-001, SC-002 |
| FR-011–FR-015 | T012–T013, T028–T034, T051 | SC-003, SC-005, SC-009 |
| FR-016–FR-020 | T005, T009, T035–T044, T063–T065 | SC-003–SC-006, SC-009 |
| FR-021–FR-023 | T003–T011, T045–T046, T049–T052 | SC-007 |
| FR-024 | T047–T048, T053–T054 | SC-007 |
| FR-025–FR-028 | T055–T065, T066–T069 | SC-006, SC-008 |

### Formato y estado de generación

69 tareas, todas pendientes: Setup 2; base 11; US1 7; US2 7; US3 7; US4 10; US5 10; US6 11; cierre 4. Hay 32 tareas marcadas como paralelizables, sujetas a sus dependencias y separación de archivos. Los criterios independientes figuran en cada fase de historia. Los requisitos se relacionan con las tareas de implementación y con evidencia futura; ninguna casilla se marca por terminar esta generación.

Las comprobaciones de esta generación verifican secuencia de IDs, etiquetas por fase, rutas, dependencias acíclicas, cobertura de las seis historias y enlaces. Los hooks `before_tasks/after_tasks` se revisan al generar y al cerrar; no hay entradas registradas en la configuración actual.
