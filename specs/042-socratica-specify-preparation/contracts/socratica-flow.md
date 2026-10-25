# Contrato: `socratica_flow` con preparación para Spec Kit

Amplía el [contrato existente](../../039-socratica-prompt-refinement/contracts/mcp-tool.md). Se mantiene una sola herramienta. Los campos nuevos solo cambian el comportamiento de `mode=specify`; el contrato público de `refine/scrutiny` se conserva.

## Inicio y consulta

```json
{"action":"start","client":"codex","mode":"specify","prompt":"Quiero permitir que una persona exporte su historial."}
```

`client` admite `claude`, `codex` y `opencode`. El proyecto lo fija el servidor, no la entrada. Si hay un pendiente para ese proyecto/cliente, se devuelve con su modo real y `recovered=true`; no se reemplaza ni convierte. `get`, `resume`, `history` y `settings` conservan su finalidad.

La respuesta del nuevo modo añade:

| Campo | Significado |
| --- | --- |
| `mode` | `specify`. |
| `revision` | Revisión vigente del contenido. |
| `stage` | Etapa de preparación vigente. |
| `preparation` | Contenido y metadatos descritos en el modelo de datos, sujetos a privacidad. |
| `pending_checks` | Lista de `{code, path, message}` con faltantes y bloqueos. |
| `allowed_actions` | Acciones actualmente permitidas, calculadas por gomemory. |
| `questions_used` | Número acumulado de preguntas registradas, entre 0 y 3. |
| `review_hash` | Huella exacta del texto revisable, o nula si falta una revisión vigente. |
| `execution_granted` | Solo verdadero en la respuesta a la primera confirmación comprometida. Por defecto falso; nunca se recupera como permiso desde `get`. |
| `outcome` | Resultado comunicado, si existe. |
| `private_state_unavailable` | Existe reserva privada, pero esta instancia no dispone del payload. |
| `next` | Instrucción permitida: preparar, completar, revisar, comprobar disponibilidad, confirmar, entregar, consultar o reconciliar. |

Permanecen `flow_id`, `phase`, `original`, `proposed`, `approved_prompt`, `questions`, `answers`, las marcas de recuperación y los campos de despacho existentes. Las respuestas se protegen antes de mostrarse. No se devuelven contenidos privados de otra instancia.

## Actualizar una sección

```json
{
  "action":"prepare",
  "flow_id":"flow-id",
  "expected_revision":0,
  "preparation":{
    "operation":"set",
    "section":"intent",
    "value":{
      "objective":"Permitir consultar el historial fuera de la aplicación",
      "expected_result":"La persona obtiene un archivo con su historial autorizado",
      "actors":[{"id":"A1","name":"Persona titular del historial"}]
    }
  }
}
```

- `expected_revision` es obligatorio y debe coincidir con el contenido vigente. Un conflicto devuelve el estado actual y exige volver a leer; no aplica cambios parciales.
- `section` admite `intent`, `scope` y `stories`; `value` reemplaza la sección completa y tiene la forma de su definición en [preparation.schema.json](preparation.schema.json).
- Una sección puede estar incompleta como borrador: se conserva y devuelve los faltantes que impiden comprobarla. Los tipos, campos desconocidos y formatos estructuralmente inválidos se rechazan antes de guardar.
- Un cambio material incrementa la revisión, invalida las comprobaciones dependientes y cualquier autorización no consumida. Un reemplazo idéntico no cambia la revisión.
- En revisión o aprobado sin reclamo se permite volver a editar por esta operación; no se admite cuando hay reclamo o autorización consumida. La edición no puede adelantarse a una entrega ya concedida.
- No se aceptan comprobaciones, etapa, revisión aprobada, huellas o permisos calculados por el agente dentro de `value`.

## Comprobar una etapa

```json
{
  "action":"prepare",
  "flow_id":"flow-id",
  "expected_revision":3,
  "preparation":{"operation":"validate","stage":"intent"}
}
```

`stage` admite `intent`, `scope`, `stories` y `validation`. Se comprueba en ese orden; no se permite saltar etapas ni confiar en una comprobación con entradas obsoletas. La comprobación fallida devuelve faltantes sin habilitar el avance. Completar `validation` produce el texto final protegido, guarda su revisión y huella y pasa a `phase=review`; no aprueba ni entrega.

El contenido completo debe cumplir el esquema y, además, las reglas de referencias, bloqueos y cobertura del [modelo de datos](../data-model.md). No se usa reconocimiento automático de lenguaje natural como prueba de corrección de negocio.

## Preguntas, omisiones y recuperación

En `specify`, `answer` con `questions` registra preguntas nuevas únicamente durante la preparación sin preguntas pendientes de responder. Se añaden a la lista acumulativa y no pueden elevarla por encima de tres. Las preguntas mantienen de dos a cuatro opciones, además de las opciones de respuesta libre y omisión mostradas por el agente. Se permiten cero preguntas.

`answer` con `answers` responde índices pendientes de esa lista; `skip_rest=true` registra las omisiones. Al terminar se vuelve a la preparación en la misma etapa, sin activar un generador ni una reescritura libre. Las respuestas no resuelven por sí solas un pendiente estructurado: el agente debe proponer la actualización correspondiente y conservar como pendiente lo que no se respondió.

`get` es solo lectura. `resume` conserva las decisiones y la preparación válida. Un `specify` aprobado sin reclamo mantiene su aprobación si el contenido no cambió; si hay reclamo se aplican las reglas existentes de envío en curso o incierto. Este modo no genera con el modelo y no tiene fase `failed`, así que `retry` no aplica y se rechaza; para volver a preparar se usa `prepare` (`set`), que nunca limpia el presupuesto de preguntas. Se rechazan `retry`, `rewrite`, `edit` textual y `send_original` en este modo.

## Revisión y aprobación

La persona ve el original y `proposed` completo, incluidos supuestos y pendientes no bloqueantes, y elige «Aprobar y generar especificación», «Editar», «Copiar» o «Cancelar». Una afirmación del agente sobre completitud no constituye aprobación.

```json
{"action":"approve","flow_id":"flow-id","expected_revision":3}
```

`approve` exige que las comprobaciones y el render coincidan con la revisión vigente. Devuelve el `dispatch_id` y el texto exacto aprobado. Una aprobación repetida conserva el despacho existente; nunca devuelve `execution_granted=true`. `cancel` conserva/devuelve el original disponible y no entrega el borrador.

Antes de `ack`, la skill comprueba en el entorno activo que puede invocar `speckit-specify` en ese proyecto. La mera presencia de `specify` en PATH no basta. Si falta esa capacidad, no llama a `ack`: muestra la dependencia y conserva la preparación y aprobación. El backend controla la preparación y autorización; no afirma haber verificado una capacidad interna del anfitrión a partir de una declaración del modelo.

## Confirmación y única concesión

```json
{"action":"ack","flow_id":"flow-id","dispatch_id":"dispatch-id","ack_nonce":"nonce-conservado-para-este-intento"}
```

| Caso | `acked` | `already_dispatched` | `execution_granted` | Acción del agente |
| --- | --- | --- | --- | --- |
| Primera confirmación y commit correctos | true | false | true | Invocar una vez `speckit-specify` con `approved_prompt`. |
| Mismo nonce repetido | true | false | false | No ejecutar desde este replay. Consultar el resultado o reconciliar explícitamente si la primera respuesta se perdió. |
| Otro nonce tras el consumo | false | true | false | No ejecutar. |
| Identificador/propietario incorrecto, versión inválida o error | false o error | Según estado | false | No ejecutar; corregir o reconciliar. |

La concesión se obtiene del resultado de la primera transición atómica, no del indicador idempotente `acked`. Un rollback no concede ejecución. Una respuesta de `get`, `resume`, `approve` o `report` nunca concede ejecución.

Si la respuesta se pierde se repite el mismo nonce. Si la primera transición se confirmó, el replay informa consumo y no repite la concesión; se requiere una decisión explícita si no hay evidencia de entrega. `force_resend`, con el despacho vigente, conserva su finalidad. `mark_dispatched` no aplica: `specify` no pasa por el ejecutor local que reclama el despacho, y tras el primer `ack` el flujo ya está enviado. La elección de reenviar crea una autorización nueva, no revive la anterior. Un payload privado perdido no se puede reenviar.

## Comunicar el resultado

```json
{
  "action":"report",
  "flow_id":"flow-id",
  "dispatch_id":"dispatch-id",
  "outcome":{
    "status":"completed",
    "spec_path":"specs/NNN-feature/spec.md",
    "checklist_path":"specs/NNN-feature/checklists/requirements.md",
    "traceability":[{"criterion_id":"C1","story_ids":["US1"],"spec_reference":"Acceptance Scenarios"}]
  }
}
```

`report` solo admite un despacho consumido y vigente; no concede autorización ni llama al generador. Un resultado `completed` exige ambas rutas relativas y trazabilidad comunicada para todos los criterios preparados. Rechazar rutas absolutas, segmentos `..`, controles y referencias inexistentes. El backend no abre rutas suministradas como comandos.

`failed` y `uncertain` incluyen un mensaje depurado y no exigen artefactos inexistentes. Repetir el mismo reporte es un no-op. Corregir uno anterior exige `correct_previous=true`; no modifica el consumo. La interfaz lo rotula como resultado comunicado por el agente. La evidencia de aceptación debe inspeccionar los archivos reales.

En privado, resultado y rutas permanecen efímeros. Sin payload, solo se permiten consulta y cancelación explícita de la reserva; las demás operaciones devuelven `private_state_unavailable` sin reconstruir contenido ni permisos.

## Errores y pruebas de contrato

Categorías del nuevo modo: `revision_conflict`, `stage_blocked`, `invalid_reference`, `blocking_pending`, `review_stale`, `unsupported_specify_action`, `private_state_unavailable` y rechazo de despacho existente. Los mensajes identifican el campo o siguiente paso, sin contenido sensible ni detalles de SQL.

Comprobar esquema MCP publicado, rechazo de campos calculados, alcance por proyecto, revisión obsoleta, saltos de etapa, preguntas acumuladas, identidad del texto, bypass por acciones antiguas, falta de Spec Kit sin `ack`, concurrencia/replay y reportes sin artefactos comunicados. Se mantienen los contratos previos de los otros dos modos.
