# Socratica: refinamiento de prompts, escrutinio socrático y preparación para Spec Kit

Socratica convierte un prompt en bruto en uno fiel y ejecutable, examina una propuesta antes de actuar o prepara una necesidad para `speckit-specify`. Hace como máximo tres preguntas con opciones, propone una reescritura y **no ejecuta nada sin tu aprobación explícita**. Si cancelas, recuperas tu original intacto.

## Modos e invocación

| Modo | Para qué | Cómo se invoca |
|---|---|---|
| `refine` | Aclarar y mejorar un prompt: audiencia, criterio de éxito, restricciones, formato, alcance. Lo que falta queda como `[marcador]`. | `/socratica <texto>` en el agente; `mem socratica refine` en la terminal; tecla `p` → `r` en la TUI |
| `scrutiny` | Examinar supuestos, evidencia, alternativas y criterios de una propuesta, sin defender por defecto la hipótesis inicial. | `/socratic-q <texto>`; `mem socratica refine --mode scrutiny` |
| `specify` | Preparar una idea, solicitud o mejora técnica como entrada estructurada y comprobada para Spec Kit. Ver [Preparar una necesidad para Spec Kit](#preparar-una-necesidad-para-spec-kit-specify). | `/socratica-hu <texto>` (Claude Code y OpenCode); `$socratica prepara para speckit-specify: <texto>` (Codex); «Usa Socrática para preparar esto como entrada de Spec Kit: …» en los tres |

La invocación exacta depende del agente (habilidad nativa, comando con nombre o `$socratica` en Codex). La tabla vigente está en la [matriz de compatibilidad](../specs/039-socratica-prompt-refinement/contracts/compatibility-matrix.md). Si un agente no reconoce el comando, pídelo en lenguaje natural («aplica socratica a: …»).

La habilidad se instala en `~/.claude/skills/`, `~/.codex/skills/` y `~/.config/opencode/skills/` con `mem setup-mcp --scope global`, `mem install` (alcance global) y `mem update`, solo en los agentes que ya existen en tu equipo. También puedes copiarla a mano desde [`skills/socratica`](../skills/socratica/).

Junto a la habilidad se escriben envoltorios de comando finos, que solo nombran la habilidad y el modo: `~/.claude/commands/socratic-q.md` y `socratica-hu.md` (Claude ya expone `/socratica` como habilidad) y `~/.config/opencode/commands/socratica.md`, `socratic-q.md` y `socratica-hu.md`. Codex no tiene un directorio de comandos verificado, así que no recibe ninguno. Igual que la habilidad, se escriben solo si existe el directorio del agente y el contenido ha cambiado.

## La aprobación es la compuerta

Antes de que pulses Ejecutar no se hace ningún trabajo: Socratica solo lee contexto para aclarar la petición. Al aprobar:

- **Dentro del agente**: la habilidad llama a `socratica_flow(action=approve)`, que asigna un `dispatch_id`, y después a `socratica_flow(action=ack, dispatch_id, ack_nonce)` **antes** de ejecutar. Solo ejecuta si el ack responde `acked=true`.
- **Desde la CLI o la TUI**: `SocraticaFlow.Execute` aprueba, reserva el despacho, arranca el agente destino con el prompt aprobado y confirma el envío en cuanto el agente arranca.

gomemory registra que la autorización se entregó a un único agente; nunca afirma que la tarea se ejecutó.

### Prevención de duplicados

- Un segundo `approve` sobre el mismo flujo devuelve `duplicate=true` y el mismo `dispatch_id`. Tras el ack devuelve `already_dispatched=true`, y la habilidad no vuelve a ejecutar.
- Si se pierde la respuesta del ack, se repite con el **mismo** `ack_nonce` y se obtiene otra vez `acked=true` sin consumir otra autorización.
- Al lanzar desde la CLI o la TUI, el agente recibe `GOMEMORY_SOCRATICA_DISPATCH=<dispatch_id>` en su entorno. Si la intercepción está activa, el hook reconoce el prompt aprobado una sola vez y no lo vuelve a capturar; el siguiente prompt de la sesión se evalúa con normalidad.
- Si el agente arrancó pero no se pudo confirmar el envío, el flujo queda **incierto**: no se relanza nada. `mem socratica pending --resume <id>` te deja elegir entre marcarlo como enviado, reenviarlo o cancelarlo.
- Reenviar un flujo ya enviado es siempre una elección explícita y crea una autorización nueva.

### Un solo pendiente por agente

Cada agente tiene como máximo un flujo sin terminar por proyecto. Empezar otro retoma el pendiente con su modo real, sin convertirlo; para empezar de cero, cancélalo (`mem socratica pending --cancel <id>`).

## Preparar una necesidad para Spec Kit (`specify`)

`specify` conduce paso a paso una idea, una petición o una mejora técnica (no hace falta que esté redactada como historia de usuario) hasta una entrada completa, comprobada y aprobada por la persona, que se entrega a `speckit-specify`. Socratica **no genera la especificación ni ejecuta la necesidad**: prepara, comprueba y entrega; la especificación la genera la skill de Spec Kit.

Quién hace qué: el agente conduce la conversación y propone el contenido; gomemory decide la etapa, calcula las comprobaciones y el texto final y controla qué acciones están permitidas. El agente nunca envía validez, huellas ni permisos: se rechazan.

### Recorrido

| Etapa | Qué se prepara | Condición para avanzar |
|---|---|---|
| `intent` | Objetivo, resultado esperado y actores (un servicio o proceso es un actor válido). | Campos con información real: ni vacíos, ni relleno (`TBD`, «por definir», solo signos), ni un marcador `[sin resolver]`. |
| `scope` | Lo incluido y, declaradas expresamente, las listas de excluido, restricciones, supuestos y pendientes. `declared=true` con `items=[]` significa «no se identifica ninguno». | Alcance incluido descrito y listas declaradas; los pendientes pueden seguir abiertos aquí. |
| `stories` | Historias, criterios verificables y entregas con identificadores únicos. | Referencias existentes; cada historia con al menos un criterio y dentro de una entrega; cada entrega con criterios que comprueban sus historias. |
| `validation` | Todo el contenido. | Sin pendientes **bloqueantes** abiertos ni marcadores esenciales. Al pasar se genera el texto final y la fase pasa a `review`. |

- Las etapas se comprueban en ese orden: no se puede saltar una (`stage_blocked`) ni apoyarse en una comprobación cuyas entradas cambiaron (cada una guarda la huella de lo que comprobó).
- `prepare(set)` reemplaza una sección entera y exige la revisión leída (`expected_revision`). Un cambio material sube la revisión e invalida las comprobaciones posteriores, la revisión y cualquier autorización sin consumir; un valor idéntico no cambia nada. Una revisión vieja se rechaza con `revision_conflict`.
- Un fallo de comprobación no es un error: devuelve `pending_checks` (código, ruta y mensaje) y conserva el borrador.
- Un pendiente bloqueante abierto no se puede borrar ni degradar en silencio: se resuelve con una nota que identifique la decisión o la evidencia. Omitir una pregunta no lo resuelve y un supuesto tampoco.

### Preguntas

Como máximo **tres en total por flujo**, contando reanudaciones y ediciones; el contador no se reinicia. Cada una lleva de dos a cuatro opciones más «Otra», «Omitir» y «Omitir el resto». Se permiten cero. Agotado el límite, se muestra qué falta y se completa editando o aportando información voluntaria. Al terminar de responder se vuelve a preparar secciones: nunca a una reescritura libre.

### Revisión y aprobación

Con la preparación comprobada, el agente muestra el original y la entrada final **completa** (con supuestos y pendientes no bloqueantes) y ofrece «Aprobar y generar especificación», «Editar», «Copiar» y «Cancelar». Las comprobaciones cubren completitud, relaciones y bloqueos; la persona confirma que el texto refleja su necesidad. Solo esa elección permite llamar a `approve`, con la revisión que se mostró.

- El texto aprobado es, byte a byte, el revisado: la huella (`review_hash`, SHA-256 de los bytes exactos) se distingue de la huella normalizada que usa el hook para reconocer el prompt aprobado.
- Aprobar, volver a leer o retomar **nunca concede ejecución**. Si el contenido cambió entre la revisión y la aprobación, o la depuración de privacidad lo reescribió, la aprobación se rechaza (`review_stale`).
- `rewrite`, la edición de texto libre y `send_original` no existen en este modo; las ediciones van por `prepare`.

### Entrega única

1. **Disponibilidad primero.** La skill comprueba que puede cargar e invocar `speckit-specify` en el proyecto (un binario `specify` en el PATH no lo demuestra). Si no puede, conserva la preparación y la aprobación, muestra la dependencia y **no** llama a `ack`: no se consume la autorización.
2. **Confirmar.** `ack` con `dispatch_id` y un `ack_nonce` nuevo, que se conserva para reintentar.
3. **Una sola concesión.** Solo la primera confirmación que llega a confirmarse devuelve `execution_granted=true`; solo entonces se invoca `speckit-specify` una vez con exactamente `approved_prompt`. Repetir el mismo nonce (`acked=true` sin concesión), usar otro (`already_dispatched=true`), aprobar de nuevo o leer no vuelven a conceder. Con dos confirmaciones a la vez gana una; con una confirmación revertida no hay ninguna.
4. **Respuesta perdida.** Si se perdió la primera respuesta y no hay evidencia de que se entregó, no se ejecuta desde el replay: la persona decide entre reenviar (`force_resend`, una autorización nueva) o dejarlo como está. `mark_dispatched` no aplica a este modo: `specify` nunca pasa por el ejecutor local que reclama el despacho. Reenviar exige que no haya otro flujo pendiente para el mismo agente.
5. **Resultado comunicado.** El agente abre el `spec.md` y su checklist reales, comprueba el alcance y traza cada criterio a sus historias, y lo comunica con `report` (`completed`, `failed` o `uncertain`, con rutas relativas y trazabilidad). gomemory valida el formato y las referencias, no el contenido del documento, y lo rotula como *comunicado por el agente*. Un acuse o el estado `dispatched` no prueban que se generó nada, y reportar no reabre la autorización.

### Recuperación, consulta local y privacidad

- **Persistente** (por defecto): el estado sobrevive al cierre; al retomar se conservan decisiones, relaciones, pendientes, revisión vigente y preguntas consumidas. Una aprobación sin reclamar sigue siendo válida mientras la revisión lo siga siendo.
- **Privado** (`mem socratica private on`, antes de iniciar la sesión del agente: el servidor MCP lee el ajuste al arrancar): el contenido (original, preguntas, preparación, revisión, aprobación y rutas del resultado) vive solo en la memoria del proceso MCP que inició el flujo. En la base queda una **reserva sin texto** (identidad, fase, revisión, marcas de despacho) que mantiene el flujo pendiente único entre procesos. No hay historial. La política se fija al empezar: activar o desactivar el modo después no mueve el contenido al disco.
- **Instancia sin el contenido** (otro proceso o tras una pérdida): solo ve la reserva (`private_state_unavailable`), también si ya se entregó. Puede consultar y cancelar; no puede preparar, aprobar, confirmar, reportar ni reenviar, y nunca reconstruye una autorización. Cancelar la reserva libera el hueco para empezar de nuevo.
- **Consulta local**: `mem socratica pending --resume <id>` y la TUI (`p`) muestran fase, etapa, revisión, preguntas usadas y lo que falta, e indican retomarlo en el agente. En este modo no generan, no ejecutan, no editan texto y no reenvían; solo permiten cancelar un flujo sin terminar. `mem socratica history --show` de un `specify` ya enviado tampoco ofrece reenviarlo: eso se decide en el agente.
- La depuración histórica (`mem sanitize`) cubre la preparación y su historial: si cambia el contenido de un flujo sin consumir, la revisión y la autorización quedan invalidadas y el flujo vuelve a preparación con una revisión nueva.

## Intercepción automática (opcional)

Está **desactivada** por defecto. Cuando está activa, un prompt normal recibe una directiva para entrar en Socratica antes de actuar; el prompt original no se reescribe ni se bloquea. No se captura un comando con `/`, un mensaje con adjuntos, uno de tres palabras o menos, una respuesta a un flujo pendiente ni el prompt aprobado que se acaba de lanzar. Empieza el mensaje con `>` para enviarlo sin refinar.

El prefijo `>` es una vía de escape para la persona, no una frontera de seguridad: quien escribe el prompt puede saltarse siempre la intercepción. Lo que no se puede falsificar escribiendo es el origen «aprobado»: solo lo concede un `dispatch_id` real cuyo texto coincide con el aprobado, y una sola vez.

Solo se activa desde la CLI o la TUI, nunca desde el propio agente: `/socratica on` dentro del agente te devuelve la orden exacta.

```sh
mem socratica on --client claude      # solo si la capacidad tiene evidencia de aceptación
mem socratica on --client claude --unverified   # para obtener esa evidencia; status lo marca «sin verificar»
mem socratica off --client claude
mem socratica status
```

Hoy ninguna capacidad tiene evidencia registrada, así que `on` sin `--unverified` responde «no soportado».

## Historial y privacidad

- `mem socratica history` lista los últimos 20 refinamientos por agente (`socratica_history_limit`). `--show <id>` muestra el original y la propuesta, aunque el flujo ya se haya purgado.
- Los textos pasan por la misma protección que las memorias: se eliminan las regiones `<private>` y se ocultan credenciales, al escribir y al leer. `mem sanitize` cubre también las tablas de Socratica.
- `mem socratica private on` desactiva el historial del proyecto. En modo privado, un flujo enviado se **redacta** (se vacían sus textos y se conserva solo lo necesario para reconocer el despacho) y uno cancelado se borra.
- Los prompts y respuestas de Socratica no quedan como origen de las memorias que se guarden en ese turno.
- En `specify`, el modo privado no escribe contenido en disco ni historial; véase la sección anterior para la reserva sin texto. Un flujo `specify` conserva la política con la que se creó: si el modo privado se activa después, no se redacta al enviarse (su resultado se puede seguir comunicando con `report`), aunque ya no se registra en el historial.
- `mem gc` purga los pendientes con más de 7 días (salvo los aprobados sin confirmar) y los terminados con más de 24 horas.

## Coste en tokens

Un refinamiento añade la generación de hasta tres preguntas y una reescritura, más los turnos de respuesta. Con la intercepción activa, cada prompt capturado lleva además una directiva corta. Fuera del agente, la generación usa el agente que ya tienes instalado y autenticado, sin herramientas ni escritura (`claude -p … --tools ""` o `codex exec --sandbox read-only …`), así que no necesita credenciales nuevas.

## Uso desde la CLI

Cada flujo nuevo muestra un selector de harness: Claude Code, Codex u OpenCode.
La elección sirve para preguntas, reescritura y ejecución aprobada. No se guarda
como preferencia ni cambia automáticamente ante un fallo. Las opciones no
disponibles muestran su motivo. Los paneles de progreso y los selectores
reutilizan la presentación de `mem index`; la animación termina antes de pedir
entrada o entregar la terminal al harness.

```sh
mem socratica refine "envía el informe al cliente"         # preguntas y revisión interactivas
mem socratica refine --client codex --mode scrutiny -      # texto por stdin
mem socratica refine --print "texto"                        # imprime >prompt aprobado en vez de lanzar el agente
mem socratica pending [--resume <id> | --cancel <id>]
mem socratica history [--client <agente>] [--show <id>] [--json]
mem socratica private on|off
```

`--client claude|codex|opencode` selecciona el mismo harness para refinar y ejecutar, y omite el selector. Es obligatorio sin TTY; en ese caso nunca se ejecuta nada: se imprime la revisión y el flujo queda pendiente. En la revisión puedes ejecutar, editar, copiar (OSC 52, o un bloque delimitado) o cancelar. Escape sale de un selector sin ejecutar; si el flujo ya existe, queda pendiente. Con `--print`, stdout contiene solo el prompt aprobado y los paneles van a stderr. Códigos de salida: 0 éxito, 1 error de uso o validación, 2 harness no disponible o sin elección explícita cuando no hay TTY, 3 memoria no inicializada.

Si falla la generación externa, **Continuar asistido** abre el harness elegido
con tu original y las instrucciones para hacer preguntas, proponer y esperar
aprobación en la conversación. No le entrega el diagnóstico como tarea ni
aprueba la ejecución del original. Al salir del harness, la consola termina sin
reiniciar la generación fallida. Con `--print` esta acción no se ofrece.

También puedes **Cambiar harness** para iniciar otro refinamiento con el mismo
original; no se reemplaza un pendiente ajeno del harness elegido. Los rechazos
de proveedor, autenticación o esquema no ofrecen un reintento idéntico. Solo
los fallos transitorios (tiempo de generación agotado o respuesta mal formada)
permiten reintentar explícitamente; cualquier otro fallo, aunque su redacción
sea nueva, no ofrece reintento. En la TUI, `a` abre
el refinamiento asistido y `h` cambia de harness desde un flujo fallido.

## Uso desde la TUI

Pulsa `p` en la lista principal. La pantalla muestra el seguimiento «Tu prompt · Preguntas · Reescritura · Revisión», los pendientes y el historial. `r` abre el selector de harness: flechas para elegir, Enter para confirmar y Escape para cancelar. Después escribe el prompt. La selección se pide incluso si solo hay un harness disponible; aparece en la cabecera y se conserva al retomar un pendiente. `enter` abre un detalle, `i` alterna la intercepción del agente seleccionado (respetando la evidencia) y `p` el modo privado. Ejecutar cede la terminal al agente y la TUI vuelve al terminar.

## Ajustes

Viven en `.memory/settings.json` del proyecto y se validan al cargar: un valor inválido falla con la clave y el valor, sin sustituirlo por el de fábrica.

| Clave | Defecto | Efecto |
|---|---|---|
| `socratica_interception` | *(ninguno)* | Intercepción por agente (`claude`, `codex`, `opencode`). Solo se cambia con `mem socratica on`/`off` o la TUI, nunca por MCP |
| `socratica_interception_unverified` | *(ninguno)* | Marca los agentes activados con `--unverified`; `status` los muestra «sin verificar» |
| `socratica_private` | `false` | Modo privado: sin historial (`mem socratica private on`/`off`) |
| `socratica_history_limit` | `20` | Refinamientos que se conservan por agente (1–1000) |
| `socratica_generators` | `["claude","codex","opencode"]` | Harnesses habilitados para refinar; el orden no establece prioridad ni reemplazo automático |
| `socratica_generator_timeout_seconds` | `120` | Tiempo máximo del generador. Además, su salida tiene tope: 8 MiB en stdout y 64 KiB en stderr. Al superarlo, el proceso se cancela al momento y la generación falla como respuesta inválida |
| `socratica_query_timeout_ms` | `5000` | Tiempo máximo de cada consulta al almacén (mínimo 100) |

`mem socratica status` muestra por agente la intercepción y las capacidades, el modo privado y la disponibilidad de los harnesses; `socratica_flow(action=settings)` devuelve por MCP la intercepción, las capacidades y el modo privado del cliente. Los pendientes conservan su cliente; retomar uno no abre un selector nuevo.

## Limitaciones

- Ningún agente ofrece paneles nativos para Socratica: la revisión es texto (o la TUI de `mem`).
- El adaptador OpenCode requiere V2. Usa un agente temporal con todas las herramientas denegadas y conserva la autenticación y el modelo configurados en OpenCode. La aceptación de generación real sigue pendiente: el proveedor configurado respondió `403` (`FreeTierError`) en la prueba local. El error se muestra sin cambiar de modelo ni de harness.
- Las capacidades por agente (comando con nombre, intercepción, portapapeles, devolver el original al campo de entrada) se habilitan solo con evidencia de aceptación en sesión real; consulta la matriz.
- `specify`: la puerta automática usa un agente de Spec Kit sintético y comprueba quién puede ejecutar, cuántas veces y con qué texto exacto. El recorrido completo con generación real en Claude Code, Codex y OpenCode se registra por agente en `specs/042-socratica-specify-preparation/evidence/`; mientras un agente no tenga su evidencia, ese recorrido está **pendiente** para él.
- `specify`: la calidad de las preguntas y del contenido que propone el agente depende del modelo; gomemory comprueba estructura, relaciones y bloqueos, no si el texto refleja el negocio. Esa confirmación es de la persona.
- `specify` no sustituye a Spec Kit ni lo instala; sin la skill disponible en el agente, la preparación se conserva y la entrega queda en espera.
