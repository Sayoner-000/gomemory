# Changelog

All notable changes to gomemory are documented in this file.

The format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versioning follows [Semantic Versioning](https://semver.org/).

## [2.18.1] - 2026-09-03

### Changed

- Aplica política de delegación Octopus AAR a todos los agentes (Claude Code, OpenCode, Codex)
- Migra hooks de Codex al sistema compartido `octopusDelegationPolicy` desde `setupCodexGlobal`

### Removed

- Retira workflow de CI independiente (`release.yml`)

## [2.16.10] - 2026-09-02

### Correcciones

#### Pegado funciona en todas las cajas de la TUI

`updateFocusedInput` enruta el pegado por pantalla en vez de recorrer el modelo
con `Focused()`. Antes, `saveContent` se enfocaba al construir el modelo y
nunca se desenfocaba al salir de la pantalla de guardar, así que se tragaba el
pegado de todas las demás cajas (ruta de import, ajustes de IA, documentos),
que quedaban sin poder pegar.

Dentro de una pantalla con varios inputs se le pasa el mensaje a todos: el
`textinput` ignora lo que recibe mientras está desenfocado.

#### Foco correcto al guardar

`saveAndReturn` usa `updateFocus()` en lugar de `Focus()` suelto para evitar
doble foco tras tabular entre cajas de la pantalla de guardado.

---

## [2.16.7] - 2026-08-30

### Correcciones

#### Un resultado `failure` ya no puede reescribir las fuentes del consenso

La corrección de 2.16.6 que devolvió el fail-closed dejaba pasar todo resultado
`failure` sin mirar su contenido. Como los hallazgos se persisten con
`ON CONFLICT DO UPDATE`, un `failure` que adjuntara un hallazgo con un
`local_id` ya existente reescribía su enunciado, su severidad o su evidencia
—sobre fuentes que el consenso ya había clasificado—. La declaración de fallo se
había convertido en la vía de escritura que la guarda existía para cerrar.

Declarar `failure` sigue siendo siempre posible, que es lo que sostiene el
fail-closed, pero fuera de la fase de recogida no puede traer hallazgos. Es la
misma regla que ya rige en las rondas de revalidación y por el mismo motivo: no
hay consenso que pueda clasificar un hallazgo que llegue ahí.

**Cambio observable:** `review_submit` rechaza un resultado `failure` con
hallazgos cuando la revisión ya salió de la fase de recogida. Envíalo sin ellos
y abre una revisión nueva para el material no clasificado.

#### El hook de fin de turno respeta el dialecto del agente

`turn-end` emitía el sobre JSON de Claude Code a cualquier agente. Codex toma el
stdout del hook como contexto tal cual y no reconoce ese sobre, así que
rechazaba la salida entera y el refuerzo de preferencias no llegaba nunca. El
campo `Emit` de la tabla de hooks de Codex existía justo para esto, y `turn-end`
—el otro hook que inyecta texto al modelo— se había registrado sin pedirlo.

Ahora `turn-end` acepta `--emit`, se registra en Codex con `Emit: "text"`, y
traduce sus dos mensajes según el destinatario: el aviso de compactación va a
quien mira la terminal y el refuerzo de preferencias al modelo. En los dialectos
planos ambos salen como texto desnudo, y el silencio es la cadena vacía en vez
de `{}`.

La salida para Claude Code no cambia.

## [2.16.6] - 2026-08-30

Correcciones surgidas de una revisión adversarial por consenso sobre 2.16.5.

### Correcciones

#### Un revisor vuelve a poder declarar `failure` con el consenso listo

La guarda de fase que 2.16.5 introdujo en el envío de resultados rechazaba
cualquier escritura fuera de la fase de recogida. Como el estado
`consensus_ready` lo escribe el propio envío del segundo revisor, la ventana se
cerraba en el mismo instante en que ambos terminaban: a partir de ahí nadie
podía declarar `failure`, y esa era la única vía a `INCOMPLETE` desde ese
estado. La transición quedaba inalcanzable aunque el dominio siguiera
declarándola legal, así que una revisión con ejecución inválida podía terminar
`APPROVED`.

Ahora la guarda deja pasar todo resultado `failure` y rechaza solo lo que debía
proteger: el reenvío que altera las fuentes sobre las que se construyó el
consenso.

#### `review_submit` recupera la idempotencia de reenvío

Por la misma guarda, un reintento con contenido idéntico tras `consensus_ready`
pasaba de no-op a error. Un reintento de transporte —un timeout cuya escritura
sí se confirmó— fallaba en el segundo intento. El reenvío vuelve a compararse
por contenido; la comparación se hace sobre los valores ya redactados, que es lo
que hay almacenado.

#### Un `failure` ya persistido no lo pisa un `success` concurrente

«Un resultado `failure` es final para la ronda» se comprobaba leyendo fuera de
toda transacción. Entre esa lectura y la escritura cabía un envío `success` que
sobrescribía el fallo y dejaba el ledger sin rastro de la ejecución fallida. La
comprobación previa sigue dando errores tempranos; la que cierra la carrera vive
ahora dentro del mismo bloqueo de escritura.

#### El digest de `--pending` deja de admitir dos lecturas

Los campos se concatenaban separados por NUL, pero ni la identidad del índice ni
el contenido del archivo tienen prohibido contenerlo: el mismo flujo de bytes
admitía más de un reparto, y dos estados pendientes distintos podían compartir
identidad congelada. Cada campo se escribe ahora precedido de su longitud.

**Cambio observable:** el digest de `--pending` cambia para el mismo árbol de
trabajo. Una revisión congelada con una versión anterior y continuada con esta
se rechaza con `target changed`; hay que volver a congelarla.

### Pruebas

Cobertura de las ramas que decidían en producción sin tenerla: la guarda de fase
y ronda del adaptador real —hasta ahora solo se afirmaba sobre la copia de esa
lógica en el doble de memoria—, las etapas 1/2/3 de un conflicto de merge en la
identidad del índice, y la colisión de reparto del digest de `--pending`.

## [2.16.5] - 2026-08-30

### Correcciones

#### La marca de resultados detecta cambios entre lecturas

`ReviewerResultsMark` resume resultados y hallazgos de una ronda como hash
SHA-256. Se usa en la comparación atómica de finalización para detectar si
alguien modificó los resultados entre la lectura y la escritura. Antes, solo
se comparaba el estado de la revisión y los re-juicios, no el contenido de los
resultados.

#### El envío de resultados comprueba fase y ronda bajo el mismo bloqueo

`UpsertReviewerResultAtomically` verifica el estado terminal, la fase y la ronda
dentro del `BEGIN IMMEDIATE` que persiste el resultado. La comprobación previa
del caso de uso sigue dando errores tempranos, pero la que cierra la carrera es
esta.

#### El linaje por revisor solo publica la ronda vigente

`ReJudgmentsByReviewer` filtra por ronda. Antes, un estado declarado en una ronda
ya superada seguía apareciendo en `review_status`, `review_rejudge` y
`mem review show`, de modo que la auditoría podía atribuir a un revisor una
postura que ya no sostenía.

#### La identidad del índice en `--pending` incluye modo y etapa

`blobsPreparados` conservaba solo el SHA del blob. El modo distingue un archivo
ejecutable de uno normal aunque los bytes sean idénticos, y la etapa distingue
las entradas 1/2/3 de un conflicto de merge, que además podían sobrescribirse
entre sí al compartir ruta.

**Cambio observable:** el digest de `--pending` cambia para el mismo árbol de
trabajo respecto de 2.16.4.

## [2.16.4] - 2026-08-29

### Correcciones

#### La finalización verifica la marca de re-juicios en el orden correcto

`FinalizeReview` leía la marca de re-juicios después de los datos de los que
se deriva el veredicto. Una retractación que aterrizara entre ambas lecturas
quedaba dentro de la marca pero el veredicto ya estaba calculado con hallazgos
obsoletos. Ahora la marca se lee primero.

`SetReviewStatusAtomically` reemplaza a `FinalizeReviewAtomically` y agrega
`ExpectedRejudgmentMark` a la comparación atómica.

#### Un hallazgo solo se muestra resuelto si lo está en la ronda vigente

`ConsensusFinding` gana `EstadoVigente` y `ResueltoEn`: un solo criterio de
vigencia. Antes, consultas de estado y de detalle leían la columna a secas y
mostraban `RESOLVED` mientras la finalización se negaba a cerrar sin decir cuál
era el hallazgo ni por qué.

## [2.16.3] - 2026-08-29

### Correcciones

#### El consenso se reemplaza de forma atómica

`BuildConsensus` ahora llama `ReplaceConsensusRound`: una sola transacción que
verifica que no existe y escribe todas las filas. Antes eran dos pasos separados
(listar y luego upserts individuales), y dos llamadas concurrentes veían el
ledger vacío y las dos escribían, mezclando la ronda.

#### La finalización ya no pierde actualizaciones concurrentes

`FinalizeReview` usa compare-and-swap: lee el estado antes de la transacción,
verifica que no cambió, y actualiza solo si coincide. `UpdateReview` reescribía
todas las columnas, así que una ronda de corrección entre la lectura y la
escritura se perdía sin aviso.

## [2.16.1] - 2026-08-29

### Correcciones

Cuatro defectos encontrados al revisar la versión 2.16.0 con su propio protocolo. Tres
viven en la misma ruta —qué ocurre DESPUÉS de una corrección— y ninguna prueba los veía,
porque todos los fixtures enviaban los resultados post-corrección sin hallazgos. El
cuarto solo aparece con los dos revisores trabajando de verdad en paralelo.

#### El consenso ya no se sobrescribe entre rondas

`BuildConsensus` aceptaba reconstruir el consenso en una ronda posterior. Los
`ConsensusLocalID` se generan por posición (C-001, C-002…) y son únicos por revisión, no
por ronda, así que el segundo cálculo reasignaba los identificadores y sobrescribía los
hallazgos confirmados de la ronda anterior: un `CONFIRMED HIGH` se convertía en un
`CONFIRMED LOW` con otras fuentes, y el `addressed_consensus_ids` de la corrección
quedaba apuntando a un hallazgo distinto del que decía haber arreglado.

Ahora el consenso se construye una sola vez, en la ronda de descubrimiento. Las rondas
posteriores revalidan con `review_rejudge`.

#### Una revisión con hallazgos post-corrección ya puede terminar

`SubmitReviewerResult` aceptaba hallazgos nuevos en rondas de revalidación. Quedaban sin
clasificación de consenso —y no podían tenerla, por el defecto anterior—, así que
`DeriveVerdict` los veía sin clasificar y la revisión no alcanzaba ningún estado
terminal: ni aprobada ni escalada, nunca.

Ahora esas rondas rechazan hallazgos y encaminan a `review_rejudge`, que es el canal que
el protocolo ya definía para revalidar.

#### Los re-juicios en paralelo ya no se pierden

`UpsertReJudgment` abría su transacción con el `BEGIN` diferido de `database/sql`, que
en modo WAL toma el bloqueo de escritura en el primer `INSERT`. El perdedor recibía
`SQLITE_BUSY`, que el `busy_timeout` no reintenta para una transacción que ya leyó.

Afectaba al flujo normal del protocolo, no a un caso raro: dos revisores independientes
re-juzgando a la vez es exactamente para lo que existe. Con 16 re-juicios simultáneos
fallaban 11 con «database is locked». Ahora usa una conexión dedicada con
`BEGIN IMMEDIATE`, el mismo patrón que ya empleaba el registro de correcciones.

#### La unanimidad ya no cruza rondas

`AggregateReJudgment` recibía todos los re-juicios de un hallazgo, de todas las rondas.
Un `RESOLVED` emitido sobre una corrección anterior completaba la unanimidad de una
posterior que ese revisor nunca vio, y la revisión terminaba aprobada. Se añadió
`AggregateReJudgmentForRound`, que filtra por ronda antes de agregar.

## [2.16.0] - 2026-08-29

### Correcciones

#### La revisión adversarial ya no puede aprobar con un defecto oculto

Tres defectos confirmados por dos revisores independientes sobre la propia
implementación de `mem review`. Los tres permitían que el protocolo diera por buena
una revisión que no lo estaba.

**Consenso parcial y severidad degradada.** La clasificación de consenso se validaba
hallazgo a hallazgo mientras se recorría la entrada, así que nadie comprobaba el
conjunto: omitir un hallazgo grave no producía ningún error, simplemente no se
mencionaba. Y la severidad venía del orquestador, de modo que un `HIGH` corroborado
por ambos revisores podía persistirse como `LOW` y desaparecer del veredicto. Ahora
la clasificación debe cubrir cada hallazgo de la ronda exactamente una vez, y la
severidad se deriva como el máximo de sus fuentes.

**La revisión de solo lectura se quedaba bloqueada.** Una revisión con un defecto
grave confirmado y sin autorización para corregir devolvía «review is not ready to
finalize» indefinidamente, porque el presupuesto de rondas permitía una corrección
que su alcance prohibía. `--read-only` y `review_fix_authorized` declaran ese alcance,
y esas revisiones terminan `ESCALATED` en una sola llamada.

**Las métricas no cumplían su propio contrato.** `review_finalize` prometía ocho
campos en `snake_case` y emitía cinco en PascalCase, porque el struct no tenía
etiquetas JSON. Faltaban `duration`, `memory_promoted` y `memory_deduplicated`.

### Otras correcciones del mismo trabajo

- **Correcciones concurrentes se perdían.** Registrar una ronda eran cuatro
  operaciones sueltas: dos procesos derivaban la misma ronda y el segundo
  sobrescribía al primero sin dejar rastro. Ahora es una transacción `BEGIN
  IMMEDIATE` que revalida dentro lo que se leyó fuera.
- **Los estados terminales no lo eran.** La máquina de estados existía y era
  correcta, pero ningún caso de uso la usaba: una revisión `APPROVED` aceptaba
  resultados, consenso y correcciones nuevas.
- **La política del proyecto no se aplicaba.** `review_max_fix_rounds` y
  `review_auto_fix_severities` existían en `settings.json` desde la versión anterior
  y nadie los leía; el código reimplantaba los defectos a mano. Además, guardar
  cualquier otra preferencia **borraba** esa política, porque no viajaba en la
  estructura que se reescribe al guardar.
- **Se podía promover aprendizaje desde una revisión sin aprobar.**
- **La identidad de los revisores no se comprobaba**, así que la independencia que
  la revisión declaraba no era verificable.
- **El re-juicio no distinguía quién lo emitía**: un solo revisor bastaba para marcar
  un defecto como resuelto. Ahora se registra por revisor, con evidencia, y `RESOLVED`
  exige unanimidad.

### Novedades

- `mem review --pending` congela **todo** el trabajo pendiente —preparado, sin
  preparar y archivos nuevos—, con una identidad reproducible. `--diff` usa `git
  diff`, que no ve los archivos sin seguimiento, así que congelaba menos de lo que
  parecía.
- `mem review --read-only` para revisiones que validan sin corregir.
- `mem review show` reconstruye el linaje completo de cada hallazgo: fuentes, ronda
  de corrección, lo que declaró cada revisor y el estado agregado.
- `review_status` devuelve target original y vigente, política, revisores, recuentos
  por clasificación/severidad/re-juicio y el linaje de cada hallazgo.

### Migración

Aditiva. Una base anterior abre sin cambios: su target vigente se interpreta como el
original y sus revisiones siguen autorizando corrección, que es su comportamiento
histórico. Ningún hallazgo se borra.

## [2.15.0] - 2026-08-29

### Novedades

#### Revisión adversarial por consenso

Nuevo protocolo de revisión con dos revisores independientes de solo lectura. Cada revisión trabaja sobre un target congelado (hash SHA-256 del contenido a evaluar), lo que garantiza que ambos revisores ven exactamente lo mismo sin importar cuándo ejecuten.

El flujo: `review start` congela el target, `review submit` recibe los hallazgos de cada revisor de forma independiente, `review consensus` cruza los hallazgos buscando convergencia, y `review finalize` decide si el cambio se aprueba, escala o corrige. Los hallazgos deconvergentes quedan registrados para análisis posterior.

Las herramientas MCP disponibles son `review_start`, `review_submit`, `review_consensus`, `review_finalize` y `review_fix`. Cada una maneja su fase del protocolo y valida las transiciones de estado.

#### El ledger de revisión redacta secretos

Los campos `claim`, `evidence` y `verification` de cada hallazgo pasan por el mismo pipeline de redacción que usa la memoria persistente. Cuando `redact_secrets_enabled` está activo (lo es por defecto), los tokens, credenciales y datos sensibles se reemplazan por `<private>` antes de escribirse al ledger.

#### Promoción de hallazgos a memorias

`review promote` convierte un hallazgo de consenso en una memoria persistente del proyecto. Esto cierra el ciclo entre la revisión y el sistema de memoria: un problema encontrado por los revisores termina como conocimiento estructurado que los agentes pueden consultar.

### Cambios

#### Limpieza del repositorio

`.agents/`, `.specify/` y `.github/workflows/` se retiraron del tracking de git. Son generados localmente por speckit y las herramientas de setup, y no deberían versionarse.

## [2.14.0] - 2026-08-29

### Novedades

#### Codex recibe contexto en cada turno, no solo al arrancar

Hasta ahora Codex obtenía el protocolo de memoria una sola vez al inicio de la sesión, en un archivo estático que se diluye a medida que crece la conversación. Se verificó en sesión interactiva real (Codex 0.151.0) que el evento `UserPromptSubmit` dispara y su stdout llega al modelo como contexto, el mismo canal que usa Claude Code para la inyección por turno. La matriz de canales declaraba que Codex no exponía esto, y sobrevivió porque una celda marcada "no aplicable" no la revisa nadie.

Ahora `mem setup-mcp --scope global` escribe el enganche `user-prompt-submit` con `--emit=text` en el config de Codex, y el hook traduce su salida al dialecto correcto por agente.

#### Habilidades disponibles en los tres agentes

Antes, gomemory escribía habilidades únicamente en `.claude/skills/`. Codex y OpenCode nunca recibieron la descomposición atómica ni la constitución como habilidad descubrible. Ahora un instalador genérico (`InstallAgentSkill`) distribuye cada skill al directorio de habilidades de cada agente que lo exponga: `.claude/skills/`, `.codex/skills/`, `.opencode/skills/`. Cada skill tiene un solo dueño que la escribe, y los agentes que no usan ese canal no reciben archivos sobrantes.

#### `mem review`: revisión adversarial por consenso (ACR)

Protocolo completo de revisión con dos revisores independientes sobre un target congelado (diff, commit o archivo). Un defecto solo se confirma cuando ambos lo encuentran por separado; la corrección automática queda limitada a hallazgos confirmados de severidad CRITICAL o HIGH, con un presupuesto de rondas configurable (2 por defecto). El agente propone la clasificación de consenso y la corrección; gomemory valida cada paso contra el estado persistido y deriva el veredicto (APPROVED, ESCALATED, INCOMPLETE) — el veredicto no se puede pasar como parámetro.

Un defecto confirmado y resuelto puede promoverse a memoria del proyecto (problema, causa raíz, resolución, verificación), reutilizando el mismo dedup por `topic_key` de las memorias normales en vez de un almacén paralelo.

CLI: `mem review --diff|--commit|--file`, `status`, `history`, `show`. Siete tools MCP nuevas (`review_start`, `_submit`, `_consensus`, `_fix_record`, `_rejudge`, `_finalize`, `_promote_memory`) más `review_status` de solo lectura. Detalle completo en [`specs/027-adversarial-consensus-review`](specs/027-adversarial-consensus-review/).

Se distribuye además como habilidad (`SKILL.md`) en los tres agentes, agnóstica de proveedor y de las herramientas de gomemory, para que un agente sin este servidor MCP conectado igual conozca el protocolo.

### Correcciones

#### `mem setup-mcp --scope global` sin argumentos registraba solo dos agentes

El valor por defecto de `--agents` era `opencode,claude`. Codex estaba en `globalScopeAgents` pero ausente del defecto, así que quien no supiera pasar `--agents codex` se quedaba sin su ciclo de memoria entero, y el comando terminaba en éxito sin advertir nada.

Ahora el defecto es `opencode,claude,codex`, con un test que fija la invariante: todo agente del defecto debe existir en `globalScopeAgents`.

#### macOS mataba el binario recién instalado con SIGKILL

El script de instalación usaba `cp` directo para reemplazar el binario. En macOS la firma de código se cachea por inodo, y `cp` sobre un ejecutable existente reutiliza el mismo inodo. El resultado era un `mem` que moría con exit 137 en cada invocación, sin mensaje ni error de firma.

Ahora el fallback elimina el binario antiguo antes de copiar, forzando un inodo nuevo. `codesign -v` no reporta problema con ninguno de los dos estados; la diferencia solo se ve al ejecutar.

#### Los tests modificaban el config real de Codex

`go test ./...` escribía en `~/.codex/config.toml` de quien ejecutaba la suite. Varios tests lanzan el binario como subproceso fijando `cmd.Dir` pero no `cmd.Env`, y el hijo hereda el HOME del proceso de test. `t.Setenv("HOME", ...)` protege al código in-process, no a un subproceso sin `Env` explícito.

Los `TestMain` de `tests/contract` y `tests/integration` ahora aíslan HOME y USERPROFILE además del `GOMEMORY_DATA_HOME`. `anclarCachesDeGo()` fija GOCACHE y GOMODCACHE antes de mover el HOME, o cada ejecución recompilaría el mundo.

#### `mem review` podía aprobar una revisión sin calcular el consenso

`review_finalize` derivaba el veredicto sobre los hallazgos de consenso; si ese paso fallaba o simplemente no se ejecutaba, la lista de hallazgos quedaba vacía y `DeriveVerdict` aprobaba con "0 hallazgos" aunque ambos revisores hubieran reportado un defecto. Lo destapó ejecutar el protocolo completo contra el servidor MCP real, no la suite de tests.

`DeriveVerdict` ahora exige que exista consenso calculado cuando algún revisor reportó hallazgos; sin eso, la revisión no está lista para finalizar. De paso, `review_consensus` dejó de exigir `unmatched` cuando no hay hallazgos sueltos que declarar.

#### El ledger de revisión guardaba secretos en claro

Las memorias se redactan al guardarse desde hace varias versiones; las tablas de `mem review` (`findings`, `consensus_findings`, `fix_rounds`) no. Un revisor cita el código que analiza, así que una credencial en esa línea quedaba persistida tal cual y se servía después por `mem review show`.

Ahora `claim`, `evidence` y `verification` pasan por el mismo `RedactSecrets`/`RedactPrivate` que ya protege a las memorias.

## [2.13.0] - 2026-08-25

### Novedades

#### Codex ejerce el ciclo de memoria, no solo lo consulta

Hasta ahora Codex recibía el registro del servidor MCP y nada más: podía consultar la memoria si se le pedía, pero
gomemory no inyectaba contexto al arrancar la sesión ni registraba la actividad al cerrar cada turno. `mem doctor`
tampoco reportaba una sola de sus filas, así que el hueco no era visible: el diagnóstico cerraba con «Sin problemas»
mientras el agente funcionaba con una fracción de sus canales.

La instalación ahora registra en `~/.codex/config.toml` los enganches del ciclo —contexto al arrancar, recuperación
tras compactar y registro de actividad al cerrar el turno— y escribe el bloque de protocolo en `~/.codex/AGENTS.md`.
La escritura preserva íntegra la configuración ajena: solo se añaden los enganches que faltan, reinstalar no los
duplica, y las referencias de confianza las genera Codex al autorizarlos. La bandera `[features] hooks` queda activa,
sin la cual Codex ignoraría la sección entera.

Codex aparece además en `mem doctor` con sus canales propios, y las dos capacidades que su ciclo no ofrece —los bordes
de entrada y salida del modo plan— se reportan como degradaciones declaradas, con su motivo, en vez de desaparecer del
informe.

#### El diagnóstico distingue disponibilidad de ejercicio

Un canal nuevo separa que un agente PUEDA consultar la memoria de que gomemory EJERZA su ciclo en él. Antes, un
registro MCP presente bastaba para dar por sano a un agente que no recibía contexto ni generaba checkpoints.

### Correcciones

#### El contexto vuelve a caber en su presupuesto

El registro automático de actividad era la única sección que no respetaba el techo de `budget`: se emitía completo, sin
acotar. En un proyecto con historia acumulada llegó a ocupar el 68 % de un documento que más que duplicaba su
presupuesto. Ahora se acota como cualquier otra sección, con un puntero `get_memory` al detalle completo.

#### La actividad automática deja de crecer sin límite

Un checkpoint guardaba hasta cinco comandos por turno, pero sin límite de largo: un comando que incluyera un archivo
entero se almacenaba literal. Cada comando queda acotado y se declara cuánto se omitió.

Los checkpoints también se deduplican por contenido. La actividad de un mismo turno se registraba dos veces —una por
el agente principal y otra por el subagente— con títulos distintos y cuerpo idéntico, y esas copias se acumulaban sin
que nada las fundiera.

#### El historial deja de viajar dos veces en la misma sesión

Al entrar en modo plan, gomemory sustituye el historial por un aviso si ya se entregó en esa sesión. La sustitución no
llegaba a aplicarse cuando el contexto lo entregaba el enganche de arranque, que es la vía habitual: la entrega no
quedaba anotada y no había con qué compararla.

#### La versión informada corresponde al código

Una compilación desde el árbol reportaba una versión anterior a la publicada, porque la constante interna se había
quedado atrás respecto de las etiquetas de release.
## [2.18.0] - 2026-09-03

### Novedades

#### Octopus AAR, enrutador adaptativo de agentes

- Añade un módulo opt-in que decide si una unidad se ejecuta en el agente principal o se delega, con presupuesto de contexto y contrato de ejecución. Octopus produce la política; el runtime ejecuta e informa el coste real.
- La política es determinista: aplica 13 reglas en orden fijo y devuelve una razón de un catálogo cerrado.
- Permite enrutar planes con dependencias y grupos paralelos, reutiliza el Context Optimization Engine, protege una reserva de validación y registra resultados para comparar estimaciones con consumo real.
- El módulo empieza apagado. Se activa desde Configuración en la TUI o con `octopus_enabled: true` en `.memory/settings.json`. Mientras está apagado no registra sus tools MCP, no altera el protocolo ni el bootstrap de ToolSearch y no escribe filas en la base.
- Añade `mem octopus plan|route|status|usage|history` y las tools MCP `octopus_route_task`, `octopus_route_plan`, `octopus_report` y `octopus_status`.
- La tabla `octopus_executions` guarda identificadores, enums y cifras, sin columnas de texto libre alimentadas por contenido. Una prueba de contrato protege ese esquema.

## [2.12.0] - 2026-08-25

### Novedades

#### Codex consolida automáticamente sus hooks en una sola fuente

`mem install` y `mem setup-mcp --scope global --agents codex` migran los hooks heredados de
`~/.codex/hooks.json` a `~/.codex/config.toml`. La migración interpreta ambos formatos, conserva eventos, filtros,
comandos, límites y campos compatibles, y elimina grupos semánticamente equivalentes sin distinguir entre GoMemory,
Herdr u otros proveedores.

Antes de modificar los archivos se crean respaldos recuperables. `hooks.json` solo se retira después de serializar y
validar el TOML consolidado; si el JSON es inválido, permanece intacto y el registro MCP de GoMemory continúa por
separado. Las referencias de confianza asociadas a posiciones anteriores se eliminan para que Codex vuelva a autorizar
los hooks desde su ubicación vigente.

## [2.11.1] - 2026-08-24

### Correcciones

#### Constitución predeterminada completamente agnóstica

La constitución ya no conserva referencias al ecosistema Speckit ni al proyecto Kolmena Core. Su texto inicial ahora describe criterios reutilizables para cualquier proyecto nuevo, sin atribuirlos a una organización, proyecto o persona.

Una prueba protege la plantilla contra la reintroducción de esas referencias. Otra verifica que restaurar una constitución solo ocurre mediante una acción explícita, preservando las personalizaciones del equipo durante la instalación y la actualización.

### Documentación y seguridad

La mejora quedó formalizada en la especificación 025, con plan, contratos, modelo de datos, guía de validación y tareas convergidas con el código publicado. El archivo local `.env` se añade a las exclusiones de Git para impedir que una credencial de desarrollo se incorpore por accidente.

## [2.11.0] - 2026-08-24

### Novedades

#### Copiar, pegar y desplazar contenido en la TUI

La TUI permite copiar la vista actual con `Ctrl+Y`. En el detalle de una memoria, la copia incluye el contenido completo aunque solo una parte esté visible. Los campos activos también reciben correctamente el contenido pegado, incluido el pegado entre corchetes del terminal.

Los detalles extensos se pueden recorrer con las flechas, `j`/`k`, `PgUp`/`PgDn`, `Home`/`End` y `g`/`G`.

### Correcciones

#### Codex usa un único registro MCP global

Las instalaciones anteriores creaban una tabla `gomemory_*` por proyecto y fijaban su directorio con `cwd`. Cuando ese directorio dejaba de existir, Codex intentaba iniciar el servidor desde una ruta inválida y mostraba `No such file or directory (os error 2)`.

La instalación ahora migra esas tablas a un único registro `[mcp_servers.gomemory]`, ejecuta `mem mcp` sin depender del directorio de un proyecto y conserva intacta la configuración de otros servidores. Antes de reemplazar el archivo crea un respaldo, mantiene sus permisos y realiza una escritura atómica.

#### Constitución predeterminada sin referencias específicas

La constitución incluida por defecto ya no contiene el título ni la autoría asociados a una organización o persona. El contenido queda disponible como base agnóstica para cualquier proyecto compatible.

## [2.10.2] - 2026-08-23

### Cambios en `mem install`

#### OpenCode registra su memoria en el scope usuario, ya no por proyecto

El registro del servidor MCP y los permisos de las tools se escribían en el `opencode.json` de cada proyecto. Ese registro duplicaba lo que la configuración global del agente ya resuelve: OpenCode combina el nivel usuario con el de proyecto, y el plugin siempre se instala en el directorio global.

Ahora `mem install` registra el servidor y sus permisos una sola vez en `~/.config/opencode/opencode.json`, y retira el registro que instalaciones anteriores dejaron en el proyecto. Si tras retirarlo el archivo solo conserva el `$schema`, se elimina. Si conserva configuración ajena a gomemory, se reescribe sin esas claves y permanece.

`mem doctor` verifica la migración con un canal nuevo (`opencode · user · server_config`): se muestra en verde cuando el registro global existe y como problema con su remedio cuando falta. Antes, perder esa entrada no producía ningún síntoma.

## [2.10.1] - 2026-08-23

### Correcciones

#### La vitalidad de los canales era ciega para Claude Code

El informe de vitalidad solo vigilaba el canal de entrada al plan de OpenCode: los hooks de Claude Code no dejaban rastro de actividad, así que un cambio silencioso del agente dejaba la inyección muerta y el informe seguía en verde.

Ahora la entrada al plan registra el ejercicio del canal (`claude · user · plan_entry`), en paridad con lo que hace el complemento de OpenCode, y el informe vigila ambos agentes. Un canal que demostró funcionar y se apagó aparece como problema con su efecto y su remedio; uno que nunca se ejerció sigue sin alarmar, porque las sesiones no se atribuyen por agente.

#### Salida doble y ruta inválida en la entrada al plan

Ante un fallo al localizar el proyecto, o con el gate del plan atómico deshabilitado, el hook `plan-entered` continuaba ejecutándose después de emitir su salida de silencio: podía trabajar sobre una ruta inválida y, con el gate activo, emitía el documento vacío dos veces.

Ahora cada salida temprana termina la ejecución del hook.

#### Un conteo de sesiones fallido se confundía con cero

Si la consulta de sesiones fallaba, `SessionsSince` devolvía cero: el informe trataba un dato que no pudo leerse como evidencia de que nadie trabajó. En la misma línea, si la lectura del contexto o del recordatorio de guardado fallaba dentro del complemento de OpenCode, la inyección se reducía sin dejar rastro — justo lo que la vitalidad quería evitar.

Ahora un conteo fallido devuelve «desconocido» y el informe calla donde no hay evidencia en ninguna dirección; en el complemento, esos fallos quedan anotados como `channel-error`, con el mismo rastro que ya tenían los fallos del checkpoint de turno.

## [2.10.0] - 2026-08-23

### La matriz de canales como fuente única

Qué artefacto recibe cada agente estaba declarado en catorce tablas repartidas por el código, y solo tres se consultaban realmente: al cambiar un artefacto había que acordarse de editar todas, y el olvido de la mayoría producía los bugs recurrentes entre agentes.

Ahora esa relación vive en una sola matriz (`domain/channel_matrix.go`) y los consumidores derivan de ella lo que necesitan. Un test de contrato verifica que cada agente con soporte tenga sus canales declarados y coherentes con lo que la instalación escribe.

### Economía del contexto en plan-context

La operación de contexto para planificar reenviaba íntegro el historial que la sesión ya había recibido por otro canal. Medido en este repositorio: el contexto de planificación pasó de 12.214 a 1.057 tokens (91 % menos), y el historial duplicado era de unos 11.156 tokens.

Ahora cada entrega queda registrada por sesión y por canal. Una entrega posterior suprime el material ya enviado; una sesión nueva arranca con el historial completo porque no hereda ese registro.

Si la sesión perdió el material —por una compactación, por ejemplo— y el registro sigue creyendo que el agente lo tiene, `mem plan-context --full` entrega el historial completo ignorando el registro.

### Diagnóstico accionable en `mem doctor`

El informe nombraba el archivo ausente y nada más: saber si el problema importaba y encontrar el comando correcto exigía conocer el sistema por dentro.

Ahora, para cada canal roto, el informe describe qué deja de funcionar y propone el comando que lo restablece. Los remedios se agrupan cuando varios canales se corrigen con el mismo comando, para no sugerir ejecuciones repetidas. La salida `--json` entrega los mismos datos como campos (`efecto`, `remedio`, `remedio_advierte`).

### Vitalidad de los canales

El informe comprobaba que el artefacto existiera, pero un complemento presente cuyo agente renombró la operación que usa queda muerto con el archivo intacto: presencia y salud no son lo mismo.

Ahora el sistema registra cuándo se ejerció cada canal por última vez y qué falló si algo falló:

- El complemento de OpenCode anota cada ejercicio correcto (`mem hook channel-fired`) y cada fallo (`mem hook channel-error`) que antes absorbía en silencio.
- Un test de contrato verifica que los hooks que registra el complemento siguen declarados en la interfaz publicada por OpenCode: si el agente renombra una operación, el fallo aparece en la batería de tests y no en la máquina de quien usa la herramienta.
- El informe distingue un canal sin uso porque no hubo trabajo de uno que no responde habiéndolo: con sesiones recientes y sin actividad del canal, lo reporta como problema; sin sesiones, lo declara como degradación que no requiere acción.

### Correcciones

#### La desinstalación dejaba la configuración MCP de algunos agentes

La lista de archivos de configuración MCP estaba escrita a mano y solo conocía el esquema de algunos agentes: la entrada de un agente con esquema propio sobrevivía a toda desinstalación, sin errores ni avisos.

Ahora la lista se deriva de la matriz de canales, así que ya no pueden separarse. Los registros que pertenecen a la persona y no al proyecto —como `~/.codex/config.toml`— quedan fuera por diseño y el informe lo declara.

#### `mem uninstall` no retiraba los artefactos de OpenCode

La desinstalación retiraba la configuración de Claude pero dejaba intactos los permisos pre-aprobados que la instalación escribe en `opencode.json`.

Ahora retira esos permisos con la misma simetría. El plugin de OpenCode no se elimina: reside en el HOME del usuario y lo comparten todos los proyectos.

## [2.9.0] - 2026-08-23

### Cambios en `mem install`, reglas y constitución

A partir de esta versión, `mem install` **ya no escribe archivos de instrucciones en el repositorio destino**.

El bloque que anteriormente se inyectaba en `AGENTS.md`/`CLAUDE.md` duplicaba información que el servidor MCP ya entrega en la respuesta `initialize`. Además, la constitución que se copiaba al repositorio era una versión congelada de 635 líneas. Mantener ambas copias introducía un riesgo evidente: bastaba modificar una de ellas para que terminaran divergiendo.

En su lugar, esta versión centraliza la información en memoria y mantiene dos documentos:

- **Reglas de trabajo:** `get_context()` las entrega completas en cada sesión.
- **Constitución:** permanece disponible en memoria y se consulta cuando es necesaria.

#### Las reglas son un punto de partida

Las reglas incluidas por defecto proporcionan un punto de partida para el equipo. Sin embargo, ofrecerlas sin una forma sencilla de reemplazarlas convertiría a `gomemory` en la autora de las normas de cada equipo.

Por eso esta versión incorpora `mem docs`, que permite consultar, exportar, importar y restaurar los documentos administrados por memoria:

```bash
mem docs list
mem docs export rules -o reglas.md
mem docs import rules reglas.md
mem docs reset rules
```

Las cuatro operaciones también están disponibles desde la TUI. Un **test de contrato** garantiza que ambas superficies mantengan el mismo comportamiento.

El catálogo interno utiliza un diseño **table-driven**: incorporar un nuevo documento requiere agregar una entrada al catálogo, sin necesidad de crear un nuevo comando ni una nueva pantalla.

### Novedades

- **Inicialización automática de reglas y constitución:** `mem install` y el arranque del servidor MCP crean estos documentos únicamente cuando todavía no existen.
- **Los documentos existentes nunca se sobrescriben:** una vez creado, el documento pasa a estar bajo control del equipo y las operaciones posteriores respetan su contenido.
- **Nuevas operaciones de documentación:** se incorporan `mem docs`, `mem constitution [--sync]`, `mem rules` y `mem seed`.
- **Envoltorio `/constitution`:** disponible para Claude Code y OpenCode. No mantiene una copia del texto; resuelve la constitución directamente desde la memoria cuando se invoca.
- **Instalación automática de Windsurf y Cline eliminada:** ambos agentes dejaban una carpeta en la raíz de cada proyecto para almacenar un único JSON. Siguen disponibles de forma explícita mediante:
  ```bash
  mem setup-mcp --agents windsurf,cline
  ```

### Correcciones

#### `ListMemories` no devolvía `topic_key`

`ListMemories` no incluía `topic_key`, a diferencia de `ListAllMemories`. Como consecuencia, la columna llegaba vacía a todos los consumidores que utilizaban esta vía, sin generar errores ni advertencias.

Ahora ambas operaciones mantienen la información necesaria.

#### La inicialización podía publicar la constitución en un ADR externo

Con `adr_sync_enabled=true`, la instalación podía publicar accidentalmente la constitución completa en el ADR externo configurado por el usuario. Esto podía ocurrir de forma síncrona y sin una acción explícita del usuario.

La inicialización, importación y restauración utilizan ahora una **inserción sin efectos secundarios**, evitando cualquier sincronización externa no solicitada.

La depuración de secretos continúa activa. Esta protección forma parte del procesamiento de seguridad y no se utiliza como canal lateral para la sincronización.

#### Las memorias fijadas podían desaparecer del contexto

Una memoria fijada podía dejar de aparecer silenciosamente porque su inclusión dependía de la ventana de recencia.

Ahora las memorias fijadas se resuelven mediante su **clave de tópico**, garantizando su presencia independientemente de la antigüedad.

#### `mem install` podía utilizar el store incorrecto

Cuando `mem install` se ejecutaba desde un directorio diferente al repositorio destino, podía crear las memorias en el store equivocado.

Además, el comando podía informar que los documentos iniciales ya estaban presentes aunque en realidad no se hubiera escrito nada.

Ahora la instalación resuelve correctamente el destino antes de realizar la inicialización y reporta el estado real de la operación.

#### `mem docs export` no escribía correctamente el archivo

El comando:

```bash
mem docs export <alias> -o <archivo>
```

terminaba escribiendo el contenido en `stdout` y dejando vacío el archivo indicado.

La causa estaba en el parser de flags de Go, que deja de procesar flags después del primer argumento posicional. Se corrigió el procesamiento de los argumentos para que `-o` funcione correctamente.

#### Flake en los tests de integración

Se corrigió un flake preexistente en los tests de integración.

Un proceso desacoplado continuaba escribiendo en `.memory/` después de que el test hubiera terminado, provocando una condición de carrera con la limpieza del directorio temporal.

Ahora el proceso se gestiona correctamente antes de finalizar el test, evitando la escritura posterior y la competencia con la limpieza.

## [2.8.0] - 2026-08-20

### Added

- **`mem usage`** — a measured token benchmark per session. Reports baseline
  tokens (what a response would have cost unoptimized), emitted tokens, the
  saved delta and reduction ratio, broken down by operation and by channel
  (`mcp`, `cli`, `tui`). Available as `mem usage [--session ID|--all] [--json]`.
  The header always declares that the counting method is a neutral
  approximation (~4 chars/token), not any provider's tokenizer — figures are
  comparable against themselves, never against anyone's billing. An optional
  reference-window setting (`usage_window_tokens`, off by default) adds an
  estimated "footprint avoided" percentage, clearly labeled `(estimated)`.
  Machine-readable contract: [`docs/USAGE-REPORT-CONTRACT.md`](docs/USAGE-REPORT-CONTRACT.md).
- **Usage screen in the interactive UI** (`u` key) — shows the same session
  report as `mem usage`, plus an on-demand context-optimization snapshot for
  a specific task (same engine as `mem pack build`), which never persists
  between visits.
- **`mem consolidate [--apply]`** — merges redundant memories within a
  project by two criteria: shared topic key, and automatic activity
  checkpoints with byte-identical content. Previews by default (the
  operation is irreversible); no content is lost, distinct text within a
  merged group is concatenated into the row that's kept. Also available from
  the interactive UI's Maintenance screen.
- **`mem get <id>`** — retrieves a memory's full detail by ID from the
  command line, mirroring the `get_memory` MCP tool's capability on the CLI
  channel.
- **Index mode for `get_context`** (`context_index_mode` setting, off by
  default) — emits the working protocol in full plus a one-line index per
  memory (id, type, title), with detail fetched on demand via the existing
  `get_memory` capability. Reversible: toggling it off returns emission to
  byte-identical output.

### Changed

- `charmbracelet/bubbletea` v0.26.1 → v1.3.10, `bubbles` v0.18.0 → v1.0.0,
  `lipgloss` v1.0.0 → v1.1.0. No application code changes were needed; every
  existing screen and test kept its exact behavior.

### Fixed

- **MCP usage-recording middleware never fired.** The fallback middleware
  that records non-self-reporting tool calls (`save_memory`, `get_memory`,
  and others) type-asserted the request params to `*mcp.CallToolParams`, but
  the SDK hands middlewares the *unparsed* params (`*mcp.CallToolParamsRaw`)
  — the assertion silently failed for every call. Found by an end-to-end
  test that drives a real MCP client against the real server, not a
  schema-listing check. Fixed and covered by two new integration tests.
- `search_memories`/`list_memories` usage accounting could report emitted
  tokens higher than baseline tokens for short memories, because the raw
  baseline only summed memory content and ignored the rendered wrapper
  (id/type/title/formatting). Baseline is now derived as emitted + what was
  actually truncated, guaranteeing baseline ≥ emitted.
- `ListAllMemories` never selected the `topic_key` column, so every memory
  loaded through it appeared to have no topic key regardless of what was
  stored — this silently broke topic-key grouping before it was ever used.
- `mem usage`'s "no session" report collapsed with the "all sessions"
  report internally (both used an empty session ID), so an idle project
  could show accumulated historical totals instead of zeros.

## Earlier releases

See [GitHub Releases](https://github.com/Sayoner-000/gomemory/releases) for
v2.7.0 and earlier.
