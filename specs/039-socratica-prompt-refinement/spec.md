# Feature Specification: Socratica — refinamiento de prompts y escrutinio socrático con aprobación obligatoria

**Feature Branch**: `main` (rama existente; sin creación de rama en esta invocación)

**Created**: 2026-10-15

**Status**: Draft

**Input**: User description: "Crear en gomemory una funcionalidad basada en la mayéutica socrática, disponible en línea de comandos, en los agentes de IA (Claude Code, Codex y OpenCode) y en la TUI. Borrador aportado: 'Feature specification: Socratica for GoMemory' — habilidad portable `socratica` con modos `socratica` (refinamiento de prompt) y `socratic-q` (escrutinio socrático), máximo tres preguntas, revisión lado a lado, aprobación como compuerta absoluta de ejecución, prevención de doble envío, intercepción automática opcional y desactivada por defecto, historial privado por proyecto y cliente con tope de 20, sin proveedor externo obligatorio, Octopus opcional y UI nativa solo donde el anfitrión lo permita."

## Clarifications

### Session 2026-10-15

- Q: Fuera de un agente (línea de comandos o TUI) no hay modelo activo y no se exigen credenciales externas; ¿cómo se generan preguntas y reescritura? → A: Ambas con detección: la línea de comandos y la TUI siempre gestionan estado; si hay un agente compatible instalado localmente, se delega en él la generación en modo no interactivo; si no, se indica usar el refinamiento desde un agente.
- Q: ¿Cuántos flujos pendientes puede haber a la vez? → A: Uno por proyecto y agente; iniciar otro retoma el pendiente (resultado de /speckit-analyze, F2).
- Q: ¿Quién cambia la intercepción automática? → A: Solo la persona, por línea de comandos o TUI; dentro del agente `/socratica on|off` devuelve la orden a ejecutar (F1).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Refinar un prompt de forma explícita desde el agente (Priority: P1)

Como persona que usa gomemory con un agente de IA (Claude Code, Codex u OpenCode), quiero invocar `/socratica <prompt>` (o pedirlo en lenguaje natural donde no existan comandos) para que mi instrucción en bruto se transforme en un prompt fiel y ejecutable, revisarlo junto al original y decidir si se ejecuta, sin que el agente empiece a trabajar en la tarea antes de que yo lo apruebe.

**Why this priority**: Es el valor central de la funcionalidad y funciona en los tres agentes sin depender de ninguna capacidad opcional del anfitrión (paneles, intercepción, portapapeles).

**Independent Test**: En cada uno de los tres agentes, invocar la habilidad con un prompt ambiguo, responder u omitir las preguntas, y verificar que se muestran original y propuesta, que no se ejecuta ningún trabajo de la tarea hasta elegir "Ejecutar", y que "Cancelar" devuelve el texto original sin cambios.

**Acceptance Scenarios**:

1. **Given** un prompt ambiguo, **When** la persona invoca `/socratica <prompt>`, **Then** el sistema conserva el texto original exacto, hace entre cero y tres preguntas relevantes para la decisión, cada una con dos a cuatro opciones cortas más "Otra" y "Omitir", y no ejecuta la tarea.
2. **Given** preguntas en curso, **When** la persona elige "Omitir el resto", **Then** el sistema pasa directamente a la reescritura usando solo las respuestas dadas.
3. **Given** un prompt cuya ambigüedad ya resuelve el contexto disponible, **When** se invoca la habilidad, **Then** el sistema no hace preguntas redundantes y pasa a la reescritura.
4. **Given** una propuesta generada, **When** se muestra la revisión, **Then** aparecen el original y la propuesta, un resumen de palabras antes/después y las adiciones sustantivas, y las acciones Ejecutar, Editar, Copiar y Cancelar.
5. **Given** una propuesta en revisión, **When** la persona elige "Ejecutar", **Then** el prompt aprobado se envía una sola vez para su ejecución, marcado como aprobado por Socratica para que no vuelva a ser interceptado.
6. **Given** una propuesta en revisión, **When** la persona elige "Cancelar", **Then** el borrador no aprobado se descarta y el texto original se devuelve íntegro (al campo de entrada donde el anfitrión lo permita; si no, repetido literalmente en la conversación).
7. **Given** un dato esencial desconocido (por ejemplo un nombre, una métrica o una fecha), **When** se reescribe el prompt, **Then** queda como `[marcador]` explícito en lugar de inventarse.

---

### User Story 2 - Escrutar una propuesta con preguntas socráticas (Priority: P1)

Como persona que va a comprometerse con una implementación o decisión, quiero invocar `/socratic-q <propuesta>` para que se examinen sus premisas, la evidencia disponible, alternativas creíbles, contraejemplos, riesgos y consecuencias verificables, y obtener una síntesis que no presente especulaciones como hechos.

**Why this priority**: Es el segundo modo canónico solicitado y comparte la misma compuerta de aprobación; aporta valor independiente del refinamiento.

**Independent Test**: Invocar el modo con una propuesta técnica y verificar que la salida separa premisas, evidencia, huecos de evidencia, alternativas y criterios verificables, marca lo especulativo como tal y no ejecuta ninguna acción derivada sin aprobación explícita.

**Acceptance Scenarios**:

1. **Given** una propuesta, **When** se invoca `/socratic-q <propuesta>`, **Then** el sistema plantea como máximo tres preguntas de escrutinio y presenta una síntesis estructurada en premisas, evidencia, alternativas, riesgos y criterios de éxito comprobables.
2. **Given** una afirmación no respaldada por evidencia del contexto, **When** aparece en la síntesis, **Then** se rotula como hipótesis y no como hecho.
3. **Given** una síntesis final, **When** la persona no elige "Ejecutar", **Then** no se realiza ninguna acción derivada de la propuesta.
4. **Given** una síntesis o prompt aprobado, **When** la persona elige explícitamente convertirlo en entrada de especificación, **Then** se le ofrece como entrada para `/speckit-specify`, sin ejecutar la generación de la especificación de forma automática.

---

### User Story 3 - Instalar y actualizar la habilidad en todos los agentes (Priority: P1)

Como persona que instala o actualiza gomemory, quiero que la habilidad `socratica` llegue de forma idempotente a Claude Code, Codex y OpenCode por el mismo canal que las demás habilidades de gomemory, y que desaparezca al desinstalar, para no mantener copias manuales que diverjan.

**Why this priority**: Sin distribución, la funcionalidad no está disponible en ningún agente; es requisito previo para las historias 1 y 2 en uso real.

**Independent Test**: Ejecutar la instalación dos veces en un HOME aislado con los tres agentes configurados y comprobar que la habilidad existe una sola vez por agente con contenido idéntico, que la segunda ejecución no reescribe nada, y que la desinstalación no deja rastros.

**Acceptance Scenarios**:

1. **Given** un sistema con los directorios de configuración de los tres agentes, **When** se instala o actualiza gomemory, **Then** cada agente recibe exactamente una copia de la habilidad `socratica`, idéntica a la fuente canónica única.
2. **Given** la habilidad ya instalada y sin cambios, **When** se repite la instalación, **Then** no se reescribe ningún archivo.
3. **Given** un agente cuyo directorio de configuración no existe, **When** se instala, **Then** no se crea ese directorio.
4. **Given** la habilidad instalada, **When** se desinstala gomemory, **Then** la habilidad se elimina de todos los agentes conforme a las convenciones actuales de desinstalación.
5. **Given** un agente que admite comandos con nombre, **When** se instala, **Then** `/socratica` y `/socratic-q` quedan disponibles como comandos; en agentes que no los admiten, la habilidad se activa por lenguaje natural o invocación explícita.

---

### User Story 4 - Gestionar Socratica desde la línea de comandos (Priority: P2)

Como persona que trabaja en terminal, quiero usar `mem socratica` para consultar el historial, activar o desactivar la intercepción automática por agente, ver, retomar o cancelar el flujo pendiente de cada agente, y refinar un prompt sin abrir un agente, con mensajes claros cuando algo falle.

**Why this priority**: Da acceso fuera de los agentes y permite inspeccionar y corregir el estado; depende del almacenamiento de las historias anteriores.

**Independent Test**: Ejecutar los subcomandos de historial, activación/desactivación y estado en un proyecto aislado y verificar salida, persistencia por agente y mensajes de error en lenguaje llano con código de salida distinto de cero.

**Acceptance Scenarios**:

1. **Given** registros finalizados, **When** se ejecuta `mem socratica history`, **Then** se listan como máximo los 20 más recientes por proyecto y agente (FR-028), con opción de filtrar por agente, y seleccionar uno muestra su revisión en texto.
2. **Given** un agente con intercepción verificada, **When** se ejecuta `mem socratica on --client <agente>`, **Then** la preferencia queda persistida solo para ese agente y proyecto, sin alterar la de otros agentes.
3. **Given** un agente sin capacidad de intercepción verificada, **When** se intenta activar, **Then** se informa claramente que no está soportado y no se persiste ninguna activación.
4. **Given** un flujo pendiente, **When** la persona lo consulta o cancela desde la línea de comandos, **Then** se muestra su fase estable y el original se conserva íntegro.
5. **Given** cualquier fallo (proyecto no inicializado, entrada vacía, almacenamiento no disponible), **When** se ejecuta un subcomando, **Then** se devuelve un mensaje en lenguaje llano y un código de salida de error, sin perder texto.
6. **Given** un prompt pasado a `mem socratica refine`, **When** se ejecuta, **Then** si hay un agente compatible instalado localmente, la generación de preguntas y reescritura se delega en él en modo no interactivo (las preguntas se responden en la terminal) y la revisión con aprobación ocurre en la terminal; si no hay ninguno, se informa en lenguaje llano que el refinamiento debe hacerse desde un agente, sin perder el texto.
7. **Given** un prompt aprobado en la línea de comandos, **When** la persona elige "Ejecutar", **Then** el prompt se entrega una sola vez al agente elegido (o se emite por la salida estándar para reutilizarlo) con la misma prevención de doble envío.

---

### User Story 5 - Gestionar Socratica desde la TUI de gomemory (Priority: P2)

Como persona que usa la TUI de `mem`, quiero una pantalla de Socratica para ver el historial, el detalle de una revisión con original y propuesta lado a lado, los flujos pendientes y el estado de la intercepción automática por agente, con la identidad visual definida y sin perder texto en ventanas estrechas.

**Why this priority**: Mejora la inspección y la gestión, pero el flujo esencial ya funciona por agente y por línea de comandos.

**Independent Test**: Abrir la TUI en anchos estrecho, medio y amplio, navegar al historial y al detalle de una revisión, alternar la intercepción de un agente, y verificar que la presentación respeta el contrato visual y que ningún dibujado modifica estado.

**Acceptance Scenarios**:

1. **Given** registros finalizados, **When** se abre la pantalla de Socratica, **Then** se muestra el historial (máximo 20 por proyecto y agente) y el seguimiento de cuatro etapas: "Tu prompt | Preguntas | Reescritura | Revisión".
2. **Given** un registro seleccionado, **When** se abre su detalle, **Then** aparecen las tarjetas de original y propuesta lado a lado (o apiladas en ancho medio), con desplazamiento, la propuesta con contorno dorado y el resumen de palabras y adiciones.
3. **Given** una ventana demasiado estrecha para la revisión, **When** se intenta mostrar, **Then** se muestra un aviso de una línea y el texto original permanece accesible e íntegro.
4. **Given** cualquier pantalla de Socratica, **When** se dibuja, **Then** el dibujado no escribe en almacenamiento, no cambia estado, no llama a modelos ni envía nada.

---

### User Story 6 - Intercepción automática opcional antes del envío (Priority: P3)

Como persona que quiere refinar por defecto, quiero activar la intercepción automática en un agente que lo soporte para que mis prompts normales pasen primero por Socratica, con una forma de saltarla puntualmente y sin capturar mensajes que no son prompts nuevos.

**Why this priority**: Es una comodidad opcional, desactivada por defecto y dependiente de capacidades del anfitrión que no todos los agentes ofrecen.

**Independent Test**: Con la intercepción activada en un agente con ganchos verificados, enviar prompts de cada categoría de exclusión y verificar con pruebas de decisión que solo se captura el prompt de texto plano con el agente en reposo.

**Acceptance Scenarios**:

1. **Given** intercepción activada y agente en reposo, **When** se envía un prompt de texto plano, **Then** se captura y entra al flujo de refinamiento.
2. **Given** intercepción activada, **When** la entrada es un comando con barra, lleva adjuntos o imágenes, el agente está ocupado, es una respuesta de continuación o de baja información, o es un mensaje aprobado o inyectado por Socratica, **Then** pasa sin capturar.
3. **Given** intercepción activada, **When** la entrada empieza por `>`, **Then** pasa sin capturar y se elimina exactamente un `>` inicial antes de reenviarla.
4. **Given** intercepción desactivada (estado por defecto), **When** se envía cualquier prompt, **Then** no se captura nada.

---

### Edge Cases

- **Doble envío**: reintentos, recargas o pulsaciones repetidas de "Ejecutar" nunca envían dos veces el mismo prompt aprobado.
- **Caída tras aprobar**: al reanudar un flujo aprobado sin confirmación de envío, no se reenvía automáticamente; se muestra una revisión recuperable.
- **Recarga a mitad del flujo**: se reanuda la última fase estable; solo se reintentan automáticamente la generación de preguntas y la reescritura.
- **Error del modelo**: se conserva el original y se ofrecen "Reintentar", "Enviar mi original" y "Cancelar"; enviar el original también requiere elección explícita.
- **Fallo al abrir un panel o ventana estrecha**: se restaura el original y se muestra un aviso de una sola línea.
- **Respuesta malformada del modelo** (más de tres preguntas, menos de dos o más de cuatro opciones, formato roto): se repara o rechaza; nunca se muestran controles inválidos.
- **Contexto con instrucciones incrustadas**: el contexto recuperado de gomemory se trata como datos y no puede alterar las reglas de control (aprobación, máximo de preguntas, no ejecución previa).
- **Contenido `<private>`**: nunca se persiste ni aparece en el historial.
- **`/socratica` sin argumento**: solicita el prompt en el campo de entrada si el anfitrión lo permite; si no, lo pide en la conversación.
- **Proyecto sin memoria inicializada**: la habilidad funciona en modo texto sin historial y lo indica, sin fallar.
- **Modo privado/efímero activo**: el flujo funciona completo pero no queda registro en el historial.

## Requirements *(mandatory)*

### Functional Requirements

**Núcleo portable**

- **FR-001**: El sistema MUST proporcionar una única habilidad canónica llamada `socratica`, conforme al formato abierto de Agent Skills, cuyo nombre coincida con su directorio, con una descripción que permita su descubrimiento automático e instrucciones neutrales respecto al proveedor.
- **FR-002**: La habilidad MUST ofrecer dos modos diferenciados —`socratica` (refinamiento de prompt) y `socratic-q` (escrutinio socrático)— que compartan las mismas reglas de preguntas, aprobación y privacidad sin mantener algoritmos contradictorios.
- **FR-003**: Ante una invocación explícita, el sistema MUST conservar el texto original exacto, hacer de cero a tres preguntas relevantes para la decisión, generar una sola propuesta con `[marcadores]` donde falten datos esenciales, mostrar original y propuesta, y esperar aprobación explícita antes de enviar nada a ejecución.
- **FR-004**: Cada pregunta MUST ofrecer de dos a cuatro opciones cortas más "Otra" y "Omitir"; MUST existir "Omitir el resto"; y las preguntas MUST omitirse cuando el contexto ya resuelva la ambigüedad.
- **FR-005**: El sistema MUST ofrecer las acciones Ejecutar, Editar, Copiar y Cancelar en la revisión, y MUST NOT ejecutar ningún trabajo de la tarea durante lectura, preguntas, reescritura o revisión.
- **FR-006**: La reescritura MUST preservar la intención y MUST NOT inventar nombres, métricas, restricciones técnicas, precios, permisos de implementación ni fechas; MUST incluir, cuando aplique, objetivo, audiencia, contrato de salida, restricciones y definición de terminado.
- **FR-007**: El modo `socratic-q` MUST examinar premisas, alternativas creíbles, contraejemplos, huecos de evidencia y consecuencias verificables, y su síntesis MUST rotular como hipótesis toda afirmación no respaldada.
- **FR-008**: La conversión de un resultado aprobado en entrada para `/speckit-specify` MUST ser una elección explícita de la persona; Socratica MUST NOT iniciar la generación de especificaciones por sí misma.
- **FR-009**: La habilidad MUST funcionar sin credenciales de servicios externos, usando el modelo del agente activo; el enrutamiento hacia un modelo más rápido MAY usarse solo si ya está disponible.
- **FR-010**: Toda capacidad del anfitrión (comandos con nombre, panel, portapapeles, intercepción, devolución al campo de entrada) MUST estar declarada por agente y respaldada por evidencia de una prueba ejecutada; una capacidad sin evidencia MUST tratarse como ausente. Si falta, el sistema MUST ofrecer un flujo de texto funcional equivalente y MUST NOT mostrar controles emulados que no puedan funcionar.

**Ciclo de vida y estado**

- **FR-011**: Cada flujo MUST seguir los estados CAPTURADO → PREGUNTAS → REESCRITURA → REVISIÓN → APROBADO → ENVIADO, con los estados laterales FALLIDO y CANCELADO; las transiciones permitidas MUST validarse en un único punto independiente de la presentación.
- **FR-012**: El estado de cada flujo MUST persistirse versionado y aislado por proyecto, agente y flujo, con el original, el modo, las respuestas, la revisión, la última fase estable y la fecha de actualización, sin metadatos sensibles innecesarios.
- **FR-013**: Elegir "Ejecutar" MUST marcar el flujo como aprobado y asignarle un identificador de envío único de forma atómica; el envío MUST rechazar identificadores ya usados y MUST marcar el mensaje con origen "aprobado por Socratica" de forma que la intercepción lo reconozca también cuando el envío ocurre en una sesión de agente nueva lanzada desde la línea de comandos o la TUI. El envío MUST confirmarse solo después de que el agente receptor haya arrancado con éxito; si el arranque falla, el flujo MUST quedar aprobado sin confirmar y recuperable. Dos solicitudes simultáneas de "Ejecutar" sobre el mismo flujo MUST producir un único arranque del agente. Si el agente arrancó pero el envío no pudo confirmarse, o si una reserva de envío quedó sin confirmar, el flujo MUST quedar como envío incierto y MUST NOT reenviarse sin una elección explícita de la persona (marcar como enviado, reenviar o cancelar). Cuando el propio agente activo ejecuta el prompt aprobado, la confirmación MUST ser atómica y preceder a la ejecución: solo quien la obtiene ejecuta.
- **FR-014**: Al reanudar, el sistema MUST volver a la última fase estable y reintentar automáticamente solo la generación de preguntas o reescritura; un flujo aprobado sin confirmación de envío MUST mostrarse como revisión recuperable y MUST NOT reenviarse automáticamente.
- **FR-015**: Cancelar MUST descartar el borrador no aprobado y devolver el original íntegro (al campo de entrada si el anfitrión lo permite, o repetido literalmente en texto).
- **FR-016**: Ante un error del modelo, el sistema MUST conservar el original y ofrecer "Reintentar", "Enviar mi original" y "Cancelar"; enviar el original MUST requerir elección explícita.
- **FR-017**: Todo comando MUST responder de inmediato (con un acuse previo si debe abrir una interfaz de forma asíncrona) y MUST devolver un mensaje en lenguaje llano ante cualquier error.

**Intercepción automática**

- **FR-018**: La intercepción automática MUST estar desactivada por defecto y MUST activarse o desactivarse por agente y proyecto desde la línea de comandos o la TUI, sin alterar la preferencia de otros agentes. Dentro de un agente, `/socratica on|off` MUST responder con la orden exacta de línea de comandos que la persona debe ejecutar y MUST NOT cambiar la preferencia por sí mismo (un modelo no puede activar la captura de los mensajes de la persona).
- **FR-019**: Activar la intercepción en un agente sin ganchos de pre-envío verificados MUST devolver un mensaje claro de "no soportado" y MUST NOT persistir la activación, salvo que la persona pida explícitamente el modo de verificación (necesario para obtener la evidencia de FR-037). En ese caso MUST mostrarse un aviso y el estado MUST figurar como "sin verificar".
- **FR-020**: La intercepción MUST aplicarse solo a texto plano del usuario con el agente en reposo, y MUST dejar pasar comandos con barra, adjuntos, imágenes, turnos en curso, respuestas de continuación, mensajes de baja información (umbral conservador común a todos los agentes, ajustable por agente solo si la evidencia lo justifica), y mensajes aprobados o inyectados por Socratica.
- **FR-021**: Una entrada que empiece por `>` MUST omitir la intercepción. Donde el anfitrión permite modificar el mensaje, MUST eliminarse exactamente un `>` inicial antes de reenviarlo; donde no lo permite, MUST indicarse al agente que trate el mensaje sin ese `>`, y la matriz de compatibilidad MUST distinguir ambos casos (garantizado / por directiva).
- **FR-022**: La invocación explícita MUST funcionar con independencia de que la intercepción esté activada o soportada.

**Integración con gomemory**

- **FR-023**: La habilidad MUST distribuirse desde una única fuente canónica por el canal existente de instalación de habilidades de gomemory, a los ámbitos de usuario de Claude Code, Codex y OpenCode, de forma idempotente, sin crear directorios de agentes ausentes, y MUST eliminarse en la desinstalación.
- **FR-024**: Donde el agente admita comandos con nombre, `/socratica` y `/socratic-q` MUST registrarse como comandos que activan la habilidad, distribuidos por el mismo pipeline y sin duplicar su contenido.
- **FR-025**: El historial y los flujos pendientes MUST almacenarse en un espacio propio dentro del almacenamiento local por proyecto de gomemory, separado de las memorias generales; MUST NOT escribirse contenido dinámico en los directorios de habilidades instaladas.
- **FR-026**: Los prompts en bruto, las respuestas y los historiales MUST NOT guardarse como memorias generales del proyecto por defecto.
- **FR-027**: Todo texto persistido o mostrado MUST respetar `<private>` y las políticas de depuración vigentes; MUST existir un modo privado/efímero que desactive el historial; los pasos automáticos sin captura MUST NOT registrarse.
- **FR-028**: El historial MUST registrarse solo tras completar una reescritura o revisión y MUST conservar los 20 registros más recientes por proyecto y agente, con el límite configurable mediante la configuración existente de gomemory; los flujos pendientes MUST tener un límite propio y purgarse con las políticas de retención vigentes.
- **FR-029**: Cuando haya contexto de gomemory disponible, la habilidad MUST usar contexto acotado a la tarea, tratarlo como datos y no como instrucciones de mayor prioridad, y preferirlo a preguntas redundantes.
- **FR-030**: La integración con Octopus MUST ser opcional, respetar sus controles de enrutamiento y presupuestos, y MUST NOT introducir dependencias de modelos externos nuevos.
- **FR-031**: Gomemory MUST exponer las operaciones de estado (crear/avanzar flujo, aprobar con identificador de envío, confirmar envío, cancelar, consultar historial, leer la preferencia de intercepción —sin cambiarla, ver FR-018—) por su superficie de agente existente solo en la medida necesaria, sin una colección redundante de herramientas.

**Línea de comandos y TUI**

- **FR-032**: La línea de comandos MUST ofrecer `mem socratica` con, al menos: historial (máximo 20, filtrable por agente, con revisión en texto), activar/desactivar intercepción por agente, consultar, retomar y cancelar el flujo pendiente, y modo privado.
- **FR-032a**: `mem socratica refine` MUST detectar agentes compatibles instalados localmente y, si hay alguno, delegar en él la generación de preguntas y reescritura en modo no interactivo, manteniendo en gomemory el estado, la revisión y la compuerta de aprobación; si no hay ninguno, MUST informarlo en lenguaje llano y conservar el texto. La TUI MUST ofrecer la misma capacidad con la misma detección.
- **FR-033**: La TUI de gomemory MUST incluir una pantalla de Socratica con historial, detalle de revisión, flujos pendientes y estado de intercepción por agente.
- **FR-034**: Toda presentación (TUI o panel nativo) MUST ser una proyección pura del estado: MUST NOT escribir en almacenamiento, cambiar estado, llamar a modelos ni enviar nada durante el dibujado.
- **FR-035**: Donde exista presentación enriquecida, MUST seguir el contrato visual: seguimiento de cuatro etapas (completada verde #3DD68C, actual dorado #F2C46D, siguientes atenuadas); texto principal #E6EDF3; etiquetas #7D8590; controles #262B33; avisos #E3B341; título en mayúsculas con segmento inicial subrayado en dorado y continuación horizontal tenue; sin animaciones, desenfoques ni emojis; preguntas numeradas en marcos de trazo grueso de hasta tres filas con marca verde tras responder; opciones como chips compactos con contador de respuestas; acciones de revisión sobre tarjetas lado a lado desplazables, propuesta con contorno dorado y línea resumen de palabras y adiciones.
- **FR-036**: La presentación MUST adaptarse a anchos estrecho y amplio con filas acotadas, elipsis o desplazamiento horizontal, sin ajuste accidental de líneas; por debajo del ancho mínimo MUST restaurar el original con un aviso y nunca perder texto.

**Verificación y documentación**

- **FR-037**: Cada agente (Claude Code, Codex, OpenCode) MUST verificarse por separado y documentarse en una matriz de compatibilidad que indique, con evidencia de pruebas ejecutadas, qué admite: comando con nombre, panel, intercepción, historial en interfaz y restauración del borrador.
- **FR-038**: La documentación MUST explicar on, off, historial, aprobación y limitaciones; demostrar que no se trabaja antes de "Ejecutar" y la prevención de doble envío; e informar de privacidad y del coste adicional en tokens.
- **FR-039**: Solo si se implementa un plugin real de Claude, MUST validarse con las herramientas de validación del cliente instalado tras comprobar que existen, registrando la orden y su salida; MUST NOT afirmarse que esa validación aplica a una habilidad portable.

### Key Entities

- **Flujo de refinamiento**: una ejecución en curso de Socratica. Atributos: identificador, versión de esquema, proyecto, agente, modo (`socratica` = `refine`, `socratic-q` = `scrutiny`), prompt original, preguntas, respuestas, propuesta, síntesis de escrutinio (solo `socratic-q`), fase, revisión, identificador de envío y confirmación (opcionales), último error (opcional), fechas de creación y actualización.
- **Registro de historial**: un refinamiento completado. Atributos: identificador, proyecto, agente, modo, original, prompt aprobado (opcional), adiciones sustantivas, palabras antes y después, fecha de finalización. Máximo 20 por proyecto y agente por defecto.
- **Preferencia de intercepción**: estado activado/desactivado por proyecto y agente, desactivado por defecto. El **modo privado** es una preferencia aparte, por proyecto y común a todos los agentes.
- **Capacidades del anfitrión**: conjunto detectado por agente (comando con nombre, panel, portapapeles, intercepción, devolución al campo de entrada) que decide entre presentación enriquecida y flujo de texto.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En el 100 % de las pruebas de extremo a extremo de los tres agentes, no se ejecuta ningún trabajo de la tarea antes de elegir "Ejecutar".
- **SC-002**: En el 100 % de los escenarios de reintento, recarga, pulsación repetida y pérdida de respuesta, cada autorización de envío se consume exactamente una vez y ningún prompt aprobado se reenvía sin una elección explícita de la persona. «Enviado» significa autorización consumida y entregada a un único receptor; el sistema no afirma que la tarea llegara a ejecutarse. Si el receptor cae después de consumirla, la recuperación es una elección explícita de reenviar, nunca un reenvío automático.
- **SC-003**: En el 100 % de las cancelaciones, fallos del modelo y ventanas estrechas, el texto original se recupera idéntico carácter a carácter.
- **SC-004**: Ningún flujo hace más de tres preguntas, y toda pregunta mostrada tiene entre dos y cuatro opciones más "Otra" y "Omitir".
- **SC-005**: En un conjunto fijo de 5 prompts de referencia con datos esenciales ausentes, ejecutado manualmente en cada agente durante la aceptación, el 100 % de esos datos aparece como `[marcador]` y ninguno se inventa (evidencia registrada; sin llamadas de pago en las pruebas automáticas, SC-010).
- **SC-006**: Instalar dos veces consecutivas produce cero escrituras en la segunda, y desinstalar deja cero rastros de la habilidad en los tres agentes.
- **SC-007**: El historial nunca supera el límite configurado (20 por defecto) por proyecto y agente, y no contiene contenido `<private>` ni entradas de pasos sin captura.
- **SC-008**: La matriz de compatibilidad cubre los tres agentes y cada celda está respaldada por una prueba ejecutada o marcada explícitamente como no soportada con su flujo de texto alternativo verificado.
- **SC-009**: La suite de pruebas existente de gomemory sigue pasando sin regresiones en inicialización del servidor de agentes, sesiones, distribución de habilidades, ganchos existentes y Octopus.
- **SC-010**: Ninguna prueba de la funcionalidad requiere llamadas de pago a modelos.

## Assumptions

- "Agente" y "cliente" son equivalentes en esta especificación y se refieren a Claude Code, Codex y OpenCode, los tres que gomemory ya soporta.
- Los comandos con nombre, la intercepción de pre-envío, los paneles y el portapapeles se consideran capacidades por agente que se verificarán en planificación contra los binarios instalados; la especificación no promete paridad de interfaz nativa entre agentes.
- La "TUI" de esta funcionalidad es la TUI existente de `mem`; los paneles dentro de cada agente son la capa opcional de presentación nativa.
- Por defecto, el historial guarda el original y el prompt aprobado depurados según las políticas de privacidad vigentes; el modo privado/efímero desactiva el registro por completo.
- Delegar en un agente instalado localmente desde la línea de comandos o la TUI reutiliza la configuración y credenciales que ese agente ya tiene; no se añade ninguna credencial ni proveedor nuevo (FR-009).
- El límite de flujos pendientes se fija en planificación, siguiendo las políticas de retención ya existentes.
- Hay como máximo un flujo pendiente por proyecto y agente: iniciar uno nuevo con otro pendiente retoma el existente, salvo que la persona lo cancele primero.
- La habilidad responde en el idioma de la persona; su texto canónico sigue la convención de idioma de las demás habilidades de gomemory.
- El borrador "Prompt Upgrade" aportado por la persona es referencia de comportamiento; no se copian APIs específicas de un cliente sin verificar sus contratos.
