# Feature Specification: Modo Plan Atómico con Memoria

**Feature Branch**: `013-atomic-plan-mode`

**Created**: 2026-08-06

**Status**: Draft

**Input**: User description: "tengo este skill /Users/josegomezj/home/DOCS/SKILL_ATOMIC_DESCOMPOSITION_SYSTEMS_ADS.md que quiero que cualquier agente la pueda tener para iniciar ya sea en claude en modo /plan y opencode en modo /plan . Se puede optimizar , y quiero que gomemory active cuando se entre en modo plan , ya un extensor que permite ir con codebase memory, pero para que sea efectivo estos planes sean unas tareas atomicas para garantizar y asegurar que se llegue al objetivo."

## Contexto y Problema

Hoy, cuando un agente entra en modo plan, planifica **a ciegas y en grueso**:

1. **Planifica sin historia.** El proyecto acumula decisiones, causas raíz de bugs,
   convenciones y estructura de código en gomemory, pero ese material solo llega al
   agente si alguien lo pide explícitamente o si la sesión arranca desde cero. Un plan
   redactado sin ese material vuelve a proponer enfoques ya descartados y reintroduce
   bugs cuya causa raíz ya está documentada.
2. **Produce pasos gruesos, no verificables.** Un plan típico contiene ítems como
   "implementar la integración" o "arreglar el módulo": no se puede demostrar que
   estén hechos, así que se dan por completados sin evidencia y el objetivo real
   queda a medias.
3. **La guía de descomposición no viaja con el proyecto.** Existe un método probado de
   descomposición atómica, ya optimizado por el usuario y conservado en
   [`reference-ads-baseline.md`](./reference-ads-baseline.md), pero vive como archivo
   suelto en el equipo de una persona: hay que pegarlo a mano en cada agente y en cada
   proyecto, y todavía no usa el historial del proyecto al descomponer.

Esta funcionalidad convierte esas tres carencias en una capacidad instalable: al entrar
en modo plan, el agente **invoca la memoria por su propia iniciativa**, aplica el
**método de descomposición atómica** y entrega un plan cuyas tareas son verificables una
por una.

La activación es autónoma del agente y está dirigida por el protocolo del proyecto, no
por un mecanismo propio de un agente concreto. Esa es la decisión de diseño que hace
universal la cobertura: cualquier agente que lea el protocolo y pueda alcanzar gomemory
queda cubierto —los de hoy y los que aparezcan después— sin escribir una integración
nueva para cada uno.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Planificar con el historial del proyecto ya cargado (Priority: P1)

Una persona desarrolladora entra en modo plan en su agente —el que sea— para abordar un
cambio. Sin escribir ningún comando adicional, el agente consulta la memoria del proyecto
por iniciativa propia y trae el historial: decisiones de arquitectura previas, bugs
corregidos con su causa raíz, convenciones establecidas y el resumen de estructura de
código disponible. El plan que redacta se apoya en ese material en lugar de reinventarlo.

**Why this priority**: Es el núcleo del valor y lo que el usuario pidió primero
("quiero que gomemory active cuando se entre en modo plan"). Por sí sola ya evita la
clase de error más cara: replanificar sobre terreno ya recorrido. Es entregable sin
ninguna de las otras historias.

**Independent Test**: En un proyecto con memorias guardadas, entrar en modo plan y pedir
un plan sobre un área que tiene decisiones y bugs registrados. Se verifica que el plan
referencia al menos un elemento del historial (una decisión previa, una causa raíz o una
convención) que no fue mencionado por la persona en su solicitud.

**Acceptance Scenarios**:

1. **Given** un proyecto con memoria inicializada y memorias guardadas, **When** la
   persona entra en modo plan, **Then** el agente consulta la memoria por iniciativa
   propia y dispone del contexto antes de redactar el plan, sin intervención manual.
2. **Given** un proyecto sin memoria inicializada, o con la integración apagada,
   **When** la persona entra en modo plan, **Then** el modo plan funciona con normalidad
   y no aparece ningún error ni mensaje de fallo.
3. **Given** un proyecto donde el historial fue cargado antes en la misma sesión,
   **When** la persona vuelve a entrar en modo plan, **Then** el contexto no se
   reinyecta de forma redundante ni satura la conversación.
4. **Given** un proyecto con un proveedor de grafo de código conectado, **When** se
   carga el contexto de planificación, **Then** el resumen de estructura de código
   aparece rotulado como tal y separado del historial de memorias.

---

### User Story 2 - Recibir el plan como árbol de tareas atómicas verificables (Priority: P1)

La persona recibe el plan no como prosa ni como una lista de intenciones, sino como un
árbol de tareas donde cada hoja es **atómica**: una acción concreta, ejecutable sin
decidir "cómo", con un resultado verificable declarado. Puede recorrer la lista y, para
cada ítem, saber exactamente qué debe existir para considerarlo hecho.

**Why this priority**: Es la otra mitad del objetivo declarado ("que estos planes sean
unas tareas atómicas para garantizar y asegurar que se llegue al objetivo"). Sin esto, el
contexto histórico solo produce planes mejor informados pero igual de imposibles de
verificar. Es independiente de la Historia 1: aporta valor incluso en un proyecto sin
historial acumulado.

**Independent Test**: Pedir un plan para una tarea de varios pasos en un proyecto nuevo
(sin memorias). Se verifica que el resultado es un árbol jerárquico numerado, que cada
hoja declara un resultado verificable, y que ninguna hoja es del tipo "implementar la
funcionalidad" o "arreglar el módulo".

**Acceptance Scenarios**:

1. **Given** una solicitud de planificación de varios pasos, **When** el agente entrega
   el plan, **Then** el plan se presenta como árbol jerárquico numerado con las tareas
   atómicas identificadas explícitamente.
2. **Given** una tarea del plan, **When** se la examina, **Then** declara una acción
   concreta, su resultado esperado y cómo se verifica que está cumplida.
3. **Given** una solicitud cuya descomposición supera el umbral de tareas manejables,
   **When** el agente entrega el plan, **Then** advierte del tamaño y propone un recorte
   o priorización en lugar de entregar una lista inabarcable.
4. **Given** una solicitud trivial de un solo paso, **When** el agente responde,
   **Then** no impone la ceremonia de descomposición sobre algo que no la necesita.
5. **Given** un plan entregado, **When** existen dependencias entre tareas, **Then** el
   orden de ejecución dependiente queda explícito.
6. **Given** un plan en borrador con una tarea hoja demasiado gruesa, **When** el agente
   ejecuta su autoverificación antes de entregar, **Then** esa tarea se descompone o se
   entrega marcada como no atómica con el motivo declarado, nunca disfrazada de atómica.

---

### User Story 3 - Disponer del método en cualquier proyecto y en cualquier agente (Priority: P2)

La persona habilita el método una sola vez y, a partir de ahí, todos sus proyectos —los
actuales y los que cree después— arrancan el modo plan con planificación atómica,
**cualquiera que sea el agente que use**: el que ya tenía, el que pruebe mañana o el que
aún no existe. No hace falta trabajo de integración específico por agente: basta con que
el agente lea el protocolo del proyecto y pueda alcanzar la memoria. Si un proyecto
concreto necesita algo distinto, instala su propia versión localmente, que prevalece.

**Why this priority**: Es lo que convierte un documento personal en una capacidad del
producto ("quiero que cualquier agente la pueda tener"). Se apoya en las Historias 1 y 2
—necesita algo que distribuir— pero es independiente en prueba y despliegue: el mismo
mecanismo ya distribuye el brazo extensor existente.

**Independent Test**: Habilitar el método en ámbito global y verificar que un proyecto
que nunca lo instaló arranca el modo plan con planificación atómica; repetir la prueba
con un agente que no tenga integración dedicada y comprobar que se comporta igual; luego
instalarlo localmente en otro proyecto y verificar que la versión local prevalece.

**Acceptance Scenarios**:

1. **Given** el método habilitado en ámbito global, **When** la persona abre un proyecto
   que nunca lo instaló, **Then** el modo plan ya cuenta con planificación atómica, con
   contenido equivalente en todos los agentes que lean el protocolo.
2. **Given** un agente sin integración dedicada en gomemory pero capaz de leer el
   protocolo del proyecto y alcanzar la memoria, **When** la persona planifica con él,
   **Then** carga el contexto y aplica la descomposición atómica igual que los agentes
   con integración dedicada, sin trabajo adicional específico para ese agente.
3. **Given** el método habilitado en ámbito global, **When** un proyecto tiene además
   una instalación local, **Then** la versión local prevalece para ese proyecto y no
   afecta a los demás.
4. **Given** un ámbito donde el pack ya está instalado y sin modificar, **When** se
   vuelve a instalar, **Then** no se reescribe contenido idéntico ni se duplican
   entradas de configuración.
5. **Given** un ámbito donde el pack está instalado en una versión anterior, **When**
   se instala una versión más nueva, **Then** el contenido se actualiza sin dejar
   restos de la versión anterior.
6. **Given** un agente que no tiene noción de modo plan, **When** se instala gomemory,
   **Then** la instalación completa el resto de su trabajo sin fallar.

---

### User Story 4 - Apagar o ajustar la activación automática (Priority: P3)

La persona decide que en un proyecto concreto no quiere que el modo plan cargue el
historial automáticamente (por ruido, por tamaño del contexto o porque el proyecto no lo
amerita) y lo desactiva desde la configuración, sin desinstalar nada ni editar archivos
de cada agente por separado.

**Why this priority**: Es control del usuario sobre un comportamiento automático que
consume contexto. Necesario para que la automatización sea aceptable, pero el valor
principal se entrega sin ella.

**Independent Test**: Desactivar la integración en la configuración del proyecto, entrar
en modo plan y verificar que no se inyecta contexto histórico; reactivarla y verificar
que vuelve a inyectarse.

**Acceptance Scenarios**:

1. **Given** la integración activa, **When** la persona la desactiva desde la
   configuración del proyecto, **Then** entrar en modo plan deja de cargar el historial y
   el resto de la planificación sigue funcionando.
2. **Given** la integración desactivada, **When** la persona la reactiva, **Then** el
   comportamiento automático se restablece sin reinstalar.
3. **Given** el método habilitado en ámbito global, **When** la persona lo apaga en un
   proyecto concreto, **Then** ese proyecto deja de activarlo y los demás siguen igual.
4. **Given** cualquier estado de la configuración, **When** la persona consulta el ajuste,
   **Then** son visibles desde la interfaz de configuración de gomemory tanto su estado
   actual como el ámbito desde el que la funcionalidad está activa.

---

### User Story 5 - Conservar el plan aprobado como contrato del objetivo (Priority: P3)

Al aprobar el plan, la lista de tareas atómicas queda registrada como el contrato de lo
que significa "objetivo cumplido" para ese trabajo. Más adelante —ya sea la persona o el
flujo de implementación existente— puede contrastar lo entregado contra esa lista y saber
qué quedó fuera y por qué.

**Why this priority**: Es lo que cierra la promesa de "garantizar y asegurar que se llegue
al objetivo": sin registro del contrato, la atomicidad se pierde en cuanto termina el
turno. Esta historia solo persiste el contrato; quién lo verifica y cuándo queda fuera
del alcance de esta funcionalidad (ver FR-020 y FR-021). Se apoya en el registro de
planes aprobados que ya existe, por lo que es una extensión y no una capacidad nueva
completa.

**Independent Test**: Aprobar un plan atómico y verificar que queda registrado con su
descomposición recuperable; consultarlo después y comprobar que la lista de tareas
atómicas es legible y contrastable contra el trabajo hecho.

**Acceptance Scenarios**:

1. **Given** un plan atómico aprobado, **When** termina el turno, **Then** el plan queda
   registrado en la memoria del proyecto conservando su descomposición en tareas.
2. **Given** un plan registrado, **When** la persona lo consulta después, **Then** puede
   recorrer las tareas atómicas y contrastarlas con lo entregado.
3. **Given** un plan rechazado o abandonado por la persona, **When** termina el turno,
   **Then** no se registra como decisión del proyecto.

---

### Edge Cases

- **Proyecto sin memoria inicializada**: entrar en modo plan no debe fallar ni mostrar
  errores; el método de descomposición atómica sigue disponible aunque no haya historial.
- **Historial muy grande**: el contexto inyectado debe respetar el presupuesto de tamaño
  ya configurado en gomemory, sin desplazar la solicitud de la persona ni el propio plan.
- **Herramienta de memoria no disponible** (binario ausente, versión incompatible, fallo
  interno): degradación silenciosa; el modo plan continúa.
- **Modo plan entrado varias veces en la misma sesión**: no reinyectar el mismo contexto
  de forma acumulativa.
- **Solicitud nueva a mitad de un plan en curso**: el comportamiento ante un cambio de
  objetivo debe estar definido y no dejar dos planes activos ambiguos.
- **Dependencias circulares entre tareas propuestas**: deben resolverse con un orden
  explícito en lugar de entregarse como ciclo.
- **Descomposición excesiva**: debe existir un límite de profundidad y un criterio de
  parada para que la atomicidad no degenere en fragmentación sin valor.
- **Agente sin noción de modo plan**: la instalación no debe fallar; la activación se
  dispara al detectar intención de planificación en la solicitud de la persona.
- **Agente sin acceso al servidor de herramientas de memoria**: debe poder alcanzar
  gomemory por línea de comandos y comportarse igual.
- **Agente que ignora la instrucción del protocolo**: el modo plan sigue funcionando; se
  pierde la carga de contexto, no la capacidad de planificar.
- **Agente que no lee el protocolo del proyecto**: queda fuera de cobertura por
  definición, sin que ello rompa nada para los demás.
- **Instalación global y local simultáneas**: debe quedar inequívoco cuál gobierna, sin
  que las dos versiones del método se apliquen a la vez ni se dupliquen entre sí.
- **Tarea que no puede hacerse atómica** (falta información o depende de una decisión de
  la persona): se entrega marcada como tal con su motivo, sin bloquear el plan completo
  ni disfrazarla de atómica.
- **Ámbito global habilitado sobre un proyecto sin memoria inicializada**: la
  planificación atómica debe seguir disponible aunque no haya historial que inyectar.
- **Conflicto con el flujo SDD existente** (`/speckit-plan`, `/speckit-tasks`): debe
  quedar claro cuál gobierna para no producir dos planes rivales sobre el mismo trabajo.
- **Archivos del pack modificados a mano por la persona**: la reinstalación debe tener un
  comportamiento definido y no destruir trabajo silenciosamente.

## Requirements *(mandatory)*

### Functional Requirements

**Activación y contexto**

- **FR-001**: Al entrar en modo plan, el agente DEBE invocar gomemory por sí mismo para
  cargar el contexto histórico del proyecto, sin que la persona lo solicite y sin
  depender de que el entorno lo inyecte por él.
- **FR-002**: La instrucción que produce esa invocación autónoma DEBE distribuirse en un
  formato que cualquier agente lea, y NO DEBE depender de un mecanismo propietario de un
  agente concreto. Un agente que sepa leer el protocolo del proyecto y alcanzar gomemory
  queda cubierto sin trabajo adicional específico para él.
- **FR-003**: El agente DEBE poder alcanzar gomemory tanto por su servidor de
  herramientas como por su interfaz de línea de comandos, y usar la que tenga disponible.
- **FR-004**: Para agentes sin un modo plan nativo, la activación DEBE dispararse igual al
  detectar intención de planificación en la solicitud de la persona.
- **FR-005**: El contexto de planificación DEBE incluir el historial que gomemory ya
  produce: memorias agrupadas por tipo (decisiones, patrones, bugfixes, aprendizajes),
  preferencias del usuario y relaciones entre memorias.
- **FR-006**: Cuando haya un proveedor de grafo de código conectado, el contexto DEBE
  incluir además su resumen de estructura, rotulado y separado del historial de memorias.
- **FR-007**: El contexto cargado DEBE respetar el presupuesto de tamaño configurado en
  gomemory, sin superarlo aunque el historial crezca.
- **FR-008**: El agente NO DEBE recargar el mismo contexto de forma repetida y acumulativa
  dentro de una misma sesión.

**Descomposición atómica**

- **FR-009**: El sistema DEBE proveer al agente, al entrar en modo plan, un método de
  descomposición que convierta el objetivo en un árbol de tareas hasta llegar a tareas
  atómicas.
- **FR-010**: El sistema DEBE definir un criterio de atomicidad verificable, con
  condiciones explícitas que una tarea cumple o no cumple.
- **FR-011**: Cada tarea atómica DEBE declarar su acción, su resultado esperado y cómo se
  verifica que está cumplida.
- **FR-012**: El plan DEBE presentarse como árbol jerárquico numerado, con las tareas
  atómicas identificadas de forma distinguible de las tareas intermedias.
- **FR-013**: El sistema DEBE establecer límites a la descomposición (profundidad máxima
  y criterio de parada) para evitar fragmentación sin valor.
- **FR-014**: El sistema DEBE hacer explícito el orden de ejecución cuando existan
  dependencias entre tareas, señalar cuáles pueden ir en paralelo, y resolver dependencias
  circulares con una prioridad declarada.
- **FR-015**: El método DEBE ser proporcional: no imponer descomposición formal a
  solicitudes triviales de un solo paso.
- **FR-016**: El método DEBE aprovechar el contexto histórico cargado al descomponer, de
  modo que las tareas atómicas se apoyen en decisiones, convenciones y causas raíz ya
  registradas en lugar de contradecirlas.
- **FR-017**: El sistema DEBE advertir y proponer priorización cuando la descomposición
  supere un volumen de tareas manejable en un solo plan.
- **FR-018**: El método DEBE incluir un paso de autoverificación previo a la entrega: el
  agente contrasta cada tarea hoja contra el criterio de atomicidad y corrige las que no
  lo cumplen **antes** de presentar el plan a la persona.
- **FR-019**: La autoverificación NO DEBE bloquear la entrega del plan. Si una tarea no
  puede hacerse atómica (por información faltante o por depender de una decisión de la
  persona), el plan se entrega igualmente con esa tarea marcada como no atómica y con el
  motivo declarado.

**Alcance del método**

- **FR-020**: En modo plan, el método DEBE cubrir la descomposición recursiva del objetivo
  y la presentación del árbol de tareas, y DEBE detenerse ahí: entrega el plan y no
  ejecuta.
- **FR-021**: El método NO DEBE, en modo plan, ejecutar tareas, marcar estado de avance ni
  producir la entrega final; esas responsabilidades permanecen en el flujo SDD ya
  existente (`/speckit-tasks`, `/speckit-implement`).
- **FR-022**: La relación entre este modo plan y el flujo SDD existente (`/speckit-plan`,
  `/speckit-tasks`) DEBE estar definida de forma que nunca queden dos planes rivales
  sobre el mismo trabajo.

**Distribución e instalación**

- **FR-023**: El pack de planificación atómica DEBE distribuirse mediante el mismo paso
  de instalación de gomemory que ya distribuye el resto de la integración, sin pasos
  manuales adicionales.
- **FR-024**: El sistema DEBE soportar dos ámbitos de instalación: **global**, que cubre
  de una sola vez todos los proyectos presentes y futuros de la persona, y **por
  proyecto**, que aplica solo al proyecto donde se instala.
- **FR-025**: El ámbito global DEBE ser el comportamiento por defecto; una instalación
  por proyecto DEBE prevalecer sobre la global para ese proyecto.
- **FR-026**: Un proyecto DEBE poder apagar la funcionalidad localmente aunque el ámbito
  global esté activo, sin afectar a los demás proyectos.
- **FR-027**: La instalación DEBE dejar el método disponible, con contenido equivalente,
  para todo agente que lea el protocolo del proyecto — no solo para los agentes con
  integración dedicada.
- **FR-028**: Cuando un agente admita además un formato propio más conveniente para
  invocar el método, la instalación PUEDE proveerlo, siempre que el contenido sea
  equivalente al del protocolo común y no lo contradiga.
- **FR-029**: La instalación DEBE ser idempotente: repetirla sobre un proyecto o ámbito
  sin cambios no reescribe contenido idéntico ni duplica entradas de configuración.
- **FR-030**: La instalación DEBE actualizar el pack cuando exista una versión anterior,
  sin dejar restos de la versión previa, en ambos ámbitos.
- **FR-031**: La ausencia de soporte de modo plan en un agente NO DEBE hacer fallar la
  instalación del resto de la integración.

**Control, resiliencia y registro**

- **FR-032**: La persona DEBE poder desactivar y reactivar la activación automática desde
  la configuración del proyecto, sin desinstalar ni editar la configuración de cada
  agente por separado.
- **FR-033**: El estado de ese ajuste, y el ámbito desde el que la funcionalidad está
  activa (global o local), DEBEN ser consultables desde la interfaz de configuración de
  gomemory.
- **FR-034**: Toda la cadena DEBE degradar en silencio: memoria no inicializada,
  herramienta ausente o fallo interno terminan sin salida y sin error, y nunca
  interrumpen el modo plan. El método de descomposición DEBE seguir aplicándose aunque no
  haya contexto que cargar.
- **FR-035**: Un plan atómico aprobado DEBE quedar registrado en la memoria del proyecto
  conservando su descomposición en tareas, de modo que sea recuperable y contrastable
  después.
- **FR-036**: Un plan que la persona no aprueba NO DEBE registrarse como decisión del
  proyecto.

### Key Entities

- **Pack de planificación atómica**: el conjunto instalable que hace posible la
  funcionalidad — el método de descomposición, la activación en modo plan y su
  configuración. Es lo que se distribuye e idempotentemente se actualiza.
- **Método de descomposición atómica**: las reglas que convierten un objetivo en un árbol
  de tareas: criterio de atomicidad, límites de profundidad, formato de presentación y
  manejo de dependencias. Parte de la línea base en
  [`reference-ads-baseline.md`](./reference-ads-baseline.md), a la que se añaden el uso
  del historial y la autoverificación.
- **Instrucción de activación**: la directiva, escrita en el protocolo del proyecto, que
  ordena al agente cargar el contexto y aplicar el método al entrar en modo plan. Es el
  elemento que hace universal la cobertura: no es código de integración por agente, sino
  texto que cualquier agente lee.
- **Tarea atómica**: la unidad hoja del plan. Atributos: identificador jerárquico, acción,
  objeto, resultado esperado, criterio de verificación y dependencias con otras tareas.
- **Contexto de planificación**: el material histórico que se entrega al agente al entrar
  en modo plan — memorias por tipo, preferencias, relaciones y, si existe, resumen de
  estructura de código. Acotado por un presupuesto de tamaño.
- **Registro de plan aprobado**: el plan aceptado por la persona, persistido como decisión
  del proyecto con su descomposición conservada, que sirve de contrato de "objetivo
  cumplido".
- **Ajuste de activación**: la preferencia por proyecto que enciende o apaga la carga
  automática de contexto en modo plan.
- **Ámbito de instalación**: dónde vive el pack y a qué proyectos alcanza — global (todos
  los proyectos de la persona) o local (uno solo). El local prevalece sobre el global.
- **Resultado de autoverificación**: por cada tarea hoja, si cumple o no el criterio de
  atomicidad y, cuando no lo cumple, el motivo por el que no pudo descomponerse más.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En un proyecto con historial acumulado, al menos el 80 % de los planes
  producidos en modo plan referencian explícitamente al menos un elemento del historial
  (decisión, convención o causa raíz) que la persona no mencionó en su solicitud.
- **SC-002**: El 100 % de las tareas hoja de un plan declara un resultado verificable, o
  bien queda marcada explícitamente como no atómica con su motivo. Ninguna tarea hoja
  queda enunciada como una intención genérica sin una de las dos cosas.
- **SC-003**: Una persona puede recorrer un plan entregado y determinar, para cada tarea
  atómica, si está cumplida o no, sin pedir aclaraciones adicionales al agente.
- **SC-004**: Tras habilitar la funcionalidad una sola vez en ámbito global, un proyecto
  nuevo la tiene disponible sin ningún paso adicional de instalación.
- **SC-005**: Incorporar un agente nuevo, no previsto durante la construcción, no requiere
  ningún cambio en la funcionalidad: basta con que lea el protocolo del proyecto y pueda
  alcanzar la memoria.
- **SC-006**: El comportamiento en modo plan —contexto cargado y forma del plan— es
  equivalente entre los agentes verificados, sin que ninguno quede con una versión
  degradada del método.
- **SC-007**: Habilitar la funcionalidad por primera vez, en cualquiera de los dos
  ámbitos, se completa en menos de 2 minutos.
- **SC-008**: Repetir la instalación sobre un ámbito sin cambios no produce ninguna
  modificación de contenido ni entradas de configuración duplicadas.
- **SC-009**: En proyectos sin memoria inicializada, con la integración apagada o con la
  herramienta de memoria ausente, el modo plan funciona con normalidad en el 100 % de los
  casos, sin errores visibles, y la descomposición atómica se sigue aplicando.
- **SC-010**: El contexto cargado nunca supera el presupuesto de tamaño configurado, en
  el 100 % de las activaciones.
- **SC-011**: Los planes aprobados quedan recuperables con su descomposición en tareas en
  el 100 % de los casos aprobados; los no aprobados no se registran en ninguno.
- **SC-012**: Activar o desactivar el comportamiento automático toma menos de 30 segundos
  y no requiere editar la configuración de ningún agente a mano.
- **SC-013**: Cuando coexisten instalación global y local, el 100 % de las sesiones de
  modo plan aplica exactamente una de las dos versiones del método, nunca ambas.

## Assumptions

- **Modelo de activación: autónoma y universal**. La activación no la produce el entorno
  inyectando contexto por el agente, sino el propio agente invocando la memoria al entrar
  en modo plan, siguiendo una instrucción del protocolo del proyecto. Esto es lo que
  permite cubrir cualquier agente sin integración dedicada: la condición de compatibilidad
  es leer el protocolo y poder alcanzar gomemory (por servidor de herramientas o por
  línea de comandos), no pertenecer a una lista.
- **Agentes de referencia para verificación**: Claude Code y OpenCode, los dos que el
  usuario nombró y los dos con integración completa en el instalador actual, son los que
  se usan para verificar la funcionalidad. No son, sin embargo, el límite del soporte:
  Cursor, Windsurf, Cline, Codex y cualquier agente futuro quedan cubiertos por el mismo
  mecanismo de protocolo, y el instalador ya les escribe ese protocolo hoy.
- **Fiabilidad de la activación autónoma**: depende de que el agente siga la instrucción
  del protocolo, no de una garantía del entorno. Es el mismo criterio con el que ya opera
  el protocolo de memoria existente del proyecto, y se acepta conscientemente a cambio de
  la cobertura universal.
- **Reutilización del contexto existente**: el material histórico es el que gomemory ya
  produce hoy para el brazo extensor de spec-kit —incluido el presupuesto de tamaño y el
  resumen rotulado del grafo de código externo—. No se define un formato de contexto
  nuevo para esta funcionalidad.
- **Reutilización del registro de planes**: ya existe captura de planes aprobados como
  decisión del proyecto. Esta funcionalidad se apoya en ella en lugar de crear un
  registro paralelo.
- **Patrón de distribución establecido**: el mecanismo de instalación por proyecto que ya
  distribuye el brazo extensor a Claude Code y OpenCode es el patrón base para el ámbito
  local; el ámbito global sigue el criterio del registro global del servidor de memoria
  ya existente. Se asume que ambos agentes admiten configuración de usuario además de la
  de proyecto, condición necesaria para el ámbito global (a confirmar en `/speckit-plan`).
- **Autoverificación sin componente externo**: la exigencia de atomicidad se cumple
  dentro del propio método, mediante un paso de revisión que el agente ejecuta antes de
  entregar. No se construye un validador que inspeccione el texto del plan desde fuera ni
  que bloquee su aprobación.
- **La verificación de cumplimiento no es parte de esta funcionalidad**: el plan atómico
  deja el contrato registrado, pero contrastar lo entregado contra él corresponde a la
  persona o al flujo SDD existente.
- **Degradación silenciosa como norma**: es el criterio ya adoptado en la integración
  existente y se mantiene aquí — la memoria nunca bloquea el flujo del agente.
- **Idioma**: el método y sus mensajes se redactan en español neutro, consistente con el
  resto de la integración y la preferencia registrada del usuario.
- **Línea base del método**: el usuario aportó una versión ya optimizada del documento ADS
  original, conservada en [`reference-ads-baseline.md`](./reference-ads-baseline.md). Esa
  versión es el punto de partida de `/speckit-plan`, no el documento original. Le falta
  únicamente la parte de memoria (FR-016), la autoverificación (FR-018, FR-019) y la
  invocación autónoma (FR-001); el resto del método ya está resuelto ahí.
- **La rama de ejecución del método no contradice el alcance**: la línea base incluye un
  comportamiento para cuando el método se invoca fuera de modo plan. En modo plan —el
  alcance de esta feature— ordena entregar el árbol y detenerse, que es exactamente lo
  que exigen FR-020 y FR-021.
- **Ámbito de la especificación**: describe qué debe ocurrir al planificar y qué debe
  quedar instalado; no prescribe el mecanismo técnico de detección del modo plan en cada
  agente, que corresponde a la fase de planificación técnica.

## Dependencies

- Historial y contexto de proyecto ya producidos por gomemory (memorias, sesiones,
  relaciones, presupuesto de tamaño).
- Captura de planes aprobados ya existente en la integración con los agentes.
- Mecanismo de instalación y actualización idempotente de artefactos por proyecto ya
  usado por el brazo extensor de spec-kit.
- Configuración por proyecto de gomemory y su interfaz de ajustes, donde vive el
  interruptor de activación.
- Opcional: proveedor externo de grafo de código, cuando esté conectado, para la parte de
  estructura del contexto.

## Out of Scope

- **Ejecución de las tareas atómicas** (fase 3 del método ADS original): marcar avance
  tarea por tarea, conservar resultados intermedios y corregir el rumbo durante la
  ejecución. Permanece en `/speckit-implement`.
- **Integración y entrega final verificada** (fase 4 del método ADS original): consolidar
  los resultados parciales y emitir el informe de objetivo cumplido.
- Un validador externo que inspeccione el plan y bloquee su aprobación si detecta tareas
  no atómicas.
- Sustituir o reescribir el flujo SDD existente (`/speckit-specify`, `/speckit-plan`,
  `/speckit-tasks`, `/speckit-implement`).
- Integraciones dedicadas agente por agente. La cobertura universal se logra por el
  protocolo común; construir un mecanismo propio para cada agente del mercado queda fuera.
- Garantizar la activación por medios del entorno cuando un agente ignore la instrucción
  del protocolo. La activación es autónoma por diseño y depende del cumplimiento del
  agente.
- Seguimiento de progreso de tareas atómicas en una herramienta externa de gestión.
- Métricas o telemetría sobre la calidad de los planes producidos.
