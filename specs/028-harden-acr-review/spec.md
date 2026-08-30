# Feature Specification: Fortalecimiento de la revisión ACR

**Feature Branch**: `[028-harden-acr-review]`

**Created**: 2026-08-29

**Status**: Draft

**Input**: User description: "Corregir los hallazgos de la revisión adversarial de `mem review`, resolverlos sin borrar el ledger y validar el resultado mediante una nueva revisión independiente, siguiendo el plan atómico acordado."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Impedir aprobaciones falsas (Priority: P1)

Como responsable de un proyecto, quiero que toda revisión clasifique la totalidad de los hallazgos de ambos revisores y conserve su severidad real, para que un defecto grave no pueda quedar oculto detrás de un consenso parcial o degradado.

**Why this priority**: Una aprobación falsa elimina el valor de seguridad del protocolo y puede presentar como confiable un cambio que todavía contiene defectos graves.

**Independent Test**: Se puede probar presentando resultados con varios hallazgos graves e intentando omitirlos, repetirlos o reducir su severidad durante el consenso; ningún caso debe terminar aprobado.

**Acceptance Scenarios**:

1. **Given** dos revisores que reportaron hallazgos, **When** el consenso omite al menos uno, **Then** la revisión rechaza la clasificación y no puede finalizar como aprobada.
2. **Given** un hallazgo grave reportado por cualquiera de los revisores, **When** el consenso intenta asignarle una severidad menor, **Then** la revisión conserva la severidad más alta respaldada por las fuentes.
3. **Given** una ronda de consenso ya registrada, **When** se reenvía exactamente la misma clasificación, **Then** el resultado es idempotente y no duplica ni altera el ledger.
4. **Given** una ronda de consenso ya registrada, **When** se intenta reemplazarla por una clasificación diferente, **Then** la operación se rechaza y se conserva el registro original.

---

### User Story 2 - Corregir y revalidar con trazabilidad completa (Priority: P1)

Como responsable de una revisión con defectos confirmados, quiero que cada corrección mantenga una cadena verificable entre el target original, el target corregido, los hallazgos abordados y dos re-juicios independientes, para saber que el defecto realmente se resolvió.

**Why this priority**: Sin esta cadena, una revisión puede declarar resuelto un defecto que no fue corregido o que fue inspeccionado sobre una revisión equivocada del target.

**Independent Test**: Se puede probar registrando dos defectos confirmados, corrigiendo solo uno e intentando resolver ambos; únicamente el defecto incluido en la corrección y validado por los dos revisores debe poder quedar resuelto.

**Acceptance Scenarios**:

1. **Given** una primera corrección, **When** su target base no coincide con el target congelado, **Then** la corrección se rechaza.
2. **Given** varias rondas de corrección, **When** una nueva ronda no parte del último target corregido, **Then** la cadena se considera inválida y no avanza.
3. **Given** un hallazgo no incluido en la corrección vigente, **When** se intenta marcarlo como resuelto, **Then** el re-juicio se rechaza.
4. **Given** un hallazgo corregido, **When** solo un revisor lo considera resuelto, **Then** permanece sin resolver.
5. **Given** dos re-juicios independientes sobre el mismo hallazgo corregido, **When** ambos lo consideran resuelto, **Then** el hallazgo queda `RESOLVED` con evidencia de ambas fuentes.
6. **Given** dos correcciones concurrentes para la misma ronda, **When** ambas intentan registrarse, **Then** solo una transición válida se conserva y ninguna corrección se sobrescribe silenciosamente.

---

### User Story 3 - Respetar el ciclo de vida y la política de revisión (Priority: P2)

Como operador de `mem review`, quiero distinguir entre una revisión de solo lectura y una revisión autorizada para corregir, respetar la configuración del proyecto y mantener inmutables los estados terminales, para que el protocolo siempre termine de forma segura y predecible.

**Why this priority**: Una revisión sin permiso de corrección no debe quedar bloqueada indefinidamente ni abrir una vía para mutar el target; una revisión terminada tampoco debe poder reabrirse por accidente.

**Independent Test**: Se puede probar una revisión de solo lectura con un defecto grave confirmado y verificar que termine escalada; también se puede intentar enviar nuevos resultados después de un estado terminal y comprobar que se rechacen.

**Acceptance Scenarios**:

1. **Given** una revisión de solo lectura con un defecto grave confirmado, **When** se solicita el veredicto final, **Then** termina escalada en vez de quedar pendiente de una corrección no autorizada.
2. **Given** una revisión autorizada para corregir, **When** quedan rondas disponibles y existe un defecto grave abierto, **Then** permanece disponible para corrección sin aprobarse.
3. **Given** una revisión aprobada, escalada o incompleta, **When** se intenta enviar resultados, consenso, correcciones o re-juicios adicionales, **Then** la operación se rechaza y el veredicto no cambia.
4. **Given** una política de rondas y severidades configurada para el proyecto, **When** se inicia una revisión sin valores explícitos, **Then** la revisión usa esa política sin perderla en lecturas o escrituras posteriores.
5. **Given** un hallazgo confirmado y resuelto, **When** la revisión todavía no está aprobada, **Then** el aprendizaje no puede promoverse a memoria persistente.

---

### User Story 4 - Auditar el protocolo y congelar el target completo (Priority: P2)

Como auditor o mantenedor, quiero consultar un resumen completo, métricas coherentes y el linaje de cada hallazgo, además de congelar todos los cambios pendientes del proyecto, para reconstruir la revisión sin depender de información externa.

**Why this priority**: La auditoría incompleta y los targets parciales generan confianza falsa aunque las decisiones internas sean correctas.

**Independent Test**: Se puede iniciar una revisión con cambios preparados, no preparados y archivos nuevos, y después consultar el ledger para verificar que el target y toda la cadena de evidencia sean visibles y coherentes.

**Acceptance Scenarios**:

1. **Given** cambios preparados, no preparados y archivos nuevos, **When** se congela el conjunto de cambios pendientes, **Then** todos forman parte de una identidad determinista del target.
2. **Given** un proyecto sin cambios pendientes, **When** se intenta iniciar una revisión del conjunto de cambios, **Then** la operación se rechaza con un diagnóstico claro.
3. **Given** una revisión con hallazgos, consenso, corrección y re-juicios, **When** se consulta su detalle, **Then** se muestran las fuentes, clasificaciones, correcciones, verificaciones y estados necesarios para reconstruir el linaje.
4. **Given** una revisión finalizada, **When** se consultan sus métricas, **Then** los nombres, cantidades, duración, rondas y resultados de promoción coinciden con el contrato publicado.

---

### User Story 5 - Cerrar los hallazgos sin borrar el historial (Priority: P3)

Como equipo responsable de GoMemory, quiero cerrar la revisión que descubrió estos defectos y ejecutar una revisión adversarial nueva sobre el resultado corregido, para demostrar que las causas fueron eliminadas sin destruir la evidencia histórica.

**Why this priority**: La corrección solo es confiable cuando se verifica en el flujo real y por revisores independientes; borrar los hallazgos impediría demostrarlo.

**Independent Test**: Se puede registrar el delta de corrección en la revisión original, obtener dos re-juicios y luego revisar el target corregido desde cero; la revisión original debe conservar los hallazgos resueltos y la nueva no debe confirmar defectos graves.

**Acceptance Scenarios**:

1. **Given** los defectos confirmados de la revisión original, **When** se registra el delta que los corrige y ambos revisores los revalidan, **Then** quedan `RESOLVED` sin eliminar sus registros.
2. **Given** el target corregido, **When** dos revisores independientes ejecutan una revisión nueva, **Then** el protocolo puede finalizar `APPROVED` únicamente si no quedan defectos graves ni contradicciones severas.

### Edge Cases

- Un mismo hallazgo se referencia dos veces dentro de una clasificación o aparece simultáneamente como emparejado y no emparejado.
- Los revisores informan severidades diferentes para el mismo defecto subyacente.
- Un resultado declara un revisor distinto del que se asignó al iniciar la revisión o no identifica proveedor/modelo.
- Un hallazgo carece de identificador local, ubicación, categoría, afirmación, evidencia, clase de evidencia o confianza.
- Una corrección se reenvía después de un timeout, o dos procesos intentan registrar la misma ronda al mismo tiempo.
- Los dos re-juicios discrepan entre `RESOLVED`, `UNRESOLVED` y `REGRESSED`.
- Un target contiene nombres de archivo con espacios, archivos vacíos, binarios o archivos nuevos eliminados antes de comenzar la revisión.
- La configuración de revisión está ausente, parcialmente definida o se escribe junto con otras preferencias del proyecto.
- La revisión no contiene hallazgos y ambos revisores terminaron correctamente.
- Uno de los revisores falla después de una corrección o presenta un digest distinto.
- Se intenta promover memoria después de resolver un hallazgo, pero antes de emitir un veredicto aprobado.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema MUST exigir que cada hallazgo fuente de la ronda activa sea clasificado exactamente una vez.
- **FR-002**: El sistema MUST rechazar referencias duplicadas, desconocidas, cruzadas entre rondas o asignadas a más de una clasificación.
- **FR-003**: El sistema MUST derivar la severidad de una clasificación desde sus hallazgos fuente mediante la severidad más alta aplicable.
- **FR-004**: El sistema MUST impedir un veredicto aprobado mientras exista cualquier hallazgo fuente sin clasificar.
- **FR-005**: El sistema MUST aceptar como idempotente el reenvío exacto de una ronda de consenso y rechazar cualquier reemplazo divergente de esa ronda.
- **FR-006**: El sistema MUST conservar la identidad esperada de ambos revisores y comprobarla contra cada resultado recibido.
- **FR-007**: El sistema MUST rechazar resultados de revisión que omitan cualquiera de los campos obligatorios del hallazgo estructurado.
- **FR-008**: El sistema MUST conservar como target original el artefacto congelado al iniciar la revisión y como target vigente el último artefacto corregido de una cadena válida.
- **FR-009**: La primera corrección MUST partir del target original y cada corrección posterior MUST partir del target corregido por la ronda anterior.
- **FR-010**: El sistema MUST registrar cada ronda de corrección como una transición indivisible que no pueda perderse ni sobrescribirse por concurrencia.
- **FR-011**: Los resultados posteriores a una corrección MUST validarse contra la identidad del target corregido vigente.
- **FR-012**: El sistema MUST registrar los re-juicios por revisor, ronda y hallazgo, incluyendo estado y evidencia verificable.
- **FR-013**: Un hallazgo MUST poder declararse `RESOLVED` únicamente cuando la corrección vigente lo incluya explícitamente y ambos revisores independientes lo consideren resuelto.
- **FR-014**: Si al menos un revisor declara `REGRESSED`, el resultado agregado MUST ser `REGRESSED`; si no existe unanimidad para `RESOLVED`, MUST permanecer `UNRESOLVED`.
- **FR-015**: Toda operación que cambie una revisión MUST validar la transición de estado antes de persistirla.
- **FR-016**: Los estados terminales `APPROVED`, `ESCALATED` e `INCOMPLETE` MUST ser inmutables.
- **FR-017**: El sistema MUST conservar y aplicar la política configurada de máximo de rondas y severidades autorizadas cuando no existan valores explícitos para una revisión.
- **FR-018**: Cada revisión MUST declarar si es de solo lectura o si autoriza la corrección de hallazgos confirmados.
- **FR-019**: Una revisión de solo lectura con un defecto grave confirmado MUST finalizar `ESCALATED` sin requerir una corrección no autorizada.
- **FR-020**: Una revisión autorizada para corregir MUST respetar el presupuesto de rondas y escalar cuando este se agote con defectos graves abiertos.
- **FR-021**: El sistema MUST permitir promover aprendizaje solo desde una revisión `APPROVED` y un hallazgo confirmado, resuelto y reutilizable.
- **FR-022**: El resumen de estado MUST incluir cantidades de hallazgos por clasificación y estado de re-juicio.
- **FR-023**: El detalle de una revisión MUST permitir recorrer hallazgos fuente, consenso, correcciones, verificaciones, re-juicios y veredicto final.
- **FR-024**: Las métricas finales MUST incluir duración, total de hallazgos, confirmados, sospechosos, contradicciones, rondas de corrección y resultados de promoción/deduplicación, usando los nombres publicados.
- **FR-025**: La congelación de cambios pendientes MUST incluir cambios preparados, no preparados y archivos nuevos no ignorados mediante una representación determinista.
- **FR-026**: El sistema MUST rechazar la creación de una revisión de cambios cuando el target resultante esté vacío.
- **FR-027**: El ledger MUST conservar redacción de secretos y datos sensibles en todos los nuevos campos de evidencia, métricas y re-juicio.
- **FR-028**: La resolución de un hallazgo MUST conservar el registro original, sus fuentes, su corrección y sus re-juicios; el sistema MUST NOT borrar evidencia para presentar una revisión limpia.
- **FR-029**: La revisión original `acr_96710834-8273-49f3-bd11-42764b2f11d4` MUST poder registrar la corrección de sus hallazgos confirmados y finalizar con esos hallazgos en estado resuelto.
- **FR-030**: El target corregido MUST superar una nueva revisión con dos revisores independientes antes de considerarse validado para entrega.

### Scope Boundaries

- Incluye los defectos confirmados y los hallazgos sospechosos de la revisión `acr_96710834-8273-49f3-bd11-42764b2f11d4`, que deben reproducirse antes de corregirse.
- Incluye los canales públicos de revisión por línea de comandos y MCP, sus contratos y la distribución coherente de resultados.
- Incluye la migración compatible de revisiones existentes y la conservación de todo el ledger previo.
- Excluye borrar o reescribir hallazgos históricos para alterar el resultado de una revisión.
- Excluye ejecutar modelos dentro de GoMemory; los revisores y el actor de corrección siguen siendo externos.
- Excluye commit, push, merge, despliegue o publicación.
- Excluye cambios no relacionados, incluido el cambio local previo en `go.mod`.

### Key Entities

- **Revisión**: Ciclo adversarial completo; conserva identidad, política, revisores esperados, ronda, estado terminal y veredicto.
- **Target congelado**: Representación estable del artefacto que inspeccionan ambos revisores; distingue el original de la revisión corregida vigente.
- **Identidad de revisor**: Rol A/B y datos necesarios para demostrar que el resultado corresponde al revisor asignado.
- **Hallazgo fuente**: Afirmación estructurada de un revisor con severidad, ubicación, categoría, evidencia y confianza.
- **Clasificación de consenso**: Decisión que cubre uno o dos hallazgos fuente y los clasifica como confirmado, sospechoso, contradicción o información.
- **Delta de corrección**: Registro inmutable que conecta target base, target corregido, hallazgos abordados y evidencia de verificación.
- **Re-juicio**: Resultado independiente de un revisor sobre un hallazgo corregido; puede ser resuelto, no resuelto o regresado.
- **Política de revisión**: Define si se autoriza corregir, qué severidades pueden corregirse y cuántas rondas están disponibles.
- **Métricas de revisión**: Resumen auditable de duración, hallazgos, rondas y promoción de aprendizaje.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El 100 % de los intentos de omitir, duplicar o reducir la severidad de un hallazgo son rechazados y ninguno produce `APPROVED`.
- **SC-002**: El 100 % de los hallazgos marcados `RESOLVED` puede trazarse hasta una corrección que los incluya y dos re-juicios independientes concordantes.
- **SC-003**: En 100 ejecuciones concurrentes de registro de una misma ronda, el ledger conserva una única transición válida y no pierde ni sobrescribe correcciones.
- **SC-004**: Los targets formados por cambios preparados, no preparados y archivos nuevos producen identidades reproducibles; modificar cualquiera de esos contenidos cambia la identidad en el 100 % de los casos.
- **SC-005**: Una revisión de solo lectura con un defecto grave confirmado alcanza un estado terminal escalado en una única solicitud de finalización.
- **SC-006**: Un auditor puede reconstruir el recorrido completo de cualquier hallazgo, desde sus fuentes hasta el veredicto, usando una sola consulta de detalle y sin consultar archivos internos.
- **SC-007**: El 100 % de las respuestas públicas de estado y finalización coincide en nombres y contenido con los contratos publicados.
- **SC-008**: Las consultas de estado y finalización completan en menos de 2 segundos para revisiones de hasta 1.000 hallazgos en un entorno local soportado.
- **SC-009**: La revisión original conserva todos sus hallazgos y finaliza con los defectos corregidos en estado `RESOLVED`.
- **SC-010**: Una revisión adversarial nueva, ejecutada por dos revisores independientes sobre el target corregido, finaliza `APPROVED` sin defectos graves confirmados ni contradicciones severas.
- **SC-011**: Todas las pruebas de regresión históricas de ACR y la validación integral del proyecto completan sin fallos atribuibles a esta funcionalidad.

## Assumptions

- La aprobación posterior del plan de implementación autorizará actualizar únicamente las pruebas existentes que hoy exigen el comportamiento defectuoso; el resto se preservará sin cambios.
- Los hallazgos sospechosos se convierten en trabajo correctivo solo después de reproducirse con evidencia determinista o contractual.
- Los valores de configuración explícitos de una revisión tienen precedencia sobre la política predeterminada del proyecto.
- Los archivos ignorados por el control de versiones permanecen fuera del target de cambios pendientes.
- Los hallazgos históricos nunca se eliminan; “quitar los hallazgos” significa resolver su causa, registrar la corrección y superar el re-juicio.
- La revisión original y sus memorias asociadas continúan disponibles durante la implementación y validación.
