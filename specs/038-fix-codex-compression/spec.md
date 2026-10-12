# Feature Specification: Compresión compatible según capacidades del agente

**Feature Branch**: `main` (rama existente; sin creación de rama en esta invocación)

**Created**: 2026-10-12

**Status**: Draft

**Input**: User description: "Unificar la prioridad de herramientas según capacidades del agente, incluir la excepción Codex en los recordatorios y validar una sesión interactiva con llamadas y ahorro medido. La evidencia aportada identifica una limitación de Codex y una alternativa comprimida que no se utiliza; no demuestra un motor de compresión roto ni un fallo actual de hook."

## Clarifications

### Session 2026-10-12

- Q: ¿Qué contenido debe incluirse al medir el ahorro de las recuperaciones comprimidas? → A: La respuesta completa que recibe el agente, incluidos encabezados, metadatos y avisos.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Obtener contexto comprimido por una vía compatible (Priority: P1)

Como persona que usa gomemory con Codex, quiero que el agente obtenga contexto de código por una vía compatible con sus capacidades, para reducir el contexto consumido sin tener que corregir manualmente instrucciones contradictorias.

**Why this priority**: Una recomendación incompatible impide aprovechar la compresión disponible aunque el motor funcione correctamente.

**Independent Test**: Iniciar una sesión con las instrucciones vigentes y solicitar una exploración de código elegible para compresión; comprobar la ruta utilizada y que la respuesta conserve la evidencia necesaria para resolver la consulta.

**Acceptance Scenarios**:

1. **Given** un agente que no admite sustituir resultados recibidos y una vía de recuperación comprimida disponible, **When** consulta código, **Then** utiliza esa vía de forma predeterminada sin solicitar una sustitución incompatible.
2. **Given** un agente con compresión automática compatible, **When** consulta código, **Then** conserva su ruta compatible y no recibe una excepción que lo obligue a usar la alternativa destinada a Codex.
3. **Given** que la vía comprimida no está disponible o no aporta evidencia suficiente, **When** se necesita continuar la consulta, **Then** el agente recurre a una fuente disponible, indica la limitación y no presenta esa recuperación como comprimida.

---

### User Story 2 - Recibir instrucciones consistentes durante la sesión (Priority: P2)

Como persona que instala o actualiza gomemory, quiero que las instrucciones permanentes y los recordatorios de sesión indiquen la misma prioridad de recuperación, para que el agente mantenga una conducta consistente desde el inicio y durante el trabajo.

**Why this priority**: Un recordatorio contradictorio puede anular en la práctica una instrucción correcta y volver a la ruta sin compresión.

**Independent Test**: Comparar las instrucciones gestionadas por gomemory y los recordatorios emitidos para cada perfil de capacidad; verificar que describen la misma regla, excepción y alternativa ante indisponibilidad.

**Acceptance Scenarios**:

1. **Given** una instalación o actualización para Codex, **When** se revisan las instrucciones gestionadas y los recordatorios, **Then** todos priorizan la recuperación comprimida compatible y ninguno exige incondicionalmente una ruta contradictoria.
2. **Given** instrucciones ajenas a gomemory y reglas de privacidad existentes, **When** se actualizan las instrucciones gestionadas, **Then** se preservan esos contenidos y protecciones.
3. **Given** un perfil cuya capacidad de sustitución no está confirmada, **When** se emiten instrucciones, **Then** no se promete compresión automática y se recomienda una vía disponible que no dependa de esa capacidad desconocida.

---

### User Story 3 - Demostrar uso real y ahorro atribuible (Priority: P3)

Como responsable de validar la integración, quiero observar una sesión interactiva real con consultas y ahorro comparable, para distinguir una mejora efectiva de un cambio de redacción o de una estimación sin evidencia.

**Why this priority**: La evidencia histórica muestra ausencia de uso de la alternativa; actualizar instrucciones por sí solo no demuestra que esa conducta cambie.

**Independent Test**: Ejecutar una sesión interactiva sobre un conjunto documentado de consultas, observar sus recuperaciones y comparar el contexto entregado con las respuestas equivalentes sin compresión mediante una misma unidad y método de medición.

**Acceptance Scenarios**:

1. **Given** una sesión con las instrucciones actualizadas, **When** se realizan al menos tres consultas de código con evidencia suficiente y posibilidad de reducción, **Then** las tres utilizan la vía comprimida sin que la persona tenga que nombrar la herramienta que debe usar.
2. **Given** las recuperaciones observadas y sus referencias equivalentes sin compresión, **When** se calcula el ahorro, **Then** se muestran cantidades antes y después, unidad, método, alcance de la sesión y diferencia atribuible; las estimaciones se identifican como tales.
3. **Given** ausencia de mediciones comparables, ahorro nulo o imposibilidad de ejecutar una sesión real, **When** se informa el resultado, **Then** se declara el cierre pendiente o bloqueado, sin sustituirlo por pruebas simuladas ni atribuirlo automáticamente a una avería del motor.

### Edge Cases

- Recuperación comprimida sin resultados, no disponible o incompleta: permitir una alternativa explícita sin inventar evidencia ni ahorro.
- Respuesta pequeña que no admite reducción útil: conservar la información y registrar ahorro nulo; no forzar compresión destructiva.
- Instrucciones externas contradictorias que no gestiona gomemory: advertir el conflicto sin sobrescribirlas silenciosamente.
- Cambio de versión o capacidades del agente: aplicar la regla correspondiente a la capacidad comprobada, no asumir que una limitación histórica es permanente.
- Métricas acumuladas de sesiones anteriores: excluirlas o aislar la diferencia de la sesión evaluada.
- Agente que ignora la prioridad pese a recibir instrucciones correctas: registrar incumplimiento y mantener pendiente la validación de comportamiento.
- Contenido sensible en evidencia o mediciones: excluir secretos y datos privados del informe.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Las instrucciones gestionadas por gomemory DEBEN establecer una prioridad única de recuperación según la capacidad comprobada del agente para recibir resultados comprimidos.
- **FR-002**: Para Codex sin sustitución de resultados compatible, las instrucciones DEBEN priorizar la recuperación comprimida en origen y NO DEBEN exigir incondicionalmente una ruta incompatible con esa prioridad.
- **FR-003**: Los recordatorios de sesión DEBEN reproducir la misma prioridad, excepción y alternativa que las instrucciones permanentes del perfil correspondiente.
- **FR-004**: Los agentes con compresión automática compatible DEBEN conservar su comportamiento compatible existente; una capacidad desconocida NO DEBE presentarse como confirmada.
- **FR-005**: Ante indisponibilidad o insuficiencia de la recuperación comprimida, el agente DEBE poder recurrir a una fuente alternativa disponible, comunicar el motivo y distinguir ese resultado de uno comprimido. La aceptación DEBE observarlo en una solicitud negativa real, prefijada y separada del banco de ahorro, que requiera un cuerpo literal no cubierto por la ruta comprimida estructural; no bastan tests de instrucciones o fixtures para acreditar que comunica la degradación.
- **FR-006**: La instalación o actualización DEBE mantener consistentes las instrucciones que gestiona gomemory, sin sobrescribir contenido ajeno ni relajar privacidad o protección de información crítica.
- **FR-007**: La validación DEBE incluir una sesión interactiva real con al menos tres consultas elegibles documentadas y un registro de las recuperaciones realmente utilizadas, sin indicaciones manuales sobre qué herramienta elegir durante esas consultas.
- **FR-008**: El informe de validación DEBE comparar la respuesta completa que recibe el agente, incluidos encabezados, metadatos y avisos, con una referencia equivalente sin compresión medida con el mismo alcance; DEBE registrar cantidades antes y después y especificar una unidad y método comunes, identificar las estimaciones y aislar el ahorro de la sesión respecto de métricas históricas.
- **FR-009**: La validación DEBE comprobar que cada respuesta preserva los hechos, referencias y restricciones necesarios para responder la consulta; ahorrar contexto NO DEBE ocultar evidencia crítica.
- **FR-010**: El resultado DEBE distinguir incompatibilidad del agente, instrucciones contradictorias, ausencia de uso y fallo de recuperación; NO DEBE diagnosticar una avería del motor ni un fallo de hook sin evidencia específica.
- **FR-011**: Si no hay sesión real verificable, medición comparable, uso de la vía prevista o ahorro positivo agregado en las consultas elegibles, el cierre DEBE permanecer pendiente o bloqueado con motivo explícito.

### Key Entities *(include if feature involves data)*

- **Perfil de capacidades**: agente y versión evaluados, capacidades confirmadas, desconocidas o incompatibles y prioridad aplicable.
- **Regla de recuperación**: prioridad, excepción y alternativa que deben compartir las instrucciones permanentes y los recordatorios.
- **Evidencia de validación**: sesión, consultas, rutas utilizadas, información necesaria preservada y limitaciones observadas; sin secretos ni transcripciones sensibles completas.
- **Comparación de ahorro**: respuesta de referencia y respuesta entregada completas para una consulta equivalente, incluidos encabezados, metadatos y avisos, cantidades, unidad, método y diferencia atribuible a la sesión.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El 100 % de las instrucciones y recordatorios gestionados incluidos en la validación expresan una prioridad consistente para cada perfil evaluado, sin contradicciones entre superficies.
- **SC-002**: En una sesión interactiva real, tres de tres consultas elegibles utilizan la recuperación comprimida compatible sin corrección manual sobre la herramienta elegida.
- **SC-003**: El volumen agregado de las respuestas completas que recibe el agente en esas consultas, incluidos encabezados, metadatos y avisos, es menor que el de sus referencias equivalentes sin compresión medidas con el mismo alcance; el informe muestra valores antes y después, diferencia absoluta y porcentaje con un método común.
- **SC-004**: Las tres respuestas permiten resolver las consultas y conservan el 100 % de los hechos y referencias definidos como necesarios para su aceptación.
- **SC-005**: El 100 % de los casos de indisponibilidad, capacidad desconocida y medición insuficiente evaluados comunican la limitación y no presentan ahorro ni compatibilidad no demostrados. La comprobación incluye una respuesta real de Codex TUI ante insuficiencia estructural, evaluada aparte de las tres consultas elegibles.
- **SC-006**: Todos los perfiles previamente compatibles incluidos en la validación conservan su ruta prevista sin introducir instrucciones incompatibles.

## Assumptions

- Aunque el encabezado menciona OpenCode, los hechos y el cierre solicitado se refieren a Codex. El objetivo principal es Codex; otros agentes entran únicamente para comprobar consistencia según capacidades y evitar regresiones.
- La evidencia histórica proporcionada y la memoria 450 son antecedentes del problema, no una validación ejecutada por esta especificación: Codex 0.162.1 rechaza la sustitución descrita, y tres sesiones examinadas acumulan 29 recuperaciones externas sin uso observado de las alternativas comprimidas.
- No se presume que el motor de compresión esté roto ni que exista un fallo actual de hook. Investigar defectos nuevos exige evidencia propia y queda fuera del cierre descrito.
- Se reutilizan las capacidades de recuperación comprimida existentes. Quedan fuera de alcance cambios al agente externo, un motor nuevo, una reescritura general de OpenCode y correcciones de fallos no reproducidos.
- La validación depende de acceso a una sesión interactiva real, fuentes de código disponibles y referencias comparables. Si esos recursos faltan, se informa el bloqueo.
- Tres consultas constituyen una comprobación mínima de comportamiento, no una garantía estadística para todas las sesiones. No se fija un porcentaje arbitrario de ahorro; sí se exige reducción positiva agregada y conservación de evidencia necesaria.
- Esta especificación ya cuenta con plan, tareas, contratos y validación de análisis; no implementa cambios de integración ni ejecuta la validación interactiva de aceptación.
