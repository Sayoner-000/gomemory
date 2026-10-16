# Socratica: refinamiento de prompts y escrutinio socrático

Socratica convierte un prompt en bruto en uno fiel y ejecutable, o examina una propuesta antes de actuar. Hace como máximo tres preguntas con opciones, propone una reescritura y **no ejecuta nada sin tu aprobación explícita**. Si cancelas, recuperas tu original intacto.

## Modos e invocación

| Modo | Para qué | Cómo se invoca |
|---|---|---|
| `refine` | Aclarar y mejorar un prompt: audiencia, criterio de éxito, restricciones, formato, alcance. Lo que falta queda como `[marcador]`. | `/socratica <texto>` en el agente; `mem socratica refine` en la terminal; tecla `p` → `r` en la TUI |
| `scrutiny` | Examinar supuestos, evidencia, alternativas y criterios de una propuesta, sin defender por defecto la hipótesis inicial. | `/socratic-q <texto>`; `mem socratica refine --mode scrutiny` |

La invocación exacta depende del agente (habilidad nativa, comando con nombre o `$socratica` en Codex). La tabla vigente está en la [matriz de compatibilidad](../specs/039-socratica-prompt-refinement/contracts/compatibility-matrix.md). Si un agente no reconoce el comando, pídelo en lenguaje natural («aplica socratica a: …»).

La habilidad se instala en `~/.claude/skills/`, `~/.codex/skills/` y `~/.opencode/skills/` con `mem setup-mcp --scope global`, `mem install` (alcance global) y `mem update`, solo en los agentes que ya existen en tu equipo. También puedes copiarla a mano desde [`skills/socratica`](../skills/socratica/).

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

```sh
mem socratica refine "envía el informe al cliente"         # preguntas y revisión interactivas
mem socratica refine --client codex --mode scrutiny -      # texto por stdin
mem socratica refine --print "texto"                        # imprime >prompt aprobado en vez de lanzar el agente
mem socratica pending [--resume <id> | --cancel <id>]
mem socratica history [--client <agente>] [--show <id>] [--json]
mem socratica private on|off
```

En la revisión: `e` ejecutar, `d` editar, `c` copiar (OSC 52, o un bloque delimitado), `x` cancelar. Sin terminal interactiva nunca se ejecuta nada: se imprime la revisión y el flujo queda pendiente. Códigos de salida: 0 éxito, 1 error de uso o validación, 2 sin generador disponible (se repite tu original), 3 memoria no inicializada.

## Uso desde la TUI

Pulsa `p` en la lista principal. La pantalla muestra el seguimiento «Tu prompt · Preguntas · Reescritura · Revisión», el pendiente de cada agente y el historial. `r` inicia un refinamiento (verás «Generando preguntas con <agente>…» al instante), `enter` abre el detalle, `i` alterna la intercepción del agente seleccionado (respetando la evidencia) y `p` el modo privado. Ejecutar cede la terminal al agente y la TUI vuelve al terminar.

## Limitaciones

- Ningún agente ofrece paneles nativos para Socratica: la revisión es texto (o la TUI de `mem`).
- OpenCode no es generador fuera del agente: `opencode run` no tiene un modo sin herramientas verificado.
- Las capacidades por agente (comando con nombre, intercepción, portapapeles, devolver el original al campo de entrada) se habilitan solo con evidencia de aceptación en sesión real; consulta la matriz.
