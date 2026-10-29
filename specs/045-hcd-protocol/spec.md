# Feature Specification: HCD — protocolo propio de especificación y desarrollo continuo

**Feature Branch**: `045-hcd-protocol`

**Created**: 2026-10-27

**Status**: Draft

**Input**: User description: "HCD (Harness Continual Development): protocolo propio de GoMemory para conducir una unidad de trabajo (feature, evolución, corrección o mejora técnica) desde su intención hasta su entrega verificada, con Playbook versionado, tareas atómicas, evidencia, memoria de decisiones, invocación desde terminal, TUI y agentes, coexistencia con Spec Kit y migración voluntaria." (propuesta original de la persona: `HCD_Speck.md`, 31 secciones, no versionada; esta spec es autocontenida)

## Contexto y trazabilidad con la propuesta original

- Los identificadores de historias (US1–US8) y criterios (C-01–C-24) se conservan de la propuesta original de la persona (`HCD_Speck.md`, entregada en la conversación del 2026-10-27 y no versionada) para mantener trazabilidad. Esta spec y sus contratos son autocontenidos: ninguna tarea depende de ese documento. Cada escenario de aceptación cita el criterio que materializa.
- El único actor humano es el **Implementador**, que también ejerce de mantenedor. El **Agente de desarrollo** asiste y nunca convierte sus inferencias en decisiones aprobadas.
- La invocación principal es conversacional: `/hcd` (Claude Code, OpenCode) y `$hcd` (Codex), más una secuencia de skills `hcd-*` al estilo de Spec Kit. El motor `mem hcd … --json` lo llaman las skills por dentro; el Implementador no necesita escribirlo.
- Las skills, el motor y las entradas por entorno son **capacidades propuestas**. No se documentarán como disponibles hasta que estén implementadas y verificadas.

## Clarifications

### Session 2026-10-27

- Q: ¿El Implementador debe invocar HCD escribiendo comandos de consola? → A: No. El protocolo se invoca con `/hcd` y `$hcd` y una secuencia de skills `hcd-*` al estilo Spec Kit, con el enfoque del harness continuo; escribir comandos de consola no es la interfaz esperada.
- Q: ¿Quién aplica las reglas del harness detrás de las skills `/hcd`? → A: Las skills llaman por dentro al motor `mem hcd … --json` de GoMemory, que aplica revisiones, idempotencia, atomicidad y la regla «sin evidencia no hay verificado». La CLI queda como motor y respaldo, no como interfaz principal.
- Q: ¿Cómo se resuelve el choque con la constitución §18, que exige artefactos Spec Kit en cada feature? → A: Se corrige la §18 en esta entrega (tarea de D1, con aprobación del Implementador): artefactos por enfoque, `spec/plan/tasks/checklist` para Spec Kit y `definition/delivery/verification` para HCD.
- Q: ¿Dónde se registran los controles y las decisiones? → A: El estado de los controles se guarda internamente en tablas de la base SQLite de GoMemory, y sus relaciones (unidad, historia, criterio, slice, tarea, control, evidencia, decisión) aprovechan el grafo de GoMemory. Las decisiones se registran automáticamente a medida que avanza el protocolo, en el paso que las produce.
- Q: ¿Cómo se distingue una aprobación del Implementador de una del agente? → A: Aprobación conversacional: la skill pide una elección explícita y el motor registra el origen «conversación» con el texto citado de la persona. Se acepta que la garantía se basa en confianza y es auditable; no impide técnicamente que un agente apruebe por su cuenta.
- Q: ¿Cómo sabe cada agente qué unidad está activa, y cómo se registran y recuperan las decisiones entre sesiones? → A: Con una marca interna de unidad activa en la base de GoMemory, nunca en archivos de contexto del agente ni en marcadores como los `SPECKIT` de Spec Kit. Un heartbeat en el fin de turno de Claude Code, Codex y OpenCode (el mismo evento que crea los checkpoints, pero con registro propio, no como memoria `checkpoint`) actualiza la actividad de la unidad. Las decisiones se recuperan al arrancar la sesión y con `/hcd`.
- Q: ¿El Playbook puede tener un valor por defecto, cargarse con GoMemory y actualizarse? → A: Sí. Es un documento fijado como la Constitución (valor por defecto embebido, versión por proyecto, importar, exportar, restaurar y actualización solo si no fue editado). No copia la Constitución: remite a sus secciones.
- Q: ¿El Playbook incluye principios de diseño? → A: Sí: Arquitectura Limpia y SOLID como reglas propias del Playbook, complementarias de la arquitectura hexagonal de la Constitución, con comprobación estructural o de revisión según la regla.
- Q: ¿Qué ocurre si la solicitud no tiene contexto suficiente? → A: El protocolo incluye un paso de aclaración (al estilo `speckit-clarify`) que refina el contexto con preguntas al Implementador, para que los agentes no resuelvan ambigüedades por probabilidad ni tomen decisiones por su cuenta.

### Principios HCD

Estos principios orientan todo el protocolo y son la base de la sección «Principios HCD» del Playbook:

1. **Intención antes que solución:** definir el resultado antes de imponer diseño técnico.
2. **Comportamiento verificable:** cada historia tiene criterios observables.
3. **Entrega incremental:** preparar el siguiente slice sin exigir el diseño completo de toda la unidad.
4. **Atomicidad antes de ejecutar:** no iniciar tareas ambiguas o que agrupen resultados independientes.
5. **Pruebas desde los criterios:** definir la estrategia de verificación antes de implementar.
6. **Evidencia para verificar:** un cambio de código no demuestra cumplimiento.
7. **Memoria con procedencia:** toda decisión persistida identifica su ámbito y origen.
8. **Alcance explícito:** no introducir capacidades por conveniencia técnica.
9. **Bloqueo localizado:** detener solo el trabajo afectado por una incertidumbre y continuar el trabajo independiente autorizado.
10. **Un solo enfoque activo:** no ejecutar HCD y Spec Kit a la vez sobre la misma unidad.
11. **Proporcionalidad:** cada paso obligatorio aclara, decide, limita o verifica.
12. **Continuidad:** retomar no repite automáticamente acciones ya ejecutadas.

### Terminología

| Término | Definición |
|---|---|
| Unidad de trabajo | Feature, evolución, corrección o mejora técnica gestionada por HCD. |
| Enfoque | Protocolo activo de una unidad: `hcd`, `speckit` o `unclassified`. |
| Playbook | Documento versionado que define el protocolo, sus reglas y sus controles. |
| Core HCD | Modelo y reglas de definición, decisiones, trazabilidad, estados y verificaciones. |
| Harness HCD | Componente que conduce el protocolo, calcula las acciones permitidas y aplica los controles. |
| Adaptador | Entrada desde una skill o comando de agente (`/hcd`, `$hcd`, `hcd-*`), la TUI o la CLI que usa el mismo core. |
| Motor HCD | Interfaz `mem hcd … --json` del core. La llaman las skills por dentro; no es la interfaz principal del Implementador. |
| Secuencia de skills HCD | Conjunto de skills `hcd-*` que acompaña cada paso del protocolo, al estilo de las skills `speckit-*`, y delega las transiciones en el motor. |
| Vertical slice | Incremento que entrega un comportamiento comprobable y atraviesa solo las capas necesarias. |
| Tarea atómica | Acción acotada con un resultado principal y una verificación concretos. |
| Evidencia | Registro reproducible o inspeccionable de una prueba, un control o una revisión. |
| Aprobación | Decisión explícita del Implementador sobre un objeto y una revisión determinados. |
| Archivo de contexto del agente | Cualquier archivo de instrucciones o contexto que lee un agente al trabajar en el proyecto, sea cual sea su nombre o agente. HCD no los escribe ni depende de ellos. |
| Documento fijado | Documento que GoMemory trae con un valor por defecto embebido, carga desde la memoria del proyecto si existe una versión propia y permite ver, importar, exportar, restaurar y actualizar (catálogo `PinnedDocs`, como la Constitución). |

## User Scenarios & Testing *(mandatory)*

### User Story 2 - Definir y aprobar una unidad de trabajo (Priority: P1)

Como Implementador, quiero definir features, evoluciones, correcciones y mejoras técnicas con criterios verificables, para delimitar el resultado esperado antes de ejecutar.

**Why this priority**: es la primera mitad del recorrido mínimo (crear → aprobar). Sin una definición aprobada no existe alcance autorizado y ninguna otra historia puede operar.

**Independent Test**: crear una unidad de tipo `fix`, incorporar una definición incompleta, comprobar que sigue en `draft` con los faltantes listados, completarla, validarla y aprobar su revisión exacta.

**Acceptance Scenarios**:

1. **Given** una unidad nueva de cualquier tipo, **When** se presenta una definición sin alguno de los campos mínimos (FR-015), **Then** la validación enumera cada faltante y la unidad permanece en `draft`. *(C-03)*
2. **Given** una definición completa y validada en la revisión R, **When** el Implementador aprueba la revisión R, **Then** la aprobación queda vinculada a R y la unidad pasa a `ready`. *(C-04)*
3. **Given** una unidad aprobada en la revisión R, **When** se modifica su alcance y se genera la revisión R+1, **Then** la aprobación de R no autoriza R+1 y el trabajo que dependa del nuevo alcance queda pendiente de aprobación. *(C-04)*
4. **Given** una unidad de tipo `technical`, **When** se define, **Then** puede describir un resultado observable para un operador, desarrollador o sistema sin inventar una historia comercial.
5. **Given** una definición válida, **When** se ejecuta la validación, **Then** el resultado indica completitud estructural y aclara que no equivale a aprobar su significado ni su alcance.
6. **Given** una definición con una ambigüedad que cambia el alcance, **When** el agente prepara la unidad, **Then** abre el paso de aclaración con preguntas al Implementador en lugar de elegir una interpretación, y la aprobación queda bloqueada hasta registrar la respuesta como decisión.
7. **Given** una revisión mostrada al Implementador, **When** este responde «aprobar» en la conversación, **Then** la aprobación queda registrada con origen «conversación», su texto citado y la revisión exacta; un intento de aprobar sin respuesta citada se rechaza.

---

### User Story 3 - Ejecutar slices mediante tareas atómicas (Priority: P1)

Como Implementador, quiero ejecutar cada slice mediante tareas atómicas, para comprobar cada incremento sin diseñar por adelantado toda la unidad.

**Why this priority**: es el núcleo del valor de HCD (entrega incremental y controlada) y forma parte del recorrido mínimo.

**Independent Test**: sobre una unidad aprobada, registrar un slice con dos tareas (una válida y otra que referencia un criterio inexistente), comprobar que solo la válida puede comenzar, iniciarla y registrar su resultado.

**Acceptance Scenarios**:

1. **Given** una tarea sin vínculo con slice, historia o criterio existentes, **When** se intenta iniciarla, **Then** el inicio se rechaza y se indican las referencias faltantes. *(C-05)*
2. **Given** una tarea con varios resultados independientes o con decisiones funcionales pendientes, **When** se comprueba si puede comenzar, **Then** no se admite como lista para ejecutar y se indica qué condición de atomicidad incumple. *(C-06)*
3. **Given** un slice con alcance y tareas preparados y otros slices todavía sin detallar, **When** se solicita ejecutar ese slice, **Then** se permite sin exigir el detalle de los slices futuros. *(C-07)*
4. **Given** una tarea bloqueada y otra independiente y autorizada, **When** se solicita iniciar la independiente, **Then** puede comenzar.
5. **Given** un conjunto de tareas cuyas dependencias forman un ciclo, **When** se incorporan, **Then** se rechazan e identifican el ciclo.
6. **Given** una tarea iniciada mediante una ejecución gestionada, **When** no hay ejecutor disponible, **Then** se informa esa limitación y la tarea no se presenta como ejecutada.

---

### User Story 6 - Revisar evidencia y cerrar con conocimiento de los límites (Priority: P1)

Como Implementador, quiero revisar la evidencia y las excepciones, para aceptar una entrega conociendo sus límites.

**Why this priority**: cierra el recorrido mínimo (evidencia → verificar → cerrar). Sin esta historia, HCD no puede afirmar cumplimiento.

**Independent Test**: registrar una evidencia para una tarea, verificar el slice, modificar una entrada relevante, comprobar que la verificación queda pendiente de revalidación, revalidar y cerrar la revisión exacta.

**Acceptance Scenarios**:

1. **Given** una tarea implementada sin evidencia suficiente, **When** se intenta verificarla, **Then** conserva el estado `implemented` o pendiente de verificación. *(C-08)*
2. **Given** una verificación vigente, **When** cambia una entrada relevante (definición, código evaluado o revisión normativa), **Then** la verificación afectada se marca como pendiente de revalidación, conserva su historial y no puede justificar el cierre actual. *(C-09)*
3. **Given** un bloqueo crítico, un control obligatorio fallido o una excepción vencida, **When** se intenta cerrar la unidad, **Then** el cierre se impide y se enumeran las causas. *(C-22)*
4. **Given** todos los controles obligatorios satisfechos sobre la revisión R, **When** el Implementador acepta y cierra R, **Then** la unidad pasa a `completed` con la aceptación registrada; una revisión distinta de R no puede cerrarse con esa verificación. *(C-23)*
5. **Given** un informe de verificación con resultado PASS, **When** el Implementador lo consulta, **Then** el informe distingue PASS de aceptación: la entrega no queda aceptada hasta la decisión explícita.

---

### User Story 1 - Seleccionar el enfoque al iniciar una especificación (Priority: P2)

Como Implementador, quiero elegir entre HCD y Spec Kit al iniciar una especificación, para adoptar el nuevo protocolo sin interrumpir el trabajo existente.

**Why this priority**: habilita la coexistencia, pero el recorrido mínimo puede demostrarse antes invocando HCD directamente.

**Independent Test**: iniciar una especificación nueva sin enfoque, elegir cada opción por separado y comprobar que solo se inicializa el protocolo elegido; abrir una unidad Spec Kit existente y comprobar que sus artefactos no cambian.

**Acceptance Scenarios**:

1. **Given** una especificación nueva sin enfoque, **When** el Implementador entra en la ceremonia de Especificación, **Then** se ofrecen «HCD Playbook — recomendado» y «Spec Kit — legado», se elige una sola opción y se inicializa solo ese protocolo; solo la elección de HCD se registra en HCD (FR-001). *(C-01)*
2. **Given** una invocación explícita de inicio de HCD, **When** se ejecuta, **Then** equivale a elegir HCD y no presenta de nuevo la elección.
3. **Given** una unidad Spec Kit existente, **When** se abre después de incorporar HCD, **Then** continúa con Spec Kit, sin migración ni modificación automática de sus artefactos, y la migración aparece solo como acción opcional. *(C-02)*
4. **Given** una unidad cuyo enfoque no puede identificarse, **When** se abre, **Then** se muestra la evidencia encontrada y se solicita su clasificación antes de modificar archivos.
5. **Given** una unidad con enfoque HCD, **When** se ejecuta, **Then** no se exigen artefactos Spec Kit; y una unidad Spec Kit no recibe controles HCD antes de migrarse.

---

### User Story 4 - Retomar con reglas y decisiones persistidas (Priority: P2)

Como Implementador, quiero consultar las reglas y decisiones persistidas, para continuar entre sesiones sin perder contexto.

**Why this priority**: cubre el paso «retomar» del recorrido mínimo y es la diferencia principal frente a reconstruir el contexto desde la conversación.

**Independent Test**: registrar una decisión aceptada, cerrar la sesión, retomar la unidad desde otra sesión y comprobar que se recuperan el estado, la revisión normativa y la decisión, sin repetir ninguna ejecución.

**Acceptance Scenarios**:

1. **Given** una unidad con trabajo en curso, **When** se retoma desde otra sesión o entorno, **Then** se recuperan su estado, sus reglas y sus decisiones vinculadas sin recrear artefactos ni repetir ejecuciones. *(C-10)*
2. **Given** cualquier unidad, **When** se consulta `next`, `status`, `show` o `resume`, **Then** no se ejecutan tareas ni se consumen autorizaciones. *(C-11)*
3. **Given** una decisión aceptada, **When** se registra, **Then** su contexto y su referencia (unidad, revisión, elementos afectados, origen, fecha) se persisten en la memoria de GoMemory, y una propuesta nunca se registra como aprobación. *(C-12)*
4. **Given** un artefacto registrado cuya persistencia en memoria falla, **When** se consulta el estado, **Then** el pendiente de sincronización es visible y un reintento lo completa sin duplicar la decisión. *(C-13)*
5. **Given** dos memorias contradictorias vinculadas a la unidad, **When** se retoma, **Then** el conflicto se presenta con sus fuentes y revisiones, sin asumir que la más reciente tiene razón.
6. **Given** una unidad activa iniciada desde Claude Code, **When** el Implementador abre una sesión nueva en Codex u OpenCode, **Then** el contexto de arranque muestra la sección «HCD activo» con su paso, sus decisiones recientes y su siguiente acción, sin haber escrito ningún archivo de contexto del agente.
7. **Given** una unidad activa, **When** termina un turno en el que el agente modificó archivos sin una ejecución vigente, **Then** el heartbeat registra el desvío y la siguiente sesión lo muestra como aviso, sin crear decisiones ni bloquear el turno.

---

### User Story 8 - El agente recibe solo trabajo autorizado (Priority: P2)

Como agente de desarrollo, quiero recibir la tarea autorizada y las acciones permitidas, para trabajar sin inventar alcance.

**Why this priority**: es la garantía central frente a la ampliación silenciosa del alcance; la necesita cualquier ejecución asistida.

**Independent Test**: solicitar la siguiente acción de una unidad, ejecutar una tarea autorizada y, a mitad del trabajo, intentar registrar una capacidad no contemplada; comprobar que la parte afectada se detiene y queda como decisión pendiente.

**Acceptance Scenarios**:

1. **Given** una tarea autorizada, **When** el agente inicia su ejecución gestionada, **Then** recibe la tarea, sus restricciones, la revisión vigente y una identidad de ejecución.
2. **Given** trabajo en curso, **When** aparece una ampliación de alcance, una excepción no aprobada, una revisión incompatible, una acción fuera de los permisos del entorno o un resultado incierto, **Then** el agente detiene solo la parte afectada y registra un bloqueo o una aclaración sobre ella, que genera una decisión al resolverse.
3. **Given** una ejecución cuya respuesta se pierde, **When** se consulta su estado, **Then** queda como incierta: no se declara éxito ni se repite automáticamente. *(C-19)*
4. **Given** un adaptador que intenta fijar directamente un estado «verificado», **When** envía la mutación, **Then** se rechaza: los estados de cumplimiento se derivan de evidencia y controles.

---

### User Story 5 - Invocar el mismo protocolo desde agentes, TUI y terminal (Priority: P3)

Como Implementador, quiero invocar el mismo protocolo con `/hcd` o `$hcd` desde mi agente, desde la TUI o, como respaldo, desde la terminal, para obtener un comportamiento consistente en cualquier entorno sin escribir comandos de consola.

**Why this priority**: amplía el recorrido mínimo, que se valida primero con un entorno de agente (Claude Code con `/hcd`) apoyado en el motor.

**Independent Test**: realizar la misma transición (iniciar una unidad con `start`; la TUI no aprueba, FR-017b) con `/hcd` en un agente y desde la TUI sobre unidades equivalentes, y comparar las precondiciones aplicadas y el resultado.

**Acceptance Scenarios**:

1. **Given** dos adaptadores distintos, **When** solicitan la misma transición sobre el mismo estado, **Then** aplican las mismas precondiciones y producen el mismo resultado. *(C-16)*
2. **Given** una mutación que declara una revisión esperada antigua, **When** se envía, **Then** se rechaza sin sobrescribir ni perder cambios ajenos. *(C-17)*
3. **Given** una operación ya registrada, **When** se repite con el mismo identificador y el mismo contenido, **Then** devuelve el resultado original sin repetir la mutación; con el mismo identificador y otro contenido, se rechaza. *(C-18)*
4. **Given** una ejecución no interactiva del motor sin argumentos suficientes, **When** se invoca, **Then** muestra ayuda y no elige ni ejecuta acciones implícitas.
5. **Given** un entorno en el que el comando conversacional HCD no está instalado, **When** se consulta la compatibilidad en la documentación (README y guía técnica, que solo listan los entornos con evidencia real, SC-008), **Then** no se afirma soporte y la sintaxis de terminal sigue siendo válida.
6. **Given** la entrada Socrática existente para preparar Spec Kit, **When** se usa después de incorporar HCD, **Then** conserva su comportamiento y no activa HCD.
7. **Given** una skill `hcd-*` invocada por el Implementador, **When** necesita cambiar el estado de la unidad, **Then** delega la transición en el motor y presenta su resultado; si el motor rechaza la transición, la skill informa el rechazo y no la simula editando artefactos.

---

### User Story 7 - Migrar voluntariamente desde Spec Kit (Priority: P4)

Como Implementador, quiero migrar voluntariamente una unidad desde Spec Kit, conservando referencias y decisiones.

**Why this priority**: se incorpora sobre el recorrido mínimo ya comprobado; las unidades legadas pueden seguir con Spec Kit mientras tanto.

**Independent Test**: generar una vista previa de migración de una unidad Spec Kit representativa, comprobar que sigue activa en Spec Kit, aprobar y activar la propuesta exacta y verificar que las fuentes se conservan como referencia legada.

**Acceptance Scenarios**:

1. **Given** una propuesta de migración generada, **When** todavía no ha sido aprobada y activada, **Then** la unidad sigue activa en Spec Kit. *(C-20)*
2. **Given** una propuesta aprobada para la revisión exacta, **When** se activa, **Then** se conservan las fuentes y las correspondencias, y queda un único enfoque activo (HCD). *(C-21)*
3. **Given** una propuesta de migración, **When** se revisa, **Then** cada elemento de origen aparece clasificado como migrado, consolidado, referencia, descartado con justificación o pendiente, y ninguna inferencia aparece como decisión aprobada.
4. **Given** una tarea marcada como terminada en Spec Kit, **When** se migra, **Then** no se convierte automáticamente en verificada en HCD.
5. **Given** fuentes que cambian después de generar la propuesta, **When** se intenta activarla, **Then** la activación exige revisar de nuevo la propuesta.
6. **Given** una activación que falla a mitad, **When** se consulta la unidad, **Then** conserva un único enfoque operativo y permite recuperarse, sin quedar parcialmente migrada.

---

### Edge Cases

- **Playbook actualizado con unidades activas**: las unidades conservan su revisión hasta una adopción explícita que evalúe el impacto. *(C-14)* Una obligación urgente de seguridad genera una revisión explícita de las unidades afectadas, no un cambio silencioso.
- **Playbook y Constitución contradictorios**: un conflicto estructural (FR-013) bloquea la adopción hasta armonizarlo o registrar una excepción §23/§25 válida de la regla superior; una nota de la unidad no basta. Los conflictos de sentido se avisan como revisión técnica. *(C-15)*
- **`not_applicable` usado para ocultar un criterio incumplido**: se rechaza sin una decisión justificada registrada.
- **Excepción vencida**: no satisface el control correspondiente y bloquea el cierre.
- **Evidencia sobre cambios sin commit**: requiere una identificación reproducible del contenido evaluado. Una referencia externa inaccesible no basta para afirmar verificación.
- **Edición externa de los artefactos**: se valida antes de usarla en una transición y nunca se interpreta como aprobación.
- **Resolver un bloqueo**: devuelve el elemento a su estado previo solo si sus condiciones siguen siendo válidas.
- **Tarea demasiado amplia sin información para dividirla**: queda pendiente de definición y abre el paso de aclaración, sin presentarse como atómica.
- **Ambigüedad descubierta durante la ejecución**: el agente detiene solo la tarea afectada, abre la aclaración y continúa con el trabajo independiente autorizado.
- **Aprobación emitida por el agente sin respuesta de la persona**: el motor solo puede detectarla si falta el texto citado. El riesgo residual (un agente que fabrique la cita) se declara en la documentación y queda auditable en el informe por su origen.
- **Entorno solo MCP, sin terminal**: ofrece consulta y asistencia, pero no ejecución HCD gestionada en esta entrega.
- **Memoria no disponible al cerrar**: el cierre exige las decisiones obligatorias persistidas o una excepción explícita para esa indisponibilidad.
- **Secretos**: nunca se guardan en artefactos ni memorias. Una evidencia que contiene un comando no habilita ejecutarlo.

## Requirements *(mandatory)*

### Functional Requirements

**Enfoque y coexistencia**

- **FR-001**: El sistema DEBE ofrecer, al iniciar una especificación nueva sin enfoque, la elección entre «HCD Playbook — recomendado» y «Spec Kit — legado», registrar una sola selección antes de crear artefactos e inicializar solo el protocolo elegido. Elegir HCD se registra como decisión de la unidad creada. Elegir Spec Kit no persiste nada en HCD (no hay unidad HCD donde anclarlo): solo dirige a la entrada de Spec Kit, que sigue su propio flujo.
- **FR-002**: El sistema DEBE registrar el enfoque de cada unidad (`hcd`, `speckit` o `unclassified`) y garantizar un solo enfoque activo por unidad.
- **FR-003**: El sistema DEBE conservar el enfoque Spec Kit de las unidades existentes sin migración ni modificación automática de sus artefactos. La ausencia de metadatos nuevos no convierte una unidad legada en HCD.
- **FR-004**: El sistema DEBE presentar la evidencia encontrada y solicitar clasificación antes de modificar los archivos de una unidad cuyo enfoque no pueda identificarse.
- **FR-005**: El sistema NO DEBE exigir artefactos Spec Kit a unidades HCD ni controles HCD a unidades Spec Kit no migradas.
- **FR-006**: Spec Kit, sus skills y su adaptador secundario DEBEN seguir disponibles y sin modificaciones al entregar esta iniciativa. Única excepción explícita: la copia `.specify/memory/constitution.md` se actualiza al aplicar la corrección aprobada de la §18 (FR-010), por la vía de sincronización ya existente.

**Playbook y Constitución**

- **FR-007**: El sistema DEBE proporcionar el Playbook como **documento fijado** de GoMemory (alias `playbook`), igual que la Constitución:
  - un valor por defecto embebido en el binario;
  - una versión propia del proyecto en su memoria, si existe;
  - consulta, exportación, importación y restauración al valor por defecto desde `mem docs`, `mem playbook` y la TUI;
  - actualización automática al cambiar de versión de GoMemory solo si la copia del proyecto sigue intacta; si fue editada, se avisa y no se toca.

  El Playbook define el protocolo, las condiciones de avance, la atomicidad, la trazabilidad, la memoria, las pruebas y la evidencia, las excepciones y la migración. `hcd/hcd-playbook.md` es una exportación para revisión en git (`--sync`), no la fuente.
- **FR-007a**: El Playbook NO DEBE copiar el texto de la Constitución. Los controles de ingeniería se expresan como una tabla de correspondencia que remite por sección a la Constitución vigente (también un documento fijado). HCD resuelve ambos documentos al usarlos, de modo que un cambio en la Constitución se refleja sin editar el Playbook.
- **FR-007b**: El Playbook DEBE incluir una sección de **principios de diseño** con reglas propias del Playbook (origen `playbook`), cada una con el formato de FR-008:
  - **Arquitectura Limpia**: regla de dependencia (las dependencias apuntan hacia el dominio); entidades y casos de uso independientes de frameworks, base de datos, interfaz y agentes; límites cruzados solo mediante ports; datos que cruzan un límite como estructuras simples, sin tipos de infraestructura.
  - **SOLID**: responsabilidad única (SRP), abierto/cerrado (OCP), sustitución de Liskov (LSP), segregación de interfaces (ISP) e inversión de dependencias (DIP).

  Estas reglas **complementan** la arquitectura hexagonal de la Constitución (§3) y nunca la contradicen ni la relajan; cuando coincidan, la regla del Playbook remite a la sección constitucional en lugar de repetirla (FR-007a). Cada regla declara si su comprobación es estructural (por ejemplo, la regla de dependencia, que ya verifica `tests/contract/layer_rules_test.go`) o de revisión técnica (por ejemplo, SRP u OCP), y su evidencia esperada en la verificación HCD.
- **FR-008**: Cada regla obligatoria del Playbook DEBE tener identificador, descripción, ámbito, forma de comprobación, evidencia esperada, política de excepción y, cuando corresponda, origen normativo.
- **FR-009**: El Playbook DEBE incluir una correspondencia explícita regla constitucional → regla del Playbook → ámbito → control, sin eliminar ni relajar requisitos técnicos.
- **FR-010**: Esta entrega DEBE corregir la §18 de la Constitución para exigir artefactos por enfoque: `spec.md`, `plan.md`, `tasks.md` y `checklists/requirements.md` para unidades Spec Kit, y `definition.md`, `delivery.md` y `verification.md` (generado) para unidades HCD. La corrección es una tarea de D1, requiere la aprobación del Implementador y DEBE estar aprobada antes de crear la primera unidad HCD. No se resuelve con una excepción en la §25.
- **FR-011**: El sistema DEBE aplicar la precedencia: seguridad y cumplimiento → reglas constitucionales → Playbook y extensiones aprobadas → decisiones y excepciones de la unidad → definición aprobada → slices y tareas. Una regla superior solo puede derogarse mediante su mecanismo de excepción. Para reglas de origen constitucional (`§N`), solo cuenta una excepción registrada según la §23 en la sección §25 de la Constitución del proyecto, con su ADR. Las excepciones de HCD (`hcd_exceptions`) aplican únicamente a reglas de origen `playbook` y nunca relajan la Constitución.
- **FR-012**: Cada unidad DEBE fijar la revisión (huella del contenido resuelto) del Playbook y de la Constitución que usa, y el sistema DEBE conservar el contenido de cada revisión fijada por alguna unidad para poder mostrar diferencias. Una revisión nueva de cualquiera de los dos (por importación, restauración o actualización del valor por defecto) NO cambia ninguna unidad activa: solo se aplica a una unidad por **adopción explícita**, tras mostrar las diferencias y su impacto. El motivo y el responsable se registran en ese único momento, la adopción, con la cita del Implementador y por cada unidad que adopta. La importación, la restauración y la actualización del valor por defecto no piden motivo ni responsable.
- **FR-013**: El sistema DEBE detectar y señalar los conflictos estructurales entre el Playbook y la Constitución: una regla del Playbook que cita una sección constitucional inexistente; una regla del Playbook que declara relajar, sustituir o exceptuar una regla `§N`; y texto de la Constitución copiado en el Playbook (FR-007a). Un conflicto estructural **bloquea** la adopción de esa revisión del Playbook (`blocked`) hasta armonizarla o registrar la excepción constitucional correspondiente (§23/§25). Los conflictos de sentido que no se pueden detectar por la estructura se tratan como revisión técnica en `PlaybookValidate` y se avisan sin bloquear.

**Tipos de unidad y definición**

- **FR-014**: El sistema DEBE admitir los tipos de unidad `feature`, `evolution`, `fix` y `technical`, cada uno con su información particular: nuevo comportamiento y actor (`feature`); comportamiento actual, cambio y compatibilidad (`evolution`); fallo observado, reproducción o evidencia, resultado esperado y causa raíz cuando se determine (`fix`); problema técnico observable, beneficiario y condición de mejora verificable (`technical`).
- **FR-015**: Una definición lista para revisión DEBE contener identificador, tipo, título, Implementador, intención, actor beneficiado, resultado esperado, alcance incluido y excluido, restricciones, historias, criterios verificables, estrategia de verificación, decisiones pendientes y referencias normativas. Cada historia DEBE tener al menos un criterio.
- **FR-016**: La validación DEBE enumerar cada faltante y mantener la unidad en `draft` mientras la definición esté incompleta o tenga ambigüedades bloqueantes abiertas.
- **FR-016a**: El protocolo DEBE incluir un paso de aclaración, invocable con su propia skill de la secuencia. Abrir una aclaración cuando la definición, un slice o una tarea contengan ambigüedades que cambien comportamiento, contrato, datos, seguridad o alcance es una **obligación del agente**, escrita en las skills, porque el motor no puede detectar ambigüedades semánticas. El motor aporta el respaldo estructural: la aclaración bloqueante impide aprobar e iniciar (`ATOM-PENDING`), y `validate` señala los campos vacíos o con marcadores de pendiente. La aclaración DEBE plantear al Implementador preguntas acotadas, una a una, con opciones y una recomendación justificada. Cada respuesta se integra en el artefacto correspondiente y se registra como decisión (FR-040b). Mientras quede una ambigüedad bloqueante abierta, la unidad no puede aprobarse y la tarea afectada no puede comenzar. El agente NO DEBE resolver esas ambigüedades eligiendo la interpretación más probable.
- **FR-017**: La aprobación DEBE ser una decisión explícita del Implementador sobre una revisión exacta. Cualquier cambio del contenido de la definición (nueva `definition_rev`) se trata como cambio de alcance y requiere una nueva aprobación, porque el motor no puede juzgar por la estructura si un cambio altera el alcance. No DEBE solicitarse de nuevo mientras se mantengan el objeto, la revisión y el alcance autorizado, y no DEBE autorizar revisiones posteriores con cambios de alcance.
- **FR-017a**: Las aprobaciones (revisión, excepción, cierre, cancelación, clasificación, adopción de revisiones normativas, aceptación o sustitución de decisiones y activación de migración) DEBEN solicitarse en la conversación (o escribirse en la terminal, FR-017b) como una elección explícita sobre el objeto y la revisión mostrados. El motor DEBE registrar el origen (`conversation` o `terminal`, según FR-017b), el texto citado de la respuesta del Implementador, el entorno, la fecha y la revisión. Una aprobación sin respuesta citada se rechaza. El informe de verificación DEBE mostrar el origen de cada aprobación, y la documentación DEBE declarar que esta garantía es auditable pero se basa en la confianza en el agente.
- **FR-017b**: Una aprobación DEBE originarse en la conversación (`origin=conversation`, cuando la invoca una skill con `--client` de agente) o en la terminal de la persona (`origin=terminal`, cuando `--client terminal`). En la terminal, la cita es el texto que la persona escribe en `--quote`, y se valida con las mismas reglas que en la conversación (FR-017a). La TUI no aprueba en esta entrega: inicia unidades y consulta su estado.
- **FR-017c**: Las acciones que exigen cita (FR-017a) enviadas con `--client tui` DEBEN rechazarse con `not_allowed`.

**Protocolo, slices y tareas**

- **FR-018**: El sistema DEBE conducir el protocolo Contexto → Intención → Definición → Aclaración (cuando haya ambigüedades) → Aprobación → Slicing → Descomposición → Ejecución → Verificación → Continuidad → Cierre y calcular, para cada unidad, el paso actual, las comprobaciones pendientes, los bloqueos, las acciones permitidas y la siguiente acción.
- **FR-019**: El sistema DEBE permitir preparar y ejecutar un slice sin exigir el detalle de los slices futuros.
- **FR-020**: Cada slice DEBE indicar identificador, comportamiento observable, historias y criterios, límites, impacto arquitectónico conocido, verificación esperada, incertidumbres, dependencias y estado. Las agrupaciones puramente horizontales solo se admiten como tareas habilitantes vinculadas a un slice.
- **FR-021**: Una tarea DEBE poder comenzar solo si cumple el contrato de atomicidad: verbo y objeto concretos, un resultado principal inspeccionable, forma de comprobarlo, entradas y dependencias, alcance y exclusiones, sin decisiones funcionales pendientes, ejecutable como unidad enfocada, vinculada a un slice y a una justificación, e identificación de reglas y excepciones aplicables.
- **FR-022**: El sistema DEBE rechazar el inicio de tareas huérfanas o con referencias inexistentes, e identificar las referencias faltantes. (Caso particular de FR-021, condición de vinculación; se verifica con el código `ATOM-LINK`.)
- **FR-023**: Las dependencias entre tareas DEBEN formar un grafo sin ciclos. Una tarea bloqueada NO DEBE impedir ejecutar otra independiente y autorizada.
- **FR-024**: Una tarea habilitante DEBE indicar qué slice desbloquea, a qué criterio beneficia indirectamente o qué obligación normativa satisface, y cómo se verifica. No se inventarán historias ni criterios para justificarla.
- **FR-025**: La ejecución gestionada de una tarea DEBE comprobar trazabilidad, dependencias y autorización; vincularse a la revisión vigente; entregar la tarea y sus restricciones al ejecutor; registrar una identidad de ejecución y esperar el reporte del resultado. Sin ejecutor disponible, DEBE informarlo sin presentar la tarea como ejecutada. Hay ejecutor disponible cuando `run` se invoca con un `--client` de agente reconocido (`claude`, `codex`, `opencode`), que es quien ejecutará la tarea; con `--client terminal` o sin cliente, `run` responde `no_executor` y no concede la ejecución.
- **FR-026**: Una ejecución cuya respuesta se pierda DEBE quedar en estado incierto hasta reconciliarse, sin declararse exitosa ni repetirse automáticamente.

**Estados**

- **FR-027**: El sistema DEBE gestionar los estados de la unidad (`draft → ready → in_progress → verification → completed`, más `blocked`, `cancelled` y `superseded`) y de historias, criterios, slices, tareas, excepciones y migraciones, según la tabla de transiciones y los estados de `data-model.md` §1–§2.
- **FR-027a**: El sistema DEBE permitir cancelar una unidad no terminal con una cita del Implementador (`cancel`); la unidad pasa a `cancelled`, o a `superseded` si se indica la unidad que la reemplaza (`--superseded-by`), y en ambos casos libera la unidad activa (FR-041a).
- **FR-028**: Los estados agregados de cumplimiento DEBEN derivarse de los elementos relacionados y no podrán fijarse directamente desde un adaptador. `covered` significa que existe una estrategia de comprobación vinculada, no que la prueba haya pasado. `not_applicable` exige una decisión justificada.

**Evidencia y verificación**

- **FR-029**: Cada criterio DEBE asociarse con al menos una prueba o un control: automatizado, manual reproducible, inspección documental, revisión técnica o comprobación del sistema real.
- **FR-030**: Cada evidencia DEBE registrar identificador, unidad y elementos cubiertos, revisión de la definición, identidad del código o artefacto evaluado, procedimiento, entorno relevante, resultado, fecha, ejecutor y referencia al resultado inspeccionable.
- **FR-031**: Sin evidencia suficiente, el sistema DEBE mantener las tareas en `implemented` o pendientes de verificación. (Consecuencia de FR-028: el estado `verified` se deriva de la evidencia; FR-035 aplica la misma regla al cierre.)
- **FR-032**: Cuando cambie una entrada relevante, el sistema DEBE conservar la evidencia histórica, marcar la verificación afectada como pendiente de revalidación e impedir que el resultado anterior justifique el cierre.
- **FR-033**: La verificación DEBE comprobar automáticamente referencias, identificadores, campos requeridos, dependencias, estados, evidencia registrada, revisiones, excepciones y aislamiento de enfoques, y emitir PASS, FAIL o BLOCKED con las acciones requeridas.
- **FR-034**: La vista de progreso DEBE distinguir historias satisfechas, criterios verificados, slices verificados, tareas implementadas y verificadas, controles aprobados y fallidos, bloqueos, excepciones pendientes o vencidas, verificaciones desactualizadas y memorias pendientes de sincronización. Un porcentaje no podrá ocultar incumplimientos.
- **FR-035**: El sistema DEBE impedir el cierre con bloqueos críticos, controles obligatorios fallidos, excepciones vencidas o verificaciones desactualizadas, y permitir al Implementador aceptar y cerrar solo la revisión exacta verificada.

**Alcance y excepciones**

- **FR-036**: El protocolo DEBE exigir una decisión registrada del Implementador antes de ejecutar trabajo que implique una nueva capacidad, un cambio de contrato, una migración de datos no autorizada, una dependencia relevante no contemplada, una desviación arquitectónica, una relajación de seguridad, un refactor amplio o un cambio de enfoque. Como el motor no puede saber por la estructura que un cambio amplía el alcance, esta exigencia es una **obligación del agente**, escrita en las skills (detenerse y abrir `hcd-clarify` o `block`). El motor aporta un respaldo comprobable:
  - el desvío `files_without_execution` del heartbeat (FR-041c);
  - la regla de admisión `ATOM-SCOPE`, que impide iniciar una tarea sin alcance declarado (FR-021);
  - la exigencia de una decisión aceptada para toda tarea que declare `Tipo: scope-change`.
- **FR-037**: Cada excepción DEBE registrar regla afectada, ámbito, motivo, alternativas, riesgos, controles compensatorios, responsable, aprobación y vencimiento o condición de revisión. Una excepción vencida no satisface su control.

**Memoria**

- **FR-038**: El sistema DEBE usar la memoria existente de GoMemory, sin crear un sistema paralelo, para registrar decisiones funcionales y técnicas, restricciones, aclaraciones, excepciones, causas raíz, descubrimientos, cambios de alcance, alternativas rechazadas, resultados significativos de verificación y migraciones. No DEBE duplicar como memorias los eventos rutinarios.
- **FR-039**: Cada registro relevante DEBE poder relacionarse con proyecto, unidad, tipo de unidad, revisión de la definición, identificador de decisión o evento de origen, elementos afectados, estado (`proposed`, `accepted`, `rejected` o `superseded`), fuente y fecha.
- **FR-040**: Las fuentes de verdad DEBEN repartirse así: los artefactos versionados (`definition.md` y `delivery.md`) contienen el contenido redactado (intención, alcance, historias, criterios, slices y tareas); las tablas internas de la base SQLite de GoMemory contienen el estado de control (aprobaciones, ejecuciones, operaciones, resultados de controles, registro de evidencias, decisiones, excepciones, bloqueos y vigencia de las verificaciones); `verification.md` se genera desde ese estado. Una memoria NO DEBE cambiar por sí sola el alcance aprobado, y la memoria de una especificación conservará un resumen fiel con referencia a su revisión, no una copia editable.
- **FR-040a**: Las relaciones entre unidad, historias, criterios, slices, tareas, controles, evidencias, decisiones y memorias DEBEN representarse como aristas tipadas en el estado de control, con la misma forma que las relaciones del grafo de memorias de GoMemory (proyecto, extremos, tipo de relación, fecha y unicidad por par), de modo que la trazabilidad (historia → criterio → slice → tarea → control → evidencia) se recupere por recorrido de esas aristas. Esas aristas se enlazan con el grafo de memorias existente a través de las decisiones, que también son memorias, con relaciones `supersedes` entre ellas. No se duplica en otra estructura lo que ya guarda el grafo de memorias. Una tabla de aristas propia es necesaria porque las relaciones de memorias solo admiten memorias como extremos.
- **FR-040b**: Cada paso del protocolo que produce una decisión (selección de enfoque, aprobación de revisión, resolución de ambigüedad, cambio de alcance, excepción, `not_applicable`, adopción de revisión del Playbook, aceptación o rechazo de la entrega, apertura o resolución de un bloqueo, cancelación o sustitución de la unidad, clasificación de una unidad sin enfoque, activación de migración) DEBE registrarla automáticamente en ese mismo paso, con su estado, procedencia, revisión y elementos afectados, sin requerir un registro manual posterior.
- **FR-040c**: Ningún adaptador ni skill DEBE modificar el estado de control salvo a través del motor (la protección frente a ediciones directas de la base desde fuera de GoMemory queda fuera de alcance). Cuando el contenido de un artefacto versionado cambie respecto de la revisión registrada en el estado de control, el sistema DEBE detectarlo, crear una revisión nueva y aplicar la invalidación de FR-032, sin tratar la edición como aprobación.
- **FR-041**: Al iniciar o retomar, el sistema DEBE identificar el proyecto y la unidad, cargar la definición y su revisión normativa, recuperar solo las decisiones vinculadas y las reglas relevantes, detectar contradicciones y presentar las decisiones pendientes.
- **FR-041a**: El sistema DEBE mantener una marca interna de **unidad activa** por proyecto en la base de GoMemory, compartida por Claude Code, Codex, OpenCode y la TUI. Iniciar o retomar una unidad la fija; cerrarla o cancelarla la libera. HCD NO DEBE escribir ni depender de archivos de contexto del agente, de marcadores como `SPECKIT START/END` ni de un archivo de feature activa en el repositorio. Fuera de la base de GoMemory, HCD solo escribe bajo `hcd/` del proyecto.
- **FR-041b**: Con una unidad activa, el sistema DEBE registrar un **heartbeat** en cada fin de turno del agente y de sus subagentes (los mismos eventos `turn-end` y `subagent-stop` que hoy crean los checkpoints en los tres agentes; los archivos que toque un subagente cuentan igual para los desvíos), haya habido o no actividad de archivos o comandos, con sesión, cliente, paso del protocolo y archivos tocados. El heartbeat se guarda en el estado de control HCD, NO como memoria `checkpoint` (que se deduplica, se recorta a las 5 últimas y no transporta masa). El heartbeat NO DEBE inferir ni registrar decisiones: las decisiones solo las registra el motor en el paso que las produce (FR-040b).
- **FR-041c**: El heartbeat DEBE detectar y dejar visibles, sin bloquear el turno, los desvíos de ceremonia: archivos modificados sin una ejecución `granted` vigente en la unidad activa, ejecuciones `granted` sin `report` al cerrar la sesión y unidades activas sin heartbeat durante más días de los configurados (`hcd_inactive_days`, 7 por defecto); este último desvío se calcula al consultar (`status`, `resume` y contexto de arranque), no en el propio heartbeat. Solo detecta desvíos estructurales: no interpreta el texto de la conversación. Los desvíos aparecen en `status`, en el informe de verificación y en el contexto de arranque como avisos, no como decisiones.
- **FR-041d**: Al arrancar una sesión en cualquiera de los agentes (contexto de arranque y `get_context`), el sistema DEBE incluir una sección **HCD activo** con: unidad, paso, siguiente acción, aclaraciones abiertas, las últimas decisiones aceptadas (ID, resumen y revisión) y los desvíos pendientes, con presupuesto acotado y sin cargar todo el historial (FR-041). `/hcd` sin argumentos retoma la unidad activa.
- **FR-042**: Ante memorias contradictorias, el sistema DEBE consultar sus fuentes y revisiones, conservar la distinción entre propuesta y decisión, usar el mecanismo de juicio de GoMemory cuando corresponda y pedir al Implementador que resuelva los conflictos funcionales que la evidencia no resuelva.
- **FR-043**: Si la persistencia en memoria falla, el sistema DEBE conservar el artefacto, marcar la sincronización como pendiente, permitir un reintento idempotente y no declarar completada la sincronización.
- **FR-044**: El sistema NO DEBE guardar secretos en artefactos ni en memorias.

**Invocación y contrato común**

- **FR-045**: El sistema DEBE ofrecer como entrada principal del Implementador `/hcd` (Claude Code, OpenCode) y `$hcd` (Codex), que identifican la unidad, recuperan su estado y conducen al paso siguiente, más una secuencia de skills `hcd-*` que acompaña cada paso del protocolo (definir, aclarar, aprobar, preparar slice, descomponer tareas, ejecutar, verificar, cerrar, gobernar y migrar), al estilo de las skills `speckit-*`. El Implementador NO DEBE necesitar escribir comandos de consola para recorrer el protocolo.
- **FR-045a**: El sistema DEBE ofrecer un motor `mem hcd` con los comandos de secuencia y de gobierno definidos en `contracts/engine-cli.md` §2, que las skills invocan por dentro. Sigue siendo utilizable como respaldo desde la terminal. En modo no interactivo y sin argumentos suficientes muestra ayuda sin ejecutar acciones implícitas.
- **FR-045b**: Las skills `hcd-*` DEBEN delegar toda transición de estado, aprobación, registro de evidencia, verificación y cierre en el motor. NO DEBEN mutar el estado editando artefactos directamente ni fijar estados de cumplimiento. Si el motor no está disponible, la skill informa la limitación y no simula el avance.
- **FR-046**: Las consultas (`next`, `status`, `show`, `resume`) NO DEBEN ejecutar tareas ni consumir autorizaciones. La única escritura permitida es fijar la unidad activa al hacer `resume <id>` (FR-041a); esa marca no cambia `state_revision`, no consume autorizaciones y no es un cambio de estado de la unidad.
- **FR-047**: El motor DEBE ofrecer a las skills una salida estructurada con `protocol_version`, `unit_id`, `approach`, `revision`, `playbook_revision`, `state`, `stage`, `allowed_actions`, `pending_checks`, `blockers`, `next_action`, `execution_id` y `memory_sync_status`, según corresponda.
- **FR-048**: Las mutaciones sobre unidades existentes DEBEN admitir revisión esperada e identificador de operación. Una revisión desactualizada se rechaza sin sobrescribir; repetir una operación idéntica devuelve su resultado; reutilizar el identificador con otro contenido se rechaza.
- **FR-049**: La TUI DEBE ofrecer la entrada «Especificación → HCD / Spec Kit» y usar los mismos casos de uso que el motor, sin transiciones exclusivas.
- **FR-050**: Las skills y comandos HCD de los agentes DEBEN operar mediante el motor, sin mantener una máquina de estados propia. El soporte de cada entorno (Codex, Claude Code, OpenCode, otros con terminal) solo se declara tras instalarlo y verificarlo en ese entorno real. Los entornos solo MCP ofrecen consulta y asistencia, sin ejecución gestionada.
- **FR-051**: La entrada Socrática existente para preparar Spec Kit DEBE conservar su comportamiento y no activar HCD.

**Artefactos**

- **FR-052**: Cada unidad HCD DEBE tener como mínimo una definición, un registro de entrega (slices, tareas y dependencias) y un informe de verificación generado desde el estado de control, que no sea una fuente manual adicional. Las decisiones, excepciones y evidencias viven en el estado de control y aparecen en el informe generado. Los identificadores y los campos normativos DEBEN tener un formato parseable documentado.
- **FR-053**: Las ediciones externas de los artefactos DEBEN validarse antes de usarse en una transición y nunca interpretarse como aprobación.

**Migración**

- **FR-054**: El sistema DEBE generar, a petición explícita, una propuesta de migración desde Spec Kit sin activar HCD, clasificando cada elemento como migrado, consolidado, referencia, descartado con justificación o pendiente, sin convertir inferencias en decisiones aprobadas.
- **FR-055**: La activación de una migración DEBE requerir la aprobación de la propuesta exacta y de su revisión, exigir una nueva revisión si las fuentes cambiaron, conservar las fuentes como referencia legada y, si falla, dejar un único enfoque operativo recuperable.
- **FR-056**: Las tareas terminadas en Spec Kit NO DEBEN migrarse como verificadas.

### Key Entities

- **Unidad de trabajo**: identificador, tipo, título, enfoque, estado, revisión de la definición, revisiones normativas fijadas e Implementador. Agrupa historias, criterios, slices, tareas, decisiones, excepciones, evidencias y verificaciones.
- **Revisión**: versión identificable de una definición, de un Playbook o de una propuesta de migración. Las aprobaciones, las evidencias y los cierres se vinculan a una revisión exacta.
- **Playbook / Regla**: documento versionado; cada regla tiene identificador, ámbito, comprobación, evidencia esperada, política de excepción y origen normativo.
- **Historia y criterio**: comportamiento esperado de un actor y su condición observable; cada criterio tiene una estrategia de verificación.
- **Vertical slice**: incremento comprobable vinculado a historias y criterios, con límites, verificación esperada y dependencias.
- **Tarea**: acción atómica vinculada a un slice, con resultado, verificación, dependencias, reglas aplicables y estado.
- **Ejecución**: identidad de una ejecución gestionada de una tarea; registra la revisión, el resultado reportado o su incertidumbre.
- **Evidencia**: registro del procedimiento, el resultado y la identidad del contenido evaluado, con vigencia ligada a sus entradas.
- **Decisión**: propuesta, aceptación, rechazo o sustitución con procedencia, vinculada a elementos y revisiones y reflejada en la memoria.
- **Excepción**: desviación aprobada de una regla, con ámbito, controles compensatorios y vencimiento.
- **Bloqueo**: impedimento localizado sobre elementos concretos, con justificación y resolución.
- **Propuesta de migración**: correspondencia clasificada entre los artefactos Spec Kit de origen y los elementos HCD, con estado propio.
- **Operación**: identificador de una mutación para garantizar idempotencia y detectar conflictos.
- **Estado de control**: registro interno en la base de GoMemory de aprobaciones, ejecuciones, operaciones, resultados de controles, evidencias, decisiones, excepciones, bloqueos y vigencias, enlazado por el grafo de GoMemory con los elementos de la unidad y sus memorias. Es la fuente de `verification.md`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Una unidad de tipo `fix` con una historia, un criterio, un slice y una tarea recorre el camino mínimo (crear → aprobar → slice y tareas → ejecutar → evidencia → verificar → retomar → cerrar) usando solo `/hcd` y las skills `hcd-*` desde un agente, con cero comandos de consola escritos por el Implementador y sin producir artefactos Spec Kit.
- **SC-002**: Los seis escenarios de extremo a extremo (feature nueva, corrección reproducible, evolución con compatibilidad, mejora técnica, unidad retomada desde otro entorno y migración desde Spec Kit) se completan con evidencia registrada.
- **SC-003**: El 100 % de los criterios C-01 a C-24 tiene al menos un escenario de prueba vinculado y evidencia de cumplimiento.
- **SC-004**: El 100 % de las unidades Spec Kit existentes en el repositorio en el momento de la entrega se abren y continúan con su enfoque, con cero bytes modificados en sus artefactos.
- **SC-005**: Ninguna consulta (`next`, `status`, `show`, `resume`) cambia el estado, la revisión ni las autorizaciones de una unidad, comprobado comparando el estado antes y después.
- **SC-006**: Repetir cualquier mutación con el mismo identificador y contenido produce una sola mutación registrada en el 100 % de los casos probados; las mutaciones con revisión obsoleta se rechazan en el 100 % de los casos sin pérdida de datos.
- **SC-007**: Retomar una unidad desde una sesión nueva con una sola invocación de `/hcd` (o `$hcd`) presenta el paso actual, las decisiones pendientes y la siguiente acción, sin releer la conversación previa.
- **SC-008**: Cada entorno declarado compatible tiene evidencia de una invocación real; ningún entorno sin esa evidencia figura como compatible en la documentación.
- **SC-009**: Una unidad Spec Kit representativa migra con el 100 % de sus elementos de origen clasificados y cero tareas promovidas automáticamente a verificadas.
- **SC-010**: Para una corrección pequeña, la cantidad de artefactos obligatorios de HCD (definición, entrega y verificación generada) es menor que la de Spec Kit (spec, plan, tasks y checklist).
- **SC-011**: En el 100 % de los arranques de sesión con una unidad activa (Claude Code, Codex y OpenCode), el contexto incluye la sección «HCD activo», y ningún archivo fuera de `hcd/` cambia por causa de HCD, incluidos los archivos de contexto del agente (comprobado con `git status`).

### Criterio de entrega de la iniciativa

- **C-24** (criterio de aceptación de la entrega): al entregar esta feature, Spec Kit, sus skills y `adapters/secondary/speckit` siguen disponibles y sin cambios (FR-006); lo comprueba la tarea final de verificación de Spec Kit.

## Assumptions

- **La «ceremonia de Especificación» no existe hoy como pantalla ni comando.** En el código actual, la especificación entra mediante `socratica_flow(mode=specify)` → `speckit-specify` y las skills `speckit-*`; la TUI solo expone el interruptor «Brazo extensor spec-kit». Esta feature crea el punto de entrada con la elección de enfoque (FR-001, FR-049) y no reutiliza la entrada Socrática (FR-051).
- La Constitución vigente (§18) exige `spec.md`, `plan.md`, `tasks.md` y `checklists/requirements.md` por feature. Esta entrega la corrige (FR-010) con aprobación del Implementador antes de crear la primera unidad HCD. Esta misma feature 045 se gestiona con Spec Kit.
- El estado de control de HCD se guarda en tablas nuevas de la base SQLite que GoMemory ya usa (junto a `memories`, `memory_relations`, `socratica_flows`, etc.), con su migración de esquema correspondiente. Las relaciones reutilizan el grafo de GoMemory (relaciones entre memorias y, cuando aporte, el grafo de código). Ampliar tablas internas no amplía el esquema MCP ni añade herramientas MCP.
- La organización técnica respeta la arquitectura hexagonal vigente; esta especificación define responsabilidades, no una reorganización de paquetes ni un binario independiente.
- La ubicación de los artefactos de cada unidad (`hcd/features/{NNN}-{slug}/definition.md|delivery.md|verification.md`) se define en `data-model.md` §4. El Playbook no vive como fuente en `hcd/`: es un documento fijado de GoMemory (FR-007), y `hcd/hcd-playbook.md` solo existe como exportación opcional para git. El formato de los artefactos, la identificación de revisiones y los contratos de payload, errores y códigos de salida están cerrados en `research.md` (R4, R5), `contracts/artifact-format.md` y `contracts/engine-cli.md`.
- Las pendientes de diseño no bloqueantes del documento original (renombrado de skills Spec Kit, edición concurrente entre agentes más allá de la revisión esperada, evidencia externa, métricas de deprecación) quedan fuera de esta entrega.
- Los cambios de etapa de la transición (HCD predeterminado, deprecación y retirada de Spec Kit) requieren decisiones independientes posteriores; la disponibilidad de HCD no autoriza la retirada.

### Fuera de alcance

- Retirar físicamente Spec Kit o sus skills, o modificar `adapters/secondary/speckit`.
- Migración masiva automática y sincronización con el upstream de Spec Kit.
- Nuevas herramientas MCP, un binario independiente o una reestructuración general de GoMemory.
- Un sistema de memoria paralelo.
- Un plan técnico exhaustivo obligatorio por unidad.
- Automatizar decisiones funcionales.
- Garantizar la ausencia absoluta de defectos.
- Ejecutar comandos por el solo hecho de figurar en una evidencia.
- Impedir cambios realizados fuera de GoMemory (el harness controla las acciones gestionadas).
