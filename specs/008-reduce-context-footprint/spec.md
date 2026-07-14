# Feature Specification: Reducir la huella de contexto de gomemory en la sesión

**Feature Branch**: `008-reduce-context-footprint`

**Created**: 2026-07-13

**Status**: Draft

**Input**: User description: "Cómo evitar que las sesiones con IA se encarezcan. El `/usage` de Claude reporta: 65% del gasto ocurrió con >150k de contexto, y 22% vino del servidor MCP «gomemory» porque los resultados de sus tools quedan en el contexto el resto de la sesión. ¿Cómo compactar el resumen que gomemory inyecta, al finalizar el turno?"

## Contexto del problema *(no normativo — fundamenta el porqué)*

El usuario paga por tokens de contexto. Cada respuesta de una tool MCP queda
adherida a la ventana del cliente (Claude Code) hasta que el usuario ejecute
`/compact` o `/clear`. Hoy `get_context` devuelve el resumen completo del
proyecto (hasta 100 memorias con su **contenido íntegro** para varios tipos),
lo que produce un bloque de decenas de KB que se paga una y otra vez en cada
turno posterior. El resultado observable: gomemory concentra ~22% del gasto de
la sesión y empuja la conversación por encima de 150k de contexto, donde cada
turno cuesta más aunque haya caché.

Un servidor MCP **no puede desalojar** de la ventana un resultado que ya emitió;
esa palanca es exclusiva del cliente (`/compact`, `/clear`). Por eso la única
palanca que gomemory controla es **emitir menos** desde el principio y ofrecer
formas compactas de refrescar el contexto por turno.

**Agnóstico al agente (principio rector).** gomemory es un servidor MCP
consumible por cualquier agente (Claude Code, Cursor, otros clientes MCP o el
propio CLI `mem`). La reducción de huella DEBE ser universal: como opera sobre
el **texto emitido**, beneficia a todo consumidor sin depender de comandos ni de
capacidades propias de un cliente concreto. Cualquier señal de compactación se
expresa como texto/recordatorio neutral —«conviene compactar el contexto»— y no
como un comando específico de un agente; el cliente decide cómo actuar.

## Validación contra el estado del arte (engram) *(no normativo)*

El proyecto **engram** (Gentleman-Programming) resuelve el mismo problema con
patrones que confirman esta dirección y aportan optimizaciones a adoptar:

- **Progressive disclosure (drill-down de 3 capas)**: resultados de búsqueda
  compactos (~100 tokens c/u, con id) → contexto de timeline (qué pasó antes/
  después) → contenido íntegro **solo bajo demanda**. Es el patrón exacto de las
  historias P1/P2 y fija una referencia concreta de tamaño por ítem.
- **Deduplicación y upsert por tópico**: hash (proyecto+scope+tipo+título) para
  evitar inserciones repetidas; consolidación de duplicados (`duplicate_count`,
  `last_seen_at`) y upsert por `topic_key` que **actualiza la memoria existente**
  (`revision_count`) en vez de crear filas nuevas. Reduce el contexto **en la
  fuente**: menos memorias redundantes ⇒ menos que inyectar.
- **Soft-delete + revisión**: las memorias borradas o «a revisar» se excluyen de
  búsqueda/contexto/timeline, bajando el ruido.
- **Resumen de sesión estructurado** (Objetivo / Hallazgos / Logrado / Próximos
  pasos / Archivos) que se auto-inyecta en la siguiente sesión, compactando
  trabajo multi-turno en un solo bloque.

**Diferencia deliberada**: engram NO impone presupuesto de tokens ni decaimiento;
se apoya en relevancia FTS + juicio del agente. gomemory añade un **presupuesto
explícito** (FR-001/FR-002) como techo duro y agnóstico al agente, encima de la
progressive disclosure — una garantía de tamaño que la sola relevancia no da.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Contexto de arranque acotado por presupuesto (Priority: P1)

Como desarrollador que abre una sesión de trabajo, quiero que el contexto que
gomemory inyecta al inicio (`get_context` / `mem context` / el hook de
`SessionStart`) esté acotado a un presupuesto de tamaño razonable, para que el
arranque no cargue decenas de KB que pagaré durante toda la sesión.

**Why this priority**: Es la fuente única mayor del gasto (un solo bloque de
~82KB que persiste todo el resto de la sesión) y la que el usuario nombra
primero. Reducirla ataca directamente el 22% reportado y el umbral de 150k.

**Independent Test**: Con un proyecto que tiene ≥100 memorias, invocar
`get_context` y verificar que la salida no supera el presupuesto configurado, y
que sigue conteniendo lo esencial (protocolo activo, conflictos, sinapsis clave,
preferencias, decisiones/patrones/bugfixes recientes y punteros para profundizar
bajo demanda).

**Acceptance Scenarios**:

1. **Given** un proyecto con 100+ memorias de contenido largo, **When** el
   agente llama a `get_context`, **Then** la salida se mantiene dentro del
   presupuesto de tamaño y cada entrada larga aparece resumida/truncada con un
   puntero (`get_memory <id>`) para recuperar el detalle completo bajo demanda.
2. **Given** que existen conflictos sin resolver y sinapsis, **When** se genera
   el contexto acotado, **Then** los conflictos se conservan íntegros (son
   accionables) y las sinapsis se limitan a las más relevantes.
3. **Given** un proyecto pequeño cuyo contexto ya cabe en el presupuesto,
   **When** se llama a `get_context`, **Then** la salida es idéntica en
   contenido a la actual (sin truncados innecesarios).

---

### User Story 2 - Progressive disclosure: resultados de tools livianos por defecto (Priority: P2)

Como desarrollador en una sesión larga, quiero que las tools de consulta de
gomemory (`search_memories`, `list_memories`, `get_memory`) sigan el patrón de
**revelación progresiva** —resultados compactos por defecto, detalle solo bajo
demanda— para que cada llamada no vaya sumando bloques pesados que quedan pegados
a la ventana el resto de la sesión.

**Why this priority**: Complementa a P1: aunque el arranque sea liviano, decenas
de consultas durante la sesión reinflan el contexto. Es la segunda fuente de
acumulación de la huella MCP. El patrón está validado por engram (drill-down de
3 capas, ~100 tokens por resultado).

**Independent Test**: Ejecutar `search_memories` y `list_memories` sobre un
proyecto con memorias largas y verificar que cada ítem se muestra resumido
(título + extracto + id), no con el contenido completo, salvo que se pida el
detalle explícito con `get_memory`.

**Acceptance Scenarios**:

1. **Given** una búsqueda que empareja 10 memorias largas, **When** el agente
   llama a `search_memories`, **Then** cada resultado se presenta como
   título + extracto acotado (referencia: ~100 tokens) + id, y el total respeta
   un límite de tamaño.
2. **Given** que el agente necesita el contenido íntegro de una memoria puntual,
   **When** llama a `get_memory <id>`, **Then** recibe el contenido completo sin
   truncar (la capa de detalle bajo demanda sigue disponible).
3. **Given** un id de memoria devuelto por una búsqueda compacta, **When** el
   agente quiere entender su contexto, **Then** puede obtener qué se guardó
   antes/después en la misma sesión sin volcar todos los contenidos íntegros
   (capa intermedia de timeline).

---

### User Story 3 - Compactación al cierre del turno / de la sesión (Priority: P3)

Como desarrollador, quiero que al finalizar un turno o una sesión gomemory
consolide lo aprendido en un resumen breve y estructurado, y me señale de forma
neutral cuándo conviene compactar el contexto, para que la huella acumulada de
resultados previos no siga pagándose sin necesidad —sea cual sea el agente que
esté usando.

**Why this priority**: Cierra el ciclo. Depende de que P1/P2 ya reduzcan la
huella; aquí se trata de consolidar (patrón de resumen estructurado validado por
engram) y de señalar el momento de compactar (acción que solo el cliente puede
ejecutar, cualquiera que sea).

**Independent Test**: Al cerrar sesión (`end_session`) verificar que se persiste
un resumen breve y estructurado consultable después, y que el hook de fin de
turno emite una señal/recordatorio neutral de compactación cuando la huella
estimada supera un umbral, sin bloquear ni alterar el trabajo en curso.

**Acceptance Scenarios**:

1. **Given** una sesión con varias decisiones y un bugfix, **When** se llama a
   `end_session`, **Then** queda persistido un resumen breve y estructurado
   (Objetivo / Hallazgos / Logrado / Próximos pasos / Archivos) que en la próxima
   sesión aparece en «Sesiones Recientes» sin volcar el detalle íntegro.
2. **Given** que la huella estimada de la sesión supera el umbral configurado,
   **When** finaliza el turno, **Then** gomemory emite un recordatorio neutral y
   no bloqueante («conviene compactar el contexto»), sin nombrar un comando
   específico de un agente ni ejecutar acciones destructivas por su cuenta.

---

### User Story 4 - Reducir la redundancia en la fuente (dedup + upsert) (Priority: P4)

Como desarrollador que guarda memorias a lo largo de muchas sesiones, quiero que
gomemory evite acumular memorias casi idénticas, para que el contexto no se
infle con repeticiones del mismo aprendizaje y cada inyección sea más densa en
información útil.

**Why this priority**: Ataca la causa raíz del tamaño: menos memorias
redundantes ⇒ menos que resumir e inyectar. Es la palanca de más largo plazo y
la más independiente (patrón validado por engram: dedup por hash + upsert por
tópico). Se prioriza tras P1–P3 porque su efecto es acumulativo, no inmediato.

**Independent Test**: Guardar dos veces una memoria equivalente (mismo
proyecto/tipo/título o mismo tópico) y verificar que no se crean dos filas
distintas, sino que la existente se actualiza/consolida.

**Acceptance Scenarios**:

1. **Given** una memoria ya guardada, **When** se intenta guardar otra
   equivalente (mismo proyecto+tipo+título dentro de una ventana), **Then** no se
   crea una fila nueva: se consolida sobre la existente (marca de re-visto), y el
   contexto no muestra el duplicado.
2. **Given** una memoria asociada a un tópico, **When** se guarda una
   actualización de ese mismo tópico, **Then** se actualiza la memoria existente
   (con marca de revisión) en lugar de crear una nueva.

---

### Edge Cases

- **Memoria más grande que el presupuesto por sí sola**: se muestra truncada con
  puntero a `get_memory <id>`; nunca se omite silenciosamente una entrada
  accionable (p. ej. un conflicto).
- **Presupuesto configurado en 0 o negativo**: se trata como «sin límite»
  (comportamiento actual), documentado, para no romper flujos existentes.
- **Proyecto sin memorias**: el contexto acotado sigue devolviendo el protocolo
  activo y una nota de que no hay historial (no una salida vacía).
- **Conflictos numerosos**: los conflictos no se recortan por presupuesto (son
  accionables); si abundan, se prioriza mostrarlos sobre secciones informativas.
- **El cliente no soporta señales de compactación**: el recordatorio de P3 es
  informativo; su ausencia no rompe nada.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE producir el contexto de arranque
  (`get_context` / recurso `mem://context` / salida del hook de `SessionStart`)
  acotado a un presupuesto de tamaño configurable, midiendo el tamaño del texto
  emitido.
- **FR-002**: El sistema DEBE truncar el contenido largo de cada entrada del
  contexto conservando lo suficiente para orientar, y adjuntar un puntero
  (`get_memory <id>`) para recuperar el detalle íntegro bajo demanda.
- **FR-003**: El sistema DEBE preservar íntegras las secciones accionables
  (protocolo de memoria activo y conflictos sin resolver) aun cuando otras
  secciones se recorten por presupuesto.
- **FR-004**: El sistema DEBE tratar un presupuesto ≤ 0 como «sin límite»
  (comportamiento actual), de modo que la funcionalidad sea opt-in y no rompa
  flujos existentes.
- **FR-005**: Las tools de consulta (`search_memories`, `list_memories`) DEBEN
  devolver, por defecto, cada ítem en forma compacta (título + extracto acotado
  + id) en lugar del contenido completo.
- **FR-006**: El sistema DEBE mantener una vía explícita (`get_memory <id>`) que
  devuelva el contenido completo sin truncar, para el detalle bajo demanda.
- **FR-007**: `end_session` DEBE persistir un resumen breve de la sesión que en
  el contexto posterior aparezca acotado (no el detalle íntegro).
- **FR-008**: El hook de fin de turno DEBE poder emitir un recordatorio no
  bloqueante de compactación cuando la huella estimada supere un umbral
  configurable, sin ejecutar por su cuenta acciones destructivas ni de
  compactación (que solo el cliente puede realizar).
- **FR-009**: El presupuesto y el umbral DEBEN ser configurables por el usuario
  con valores por defecto sensatos, y el comportamiento por defecto NO DEBE
  degradar la utilidad del contexto para proyectos pequeños.
- **FR-010**: El sistema NO DEBE eliminar ni alterar memorias persistidas al
  acotar la salida; la reducción ocurre solo en la representación emitida, no en
  el almacenamiento.
- **FR-011**: Toda reducción de huella DEBE ser **agnóstica al agente**: opera
  sobre el texto emitido y NO DEBE depender de comandos ni capacidades propias de
  un cliente concreto. Cualquier señal de compactación se expresa en lenguaje
  neutral, no como un comando específico de un agente.
- **FR-012**: `end_session` DEBE poder estructurar el resumen en secciones
  estables (Objetivo / Hallazgos / Logrado / Próximos pasos / Archivos) para que
  su inyección posterior sea compacta y predecible.
- **FR-013**: Al guardar una memoria equivalente a una existente (por identidad
  proyecto+tipo+título dentro de una ventana, o por tópico), el sistema DEBE
  consolidar/actualizar la existente en lugar de crear una fila nueva, y el
  contexto NO DEBE mostrar duplicados de la misma información.

### Key Entities *(include if feature involves data)*

- **Presupuesto de contexto**: límite de tamaño (aprox. en tokens/caracteres)
  para la salida de arranque; configurable; ≤ 0 = sin límite.
- **Umbral de compactación**: tamaño de huella estimada de la sesión a partir
  del cual se sugiere `/compact`; configurable.
- **Entrada de contexto acotada**: representación de una memoria en el contexto,
  con título, extracto truncado y puntero al detalle (`get_memory <id>`).
- **Resumen de sesión**: texto breve y estructurado (Objetivo / Hallazgos /
  Logrado / Próximos pasos / Archivos) persistido por `end_session`, consumido de
  forma acotada por el contexto posterior.
- **Identidad de memoria**: criterio de equivalencia (proyecto+tipo+título o
  tópico) usado para deduplicar/consolidar en la fuente en vez de crear filas
  nuevas.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En un proyecto con ≥100 memorias, la salida de `get_context` con
  el presupuesto por defecto no supera ~una cuarta parte del tamaño actual
  (referencia: de ~82KB observados a ≤ ~20KB), conservando protocolo, conflictos
  y lo esencial reciente.
- **SC-002**: La participación de gomemory en el `/usage` de una sesión de
  trabajo típica baja de forma medible respecto del 22% reportado.
- **SC-003**: Un proyecto pequeño (contexto ya por debajo del presupuesto)
  obtiene una salida sin pérdida de contenido respecto de la actual.
- **SC-004**: El detalle íntegro de cualquier memoria sigue recuperable en un
  solo paso (`get_memory <id>`), sin importar cuánto se haya acotado el contexto.
- **SC-005**: Ningún cambio de acotamiento borra ni modifica datos persistidos
  (verificable comparando la base antes/después de generar contexto).
- **SC-006**: La reducción funciona de forma idéntica para cualquier consumidor
  (Claude Code, otro cliente MCP o el CLI `mem`), verificable comparando la
  salida de `get_context` / `mem context` entre vías: mismo texto acotado.
- **SC-007**: Guardar N veces una memoria equivalente resulta en 1 sola entrada
  consolidada, no en N filas (verificable contando filas antes/después).

## Assumptions

- El objetivo es reducir la **huella emitida** por gomemory; la evicción de lo
  ya presente en la ventana la ejecuta el cliente (`/compact`/`/clear`), no el
  servidor. La feature ofrece la señal, no la evicción.
- El presupuesto por defecto se elige para preservar utilidad; el usuario podrá
  ajustarlo (incluido desactivarlo con ≤ 0). Valor concreto a fijar en `/plan`.
- «Tamaño» se aproxima con una heurística de caracteres/tokens; no requiere un
  tokenizador exacto del modelo para ser útil.
- El detalle completo permanece siempre accesible bajo demanda; acotar el
  contexto no es perder información, es diferir su carga.
- Se reutiliza el mecanismo de configuración existente del proyecto (settings) y
  el hook de fin de turno ya presente; no se introduce infraestructura nueva de
  cliente.
- Las tools que ya devuelven identificadores (para `get_memory`) siguen
  exponiéndolos, de modo que la vía compacta → detalle es navegable.
