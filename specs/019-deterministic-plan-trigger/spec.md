# Feature Specification: Activación determinista del modo plan atómico

**Feature Branch**: `019-deterministic-plan-trigger`

**Created**: 2026-08-17

**Status**: Draft

**Input**: User description: revisión de la activación del modo plan atómico en v2.6.0 — el mecanismo responde y los tests pasan, pero la activación es probabilística: el disparador solo viaja en el primer prompt de la sesión en Claude Code, no hay señal en la ENTRADA al modo plan, la cobertura entre agentes es asimétrica (OpenCode lo recibe cada turno vía su plugin), el archivo de instrucciones de nivel usuario sigue en protocolo v4 sin la sección de modo plan, y las directivas del grafo de código externo compiten con el disparador. Además, actualizar el bloque de protocolo trunca el contenido posterior al bloque. Revisión posterior de estrategia: conseguir el determinismo **sin dañar el brazo extensor de codebase**, sustituyendo la competencia de directivas por composición de roles y trasladando el determinismo al borde de salida del plan.

## Contexto y Problema

La feature 013 entregó el modo plan atómico: al entrar en modo plan, el agente debía invocar la
memoria por su propia iniciativa y devolver un árbol de tareas atómicas verificables. La decisión
de diseño fue deliberada: **la activación la dirige el protocolo del proyecto, no un mecanismo
propio de un agente concreto**, para que cualquier agente presente o futuro quede cubierto.

La auditoría de v2.6.0 confirma que las piezas están intactas —la capacidad responde, el texto
está instalado, la batería de pruebas pasa— y a la vez que la promesa **no se cumple en la
práctica**, por cinco huecos verificados:

1. **El disparador caduca dentro de la sesión.** En Claude Code el recordatorio "al entrar en modo
   plan, carga el contexto de planificación" viaja una sola vez, en el primer prompt. A partir del
   segundo, el único mensaje que llega al agente es el recordatorio de guardado. Como el modo plan
   se entra casi siempre a mitad de sesión, la instrucción ya quedó decenas de miles de tokens
   atrás cuando hace falta.
2. **No hay señal en la entrada, solo en la salida.** Lo único registrado dispara cuando el plan
   ya está redactado y aprobado: sirve para **capturar** el plan, no para **activar** el método.
3. **La cobertura es asimétrica entre agentes.** Un agente reinyecta el disparador en cada turno;
   otro lo recibe una vez por sesión. Ambos "reciben el texto", pero no con la misma persistencia,
   así que la experiencia depende del agente y eso contradice el objetivo agnóstico.
4. **El canal de nivel usuario está desactualizado.** El archivo de instrucciones global de un
   agente sigue en una versión anterior del protocolo, sin la sección de modo plan. La cobertura
   amplia que hoy se observa es un efecto colateral de que el directorio personal de la persona
   quedó instalado como si fuera un proyecto, no del canal de nivel usuario. Un proyecto nuevo, por
   tanto, no queda cubierto de verdad.
5. **Dos directivas puestas a competir por el mismo instante.** El brazo extensor del grafo de
   código pide, sin condiciones, usar sus herramientas para cualquier exploración. El texto propio
   de gomemory refuerza esa misma orden reclamando explícitamente "cualquier tarea: chat, plan,
   resumen", y a continuación pide planificar de forma atómica en una frase condicional al final de
   un bloque largo. En modo plan el agente sale a explorar el grafo y el árbol atómico nunca
   aparece.

Como hallazgo adicional del mismo recorrido: **actualizar el bloque de protocolo a una versión
nueva descarta todo el contenido que la persona tenga después del bloque**. Eso convierte el
arreglo del punto 4 en una operación hoy insegura, así que entra en el alcance de esta feature.

### Encuadre corregido: composición en vez de competencia

La lectura inicial del punto 5 —"el disparador de plan pierde, hay que subirle el volumen"— es un
error de diseño y esta especificación lo descarta de forma explícita. Subir el volumen abre una
guerra de imperativos donde la única forma de ganar es debilitar al brazo extensor: exactamente el
resultado que no se quiere. Y no hay conflicto real que resolver, porque **las dos directivas
responden preguntas distintas**:

- el brazo extensor define **con qué instrumento se averigua qué hace el código**;
- el modo plan atómico define **qué forma debe tener el resultado entregado**.

Son ortogonales y componibles: durante un plan, el grafo es precisamente la herramienta correcta
para explorar, y el árbol atómico es la forma obligatoria de la salida. Compiten hoy solo porque el
texto propio de gomemory las enuncia como dos órdenes rivales para el mismo momento. La corrección
vive en ese texto propio, no en el canal del extensor: emitir **una instrucción compuesta y
secuenciada** ("explora con el grafo; entrega el árbol atómico") en lugar de dos mandatos que se
pisan. El brazo extensor sale reforzado del cambio, no debilitado: queda nombrado como el
instrumento del paso de exploración del plan.

### Dónde vive el determinismo

La entrada al modo plan depende de una capacidad que hoy **no está verificada** (que el canal de
entrada del agente admita devolver contenido al modelo). El borde de **salida** no: en el momento
en que el agente va a presentar el plan existe una señal observable y un mecanismo documentado para
devolverle una decisión con su motivo. Por eso esta especificación coloca el determinismo ahí: un
plan que no cumple el contrato de forma **no llega a la persona**, se devuelve para rehacerlo. Eso
sustituye "esperar que el agente recuerde una instrucción" por "el sistema no deja pasar lo que no
cumple", y no requiere ninguna apuesta sobre capacidades no comprobadas.

La inyección en la entrada sigue en el alcance, pero como **mejor esfuerzo**: aporta calidad
(historial del proyecto disponible antes de redactar), no el determinismo.

## Invariantes de convivencia con el brazo extensor *(no negociables)*

Estos invariantes acotan la solución antes de enunciar requisitos. Cualquier propuesta que los
viole queda descartada aunque mejore el determinismo:

- **INV-1**: gomemory administra únicamente su propio texto y sus propias entradas de
  configuración. No altera, silencia, reordena, reescribe ni condiciona los mensajes ni los canales
  del brazo extensor.
- **INV-2**: la instalación y la reinstalación preservan íntegras las entradas de configuración de
  terceros, y no duplican las propias al repetirse.
- **INV-3**: gomemory no interfiere con las restricciones que el brazo extensor imponga sobre
  herramientas de exploración; en particular, no añade restricciones propias sobre las mismas
  herramientas.
- **INV-4**: si el brazo extensor no está presente, nada de esta feature cambia de comportamiento
  ni emite avisos por su ausencia.
- **INV-5**: el refuerzo del modo plan nunca se expresa como una degradación del papel del grafo de
  código. Ambas capacidades se enuncian como complementarias y secuenciadas.
- **INV-6**: **ningún agente es el agente de referencia.** La capacidad se expresa como un contrato
  neutral que cualquier agente puede implementar, y los formatos propios de cada agente son
  traducciones de ese contrato, nunca su definición. Conectar un agente nuevo no debe exigir cambios
  en la lógica de gomemory.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Un plan que no cumple el contrato no llega a la persona (Priority: P1)

Una persona pide un cambio no trivial en modo plan. Si el agente redacta prosa o una lista de
intenciones gruesas, el sistema se lo devuelve con el motivo antes de que la persona lo vea, y el
agente entrega el árbol de tareas atómicas verificables. La persona nunca recibe un plan que no
cumple la forma acordada.

**Why this priority**: Es el único eslabón que convierte la promesa en determinismo real, y se
apoya en un mecanismo observable en el momento en que el plan se presenta. Entregada sola, ya
garantiza el resultado que la persona echa en falta.

**Independent Test**: Pedir en modo plan un cambio no trivial forzando una respuesta en prosa. Se
verifica que la persona no recibe esa prosa, que el agente recibe el motivo y que el plan
finalmente presentado es un árbol atómico.

**Acceptance Scenarios**:

1. **Given** una solicitud no trivial en modo plan, **When** el agente intenta presentar un plan
   que no es un árbol de hojas verificables, **Then** el plan se devuelve con el motivo y no llega
   a la persona.
2. **Given** el mismo episodio, **When** el plan ya se devolvió una vez, **Then** el sistema no
   vuelve a devolverlo: la persona nunca queda bloqueada dos veces por el mismo plan.
3. **Given** una solicitud trivial de un solo paso, **When** el agente presenta un plan sin árbol,
   **Then** el sistema lo deja pasar: el método ya excluye descomponer lo trivial.
4. **Given** cualquier duda sobre si el plan cumple la forma, **When** el sistema evalúa,
   **Then** deja pasar en silencio; el criterio se aplica solo a incumplimientos claros.
5. **Given** la exigencia apagada en la configuración, **When** se presenta cualquier plan,
   **Then** no hay devolución ni mensajes.

---

### User Story 2 - Recibir el método y el historial al entrar en modo plan, en cualquier momento de la sesión (Priority: P1)

Una persona lleva media sesión de trabajo y entonces entra en modo plan. Sin escribir nada
adicional, el agente dispone del método de descomposición atómica y del historial del proyecto
**antes** de empezar a redactar, así que el plan se apoya en decisiones y causas raíz ya registradas
en lugar de reinventarlas.

**Why this priority**: Es la calidad del plan, no solo su forma. La Historia 1 garantiza el árbol;
esta garantiza que el árbol esté informado. Es entregable de forma independiente y es el hueco
concreto que la persona reportó.

**Independent Test**: Gastar tres o más turnos en cualquier tarea y solo entonces entrar en modo
plan pidiendo un cambio no trivial. Se verifica que el plan referencia al menos un elemento del
historial que la persona no mencionó.

**Acceptance Scenarios**:

1. **Given** una sesión con varios turnos ya gastados, **When** la persona entra en modo plan,
   **Then** el agente dispone del método y del historial antes de redactar, sin intervención manual.
2. **Given** una sesión reanudada o compactada, **When** la persona entra en modo plan, **Then** la
   activación ocurre igual que en una sesión nueva.
3. **Given** un agente que no expone señal observable de entrada al modo plan, **When** la persona
   entra en modo plan, **Then** cuenta con el disparador porque lo recibió en el turno en curso.
4. **Given** un plan en curso, **When** el agente explora el código para fundamentarlo, **Then**
   usa las herramientas del grafo y eso no compite con la obligación de entregar el árbol atómico:
   el material explorado alimenta las hojas del árbol.

---

### User Story 3 - Cualquier agente, presente o futuro, con las diferencias declaradas (Priority: P2)

La persona trabaja con más de un agente sobre el mismo proyecto y en todos observa el mismo
comportamiento al entrar en modo plan. Quien quiera conectar un agente que gomemory todavía no
conoce puede hacerlo leyendo un contrato publicado, sin esperar una versión nueva ni tocar código de
gomemory. Y donde un agente no pueda ofrecer alguna de las garantías, el sistema lo dice en su
reporte de estado en lugar de aparentar cobertura completa.

**Why this priority**: El valor prometido es "cualquier agente", incluidos los que aún no existen.
Si la garantía se define en el formato de un agente concreto, el resto queda de segunda clase por
construcción — y eso es justamente la asimetría que esta feature venía a corregir. Necesita que las
Historias 1 y 2 definan qué se iguala.

**Independent Test**: Entrar en modo plan a mitad de sesión en dos agentes distintos con el mismo
proyecto y comparar resultados. Por separado, y sin modificar gomemory, conectar un tercer agente
—o un script que lo imite— siguiendo únicamente el contrato publicado, y comprobar que obtiene la
misma garantía.

**Acceptance Scenarios**:

1. **Given** dos agentes soportados sobre el mismo proyecto, **When** en ambos se entra en modo
   plan a mitad de sesión, **Then** los dos entregan un plan atómico apoyado en el historial.
2. **Given** un integrador con un agente que gomemory no conoce, **When** implementa el contrato
   publicado (invocar el comando en el momento acordado e interpretar la decisión), **Then** obtiene
   la garantía completa sin cambios en gomemory.
3. **Given** un agente que no declara ningún dialecto propio, **When** invoca la capacidad,
   **Then** recibe la decisión en el formato neutral, que no pertenece a ningún agente en concreto.
4. **Given** un agente donde alguna garantía no está disponible, **When** la persona consulta el
   reporte de estado, **Then** el reporte declara esa degradación y qué canal cubre el hueco.
5. **Given** cualquier agente soportado, **When** el sistema emite contenido hacia el agente,
   **Then** cabe dentro del límite de tamaño de ese canal y no se trunca a mitad de frase.

---

### User Story 4 - Habilitar una vez y quedar cubierto en todos los proyectos (Priority: P2)

La persona habilita la planificación atómica a nivel de usuario una sola vez. A partir de ahí, un
proyecto **nuevo**, sin instalación propia, entra en modo plan y recibe el método atómico. Y al
actualizar el bloque de protocolo, nada de lo que la persona escribió a mano se pierde.

**Why this priority**: Es la promesa de "habilitar una vez cubre todos los proyectos" que hoy no se
cumple: lo que da cobertura es un accidente de instalación. No se puede arreglar sin antes hacer
segura la actualización del bloque, porque hoy destruye contenido.

**Independent Test**: En un directorio temporal sin instalación de gomemory, entrar en modo plan y
verificar que el método se aplica. Por separado, tomar un archivo de instrucciones con contenido
propio **antes y después** de un bloque de versión anterior, actualizarlo y verificar que ambos
textos siguen íntegros.

**Acceptance Scenarios**:

1. **Given** la funcionalidad habilitada a nivel usuario, **When** se trabaja en un proyecto que
   nunca instaló gomemory, **Then** el modo plan aplica el método atómico.
2. **Given** un archivo de instrucciones con una versión anterior del bloque y texto propio después
   de él, **When** se actualiza a la versión vigente, **Then** el texto propio anterior y posterior
   se conserva íntegro y no queda ningún resto del bloque viejo.
3. **Given** un agente cuyo directorio de configuración de usuario no existe, **When** se habilita a
   nivel usuario, **Then** ese agente se omite sin crear directorios ni emitir error.
4. **Given** un proyecto con su propia instalación, **When** coexiste con la de nivel usuario,
   **Then** no aparece el bloque dos veces ni instrucciones contradictorias.

---

### User Story 5 - Detectar por adelantado que algo se rompió, en cualquiera de los dos brazos (Priority: P3)

Antes de publicar, quien mantiene el proyecto ejecuta una verificación que reporta, por agente y
por canal, el estado del disparador de modo plan **y** que el brazo extensor sigue activándose
igual que antes del cambio. La verificación falla cuando un canal falta, quedó desactualizado, se
duplicó o cuando la activación del brazo extensor cambió.

**Why this priority**: Es la red de seguridad. Su ausencia explica que el hueco viviera varias
versiones con la batería de pruebas en verde. Cubre además el riesgo propio de esta feature: tocar
la configuración compartida sin darse cuenta de que se dañó al vecino.

**Independent Test**: Ejecutar la verificación en el repositorio: pasa. Degradar a mano un canal, o
retirar la activación del brazo extensor, y volver a ejecutarla: falla señalando el brazo, el
agente y el canal exactos.

**Acceptance Scenarios**:

1. **Given** una instalación al día, **When** se ejecuta la verificación, **Then** reporta todos los
   canales presentes en ambos brazos y termina con éxito.
2. **Given** un canal con la versión del protocolo desactualizada, **When** se ejecuta la
   verificación, **Then** falla e identifica el agente, el canal y la versión encontrada.
3. **Given** dos instalaciones consecutivas, **When** se ejecuta la verificación, **Then** no
   reporta entradas duplicadas.
4. **Given** que la activación del brazo extensor dejó de producirse, **When** se ejecuta la
   verificación, **Then** falla señalando ese brazo, aunque el modo plan funcione.
5. **Given** un agente no instalado en la máquina, **When** se ejecuta la verificación, **Then** lo
   reporta como no aplicable sin contarlo como fallo.

---

### Edge Cases

- **El canal de entrada no admite devolver contenido al agente**: la Historia 2 degrada al refuerzo
  por turno y lo declara en el reporte de estado. El determinismo no se pierde: lo sostiene la
  Historia 1, que no depende de ese canal.
- **El texto del plan no está disponible en el momento de presentarlo**: la exigencia de forma no se
  aplica; se deja pasar en silencio en lugar de bloquear a ciegas.
- **La evaluación de la forma es dudosa** (plan mixto, árbol parcial, formato inusual): prevalece
  dejar pasar. Un falso bloqueo cuesta más que un plan mediocre.
- **La persona rechaza el plan devuelto o abandona el modo plan**: no queda ningún estado que
  bloquee turnos posteriores.
- **La persona entra y sale del modo plan varias veces en una sesión**: el disparador no reinyecta
  el bloque completo cada vez, y la exigencia de forma cuenta una sola devolución por episodio.
- **El contexto de planificación excede el límite de tamaño del canal**: prevalece el método
  completo; el historial se recorta al presupuesto y se indica cómo recuperar el resto. Nunca un
  mensaje cortado a mitad de frase.
- **Un agente que gomemory no conoce invoca la capacidad**: se le responde en el formato neutral, no
  se le rechaza por no estar en ninguna lista.
- **Un agente sin ningún sistema de enganches**: recibe el piso textual completo y su imposibilidad
  de ofrecer garantías deterministas queda declarada, no disimulada.
- **El brazo extensor no está instalado**: todo funciona igual, sin avisos sobre su ausencia.
- **El brazo extensor cambia su propia configuración**: gomemory no la reconcilia ni la corrige;
  solo verifica y reporta.
- **Proyecto sin memoria inicializada, o integración apagada**: modo plan normal, sin error, sin
  ruido y sin bloquear el turno.
- **El directorio personal de la persona quedó instalado como proyecto**: la cobertura debe seguir
  cumpliéndose si se deshace esa instalación accidental.

## Requirements *(mandatory)*

### Functional Requirements

**Determinismo de la forma del plan (borde de salida)**

- **FR-001**: Cuando el agente vaya a presentar un plan para una solicitud no trivial, el sistema
  DEBE evaluar si cumple el contrato de forma —árbol de tareas con resultado verificable por hoja— y
  DEBE devolverlo con el motivo cuando claramente no lo cumple, antes de que llegue a la persona.
- **FR-002**: La devolución DEBE ocurrir como máximo una vez por episodio de plan.
- **FR-003**: La exigencia NO DEBE aplicarse a solicitudes triviales de un solo paso, ni cuando el
  texto del plan no esté disponible, ni cuando la evaluación sea dudosa: en esos casos DEBE dejar
  pasar en silencio.
- **FR-004**: La exigencia DEBE poder apagarse desde la configuración, y apagada NO DEBE producir
  devoluciones ni mensajes.

**Disponibilidad del método y del historial (borde de entrada)**

- **FR-005**: Al entrar en modo plan, el sistema DEBE poner a disposición del agente el método de
  descomposición atómica y el historial del proyecto antes de que redacte, sin intervención manual,
  **en cualquier punto de la sesión** y no solo en su primer turno.
- **FR-006**: Donde el agente exponga una señal observable de entrada al modo plan, la activación
  DEBE apoyarse en esa señal; donde no exista, el sistema DEBE emitir el disparador en cada turno,
  de forma que su vigencia no dependa de la antigüedad de la sesión.
- **FR-007**: El contenido que el sistema emita hacia el agente DEBE respetar el límite de tamaño
  del canal, priorizando el método sobre el historial, sin truncamiento a mitad de frase y dejando
  indicado cómo recuperar el material omitido.
- **FR-008**: La activación DEBE ser idempotente por episodio de plan: reentrar en modo plan dentro
  de la misma sesión no repite el bloque completo ni satura la conversación.
- **FR-009**: Con el proyecto sin memoria inicializada o con la integración apagada, la activación
  DEBE degradar en silencio: modo plan normal, sin error y sin bloquear el turno.

**Convivencia con el brazo extensor**

- **FR-010**: El sistema DEBE enunciar la exploración con el grafo de código y la entrega del árbol
  atómico como pasos complementarios y secuenciados de una misma instrucción, y NO DEBE presentarlos
  como mandatos rivales ni condicionar el cumplimiento de uno al del otro.
- **FR-011**: El sistema NO DEBE alterar, silenciar, reordenar ni reescribir los mensajes ni los
  canales del brazo extensor, ni añadir restricciones propias sobre las herramientas de exploración
  que este gestiona (INV-1, INV-3).
- **FR-012**: Instalar o reinstalar DEBE preservar íntegras las entradas de configuración de
  terceros y NO DEBE duplicar las propias al repetirse, incluidas las que esta feature añada
  (INV-2).
- **FR-013**: Con el brazo extensor ausente, el comportamiento DEBE ser idéntico y NO DEBE emitirse
  ningún aviso por su ausencia (INV-4).

**Agnosticismo de agente**

- **FR-A1**: La capacidad DEBE exponerse como un **contrato neutral** —un comando invocable, un
  momento de invocación y un formato de decisión— que no pertenezca a ningún agente en concreto.
  Los formatos propios de cada agente DEBEN ser traducciones de ese contrato.
- **FR-A2**: El sistema DEBE aceptar la solicitud y emitir la decisión en el **dialecto** que el
  agente entienda, seleccionándolo a partir de lo que el propio agente envía y permitiendo indicarlo
  de forma explícita. Cuando no se pueda determinar, DEBE usar el formato neutral.
- **FR-A3**: Conectar un agente que gomemory no conoce NO DEBE requerir cambios en su lógica: basta
  con que ese agente invoque el comando en el momento acordado e interprete la decisión. El contrato
  DEBE estar publicado con el detalle suficiente para implementarlo sin leer el código.
- **FR-A4**: Las capacidades por agente (qué canales soporta, en qué ámbitos, con qué dialecto)
  DEBEN declararse en **un solo lugar**, de forma que añadir un agente sea añadir una entrada y no
  editar varios sitios. El reporte de estado y la verificación DEBEN alimentarse de esa declaración.
- **FR-A5**: Todo agente DEBE recibir, como mínimo, el piso textual (protocolo + recordatorio por
  turno + envoltorio nativo cuando su formato lo permita), aunque no pueda ofrecer ninguna de las
  garantías deterministas.

**Cobertura de nivel usuario**

- **FR-014**: Al habilitar la funcionalidad a nivel de usuario, el archivo de instrucciones de
  usuario de cada agente que soporte ese ámbito DEBE quedar en la versión vigente del protocolo,
  incluida la sección de modo plan.
- **FR-015**: Actualizar el bloque de protocolo DEBE preservar íntegro todo el contenido propio de
  la persona, tanto el anterior como el **posterior** al bloque, y NO DEBE dejar restos del bloque
  anterior.
- **FR-016**: La cobertura de nivel usuario NO DEBE depender de que el directorio personal de la
  persona esté instalado como si fuera un proyecto.

**Observabilidad y regresión**

- **FR-017**: El sistema DEBE ofrecer un reporte de estado que indique, por agente y por canal, qué
  garantías del modo plan están activas y en qué versión del protocolo, declarando explícitamente
  las degradaciones en lugar de aparentar cobertura completa.
- **FR-018**: DEBE existir una verificación ejecutable que falle cuando un canal del modo plan
  falte, esté desactualizado o duplicado, **y** cuando la activación del brazo extensor deje de
  producirse, reportando en cada caso el brazo, el agente y el canal afectados.
- **FR-019**: La verificación DEBE distinguir "agente no instalado en esta máquina" (no aplicable)
  de "agente instalado con el canal roto" (fallo).

### Key Entities

- **Contrato de forma del plan**: condición que un plan debe cumplir para llegar a la persona —árbol
  de tareas con resultado verificable por hoja—, con excepción declarada para solicitudes triviales.
- **Episodio de plan**: intervalo entre la entrada y la salida del modo plan. Unidad respecto a la
  cual son idempotentes tanto la activación como la devolución por incumplimiento.
- **Canal de activación**: vía por la que una garantía del modo plan llega a un agente concreto.
  Tiene ámbito (proyecto o usuario), momento de emisión (entrada al plan, presentación del plan,
  cada turno) y estado (presente y al día / desactualizado / duplicado / ausente / no aplicable).
- **Brazo extensor**: proveedor externo de grafo de código con sus propios canales y mensajes.
  gomemory lo observa y lo nombra; nunca lo administra.
- **Bloque de protocolo versionado**: fragmento delimitado y con marca de versión que el sistema
  administra dentro de un archivo cuyo resto pertenece a la persona.
- **Reporte de cobertura**: resultado legible que cruza agentes con canales de ambos brazos y
  declara el estado de cada combinación, incluidas las degradaciones conocidas.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% de los planes presentados para solicitudes no triviales llegan a la persona como
  árbol de tareas atómicas verificables.
- **SC-002**: 0 casos en que la persona quede bloqueada más de una vez por el mismo plan, y 0
  devoluciones sobre solicitudes triviales.
- **SC-003**: En 5 de 5 intentos, entrar en modo plan después de al menos tres turnos de sesión
  produce un plan que referencia al menos un elemento del historial del proyecto que la persona no
  mencionó.
- **SC-004**: 0 regresiones en la activación del brazo extensor, medidas con la misma verificación
  antes y después del cambio.
- **SC-005**: 0 entradas de configuración de terceros perdidas y 0 entradas propias duplicadas tras
  dos instalaciones consecutivas.
- **SC-006**: Todos los agentes soportados presentan el mismo comportamiento observable al entrar en
  modo plan, o su degradación queda declarada en el reporte de estado: 0 casos de cobertura aparente
  no declarada.
- **SC-A1**: Un agente que gomemory no conoce obtiene la garantía completa implementando solo el
  contrato publicado, con **0 líneas de código de gomemory modificadas** (demostrado con un cliente
  de prueba que imita a un agente nuevo).
- **SC-A2**: Añadir un agente a la declaración de capacidades exige tocar **un solo lugar**, y el
  reporte de estado y la verificación lo recogen sin cambios adicionales.
- **SC-A3**: 100% de los agentes declarados reciben el piso textual, incluidos los que no soportan
  ninguna garantía determinista.
- **SC-007**: Un proyecto recién creado, sin instalación propia, aplica el método atómico en modo
  plan tras haber habilitado la funcionalidad una sola vez a nivel usuario.
- **SC-008**: 0 caracteres de contenido propio de la persona se pierden al actualizar el bloque de
  protocolo, verificado con contenido situado antes y después del bloque.
- **SC-009**: 0 mensajes emitidos hacia el agente exceden el límite del canal o llegan cortados a
  mitad de frase.
- **SC-010**: La verificación detecta un canal roto —de cualquiera de los dos brazos— en menos de un
  minuto de ejecución y antes de publicar, con 0 falsos fallos por agentes no instalados.
- **SC-011**: Con la integración apagada o sin memoria inicializada, 0 errores y 0 mensajes de fallo
  visibles al entrar en modo plan.

## Assumptions

- **La decisión de diseño de la feature 013 se conserva**: el protocolo textual sigue siendo la
  cobertura universal. Esta feature añade garantías encima donde el agente lo permita; no sustituye
  el enfoque agnóstico ni escribe una integración por agente para cada capacidad.
- **El determinismo se ancla en el borde de salida porque ahí el mecanismo está documentado**
  (existe una decisión con motivo devuelta al agente en el momento de presentar el plan) mientras
  que en la entrada la capacidad de devolver contenido **no está verificada**. Ese punto se
  comprueba en vivo durante la planificación; si la entrada no lo admite, la Historia 2 se cumple
  por refuerzo por turno y ninguna otra historia se ve afectada.
- **La evaluación del contrato de forma es heurística y sesgada a permitir**: se prefiere dejar
  pasar un plan mediocre antes que bloquear uno válido. El criterio se afina con el uso, no se
  presume perfecto desde el primer día.
- **El límite de tamaño del canal es una restricción real** (los mensajes hacia el agente se
  recortan al llegar a un tope de caracteres), por eso FR-007 fija la prioridad método > historial
  en vez de asumir que todo cabe.
- **El refuerzo por turno debe ser barato**: una línea corta, no el bloque completo, para no gastar
  contexto en cada turno de una sesión larga.
- **La fusión de configuración ya preserva entradas ajenas** —verificado en el código actual— y los
  canales de ambos brazos coexisten sin pisarse; el riesgo real al añadir canales nuevos no es la
  colisión sino la **duplicación** de los propios al reinstalar, por eso FR-012 lo exige de forma
  explícita y FR-018 lo verifica.
- **Los agentes sin ámbito de usuario** quedan cubiertos por el ámbito de proyecto, como hoy: no
  pierden la funcionalidad, solo el "habilitar una sola vez".
- **La corrección del borrado de contenido al subir de versión entra en el alcance** aunque se
  detectó como hallazgo aparte, porque sin ella no es seguro actualizar el canal de nivel usuario
  que exige la Historia 4.
- **La instalación accidental del directorio personal como proyecto no se toma como cobertura
  válida**; la feature no obliga a deshacerla, pero tampoco puede depender de ella.
- **El proyecto ya cuenta con un script de regresión por canales** (feature 014) como punto de
  partida natural para FR-018; no se asume un mecanismo de verificación nuevo desde cero.
