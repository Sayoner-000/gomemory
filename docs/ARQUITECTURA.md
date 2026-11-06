# Arquitectura de gomemory

La descripción de capas, adaptadores, flujos de instalación y contratos de agentes está en [architecture.md](architecture.md). Se conserva esa ruta porque ya la enlazan otras guías del repositorio.

La [guía técnica](Guia-Tecnica_GOMEMORY-v2.49.html) resume la arquitectura completa en una sola página (HTML imprimible a PDF), y la [guía SDD_HCD](Guia-SDD_HCD_GOMEMORY-v2.49.html) explica cómo usar gomemory en un harness con Spec Kit y con HCD, y cómo conviven los dos. El repositorio conserva solo las guías vigentes.


## Expand (040)

`hook expand` adapta el payload del harness a `ExpandSignals`, consulta una vez el router puro `domain.RouteSupertools` y ejecuta sus acciones mediante `usecases.ExpandHook`. Las reglas se evalúan primero; después se agrega contexto y se ejecutan las observaciones silenciosas. El adaptador aplica los interruptores, deduplica sugerencias por episodio y renderiza el dialecto del harness sin aprobar herramientas.

Los puertos `RuleSourcePort` y `AgentConfigReaderPort` aíslan la lectura de reglas y configuración. `ObservationQueueRepository` y `ShieldStateRepository` conservan el estado de archivos; `InstinctRepository` y `InstinctGlobalRepository` aíslan las estadísticas y la promoción entre proyectos. El composition root construye todos los adaptadores.

La ruta ligera del hook no abre SQLite para permitir herramientas ni observar comandos. Solo abre el contenedor completo al registrar una activación o plegar observaciones. Las decisiones tienen presupuesto de 150 ms y fallo abierto; el registro se realiza después de emitir la decisión, con su propio límite. La inyección de instincts ocurre únicamente en `Builder`, con umbral 0,7 e interruptores aplicados.

Consulta [expand.md](expand.md) para comandos, formatos, permisos, límites y excepciones de cada harness.

## HCD (045)

HCD conduce una unidad de trabajo por su ceremonia: `define → clarify → approve → slice → tasks → implement → verify → close` en el carril completo, o la Vía rápida (`define → tasks → implement → verify → close`, sin `approve` ni `slice`). La persona la recorre con las skills `/hcd` o `$hcd` y `hcd-*`; cada skill llama al motor `mem hcd`, que es el único que registra aclaraciones, aprobaciones, evidencia y cierres.

- **Dominio** (`domain/hcd.go`, `hcd_doc.go`, `hcd_stage.go`): `ParseUnitDoc` lee el documento de la unidad; `domain.HCDState` deriva el paso, la siguiente skill, el gate de evidencia, el gate TDD (`RedMissing`) y `Analyze`. El paso nunca se guarda.
- **Revisiones**: la de la definición (todas sus secciones obligatorias: Intención, Reproducción, Alcance, «¿Qué pasa si no lo hacemos?», «Alternativa más barata», Historias y Aclaraciones) es la que aprueba `approve`; la de trabajo (la definición más Slices y Checklist, sin casillas) es la que usan la evidencia y el cierre.
- **Caso de uso** (`application/usecases/hcd.go`): `Start`, `Show`, `Clarify`, `Approve`, `Verify`, `Analyze`, `Close`, `Cancel`, `Promote` y `ActiveHCDContext`. Las decisiones se reflejan como memorias `decision`.
- **Puertos** (`application/ports/hcd.go`): `HCDStore` (unidades, evidencia y aprobaciones), `HCDWorkspace` (documento, huella y ejecución) y `HCDContextRepository` (sección «HCD activo» del contexto de arranque).
- **Adaptadores**: `persistence.HCDStore` sobre las tablas `hcd_units`, `hcd_evidence` y `hcd_approvals` (ADR 0003); `hcdfiles.Workspace`, que solo escribe bajo `hcd/` sin seguir symlinks, calcula la huella (`HEAD` más los archivos cambiados y sin rastrear, excluyendo `hcd/`) y ejecuta las comprobaciones con tiempo límite; `cli/hcd`, el motor con salida `--json`; y el formulario «Especificación» de la TUI.
- **Normas transversales**: el caso de uso resuelve la Constitución y el Playbook vigentes (`ConfigureNorms`, sobre `ResolvePinnedDoc`). Cada aprobación guarda sus huellas; si cambian tras aprobar la definición, `HCDState.NormsChanged` hace que `analyze` lo reporte y que el cierre exija reaprobar.
- **Hook**: en Claude Code, `mem hook expand pre-tool` avisa una vez por sesión, sin bloquear, al editar código sin unidad activa. Solo consulta la tabla de unidades, para caber en el plazo del hook.

La única fuente de contenido es el documento `hcd/HCD-NNN-<slug>.md`, que viaja en el PR. SQLite guarda solo el control: qué unidad está activa, qué se observó y qué aprobó la persona.

## Optimización de Grafos y Relaciones (HCD-025)

El subsistema de optimización sanea el grafo de relaciones entre memorias, las estructuras del grafo de código interno en SQLite y los residuos de snapshots de proveedores externos:

- **Dominio** (`domain/optimize.go`): Modelos de reporte `StorageOptimizationReport` (relaciones huérfanas, duplicadas consolidadas, nodos/aristas/paquetes de código saneados y optimizaciones PRAGMA/FTS), `ExternalProviderOptimizeReport` y `OptimizeReport` general.
- **Puertos**: `MaintenanceRepository.OptimizeStorage` en `application/ports/maintenance_repository.go` y `CodeGraphMaintainer` en `application/ports/code_graph_maintainer.go`.
- **Caso de uso** (`application/usecases/optimize_graph.go`): `OptimizeGraph` valida el ámbito (`all`, `internal`, `external`), previene ejecuciones con flags contradictorios y orquesta de forma desacoplada la optimización del repositorio de mantenimiento y de los proveedores de código registrados.
- **Persistencia** (`adapters/secondary/persistence/optimize_storage.go`): Ejecuta el saneamiento en SQLite de forma transaccional garantizando aislamiento estricto por proyecto, eliminación en orden seguro (nodos -> aristas -> paquetes huérfanos), purga de filas huérfanas en el índice virtual FTS5 `code_search` antes del `optimize`, y ejecución de `PRAGMA optimize`.
- **Proveedores externos** (`adapters/secondary/codegraph/...`): `codebasememory` y `codegraphcli` sanean snapshots huérfanos o corruptos en `~/.memory/` preservando snapshots de otros proveedores válidos, manejan ausencia de binarios en PATH de forma no bloqueante y permiten reindexado explícito con `--reindex-external`.
- **Interfaces primarias**: Comando CLI `mem optimize` (`adapters/primary/cli/cmd_optimize.go`) con soporte para `--dry-run`, `--yes`, `--target` y `--reindex-external`, y opción 4 en el menú de mantenimiento interactivo de la TUI (`adapters/primary/tui/tui.go`).

## Hooks de Subagentes, Regla Graph-Before-Grep y Contexto de Edición (HCD-026 a HCD-029)

Este bloque extiende el ciclo de vida de los hooks de gomemory en los cuatro runtimes soportados (Claude Code, OpenAI Codex, Antigravity y OpenCode):

- **Aviso mínimo a subagentes (HCD-026)**:
  - CLI `mem hook subagent-start` y adaptadores dialectales (`hook_agy.go`, plugin OpenCode).
  - Al crearse un subagente o proceso delegado, el harness inyecta únicamente un recordatorio conciso de solo lectura en vez del volcado completo de memorias del proyecto, preservando el presupuesto de tokens del contexto hijo.

- **Regla del core `graph-before-grep` (HCD-027)**:
  - Dominio (`domain/rules_graph.go`): define la detección de comandos `grep` o `git grep` dirigidos a código fuente cuando el proyecto cuenta con índice de grafo disponible.
  - Adaptador `hook expand pre-tool`: bloquea la ejecución de búsquedas por texto plano sobre código indexado e instruye al agente a consultar el grafo (`codegraph_explore`, `search_graph`, `search_code`, `get_symbol`). No interfiere en búsquedas sobre documentación ni archivos fuera del grafo.

- **Contexto de grafo en edición de código (HCD-028)**:
  - Puertos (`application/ports/edit_context_source.go`): abstracción para consultar dependencias y llamadores del símbolo afectado.
  - Casos de uso (`application/usecases/edit_context.go`): genera el contexto acotado por presupuesto de tokens.
  - Persistencia (`adapters/secondary/persistence/edit_context.go`): registra la huella de archivo y sesión para inyectar la información una sola vez por archivo editado, evitando ruido reiterativo.
  - Integración en runtimes: se ejecuta en los hooks pre-tool de edición (`edit`, `Edit`, `write_file`) en Claude, Codex, Agy y OpenCode.

- **Persistencia y saneamiento incremental del grafo interno (HCD-029)**:
  - `adapters/secondary/persistence/code_graph.go` (`ReplaceFileNodes`): al reindexar un archivo por cambios de hash, captura previamente las aristas entrantes provenientes de otros archivos.
  - Tras sustituir los nodos del archivo, restaura las aristas entrantes asociándolas a los nuevos identificadores de nodo por identidad del símbolo. Si un símbolo fue eliminado, su arista se descarta en lugar de apuntar a un nodo incorrecto o quedar huérfana en SQLite.

## Regla de Serie Native-Edit y Correlatividad de Unidades HCD (HCD-030 y HCD-031)

Este bloque incorpora salvaguardas de edición de código y coherencia en la numeración del ciclo de vida HCD:

- **Regla del core `native-edit-over-scripts` (HCD-030)**:
  - Dominio (`domain/rules_edit.go`): define la detección de scripts inline (`python`, `python3`, `node`, `bun`, `deno`, `ruby`, `perl`), herramientas de reemplazo in-place (`sed -i`, `awk`) y redirecciones de terminal (`cat >`, `echo >`, `printf >`, `tee`) dirigidos a sobreescribir archivos de código en las extensiones del mercado (`.ts`, `.tsx`, `.js`, `.jsx`, `.vue`, `.svelte`, `.astro`, `.go`, `.py`, `.rs`, `.java`, `.kt`, `.cs`, `.cpp`, `.php`, `.rb`, `.swift`, `.sh`, `.dart`, `.sql`, etc.).
  - Adaptador `hook expand pre-tool`: bloquea el comando en los cuatro runtimes e instruye al agente a usar las herramientas de edición nativas (`edit_file`/`Edit`) para preservar diffs limpios y activar la inyección de contexto de grafo (`HCD-028`).

- **Correlatividad y numeración de unidades desde disco (HCD-031)**:
  - Puertos (`application/ports/hcd.go`): método `MaxUnitNumber()` en `HCDWorkspace`.
  - Adaptadores (`adapters/secondary/hcdfiles/workspace.go`): escanea `hcd/HCD-*.md` para extraer el prefijo numérico más alto en disco.
  - Casos de uso (`application/usecases/hcd.go`): `HCD.Start` calcula `MAX(sqlite_max, disk_max) + 1`, garantizando que en clones frescos de repositorios con base de datos SQLite recién inicializada se continúe la numeración correlativa sin reiniciar en `HCD-001` ni colisionar con unidades históricas.

## Revisión de seguimiento, avisos de analyze y una sesión por agente (HCD-032 a HCD-036)

**Revisión ACR de seguimiento (HCD-032 y HCD-035).** `StartReviewSince`
(`application/usecases/start_review.go`) enlaza una revisión con otra anterior
ya terminada. El dominio decide qué se hereda (`domain/review_inheritance.go`)
y `adapters/secondary/gitdiff` calcula con git el delta entre los dos commits.
Para estimar el ahorro, el port `RevisionFiles` lee el alcance tal como estaba
en el commit con un solo `git cat-file --batch`, y
`usecases.ReviewSinceSavings` lo cuenta con el `TokenCounter` de siempre. El
cálculo en sí es una función pura (`domain/review_savings.go`).

**Avisos de `analyze` (HCD-034).** `domain/hcd_lens.go` detecta tareas sin
archivo, rutas citadas y alternativas de relleno sin tocar el disco. Saber si
una ruta existe es trabajo del port `HCDWorkspace.PathExists`, que no sale de
la raíz del proyecto; el caso de uso pasa al dominio la lista de las que faltan.

**Una sesión por conversación de agente (HCD-036).** Antes había una única
sesión activa por proyecto, y dos agentes trabajando a la vez se la quitaban.
Ahora:

- `sessions.conversation_id` (esquema 10, aditivo) ata cada sesión a su
  conversación. `SessionRepository`, `DeliveryLogRepository` y
  `DeliveredBlocksRepository` reciben un `ports.ConversationResolver` con
  `WithConversation`, así que los llamadores de `Active(project)` no cambian.
- `adapters/secondary/hostbind` sigue el patrón hostbind de engram. Un hook
  publica «pid del proceso host → conversación» en
  `projects/<clave>/hostbind/<pid>.json`, guardando también la hora de arranque
  del proceso para que un PID reutilizado no herede nada. El MCP y la CLI lo
  encuentran recorriendo sus procesos ancestros.
- `hostbind.Resolver` decide en este orden: el `session_id` del hook, el
  enlace del host y, en la CLI, el terminal. Sin ninguno, el proceso usa la
  sesión de proyecto con `conversation_id IS NULL`, que nunca es la de un
  agente identificado.
- El estado por turno de `.memory/` se guarda en
  `.memory/conversations/<hash del id>/` (`perConversationPath` en
  `adapters/primary/cli/conversation.go`).
- El MCP abre su sesión en segundo plano y espera unos segundos a que aparezca
  el enlace. Si la abriera de inmediato, chocaría con el SessionStart de su
  propio host y quedaría una sesión huérfana.
