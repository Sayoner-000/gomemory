# Contrato CLI: `mem socratica`

Actualización de implementación (2026-10-17): cada flujo nuevo pide elegir un
harness, sin guardar preferencia. `--client` elige el mismo harness para generar
y ejecutar y omite el selector; sin TTY es obligatorio (su ausencia devuelve 2
sin crear un flujo). No hay reemplazo automático si el elegido falla. Los
pendientes conservan su cliente. Los selectores y paneles reutilizan `console`;
Escape sale sin ejecutar y conserva el pendiente si ya existe. `--print`
reserva stdout para el prompt aprobado y envía la presentación a stderr.

Recuperación asistida: un fallo permite abrir una conversación de refinamiento
en el mismo harness (`a`) o cambiarlo (`h`) conservando el original. La apertura
asistida no llama a `Execute`, no aprueba el original ni consume su autorización:
envía una instrucción para preguntar, reescribir y esperar aprobación. Al volver
del harness, la consola sale sin regenerar. `--print` excluye esta apertura.
Los rechazos conocidos de proveedor/autenticación/esquema no ofrecen reintento;
sus diagnósticos se traducen en mensajes breves, incluso en pendientes antiguos.

Registro en `adapters/primary/cli/dispatcher.go` como subcomando de proyecto (igual que `review` y `octopus`). Toda salida de texto pasa por `domain.ProtectText`. Los errores se muestran en lenguaje llano por stderr. Códigos de salida: `0` éxito, `1` error de uso o validación, `2` sin generador disponible, `3` almacenamiento no disponible o proyecto no inicializado.

| Uso | Comportamiento | Spec |
|---|---|---|
| `mem socratica refine [--mode refine\|scrutiny] [--client <agente>] [--print] [texto\|-]` | Sin texto: lo pide en la terminal. Selecciona un harness por flujo mediante el selector o `--client`; lo usa para preguntas, reescritura y ejecución aprobada. Antes de generar muestra «Generando preguntas con <agente>…» (FR-017). Si el elegido no está disponible, devuelve 2 con el original. Los selectores permiten responder, escribir Otra, omitir una pregunta o el resto, ejecutar, editar, copiar y cancelar. Sin TTY exige `--client`, omite preguntas y deja la revisión pendiente sin ejecutar. | US4-6/7, FR-032a |
| `mem socratica history [--client <agente>] [--show <id>] [--json]` | Lista hasta `socratica_history_limit` registros (por defecto 20), del más reciente al más antiguo. `--show` muestra la revisión en texto. | US4-1 |
| `mem socratica on --client <agente> [--unverified]` | Si `SocraticaCapabilities[agente].Interception` es falso, informa «no soportado en <agente>» y sale con código 1 sin persistir nada. Con `--unverified` (solo para obtener evidencia de aceptación) persiste la activación de ese agente, avisa «intercepción sin verificar en <agente>» y `status` la muestra como «sin verificar». Si la capacidad es verdadera, persiste solo la clave de ese agente. | US4-2/3, FR-018/019 |
| `mem socratica off --client <agente>` | Pone a `false` solo la clave de ese agente; es idempotente. | FR-018 |
| `mem socratica status` | Muestra, por agente, la intercepción (activa / sin verificar / desactivada), las capacidades, el flujo pendiente con su antigüedad, el modo privado y la disponibilidad de cada harness. | US4-4 |
| `mem socratica pending [--resume <id>\|--cancel <id>]` | Lista el flujo no terminal de cada agente (uno como máximo) con su fase estable. `--resume` lo reanuda según la tabla `resume` de `data-model.md` (regenera solo si estaba en captured o rewriting; approved sin ack → revisión recuperable). `--cancel` pasa el flujo a CANCELLED e imprime el original literal. | US4-4, FR-014, FR-015 |
| `mem socratica private on\|off` | Activa o desactiva `socratica_private`. | FR-027 |

Ejecutar desde la CLI (R4) llama a `SocraticaFlow.Execute` con `ExecLauncher` (o con `PrintLauncher` si se usa `--print`); la CLI no orquesta los pasos. El caso de uso hace tres pasos sin transacción que abarque el proceso: (1) `approve` en su propia transacción; (2) arranque del agente con el prompt aprobado y `GOMEMORY_SOCRATICA_DISPATCH=<dispatch_id>` en su entorno, para que la intercepción no lo recapture; (3) `ack` solo si el arranque tuvo éxito. Todo `Execute` retiene el lock de archivo del flujo, y entre (1) y (2) `ClaimDispatch` reserva el despacho. Una segunda ejecución, o un `pending --resume` que intente reenviar, marcar o cancelar mientras el lock está retenido, informa «envío en curso» y no cambia nada. El lanzador es bloqueante: el `ack` se escribe en `onStarted`, nada más arrancar el agente, y la CLI espera a que el agente termine. Si el agente arranca pero el `ack` falla, se reintenta una vez cuando el agente termina; si vuelve a fallar, sale con código 1 con «el agente arrancó pero no se pudo confirmar el envío; no reintentes» y el flujo queda incierto: `pending --resume` ofrece `[m]arcar enviado [r]eenviar [x]cancelar`, y reenviar exige elegirlo explícitamente. Un reclamo sin ack cuyo proceso ya no retiene el lock (terminó o murió) también se trata como incierto. Si el arranque falla, se libera el reclamo (solo el propio), informa el error, sale con código 1 y el flujo queda aprobado sin confirmar (recuperable con `pending --resume`). Con `--print`: `approve`, escritura por stdout de `>` seguido del prompt (para que al pegarlo en un agente con intercepción activa no se recapture; `--raw` omite el prefijo) y `ack`. Una segunda ejecución sobre un flujo ya confirmado informa «ya enviado (<dispatch_id>)» y no relanza nada.

Fallo del generador (FR-016): ante un timeout o un JSON irreparable se conserva el original byte a byte y se ofrece `[r]eintentar [o]riginal [x]cancelar`. «Enviar mi original» exige elegirlo explícitamente y pasa por `Execute`.

Copiar: usa OSC 52 si la terminal lo soporta. Si no, imprime el prompt entre delimitadores para copiarlo a mano. Nunca invoca utilidades del sistema sin detectarlas antes.

Ajustes inválidos (constitución §9): si algún `socratica_*` presente está fuera de rango, todas las subórdenes salen con código 1 y el mensaje de `ErrInvalidSocraticaSettings` (clave y valor) antes de tocar la base o el generador. Las claves ausentes usan su valor por defecto.

Autorización consumida (I10): un flujo `dispatched` significa que la autorización se consumió y se entregó a un único agente, no que la tarea se ejecutara. `mem socratica history --show <id>` lo muestra como «ya enviado» y ofrece `[r]eenviar` mientras el flujo exista (`TerminalRetention`, 24 h), solo en una TTY y con confirmación explícita (`force_resend`, autorización nueva). Nunca se reenvía de forma automática.
