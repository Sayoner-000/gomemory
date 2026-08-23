# Feature Specification: Economía del contexto

**Feature Branch**: `023-context-economy`

**Created**: 2026-08-23

**Status**: Draft

**Input**: Mediciones de duplicación entre canales y de conducta inducida, tomadas en una sesión
real del 2026-08-23. Se separó de la especificación 022, que atiende la estructura de los
canales; esta atiende lo que cuesta usarlos.

## Contexto del problema

gomemory hace llegar el mismo material al agente por varios canales, y ningún canal sabe qué
entregaron los otros. El resultado es que la misma información se paga varias veces en la misma
sesión.

Medido el 2026-08-23:

| Duplicación | Costo | Cómo se verificó |
|---|---|---|
| La operación de contexto para planificar reenvía íntegro lo que ya entregó la operación de contexto general | ~6.100 tokens cobrados dos veces | 180 de 180 líneas únicas idénticas |
| Las reglas de trabajo llegan por el archivo de instrucciones de la persona y otra vez con el contexto | ~975 tokens duplicados | 48 líneas idénticas |
| El bloque de protocolo viaja en la respuesta de conexión, en el archivo de instrucciones y como preámbulo de cada respuesta | ~1.087 tokens por copia | inspección de los tres canales |

Un canal sí acota su huella: entregó 21.290 bytes y solo expuso unos 2 KB, porque persiste el
resto fuera del contexto. Ese comportamiento es el que falta generalizar.

### El costo que no se ve es el que manda

En esa misma sesión, la huella total de gomemory fue de 27.793 tokens en 8 operaciones. El
trabajo delegado a agentes auxiliares consumió 178.031. **Seis veces más que todo el sistema de
memoria que se estaba auditando por consumo.**

Esa delegación no fue una decisión libre del agente. El documento de reglas que gomemory
inicializa por defecto se la ordena, en su sección de orquestación: *«Usa subagentes
libremente»*, *«Delega investigación, exploración y análisis paralelo»*, *«Para problemas
complejos, utiliza más cómputo mediante subagentes»*. La instrucción llega además por tres
copias simultáneas, dos de ellas en el mismo archivo de instrucciones, donde el bloque aparece
repetido. La única indicación en sentido contrario proviene del entorno del agente, es genérica
y queda en minoría.

**gomemory no entrega solo contexto: entrega directivas de conducta.** Su costo no se paga en el
tamaño del texto, sino en lo que ese texto hace que el agente haga después. Medir la huella e
ignorar la conducta inducida mide la parte pequeña.

Hay un segundo efecto, ya observado: cuando la persona impone una condición de trabajo que
contradice el documento de reglas, la contradicción permanece en el contexto de cada sesión.
No se resuelve entregando más contexto.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Que las reglas por defecto no ordenen gastar (Priority: P1)

Quien instala gomemory recibe un documento de reglas que el agente lee en cada sesión. Hoy ese
documento ordena delegar sin límite, y el agente obedece. Esa obediencia es el gasto dominante.

**Why this priority**: es el mayor ahorro medido y el más barato de aplicar, porque el documento
de reglas es reemplazable sin tocar código. Una sola edición cambia el comportamiento en todos
los proyectos y todos los agentes.

**Independent Test**: revisar el documento por defecto y comprobar que ninguna indicación ordena
delegar sin declarar su costo; después, comprobar que el bloque no llega repetido.

**Acceptance Scenarios**:

1. **Given** una instalación nueva, **When** se consulta el documento de reglas por defecto,
   **Then** la sección de orquestación presenta la delegación como una decisión con costo, no
   como práctica recomendada.
2. **Given** el documento por defecto, **When** el agente enfrenta una tarea con varios ángulos,
   **Then** el documento declara explícitamente que esa condición no basta para delegar.
3. **Given** el bloque de reglas administrado por gomemory, **When** se inspecciona el archivo
   de instrucciones de la persona, **Then** el bloque aparece una sola vez.
4. **Given** un equipo que prefiere delegación amplia, **When** reemplaza el documento, **Then**
   su versión prevalece y no se sobrescribe.
5. **Given** una condición de trabajo que la persona impuso y que contradice el documento por
   defecto, **When** se entrega el contexto, **Then** la condición de la persona aparece por
   encima del texto por defecto, sin depender de que el agente resuelva la contradicción.

---

### User Story 2 - Dejar de pagar dos veces por el mismo contexto (Priority: P1)

Quien planifica necesita el método de descomposición y el historial. Hoy recibe el historial dos
veces: al abrir la sesión y al pedir el contexto de planificación. La segunda copia no aporta
información nueva.

**Why this priority**: es el mayor ahorro entre los canales medidos y el más fácil de verificar.

**Independent Test**: pedir el contexto de planificación en una sesión que ya cargó el contexto
general, y comprobar que no se repite lo entregado y que quien planifica conserva el material.

**Acceptance Scenarios**:

1. **Given** una sesión que ya cargó el contexto general, **When** se pide el contexto para
   planificar, **Then** se entrega el método sin repetir el historial, indicando que el
   historial ya está disponible en la sesión.
2. **Given** una sesión que **no** cargó el contexto general, **When** se pide el contexto para
   planificar, **Then** se entregan método e historial completos.
3. **Given** un historial que cambió desde la primera entrega, **When** se pide el contexto para
   planificar, **Then** se entrega lo que cambió, no el historial completo ni nada.
4. **Given** que el archivo de instrucciones de la persona ya contiene las reglas de trabajo,
   **When** se entrega el contexto, **Then** las reglas no se repiten y se indica dónde están.
5. **Given** una sesión que perdió el material por compactación, **When** se pide el contexto,
   **Then** existe una forma explícita de recibirlo completo.

---

### User Story 3 - Ver qué cuesta cada canal y qué se duplica (Priority: P2)

Quien mantiene el proyecto quiere saber cuánto entregó cada canal en una sesión y qué porción ya
había sido entregada por otro. Hoy esa cifra solo se obtiene ejecutando comandos, guardando
salidas y comparándolas línea por línea, que es como se construyó la tabla de arriba.

**Why this priority**: produce la evidencia que permite demostrar que las dos historias
anteriores funcionaron. Va en P2 porque el ahorro ya está identificado: la medición sirve para
sostenerlo en el tiempo, no para descubrirlo.

**Independent Test**: ejecutar una sesión que cargue contexto por dos vías y pedir el informe.
El informe debe nombrar los canales, su costo y la porción duplicada.

**Acceptance Scenarios**:

1. **Given** una sesión con contexto cargado por dos vías, **When** se consulta el informe,
   **Then** indica cuánto aportó cada operación y cuánto de la segunda venía en la primera.
2. **Given** un mismo bloque entregado por tres canales, **When** se consulta el informe,
   **Then** lo identifica como repetido y suma el costo de las copias.
3. **Given** una sesión sin duplicación, **When** se consulta el informe, **Then** lo declara
   explícitamente en lugar de omitir la sección.
4. **Given** una sesión con trabajo delegado, **When** se consulta el informe, **Then** el
   material consumido por la delegación aparece separado del que entrega la memoria.

---

### Edge Cases

- La sesión se compacta y el agente pierde material que el registro cree entregado: la supresión
  de la segunda copia no puede dejarlo sin contexto. La vía de recuperación explícita es la
  salida, y el plan debe decidir si se activa a mano o al detectar la compactación.
- Dos canales entregan el mismo contenido con diferencias mínimas de formato: la detección debe
  reconocerlo o declarar que solo cubre coincidencia literal.
- Varias sesiones del mismo proyecto corren a la vez: lo entregado en una no puede suprimir
  material en otra.
- El registro de entregas crece en sesiones largas: necesita una política de retención que no
  degrade el arranque.
- El agente no reporta su consumo real y la medición es aproximada: el informe debe declararlo.
- Un equipo reemplazó el documento de reglas y su versión ordena delegar: es su decisión y debe
  respetarse; el informe sigue mostrando el costo.

## Requirements *(mandatory)*

### Functional Requirements

**Reglas que no ordenen gastar (Historia 1)**

- **FR-001**: El documento de reglas por defecto NO DEBE indicar al agente que delegue trabajo
  sin declarar el costo de hacerlo.
- **FR-002**: El documento por defecto DEBE acotar la delegación a los casos en que aporta algo
  que el trabajo directo no puede dar, y DEBE declarar qué condiciones no bastan para delegar.
- **FR-003**: El bloque de reglas administrado por gomemory DEBE aparecer una sola vez en el
  archivo de instrucciones de la persona.
- **FR-004**: Un documento de reglas reemplazado por el equipo DEBE prevalecer y no ser
  sobrescrito, incluida esta modificación del contenido por defecto.
- **FR-005**: Una condición de trabajo registrada por la persona DEBE presentarse en el contexto
  por encima del contenido por defecto con el que entre en conflicto.

**Entrega sin repetición (Historia 2)**

- **FR-006**: La operación de contexto para planificar NO DEBE reenviar el material que la
  operación de contexto general ya entregó en la misma sesión.
- **FR-007**: Cuando se suprima material por haberse entregado antes, la respuesta DEBE indicar
  que se suprimió y dónde está disponible.
- **FR-008**: Si el material cambió desde la entrega anterior, el sistema DEBE entregar lo que
  cambió.
- **FR-009**: Si no consta una entrega previa en la sesión, el sistema DEBE entregar el material
  completo. La reducción no puede dejar al agente sin contexto.
- **FR-010**: El sistema DEBE ofrecer una forma explícita de pedir el material completo,
  ignorando la supresión.
- **FR-011**: El sistema NO DEBE repetir en el contexto el material que ya viaja en el archivo
  de instrucciones de la persona, y DEBE indicar dónde está.
- **FR-012**: La supresión DEBE estar acotada a la sesión: lo entregado en una no puede suprimir
  material en otra.

**Medición (Historia 3)**

- **FR-013**: El sistema DEBE registrar, por sesión, qué canal entregó material y su tamaño
  aproximado.
- **FR-014**: El sistema DEBE identificar qué porción entregada por un canal ya había sido
  entregada por otro en la misma sesión, y reportarla como duplicada.
- **FR-015**: El informe DEBE desglosar por canal y declarar el total duplicado.
- **FR-016**: El informe DEBE separar el material que entrega la memoria del que consume el
  trabajo delegado, para que su peso relativo se lea sin cálculo manual.
- **FR-017**: El informe DEBE declarar que sus cifras son aproximadas y comparables contra sí
  mismas, no contra la facturación de ningún proveedor.
- **FR-018**: El registro de entregas DEBE tener una política de retención declarada.

### Key Entities

- **Registro de entrega**: lo que se entregó por un canal en una sesión — canal, tamaño
  aproximado, identificador de contenido para detectar repetición, y momento.
- **Informe de consumo**: la vista agregada por sesión: costo por canal, porción duplicada, y
  separación entre lo que entrega la memoria y lo que consume el trabajo delegado.
- **Documento de reglas**: el texto de orquestación que el agente lee en cada sesión. Tiene
  contenido por defecto reemplazable y, una vez reemplazado, queda bajo control del equipo.
- **Condición de trabajo**: una regla que la persona impuso y que prevalece sobre el contenido
  por defecto en conflicto.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En una tarea equivalente a la sesión de referencia, el material consumido por
  trabajo delegado baja al menos un 70 % respecto de la línea base de 178.031 tokens, sin que la
  tarea pierda alcance ni verificación.
- **SC-002**: En una sesión que carga contexto general y después contexto de planificación, el
  material entregado baja al menos un 40 % respecto de la línea base de 13.200 tokens, sin que
  quien planifica pierda acceso a ningún elemento del historial.
- **SC-003**: El bloque de reglas administrado por gomemory aparece exactamente una vez en el
  archivo de instrucciones de la persona.
- **SC-004**: Un equipo que reemplaza el documento de reglas conserva su versión tras una
  reinstalación y tras una actualización.
- **SC-005**: Una condición de trabajo registrada por la persona aparece en el contexto por
  encima del texto por defecto con el que entra en conflicto.
- **SC-006**: En una sesión donde se suprimió material, quien planifica lo recupera completo en
  un solo paso.
- **SC-007**: Quien mantiene el proyecto obtiene el desglose por canal y la porción duplicada en
  una sola consulta, sin ejecutar comandos adicionales ni comparar salidas a mano.
- **SC-008**: El informe distingue el material que entrega la memoria del que consume el trabajo
  delegado.

## Assumptions

- La medición de tamaño sigue usando la aproximación neutral que el proyecto ya emplea. No se
  introduce dependencia de ningún tokenizador de proveedor.
- La detección de duplicados se basa en coincidencia de contenido, no en equivalencia semántica.
- El registro de entregas vive junto al resto del estado del proyecto y respeta la política de
  privacidad vigente: nada marcado como secreto se persiste.
- Las cifras citadas provienen de una sesión concreta del 2026-08-23 y sirven como línea base de
  comparación, no como promedio de toda sesión.
- El comportamiento del canal que persiste su salida fuera del contexto se toma como referencia
  de lo que se quiere generalizar.

## Out of Scope

- Gobernar cómo trabaja quien conduce la sesión. Esta feature hace **visible** el costo de la
  delegación y retira la instrucción que la ordena; no la prohíbe.
- Cambiar el método de conteo por el tokenizador propio de algún proveedor.
- Acortar el texto del protocolo de memoria. Aquí se ataca la repetición entre canales, no la
  longitud del texto.
- La estructura de los canales y su ciclo de vida. Corresponde a la especificación 022.
- Detectar que un canal dejó de ejercerse. Corresponde a la especificación 024.
