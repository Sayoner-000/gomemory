# Modelo de datos: preparación para especificación

## Agregado y compatibilidad

`RefinementFlow` sigue siendo el agregado principal. Añade el modo `specify`, una preparación opcional y una política de payload fijada al crearlo. `refine/scrutiny` y los flujos antiguos conservan preparación nula y política persistente; no se les aplican las nuevas condiciones.

| Campo | Tipo conceptual | Regla |
| --- | --- | --- |
| `Mode` | `refine / scrutiny / specify` | No convertir un pendiente existente al iniciar otro modo. |
| `Revision` | entero no negativo | Fuente única de revisión del contenido de `specify`; aumenta en cada cambio material de sección. |
| `PayloadPolicy` | `persistent / ephemeral` | Inmutable por flujo. `ephemeral` se elige solo al crear `specify` privado. |
| `Preparation` | agregado opcional | Obligatorio para operar el modo nuevo; ausente en modos anteriores. |
| `Questions`, `Answers` | listas acumulativas | Máximo tres preguntas por flujo. Editar o retomar no limpia la lista. |
| `ProposedPrompt` | texto protegido | Render determinista de la preparación revisable, no texto libre del agente. |
| `ApprovedPrompt` | texto protegido | Copia exacta del render aprobado. No se regenera tras aprobar. |
| Campos de despacho existentes | identificadores, huellas y marcas | Mantienen su función; no reutilizar `DispatchConsumed` como permiso de ejecutar. |

`RefinementHistory` añade una instantánea opcional de la preparación y revisión. Se conserva la política existente de historial por flujo, no un registro ilimitado de todas las revisiones. No se guarda historial de `specify` privado.

## Preparación

| Campo | Contenido |
| --- | --- |
| `schema_version` | Versión 1 del agregado de preparación; independiente de la versión del catálogo SQLite. |
| `stage` | `intent`, `scope`, `stories`, `validation`, `review`, `approved` o `delivery`. |
| `content` | Secciones `intent`, `scope` y `stories`; pueden faltar mientras se completa el borrador. |
| `checks` | Comprobaciones calculadas por gomemory; etapa, huella de sus entradas, resultado y faltantes. |
| `review` | Revisión del contenido y huella exacta del texto revisado; nula hasta completar la comprobación final. |
| `outcome` | Resultado comunicado por el agente para un despacho consumido; no constituye permiso de ejecución. |

El [esquema de contenido completo](contracts/preparation.schema.json) describe la forma del contenido cuando se comprueba para revisión. Los borradores parciales se validan por sección al actualizarse. El esquema no sustituye las reglas de referencias, etapas o bloqueos de dominio.

### Sección `intent`

- `objective`: necesidad que se quiere resolver.
- `expected_result`: resultado observable esperado.
- `actors`: lista no vacía de `{id, name}`; identificadores únicos. Puede incluir actores técnicos.

### Sección `scope`

- `included`: lista no vacía de necesidades dentro del alcance.
- `excluded`, `constraints`, `assumptions`: `{declared, items}`. Para completar la sección se exige `declared=true`; `items=[]` declara expresamente que no se identifican elementos. La ausencia de declaración sigue siendo un faltante.
- `pending`: `{declared, items}`; cada pendiente tiene `id`, `description`, `blocking`, `resolution` y, al resolverlo, `resolution_note`.
- Un pendiente `blocking=true` y `resolution=open` impide la revisión. Un supuesto o una respuesta omitida no lo resuelve. Resolverlo exige una nota que identifique la decisión o evidencia; su validez de negocio se confirma en revisión. Cambiar un bloqueo a no bloqueante exige explicar el motivo en esa nota. No se elimina silenciosamente un bloqueo abierto de una versión anterior.

### Sección `stories`

- `stories`: lista no vacía de `{id, actor_id, need, outcome}`.
- `criteria`: lista no vacía de `{id, story_ids, condition, expected_result}`.
- `deliveries`: lista no vacía de `{id, title, outcome, story_ids, criterion_ids}`.
- Los identificadores son únicos dentro de su tipo. Las referencias deben existir. Cada historia tiene al menos un criterio y participa en una entrega; cada criterio pertenece a una o más historias. Cada entrega incluye criterios que comprueban sus historias, sin referencias ajenas al alcance que declara cubrir.
- Las listas conservan el orden propuesto por la persona/agente; el renderizador usa ese orden y un orden fijo de secciones. La identidad de cada elemento permite rastrearlo aunque la especificación resultante use otra numeración.

## Comprobaciones y revisión

| Etapa | Entradas comprobadas | Condición de avance |
| --- | --- | --- |
| `intent` | Objetivo, resultado y actores | Contenido no vacío, actores identificables. |
| `scope` | Intención y alcance | Inclusión descrita; declaraciones explícitas; pendientes clasificados. Los bloqueos pueden seguir visibles mientras se prepara. |
| `stories` | Intención, alcance, historias, criterios y entregas | Campos y referencias válidos; cobertura de historias por criterios y entregas. |
| `validation` | Todo el contenido vigente | Comprobaciones previas vigentes, sin bloqueos abiertos ni marcadores esenciales pendientes. |
| `review` | Render protegido de la revisión vigente | Espera elección humana; completar datos no aprueba el contenido. |

Cada comprobación guarda la huella de la proyección que utiliza. Cambiar `intent` invalida todas; cambiar `scope` invalida alcance, historias y comprobación final; cambiar `stories` invalida historias y comprobación final. Cualquier cambio material elimina `review`, `ProposedPrompt`, `ApprovedPrompt` y la autorización aún no consumida. Un cambio idéntico no incrementa la revisión.

La revisión guarda `{revision, prompt_hash}`. `prompt_hash` es SHA-256 de los bytes UTF-8 exactos de `ProposedPrompt`. La huella normalizada existente para procedencia del hook conserva su función separada. Tanto `approve` como `ack` comprueban que el contenido, las comprobaciones, la revisión y el texto aprobado siguen coincidiendo. No se acepta una huella aportada por el agente como reemplazo de la comprobación propia.

## Transiciones

| Operación | Fase general | Efecto en `specify` |
| --- | --- | --- |
| `start` | → `captured` | Crea etapa `intent` y revisión 0, o devuelve el pendiente existente sin convertirlo. |
| `prepare(set)` | `captured/questions/review/approved` sin reclamo | Reemplaza una sección con revisión esperada; invalida desde esa sección. Rechaza si hay entrega reclamada o consumida. |
| Registrar preguntas | `captured` → `questions` | Añade preguntas sin superar tres acumuladas; conserva la etapa de preparación. |
| `answer/skip_rest` | `questions` → `captured` al terminar | Registra respuestas/omisiones; vuelve a la preparación sin generar una propuesta libre. |
| `prepare(validate)` | `captured` | Comprueba la etapa permitida. Un fallo devuelve faltantes sin habilitar etapas posteriores. La comprobación final correcta genera render y pasa a `review`. |
| `approve` | `review` → `approved` | Exige revisión vigente y elección explícita; vincula el despacho al mismo texto. Duplicados no conceden ejecución. |
| `ack` | `approved` → `dispatched` | Exige identidad vigente. Solo la primera transición confirmada devuelve `execution_granted=true`. |
| `report` | `dispatched` → `dispatched` | Registra resultado del despacho vigente y actualiza su instantánea de historial cuando corresponda. |
| `resume/get` | Conserva la etapa válida | En `specify`, una aprobación sin reclamo puede conservarse si su revisión sigue vigente; no se ejecuta nada. Los modos previos mantienen sus reglas. |
| `cancel` | No terminal → `cancelled` | Devuelve el original disponible y retira la preparación activa; en privado elimina payload y reserva según la política vigente. |
| `force_resend` | Reconciliación existente | Solo por elección explícita. `mark_dispatched` no aplica: `specify` nunca reclama el despacho en el ejecutor local. Un reenvío usa nueva autorización y conserva el contenido aprobado; no reconstruye un payload privado perdido. |

Para `specify`, se rechazan `rewrite`, la edición textual libre y `send_original`. Los errores de generación no habilitan el envío del original hacia Spec Kit sin preparación. Como este modo no genera con el modelo, no alcanza la fase `failed` y `retry` se rechaza; volver a preparar se hace con `prepare(set)`, que respeta las preguntas acumuladas. La ejecución y la asistencia locales genéricas no deben despachar este modo.

## Resultado comunicado

`outcome` contiene `dispatch_id`, `status` (`completed`, `failed` o `uncertain`), rutas relativas de especificación/checklist cuando corresponda, trazabilidad comunicada de criterios y un mensaje depurado. Un reporte completado exige ambas rutas y una relación para cada criterio preparado. Es información aportada por el agente después de inspeccionar el resultado; el backend comprueba formato y coherencia de referencias, no fidelidad semántica del documento generado.

Reportar no consume otra autorización ni inicia un nuevo proceso. Un duplicado idéntico es un no-op. Un reporte no puede afirmar resultado para un `dispatch_id` obsoleto. Una corrección de un resultado previamente comunicado debe ser explícita y nunca rehabilita la autorización consumida.

## Persistencia y migración

1. Añadir `preparation_json` nullable a flujos e historial y `payload_policy` no nulo con valor predeterminado `persistent` a flujos. Versionar el catálogo aditivamente de 2 a 3, si 2 sigue siendo la versión anterior al implementar.
2. Actualizar proyecciones explícitas, argumentos de escritura, lectores y pruebas golden; no usar `SELECT *`. Escribir preparación y transición en el mismo UnitOfWork, manteniendo el control de concurrencia existente y la revisión esperada.
3. Una fila antigua con preparación nula se sigue leyendo. Nunca inferir retrospectivamente que estaba preparada. Una versión nueva no soportada se conserva y se rechaza al intentar mutarla, sin sobrescribirla.
4. La retención y el borrado de las filas eliminan su agregado completo. La sanitización recorre todas las hojas textuales y las instantáneas de revisión; cualquier cambio en un flujo no consumido invalida su preparación/aprobación.

### Payload efímero

Para el modo privado nuevo, el adaptador conserva una reserva SQL sin texto y el contenido solo en memoria. Original, preguntas, respuestas, preparación, propuesta, aprobación, errores textuales y rutas comunicadas no se escriben en SQL, archivos temporales, locks o historial. Los metadatos necesarios para aislamiento y despacho pueden persistir; no se guarda contenido protegido para calcularlos.

El mismo adaptador coordina la transacción de metadatos con el depósito efímero mediante mutex y cambios preparados que se publican después del commit. Las lecturas de esa instancia ven el cambio completo o el estado anterior. Una caída después del commit puede perder el contenido efímero y debe producir `private_state_unavailable`, nunca recuperar una autorización ejecutable.

Las instancias sin el payload solo ven la reserva. No admiten `prepare`, `approve`, `ack`, `report` ni reenvío; permiten consulta y cancelación explícita. El índice único existente sigue impidiendo otro pendiente para el mismo proyecto/agente. La política del flujo se mantiene aunque cambie la preferencia privada del proyecto.
