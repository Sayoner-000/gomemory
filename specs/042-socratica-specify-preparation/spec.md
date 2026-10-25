# Feature Specification: Socrática → Spec Kit — preparación guiada y validada

**Feature Branch**: `main` (rama existente; esta invocación no crea una rama)

**Created**: 2026-10-24

**Status**: Draft

**Input**: User description: "Aprovechar gomemory para distribuir Socrática a los agentes y controlar el recorrido desde una idea, necesidad de negocio, historia o mejora técnica hasta una entrada preparada, revisada y aprobada para speckit-specify. El agente conduce la conversación; gomemory comprueba las condiciones de avance."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Preparar una solicitud con ayuda del contexto (Priority: P1)

Como persona que trabaja con un agente, quiero convertir una idea clara o incompleta en una preparación organizada para especificación, aprovechando el contexto disponible y respondiendo solo las preguntas que cambian el resultado.

**Why this priority**: Permite obtener un borrador útil y revisar lo que se sabe antes de comprometerse con la generación de una especificación.

**Independent Test**: Desde un agente, preparar una necesidad de negocio y una mejora técnica, sin iniciar Spec Kit; comprobar que el borrador conserva la intención, distingue hechos y supuestos y permite reconocer lo que falta.

**Acceptance Scenarios**:

1. **Given** una solicitud con contexto relevante disponible, **When** la persona inicia la preparación, **Then** el agente consulta ese contexto antes de preguntar, conserva el original y no pregunta datos ya resueltos de forma fiable.
2. **Given** una solicitud clara y suficiente, **When** se prepara, **Then** puede llegar a revisión sin preguntas y sin inventar datos de negocio.
3. **Given** una idea ambigua, **When** se conduce el flujo, **Then** se hacen como máximo tres preguntas en todo el flujo y se registran como pendientes los datos esenciales que siguen sin resolver.
4. **Given** una necesidad técnica sin usuario final directo, **When** se prepara, **Then** se identifican los actores afectados, el resultado observable y la forma de comprobarlo sin forzar una historia ficticia de consumidor.
5. **Given** una solicitud demasiado grande, **When** se proponen entregas, **Then** cada entrega expresa un resultado completo verificable, conserva sus relaciones con historias y criterios y distingue el alcance elegido del trabajo restante, sin excluirlo silenciosamente.

---

### User Story 2 - Comprobar la preparación antes de revisarla (Priority: P1)

Como persona que solicita una especificación, quiero conocer la etapa actual, qué impide avanzar y qué puedo hacer a continuación, para que un texto convincente pero incompleto no se considere preparado.

**Why this priority**: Es la diferencia central frente al refinamiento textual: completar y relacionar la necesidad antes de solicitar autorización para generar una especificación.

**Independent Test**: Partir de borradores preparados con un objetivo ausente, una historia sin criterio, una referencia inexistente y un pendiente bloqueante; intentar avanzar y comprobar que cada defecto impide declarar la preparación lista y que corregirlo permite volver a comprobarla.

**Acceptance Scenarios**:

1. **Given** un borrador al que le faltan datos necesarios de la etapa, **When** se solicita avanzar, **Then** permanece en una etapa válida, muestra las comprobaciones pendientes y permite completar o editar el borrador.
2. **Given** historias, criterios y entregas propuestos, **When** se comprueba la preparación, **Then** toda historia tiene al menos un criterio verificable, todo criterio refiere a historias existentes y cada entrega se vincula con las historias y criterios que permiten verificarla.
3. **Given** una pregunta sobre un dato bloqueante, **When** se omite la respuesta o el resto de preguntas, **Then** el pendiente sigue bloqueando la preparación y no se presume una respuesta ni una aprobación.
4. **Given** tres preguntas ya formuladas y bloqueos restantes, **When** la persona continúa, **Then** se muestra qué falta y se permite completarlo mediante edición o aportes voluntarios, sin abrir otra ronda de preguntas.
5. **Given** campos completos y relaciones válidas, sin pendientes bloqueantes, **When** se termina la comprobación, **Then** se habilita la revisión y se explica que la preparación comprobada aún necesita confirmación humana de su significado de negocio.

---

### User Story 3 - Revisar y autorizar el contenido exacto (Priority: P1)

Como persona responsable de la necesidad, quiero revisar el original y la entrada final que recibirá Spec Kit, corregirlos o cancelar, y autorizar la generación solo cuando esa entrada represente lo que necesito.

**Why this priority**: La comprobación de estructura y referencias no garantiza que la solicitud sea correcta para el negocio; la decisión de generar corresponde a la persona.

**Independent Test**: Con una preparación válida, mostrar la revisión, modificar el alcance y verificar que se exige volver a comprobar y revisar; después aprobar y comparar la entrada autorizada con la que se entregaría.

**Acceptance Scenarios**:

1. **Given** una preparación comprobada, **When** se abre la revisión, **Then** se presentan el original, la entrada final completa y los supuestos y pendientes no bloqueantes, junto con las acciones «Aprobar y generar especificación», «Editar», «Copiar» y «Cancelar» según las capacidades disponibles.
2. **Given** una revisión abierta, **When** la persona modifica una sección anterior, **Then** las comprobaciones posteriores, la revisión y cualquier autorización anterior dejan de habilitar el traspaso hasta comprobar y aprobar la nueva versión.
3. **Given** una entrada final en revisión, **When** la persona no ha elegido «Aprobar y generar especificación», **Then** no se genera ninguna especificación ni se ejecuta el trabajo descrito por la solicitud.
4. **Given** una preparación en curso, **When** se cancela, **Then** se conserva y devuelve el original, se cierra el flujo cancelado y no se entrega la propuesta a Spec Kit.
5. **Given** una entrada final revisada, **When** la persona aprueba, **Then** la autorización corresponde únicamente a esa versión y al mismo texto que recibirá Spec Kit, sin una nueva reescritura entre revisión y entrega.

---

### User Story 4 - Generar la especificación con una única entrega autorizada (Priority: P1)

Como persona que aprobó la entrada, quiero que el agente la entregue a Spec Kit una sola vez y me indique el resultado real, conservando el trabajo preparado si todavía no puede generar la especificación.

**Why this priority**: Completa el recorrido solicitado y evita perder una preparación o crear especificaciones duplicadas por reintentos.

**Independent Test**: Usar una entrada ya aprobada con el generador disponible, ausente y con una entrega de resultado incierto; repetir las solicitudes de aprobación y entrega y comprobar los artefactos realmente producidos.

**Acceptance Scenarios**:

1. **Given** una entrada aprobada y Spec Kit disponible, **When** el agente confirma el derecho a entregarla, **Then** entrega exactamente el texto aprobado una sola vez y Spec Kit produce su especificación y su checklist.
2. **Given** una entrada preparada o aprobada y Spec Kit ausente, **When** se intenta generar, **Then** se informa qué falta, se conserva la entrada y no se consume su autorización; cuando Spec Kit esté disponible se puede continuar con la misma versión.
3. **Given** una autorización ya consumida, **When** se repiten la aprobación o la confirmación de entrega, incluso simultáneamente, **Then** no se inicia una segunda generación.
4. **Given** una confirmación cuya respuesta se pierde, **When** se reintenta la misma confirmación, **Then** no se concede una segunda ejecución; si el resultado de la entrega queda incierto, se requiere una elección explícita para resolverlo y no se reenvía automáticamente.
5. **Given** una entrega que sigue en curso, **When** se retoma el flujo, **Then** se informa ese estado y se consulta su resultado sin iniciar otra generación.
6. **Given** una especificación generada, **When** se revisan sus artefactos, **Then** conserva el alcance aprobado y permite rastrear sus criterios hasta las historias de la preparación; el agente comunica dónde quedaron la especificación y su checklist.
7. **Given** una autorización consumida y una generación fallida o sin resultado comprobable, **When** se comunica el resultado, **Then** no se afirma que la especificación se generó correctamente y cualquier nuevo intento de entrega requiere una decisión explícita.

---

### User Story 5 - Retomar el trabajo sin perder decisiones ni privacidad (Priority: P2)

Como persona que interrumpe una preparación, quiero consultar y retomar su última etapa válida desde el agente, la línea de comandos o la TUI, conservando mis decisiones y pendientes dentro de las políticas de privacidad de Socrática.

**Why this priority**: Hace viable el recorrido para solicitudes complejas sin exigir que toda la conversación ocurra en una única sesión.

**Independent Test**: Interrumpir un flujo con historias y pendientes, volver a abrirlo y comparar su preparación, revisión y preguntas ya realizadas; repetir con texto protegido y modo privado e inspeccionar lo retenido.

**Acceptance Scenarios**:

1. **Given** una preparación guardada, **When** se retoma desde el mismo proyecto y agente, **Then** se recuperan el original según las reglas de protección vigentes, las decisiones, la preparación, sus relaciones, la revisión vigente, los pendientes y la última etapa válida.
2. **Given** preguntas ya formuladas antes de la interrupción, **When** se retoma o edita el flujo, **Then** el total permitido sigue siendo de tres preguntas para ese flujo.
3. **Given** el nuevo tipo de preparación pendiente, **When** se consulta desde la línea de comandos o la TUI, **Then** se reconoce su modo y se muestran la etapa y lo necesario para retomarla en el agente, sin enviarla por el simple hecho de consultarla.
4. **Given** preparación con texto protegido, **When** se guarda, se revisa en historial o se copia para entregar, **Then** no se incorpora contenido protegido a registros persistentes ni a la entrada entregada.
5. **Given** modo privado, **When** se realiza la preparación, **Then** el recorrido funciona mientras su estado efímero esté disponible y no deja preparación ni revisión en el historial persistente; al perder ese estado no se promete recuperación posterior.
6. **Given** una preparación retenida, **When** se aplica el borrado o la retención vigente de Socrática, **Then** sus nuevos datos y revisión quedan sujetos al mismo alcance de eliminación que el resto del flujo.

---

### User Story 6 - Acceder al mismo recorrido desde los tres agentes (Priority: P2)

Como persona que usa Claude Code, Codex u OpenCode, quiero recibir el nuevo recorrido con la instalación o actualización de gomemory y activarlo mediante la invocación disponible en mi agente, manteniendo los modos actuales de Socrática.

**Why this priority**: Evita mantener habilidades divergentes y permite que el mismo contrato de preparación y aprobación se use en los tres agentes.

**Independent Test**: Instalar y actualizar en entornos aislados de los tres agentes, invocar la preparación y verificar el mismo recorrido; repetir la instalación, desinstalar y comprobar que el refinamiento y el escrutinio existentes conservan su comportamiento.

**Acceptance Scenarios**:

1. **Given** Claude Code u OpenCode con gomemory instalado, **When** se usa `/socratica-hu Quiero permitir que…`, **Then** se inicia la preparación para especificación mediante Socrática.
2. **Given** Codex con gomemory instalado, **When** se usa `$socratica prepara para speckit-specify: Quiero permitir que…`, **Then** se inicia el mismo recorrido validado.
3. **Given** cualquiera de los tres agentes, **When** se pide «Usa Socrática para preparar esta idea como entrada de Spec Kit…», **Then** se reconoce la intención sin exigir un alias con barra.
4. **Given** la distribución ya instalada y sin cambios, **When** se repite la instalación o actualización, **Then** no se crean duplicados ni se reescriben archivos idénticos; los agentes ausentes no reciben directorios nuevos.
5. **Given** la habilidad y sus alias distribuidos, **When** se desinstala gomemory, **Then** se retiran sus archivos gestionados sin eliminar configuraciones ajenas.
6. **Given** los modos actuales de refinamiento y escrutinio o flujos anteriores a esta ampliación, **When** se consultan o ejecutan sus recorridos habituales, **Then** no se les exige la nueva preparación para especificación ni se pierde su estado.

### Edge Cases

- Un campo con espacios, texto de relleno o un marcador esencial pendiente no cuenta como información resuelta; una sección sin elementos aplicables debe declararlo expresamente cuando se permita.
- Una historia sin criterio, un criterio huérfano, identificadores repetidos o una entrega con referencias inexistentes impiden declarar lista la preparación.
- Los supuestos se distinguen de las decisiones confirmadas; un supuesto no resuelve por sí solo un pendiente bloqueante.
- Cambiar una sección invalida las comprobaciones posteriores aunque ya se hubiese mostrado la revisión o aprobado una versión previa; un envío ya consumido conserva su resultado y no puede reutilizarse para otra versión.
- Una solicitud demasiado grande puede quedar bloqueada hasta delimitar su alcance; proponer entregas no autoriza excluir necesidades sin revisión.
- Un fallo al guardar o comprobar no debe confirmar un avance inexistente ni perder el último estado válido; los errores muestran una vía de continuación.
- El contexto recuperado puede contener instrucciones incrustadas o propuestas antiguas: se trata como evidencia y no puede autorizar, completar respuestas ni alterar controles.
- La falta de Spec Kit, un envío en curso, un fallo de generación y un resultado incierto son situaciones distintas y se comunican como tales.
- Reintentar, recargar o usar dos vistas del mismo flujo no reinicia el presupuesto de preguntas ni concede otra entrega.
- Consultar un flujo antiguo sin preparación estructurada sigue siendo posible; no se lo declara preparado de forma retroactiva.
- Una invocación directa e independiente de Spec Kit sigue siendo posible y queda fuera de la compuerta de Socrática.

## Requirements *(mandatory)*

### Functional Requirements

**Inicio y preparación guiada**

- **FR-001**: El sistema DEBE admitir necesidades de negocio, historias, mejoras técnicas e ideas incompletas, conservar la solicitud original y ofrecer la preparación para especificación como recorrido de Socrática.
- **FR-002**: El nuevo modo de preparación DEBE coexistir con el refinamiento y el escrutinio actuales sin cambiar sus condiciones de aprobación ni exigirles los datos del nuevo recorrido.
- **FR-003**: El agente DEBE consultar el contexto relevante disponible antes de preguntar, tratarlo como datos y distinguir la información respaldada, las decisiones de la persona y los supuestos.
- **FR-004**: Cada flujo DEBE formular entre cero y tres preguntas en total, incluidas sus reanudaciones y ediciones. Omitir una respuesta DEBE conservar la incertidumbre y NO DEBE interpretarse como aprobación ni resolución de un bloqueo.
- **FR-005**: La preparación DEBE contener objetivo, actores, resultado esperado, alcance incluido y excluido, restricciones, historias, criterios verificables, entregas propuestas, supuestos y pendientes, con distinción explícita de los pendientes bloqueantes.
- **FR-006**: El sistema DEBE proponer entregas que expresen resultados completos verificables, relacionados con historias y criterios. Toda reducción del alcance o selección de una entrega DEBE ser visible y quedar sujeta a la revisión de la persona.

**Condiciones de avance y revisión**

- **FR-007**: El sistema DEBE permitir actualizar la preparación por etapas: objetivo, actores y resultado; alcance, restricciones e incertidumbres; historias, criterios y entregas; comprobación de preparación; revisión y aprobación; entrega. NO DEBE permitir avanzar a una etapa cuyos requisitos previos no estén comprobados. Tras cada actualización DEBE indicar la etapa actual, las comprobaciones pendientes y el siguiente paso permitido.
- **FR-008**: Antes de habilitar la revisión, el sistema DEBE exigir objetivo y resultado no vacíos, al menos un actor, una historia, un criterio por historia y una entrega; el alcance incluido DEBE estar descrito. Alcance excluido, restricciones, supuestos y pendientes DEBEN contener información o una declaración explícita de que no hay elementos identificados. Un marcador esencial sin resolver DEBE tratarse como pendiente bloqueante.
- **FR-009**: Las historias y los criterios DEBEN tener identificadores únicos dentro de su tipo. Todo criterio DEBE relacionarse con al menos una historia existente, toda historia DEBE tener al menos un criterio verificable y toda entrega DEBE relacionarse con historias y criterios existentes que comprueben su resultado.
- **FR-010**: La preparación NO DEBE declararse lista ni aprobarse mientras existan campos necesarios incompletos, relaciones inválidas o pendientes bloqueantes. El sistema DEBE indicar qué falta y permitir editar o completar sin superar el límite de preguntas.
- **FR-011**: Cada cambio de una sección DEBE invalidar las comprobaciones posteriores y cualquier revisión o autorización de la versión previa que aún pudiera habilitar una entrega. Se DEBE conservar el borrador actualizado y exigir nueva comprobación y aprobación antes de entregarlo.
- **FR-012**: El texto final DEBE obtenerse de la preparación vigente y presentarse íntegro para revisión con el original, los supuestos y los pendientes no bloqueantes. La entrada revisada, aprobada y entregada DEBE ser idéntica para una misma versión.
- **FR-013**: La revisión DEBE explicar que las comprobaciones cubren completitud, relaciones y bloqueos, mientras la persona confirma la fidelidad a su necesidad. DEBE ofrecer «Aprobar y generar especificación», «Editar», «Copiar» y «Cancelar», usando texto seleccionable si no existe capacidad de portapapeles.
- **FR-014**: Solo la aprobación explícita de la versión revisada DEBE autorizar su entrega. Preparar, consultar, retomar, copiar u omitir preguntas NO DEBE generar especificaciones ni ejecutar el trabajo solicitado.
- **FR-015**: Cancelar DEBE conservar el original y cerrar el flujo sin entregar el borrador a Spec Kit.

**Entrega y resultado**

- **FR-016**: Antes de consumir una autorización de entrega, el agente DEBE comprobar que dispone de Spec Kit para iniciar la generación. Si falta, DEBE conservar la preparación y cualquier aprobación vigente y comunicar la condición necesaria para continuar.
- **FR-017**: El traspaso DEBE mantener la secuencia existente de aprobación y confirmación previa a la ejecución. Solo quien obtiene la confirmación habilitante DEBE iniciar la generación; las aprobaciones, confirmaciones y solicitudes de entrega repetidas o simultáneas NO DEBEN iniciar una segunda generación para la misma autorización.
- **FR-018**: Ante una respuesta perdida, el agente DEBE repetir la misma confirmación, sin conceder otra ejecución si ya se consumió. Un envío en curso DEBE consultarse sin duplicarlo; un resultado incierto DEBE requerir una decisión explícita entre marcar como enviado, reenviar o cancelar, respetando los controles existentes.
- **FR-019**: Spec Kit DEBE recibir exclusivamente la entrada aprobada como solicitud de especificación. El resultado esperado DEBE incluir la especificación y su checklist, conservar el alcance aprobado y permitir rastrear sus criterios hasta las historias preparadas.
- **FR-020**: El agente DEBE informar el resultado comprobado y la ubicación de los artefactos producidos, distinguiendo preparación retenida, entrega en curso, generación fallida y generación completada. Una autorización consumida sin generación completada NO DEBE reutilizarse automáticamente.

**Recuperación, privacidad y distribución**

- **FR-021**: La preparación, sus relaciones, las comprobaciones, la revisión, los pendientes y la etapa válida DEBEN conservarse y recuperarse dentro del flujo existente, aislados por proyecto y agente y sujetos a su versión y políticas de retención.
- **FR-022**: La protección de texto y el modo privado vigentes DEBEN cubrir los nuevos campos, la revisión y la entrega. El contenido protegido NO DEBE aparecer en datos persistentes ni en la entrada entregada; el modo privado NO DEBE dejar preparación ni revisión en el historial persistente. Estos contenidos NO DEBEN guardarse como memorias generales del proyecto.
- **FR-023**: El borrado y la retención del flujo DEBEN abarcar también la preparación y su revisión. Un fallo de almacenamiento o comprobación DEBE conservar el último estado válido y NO DEBE presentarse como avance confirmado.
- **FR-024**: La línea de comandos y la TUI DEBEN reconocer el nuevo modo para consultar su etapa y retomar el flujo desde el agente, sin generar una especificación al consultar o dibujar su estado.
- **FR-025**: La instalación y actualización DEBEN distribuir el recorrido desde la habilidad Socrática canónica y mantener coherente su copia publicada y las copias gestionadas de los tres agentes. Las entradas con alias DEBEN remitir a esa habilidad, sin mantener procedimientos independientes.
- **FR-026**: Claude Code y OpenCode DEBEN admitir `/socratica-hu`; Codex DEBE admitir `$socratica prepara para speckit-specify: …`; los tres DEBEN admitir la petición equivalente en lenguaje natural. Las instrucciones del agente DEBEN seguir la etapa, las comprobaciones y el siguiente paso permitido que comunique gomemory.
- **FR-027**: Repetir una instalación sin cambios NO DEBE duplicar ni reescribir archivos idénticos. La distribución NO DEBE crear directorios de agentes ausentes y la desinstalación DEBE retirar los archivos gestionados de la habilidad y sus alias sin borrar configuraciones ajenas.
- **FR-028**: Los flujos existentes DEBEN seguir consultándose y retomándose sin pérdida de datos. La primera versión DEBE centrarse en la preparación y entrega desde agentes; una invocación directa e independiente de Spec Kit DEBE seguir siendo posible.

### Key Entities

- **Flujo de preparación**: recorrido de una solicitud por proyecto y agente; conserva original, modo, etapa válida, preguntas realizadas, versión vigente, preparación, revisión y estado de entrega. Amplía el flujo de Socrática existente.
- **Preparación**: conjunto de objetivo, actores, resultado esperado, alcance incluido y excluido, restricciones, historias, criterios, entregas, supuestos y pendientes que da origen al texto final.
- **Historia preparada**: necesidad identificable de un actor con resultado esperado, vinculada a uno o más criterios verificables. También puede representar una necesidad técnica.
- **Criterio preparado**: condición observable para aceptar una o más historias, con identidad y referencias que permiten rastrearla en la especificación resultante.
- **Entrega propuesta**: resultado completo verificable que agrupa historias y criterios dentro del alcance propuesto para revisión.
- **Pendiente**: información o decisión sin resolver, con indicación de si impide avanzar. Un supuesto no elimina su carácter bloqueante por sí solo.
- **Comprobación de preparación**: resultado de evaluar una versión y una etapa, con faltantes, relaciones inválidas y siguiente paso permitido; queda invalidado si cambian las secciones de las que depende.
- **Revisión y autorización**: texto final de una versión presentado a la persona, su decisión explícita y el derecho a una sola entrega conforme a la confirmación existente.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Las cuatro clases de solicitud de aceptación —clara, ambigua, técnica y demasiado grande— permiten obtener una preparación revisable o una explicación concreta de los bloqueos, sin inventar datos necesarios ni formular más de tres preguntas por flujo.
- **SC-002**: En el 100 % de los casos de campos necesarios ausentes, referencias inválidas y pendientes bloqueantes, no se habilita una aprobación para generar hasta corregirlos; omitir respuestas no cambia este resultado.
- **SC-003**: En el 100 % de los casos de edición posteriores a una comprobación o revisión, la nueva versión exige volver a comprobar y revisar antes de entregarse; la entrada revisada, aprobada y entregada coincide carácter a carácter.
- **SC-004**: En todos los casos de reintento, recarga, confirmación repetida y solicitudes simultáneas, cada autorización inicia como máximo una generación y ningún resultado incierto provoca un reenvío automático.
- **SC-005**: En todos los casos de falta del generador, la persona conserva la preparación y la posibilidad de continuar sin consumir la autorización; en todas las cancelaciones recupera el original íntegro dentro del ámbito permitido por su protección de texto.
- **SC-006**: El recorrido completo se demuestra al menos una vez en cada uno de los tres agentes con una especificación y un checklist realmente generados; en cada resultado se conserva el alcance aprobado y el 100 % de los criterios de aceptación derivados de la preparación puede rastrearse hasta sus historias de origen.
- **SC-007**: En todos los casos de interrupción con persistencia habilitada, retomar conserva decisiones, relaciones, pendientes y revisión vigente; en todos los casos de texto protegido o modo privado se observan cero apariciones del contenido excluido en los registros persistentes y en la entrada entregada.
- **SC-008**: En los tres agentes, instalar dos veces sin cambios produce cero escrituras en la segunda ejecución y desinstalar deja cero archivos gestionados de esta habilidad y sus alias; los modos previos y los flujos anteriores conservan su comportamiento.
- **SC-009**: En cada demostración completa, la persona confirma mediante la acción de aprobación que la entrada revisada representa su necesidad; antes de esa elección explícita se producen cero especificaciones y cero acciones de implementación de la solicitud.

## Assumptions

- Esta funcionalidad amplía [Socrática existente](../039-socratica-prompt-refinement/spec.md). La generación conjunta tras «Aprobar y generar especificación» es una elección explícita adicional del nuevo recorrido; no convierte automáticamente el refinamiento o el escrutinio actuales en generación de especificaciones.
- El vocabulario público solicitado se conserva para la planificación: modo `specify`, actualización `prepare`, siguiente paso `next` y traspaso `approve` → `ack`. Esta especificación define su comportamiento observable; los esquemas, firmas y decisiones técnicas detalladas se definen en `speckit-plan`.
- La arquitectura y las convenciones se rigen por la [constitución vigente](../../.specify/memory/constitution.md). La ampliación no autoriza cambios de arquitectura ni de los mecanismos de privacidad e idempotencia existentes.
- La primera versión conduce la conversación y genera la especificación desde el agente activo. La línea de comandos y la TUI permiten consultar y retomar el estado; no se exige un nuevo generador autónomo desde esas superficies.
- La disponibilidad de Spec Kit se comprueba en el entorno del agente antes de consumir la autorización. Instalarlo automáticamente, cambiarlo o sustituirlo por otro generador queda fuera de alcance.
- En modo persistente se reutilizan el aislamiento, el máximo de flujos pendientes, el historial y la retención existentes. El modo privado no promete retomar una preparación después de perder su estado efímero.
- Los apartados sin elementos pueden declararse vacíos de forma explícita, excepto objetivo, actores, resultado esperado, alcance incluido, historias, criterios y entregas, que deben contener la información mínima definida. Un alcance excluido aún desconocido se declara pendiente cuando afecte la delimitación.
- Ante dudas de negocio que las comprobaciones de estructura no puedan decidir, el agente las expone como supuestos o pendientes para revisión humana; no presenta la completitud como garantía de corrección.
- Resolver de forma explícita un envío incierto puede autorizar un nuevo intento conforme al control existente; la garantía de no duplicación se aplica a cada autorización y no implica ejecución exactamente una vez ante una caída del receptor.
- La invocación independiente de Spec Kit, la implementación de la necesidad de negocio y las decisiones técnicas propias de su planificación quedan fuera de esta preparación.
