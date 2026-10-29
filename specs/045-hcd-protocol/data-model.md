# Data Model: HCD

**Feature**: `045-hcd-protocol` · Fuente: [spec.md](./spec.md), decisiones en [research.md](./research.md)

El modelo tiene dos lados:

- **Contenido redactado** en artefactos versionados (`hcd/features/{id}-{name}/definition.md` y `delivery.md`).
- **Estado de control** en tablas SQLite `hcd_*` de la base del proyecto.

`verification.md` se genera desde el estado de control.

## 1. Entidades de dominio (`domain/hcd*.go`)

### Unit (unidad de trabajo)

| Campo | Tipo | Regla |
|---|---|---|
| ID | string | `HCD-NNN`, secuencial por proyecto, único. |
| Kind | enum | `feature`, `evolution`, `fix`, `technical` (FR-014). |
| Title, Slug | string | El slug define la carpeta `{NNN}-{slug}`. |
| Approach | enum | `hcd`, `speckit`, `unclassified` (FR-002). Una unidad HCD siempre es `hcd`. |
| State | enum | `draft`, `ready`, `in_progress`, `verification`, `completed`, `blocked`, `cancelled`, `superseded`. |
| StateRevision | int | Monotónico; es el `expected_revision` de las mutaciones (R4). |
| DefinitionRev, DeliveryRev | string | Hash de contenido (12 hex) del último contenido registrado. |
| ApprovedRev | string | `DefinitionRev` aprobada; vacía si no hay aprobación. |
| PlaybookRev, ConstitutionRev | string | Revisiones normativas fijadas al crear o adoptar (FR-012). |
| Implementer | string | Autor de git o `$USER`; es el único actor humano. |

### Story, Criterion (definición)

- **Story**: `ID (USn)`, título, actor, texto y estado (`draft`, `ready`, `in_progress`, `satisfied`, `blocked`, `rejected`). Necesita al menos un criterio.
- **Criterion**: `ID (C-NN)`, historias, condición observable, estrategia de verificación (`automated`, `manual`, `inspection`, `review`, `real-system`) y estado (`defined`, `covered`, `verified`, `failed`, `blocked`, `not_applicable`). `not_applicable` exige una decisión vinculada.

### Slice, Task (entrega)

- **Slice**: `ID (VS-NN)`, comportamiento observable, historias, criterios, límites, impacto conocido, verificación esperada, incertidumbres, dependencias y estado (`proposed`, `ready`, `in_progress`, `implemented`, `verified`, `blocked`, `rejected`).
- **Task**: `ID (T-NNN)`, slice, historias, criterios, tipo (`implementation`, `test`, `integration`, `documentation`, `migration`, `playbook`, `enabling`, `scope-change`), resultado, alcance, excluido, verificación, evidencia esperada, dependencias, reglas y estado (`pending`, `in_progress`, `implemented`, `verified`, `blocked`, `cancelled`).

**Regla de atomicidad** (FR-021): la función de dominio `AtomicityViolations(task, unit, clarifications)` devuelve las condiciones incumplidas:

| Código | Condición incumplida |
|---|---|
| `ATOM-VERB` | Título sin verbo y objeto. |
| `ATOM-RESULT` | Sin resultado, o con varios resultados enumerados. |
| `ATOM-VERIFY` | Sin verificación. |
| `ATOM-INPUTS` | Dependencias inexistentes. |
| `ATOM-SCOPE` | Sin alcance o sin exclusiones declaradas. (La condición «ejecutable como unidad enfocada» de FR-021 no es estructural: se comprueba en revisión técnica.) |
| `ATOM-PENDING` | Aclaración bloqueante abierta sobre la tarea. |
| `ATOM-LINK` | Slice, historia o criterio inexistentes (C-05). |
| `ATOM-RULES` | Sin reglas. |
| `ATOM-ENABLING` | Tarea habilitante sin slice desbloqueado ni justificación. |
| `ATOM-DECISION` | Tarea `scope-change` sin una decisión `accepted` vinculada (respaldo de FR-036). |

### Rule (Playbook)

`ID (HCD-XXX-NN)`, descripción, ámbito, comprobación (`structural` cuando la aplica el motor, `review` cuando la juzga una persona), evidencia esperada, política de excepción, origen (`§N` constitucional o `playbook`) y `non_exceptionable` (verdadero para reglas de seguridad que ninguna excepción puede relajar, FR-011). Una revisión del Playbook o de la Constitución puede declarar en su encabezado `Urgente: seguridad`; el motor la guarda como `urgent` y la muestra como adopción requerida en todas las unidades activas.

## 2. Transiciones de la unidad

| Desde | Acción | Precondición | Hacia |
|---|---|---|---|
| — | `start` | Tipo y título válidos; no existe la carpeta | `draft` |
| `draft` | `define` | Archivo parseable | `draft` (nueva `DefinitionRev`) |
| `draft` | `approve(definition)` | `validate` PASS, sin aclaraciones bloqueantes, cita válida, `rev == DefinitionRev` | `ready` |
| `ready` | `define` (cualquier cambio de `definition_rev` es cambio de alcance, FR-017) | — | `draft` (la aprobación anterior deja de autorizar; C-04) |
| `ready` | `run` (primera tarea) | `check` PASS | `in_progress` |
| `in_progress` | `verify` (unidad) | Todos los slices comprometidos `verified` | `verification` |
| `verification` | `close` | Verificación PASS vigente, cita válida, `rev == ApprovedRev == DefinitionRev` (C-23) | `completed` |
| cualquiera no terminal | `block --critical` sobre la unidad / su resolución | Justificación | `blocked` / estado previo, si sigue siendo válido |
| `ready`, `in_progress` | `block` sobre una tarea o un slice | Justificación | sin cambio de estado de la unidad: solo el elemento afectado pasa a `blocked` (bloqueo localizado, US3-4) |
| `in_progress`, `verification` | `define` o edición externa de `definition.md` (nueva `definition_rev`) | — | `draft` (la aprobación anterior deja de autorizar; las tareas no iniciadas quedan sin autorización) |
| `verification` | `close --reject` | Cita válida | `in_progress` |
| `verification` | caducidad de evidencia por un cambio que no es de la definición (entrega, código evaluado o revisión normativa) | — | `in_progress` (la verificación afectada queda pendiente de revalidación) |
| cualquiera no terminal | `cancel` | Cita válida | `cancelled` |
| cualquiera no terminal | `cancel --superseded-by HCD-MMM` | Cita válida; la unidad HCD-MMM existe | `superseded` |

**Volver a `draft` es definitivo hasta una nueva aprobación:** aunque se restaure el contenido anterior de `definition.md`, la unidad sigue en `draft` y necesita `approve` de la revisión vigente. No hay atajo de restauración. Mientras está en `draft`, `close` responde `not_allowed` (la acción no está en `allowed_actions`).

**Ejecuciones en curso al volver a `draft`:** las ejecuciones `granted` pierden la autorización. Un `report` posterior se registra como evidencia histórica, marcada `stale` y sin efecto sobre el estado. Las `uncertain` siguen `uncertain` hasta reconciliarlas, sin poder concederse de nuevo hasta una nueva aprobación.

**Etapa (`stage`) según el estado** (FR-018):

| Situación | `stage` |
|---|---|
| Sin unidad activa ni unidad indicada | `context` |
| `draft` sin intención completa (Problema, Actor o Resultado esperado vacíos) | `intent` |
| `draft` con intención completa | `definition` |
| `draft` con una aclaración abierta | `clarification` |
| `draft` con `validate` PASS | `approval` |
| `ready` o `in_progress` sin slice `ready` pendiente | `slicing` |
| slice `ready` con tareas sin admitir | `decomposition` |
| tarea admitida o ejecución `granted` | `execution` |
| `verification` | `verification` |
| `in_progress` con todos los slices comprometidos verificados y trabajo restante por decidir | `continuity` |
| `completed`, `cancelled` o `superseded` | `closure` |

Los estados terminales (`completed`, `cancelled`, `superseded`) no aceptan mutaciones, salvo consultas. Los estados de historia, criterio y slice se **derivan** (FR-028): un criterio es `verified` si tiene al menos una evidencia `pass` vigente y ninguna `fail` vigente; un slice es `verified` si todas sus tareas lo están y todos sus criterios están `verified` o `not_applicable` con decisión.

## 3. Tablas de estado de control (`adapters/secondary/persistence/hcd_schema.go`)

Todas tienen `project TEXT NOT NULL`, timestamps RFC3339 UTC y SQL parametrizado (§5). El esquema es solo aditivo y `schemaVersion` sube de 3 a 4 (§25.1).

| Tabla | Columnas clave | Unicidad / índices | Propósito |
|---|---|---|---|
| `hcd_units` | `id`, `kind`, `title`, `slug`, `approach`, `state`, `prev_state`, `state_revision`, `definition_rev`, `delivery_rev`, `approved_rev`, `playbook_rev`, `constitution_rev`, `implementer` | PK `(project, id)` | Agregado raíz. |
| `hcd_operations` | `operation_id`, `unit_id`, `command`, `payload_hash`, `result_json` | PK `(project, operation_id)` | Idempotencia (C-18). |
| `hcd_approval_prompts` | `id`, `unit_id`, `object`, `revision`, `options_json`, `asked_at`, `consumed` | Índice `(unit_id, object)` | Pregunta de aprobación con opciones registrada por `approve --request`; permite aceptar «A» o «recommended» solo como respuesta a ella (R7, B1). |
| `hcd_approvals` | `id`, `unit_id`, `object` (`definition`, `exception:E-01`, `close`, `reject`, `cancel`, `classify:<carpeta>`, `adopt:<doc>`, `decision:D-01`, `migration:M-01`), `revision`, `origin` (`conversation`, `terminal`), `quote`, `client`, `approved_at` | Índice `(unit_id, object)` | FR-017 y FR-017a. |
| `hcd_clarifications` | `id`, `unit_id`, `target_ref`, `question`, `options_json`, `recommendation`, `answer`, `status`, `blocking` | Índice `(unit_id, status)` | FR-016a. |
| `hcd_decisions` | `id (D-NN)`, `unit_id`, `kind`, `status` (`proposed`, `accepted`, `rejected`, `superseded`), `summary`, `revision`, `refs_json`, `source`, `memory_id`, `memory_sync` (`synced`, `pending`) | Índice `(unit_id)` y `(memory_sync)` | FR-038, FR-039, FR-040b, FR-043. |
| `hcd_exceptions` | `id (E-NN)`, `unit_id`, `rule_id` (solo reglas de origen `playbook`, FR-011), `scope`, `reason`, `alternatives`, `risks`, `compensating`, `owner`, `approval_id`, `status`, `expires_at`, `review_condition` (FR-037: vencimiento o condición de revisión, al menos uno) | Índice `(unit_id, status)` | FR-037. |
| `hcd_blocks` | `id (B-NN)`, `unit_id`, `target_ref`, `reason`, `critical`, `status`, `resolution` | Índice `(unit_id, status)` | Bloqueo localizado. |
| `hcd_executions` | `id (X-…)`, `unit_id`, `task_id`, `definition_rev`, `delivery_rev`, `state` (`granted`, `reported`, `uncertain`, `cancelled`), `granted_at`, `reported_at`, `result_hash` | Índice `(unit_id, task_id)` | `run`/`report` (R8, C-19). |
| `hcd_evidence` | `id (EV-NN)`, `unit_id`, `execution_id`, `covers_json` (tareas y criterios), `procedure`, `environment`, `result` (`pass`, `fail`), `subject_ref`, `inspect_ref`, `definition_rev`, `delivery_rev`, `playbook_rev`, `constitution_rev`, `executor`, `recorded_at` | Índice `(unit_id)` | FR-030; la vigencia se calcula (R10). |
| `hcd_links` | `from_kind`, `from_id`, `to_kind`, `to_id`, `relation` (`covers`, `belongs_to`, `depends_on`, `verifies`, `justifies`, `touches`), `unit_id` | Única `(project, unit_id, from_kind, from_id, to_kind, to_id, relation)` | Grafo de trazabilidad (FR-040a, R3). |
| `hcd_normative_revisions` | `doc` (`playbook`, `constitution`), `revision`, `content`, `first_pinned_at`, `source` (`default`, `project`), `change_kind` (`import`, `restore`, `seed_upgrade`, `initial`), `urgent` | PK `(project, doc, revision)` | Contenido de cada revisión fijada por alguna unidad, para mostrar diferencias al adoptar (FR-012). Solo se inserta, nunca se modifica. |
| `hcd_focus` | `project`, `unit_id`, `client`, `session_id`, `set_at`, `set_by` (`start`, `resume`, `tui`) | PK `(project)` | Unidad activa compartida por todos los agentes (FR-041a, R15). |
| `hcd_heartbeats` | `id`, `unit_id`, `session_id`, `client`, `stage`, `state_revision`, `files_json`, `drift_json`, `seen_at` | Índice `(unit_id, seen_at)` | Heartbeat de fin de turno y desvíos (FR-041b, FR-041c, R16). Retención: el último por sesión más los que tienen desvío. |
| `hcd_playbook_adoptions` | `id (AD-NN)`, `unit_id`, `doc` (`playbook`, `constitution`), `from_rev`, `to_rev`, `impact_json`, `reason`, `actor`, `approval_id`, `adopted_at` | PK `(project, id)`; índice `(unit_id, doc)` | Adopción explícita por unidad; único lugar donde se registran motivo y responsable de una revisión nueva (FR-012, C-14). |
| `hcd_migrations` | `id (M-NN)`, `unit_id` (vacío hasta la activación), `source_dir`, `source_rev`, `proposal_json`, `status`, `created_at`, `activated_at` | PK `(project, id)`; índice `(source_dir)` | US7 (incremento posterior). |

**Puentes con el grafo existente**:

- `hcd_decisions.memory_id` apunta a `memories(id)`. Las memorias se guardan con `type=decision`, `topic_key=hcd:{unit}:{D-NN}` y `filepath` igual a la ruta del artefacto.
- Una decisión que sustituye a otra crea `memory_relations(relation='supersedes')`.
- Las aristas `touches` (tarea → archivo) solo se escriben si `code_files` contiene la ruta.

## 4. Artefactos versionados

```text
hcd/
├── hcd-playbook.md                  # OPCIONAL: exportación (--sync) del documento fijado `playbook`; no es la fuente
└── features/
    └── {NNN}-{slug}/
        ├── definition.md            # contenido redactado (fuente de verdad del alcance)
        ├── delivery.md              # slices y tareas
        └── verification.md          # GENERADO por `mem hcd verify`; no se edita
```

El formato parseable está en [contracts/artifact-format.md](./contracts/artifact-format.md).

## 5. Invariantes verificables

1. Una unidad tiene un solo enfoque activo.
2. Una tarea `verified` tiene al menos una evidencia `pass` vigente.
3. Un criterio `verified` tiene cobertura.
4. Una excepción vencida no satisface su control.
5. Una unidad con un bloqueo crítico abierto no puede estar `completed`.
6. Una migración propuesta no activa HCD.
7. Toda acción que exige cita (aprobación, cierre, rechazo, cancelación, clasificación, adopción, aceptación o sustitución de decisiones y activación de migración) queda en `hcd_approvals` con su cita, origen y revisión; la decisión correspondiente referencia ese `approval_id`.
8. Una consulta no cambia `state_revision`.
9. Un `operation_id` corresponde a un único `payload_hash`.
10. Hay como máximo una unidad activa por proyecto, y nunca es una unidad terminal.
11. Un heartbeat nunca crea decisiones, aprobaciones ni cambios de estado de la unidad: solo registra actividad y desvíos.
12. Fuera de la base de GoMemory, HCD solo escribe bajo `hcd/`: ningún archivo de contexto del agente ni marcador `SPECKIT`.
