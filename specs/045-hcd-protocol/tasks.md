---
description: "Lista de tareas de la feature 045 — HCD"
---

# Tasks: HCD — protocolo propio de especificación y desarrollo continuo

**Input**: documentos de diseño en `specs/045-hcd-protocol/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: incluidos. La Constitución (§10, §21.4) exige pruebas con cada cambio y TDD cuando sea práctico, y la preferencia registrada del Implementador es seguir TDD y el orden de `tasks.md`. Cada prueba se escribe primero y debe fallar antes de implementar.

**Organization**: por historia de usuario, en orden de prioridad (P1: US2, US3, US6 → P2: US4, US8, US1 → P3: US5 → P4: US7).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizable (otro archivo y sin dependencias pendientes).
- **[Story]**: historia de la spec (US1…US8).
- Rutas relativas a la raíz del repositorio.

---

## Phase 1: Setup — D1 Gobierno (bloquea cualquier unidad HCD real)

**Purpose**: resolver el conflicto con la §18 y dejar el Playbook y las plantillas que el motor sembrará.

- [X] T001 Crear la rama `045-hcd-protocol` desde `main` (`git switch -c 045-hcd-protocol`), sin commit ni push, y registrar en `specs/045-hcd-protocol/evidence/baseline.md` el SHA de `main` de partida.
- [X] T002 Registrar en `specs/045-hcd-protocol/evidence/baseline.md` la huella SHA-256 de cada archivo de las skills Spec Kit instaladas en el proyecto (`.claude/skills/speckit-*`, `.agents/skills/speckit-*`, `.opencode/commands/speckit*`), que no están versionadas en git, para compararlas al final (C-24).
- [X] T003 Crear `docs/adr/0002-hcd-estado-de-control-en-sqlite.md` con el formato de `docs/adr/0001-migracion-al-abrir-la-base.md`: contexto (Spec Kit frente a HCD), decisión (motor `mem hcd` + estado de control en tablas `hcd_*` de la SQLite del proyecto + artefactos Markdown como contenido + Playbook como documento fijado + unidad activa interna sin archivos de contexto del agente), alternativas descartadas (R1, R2, R3, R15 y R18 de `research.md`), sección «Composition root» que respalda la excepción §25.2, consecuencias, responsable y fecha (§18, §23).
- [X] T004 Presentar al Implementador, una vez redactado el ADR 0002 que enlaza la §25.2 (tarea anterior), el texto de `specs/045-hcd-protocol/contracts/constitution-18-amendment.md` (corrección de la §18 y excepción §25.2 sobre `infrastructure/container.go`) y registrar su respuesta literal en `specs/045-hcd-protocol/evidence/constitution-18-approval.md`. Si la rechaza o la modifica, actualizar la propuesta y repetir. **Puerta manual: T005 no empieza sin aprobación.** **Hecha (2026-10-28): aprobación literal «Apruebo la corrección de la §18 y la excepción §25.2» en `evidence/constitution-18-approval.md`.**
- [X] T005 Aplicar la §18 y la §25.2 aprobadas en la constitución fijada mediante `mem constitution` y verificar la sincronización a `.specify/memory/constitution.md` (vía `syncSpeckitConstitution` en `adapters/primary/cli/cmd_constitution.go`), guardando la salida en `specs/045-hcd-protocol/evidence/constitution-18-applied.md`. **Hecha (2026-10-28): constitución 2.2.0 fijada y sincronizada; las dos copias son idénticas. Corrigió una divergencia previa (la fijada no tenía la §25.1). Ver `evidence/constitution-18-applied.md`.**
- [X] T006 [P] Redactar el valor por defecto del Playbook en `infrastructure/templates/hcd/hcd-playbook.md`:
  - secciones: 1. Propósito; 2. Principios HCD (los 12 de la sección «Principios HCD» de la spec y su «Terminología»); 3. Ceremonia de Especificación y selección de enfoque (FR-001 a FR-006); 4. Unidades de trabajo y sus tipos (FR-014); 5. Historias y criterios (FR-015); 6. Aclaración (FR-016a); 7. Aprobación citada (FR-017 a FR-017b); 8. Vertical slicing (FR-020); 9. Tareas atómicas y trazabilidad (FR-021 a FR-024, FR-036); 10. Pruebas, evidencia y vigencia (FR-029 a FR-032); 11. Verificación y cierre (FR-033 a FR-035); 12. Reglas para agentes (`contracts/skills.md` §2); 13. Memoria, unidad activa y heartbeat (FR-038 a FR-043); 14. Migración desde Spec Kit (FR-054 a FR-056); 15. Gobierno de excepciones y evolución del Playbook (FR-011, FR-012, FR-037);
  - la regla `HCD-MEM-01` (cierre con memoria no disponible: solo con una excepción de esa regla, aprobada y vigente);
  - cada regla obligatoria con el formato `### HCD-<ÁREA>-NN.` y las líneas `Descripción`, `Ámbito`, `Comprobación` (`structural`/`review`), `Evidencia`, `Excepción` y `Origen`;
  - la sección «Principios de diseño» (FR-007b): reglas `HCD-CLEAN-NN` de Arquitectura Limpia (regla de dependencia hacia el dominio, independencia de frameworks/BD/interfaz/agentes, límites solo por ports, datos simples al cruzar límites) y `HCD-SOLID-SRP|OCP|LSP|ISP|DIP`, cada una con `Comprobación` `structural` (citando la guarda existente, p. ej. `tests/contract/layer_rules_test.go`) o `review`, su `Evidencia` en la verificación HCD y su relación con la §3, a la que remiten sin repetirla;
  - la sección «Engineering Guardrails» como tabla de correspondencia que **remite** a las secciones §3, §5, §6, §10, §16, §20, §21 y §23 de la Constitución, **sin copiar su texto** (FR-007a);
  - el riesgo residual de la aprobación citada (FR-017a).
- [X] T007 [P] Crear las plantillas `infrastructure/templates/hcd/playbook/templates/definition.md` y `infrastructure/templates/hcd/playbook/templates/delivery.md` con exactamente el formato de `specs/045-hcd-protocol/contracts/artifact-format.md`.
- [X] T008 Escribir pruebas del documento fijado `playbook`:
  - `PinnedDocByAlias("playbook")` existe y su `DefaultSHA256` coincide con `ContentFingerprint` de la plantilla embebida (mismo control que para la Constitución);
  - `ResolvePinnedDoc` devuelve el valor por defecto sin versión de proyecto y la versión importada si existe;
  - la siembra actualiza una copia intacta de un valor anterior y no toca una editada;
  - la plantilla embebida contiene las reglas `HCD-CLEAN-*` y las cinco `HCD-SOLID-*`, cada una con `Comprobación` declarada (FR-007b);
  - en `domain/seed_test.go` y `application/usecases/seed_upgrade_test.go`.
- [X] T009 Añadir `TopicHCDPlaybook = "gomemory:hcd-playbook"` y la entrada `playbook` (`Type: Architecture`, `Title` «Playbook HCD del proyecto», `Label` «Playbook HCD», `Template` `hcd/hcd-playbook.md`, `DefaultSHA256` de T006) a `domain.PinnedDocs` en `domain/seed.go` (hace pasar T008; la TUI de documentos lo muestra sin cambios porque recorre el catálogo).
- [X] T010 Añadir el atajo `mem playbook` (`case "playbook"` en `adapters/primary/cli/dispatcher.go`) y generalizar `--sync` de `runPinnedShortcut` con un destino por alias (`constitution` → `.specify/memory/constitution.md`, igual que hoy; `playbook` → `hcd/hcd-playbook.md`, solo si existe `hcd/`), escribiendo primero su prueba en `adapters/primary/cli/cmd_constitution_test.go`.


**Checkpoint**: §18 corregida y aprobada; `mem playbook` muestra el valor por defecto y admite importar, exportar y restaurar; plantillas listas para embeber (`infrastructure/main.go` ya embebe `all:templates`).

---

## Phase 2: Foundational (bloquea todas las historias)

**Purpose**: dominio, persistencia, artefactos y esqueleto del motor sobre los que se construye cada historia.

### Pruebas primero

- [X] T011 [P] Escribir pruebas de dominio de estados y transiciones de la unidad (tabla de `data-model.md` §2, estados terminales inmutables) en `domain/hcd_test.go`; un caso por cada fila de la tabla «Etapa según el estado» de `data-model.md` §2 (FR-018); además, una tabla de transiciones válidas e inválidas para los estados de historia, criterio, slice, tarea, excepción y migración (data-model §1, FR-027).
- [X] T012 [P] Escribir pruebas del hash de revisión (normalización LF, sin espacio final, 12 hex; mismo contenido con CRLF da el mismo hash) en `domain/hcd_revision_test.go`.
- [X] T013 [P] Escribir pruebas de persistencia `hcd_*` (crear y leer unidad, `WithTransaction` todo o nada, `expected_revision` desactualizada → `ErrRevisionConflict`, operación repetida → resultado guardado, una arista duplicada en `hcd_links` (mismo par y relación) no se inserta dos veces, y la trazabilidad historia → criterio → slice → tarea → evidencia se recupera recorriendo las aristas (FR-040a); mismo `operation_id` con otro hash → `ErrOperationConflict`) con SQLite real en `t.TempDir()` en `adapters/secondary/persistence/hcd_test.go`.
- [X] T014 [P] Actualizar `adapters/secondary/persistence/db_schema_golden_test.go` con **todas** las tablas `hcd_*` de `data-model.md` §3 (incluidas `hcd_normative_revisions`, `hcd_approval_prompts`, `hcd_focus`, `hcd_heartbeats`, `hcd_playbook_adoptions` y `hcd_migrations`, para que una sola subida de versión cubra la feature) y la versión 4, y añadir un caso que abra una base v3 existente y la lleve a v4 sin perder filas. Actualizar también la expectativa `schemaVersion == 3` de `adapters/secondary/persistence/socratica_specify_migration_test.go:98` a la constante vigente, justificado por el esquema v4 (§10: el cambio legítimo actualiza prueba y spec juntas).
- [X] T015 [P] Escribir pruebas del parser y escritor de artefactos (definición válida, campo obligatorio ausente → aviso tipado, clave desconocida → aviso, ID duplicado → error, ida y vuelta sin cambios de hash) con fixtures en `adapters/secondary/hcdfiles/testdata/` y pruebas en `adapters/secondary/hcdfiles/parse_test.go`.
- [X] T016 [P] Escribir la prueba de contrato del envoltorio JSON (campos de `contracts/engine-cli.md` §3, códigos de error y exit codes de §4, `usage` con exit 2 sin argumentos y sin TTY) en `tests/contract/hcd_engine_contract_test.go`.

- [X] T017 [P] Escribir pruebas de `Focus`: fijar una unidad la deja activa (una por proyecto); fijar otra la reemplaza; leer sin unidad activa → vacío; liberar la deja vacía; una unidad terminal no puede quedar activa (FR-041a, invariante 10), en `application/usecases/hcd_focus_test.go`.

### Implementación

- [X] T018 [P] Implementar `Unit`, `Kind`, `State`, `Stage`, la tabla de transiciones, `AllowedActions(unit)` y `NextAction(unit, pending)` en `domain/hcd.go` (hace pasar T011).
- [X] T019 [P] Implementar `ContentRevision(content []byte) string` y `ErrRevisionConflict`, `ErrOperationConflict`, `ErrApproachMismatch`, `ErrInvalidQuote`, `ErrAtomicity`, `ErrBlocked` y `ErrUncertain` en `domain/hcd_revision.go` y `domain/hcd_errors.go` (hace pasar T012).
- [X] T020 [P] Definir `Story`, `Criterion` y `Definition` con `ValidateDefinition(def, kind) []PendingCheck` (FR-014, FR-015) en `domain/hcd_definition.go`.
- [X] T021 [P] Definir `Slice`, `Task`, `Execution`, `Evidence`, `Decision`, `Clarification`, `Block`, `Approval`, `Link`, `Focus` y `Heartbeat` como tipos de dominio sin lógica de infraestructura en `domain/hcd_delivery.go` y `domain/hcd_control.go`.
- [X] T022 Declarar los ports `HCDRepository` (CRUD por agregado y operaciones), `HCDUnitOfWork` (`WithTransaction(ctx, fn func(HCDRepository) error) error`) `HCDArtifactRepository` (`Read`, `Write`, `Hash`, `Exists`, `ListUnits`, `Root`) y `HCDContextRepository` (lectura de la unidad activa para el contexto de arranque) en `application/ports/hcd_repository.go`, `application/ports/hcd_unit_of_work.go` y `application/ports/hcd_artifact_repository.go`.
- [X] T023 Escribir el esquema aditivo de todas las tablas `hcd_*` con índices y unicidades de `data-model.md` §3 en `adapters/secondary/persistence/hcd_schema.go`.
- [X] T024 Incluir `hcdSchema` en la preparación transaccional y subir `schemaVersion` de 3 a 4 en `adapters/secondary/persistence/db.go` (hace pasar T014).
- [X] T025 Implementar `HCDStore`, que cumple `HCDRepository`, `HCDUnitOfWork` y `HCDContextRepository` (con `var _ ports.HCDContextRepository = (*HCDStore)(nil)`) (SQL parametrizado, sin `SELECT *`, `WithTransaction` y `writeContext` con reintento ante BUSY copiados del patrón de `SocraticaStore`, timeouts con `domain.DefaultQueryTimeout`, `domain.ErrNotFound` envuelto) en `adapters/secondary/persistence/hcd.go` (hace pasar T013).
- [X] T026 Implementar el parser y escritor del formato parseable en `adapters/secondary/hcdfiles/parse.go` y el `HCDArtifactRepository` sobre `<root>/hcd/` (escritura atómica, nunca fuera de `hcd/`, nunca bajo `specs/`) en `adapters/secondary/hcdfiles/store.go` (hace pasar T015).
- [X] T027 Implementar el núcleo del caso de uso `HCD` (constructor con ports, `Clock` existente y `newID func() string` inyectable; ayudante `mutate(ctx, unitID, expectedRev, opID, payloadHash, fn)` que aplica revisión, idempotencia y detección de edición externa FR-040c dentro de la transacción) en `application/usecases/hcd_flow.go`.
- [X] T028 Crear el slice CLI con `Run(deps *clikit.Deps, args []string) int`, parseo de subcomandos, `--json`, `--client`, `--expected-revision`, `--operation-id`, envoltorio de respuesta y errores → exit codes de `contracts/engine-cli.md` en `adapters/primary/cli/hcd/cmd_hcd.go` y `adapters/primary/cli/hcd/output.go` (hace pasar T016).
- [X] T029 Añadir el campo `HCD *usecases.HCD` en `adapters/primary/cli/clikit/deps.go`, `case "hcd"` en `adapters/primary/cli/dispatcher.go` y la ayuda de `mem hcd` en `commandHelp` de `adapters/primary/cli/presentation.go`.
- [X] T030 Cablear `HCDStore`, `hcdfiles.Store` y `usecases.HCD` en `infrastructure/container.go`, junto a `NewSocraticaStore`, y asignar el `HCDStore` como `HCDContextRepository` del `usecases.Builder` que construye el contenedor.
- [X] T031 Documentar `hcd_heartbeat_timeout_ms`, `hcd_grant_timeout_hours` y `hcd_inactive_days` (propósito, valor por defecto, rango y unidad) en `docs/CONFIGURATION.md`, junto a las claves `socratica_*`, y añadir `HCDSettings` (`hcd_heartbeat_timeout_ms`, `hcd_grant_timeout_hours`, `hcd_inactive_days`) con valores por defecto y rangos validados, siguiendo el patrón de `domain.SocraticaSettings`, en `domain/hcd_settings.go`, escribiendo primero su prueba en `domain/hcd_settings_test.go` (§20).
- [X] T032 Implementar `Focus` (fijar, leer y liberar sobre `hcd_focus`) en `application/usecases/hcd_focus.go`, para que `Start`, `Resume` y `Close` lo usen desde las fases 3 y 5 (hace pasar T017).
- [X] T033 Ejecutar `go test ./tests/contract/ -run LayerRules` para confirmar que los paquetes nuevos respetan L1–L3 (domain sin adapters; application sin adapters ni infrastructure), y guardar la salida en `specs/045-hcd-protocol/evidence/foundation-layers.md`.

- [X] T034 Documentar en `docs/DATABASE.md` las tablas `hcd_*` (propósito, columnas clave, unicidades e índices de `data-model.md` §3), la subida de `schemaVersion` de 3 a 4 bajo la excepción §25.1 y la retención de `hcd_heartbeats` (§18).

**Checkpoint**: `mem hcd --help` responde; el esquema v4 migra desde vacío y desde v3; el contrato JSON pasa.

---

## Phase 3: User Story 2 — Definir y aprobar una unidad (Priority: P1) 🎯 MVP

**Goal**: crear una unidad, definirla, aclarar ambigüedades y aprobar su revisión exacta con una cita.

**Independent Test**: quickstart, escenario 1 pasos 1–3 y escenario 4: una unidad `fix` incompleta queda en `draft` con faltantes; completada y aprobada con cita, pasa a `ready`.

### Pruebas primero

- [X] T035 [P] [US2] Escribir pruebas de `ValidateDefinition` para los cuatro tipos (campos particulares FR-014; historia sin criterio; criterio sin `Verificación`) en `domain/hcd_definition_test.go`; un campo con marcador de pendiente (`pendiente`, `TODO`, `[NEEDS CLARIFICATION: …]`) → `DEF-PENDING`, salvo `Causa raíz: pendiente` en `fix` (`contracts/artifact-format.md`).
- [X] T036 [P] [US2] Escribir pruebas del caso de uso: `Start` crea `HCD-001`, la carpeta y `definition.md` desde la plantilla fija `playbook_rev` y `constitution_rev` (`ContentFingerprint` de los documentos fijados resueltos) e inserta su contenido en `hcd_normative_revisions` si es nuevo (con `change_kind`; sin motivo ni responsable, que se registran al adoptar), y fija `focus`; `Define` cambia `definition_rev`; `Validate` no cambia `state_revision`; `Approve` exige una revisión igual a `definition_rev`, una cita válida y ninguna aclaración bloqueante; tras `ready`, una nueva `Define` vuelve a `draft` y la aprobación anterior no autoriza (C-03, C-04); `approve --request` o `approve` sobre una revisión ya aprobada con el mismo alcance → `already_approved` (FR-017), en `application/usecases/hcd_flow_test.go`.
- [X] T037 [P] [US2] Escribir pruebas de la heurística de cita (R7) y del origen (FR-017b: `--client terminal` → `origin=terminal` con la misma validación de cita; `--client` de agente → `origin=conversation`): la revisión de 12 hex y «aprobar», «apruebo», «sí, aprobar» son aceptadas; «A» o «recommended» solo con un `prompt_id` vigente de `approve --request` para el mismo objeto y revisión, y sin él → `ErrInvalidQuote`; vacío o texto sin elección → `ErrInvalidQuote`; la cita pasa por `domain.ProtectText`, en `domain/hcd_quote_test.go`.
- [X] T038 [P] [US2] Escribir pruebas de aclaraciones: abrir una bloqueante hace que `Validate` devuelva los faltantes más el `pending_check` de la aclaración, y que `Approve` responda `blocked`; responderla crea la decisión `D-NN` `accepted` con referencia a la aclaración (FR-016a, FR-040b) en `application/usecases/hcd_clarify_test.go`.

### Implementación

- [X] T039 [US2] Implementar `ValidQuote(quote, revision string) bool` en `domain/hcd_quote.go` (hace pasar T037).
- [X] T040 [US2] Implementar `Start`, `Define` y `Validate` (enlaces `belongs_to` y `covers` historia/criterio en `hcd_links`) en `application/usecases/hcd_flow.go` (hace pasar parte de T036).
- [X] T041 [US2] Implementar `RecordDecision` sobre el estado de control (`hcd_decisions`, enlaces `justifies`) en `application/usecases/hcd_memory.go`, como único camino para registrar decisiones; en esta fase no escribe memorias (el reflejo en memoria llega en la fase 6), escribiendo primero su prueba en `application/usecases/hcd_decision_test.go`.
- [X] T042 [US2] Conectar `RecordDecision` en `Start` (decisión de enfoque `hcd`, FR-001) y en `Define` sobre una unidad `ready` o posterior (decisión de cambio de alcance, FR-017), con su caso en `application/usecases/hcd_decision_test.go`.
- [X] T043 [US2] Implementar `Clarify` (abrir y responder) registrando la decisión con `RecordDecision` en `application/usecases/hcd_clarify.go` (hace pasar T038).
- [X] T044 [US2] Implementar `Approve(object=definition)` y `approve --request` (tabla `hcd_approval_prompts`, consumo único del `prompt_id`) con el registro en `hcd_approvals` (origen, cita, cliente y revisión) y la decisión de aprobación vía `RecordDecision` en `application/usecases/hcd_flow.go` (completa T036).
- [X] T045 [US2] Implementar `Resume`, `Next` y `Show` de solo lectura (paso, `allowed_actions`, `pending_checks`, aclaraciones abiertas y `next_action.skill`) en `application/usecases/hcd_query.go`.
- [X] T046 [US2] Exponer `start`, `define`, `validate`, `clarify`, `approve`, `resume`, `next`, `show` y `focus` en `adapters/primary/cli/hcd/cmd_hcd.go`, con el fixture de contrato que compruebe `state_revision` antes y después de cada consulta (C-11) en `tests/contract/hcd_engine_contract_test.go`; caso C-17: dos mutaciones con la misma `expected_revision`, la segunda recibe `revision_conflict` y el archivo registrado por la primera queda intacto.
- [X] T047 [US2] Escribir primero la prueba que verifica que cada `infrastructure/templates/hcd/*/SKILL.md` contiene las 8 reglas comunes y ninguna instrucción de escribir archivos fuera de `hcd/` (archivos de contexto del agente o marcadores `SPECKIT`); además, cada comando mutante de las fases 2 y 3 (`start`, `define`, `clarify`, `approve`, `focus`) aparece en al menos una `SKILL.md`, en `adapters/primary/setup/hcd_skills_test.go`.
- [X] T048 [P] [US2] Redactar la skill de entrada `infrastructure/templates/hcd/hcd/SKILL.md`: reglas comunes 1–8 de `contracts/skills.md` §2 literales; comportamiento sin argumentos, con texto libre y con id (§3); `focus` sin escribir archivos de contexto; `classify` y `memory sync` solo se ofrecen cuando aparecen en `allowed_actions` del motor (llegan en las fases 8 y 6), con cita cuando corresponda (`contracts/skills.md` §1).
- [X] T049 [P] [US2] Redactar `infrastructure/templates/hcd/hcd-define/SKILL.md` (redactar en un archivo temporal → `define --file` → `validate`; ante una ambigüedad, pasar a `hcd-clarify`).
- [X] T050 [P] [US2] Redactar `infrastructure/templates/hcd/hcd-clarify/SKILL.md` (una pregunta a la vez, 2–4 opciones y una recomendación; registrar con `clarify --answer`; no elegir por probabilidad; aceptar o sustituir decisiones solo con `decision --accept|--supersede --quote` y la respuesta literal de la persona).
- [X] T051 [P] [US2] Redactar `infrastructure/templates/hcd/hcd-approve/SKILL.md` (mostrar el objeto y la revisión de 12 hex, pedir una elección explícita y pasar la respuesta literal en `--quote`; nunca redactar la cita).

**Checkpoint**: US2 es demostrable con `/hcd`, `/hcd-define`, `/hcd-clarify` y `/hcd-approve` instaladas a mano en `~/.claude/skills/` (la instalación automática llega en T099).

---

## Phase 4: User Story 3 — Ejecutar slices mediante tareas atómicas (Priority: P1) 🎯 MVP

**Goal**: registrar el siguiente slice y sus tareas, admitir solo tareas atómicas y ejecutar con concesión y reporte.

**Independent Test**: quickstart, escenario 1 pasos 4–5: T-002 con `C-99` responde `atomicity`/`ATOM-LINK`; T-001 pasa `check`, `run` y `report`.

### Pruebas primero

- [X] T052 [P] [US3] Escribir pruebas de `AtomicityViolations` con un caso por código `ATOM-*` de `data-model.md` §1 (incluido `ATOM-DECISION` para tareas `scope-change`, FR-036) y de la detección de ciclos en dependencias; slice sin comportamiento observable, límites o verificación esperada → `SLICE-FIELDS` (FR-020); tarea habilitante sin slice desbloqueado o sin justificación → `ATOM-ENABLING` (FR-024), en `domain/hcd_delivery_test.go`.
- [X] T053 [P] [US3] Escribir pruebas del caso de uso: `Slice` registra sin exigir slices futuros (C-07); `Tasks` rechaza ciclos; `Check` de tarea huérfana → `ATOM-LINK` (C-05); tarea con dos resultados → `ATOM-RESULT` (C-06); una tarea bloqueada no impide `Check` de otra independiente; `Run` concede `execution_id` y pasa la unidad a `in_progress`; `Report` acepta un único resultado, devuelve el guardado si se repite igual y rechaza si cambia (C-18); `Run` entrega en su respuesta la tarea, su alcance, sus exclusiones y la revisión vigente (US8, escenario 1); `block` sobre una tarea no cambia el estado de la unidad y solo `block --critical` sobre la unidad la pasa a `blocked` (data-model §2), en `application/usecases/hcd_delivery_test.go`.

- [X] T054 [P] [US3] Escribir pruebas de `Report`: la evidencia sin procedimiento, resultado o sujeto → rechazo con los campos faltantes (FR-030); un secreto en la evidencia o en un artefacto se guarda protegido por `domain.ProtectText` (FR-044); sin ejecutor disponible (`--client terminal` o sin cliente, FR-025), `run` responde `no_executor` y la tarea no queda como ejecutada (FR-025); una evidencia cuyo `Inspeccionable` apunta a una referencia externa inaccesible no basta para verificar (caso límite de la spec), en `application/usecases/hcd_report_test.go`.

### Implementación

- [X] T055 [US3] Implementar `AtomicityViolations(task, unit, clarifications)` y `DependencyCycle(tasks)` en `domain/hcd_delivery.go` (hace pasar T052).
- [X] T056 [US3] Implementar `Slice` y `Tasks` (escritura de `delivery.md` por `HCDArtifactRepository`; enlaces `covers`, `belongs_to` y `depends_on`; `delivery_rev`) en `application/usecases/hcd_delivery.go`.
- [X] T057 [US3] Implementar `Check`, `Run` (concesión en `hcd_executions` con `definition_rev` y `delivery_rev`) y `Report` (parseo de evidencia según `contracts/artifact-format.md`, `hcd_evidence` y enlaces `verifies`) en `application/usecases/hcd_delivery.go` (hace pasar T053).
- [X] T058 [US3] Implementar `Block` (abrir y resolver, volviendo al estado previo solo si sigue siendo válido) en `application/usecases/hcd_block.go`, registrando con `RecordDecision` la apertura y la resolución de cada bloqueo (FR-040b), distinguiendo el bloqueo localizado (tarea o slice) del bloqueo crítico de la unidad, escribiendo primero su prueba en `application/usecases/hcd_block_test.go`.
- [X] T059 [US3] Exponer `slice`, `tasks`, `check`, `run`, `report`, `block` y `list` (versión básica: solo unidades HCD) en `adapters/primary/cli/hcd/cmd_hcd.go` y ampliar los casos de contrato en `tests/contract/hcd_engine_contract_test.go` con una prueba por consulta (`list` y `check` no cambian `state_revision`; contrato §5.1).
- [X] T060 [P] [US3] Redactar `infrastructure/templates/hcd/hcd-slice/SKILL.md` (solo el siguiente incremento comprobable; las agrupaciones horizontales son tareas habilitantes).
- [X] T061 [P] [US3] Redactar `infrastructure/templates/hcd/hcd-tasks/SKILL.md` (contrato de atomicidad de FR-021 y `data-model.md` §1; `check` de cada tarea antes de presentarla como lista).
- [X] T062 [P] [US3] Redactar `infrastructure/templates/hcd/hcd-implement/SKILL.md` (los 5 pasos de `contracts/skills.md` §4; reintentar `report` con el mismo `--operation-id`; nunca un `run` repetido).

**Checkpoint**: US2 + US3 ejecutan tareas con concesión y evidencia.

---

## Phase 5: User Story 6 — Verificar y cerrar con conocimiento de los límites (Priority: P1) 🎯 MVP

**Goal**: veredicto PASS/FAIL/BLOCKED derivado del estado, evidencia que caduca e informe generado; cierre de la revisión exacta.

**Independent Test**: quickstart, escenarios 1 (completo) y 3.

### Pruebas primero

- [X] T063 [P] [US6] Escribir pruebas de vigencia y veredicto: evidencia con revisiones que ya no coinciden (definición, entrega, Playbook o Constitución) → `stale`; criterio `verified` solo con una `pass` vigente y ninguna `fail` vigente; `not_applicable` sin decisión → no cuenta; slice `verified` derivado; bloqueo crítico, `fail` obligatoria o excepción vencida → `CanClose=false` con causas (C-08, C-09, C-22) en `domain/hcd_evidence_test.go`; si cambia el contenido de los archivos declarados en `Sujeto` (commit distinto o hash del árbol distinto), la evidencia pasa a `stale` (C-09, código evaluado); un criterio con estrategia de verificación y sin evidencia queda `covered`, no `verified` (FR-028).
- [X] T064 [P] [US6] Escribir pruebas del caso de uso: `Verify(slice)` y `Verify(unit)` → `verification` cuando todos los slices comprometidos están verificados; `Close` exige una revisión igual a `approved_rev` y a `definition_rev`, una verificación vigente y una cita, deja `completed` y libera `focus` (C-23); `close --reject --reason` deja la unidad en `in_progress` sin cerrarla; `cancel --quote` deja `cancelled` y `cancel --superseded-by` deja `superseded`, y ambos liberan `focus` (FR-027a); `verify` informa FAIL si la unidad no es de enfoque `hcd` (aislamiento de enfoques, FR-033); un PASS no cierra sin `Close`; una tabla de casos, uno por cada comprobación automática de FR-033 (referencias, identificadores, campos requeridos, dependencias, estados, evidencia registrada, revisiones, excepciones y aislamiento de enfoques); si la definición cambia con una ejecución `granted` en curso, esta pierde la autorización y su `report` posterior queda como evidencia `stale`; una `uncertain` sigue `uncertain` y no se concede de nuevo (data-model §2); unidad en `verification` cuyo `delivery.md` o hash de `Sujeto` cambia → vuelve a `in_progress` y `close` responde `not_allowed` (data-model §2), en `application/usecases/hcd_verify_test.go`.
  - Avance verificado: cierre con revisión errónea o sin cita, cancelación por sustitución y liberación de foco; `Verify` de otro enfoque devuelve veredicto con causa `APPROACH`; una definición nueva revoca la aprobación y deja el informe tardío `stale` tanto desde `granted` como desde `uncertain`; una `uncertain` no se concede de nuevo incluso tras aprobar la nueva definición; una edición externa de `delivery.md` impide cerrar y devuelve la unidad a `in_progress`. Verificación por slice conectada al CLI: PASS parcial no avanza la unidad, selector incluido en idempotencia e ID inexistente rechazado. Matriz FR-033 completa y validada; la excepción vencida se cubre con fixture de dominio en el MVP, conforme a T067.
- [X] T065 [P] [US6] Escribir la prueba golden del render de `verification.md` (cabecera GENERADO, secciones de `contracts/artifact-format.md`, origen y cita de cada aprobación, sección «Desvíos» con los del heartbeat, FR-041c) con un fixture determinista en `application/usecases/testdata/hcd_verification.golden.md` y `application/usecases/hcd_verify_render_test.go`.

- [X] T066 [P] [US6] Escribir pruebas de `Status` y del cierre: la vista de progreso separa implementado y verificado y muestra los fallos aunque el porcentaje sea alto (FR-034); `close` y `close --reject` registran su decisión con `RecordDecision` desde el MVP, en `application/usecases/hcd_status_test.go`.

### Implementación

- [X] T067 [US6] Implementar `EvidenceFreshness`, `CriterionStatus`, `SliceStatus` y `ClosureVerdict` en `domain/hcd_evidence.go` (hace pasar T063). En el MVP, el bloqueo del cierre por excepción vencida se prueba con un fixture de dominio; crear excepciones desde el motor llega en la fase 10.
- [X] T068 [US6] Implementar `Verify`, `Status` (vista de progreso FR-034, desvío `inactive_unit` calculado al consultar), `Close`, `close --reject` y `Cancel` (todas registran su decisión con `RecordDecision`) en `application/usecases/hcd_verify.go` (hace pasar T064).
- [X] T069 [US6] Implementar el render determinista de `verification.md` (incluida la sección «Desvíos») y su escritura por `HCDArtifactRepository` (aviso si se editó a mano) en `application/usecases/hcd_verify_render.go` (hace pasar T065).
- [X] T070 [US6] Exponer `verify`, `status`, `close` (con `--reject`) y `cancel` en `adapters/primary/cli/hcd/cmd_hcd.go` y añadir una prueba por consulta (`status`, `validate`, `resume`, `next`, `show` sin cambio de `state_revision`) y el caso de contrato de edición externa de `definition.md` → nueva revisión, evidencia `stale`, unidad en `draft` y `close` → `not_allowed` (FR-040c, C-09) en `tests/contract/hcd_engine_contract_test.go`; `resume <id>` fija `focus` y deja `state_revision` sin cambios (FR-046).
- [X] T071 [P] [US6] Redactar `infrastructure/templates/hcd/hcd-verify/SKILL.md` (presentar PASS/FAIL/BLOCKED y la acción requerida; PASS no equivale a aceptación).
- [X] T072 [P] [US6] Redactar `infrastructure/templates/hcd/hcd-close/SKILL.md` (`verify` → mostrar la revisión → elección explícita → `close --quote`; rechazo con `close --reject --reason --quote`; cancelación con `cancel --quote [--superseded-by]`).
- [X] T073 [US6] Escribir la prueba de integración del recorrido mínimo con el binario real (`start` → `define` → `approve` → `slice` → `tasks` → `check`/`run`/`report` → `verify` → `close`) y `HOME` aislado, que verifique además que no se escribe nada bajo `specs/` (SC-001), en `tests/integration/hcd_flow_test.go`.

- [X] T074 [US6] Ejecutar el quickstart, escenarios 1, 2 (sin el punto C-19, que llega en la fase 7), 3 y 4 (sin la comprobación de la memoria `type=decision`, que llega en la fase 6), con `/hcd` y la secuencia `hcd-*` en Claude Code (skills instaladas a mano en `~/.claude/skills/`), sin escribir comandos de consola, y guardar la transcripción en `specs/045-hcd-protocol/evidence/mvp-claude.md` (SC-001). **Hecha (2026-10-28): sesión real de Claude Code de 13 turnos, `HCD-001` de `start` a `completed`; destapó 12 defectos corregidos en la rama. Ver `evidence/mvp-claude.md`.**

**Checkpoint**: MVP completo (fases 1–5), con evidencia real de `/hcd` en Claude Code.

---

## Phase 6: User Story 4 — Memoria, unidad activa y heartbeat (Priority: P2)

**Goal**: decisiones en la memoria con procedencia, unidad activa compartida, heartbeat en el fin de turno y sección «HCD activo» al arrancar.

**Independent Test**: en esta fase, las pruebas automatizadas de la fase (incluida la integración con el contenedor real); la ejecución real de los escenarios 5 y 8 del quickstart se hace en la fase 9 (evidencia de entornos) y en el cierre (quickstart completo).

### Pruebas primero

- [X] T075 [P] [US4] Escribir pruebas: una decisión `accepted` crea una memoria `type=decision` con `topic_key hcd:{unit}:{D-NN}`, `filepath` del artefacto y, en su contenido, el tipo de unidad, la revisión, los elementos afectados, el origen y la fecha (FR-039); con `memory_sync=pending`, `close` se rechaza (caso límite de la spec; la excepción `HCD-MEM-01` se prueba en la fase 10); una prueba por tabla, con un caso por cada paso de FR-040b disponible en esta fase (enfoque desde `start`, aprobación, aclaración, cambio de alcance, `not_applicable`, bloqueo, aceptación y rechazo), verifica que se registra la decisión con su revisión y referencias; una sustitución crea `memory_relations(relation='supersedes')`; con el repositorio de memoria fallando → `memory_sync=pending`; `MemorySync` lo completa sin duplicar (C-12, C-13); la vista de progreso muestra las memorias pendientes de sincronización (FR-034), en `application/usecases/hcd_memory_test.go`.
- [X] T076 [P] [US4] Escribir pruebas de `Heartbeat`: sin `focus` → no-op; con `focus` registra sesión, cliente, etapa y archivos; archivos sin una ejecución `granted` → desvío `files_without_execution`; `granted` sin `report` → `execution_unreported`; nunca crea decisiones ni cambia `state_revision`; retención (último por sesión más los que tienen desvío); un turno sin archivos ni comandos también registra heartbeat; con reloj inyectado, una unidad activa sin heartbeat durante más de 7 días → `Status`/`Resume` muestran el desvío `inactive_unit` (FR-041b, FR-041c), en `application/usecases/hcd_focus_test.go`.
- [X] T077 [P] [US4] Escribir pruebas de la sección `## HCD activo` (contenido de `contracts/engine-cli.md` §6, ausente sin `focus`, dentro del presupuesto y deduplicada por el delta) en `application/usecases/build_context_hcd_test.go`.

- [X] T078 [P] [US4] Escribir pruebas de decisiones contradictorias (FR-042): dos decisiones `accepted` sobre los mismos elementos y sin `supersedes` → `Resume` las presenta como conflicto con sus fuentes y revisiones y añade el `pending_check` `DECISION-CONFLICT`, sin elegir la más reciente; las memorias reflejo que GoMemory marca como conflicto sin resolver se listan con la sugerencia de `judge_memories`; un conflicto funcional abre una aclaración para el Implementador, en `application/usecases/hcd_memory_conflict_test.go`.

### Implementación

- [X] T079 [US4] Añadir a `RecordDecision` el reflejo en memoria (memoria `type=decision` vía `MemoryRepository`, puente `memory_id`, relación `supersedes`) y `MemorySync`, en `application/usecases/hcd_memory.go`. Al ser el único camino de registro (fase 3), el reflejo cubre automáticamente todos los pasos de FR-040b ya conectados: selección de enfoque desde `start`, aprobación, aclaración, cambio de alcance, `not_applicable`, apertura y resolución de bloqueos, aceptación o rechazo de la entrega y cancelación. La selección desde la TUI se conecta en la fase 8, y la excepción, la adopción y la migración en sus fases (hace pasar T075).
- [X] T080 [US4] Implementar `Heartbeat` (sobre el `Focus` de la fase 2) con detección de desvíos y retención en `application/usecases/hcd_focus.go`, y llamar a la detección de `execution_unreported` también al cerrar la sesión (`hookSessionEnd` en `adapters/primary/cli/cmd_hook.go`) para detectar las ejecuciones que quedan sin reportar al terminar la sesión, aunque no haya otro fin de turno (hace pasar T076).
- [X] T081 [P] [US4] Escribir la prueba de rendimiento con una unidad de 50 tareas: `resume`, `status` y `next` responden en menos de 300 ms y el heartbeat en 200 ms o menos, con base SQLite real en `t.TempDir()`, en `application/usecases/hcd_perf_test.go` (metas del plan).
- [X] T082 [US4] Invocar `Heartbeat` best-effort (plazo `hcd_heartbeat_timeout_ms`, 200 ms por defecto; un fallo o vencimiento no bloquea y escribe en stderr `mem hook: hcd heartbeat: <causa>; se ignora`, como el resto de `cmd_hook.go`, §8; nunca aborta el hook) en `recordActivityCheckpointWithPayload`, justo después de calcular `activity` y antes de `if activity.empty() { return }`, sin depender de que se guarde el checkpoint, de `adapters/primary/cli/cmd_hook.go`, reutilizando el `turnActivity` ya calculado (la prueba cubre también el aviso en stderr ante un fallo del heartbeat y la ruta `subagent-stop` con `focus` activo, FR-041b), escribiendo primero su prueba en `adapters/primary/cli/cmd_hook_hcd_test.go`.
- [X] T083 [US4] Añadir la sección `## HCD activo` en `application/usecases/build_context.go` (lectura mediante el port `HCDContextRepository` ya declarado en la fase 2) y el encabezado en `deltaSections` de `application/usecases/session_delta.go`; además, una prueba de integración con el contenedor real (`infrastructure.NewContainer` sobre una base temporal con una unidad activa) confirma que `get_context` incluye la sección «HCD activo», para no depender solo del fake (hace pasar T077).
- [X] T084 [US4] Implementar la detección y presentación de decisiones contradictorias en `Resume` y `Status` y la apertura de la aclaración correspondiente en `application/usecases/hcd_memory.go` y `application/usecases/hcd_query.go` (hace pasar T078).
- [X] T085 [US4] Exponer `memory sync`, `heartbeat` y `decision` en `adapters/primary/cli/hcd/cmd_hcd.go`, registrar con `RecordDecision` el `not_applicable` de un criterio (con decisión citada, FR-028), y hacer que `resume` sin unidad use la activa e incluya las decisiones vinculadas a la revisión vigente y los desvíos (C-10), en `application/usecases/hcd_query.go`.
- [X] T086 [US4] Escribir la prueba de integración que confirma, comparando una instantánea del árbol del proyecto de prueba antes y después del recorrido completo, que ningún comando HCD crea ni modifica archivos fuera de `hcd/` (SC-011, FR-041a), en `tests/integration/hcd_no_agent_files_test.go`.

**Checkpoint**: retomar desde otra sesión muestra el contexto HCD sin releer la conversación.

---

## Phase 7: User Story 8 — El agente recibe solo trabajo autorizado (Priority: P2)

**Goal**: detención localizada, ejecución incierta y ningún estado de cumplimiento escrito desde fuera.

**Independent Test**: quickstart, escenario 2 (último punto) y la detención localizada de la spec, US8 escenario 2.

- [X] T087 [P] [US8] Escribir pruebas: una ejecución `granted` vencida (`--timeout`) → `uncertain`; `Run` de la misma tarea → `ErrUncertain`; la reconciliación (`report` tardío o `block --resolve`) sale de `uncertain` sin repetir (C-19), en `application/usecases/hcd_uncertain_test.go`.
- [X] T088 [P] [US8] Escribir la prueba de contrato que confirma que ningún comando acepta un estado de cumplimiento como entrada (`--state verified` o un campo de estado en `--file` → `usage` o se ignora con aviso) (FR-028, US8 escenario 4); `decision --accept` o `--supersede` sin cita válida → `invalid_quote`, y `decision --file` solo crea decisiones `proposed`, de modo que un agente no puede satisfacer `ATOM-DECISION` por su cuenta (FR-036, FR-017a); `approve`, `close` o `cancel` con `--client tui` → `not_allowed` (FR-017c), en `tests/contract/hcd_engine_contract_test.go`.
- [X] T089 [US8] Implementar el vencimiento de concesiones y la reconciliación en `application/usecases/hcd_delivery.go` (hace pasar T087).
- [X] T090 [US8] Rechazar o ignorar con aviso los campos de estado en los artefactos y en los flags en `adapters/secondary/hcdfiles/parse.go` y `adapters/primary/cli/hcd/cmd_hcd.go` (hace pasar T088).
- [X] T091 [US8] Añadir a `infrastructure/templates/hcd/hcd-implement/SKILL.md` la lista de detenciones de US8 escenario 2 y FR-036 (ampliación de alcance, excepción no aprobada, revisión incompatible, permiso insuficiente, resultado incierto) → `block` o `hcd-clarify` sobre la parte afectada.

---

## Phase 8: User Story 1 — Seleccionar el enfoque en la ceremonia de Especificación (Priority: P2)

**Goal**: pantalla «Especificación» en la TUI con HCD recomendado y Spec Kit legado; detección del enfoque y coexistencia.

**Independent Test**: quickstart, escenario 6 y la selección en la TUI sin crear artefactos del otro enfoque.

- [X] T092 [P] [US1] Escribir pruebas de la detección de enfoques (`specs/NNN-*` → `speckit`; `hcd/features/*` con fila → `hcd`; `hcd/features/*` sin fila, o con `spec.md` y `definition.md` a la vez → `unclassified`; nunca se escribe bajo `specs/`) (FR-002 a FR-004, C-02), en `application/usecases/hcd_approach_test.go`.
- [X] T093 [US1] Implementar `List` con detección de enfoques usando el `SpecKitReader` existente y `HCDArtifactRepository`, `approach_mismatch` en las mutaciones sobre unidades no HCD y `Classify` (decisión citada; nunca mueve ni edita archivos) de una unidad `unclassified`, y amplía `list` con las carpetas Spec Kit, en `application/usecases/hcd_approach.go` (hace pasar T092).
- [X] T094 [US1] Exponer `classify` en `adapters/primary/cli/hcd/cmd_hcd.go` y añadir su caso de contrato (FR-004) en `tests/contract/hcd_engine_contract_test.go`.
- [X] T095 [P] [US1] Escribir la prueba de la pantalla (las opciones «HCD Playbook — recomendado» y «Spec Kit — legado»; elegir HCD llama a `Start` del mismo caso de uso y registra la decisión de enfoque con `RecordDecision`; elegir Spec Kit no crea nada ni persiste la elección en HCD y muestra `/speckit-specify` o `/socratica-hu`, FR-001) en `adapters/primary/tui/specification_test.go`.
- [X] T096 [US1] Implementar `screenSpecification` al final del enum `screen` y su vista en `adapters/primary/tui/specification.go`, y su acceso desde la lista principal en `adapters/primary/tui/tui.go` (hace pasar T095).
- [X] T097 [US1] Añadir la pantalla a la documentación de la TUI que verifica `adapters/primary/tui/tui_docs_test.go`, en `adapters/primary/tui/tui_docs.go`.

---

## Phase 9: User Story 5 — Mismo protocolo desde agentes, TUI y terminal (Priority: P3)

**Goal**: skills instaladas en Claude Code, Codex y OpenCode; equivalencia entre adaptadores; compatibilidad solo con evidencia real.

**Independent Test**: quickstart, escenario 7.

- [X] T098 [P] [US5] Escribir pruebas de `InstallHCDSkills` (instala las 9 skills del MVP solo donde existe el directorio del agente; no reescribe un contenido igual; crea los envoltorios OpenCode `hcd*.md`) con `HOME` temporal en `adapters/primary/setup/hcd_skills_setup_test.go`.
- [X] T099 [US5] Implementar `InstallHCDSkills(bodies, wrappers)` sobre `InstallAgentSkill` y los envoltorios OpenCode en `adapters/primary/setup/hcd_skills_setup.go`, y llamarla desde `adapters/primary/cli/cmd_mcp_setup.go` junto a `InstallSocraticaSkill` (hace pasar T098).
- [X] T100 [P] [US5] Crear los envoltorios finos de OpenCode `infrastructure/templates/hcd/commands/hcd.md`, `hcd-define.md`, `hcd-clarify.md`, `hcd-approve.md`, `hcd-slice.md`, `hcd-tasks.md`, `hcd-implement.md`, `hcd-verify.md` y `hcd-close.md` (solo nombran la skill, sin copiar su cuerpo).
- [X] T101 [US5] Escribir la prueba de equivalencia que confirma que el mismo `Start` desde la TUI (`specification.go`) y desde `cmd_hcd.go` aplica las mismas precondiciones y produce el mismo estado, y que la TUI no expone aprobación (C-16, FR-017b), en `tests/integration/hcd_adapters_equivalence_test.go`.
- [X] T102 [US5] Ejecutar el quickstart, escenario 7, con `$hcd` en Codex y guardar la transcripción en `specs/045-hcd-protocol/evidence/env-codex.md` (si falla, registrar el bloqueo, no declarar soporte). **Hecha (2026-10-28): Codex 0.164.0 llevó `HCD-001` de `start` a `completed`. Requiere `~/.local/share/gomemory` como raíz escribible del sandbox. Ver `evidence/env-codex.md`.**
- [X] T103 [US5] Ejecutar el quickstart, escenario 7, con `/hcd` en OpenCode y guardar la transcripción en `specs/045-hcd-protocol/evidence/env-opencode.md` (mismo criterio). **Hecha (2026-10-28): OpenCode 2.1.4 llevó `HCD-001` de `start` a `completed` con artefactos por stdin. Ver `evidence/env-opencode.md`.**
- [X] T104 [US5] Ejecutar el quickstart, escenario 8, pasos 1–4, alternando los tres agentes (Claude Code, Codex y OpenCode), y guardar la evidencia del heartbeat y de la sección «HCD activo» en `specs/045-hcd-protocol/evidence/focus-heartbeat.md` (SC-011). **Hecha (2026-10-28): los cinco pasos alternando OpenCode, Claude Code y Codex; `$hcd` en Codex quedó limitado por el `mem` global del entorno. Ver `evidence/focus-heartbeat.md`.**

---

## Phase 10: User Story 6 (ampliación) — Precedencia, excepciones y revisiones del Playbook (US6 es P1; esta ampliación se entrega como P3)

**Goal**: excepciones con vencimiento y adopción explícita de revisiones del Playbook.

**Independent Test**: una excepción vencida bloquea el cierre; una revisión nueva del Playbook no cambia una unidad activa sin `adopt`.

- [X] T105 [P] [US6] Escribir pruebas de las excepciones (campos de FR-037, incluidos responsable y vencimiento o condición de revisión, al menos uno; excepción de `HCD-MEM-01` que permite cerrar con `memory_sync=pending`; una revisión nueva del Playbook o de la Constitución marcada como obligación urgente de seguridad aparece en `status` de todas las unidades activas como adopción requerida; aprobación citada `object=exception:E-NN`, vencida → bloquea el cierre) y de la adopción del Playbook (nueva revisión → la unidad conserva la suya; `adopt` muestra el impacto y requiere aprobación) (C-14, C-15); un caso por cada conflicto estructural de FR-013 que termina en `blocked` (sección constitucional citada inexistente; regla que declara relajar, sustituir o exceptuar una regla `§N`; texto de la Constitución copiado) y un caso de conflicto de sentido que solo genera aviso; una regla `non_exceptionable` no admite excepción, en `application/usecases/hcd_governance_test.go`.
- [X] T106 [P] [US6] Escribir pruebas de la precedencia normativa (FR-011): seguridad > Constitución > Playbook > decisiones y excepciones de la unidad > definición > slices y tareas; una decisión de la unidad que contradice una regla superior se rechaza con `ErrBlocked` salvo que exista una excepción aprobada y vigente de esa misma regla; una excepción no puede relajar una regla de seguridad marcada como no exceptuable; para una regla de origen `§N` solo cuenta una excepción de la §25 de la Constitución (una fila de `hcd_exceptions` no basta y se rechaza al intentar registrarla contra una regla constitucional); `hcd_exceptions` solo aplica a reglas de origen `playbook` (FR-011, §23), en `domain/hcd_governance_test.go`.
- [X] T107 [US6] Implementar `NormativeLevel` y `CanOverride(decision, rule, exceptions)` en `domain/hcd_governance.go` y aplicarlo al registrar decisiones y excepciones (hace pasar T106).
- [X] T108 [US6] Implementar `Exception`, `PlaybookShow` (vía `ResolvePinnedDoc`), `PlaybookValidate` (formato de reglas; presencia de las reglas `HCD-CLEAN-*` y `HCD-SOLID-*` con comprobación declarada; una regla que declare relajar, sustituir o exceptuar una regla `§N` es conflicto estructural y bloquea la adopción; cada sección citada existe en la Constitución resuelta; ningún bloque copiado de ella) y `PlaybookAdopt` (al adoptar, pide con la cita del Implementador el `reason` de la revisión nueva —importación, restauración o actualización de la plantilla— y registra `reason` y `actor` en `hcd_playbook_adoptions`, FR-012; excepción y adopción registran su decisión con `RecordDecision`, cubierto en T105) (diferencias entre la revisión fijada, leída de `hcd_normative_revisions`, y la resuelta actual) en `application/usecases/hcd_governance.go` (hace pasar T105).
- [X] T109 [US6] Exponer `exception` y `playbook show|validate|adopt` en `adapters/primary/cli/hcd/cmd_hcd.go`, y redactar `infrastructure/templates/hcd/hcd-exception/SKILL.md` e `infrastructure/templates/hcd/hcd-playbook/SKILL.md`; añadir `hcd-exception` y `hcd-playbook` a `InstallHCDSkills`, sus envoltorios OpenCode en `infrastructure/templates/hcd/commands/` y su caso en la prueba de instalación.
- [X] T110 [US6] Documentar en `infrastructure/templates/hcd/hcd-playbook/SKILL.md` el flujo de «propuesta» del Playbook: `mem docs export playbook` → editar → `mem docs import` (revisión nueva sin afectar unidades activas) → `hcd-playbook adopt` por unidad con diferencias y cita (R18). Probarlo de extremo a extremo en `tests/integration/hcd_playbook_adopt_test.go`.

---

## Phase 11: User Story 7 — Migración voluntaria desde Spec Kit (Priority: P4)

**Goal**: propuesta clasificada sin inventar aprobaciones; activación exacta y recuperable.

**Independent Test**: migrar una copia de `specs/044-cli-socratica-slice` en un repositorio de prueba.

- [X] T111 [P] [US7] Escribir pruebas: `MigratePreview` clasifica cada elemento de `spec.md`, `plan.md` y `tasks.md` como migrado, consolidado, referencia, descartado con justificación o pendiente; las tareas `[x]` no quedan verificadas; la unidad sigue `speckit` (C-20); una fuente cambiada tras la propuesta → `MigrateActivate` exige una nueva propuesta; un fallo a mitad → un único enfoque operativo (C-21), en `application/usecases/hcd_migrate_test.go`.
- [X] T112 [US7] Implementar `MigratePreview` y `MigrateActivate` (la activación registra su decisión con `RecordDecision`, cubierto en T111; transacción única, fuentes como `legacy-reference`, sin escribir bajo `specs/`) en `application/usecases/hcd_migrate.go` (hace pasar T111).
- [X] T113 [US7] Exponer `migrate --preview|--activate` en `adapters/primary/cli/hcd/cmd_hcd.go` y redactar `infrastructure/templates/hcd/hcd-migrate/SKILL.md`; añadir `hcd-migrate` a `InstallHCDSkills`, su envoltorio OpenCode y su caso en la prueba de instalación.
- [X] T114 [US7] Ejecutar la migración representativa en un repositorio de prueba y guardar la evidencia en `specs/045-hcd-protocol/evidence/migration-044.md` (SC-009).

---

## Phase 12: Polish & Cross-Cutting

- [X] T115 [P] Documentar HCD (qué es, `/hcd`, `$hcd`, la secuencia, qué ofrece un entorno solo MCP —consulta mediante la sección «HCD activo» de `get_context` y asistencia, sin ejecución gestionada—, la unidad activa y el heartbeat, qué no garantiza, el riesgo de la cita) en `README.md` y en la guía técnica vigente `docs/Guia-Tecnica_GOMEMORY-v2.40.html`, declarando como compatibles solo los entornos con evidencia (T102–T104).
- [X] T116 [P] Actualizar `docs/ARQUITECTURA.md` con el slice HCD: capas y ports (`HCDRepository`, `HCDUnitOfWork`, `HCDArtifactRepository`, `HCDContextRepository`), reparto de fuentes de verdad (artefactos, estado de control y `verification.md` generado), flujo skill → motor → estado de control, heartbeat en `turn-end` y sección «HCD activo» del contexto de arranque (§18).
- [X] T117 [P] Añadir la entrada `### Added` de HCD en `CHANGELOG.md`.
- [X] T118 Ejecutar `go test ./...`, `go vet ./...` y la cobertura del repositorio, y confirmar que la línea base no baja (§10). Guardar el resumen en `specs/045-hcd-protocol/evidence/full-suite.md`.
- [X] T119 Ejecutar el quickstart, escenario 9 (unidades `feature`, `evolution` y `technical` de principio a fin), y guardar la evidencia en `specs/045-hcd-protocol/evidence/kinds-e2e.md` (SC-002).
- [X] T120 [P] Escribir la prueba de regresión que confirma que `socratica_flow(mode=specify)` y `mem socratica` conservan su comportamiento y no crean filas `hcd_*` ni archivos bajo `hcd/` (FR-051), en `tests/integration/hcd_socratica_untouched_test.go`.
- [X] T121 Generar `specs/045-hcd-protocol/evidence/criteria-matrix.md` con una fila por C-01…C-24 y por SC-001…SC-011: prueba o escenario que lo cubre, evidencia y resultado. Ninguna fila sin cobertura (SC-003; SC-010 se justifica por construcción).
- [X] T122 Ejecutar el quickstart completo (escenarios 1–9) con el binario compilado desde la rama y guardar `specs/045-hcd-protocol/evidence/quickstart-run.md`. **Hecha (2026-10-28): escenarios 1–9 con el binario y con los tres agentes. C-19 (requiere esperar 1 h de concesión) y el fallo aislado de memoria quedan cubiertos solo por pruebas automáticas, declarado en `evidence/quickstart-run.md`.**
- [X] T123 Ampliar la prueba de `adapters/primary/setup/hcd_skills_test.go` para exigir que **cada** comando mutante de `contracts/engine-cli.md` (salvo `heartbeat`, interno) aparezca en al menos una `SKILL.md` (`contracts/skills.md` §6, FR-045), una vez implementadas las fases 6 a 11.
- [X] T124 Confirmar que Spec Kit no cambió (C-24): `git diff --stat <SHA de baseline.md> -- specs/ ':(exclude)specs/045-hcd-protocol' adapters/secondary/speckit` sin cambios, y las huellas de `.claude/skills/speckit-*`, `.agents/skills/speckit-*` y `.opencode/commands/speckit*` iguales a las de `evidence/baseline.md` (no están en git). `mem hcd list --json` muestra `approach: speckit` en el 100 % de las carpetas `specs/001…044` (SC-004). `.specify/memory/constitution.md` cambia solo por la §18 y la §25.2 aprobadas (excepción explícita de FR-006), con su diff registrado aparte. Guardar todo en `specs/045-hcd-protocol/evidence/speckit-untouched.md`. **Hecha (2026-10-28): sin diff en `specs/` ni en el adaptador; 11 huellas `.opencode` coinciden; los 21 archivos de skills de `.claude`/`.agents` no cambiaron desde antes de la línea base (fechas de modificación) y quedan sus huellas; 45/45 carpetas `approach: speckit`; la constitución cambia solo por la §18/§25.2 aprobadas. Ver `evidence/speckit-untouched.md`.**

---

## Dependencies & Execution Order

### Fases

- **Phase 1** (gobierno): T004 es una puerta manual. T006 y T007 pueden empezar en paralelo a T004, pero ninguna unidad HCD real se crea antes de T005.
- **Phase 2**: depende de T007 (plantillas). Bloquea todas las historias.
- **ADR antes de aplicar la Constitución**: la tarea del ADR 0002 va antes de la que aplica la §18 y la §25.2, porque la §25.2 enlaza ese ADR.
- **US2 (Phase 3)** → **US3 (Phase 4)** → **US6 (Phase 5)**: secuenciales, porque forman un recorrido (aprobar → ejecutar → verificar). Es el MVP.
- **US4 (Phase 6)**: depende de US2, US3 (`block`) y US6 (`close` y `close --reject`), cuyas decisiones registra.
- **US8 (Phase 7)**: depende de US3, de US6 (`close` y `cancel`) y de la exposición de `decision` de la fase 6.
- **US1 (Phase 8)**: depende de la Phase 2 y de `Start` (T040); es independiente de US3 y US6.
- **US5 (Phase 9)**: depende de las skills del MVP (T048–T051, T060–T062, T071–T072) y de US1 para T101.
- **Phase 10**: depende de US6. **US7 (Phase 11)**: depende de US2 y US6.
- **Polish**: al final.

### Dentro de cada historia

Las pruebas primero (deben fallar), después el dominio, el caso de uso, la CLI y, al final, las skills. Las skills `[P]` de una misma fase son archivos distintos.

## Parallel Examples

```text
# Phase 2, pruebas a la vez:
T011 domain/hcd_test.go · T012 domain/hcd_revision_test.go · T013 persistence/hcd_test.go · T014 golden · T015 hcdfiles · T016 contrato

# US2, skills a la vez (después de T046):
T048 hcd · T049 hcd-define · T050 hcd-clarify · T051 hcd-approve

# US4, pruebas a la vez:
T075 hcd_memory_test · T076 hcd_focus_test · T077 build_context_hcd_test
```

## Implementation Strategy

### MVP (Phases 1–5)

1. Gobierno: §18 aprobada y aplicada, Playbook y plantillas.
2. Fundamentos: dominio, esquema v4, store, artefactos y esqueleto `mem hcd --json`.
3. US2 → US3 → US6, con el recorrido mínimo por `/hcd` en Claude Code.
4. **PARAR Y VALIDAR**: quickstart, escenarios 1–4, con evidencia en `evidence/mvp-claude.md`.

### Incrementos

5. US4 (memoria, unidad activa y heartbeat) → validar los escenarios 5 y 8.
6. US8 → US1 → US5 (multientorno con evidencia real) → gobierno ampliado → US7.
7. Cada incremento se valida contra el binario real antes de darlo por terminado.

## Notes

- No quedan hojas no atómicas: la «propuesta» del Playbook se resolvió con el flujo de documento fijado (R18).
- Cada tarea que termina en «guardar evidencia» produce un archivo bajo `specs/045-hcd-protocol/evidence/`.
- No hacer commit sin mostrar antes la lista de archivos (regla del Implementador). `CLAUDE.md` y los archivos de IA quedan fuera.
