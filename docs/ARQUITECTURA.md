# Arquitectura de gomemory

La descripción de capas, adaptadores, flujos de instalación y contratos de agentes está en [architecture.md](architecture.md). Se conserva esa ruta porque ya la enlazan otras guías del repositorio.

La [guía técnica](Guia-Tecnica_GOMEMORY-v2.52.html) resume la arquitectura completa en una sola página (HTML imprimible a PDF), la [guía SDD_HCD](Guia-SDD_HCD_GOMEMORY-v2.52.html) explica cómo usar gomemory en un harness con Spec Kit y con HCD, y el [manual de referencia para desarrolladores](Manual-Referencia-Developers-GOMEMORY.html) junto con la [guía de explicación](Guia-Explicacion-Developers-GOMEMORY.html) detallan la convivencia de los tres grafos de código y los protocolos operativos. El repositorio conserva solo las guías vigentes.


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
  enlace del host, `GOMEMORY_CONVERSATION_ID` (`hostbind.ConversationEnv`, el
  respaldo de los sandboxes que no permiten `ps`, ADM-14) y, solo en la CLI y
  la TUI, `GOMEMORY_TERMINAL` (`hostbind.TerminalID`, id `terminal-<valor>`).
  Si `ps` falla, `psInfo` devuelve el error y el enlace no se publica ni se
  lee: nunca se reutiliza un PID sin verificar. Sin ninguno, el proceso usa
  la sesión de proyecto con `conversation_id IS NULL`, que nunca es la de un
  agente identificado. Hasta la v2.49.2 la CLI se identificaba por su TTY; eso
  la separaba del MCP y de los hooks, que no tienen terminal y no veían su
  sesión (HCD-048). `conversationFor` en `infrastructure/container.go` es el
  único sitio que asigna el id local.
- El estado por turno de `.memory/` se guarda en
  `.memory/conversations/<hash del id>/` (`perConversationPath` en
  `adapters/primary/cli/conversation.go`).
- El MCP abre su sesión en segundo plano y espera unos segundos a que aparezca
  el enlace. Si la abriera de inmediato, chocaría con el SessionStart de su
  propio host y quedaría una sesión huérfana.

## Arranque recortado y enlace del host en Windows (HCD-037 y HCD-038)

**Lo que el arranque entrega de verdad (HCD-037).** `domain.FitHookOutput`
ajusta las salidas de los hooks al tope del canal. `cutAtParagraph` también
corta justo antes de una entrada de lista (`- `). Antes solo cortaba entre
párrafos, y una sección de memorias, que es un único párrafo, se perdía entera.
Cuando el arranque se recorta, `entregaContextoDeArranque`
(`adapters/primary/cli/cmd_hook.go`) repite el mismo ajuste con
`arranqueSections`, la función que también usa `hookSessionStart`, y pasa lo
emitido a `SessionDelta.MarkVisible`. Este anota en `delivered_blocks` solo las
unidades del documento crudo que aparecen completas en lo emitido. Antes de
comparar normaliza los espacios igual que la compresión estructural, porque en
nivel max la sangría se colapsa. El hash del canal `context` no se registra:
`get_plan_context` lo usaría para suprimir un documento que el agente no
recibió entero.

**`psInfo` por plataforma (HCD-038).** El recorrido de ancestros de
`hostbind` necesita el padre, la hora de arranque y el nombre de cada proceso.
`ps_unix.go` los saca de `ps`, y `ps_windows.go` de `syscall`: el padre y el
ejecutable con `CreateToolhelp32Snapshot` y la hora de arranque con
`GetProcessTimes`. Si el padre se creó después que el hijo, se trata como un
PID reutilizado y el proceso queda huérfano. `isShell` reconoce los shells
intermedios de Windows (`cmd`, `powershell`, `pwsh`) con su ruta `\` y su
extensión `.exe`.

## Borrado con reintento, primer prompt corto y llamadas pendientes (HCD-039 a HCD-041)

**`atomicfs.Remove` (HCD-039).** `retireLocalCopy` ejecuta la copia local con
`version` para saber si es de gomemory y la borra justo después con
`removeBinaryFile`. En Windows el ejecutable recién usado puede seguir
bloqueado un instante, y `os.Remove` fallaba con «being used by another
process». `atomicfs.Remove` reintenta los errores 5, 32 y 33 durante 2
segundos como máximo. Usa `retry`, el mismo bucle que ya tenía el reemplazo
de archivos en Windows, que ahora vive en `remove.go`. Fuera de Windows
ningún error cuenta como transitorio y `Remove` es `os.Remove`.

**Recordatorio corto (HCD-040).** `memoryProtocolReminder`
(`adapters/primary/cli/cmd_hook.go`) cierra el primer prompt, `get_context` y
`get_plan_context`. Cada regla va en una o dos frases y el grafo externo usa
`domain.CodeRetrievalPolicyShort`. `buildIntegrationBlock`, las instrucciones
MCP, conserva `domain.CodeRetrievalPolicy`, la larga. Las dos constantes viven
en `domain/code_retrieval_policy.go`: si cambia la prioridad, cambian las dos.
`TestPrimerPrompt_CabeEnElTope` fija el tope en 3.000 caracteres sin Octopus
y 4.000 con él.

**Llamadas pendientes (HCD-041).** `indexFilesInternal`
(`application/usecases/index_project.go`) tiene ahora tres pasadas. La
primera guarda los nodos de cada archivo. La segunda resuelve sus aristas, y
`resolveEdges` devuelve también las llamadas que no encontraron destino, que
se guardan en `code_pending_calls`. La tercera busca las pendientes hacia los
nombres que acaban de aparecer, las resuelve con `resolveIdentCall` y
`resolveSelectorCall` y las inserta con `ResolvePendingCalls`. Esa función
añade aristas con el `src_file_id` del archivo que llama sin borrar las demás,
a diferencia de `InsertEdges`. `ReplaceFileNodes` y `DeleteCodeFile` borran
las pendientes del archivo junto con sus nodos. Con esto se cierra el límite
que había dejado HCD-029.

## Historial de decisiones, subagente splash y resiliencia de hooks (HCD-046 a HCD-054)

**Fichas de decisiones y tabla `memory_files` (HCD-046).** Antes cada paso del motor dejaba una memoria suelta en la base de datos. Ahora cada unidad conserva una única ficha viva con clave `hcd/HCD-NNN` que se actualiza en cada cambio de estado, y cada auditoría de `docs/audits/` mantiene su propia ficha `audit/<archivo>`. La tabla `memory_files` (esquema 12) indexa los archivos citados en cada ficha; cuando el agente va a editar un archivo, `get_context` recupera las decisiones previas asociadas. `mem hcd index` genera el índice histórico de forma acumulativa y la herramienta MCP `hcd_history` expone las consultas para los agentes.

**Subagente `splash` sin memoria (HCD-047 y HCD-051).** Para evitar el sesgo de confirmación al revisar documentos y propuestas técnicas, `splash` arranca con contexto cero (el modelo del pez dorado frente al modelo del elefante). No recibe memorias del proyecto ni herramientas MCP, y solo puede leer archivos locales con `read`, `grep` y `glob`. `SubagentStart` omite el contexto en Claude Code (`~/.claude/agents/splash.md`), el plugin de OpenCode intercepta la sesión hija, agy aísla el MCP con `inheritMcp: false` y Codex usa `spawn_agent` con `fork_turns: "none"`. En la ceremonia completa de HCD, `/hcd-approve` ofrece lanzar `splash` sobre el documento de la unidad antes de pedir la aprobación formal; los hallazgos de tipo Blocker detienen la cita hasta su resolución.

**Rutas de configuración en `analyze` (HCD-052).** Un documento de unidad suele citar rutas del entorno del usuario bajo `~/.claude/`, `~/.codex/` o `~/.gemini/`. `RepoMissingPaths` en `domain/hcd_lens.go` las descarta de la comprobación de archivos inexistentes para no generar falsos positivos `PATH-MISSING`. `CitedPaths` las conserva para mantener los enlaces en el grafo.

**Presupuesto y concesiones en `tool-output` (HCD-053 y HCD-054).** El hook `tool-output` comprime respuestas JSON ordenando los campos alfabéticamente para que la nota de recuperación aparezca de forma determinista en el primer campo. `runToolOutputBudget` anota la pista como mostrada solo si la salida se entrega dentro de los 150 ms del presupuesto. Si el proceso supera el plazo, `toolOutputCompress` descarta la salida y omite la llamada a `GrantAutomaticOriginals`, de modo que no se conceden referencias de almacenamiento que el agente nunca llegó a recibir.

## Cableado de agy en expand e idioma del proyecto (HCD-055 y HCD-056)

**Diagnóstico y retiro de `agy` en `mem expand` (HCD-055).** `CheckExpandWiring` y `RemoveExpandHooks` en `adapters/primary/setup/expand_wiring.go` leen y limpian también `~/.gemini/settings.json` para Antigravity CLI (`agy`). `mem expand status` muestra `agy ✓/✗` en texto e incluye `wiring.agy` en `--json`, comprobando que el grupo `gomemory-lifecycle` esté habilitado y registre los cinco eventos (`SessionStart`, `PreInvocation`, `PreToolUse`, `PostToolUse` y `Stop`) con los comandos y matchers que escribe el instalador.

**Catálogo de idiomas y `mem lang` (HCD-056).** El paquete `domain/i18n` y el puerto `ports.LocaleCatalog` (`application/ports/i18n.go`) definen `Language`, `LocaleMeta`, `DefaultLanguage` (`"es"`) y las operaciones `Supports`, `SupportedCodes`, `Get` y `GetTemplate`. El adaptador `adapters/secondary/i18n` recorre `infrastructure/templates/locales/<código>/` en el `embed.FS` y carga `meta.json`, `messages.json`, `get_context.md.tmpl` y `atomic_plan.md.tmpl`, volviendo a `"es"` cuando falta una clave o se pide un idioma desconocido. Para no mutar el `Builder` compartido de `deps.ContextBuilder` en llamadas MCP concurrentes, `(*Builder).WithLanguage` y `contextBuilderForLang` crean una copia superficial por llamada; si `Builder.Language` está vacío, `Build()` lee el idioma desde `.memory/settings.json`. En `cmd_plan_context.go`, `isContextDocumentHeader` consulta todos los títulos de idioma del catálogo para no duplicar la cabecera, y `SetLanguageValidator` en `adapters/secondary/persistence/settings.go` normaliza a `"es"` cualquier código no soportado al leer o guardar `.memory/settings.json`.
