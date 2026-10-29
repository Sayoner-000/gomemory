# Research: HCD — protocolo propio de especificación y desarrollo continuo

**Feature**: `045-hcd-protocol` · **Fecha**: 2026-10-27 · **Spec**: [spec.md](./spec.md)

Cada decisión se apoya en código existente del repositorio, verificado durante la planificación.

## R1. Motor del harness: CLI `mem hcd` como slice propio

- **Decisión**: el motor es un slice de CLI nuevo `adapters/primary/cli/hcd`, con el mismo patrón que `cli/review` y `cli/socratica` (features 043 y 044): `Run(deps *clikit.Deps, args []string) int`, enrutado desde `adapters/primary/cli/dispatcher.go`, y salida `--json` estable para las skills.
- **Justificación**: la persona eligió que las skills llamen por dentro a `mem hcd … --json` (Clarifications, Q2). El patrón de slice ya está probado y protegido por `tests/contract/layer_rules_test.go`.
- **Alternativas descartadas**: ampliar `socratica_flow` o crear una herramienta MCP `hcd_flow`, porque la spec excluye nuevas herramientas MCP y FR-051 prohíbe reutilizar la entrada Socrática; también skills que solo editan Markdown, porque no pueden aplicar idempotencia ni revisiones (C-17, C-18).

## R2. Estado de control en SQLite con versión de esquema aditiva

- **Decisión**: tablas nuevas `hcd_*` en la base SQLite por proyecto que ya abre `persistence`, declaradas en `adapters/secondary/persistence/hcd_schema.go` (igual que `socratica_schema.go`), creadas en la preparación transaccional de `db.go` y con `schemaVersion` subido de 3 a 4.
- **Justificación**: es la decisión de la persona (Q4). La excepción vigente §25.1 cubre la migración al abrir, siempre que el esquema sea solo aditivo, se prepare en una transacción y exista el golden del catálogo. HCD solo añade tablas e índices.
- **Alternativas descartadas**: una base separada `hcd.db`, porque duplica la conexión, la retención y el respaldo, y rompe el enlace con `memories`; JSON en archivos, porque no da transacciones ni unicidad para la idempotencia.

## R3. «Aprovechar el grafo de GoMemory»

- **Hallazgo**: `memory_relations` solo enlaza memorias (claves foráneas a `memories(id)`). El grafo de código (`code_nodes`/`code_edges`) solo enlaza símbolos. Ninguno de los dos admite nodos HCD sin romper sus restricciones.
- **Decisión**: tres capas que se tocan sin mezclarse:
  1. `hcd_links`: tabla de aristas entre elementos HCD (historia → criterio → slice → tarea → control → evidencia), con la forma de `memory_relations` (`project`, extremos tipados, `relation`, `created_at`, índice único por par). Es la que se recorre para la trazabilidad (FR-040a).
  2. Las decisiones con valor de memoria (FR-038) se guardan también como memorias `decision` mediante el `MemoryRepository` existente, con `topic_key = hcd:{unit}:{decision}`. `hcd_decisions.memory_id` es el puente.
  3. La sustitución entre decisiones se registra como relación `supersedes` en `memory_relations`, así las sinapsis y el juez imparcial (`judge_memories`) existentes ya las ven.
  4. Opcional y sin bloquear el MVP: aristas `touches` de tarea a archivo indexado, que solo se escriben si el grafo de código cubre la ruta.
- **Alternativas descartadas**: generalizar `memory_relations` a nodos polimórficos, porque es un cambio no aditivo sobre una tabla con 1000+ filas en uso; un grafo HCD dentro del índice de código, porque su ciclo de reindexado borraría el estado de control.

## R4. Dos nociones de revisión

- **Decisión**:
  - **Revisión de contenido** (`definition_rev`, `delivery_rev`, `playbook_rev`): SHA-256 del contenido normalizado (finales de línea LF, sin espacio final) del artefacto, abreviado a 12 hexadecimales. Es la que se aprueba, se fija en las evidencias y se cierra (C-04, C-09, C-23).
  - **Revisión de estado** (`state_revision`): entero monotónico por unidad, como el `revision` de `socratica_flows`. Es el `expected_revision` de las mutaciones (C-17).
- **Justificación**: una aprobación debe atarse al contenido, no al orden de escritura. El conflicto de concurrencia se detecta mejor con un contador. Las dos nociones ya existen en Socrática (`approved_hash` y `revision`).
- **Revisión del Playbook y de la Constitución**: `ContentFingerprint` (la huella SHA-256 que ya usa el catálogo de documentos fijados) del contenido **resuelto** (el del proyecto o el valor por defecto), abreviado a 12 hexadecimales, más su origen (`default` o `project`). Es reproducible sin depender de git, porque la fuente es la memoria y no un archivo (R18).

## R5. Formato de artefactos: Markdown con campos parseables, sin dependencias

- **Decisión**: `definition.md` y `delivery.md` en Markdown, con encabezados de ID (`## US1.`, `### C-01.`, `## VS-01.`, `### T-001.`) y líneas `Clave: valor` inmediatamente debajo. El parser vive en un adaptador secundario (`adapters/secondary/hcdfiles`), escrito con la biblioteca estándar.
- **Justificación**: la §20 prohíbe añadir dependencias sin necesidad. La plantilla de tarea de `contracts/artifact-format.md` ya usa este formato, y la persona lo lee y edita sin herramientas.
- **Alternativas descartadas**: frontmatter YAML, porque exige una dependencia de parser; JSON, porque no es legible para la persona.

## R6. Skills: `/hcd`, `$hcd` y la secuencia `hcd-*`

- **Decisión**: plantillas canónicas en `infrastructure/templates/hcd/`, una por skill, instaladas con `setup.InstallAgentSkill`, que ya cubre Claude Code, Codex y OpenCode y solo escribe donde existe el directorio del agente y el contenido difiere. Para OpenCode se añaden envoltorios de comando finos, igual que `socraticaWrapperTargets`.
- **Secuencia (MVP)**: `hcd` (entrada: iniciar, retomar, siguiente acción, estado), `hcd-define`, `hcd-clarify`, `hcd-approve`, `hcd-slice`, `hcd-tasks`, `hcd-implement` (`run` + `report`), `hcd-verify` y `hcd-close`.
- **Secuencia (incrementos posteriores)**: `hcd-exception`, `hcd-playbook` y `hcd-migrate`.
- **Invocación**: Claude Code expone cada skill como `/hcd`, `/hcd-define`…; Codex, como `$hcd`, `$hcd-define`…; OpenCode, mediante `~/.config/opencode/commands/hcd*.md`.
- **Regla de las skills (FR-045b)**: cada skill termina toda mutación en una llamada al motor y presenta su JSON. Si el motor rechaza, la skill muestra el rechazo y no edita para compensar. Si `mem` no está en PATH, prueba `./mem` y, si tampoco existe, informa la limitación sin simular el avance.
- **Alternativas descartadas**: una única skill con subacciones, porque la persona pidió explícitamente una secuencia al estilo Spec Kit; copiar la mecánica de los scripts `.specify`, porque es un segundo motor con su propia máquina de estados (FR-050).

## R7. Aprobación conversacional citada

- **Decisión**: `mem hcd approve <unit> --revision <rev> --quote <texto> --client <agente> [--object definition|exception:E-01|close|migration:M-01]`. El motor rechaza `--quote` vacío o sin relación con la acción (sin la palabra de aprobación ni la revisión mostrada) y guarda origen, cita, cliente, fecha y revisión.
- **Justificación**: es la decisión de la persona (Q5). El riesgo residual se declara en el Playbook y en el informe (FR-017a).
- **Comprobación mínima de la cita**: tras normalizarla, debe contener la revisión de 12 hexadecimales mostrada, o ser una respuesta de elección explícita («aprobar», «apruebo», «sí, aprobar»). Una letra de opción («A») o «recommended» solo valen si el motor registró en el mismo turno una pregunta con opciones para ese objeto y revisión (`approve --request`, que devuelve un `prompt_id`; la aprobación lo cita con `--prompt-id`). Es una heurística auditable, no una prueba de humanidad, y así se documenta.

## R8. Ejecución gestionada (`run` / `report`)

- **Decisión**: se reutiliza el patrón concesión–ack de Socrática (feature 042): `run` crea `hcd_executions(id, task, definition_rev, state=granted, nonce)` y devuelve `execution_id`; `report` acepta un único resultado por ejecución. Si se repite el mismo contenido, devuelve el resultado original; si el contenido es otro, lo rechaza. Una ejecución `granted` sin `report` tras `--timeout` (24 h por defecto) pasa a `uncertain` y no se repite (C-19).
- **Ejecutor**: en las skills, el ejecutor es el propio agente que las invoca. `run` no lanza procesos y no ejecuta comandos escritos en la evidencia (exclusión de la spec).

## R9. Idempotencia de operaciones

- **Decisión**: `hcd_operations(project, operation_id PK, command, payload_hash, result_json, created_at)`. Toda mutación acepta `--operation-id` (si se omite, el motor genera uno y lo devuelve). Si se repite con el mismo `payload_hash`, devuelve `result_json`; con otro hash, responde `operation_conflict` (C-18).

## R10. Invalidación de evidencia

- **Decisión**: cada evidencia guarda `definition_rev`, `delivery_rev`, `playbook_rev` y `subject_ref` (commit, o hash del árbol de archivos declarados si hay cambios sin commit). Al calcular `status` o `verify`, el motor recalcula las revisiones vigentes y marca `stale` toda evidencia cuyas entradas cambiaron. La evidencia `stale` se conserva pero no cuenta para PASS ni para el cierre (C-09).

## R11. Aclaración (FR-016a)

- **Decisión**: `hcd_clarifications(id, unit, target_ref, question, options_json, recommendation, answer, status open|answered|skipped, blocking)`. `hcd-clarify` las crea y responde, una a la vez; cada respuesta genera una decisión (FR-040b). `validate` y `approve` fallan con `blocked` si existe una aclaración bloqueante abierta sobre la definición, y `check`/`run` si existe sobre la tarea.

## R12. Ceremonia de Especificación (C-01)

- **Hallazgo**: no existe hoy. La TUI tiene las pantallas `screenSocratica*` y el interruptor de la extensión Spec Kit.
- **Decisión**: pantalla nueva `screenSpecification` al final del enum (convención de `tui.go`) con dos opciones: «HCD Playbook — recomendado» crea la unidad con el caso de uso de `start`, y «Spec Kit — legado» muestra cómo continuar con `/speckit-specify` o `/socratica-hu`, sin crear artefactos HCD. La pantalla usa los mismos casos de uso que el motor (FR-049). `/hcd` equivale a elegir HCD (spec, US1 escenario 2).
- **Prioridad**: P2. El MVP se demuestra con `/hcd`.

## R13. Corrección de la §18 de la Constitución (FR-010)

- **Decisión**: la §18 vive en la memoria fijada (`mem constitution`) y se sincroniza a `.specify/memory/constitution.md` (`syncSpeckitConstitution`). La corrección se redacta como propuesta en `specs/045-hcd-protocol/contracts/constitution-18-amendment.md`, se presenta al Implementador para aprobarla y se aplica por la vía existente (`mem constitution` y la sincronización), nunca editando solo la copia de `.specify`.
- **Orden**: es una tarea bloqueante de D1, que debe cerrarse antes de la primera unidad HCD real (FR-010).

## R14. Detección de unidades Spec Kit (C-02, FR-003)

- **Decisión**: `SpecKitReader` (port existente) ya lee `specs/NNN-*`. Una carpeta bajo `specs/` es Spec Kit; una bajo `hcd/features/` con `definition.md` y su fila en `hcd_units` es HCD. Una carpeta de `hcd/features/` sin fila, o con `spec.md` y `definition.md` a la vez, es `unclassified` (FR-004). El motor no escribe nunca bajo `specs/`.

## R15. Marca interna de unidad activa (sustituye a los marcadores `SPECKIT` y a `.specify/feature.json`)

- **Hallazgo**: Spec Kit marca la feature activa con `.specify/feature.json` y con una referencia entre `<!-- SPECKIT START -->` y `<!-- SPECKIT END -->` en un archivo de contexto del agente. Esa marca depende de qué archivo lea cada agente, toca archivos que la persona no quiere versionar ni que una herramienta modifique, y depende de que el agente la edite.
- **Decisión**: tabla `hcd_focus(project PK, unit_id, client, session_id, set_at, set_by)`, con una sola unidad activa por proyecto. `start` y `resume` la fijan (el `resume` sin id usa la activa); `close` y `cancel` la liberan. Todos los agentes la leen por el mismo motor y por el contexto de arranque, que ya comparten. HCD no escribe archivos de contexto del agente ni archivos de feature activa: fuera de la base, solo escribe bajo `hcd/`. La garantía se comprueba por exclusión (nada cambia fuera de `hcd/`), no con una lista de nombres de archivo.
- **Alternativas descartadas**: un archivo `hcd/.active`, porque sería otra fuente de verdad editable y versionable por error; una marca por cliente, porque la persona alterna entre agentes sobre la misma unidad.

## R16. Heartbeat: mismo evento que el checkpoint, registro propio

- **Hallazgo**: el checkpoint automático lo crea `recordActivityCheckpoint` en el hook `turn-end`, que dispara Claude Code (`Stop`), Codex (`Stop`, `codex_setup.go`) y OpenCode (`session.idle`). Se guarda como memoria `type=checkpoint`, que:
  - se deduplica por contenido (`persistence/memory.go`);
  - solo muestra las 5 últimas en «Actividad Reciente (auto)», recortadas (`build_context.go`);
  - no transporta masa (`memory_mass.go`) y es candidata a consolidación (`consolidate_memories.go`).
- **Decisión**: el heartbeat **cabe en el mismo hook** (`turn-end` y `subagent-stop`), con el mismo payload ya leído, pero **antes** de la salida temprana de `recordActivityCheckpointWithPayload` (que no guarda checkpoint si el turno no tuvo actividad), para que corra en todo fin de turno. Se registra en `hcd_heartbeats` y **no** como memoria checkpoint, porque las decisiones de la ceremonia no pueden depender de un registro que se deduplica, se recorta y se consolida. Es best-effort (regla de oro de los hooks: nunca abortar el turno) y solo actúa si existe `hcd_focus` para el proyecto.
- **Qué registra**: sesión, cliente, `stage`, `state_revision` y archivos tocados (del mismo `turnActivity`).
- **Qué detecta** (FR-041c, solo desvíos estructurales): archivos tocados sin una ejecución `granted` vigente, ejecuciones `granted` sin `report` Una unidad activa sin heartbeat durante 7 días no se detecta en el heartbeat (que justo indica actividad), sino al consultar (`status`, `resume` y contexto de arranque).
- **Qué no hace**: no interpreta texto ni infiere decisiones (eso sería decidir por probabilidad, contra FR-016a). Las decisiones las registra solo el motor en su paso (FR-040b).
- **Retención**: se conserva el último heartbeat por sesión más los que llevan un desvío; el resto se compacta en `hcd_units.last_seen_at`. Así la tabla no crece por turno.

## R17. Recuperación de decisiones entre sesiones y agentes

- **Decisión**: tres vías, todas sobre el mismo estado:
  1. **Arranque**: `ContextBuilder` añade la sección `## HCD activo` (unidad, paso, siguiente acción, aclaraciones abiertas, las últimas 5 decisiones aceptadas con ID, resumen y revisión, y los desvíos) cuando existe `hcd_focus`. La sección entra en el presupuesto y en el delta de sesión (`SessionDelta`, `deltaSections`), así no se repite si ya se entregó.
  2. **`/hcd` o `$hcd` sin argumentos**: `resume` de la unidad activa, con la lista completa de decisiones vinculadas a la revisión vigente.
  3. **Búsqueda transversal**: las decisiones también son memorias `type=decision` con `topic_key hcd:{unit}:{D-NN}` (R3), así `search_memories` y `get_context` las encuentran aunque no haya unidad activa.
- **Orden de autoridad al recuperar**: el estado de control `hcd_decisions` manda; la memoria es su reflejo para la búsqueda. Si discrepan (por ejemplo, con `memory_sync=pending`), se muestra el estado de control y el pendiente de sincronización.

## R18. El Playbook como documento fijado (valor por defecto, carga y actualización)

- **Hallazgo**: GoMemory ya resuelve este problema para la Constitución y las reglas de trabajo:
  - el catálogo `domain.PinnedDocs` (`domain/seed.go`) declara, por documento, el alias, `TopicKey`, la plantilla embebida (`Template`), `DefaultSHA256` y `PreviousDefaultSHA256`;
  - `usecases.ResolvePinnedDoc` devuelve la versión del proyecto o, si no existe, el valor por defecto, y lo avisa;
  - `mem docs` y la TUI (feature 021) permiten ver, exportar, importar y restaurar;
  - la siembra (`UpgradePristineSeeds`) actualiza una copia solo si coincide intacta con un valor por defecto anterior;
  - `--sync` refleja el documento en un archivo esperado por otra herramienta sin convertirlo en fuente.
- **Decisión**: el Playbook es una tercera entrada del catálogo, sin mecanismo nuevo:

  | Campo | Valor |
  |---|---|
  | `Alias` | `playbook` |
  | `TopicKey` | `TopicHCDPlaybook` |
  | `Type` | `Architecture` |
  | `Title` | «Playbook HCD del proyecto» |
  | `Label` | «Playbook HCD» |
  | `Template` | `hcd/hcd-playbook.md` (bajo `infrastructure/templates/`) |
  | `DefaultSHA256` | huella de la plantilla inicial (`PreviousDefaultSHA256` vacío) |

  - `mem playbook` es un atajo como `mem constitution` (`runPinnedShortcut`). Hoy `--sync` está fijo a `.specify/memory/constitution.md` (`syncSpeckitConstitution`); se generaliza con un destino de sincronización por alias, y el de `playbook` es `hcd/hcd-playbook.md`. La TUI de documentos recorre `domain.PinnedDocs`, así que muestra el Playbook sin cambios de pantalla.
  - `mem hcd playbook show|validate|adopt` usa `ResolvePinnedDoc`.
- **Contenido**: reglas del protocolo HCD (`HCD-<ÁREA>-NN`); principios de diseño propios del Playbook —Arquitectura Limpia (`HCD-CLEAN-NN`) y SOLID (`HCD-SOLID-SRP`, `-OCP`, `-LSP`, `-ISP`, `-DIP`)— que complementan la §3 sin repetirla (FR-007b), cada uno con comprobación `structural` cuando existe una guarda automática (regla de dependencia → `layer_rules_test.go`; DIP en adaptadores → ports en `application/ports`) o `review` cuando requiere juicio (SRP, OCP, LSP, ISP); y una tabla de correspondencia «sección de la Constitución → regla HCD → ámbito → control». **No se copia el texto de la Constitución** (FR-007a): duplicarlo crearía dos fuentes que divergen, y la Constitución está por encima del Playbook (§0, FR-011).
- **Historial y diferencias** (FR-012): un documento fijado guarda solo su versión vigente. Por eso, la primera vez que una unidad fija una revisión, el motor inserta su contenido en `hcd_normative_revisions`. `adopt` compara la revisión fijada con la resuelta actual y muestra las diferencias. Una importación, restauración o actualización de la plantilla nunca cambia una unidad activa; solo produce la revisión nueva que `status` señala como «adopción disponible».
- **Efecto en `playbook propose`**: la «propuesta» se cubre con el flujo existente de exportar, editar e importar el documento fijado más la adopción explícita por unidad. No hace falta un mecanismo de publicación aparte, y la tarea deja de ser no atómica.
- **Alternativas descartadas**:
  - un archivo `hcd/hcd-playbook.md` como fuente, porque no tendría valor por defecto ni actualización segura y obligaría a cada proyecto a copiarlo;
  - incluir la Constitución completa dentro del Playbook, por la doble fuente de verdad.
