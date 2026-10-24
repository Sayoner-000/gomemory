# Socratica: refinamiento de prompts y escrutinio socrático

Socratica convierte un prompt en bruto en uno fiel y ejecutable, o examina una propuesta antes de actuar. Hace como máximo tres preguntas con opciones, propone una reescritura y **no ejecuta nada sin tu aprobación explícita**. Si cancelas, recuperas tu original intacto.

## Modos e invocación

| Modo | Para qué | Cómo se invoca |
|---|---|---|
| `refine` | Aclarar y mejorar un prompt: audiencia, criterio de éxito, restricciones, formato, alcance. Lo que falta queda como `[marcador]`. | `/socratica <texto>` en el agente; `mem socratica refine` en la terminal; tecla `p` → `r` en la TUI |
| `scrutiny` | Examinar supuestos, evidencia, alternativas y criterios de una propuesta, sin defender por defecto la hipótesis inicial. | `/socratic-q <texto>`; `mem socratica refine --mode scrutiny` |

La invocación exacta depende del agente (habilidad nativa, comando con nombre o `$socratica` en Codex). La tabla vigente está en la [matriz de compatibilidad](../specs/039-socratica-prompt-refinement/contracts/compatibility-matrix.md). Si un agente no reconoce el comando, pídelo en lenguaje natural («aplica socratica a: …»).

La habilidad se instala en `~/.claude/skills/`, `~/.codex/skills/` y `~/.config/opencode/skills/` con `mem setup-mcp --scope global`, `mem install` (alcance global) y `mem update`, solo en los agentes que ya existen en tu equipo. También puedes copiarla a mano desde [`skills/socratica`](../skills/socratica/).

Junto a la habilidad se escriben envoltorios de comando finos, que solo nombran la habilidad y el modo: `~/.claude/commands/socratic-q.md` (Claude ya expone `/socratica` como habilidad) y `~/.config/opencode/commands/socratica.md` y `socratic-q.md`. Codex no tiene un directorio de comandos verificado, así que no recibe ninguno. Igual que la habilidad, se escriben solo si existe el directorio del agente y el contenido ha cambiado.

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

Cada agente tiene como máximo un flujo sin terminar por proyecto. Empezar otro retoma el pendiente; para empezar de cero, cancélalo (`mem socratica pending --cancel <id>`).

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
