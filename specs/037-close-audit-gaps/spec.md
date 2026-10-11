# Feature Specification: Cierre de brechas de auditoría defensiva

**Feature Branch**: `main` (rama actual; esta invocación no crea una rama)

**Created**: 2026-10-11

**Status**: Draft

**Input**: User description: "realiza la especificación para cerrar las brechas del audits"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Mantener privados los datos de memoria y sesión (Priority: P1)

La persona marca información como privada o aporta accidentalmente una credencial reconocida por la política vigente. Espera conservar el texto público útil y que el contenido protegido no quede guardado ni aparezca al consultar, cerrar una sesión, generar contexto, exportar o recuperar contenido comprimido.

**Why this priority**: H-01 y H-02 son los dos hallazgos de severidad alta. La protección debe cubrir resúmenes, relaciones y diagnósticos, además de memorias.

**Independent Test**: Introducir marcadores privados y credenciales sintéticas por cada entrada afectada en un almacén aislado; inspeccionar registros y salidas. Ningún marcador protegido debe aparecer y el texto público debe conservarse.

**Acceptance Scenarios**:

1. **Given** un resumen con texto público, un bloque privado y una credencial sintética reconocida, **When** se guarda o cierra la sesión desde consola, herramientas del agente o eventos de integración, **Then** el resumen persistido y la respuesta eliminan el bloque privado y ocultan la credencial; el contexto posterior conserva la información pública.
2. **Given** un razonamiento de relación con contenido protegido, **When** se crea, modifica o importa la relación, **Then** su consulta y exportación no revelan ese contenido y mantienen identificadores, tipo y confianza.
3. **Given** un error de integración con contenido protegido, **When** se registra y consulta el diagnóstico, **Then** no revela ese contenido y conserva una explicación útil del fallo.
4. **Given** bloques privados anidados, consecutivos, con distintas mayúsculas o Unicode, **When** se guarda o transforma el contenido, **Then** se elimina toda región privada y se conserva exactamente el texto público exterior; un cierre aislado no convierte el texto público posterior en privado.
5. **Given** una apertura privada sin cierre, **When** se guarda una memoria, procesa un mensaje o comprime una salida, **Then** desde esa apertura hasta el final se trata como privado y no se crea un original recuperable con esa región.
6. **Given** contenido ya depurado y contenido público sin secretos, **When** la protección se aplica nuevamente, **Then** el resultado depurado permanece estable y el contenido público no se altera.
7. **Given** registros históricos todavía sin sanear, **When** se consultan sesiones, relaciones, diagnósticos, contexto o exportaciones, **Then** las salidas aplican la protección sin revelar su contenido privado ni credenciales reconocidas.

---

### User Story 2 - Sanear información histórica sin perder datos públicos (Priority: P1)

La persona administradora necesita revisar qué registros antiguos requieren saneamiento y aplicarlo explícitamente, conservando el historial público y conociendo los límites respecto a copias externas.

**Why this priority**: Corregir nuevas escrituras no cierra H-01 si los datos históricos siguen conteniendo información protegida.

**Independent Test**: Preparar un almacén con registros históricos sintéticos; revisar una vista previa, aplicar el saneamiento y repetirlo. Comparar datos públicos y consultas sin mostrar secretos en el informe.

**Acceptance Scenarios**:

1. **Given** registros históricos afectados, **When** se solicita una vista previa, **Then** se informan categorías, identificadores y cantidades sin mostrar secretos ni modificar registros.
2. **Given** una vista previa revisada, **When** la persona aplica explícitamente el saneamiento, **Then** se eliminan regiones privadas y se ocultan credenciales reconocidas, manteniendo identificadores, relaciones, fechas y texto público.
3. **Given** un almacén saneado, **When** se consultan o exportan sus datos y se repite el saneamiento, **Then** no se revela contenido protegido y la segunda aplicación no produce cambios adicionales.
4. **Given** un fallo, interrupción o escritor simultáneo, **When** se vuelve a abrir el almacén, **Then** los datos públicos permanecen utilizables y cualquier saneamiento parcial se identifica como pendiente, sin corrupción.
5. **Given** archivos auxiliares y copias de recuperación controlados por gomemory, **When** termina el saneamiento, **Then** se registra su tratamiento y la verificación de residuos sin crear respaldos crudos nuevos; las copias externas no controladas quedan identificadas para tratamiento por su responsable.

---

### User Story 3 - Obtener métricas de contexto consistentes (Priority: P2)

La persona solicita contexto repetida o simultáneamente y recibe cifras de emisión y ahorro correspondientes a cada solicitud, sin acumulación accidental ni interferencia.

**Why this priority**: H-03 exagera el ahorro y presenta conflictos entre solicitudes simultáneas al mismo servicio.

**Independent Test**: Con datos, configuración y reloj de prueba constantes, comparar tres solicitudes consecutivas y ocho simultáneas al mismo servicio con solicitudes aisladas equivalentes.

**Acceptance Scenarios**:

1. **Given** memorias y configuración sin cambios, **When** se generan tres contextos consecutivos, **Then** contenido, línea base, emisión y ahorro son iguales en las tres solicitudes.
2. **Given** ocho solicitudes simultáneas al servicio de herramientas del agente, **When** se generan sus contextos, **Then** cada una obtiene contenido y métricas iguales a su equivalente aislado, sin errores ni mezcla entre solicitudes.
3. **Given** una solicitud con descartes y otra sin ellos, **When** se ejecutan sucesiva o simultáneamente, **Then** ninguna hereda los descartes ni cifras de la otra.

---

### User Story 4 - Instalar y actualizar sin aceptar artefactos dañados (Priority: P2)

La persona instala o actualiza gomemory y espera que el artefacto corresponda a la versión elegida y supere la verificación de integridad. Si falla la preparación, el ejecutable anterior sigue disponible. En Unix, otras invocaciones encuentran una versión completa durante la actualización.

**Why this priority**: H-04 puede dejar el comando ausente o incompleto. H-08 permite instalar descargas alteradas por los scripts.

**Independent Test**: Usar descargas y destinos aislados con un ejecutable anterior reconocible; simular alteración, fallos de red, permisos insuficientes, cancelación e invocaciones simultáneas.

**Acceptance Scenarios**:

1. **Given** una instalación por script con versión explícita o selección de la última estable, **When** se descarga el artefacto, **Then** artefacto y comprobante de integridad corresponden a una misma versión concreta y se verifica la integridad antes de extraer, ejecutar o instalar su contenido.
2. **Given** un comprobante ausente, inválido, sin entrada para el artefacto o con entradas duplicadas, integridad incorrecta, fallo de red o falta de capacidad de verificación, **When** se intenta instalar, **Then** se informa fallo, no se ejecuta ni instala la descarga y el ejecutable anterior permanece intacto.
3. **Given** una descarga íntegra sin un ejecutable regular, no vacío y utilizable para la plataforma, **When** se intenta instalar, **Then** se rechaza sin cambiar el destino anterior.
4. **Given** una actualización Unix y otras invocaciones durante preparación y publicación, **When** se completa el reemplazo, **Then** cada invocación encuentra el ejecutable anterior o el nuevo completo, sin intervalos de ausencia o contenido parcial.
5. **Given** cancelación o fallo de preparación o publicación antes de completar el reemplazo Unix, **When** termina la operación, **Then** el ejecutable anterior sigue funcionando y se informa fallo sin declarar éxito.
6. **Given** instalación por script en Windows, **When** se repiten los casos de integridad válida, inválida, ausente, duplicada, fallo de red y permisos insuficientes, **Then** se comprueban en Windows los resultados anteriores, preservando el ejecutable previo ante fallos anteriores a la instalación.

---

### User Story 5 - Recibir compresión efectiva y liberar la integración OpenCode (Priority: P2)

La persona activa la compresión de salidas en OpenCode v2 y espera que el agente reciba el texto reducido cuando la política vigente obtiene ganancia. Una activación fallida o el cierre del plugin deben liberar sus recursos y respetar sesiones ajenas.

**Why this priority**: H-05 descarta la compresión y H-06 mantiene recursos después de una activación fallida.

**Independent Test**: Observar el resultado recibido por el consumidor en OpenCode real. Por separado, fallar cada etapa de activación y cerrar con un evento en curso, comprobando recursos pendientes.

**Acceptance Scenarios**:

1. **Given** una sesión propia con compresión activada y salida elegible con ganancia, **When** termina la herramienta, **Then** el consumidor en OpenCode real recibe el texto reducido y puede recuperar contenido público mediante las referencias emitidas.
2. **Given** texto simple, varias partes textuales intercaladas con imágenes u otras partes, o un campo de salida textual, **When** se reduce el resultado, **Then** se actualiza todo su texto sin copias redundantes, conservando contenido, orden relativo y metadatos de las partes no textuales y los metadatos del texto retenido.
3. **Given** compresión desactivada, salida sin ganancia, contenido protegido por las reglas de preservación vigentes, resultado no completado, sesión ajena o propietario no resuelto, **When** se procesa la herramienta, **Then** el resultado permanece intacto y no se atribuye ahorro inexistente.
4. **Given** fallo en cualquiera de los registros de activación, **When** falla el inicio, **Then** se termina la suscripción propia, se liberan registros previos y el fallo permanece observable sin cerrar una sesión de memoria ajena.
5. **Given** una integración activa y un evento en curso, **When** se solicita su cierre, **Then** no se inician nuevas operaciones, el cierre termina dentro de 5 segundos en el entorno de validación y no queda actividad propia pendiente al informar éxito.
6. **Given** un fallo de liberación o rechazo de la suscripción, **When** se libera la integración una o varias veces, **Then** se intenta liberar los demás recursos, no se duplican operaciones de cierre y los recursos que no se puedan cerrar se informan como fallo depurado.

---

### User Story 6 - Distinguir ejecución real de un fallo de arranque (Priority: P2)

La persona ejecuta un comando mediante `mem wrap` y usa su resultado para decidir si continuar una automatización. Si no pudo iniciarse, recibe un fallo claro sin diálogo de guardado.

**Why this priority**: H-07 convierte errores de arranque en éxito y permite continuar automatizaciones equivocadamente.

**Independent Test**: Ejecutar con entrada cerrada un comando inexistente, uno sin permisos, uno exitoso y uno que termina con un código no cero.

**Acceptance Scenarios**:

1. **Given** un comando ausente o sin permiso de ejecución, **When** se invoca mediante `mem wrap`, **Then** termina con código distinto de cero, comunica que no pudo iniciarse con una causa útil y no solicita guardar una memoria del resultado.
2. **Given** un comando que sí se inicia, **When** termina con éxito o código no cero, **Then** `mem wrap` conserva su salida y código de terminación y mantiene el comportamiento de sesión y guardado vigente para comandos ejecutados.

---

### User Story 7 - Confiar en las regresiones que permiten cerrar la auditoría (Priority: P3)

La persona que mantiene gomemory necesita pruebas del contrato vigente y evidencia por hallazgo. Una simulación satisfactoria no debe confundirse con validación real del host o de la plataforma.

**Why this priority**: H-09 rompe dos pruebas y debilita la simulación de errores; las verificaciones incompletas no deben declararse correcciones demostradas.

**Independent Test**: Ejecutar la validación automatizada y revisar un registro que relacione los nueve hallazgos con requisitos, escenarios y evidencia reproducible.

**Acceptance Scenarios**:

1. **Given** un inicio de sesión con identificador de conversación, **When** se ejecutan las regresiones del plugin, **Then** se verifican comando, identificador y proyecto; el fallo simulado de inicio se activa aunque existan argumentos adicionales.
2. **Given** una aserción fallida del plugin, **When** termina el caso, **Then** se liberan sus recursos y no contaminan otras pruebas.
3. **Given** una propuesta de cierre, **When** se ejecuta la validación automatizada, **Then** se incluyen pruebas del plugin, núcleo e instalador y no se omiten pruebas para obtener éxito.
4. **Given** falta de Windows, de OpenCode real o de un runtime soportado, **When** se registra el resultado, **Then** se distingue pendiente de verificación de cerrado y se indica cómo completar la evidencia. Un runtime incompatible se detecta antes de ejecutar sus scripts y se informa de forma diferenciada.

### Edge Cases

- Bloques vacíos, anidamiento múltiple, apertura al final, etiquetas con mayúsculas, Unicode y cierres sin apertura.
- Credencial reconocida junto a texto público y aplicación repetida de la protección.
- Sesiones cerradas, relaciones importadas con fechas originales y diagnósticos históricos exportados.
- Saneamiento sin registros afectados, interrupción, permisos insuficientes, escritores simultáneos y copias externas inaccesibles.
- Contextos repetidos con descartes distintos y solicitudes simultáneas desde una misma instancia.
- Cambio de última versión entre descargas, comprobantes ambiguos, artefactos vacíos y cancelación de actualización.
- Lanzamientos concurrentes durante reemplazo y diferencias de actualización Windows.
- Resultados sin texto, mixtos, con varias partes textuales, sesiones ajenas o propietario no resuelto.
- Fallo en cada registro del plugin, cierre repetido, evento en curso, error de liberación y rechazo de suscripción.
- Entrada cerrada para `mem wrap` y un hijo iniciado que termina con error.
- Runtime incompatible: se informa incompatibilidad sin atribuir un defecto a producción sin evidencia.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE aplicar la política vigente de eliminación de bloques privados y ocultamiento de credenciales reconocidas a nuevas escrituras y modificaciones de memorias, resúmenes, razonamientos de relaciones y diagnósticos de integración, incluidas las importaciones afectadas.
- **FR-002**: Consola, herramientas del agente e integraciones automáticas DEBEN ofrecer la misma protección. Sus respuestas, consultas, contextos y exportaciones NO DEBEN reemitir contenido protegido de esos campos, incluidos registros anteriores al cambio.
- **FR-003**: La protección DEBE reconocer etiquetas privadas sin distinguir mayúsculas, eliminar regiones anidadas completas y tratar una apertura sin cierre como privada hasta el final; DEBE conservar texto público exterior y mantener estable una segunda aplicación.
- **FR-004**: El procesamiento de mensajes y la compresión DEBEN reconocer regiones privadas incompletas y NO DEBEN guardar ni ofrecer originales recuperables con contenido privado.
- **FR-005**: Los diagnósticos DEBEN conservar una causa útil y metadatos para identificar el fallo sin revelar bloques privados ni credenciales reconocidas.
- **FR-006**: La persona administradora DEBE poder revisar una vista previa del saneamiento histórico de resúmenes, razonamientos y diagnósticos por categoría, identificador y cantidad, sin mostrar contenido protegido ni modificar datos.
- **FR-007**: El saneamiento DEBE aplicarse solo por acción explícita, preservar texto público, identificadores, relaciones y fechas, ser idempotente y mantener un almacén válido ante interrupciones o escritores simultáneos. Un saneamiento parcial DEBE quedar identificado como pendiente.
- **FR-008**: El cierre del saneamiento DEBE registrar tratamiento y verificación de residuos en archivos auxiliares y copias controladas, sin generar respaldos crudos nuevos, y documentar acciones pendientes para copias externas y límites de eliminación física.
- **FR-009**: Cada solicitud de contexto DEBE calcular descartes, línea base, emisión y ahorro únicamente con datos de esa solicitud.
- **FR-010**: Con entradas, configuración y referencias temporales iguales, las solicitudes repetidas o simultáneas DEBEN producir contenido y métricas iguales a solicitudes aisladas, sin conflictos de acceso concurrente ni contaminación entre solicitudes.
- **FR-011**: La actualización Unix DEBE mantener visible y utilizable un ejecutable completo durante preparación y publicación; las invocaciones concurrentes DEBEN encontrar la versión anterior o la nueva completa.
- **FR-012**: Cancelación o fallo antes de publicar una actualización Unix DEBE conservar el ejecutable anterior y reportar fallo; una publicación satisfactoria DEBE dejar la versión nueva utilizable.
- **FR-013**: Los instaladores por script DEBEN resolver una única versión concreta y comprobar la integridad del artefacto correspondiente antes de extraer, ejecutar o instalar su contenido en Unix y Windows.
- **FR-014**: Los instaladores DEBEN rechazar comprobantes ausentes, inválidos o ambiguos, integridad incorrecta, fallos de red y falta de capacidad de verificación, sin modificar el ejecutable anterior ni declarar éxito.
- **FR-015**: El artefacto verificado DEBE contener un ejecutable regular, no vacío y utilizable para la plataforma; cualquier fallo de validación o permisos previo a instalarlo DEBE conservar el destino anterior.
- **FR-016**: En OpenCode v2, una reducción con ganancia habilitada por la política vigente DEBE reflejarse en el resultado recibido por el consumidor real, con referencias válidas para recuperar contenido público.
- **FR-017**: La reducción DEBE cubrir las representaciones de texto soportadas y conservar partes no textuales, su orden relativo y metadatos, además de los metadatos del texto retenido; NO DEBE dejar copias redundantes del texto original.
- **FR-018**: El plugin DEBE conservar intactos los resultados sin ganancia, con compresión desactivada, protegidos por las reglas vigentes, no completados, de otro proyecto o de propietario no resuelto; NO DEBE atribuirles ahorro.
- **FR-019**: Una activación fallida DEBE terminar la suscripción propia y liberar registros anteriores, conservando el error y sin cerrar sesiones ajenas.
- **FR-020**: El cierre normal DEBE impedir nuevas operaciones desde su solicitud, resolver actividad en curso y liberar recursos en máximo 5 segundos en el entorno de validación. Solo DEBE declarar éxito cuando no quede actividad propia pendiente.
- **FR-021**: La liberación DEBE ser idempotente y continuar con otros recursos si uno falla; los fallos DEBEN quedar observables con información depurada.
- **FR-022**: `mem wrap` DEBE terminar con código distinto de cero y diagnóstico útil si no puede iniciar el comando, sin solicitar guardar una memoria del resultado.
- **FR-023**: Si el comando se inicia, `mem wrap` DEBE preservar su salida, código de terminación y los comportamientos actuales de sesión y guardado.
- **FR-024**: Las regresiones del plugin DEBEN comprobar identidad de conversación y proyecto, simular fallos de inicio aun con argumentos adicionales y liberar recursos aunque falle una aserción.
- **FR-025**: La validación automatizada DEBE incluir regresiones del plugin, núcleo e instalador y reproducir los hallazgos para comprobar su cierre sin desactivar controles existentes.
- **FR-026**: Cada hallazgo DEBE tener un registro de cierre con requisitos, reproducción, resultados esperado y observado, revisión y entorno. Si falta evidencia requerida en Windows o OpenCode real, DEBE permanecer pendiente de verificación.
- **FR-027**: Los mínimos de runtimes de validación DEBEN declararse y comprobarse antes de ejecutar scripts; las incompatibilidades DEBEN comunicarse de forma diferenciada.

### Key Entities *(include if feature involves data)*

- **Texto protegido**: Región privada o credencial reconocida, distinta del texto público que debe preservarse.
- **Registro histórico afectado**: Resumen, razonamiento o diagnóstico previo, con identidad y fechas que deben preservarse.
- **Informe de saneamiento**: Categorías, identificadores, cantidades, estado y residuos pendientes, sin contenido protegido.
- **Resultado de contexto**: Contenido emitido y métricas de una sola solicitud.
- **Artefacto de instalación**: Descarga de una versión y plataforma, con evidencia de integridad y estado de publicación.
- **Resultado de herramienta**: Texto, referencias, partes no textuales y metadatos recibidos por el consumidor.
- **Instancia de integración**: Recursos y sesiones propios de un proyecto, con estado de activación y cierre.
- **Registro de cierre**: Evidencia que relaciona un hallazgo con su resultado corregido y limitaciones.

### Alcance y trazabilidad

Fuente: [Auditoría defensiva del 2026-10-11](../../docs/audits/2026-10-11-defensive-qa.md), revisión `30528515597292ecfcd42c7c08283cae0fbbbd12`. La revisión actual coincide con esa base. El informe aporta reproducciones y propuestas, no correcciones aplicadas. La consulta de fuente actual mediante el grafo confirma rutas pendientes. Esta especificación no obliga a adoptar literalmente los parches propuestos.

| Hallazgo | Resultado de cierre | Requisitos | Escenarios |
| --- | --- | --- | --- |
| H-01 — Redacción incompleta | Nuevas entradas, salidas y datos históricos protegidos | FR-001, FR-002, FR-005 a FR-008 | US1.1–3, US1.6–7; US2.1–5 |
| H-02 — Bloques privados | Anidados e incompletos protegidos en guardado, mensajes y compresión | FR-003, FR-004 | US1.4–6 |
| H-03 — Contexto compartido | Contenido y cifras aislados por solicitud | FR-009, FR-010 | US3.1–3 |
| H-04 — Actualización | Ejecutable completo y preservación ante fallo | FR-011, FR-012 | US4.4–5 |
| H-05 — OpenCode sin reducción | Consumidor real recibe reducción sin perder otras partes | FR-016 a FR-018 | US5.1–3 |
| H-06 — Activación parcial | Recursos propios cerrados ante fallo y cierre normal | FR-019 a FR-021 | US5.4–6 |
| H-07 — Éxito falso | Fallo de arranque observable y códigos del hijo preservados | FR-022, FR-023 | US6.1–2 |
| H-08 — Integridad de scripts | Verificación previa, misma versión y destino anterior protegido | FR-013 a FR-015 | US4.1–3, US4.6 |
| H-09 — Pruebas obsoletas | Contrato vigente y errores simulados efectivamente | FR-024, FR-025 | US7.1–3 |
| Evidencia y soporte | Cierre reproducible y limitaciones declaradas | FR-026, FR-027 | US7.3–4 |

Se incluyen los nueve hallazgos, el saneamiento histórico y las verificaciones de cierre señaladas por la auditoría. La matriz de soporte y las regresiones automatizadas se ajustan en lo necesario para ese alcance.

Quedan fuera una reescritura arquitectónica, nuevos detectores de credenciales, cambios de umbrales o reglas de compresión, firma independiente de artefactos, sustitución de las políticas actuales de actualización Windows y una garantía nueva frente a pérdida de energía. Las recomendaciones generales de límites de entrada, normalización global de errores y análisis de dependencias no se convierten en hallazgos adicionales; se conservan los controles existentes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En el 100 % de escenarios de privacidad y saneamiento, ningún marcador privado ni credencial sintética reconocida aparece en nuevas escrituras, consultas o salidas; se conservan texto público e identidades de registros.
- **SC-002**: Tres solicitudes consecutivas y ocho simultáneas con entradas equivalentes entregan contenido y cifras iguales a sus equivalentes aisladas, sin errores ni interferencia.
- **SC-003**: En todos los escenarios de actualización Unix, cada invocación concurrente encuentra una versión completa utilizable y los fallos previos a publicación conservan la versión anterior.
- **SC-004**: Las instalaciones por script Unix y Windows aceptan el artefacto íntegro y rechazan el 100 % de casos inválidos de US4 sin ejecutar contenido no verificado ni modificar la instalación anterior.
- **SC-005**: En OpenCode real, el consumidor recibe texto reducido en cada representación soportada elegible con ganancia; en los casos sin reducción prevista recibe el original intacto. Las partes no textuales conservan contenido, orden relativo y metadatos.
- **SC-006**: Tras cada fallo de activación y cierre satisfactorio quedan cero recursos propios activos y cero operaciones propias pendientes; el cierre normal con evento en curso tarda como máximo 5 segundos en el entorno de validación.
- **SC-007**: El 100 % de comandos que no pueden iniciarse mediante `mem wrap` produce fallo observable sin diálogo; los comandos iniciados conservan código y salida.
- **SC-008**: Los nueve hallazgos tienen requisitos, escenarios y evidencia de cierre identificados. Ninguno se declara cerrado con evidencia requerida pendiente; la validación automatizada incluye plugin, núcleo e instalador sin omisiones introducidas para aprobarla.

## Assumptions

- `audits` refiere al único informe presente al iniciar esta invocación: `docs/audits/2026-10-11-defensive-qa.md`.
- Se conserva la política de reconocimiento de credenciales; se amplía su cobertura y corrige el tratamiento de etiquetas privadas sin prometer detectar cualquier secreto arbitrario.
- El saneamiento requiere acción explícita. Esta invocación escribe especificación y checklist; no modifica producción ni sanea el almacén real.
- Las salidas protegen registros anteriores aun antes del saneamiento. El cierre histórico verifica el almacén y archivos controlados y documenta copias externas; no promete borrar físicamente datos de medios o copias inaccesibles.
- Se exige disponibilidad frente a fallos de proceso durante actualización Unix, sin una garantía nueva frente a pérdida de energía.
- La instalación cubre versiones estables explícitas y selección de la última estable; no agrega versiones preliminares como contrato nuevo.
- Se preservan aislamiento entre proyectos, identidad de conversación y contratos públicos de salida, incluidas salidas automáticas estructuradas.
- La compresión sigue siendo opcional y conserva límites vigentes; las referencias solo recuperan contenido público permitido.
- El límite de cierre de 5 segundos es un criterio elegido para evitar esperas indefinidas, no una medición actual. Se verifica con eventos y fallos controlados; su política de cancelación se define al planificar.
- La evidencia requiere OpenCode real soportado y un entorno Windows; las simulaciones complementan esas comprobaciones sin sustituirlas.
- La auditoría identifica un script que necesita Python 3.12 por su sintaxis. Los mínimos restantes se obtendrán de contratos y configuración al planificar, sin adoptar versiones nuevas innecesariamente.
- Las reproducciones usarán almacenes, destinos y credenciales sintéticas aislados, sin acceder a secretos ni respaldos reales.
- Diseño e implementación corresponden a fases posteriores, guiadas por la constitución y fuente actual.
