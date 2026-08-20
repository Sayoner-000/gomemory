# Feature Specification: Benchmark de tokens por sesión (`mem usage`) y tres optimizaciones validadas con esa medición

**Feature Branch**: `020-token-usage-benchmark`

**Created**: 2026-08-20

**Status**: Draft

**Input**: User description: "Benchmark de tokens (mem usage) + tres optimizaciones por fases: A) medir y persistir por sesión cuántos tokens ahorra gomemory al emitir contexto, con salida en línea de comandos y en la interfaz interactiva; B) consolidación automática de memorias que comparten clave de tópico; C) emisión de contexto en modo índice con detalle bajo demanda. Cada optimización se justifica con el Δ que arroja el benchmark de la fase A. De paso, revisar si la librería de la interfaz interactiva tiene una versión más reciente y actualizarla."

## Contexto y Problema

gomemory ya recorta lo que emite en varios frentes, pero no puede demostrar cuánto ahorra:

- El recorte por presupuesto y la divulgación progresiva (feature 008) solo dejan tras de sí un
  contador escalar de caracteres, sin desglose por operación, sin marca de tiempo y sin sesión.
- El cálculo de reducción que acompaña a la construcción de paquetes de contexto (feature 015)
  se produce en cada llamada y se descarta deliberadamente: el paquete de contexto nunca se
  persiste, y su reporte de reducción se va con él.
- El registro de sesiones no guarda ninguna métrica.

Consecuencia: el ahorro se calcula por llamada y se tira. Nadie puede responder «¿cuánto optimizó
gomemory en esta sesión?», y las tres mejoras que el usuario identificó —el costo de los
descriptores que gomemory publica, la acumulación de memorias que comparten clave de tópico, y la
emisión completa de contexto cuando bastaría un índice— no tienen con qué justificarse ni con qué
validarse una vez hechas.

Esta feature crea esa línea base medida y la usa como criterio de aceptación de las dos
optimizaciones siguientes. El orden no es arbitrario: sin la medición no hay forma de probar que
las otras dos sirven.

## Qué se mide y qué no *(decisión de honestidad — no negociable)*

gomemory no puede leer la ventana de contexto del cliente que lo consume: esa palanca pertenece al
agente. Lo que sí controla, y por tanto lo único que puede medir con honestidad, es el texto que
emite. La salida debe distinguir explícitamente ambas naturalezas:

| Dato                                                                | Naturaleza                          |
| ------------------------------------------------------------------- | ----------------------------------- |
| Tokens de línea base: lo que la respuesta habría costado sin optimizar | medido                              |
| Tokens emitidos: lo que realmente se emitió                          | medido                              |
| Ahorro absoluto y porcentaje de reducción                            | medido (derivado de los anteriores) |
| Costo de los descriptores de operación que gomemory publica          | medido (serialización real)         |
| Número de llamadas por operación                                     | medido                              |
| «Huella evitada» como porcentaje de una ventana de referencia        | estimado, y apagado por defecto     |

Dos precisiones que la salida debe llevar impresas:

1. **La ventana de referencia es un valor que provee el usuario, no una lectura.** Su valor por
   defecto equivale a «sin ventana»; con ese valor la línea del porcentaje sencillamente no
   aparece, de modo que todo lo mostrado por defecto es medido. Quien conozca la ventana de su
   cliente la configura, y entonces la línea aparece rotulada como estimada. Ningún valor por
   defecto puede corresponder a la ventana de un agente concreto.
2. **El conteo de tokens es una aproximación neutral**, no el tokenizador de ningún proveedor. Las
   cifras son comparables contra sí mismas (Δ entre antes y después, porcentajes), no contra la
   facturación de nadie. La cabecera del reporte lo declara.

## Invariantes de agnosticismo *(no negociables)*

Una capacidad de gomemory se define SIEMPRE como contrato neutral; los formatos propios de cada
agente o canal son traducciones de ese contrato, nunca su definición. Ningún canal es el de
referencia. De ahí se derivan tres invariantes que esta feature debe cumplir:

- **El registro nace en la operación, no en el canal.** Quien emite conoce la línea base y lo
  emitido, y es quien lo reporta. Un canal solo aporta su etiqueta. Todos los canales por los que
  gomemory emite quedan medidos por igual: ninguno es el principal y ninguno es una versión
  degradada del otro.
- **La etiqueta de canal es descriptiva, no una autorización.** Un canal que el sistema no conozca
  se registra con su etiqueta y aparece en el reporte; no se valida contra una lista cerrada.
- **La forma que manda es la legible por máquina.** La salida estructurada del reporte es el
  contrato para cualquier consumidor; la salida legible por personas es una traducción de ella, no
  al revés.

Criterio de éxito medible del agnosticismo: añadir un canal emisor nuevo lo hace aparecer en el
reporte sin modificar ni el modelo de datos, ni el contrato de almacenamiento, ni la lógica que
calcula el reporte, ni el formateador.

## Desviaciones respecto de lo ya decidido *(declaradas)*

- **El paquete de contexto sigue sin persistirse.** Esa decisión no se toca. Lo que se persiste es
  un registro de uso nuevo e independiente, derivado de las cifras de reducción, no el paquete.
- **La spec 017 exige que los snapshots no se persistan.** Se respeta: la sección de snapshot
  puntual sigue siendo efímera. Lo que se persiste es el histórico de uso de la sesión, que es una
  entidad distinta.
- **La huella en caracteres de la feature 008 no se elimina.** La consumen el aviso de compactación
  y el enganche de fin de turno. El benchmark en tokens es una capa nueva que convive con ella.
- **La spec 017 queda superada por esta feature**, que absorbe su alcance como segunda sección de
  la pantalla de uso, conservando todos sus requisitos funcionales.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Saber cuánto ahorró gomemory en esta sesión (Priority: P1)

Como usuario de `mem`, quiero pedir un reporte de uso y ver, para la sesión activa, cuántos tokens
habría costado el contexto sin optimizar, cuántos se emitieron realmente, cuánto se ahorró en
absoluto y en porcentaje, y el desglose por operación y por canal; para poder responder con datos
—y no con intuición— qué tanto está sirviendo la memoria persistente.

**Why this priority**: Es el valor central y la precondición de todo lo demás. Sin esta medición no
existe línea base contra la cual comparar ninguna optimización, y las historias 3 y 4 quedan sin
criterio de aceptación.

**Independent Test**: Se prueba por completo ejecutando una secuencia conocida de operaciones que
emiten contexto y comprobando que el reporte imprime esas llamadas, con línea base mayor que lo
emitido en las que optimizan, y con la suma coherente (línea base − ahorro = emitido).

**Acceptance Scenarios**:

1. **Given** una sesión activa en la que se ha emitido contexto varias veces, **When** el usuario
   pide el reporte de uso, **Then** el reporte muestra el total de llamadas, los tokens de línea
   base, los tokens emitidos, el ahorro absoluto y el porcentaje de reducción, más un desglose por
   operación.
2. **Given** una sesión en la que aún no se ha emitido nada, **When** el usuario pide el reporte,
   **Then** el reporte se muestra con ceros y una indicación clara de que no hay actividad
   registrada, sin error.
3. **Given** un reporte cualquiera, **When** se inspecciona su cabecera, **Then** declara que el
   conteo de tokens es una aproximación neutral y que las cifras son comparables contra sí mismas.
4. **Given** la ventana de referencia en su valor por defecto, **When** se pide el reporte,
   **Then** no aparece ninguna línea de porcentaje sobre ventana; **When** el usuario configura una
   ventana mayor que cero, **Then** esa línea aparece y va rotulada como estimada.
5. **Given** contexto emitido por un canal distinto de aquel por el que se pide el reporte,
   **When** se pide el reporte, **Then** esas llamadas aparecen igualmente, etiquetadas con su
   canal de origen.
6. **Given** un consumidor automatizado, **When** pide el reporte en su forma legible por máquina,
   **Then** obtiene una estructura estable y documentada cuyos totales coinciden con los de la
   salida legible por personas.

---

### User Story 2 - Ver lo mismo, y calcular un snapshot puntual, sin salir de la interfaz interactiva (Priority: P1)

Como usuario que trabaja dentro de la interfaz interactiva de `mem`, quiero una pantalla propia de
uso que muestre, en su primera sección, el mismo reporte de la sesión activa que entrega la línea
de comandos; y en una segunda sección, poder escribir una tarea y un presupuesto y ver de inmediato
cuánto se optimizaría el contexto para esa tarea concreta, sin salir a una terminal aparte.

**Why this priority**: Es el segundo de los dos artefactos que el usuario pidió, y absorbe por
completo el alcance de la spec 017, que quedó especificada y sin implementar. Es independiente de
la historia 1 en su prueba (se verifica dentro de la interfaz), pero comparte con ella el reporte.

**Independent Test**: Se prueba entrando a la pantalla desde la interfaz interactiva, comprobando
que la primera sección coincide con la salida de la línea de comandos para la misma sesión, y que
la segunda calcula un snapshot para una tarea escrita a mano sin dejar rastro entre visitas.

**Acceptance Scenarios**:

1. **Given** la interfaz interactiva abierta y una sesión con actividad, **When** el usuario entra
   a la pantalla de uso, **Then** la primera sección muestra el mismo reporte que entrega la línea
   de comandos para esa sesión.
2. **Given** la pantalla de uso abierta, **When** el usuario escribe una tarea y un presupuesto y
   dispara el cálculo, **Then** ve tokens antes y después, porcentaje de reducción y conteo de
   elementos por importancia, para esa tarea puntual.
3. **Given** un snapshot ya calculado, **When** el usuario sale de la pantalla y vuelve a entrar,
   **Then** el snapshot no reaparece: no se conserva ni se acumula entre visitas ni entre
   ejecuciones.
4. **Given** la pantalla de uso, **When** el usuario deja la tarea vacía o escribe un presupuesto
   que no es un entero positivo, **Then** recibe un mensaje de validación claro y el cálculo no se
   dispara.
5. **Given** una tarea cuyo contenido imprescindible excede el presupuesto indicado, **When** se
   calcula el snapshot, **Then** el usuario recibe un mensaje comprensible que lo explica, no un
   error crudo.
6. **Given** la pantalla de uso en cualquier estado previo al cálculo, **When** el usuario pide
   volver, **Then** regresa a la pantalla anterior sin efectos.
7. **Given** un snapshot calculado desde la interfaz, **When** se revisa el estado de la memoria
   del proyecto, **Then** nada se ha alterado por haberlo calculado.

---

### User Story 3 - Dejar de pagar N veces por el mismo tópico (Priority: P2)

Como usuario cuyo proyecto acumula memorias que comparten clave de tópico —donde cada actualización
dejó viva la anterior—, quiero poder consolidar cada grupo en una sola memoria, previsualizando
antes qué se va a fundir y qué se va a borrar; para que el contexto que se emite deje de repetir
versiones sucesivas del mismo asunto.

**Why this priority**: Es la primera optimización que la medición justifica, y ataca una causa
directa y verificable de contexto inflado. Va después de la medición porque su valor solo puede
demostrarse con el Δ que esta arroja.

**Independent Test**: Se prueba partiendo de varias memorias que comparten clave de tópico,
ejecutando la consolidación, y comprobando que queda una sola sin pérdida de contenido, y que la
línea base de la emisión de contexto baja de forma medible entre el antes y el después.

**Acceptance Scenarios**:

1. **Given** varias memorias preexistentes que comparten la misma clave de tópico, **When** el
   usuario ejecuta la consolidación, **Then** queda una sola memoria por clave y ningún contenido
   se pierde.
2. **Given** la operación de consolidación, **When** el usuario la invoca sin pedir explícitamente
   aplicarla, **Then** obtiene una previsualización de lo que ocurriría y nada se modifica.
3. **Given** una consolidación aplicada, **When** se compara la línea base de la emisión de
   contexto antes y después, **Then** la reducción queda registrada como una diferencia medida en
   el reporte de uso.
4. **Given** memorias sin clave de tópico, **When** se ejecuta la consolidación, **Then** no se
   tocan.

---

### User Story 4 - Recibir un índice y pedir el detalle solo cuando hace falta (Priority: P3)

Como usuario que quiere que la primera emisión de contexto de la sesión sea más liviana, quiero
poder elegir que esa emisión entregue el protocolo de trabajo íntegro más un índice de una línea
por memoria —identificador, tipo y título—, dejando el contenido completo disponible bajo demanda
por identificador; y quiero poder volver al modo completo cuando lo prefiera.

**Why this priority**: Es la optimización de mayor impacto potencial y también la de mayor riesgo
de degradar la utilidad del contexto, por lo que va al final: se activa solo cuando la medición ya
existe para cuantificar tanto el ahorro como cualquier pérdida.

**Independent Test**: Se prueba activando el modo índice, comprobando que la emisión contiene todos
los identificadores y ningún cuerpo de memoria, que el protocolo de trabajo se emite íntegro y sin
recortes, y midiendo la diferencia de la primera emisión de contexto de la sesión contra el modo
completo.

**Acceptance Scenarios**:

1. **Given** el modo índice activo, **When** se emite el contexto, **Then** la salida contiene el
   identificador, el tipo y el título de cada memoria seleccionada, y ningún cuerpo de memoria.
2. **Given** el modo índice activo, **When** se inspecciona la emisión, **Then** el protocolo de
   trabajo aparece íntegro, sin recorte alguno.
3. **Given** un índice recibido, **When** el usuario o el agente pide una memoria por su
   identificador, **Then** obtiene su contenido completo, usando una capacidad que ya existe en
   todos los canales, sin necesidad de ninguna capacidad nueva.
4. **Given** el ajuste de modo, **When** el usuario no lo ha tocado, **Then** el comportamiento es
   el actual (modo completo); **When** lo activa y luego lo desactiva, **Then** el comportamiento
   vuelve exactamente al anterior.
5. **Given** ambos modos, **When** se compara la primera emisión de contexto de la sesión en cada
   uno, **Then** la diferencia queda registrada como una medición, no como una estimación.

---

### User Story 5 - Una interfaz interactiva sostenida sobre una base vigente (Priority: P3)

Como responsable del proyecto, quiero que la librería sobre la que corre la interfaz interactiva
esté en una versión vigente y soportada, y que la actualización no cambie ningún comportamiento
observable de las pantallas existentes; para no acumular deuda ni quedar expuesto a defectos ya
corregidos aguas arriba.

**Why this priority**: Es higiene exigida por la constitución del proyecto («sin dependencias
desactualizadas»), y toca la misma superficie que la historia 2. No bloquea a ninguna otra
historia, por eso va en prioridad baja, pero conviene resolverla en el mismo tramo que la pantalla
nueva para no pagar dos veces el costo de verificar la interfaz.

**Independent Test**: Se prueba actualizando la dependencia, ejecutando la suite completa y
recorriendo a mano las pantallas existentes, comprobando que ninguna cambia de comportamiento.

**Acceptance Scenarios**:

1. **Given** la versión actual de la librería de interfaz interactiva, **When** se contrasta con
   las versiones publicadas, **Then** queda registrado qué versión vigente corresponde y si el
   salto es compatible o rompe la interfaz de programación.
2. **Given** la actualización aplicada, **When** se ejecuta la suite de pruebas completa,
   **Then** pasa sin modificar ninguna prueba existente.
3. **Given** la actualización aplicada, **When** el usuario recorre las pantallas existentes,
   **Then** ninguna cambia de aspecto ni de comportamiento observable.
4. **Given** un salto que rompiera la interfaz de programación, **When** se evalúa, **Then** se
   documenta y se deja fuera de esta feature en vez de acometerlo dentro de ella.

---

### Edge Cases

- **No hay sesión activa** cuando se pide el reporte: el sistema muestra el reporte de la última
  sesión conocida o un reporte vacío con explicación, nunca un error crudo.
- **Una operación que no optimiza** (por ejemplo, guardar una memoria): queda registrada con línea
  base igual a lo emitido, para que el porcentaje global no se calcule sobre un subconjunto sesgado
  que solo incluya las operaciones que sí recortan.
- **El registro de uso falla** (almacenamiento no disponible, sesión inconsistente): la operación
  que se estaba ejecutando se completa igual y entrega su resultado; medir nunca puede impedir
  emitir.
- **Ventana de referencia menor que el total consumido**: el porcentaje puede superar el 100 %; se
  muestra tal cual, rotulado como estimado, sin recortarlo ni ocultarlo.
- **Consolidación sobre un grupo de una sola memoria**: no hace nada y lo informa.
- **Consolidación cuando dos memorias del grupo se contradicen**: se conserva el contenido de todas
  al fundir; resolver contradicciones no es competencia de esta operación.
- **Modo índice con cero memorias**: se emite el protocolo íntegro y un índice vacío explícito, no
  una sección ausente.
- **Sesiones muy largas**: el reporte se mantiene legible; el desglose se agrega por operación y
  por canal, no se imprime una línea por llamada.

## Requirements *(mandatory)*

### Functional Requirements

#### Medición y registro (fase A)

- **FR-001**: El sistema DEBE registrar, por cada emisión de contexto, los tokens de línea base
  (lo que la respuesta habría costado sin optimizar) y los tokens efectivamente emitidos.
- **FR-002**: Cada registro DEBE quedar asociado a la sesión de trabajo, al proyecto, a la
  operación que lo produjo, al canal por el que se emitió y al instante en que ocurrió.
- **FR-003**: El registro DEBE originarse en la operación que conoce ambas cifras, de modo que
  toda emisión quede medida por igual con independencia del canal; el canal DEBE limitarse a
  aportar su etiqueta.
- **FR-004**: La etiqueta de canal NO DEBE validarse contra una lista cerrada: un canal
  desconocido se registra con su etiqueta y aparece en el reporte.
- **FR-005**: Las operaciones que emiten sin optimizar DEBEN quedar registradas con línea base
  igual a lo emitido, para que el porcentaje de reducción no se calcule sobre un subconjunto
  sesgado.
- **FR-006**: Un fallo al registrar el uso NO DEBE impedir ni alterar la emisión en curso.
- **FR-007**: El sistema DEBE medir el costo en tokens de los descriptores de operación que él
  mismo publica, mediante su serialización real, y presentarlo como total y como desglose por
  operación.
- **FR-008**: El sistema DEBE usar un único método de conteo de tokens, el mismo que ya emplea
  para presupuestar contexto, de modo que todas las cifras del reporte sean comparables entre sí.
- **FR-009**: La incorporación del registro de uso al almacenamiento DEBE ser aditiva e
  idempotente: aplicarla dos veces sobre un almacenamiento existente no DEBE fallar ni alterar lo
  ya guardado.

#### Reporte y salidas (fase A)

- **FR-010**: El usuario DEBE poder pedir el reporte de uso desde la línea de comandos, con la
  sesión activa por defecto, y poder pedir además una sesión concreta o el acumulado de todas.
- **FR-011**: El reporte DEBE presentar: número de llamadas, tokens de línea base, tokens
  emitidos, ahorro absoluto, porcentaje de reducción, y desglose por operación y por canal.
- **FR-012**: El reporte DEBE ofrecerse en una forma legible por máquina, estable y documentada,
  de la cual la salida legible por personas es una traducción; en ella DEBE cumplirse que línea
  base menos ahorro es igual a lo emitido.
- **FR-013**: La cabecera del reporte DEBE declarar que el conteo de tokens es una aproximación
  neutral y que las cifras son comparables contra sí mismas, no contra la facturación de ningún
  proveedor.
- **FR-014**: El sistema DEBE ofrecer un ajuste de ventana de referencia cuyo valor por defecto
  equivalga a «sin ventana»; ningún valor por defecto DEBE corresponder a la ventana de un agente
  concreto.
- **FR-015**: Con la ventana de referencia en su valor por defecto, el reporte NO DEBE mostrar
  ninguna línea de porcentaje sobre ventana; con un valor mayor que cero, esa línea DEBE aparecer
  rotulada explícitamente como estimada, distinguiéndola de todo lo demás, que es medido.
- **FR-016**: Todo dato del reporte cuya naturaleza sea estimada DEBE ir rotulado como tal; lo no
  rotulado DEBE ser medido.
- **FR-017**: Añadir un canal emisor nuevo DEBE bastar para que sus emisiones aparezcan en el
  reporte, sin modificar el modelo de datos, el contrato de almacenamiento, el cálculo del reporte
  ni el formateador.

#### Interfaz interactiva (fase A, absorbe la spec 017)

- **FR-018**: La interfaz interactiva DEBE ofrecer una pantalla de uso propia, alcanzable desde la
  pantalla principal.
- **FR-019**: La primera sección de esa pantalla DEBE mostrar, para la sesión activa, el mismo
  reporte que entrega la línea de comandos.
- **FR-020**: La segunda sección DEBE permitir escribir una tarea y un presupuesto y disparar el
  mismo proceso de optimización de contexto que ya existe, mostrando tokens antes y después,
  porcentaje de reducción y conteo de elementos por importancia.
- **FR-021**: El sistema DEBE validar, antes de calcular el snapshot, que la tarea no esté vacía y
  que el presupuesto sea un entero positivo, informándolo con un mensaje claro.
- **FR-022**: Cuando el contenido imprescindible para la tarea exceda el presupuesto indicado, el
  sistema DEBE explicarlo con un mensaje comprensible, no con un error crudo.
- **FR-023**: El snapshot NO DEBE conservarse, acumularse ni persistirse entre visitas a la
  pantalla ni entre ejecuciones.
- **FR-024**: El usuario DEBE poder volver a la pantalla anterior en cualquier momento previo al
  cálculo.
- **FR-025**: Calcular un snapshot NO DEBE alterar el contenido de la memoria del proyecto.

#### Consolidación por clave de tópico (fase B)

- **FR-026**: El sistema DEBE poder consolidar, dentro de un proyecto, todas las memorias que
  comparten una misma clave de tópico en una sola, conservando el contenido de todas.
- **FR-027**: La consolidación DEBE ofrecer una previsualización que no modifique nada, y solo
  aplicar los cambios cuando el usuario lo pida explícitamente.
- **FR-028**: La consolidación DEBE ser alcanzable tanto desde la línea de comandos como desde la
  pantalla de mantenimiento de la interfaz interactiva.
- **FR-029**: Las memorias sin clave de tópico NO DEBEN verse afectadas.
- **FR-030**: El efecto de la consolidación sobre la emisión de contexto DEBE quedar demostrado
  como una diferencia medida en el reporte de uso, comparando antes y después.

#### Emisión en modo índice (fase C)

- **FR-031**: El sistema DEBE poder emitir el contexto en modo índice: el protocolo de trabajo
  íntegro más una línea por memoria con su identificador, su tipo y su título, sin contenido.
- **FR-032**: El protocolo de trabajo NUNCA DEBE recortarse en modo índice.
- **FR-033**: El detalle de cualquier memoria del índice DEBE poder recuperarse por su
  identificador con una capacidad ya existente en todos los canales, sin introducir ninguna
  capacidad nueva.
- **FR-034**: El modo DEBE elegirse mediante un ajuste cuyo valor por defecto conserve el
  comportamiento actual, y DEBE ser reversible sin efectos residuales.
- **FR-035**: La diferencia entre ambos modos, para la primera emisión de contexto de la sesión,
  DEBE quedar registrada como una medición, no como una estimación.

#### Convivencia con lo existente

- **FR-036**: El paquete de contexto DEBE seguir sin persistirse; lo que se persiste es un
  registro de uso independiente derivado de las cifras de reducción.
- **FR-037**: El contador de huella en caracteres existente DEBE seguir funcionando sin cambios:
  el benchmark en tokens es una capa adicional, no un reemplazo.

#### Base de la interfaz interactiva

- **FR-038**: La librería sobre la que corre la interfaz interactiva DEBE actualizarse a una
  versión vigente y soportada, siempre que el salto no rompa la interfaz de programación; si la
  rompiera, la actualización DEBE documentarse y quedar fuera del alcance de esta feature.
- **FR-039**: La actualización NO DEBE cambiar el comportamiento observable de ninguna pantalla
  existente, ni requerir modificar pruebas ya escritas.

### Key Entities

- **Registro de uso**: una emisión medida. Recoge la sesión, el proyecto, la operación que la
  produjo, la etiqueta del canal por el que salió, los tokens de línea base, los tokens emitidos y
  el instante. Es la unidad mínima persistida y es independiente del paquete de contexto.
- **Reporte de uso**: agregación de registros de uso para una sesión, para varias o para todas.
  Deriva ahorro absoluto y porcentaje de reducción, y agrupa por operación y por canal. Es lo que
  se muestra y lo que se publica en forma legible por máquina.
- **Costo de descriptores publicados**: el peso en tokens de las descripciones de operación que
  gomemory publica hacia quien lo consume, medido sobre su serialización real. Es lo que gomemory
  emite, no lo que un cliente arma con ello.
- **Canal emisor**: etiqueta descriptiva del medio por el que salió una emisión. Es un dato
  abierto, no una lista cerrada de valores permitidos.
- **Ventana de referencia**: valor opcional que provee el usuario para expresar el ahorro como
  porcentaje de la ventana de su cliente. Su valor por defecto equivale a «sin ventana» y en ese
  caso el dato derivado no se muestra.
- **Grupo de tópico**: conjunto de memorias de un proyecto que comparten clave de tópico y que la
  consolidación funde en una sola.
- **Modo de emisión de contexto**: elección entre entregar el contenido completo o un índice con
  detalle bajo demanda. Su valor por defecto conserva el comportamiento actual.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tras una sesión con al menos cuatro emisiones de contexto, el usuario obtiene en un
  solo paso un reporte que muestra las cuatro llamadas con su desglose por operación, y en el que
  la línea base supera a lo emitido en todas las que optimizan.
- **SC-002**: En toda salida del reporte se cumple que línea base menos ahorro es igual a lo
  emitido, con diferencia cero.
- **SC-003**: Con la ventana de referencia en su valor por defecto, el 100 % de las líneas del
  reporte corresponde a datos medidos: no aparece ninguna cifra estimada sin rotular.
- **SC-004**: El costo reportado de los descriptores publicados coincide con la serialización real
  de las operaciones publicadas, con diferencia cero en el número de operaciones contadas.
- **SC-005**: Emitir contexto por un canal distinto del habitual hace que esa emisión aparezca en
  el reporte con su etiqueta, sin haber modificado el modelo de datos, el contrato de
  almacenamiento, el cálculo del reporte ni el formateador.
- **SC-006**: La primera sección de la pantalla de uso y la salida de la línea de comandos
  coinciden cifra por cifra para la misma sesión.
- **SC-007**: Calcular un snapshot desde la interfaz y volver a entrar a la pantalla deja cero
  rastros del snapshot anterior, y cero cambios en la memoria del proyecto.
- **SC-008**: Partiendo de un proyecto con al menos tres memorias que comparten clave de tópico,
  la consolidación deja exactamente una por clave, sin pérdida de contenido, y la línea base de la
  emisión de contexto medida antes y después baja de forma verificable.
- **SC-009**: En modo índice, la emisión contiene el 100 % de los identificadores de las memorias
  seleccionadas y cero cuerpos de memoria, y el protocolo de trabajo aparece completo.
- **SC-010**: Activar y luego desactivar el modo índice devuelve la emisión de contexto a un
  resultado idéntico al de partida.
- **SC-011**: Aplicar dos veces la incorporación del registro de uso sobre un almacenamiento
  existente termina sin error y sin alterar los datos previos.
- **SC-012**: Tras actualizar la librería de la interfaz interactiva, la suite completa pasa sin
  que se haya modificado ninguna prueba existente, y las pantallas previas conservan su
  comportamiento.

## Assumptions

- El conteo de tokens usa el método aproximado y determinista que el proyecto ya emplea para
  presupuestar contexto; no se introduce ningún tokenizador de proveedor.
- La medición se activa sin que el usuario tenga que pedirlo, y no se contempla un interruptor para
  apagarla: el costo de registrar una emisión ya medida es despreciable frente a la emisión misma.
- La sección de snapshot de la pantalla de uso hereda íntegros los requisitos de la spec 017, que
  queda marcada como superada por esta feature.
- El histórico de registros de uso se conserva sin caducidad propia: sigue el ciclo de vida de las
  sesiones del proyecto, y las herramientas de mantenimiento existentes son las que lo depuran.
- La consolidación por clave de tópico es irreversible una vez aplicada, por lo que su modo de
  previsualización es el comportamiento por defecto.
- La fase A es autónoma y entregable por sí sola: si el trabajo debe cortarse, el corte natural es
  al terminarla, porque entrega los dos artefactos pedidos y produce los datos que justifican las
  fases B y C.
- La actualización de la librería de interfaz interactiva se limita al salto compatible dentro de
  la misma línea mayor; un salto que cambie la línea mayor y rompa la interfaz de programación se
  documenta y se trata como feature aparte.

## Out of Scope *(declarado)*

- Leer o modificar la ventana de contexto del cliente que consume gomemory: esa palanca no le
  pertenece.
- Importar el reporte de consumo propio de un agente concreto: dependería de un formato externo
  fuera de control del proyecto.
- Sustituir el contador de huella en caracteres existente: el benchmark en tokens convive con él.
- Cambiar la decisión de que el paquete de contexto no se persiste.
- Migrar a una línea mayor de la librería de interfaz interactiva que rompa la interfaz de
  programación.
