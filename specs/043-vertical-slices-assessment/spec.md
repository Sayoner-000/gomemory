# Feature Specification: Evaluación de la base Go para vertical slices en arquitectura hexagonal

**Feature Branch**: `043-vertical-slices-assessment`

**Created**: 2026-10-25

**Status**: Draft

**Input**: Necesidad preparada y aprobada con Socrática (flujo `01a172f4-c849-7ee0-a273-feea6f463b3f`, revisión 3): determinar, con evidencia del código Go actual, si la base necesita refactorización para facilitar el desarrollo por vertical slices dentro de la arquitectura hexagonal existente, sin modificar archivos ni implementar la refactorización.

## Actores

- **A1 — Persona mantenedora del repositorio**: decide si se refactoriza, a partir del informe.
- **A2 — Agente evaluador**: inspecciona el código y su ejecución sin modificarlos.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Mapa estructural y de flujos reales (Priority: P1) — origen S1

La persona mantenedora necesita conocer la estructura real del código Go y cómo recorren las capas los flujos de funcionalidades representativas, para razonar sobre slices con hechos y no con la estructura declarada.

**Why this priority**: Todo hallazgo, criterio y conclusión posterior se apoya en este mapa; sin él, el resto del informe no es verificable.

**Independent Test**: Se puede comprobar por sí solo tomando cualquier paso citado del mapa y abriendo el archivo y la línea indicados, o reproduciendo el comando de inventario de dependencias, y confirmando que coincide con el código actual.

**Acceptance Scenarios** (criterio C1):

1. **Given** el repositorio en su estado actual, **When** se entrega el diagnóstico estructural, **Then** incluye el inventario de paquetes por capa (domain, application/ports, application/usecases, adapters/primary, adapters/secondary, infrastructure) con sus dependencias de importación.
2. **Given** el diagnóstico estructural, **When** se revisan los flujos descritos, **Then** hay al menos tres funcionalidades recorridas desde su punto de entrada (CLI, servidor MCP, TUI o hook) hasta persistencia o un adaptador externo, con cada paso citado como archivo:línea.

---

### User Story 2 - Hallazgos separados de preferencias y prácticas a conservar (Priority: P1) — origen S2

La persona mantenedora necesita saber qué problemas concretos dificultan desarrollar por vertical slices, distinguirlos de las preferencias de diseño y saber qué partes ya siguen bien el enfoque, para no corregir lo que funciona ni tratar gustos como defectos.

**Why this priority**: Es la base de la decisión; un hallazgo mal clasificado conduce a una refactorización innecesaria o a omitir un problema real.

**Independent Test**: Se puede comprobar por sí solo revisando cada hallazgo y cada práctica a conservar: cada uno debe tener su evidencia reproducible, su categoría y su consecuencia (o su motivo de conservación), y ninguna preferencia debe aparecer en la lista de problemas.

**Acceptance Scenarios** (criterios C2 y C3):

1. **Given** un problema observado, **When** se informa, **Then** tiene evidencia archivo:línea o un comando reproducible, una categoría (acoplamiento entre capas, dependencia invertida o incorrecta, lógica repartida, dificultad de prueba, límite de módulo o punto de entrada) y la consecuencia concreta para desarrollar por slices.
2. **Given** una observación sin defecto demostrable, **When** se incluye en el informe, **Then** aparece en una lista separada de preferencias de diseño y no se presenta como defecto.
3. **Given** una parte del código que ya sigue el enfoque, **When** se reconoce, **Then** se identifica con evidencia archivo:línea y se explica por qué favorece el desarrollo por slices o la separación hexagonal.

---

### User Story 3 - Conclusión justificada y, si procede, roadmap (Priority: P2) — origen S3

La persona mantenedora necesita decidir si refactorizar, mantener el diseño o hacer cambios puntuales, y cómo avanzar si hay cambios.

**Why this priority**: Es el resultado que se usa para decidir, pero depende de que las historias 1 y 2 estén completas.

**Independent Test**: Se puede comprobar por sí solo verificando que los criterios de evaluación aparecen declarados antes de aplicarse, que la conclusión elige una única opción citando hallazgos concretos y que cada paso del roadmap, si existe, tiene un criterio de éxito ejecutable o medible.

**Acceptance Scenarios** (criterios C4 y C5):

1. **Given** el diagnóstico completo, **When** se emite la conclusión, **Then** los criterios de evaluación de capas, módulos y pruebas están declarados antes de aplicarse y se aplican a todas las funcionalidades revisadas.
2. **Given** los criterios aplicados, **When** se emite la conclusión, **Then** elige exactamente una opción (refactorizar, mantener o cambios puntuales) y cita los hallazgos que la sostienen.
3. **Given** una conclusión que recomienda cambios, **When** se entrega el roadmap, **Then** ordena los pasos por riesgo y beneficio y cada paso nombra un slice candidato y un criterio de éxito verificable mediante un comando o una comprobación medible.
4. **Given** una conclusión de mantener el diseño, **When** se entrega el informe, **Then** no hay roadmap y se explica por qué.

---

### User Story 4 - Evaluación sin alterar el repositorio (Priority: P1) — origen S4

El agente evaluador necesita evaluar el repositorio sin alterarlo, para que la evaluación no introduzca cambios ni riesgos.

**Why this priority**: Es una restricción explícita de la solicitud; una evaluación que modifica el código invalida su propio diagnóstico.

**Independent Test**: Se puede comprobar por sí solo comparando el estado de git al empezar y al terminar la evaluación.

**Acceptance Scenarios** (criterio C6):

1. **Given** el estado de git registrado al empezar, **When** termina la evaluación, **Then** git status muestra el mismo estado y no se creó, modificó ni borró ningún archivo del repositorio.

---

### Edge Cases

- **Funcionalidad sin punto de entrada claro o con varios**: si una funcionalidad se invoca desde más de un punto de entrada (por ejemplo CLI y MCP), el recorrido cita cada entrada o declara cuál se eligió y por qué.
- **Evidencia que depende de la ejecución**: si una comprobación (compilación, pruebas, inventario de dependencias) no puede ejecutarse por falta de dependencias o red, el hallazgo se marca como no verificado y no se presenta como observado.
- **Hallazgo que es a la vez defecto y preferencia**: se clasifica como problema solo si se demuestra la consecuencia para desarrollar por slices; si no, va a preferencias.
- **Conclusión de mantener con defectos menores**: si hay hallazgos que no justifican refactorizar, la conclusión puede ser "cambios puntuales" o "mantener"; en ningún caso se elige más de una opción.
- **Archivos generados por la propia evaluación**: cualquier salida intermedia se escribe fuera del repositorio para no romper el criterio C6.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: La evaluación MUST producir un inventario de paquetes Go agrupados por capa (domain, application/ports, application/usecases, adapters/primary, adapters/secondary, infrastructure) con sus dependencias de importación. (C1)
- **FR-002**: La evaluación MUST recorrer al menos tres funcionalidades representativas de extremo a extremo, desde su punto de entrada (CLI, servidor MCP, TUI o hook) hasta persistencia o un adaptador externo, citando cada paso como archivo:línea. (C1)
- **FR-003**: Cada problema observado MUST incluir evidencia archivo:línea o un comando reproducible, una categoría de la lista cerrada (acoplamiento entre capas, dependencia invertida o incorrecta, lógica repartida, dificultad de prueba, límite de módulo, punto de entrada) y su consecuencia concreta para desarrollar por slices. (C2)
- **FR-004**: Las preferencias de diseño MUST presentarse en una lista separada de los problemas observados y MUST NOT presentarse como defectos. (C2)
- **FR-005**: La evaluación MUST identificar las prácticas que ya siguen bien el enfoque hexagonal o por slices, cada una con evidencia archivo:línea y la razón por la que conviene conservarla. (C3)
- **FR-006**: Los criterios de evaluación de capas, módulos y pruebas MUST declararse antes de aplicarse y MUST aplicarse de forma uniforme a todas las funcionalidades revisadas. (C4)
- **FR-007**: La conclusión MUST elegir exactamente una opción entre refactorizar, mantener el diseño o cambios puntuales, y MUST citar los hallazgos que la sostienen; recomendar no refactorizar es una conclusión válida. (C4)
- **FR-008**: Si la conclusión recomienda cambios, el informe MUST incluir un roadmap ordenado por riesgo y beneficio en el que cada paso nombra un slice candidato y un criterio de éxito verificable mediante un comando o una comprobación medible. (C5)
- **FR-009**: Si la conclusión es mantener el diseño, el informe MUST NOT incluir roadmap y MUST explicar por qué. (C5)
- **FR-010**: La evaluación MUST ser de solo lectura: al terminar, el estado de git MUST coincidir con el registrado al empezar, sin archivos del repositorio creados, modificados ni borrados. (C6)
- **FR-011**: Cada hallazgo MUST apoyarse en el código o en su ejecución real; una afirmación que no pudo verificarse MUST marcarse como no verificada.
- **FR-012**: La evaluación MUST usar la constitución técnica del proyecto como referencia de cómo deben escribirse las capas, y MUST señalar cuando un hallazgo contradice o confirma una regla concreta de ella.

### Key Entities

- **Paquete por capa**: unidad del inventario; tiene capa, ruta e importaciones hacia otros paquetes.
- **Flujo recorrido**: funcionalidad representativa con su punto de entrada y la secuencia de pasos citados hasta persistencia o un adaptador externo.
- **Hallazgo**: problema observado con evidencia, categoría y consecuencia para el desarrollo por slices.
- **Preferencia de diseño**: observación sin defecto demostrable, separada de los hallazgos.
- **Práctica a conservar**: parte del código que ya sigue el enfoque, con evidencia y motivo.
- **Criterio de evaluación**: regla declarada para juzgar capas, módulos o pruebas, aplicada a todos los flujos.
- **Conclusión**: una de tres opciones, con los hallazgos que la sostienen.
- **Paso del roadmap**: slice candidato, prioridad por riesgo y beneficio y criterio de éxito verificable.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El 100 % de los pasos citados en el mapa estructural y en los flujos coinciden con el código actual al comprobarlos uno a uno. (C1)
- **SC-002**: Se recorren al menos 3 funcionalidades de extremo a extremo, cada una desde un punto de entrada distinto o con su elección justificada. (C1)
- **SC-003**: El 100 % de los problemas observados tienen evidencia reproducible, categoría y consecuencia, y ninguna preferencia aparece en la lista de problemas. (C2)
- **SC-004**: El 100 % de las prácticas a conservar tienen evidencia y motivo de conservación. (C3)
- **SC-005**: La conclusión elige exactamente 1 de las 3 opciones y cita al menos un hallazgo o práctica que la sostiene. (C4)
- **SC-006**: Si hay roadmap, el 100 % de sus pasos tienen slice candidato, prioridad por riesgo y beneficio y un criterio de éxito ejecutable o medible; si no lo hay, el informe explica por qué. (C5)
- **SC-007**: Al terminar, el estado de git es idéntico al registrado al empezar: 0 archivos del repositorio creados, modificados o borrados por la evaluación. (C6)
- **SC-008**: La persona mantenedora puede tomar la decisión de refactorizar, mantener o hacer cambios puntuales solo con el informe, sin necesitar una revisión adicional del código.

## Assumptions

- La arquitectura hexagonal de referencia es la que ya declara el repositorio y su constitución técnica: domain, application (ports y usecases), adapters (primary y secondary) e infrastructure como composición.
- Un vertical slice es una funcionalidad completa (por ejemplo socratica, review o instincts) que puede desarrollarse y probarse cruzando capas sin tocar otras funcionalidades.
- El informe se entrega en la conversación o en un documento ubicado fuera del repositorio, sin generar código, para que sea compatible con el criterio C6. Los artefactos de especificación de esta carpeta pertenecen al proceso de Spec Kit, no a la evaluación.
- Las funcionalidades usadas como muestra (pendiente P1, no bloqueante) se eligen durante la evaluación según tamaño y riesgo, y la elección se justifica en el informe.
- Queda fuera de alcance: modificar archivos o implementar la refactorización; evaluar código que no es Go (scripts Python, instalador, assets, skills), salvo cuando sea punto de entrada de un flujo Go revisado; optimizar rendimiento, cambiar dependencias externas o alterar el comportamiento visible.

## Trazabilidad con la preparación aprobada

| Criterio preparado | Historias de origen | Referencia en esta especificación |
|--------------------|---------------------|-----------------------------------|
| C1 | S1 | User Story 1; FR-001, FR-002; SC-001, SC-002 |
| C2 | S2 | User Story 2 (escenarios 1–2); FR-003, FR-004; SC-003 |
| C3 | S2 | User Story 2 (escenario 3); FR-005; SC-004 |
| C4 | S3 | User Story 3 (escenarios 1–2); FR-006, FR-007; SC-005 |
| C5 | S3 | User Story 3 (escenarios 3–4); FR-008, FR-009; SC-006 |
| C6 | S4 | User Story 4; FR-010; SC-007 |

Entregas: D1 (Diagnóstico con evidencia) = User Stories 1, 2 y 4; D2 (Conclusión y roadmap) = User Story 3.
