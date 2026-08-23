# Feature Specification: Matriz de canales como fuente única

**Feature Branch**: `022-agent-channel-matrix`

**Created**: 2026-08-23

**Status**: Draft

**Input**: Consolidación de cuatro defectos verificados el 2026-08-23 que comparten una misma
forma. Reemplaza el borrador previo de esta carpeta, que mezclaba tres problemas con ciclos de
vida distintos.

## Contexto del problema

gomemory entrega su funcionalidad a cada agente por **varios canales a la vez**: el protocolo de
memoria, la entrada a modo plan, la guardia previa a presentar un plan, el recordatorio por
turno, el registro del servidor, los permisos de las operaciones y los envoltorios nativos. Cada
canal existe en **dos ámbitos**: el del proyecto y el de la persona que usa la máquina.

Eso forma una matriz de `canal × agente × ámbito`. Sobre esa misma matriz operan cinco
actividades distintas del ciclo de vida: instalar, actualizar, desinstalar, retirar artefactos
de versiones anteriores y diagnosticar el estado.

**Hoy cada actividad mantiene su propia lista de la matriz.** Existen catorce declaraciones
independientes de qué artefacto corresponde a qué agente. Solo tres consultan el registro de
capacidades, y una de esas tres lo anula con un filtro fijo a un agente concreto. Las once
restantes no saben unas de otras.

Cuando una lista se actualiza y otra no, aparece un defecto. En una sola sesión se verificaron
cuatro, todos con esa forma:

| Defecto verificado | La actividad que sabía | La actividad que no |
|---|---|---|
| La entrada del servidor en la configuración del proyecto sobrevive a toda desinstalación | Instalar la escribe con el esquema propio de ese agente | Desinstalar solo conocía el esquema de los demás |
| Pedir el registro global de un agente deja tres archivos de otro agente | La lista de agentes con ámbito de usuario incluye tres | El flujo que escribe los hooks fija uno solo |
| Un cambio de interfaz del agente deja el canal de inyección muerto sin aviso | El complemento declara las operaciones que usa | Ninguna lista declara que deban ejercerse |
| Una desinstalación dirigida a un proyecto borró un artefacto compartido por todos | Instalar sabe que ese artefacto es de ámbito de usuario | Desinstalar no distinguía el ámbito |

El cuarto ocurrió durante la propia corrección del primero, y llegó a producir daño real: la
batería de pruebas eliminó el complemento instalado en la máquina de quien la ejecutó.

**El diagnóstico es que no son cuatro defectos, sino uno estructural manifestándose cuatro
veces.** Repararlos de a uno no cierra la fuente: mientras la matriz no tenga una declaración
única de la que todas las actividades se deriven, cada agente nuevo, cada canal nuevo y cada
cambio de ámbito abre celdas que unas actividades conocen y otras no.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Que una celda sin declarar no pueda pasar desapercibida (Priority: P1)

Quien añade un canal, un agente o un ámbito necesita que el sistema le exija completar la
matriz. Hoy puede añadir la mitad y el resultado compila, pasa las pruebas y se publica; el
hueco aparece semanas después como un defecto reportado por alguien más.

**Why this priority**: es la única historia que cierra la fuente. Sin ella, las demás corrigen
las celdas conocidas hoy y no impiden las de mañana.

**Independent Test**: declarar un agente nuevo con un canal y un ámbito, sin completar las
actividades del ciclo de vida, y comprobar que la verificación falla nombrando lo que falta.

**Acceptance Scenarios**:

1. **Given** un agente declarado con un canal en un ámbito, **When** una actividad del ciclo de
   vida no tiene definido qué hacer con esa celda, **Then** la verificación falla indicando la
   celda y la actividad incompletas.
2. **Given** una celda que deliberadamente no aplica a un agente, **When** se declara el motivo,
   **Then** la verificación pasa y el motivo queda disponible para el diagnóstico.
3. **Given** una celda sin definición y sin motivo declarado, **When** se ejecuta la
   verificación, **Then** falla. La ausencia silenciosa no es un estado válido.
4. **Given** un agente que se retira del registro, **When** se ejecuta la verificación, **Then**
   ninguna actividad conserva referencias a celdas que ya no existen.

---

### User Story 2 - Que lo instalado sea exactamente lo desinstalado (Priority: P1)

Quien desinstala gomemory de un proyecto espera que se retire lo que la instalación puso, sin
residuos y sin tocar nada más. Hoy la correspondencia entre ambas actividades se mantiene a
mano, y ya divergió: hay artefactos que la instalación escribe y ninguna desinstalación retira.

**Why this priority**: es el defecto con impacto directo y verificado sobre quien usa el
producto: deja configuración huérfana apuntando a operaciones que ya no existen.

**Independent Test**: instalar en un proyecto limpio, inventariar lo escrito, desinstalar e
inventariar de nuevo. La diferencia debe ser vacía en el ámbito del proyecto.

**Acceptance Scenarios**:

1. **Given** un proyecto donde se instaló para todos los agentes, **When** se desinstala,
   **Then** no queda en el proyecto ningún artefacto que la instalación hubiera escrito.
2. **Given** un archivo de configuración que además registra servidores ajenos a gomemory,
   **When** se desinstala, **Then** solo se retira lo de gomemory y lo ajeno queda intacto.
3. **Given** un agente cuya configuración usa un esquema distinto al de los demás, **When** se
   desinstala, **Then** su entrada se retira igual que la del resto.
4. **Given** un artefacto que quedó de una versión anterior, **When** se desinstala, **Then**
   también se retira, sin que ello dependa de recordar su nombre en una lista aparte.

---

### User Story 3 - Que una actividad de proyecto no alcance el ámbito de la persona (Priority: P1)

Quien ejecuta una operación sobre un proyecto espera que sus efectos queden dentro de ese
proyecto. Un artefacto de ámbito de usuario lo comparten todos los proyectos de la máquina:
retirarlo desde uno deja sin funcionamiento a los demás.

**Why this priority**: comparte P1 porque su incumplimiento produce pérdida de datos fuera del
alcance declarado, y ya ocurrió.

**Independent Test**: ejecutar cada actividad de ciclo de vida con alcance de proyecto sobre un
directorio temporal y comprobar que el directorio de la persona queda sin modificar.

**Acceptance Scenarios**:

1. **Given** una actividad cuyo alcance declarado es un proyecto, **When** se ejecuta, **Then**
   no crea, modifica ni elimina ningún artefacto de ámbito de usuario.
2. **Given** un artefacto de ámbito de usuario que gomemory instaló, **When** se desinstala del
   proyecto, **Then** se informa su existencia, su ámbito y cómo retirarlo, en lugar de tocarlo.
3. **Given** una actividad cuyo alcance declarado es la persona, **When** se ejecuta, **Then**
   sí puede operar sobre artefactos de ámbito de usuario.
4. **Given** una actividad de alcance de proyecto ejercida desde una prueba automatizada,
   **When** se ejecuta la batería completa, **Then** el entorno de quien la ejecuta queda sin
   modificar.

---

### User Story 4 - Recibir solo lo del agente que se pidió (Priority: P2)

Quien registra gomemory para un agente concreto espera que solo se toque la configuración de ese
agente. Hoy, pedir el registro de ámbito global para un agente deja tres archivos de otro.

**Why this priority**: es un defecto reproducido y acotado. Va en P2 porque su corrección puede
adelantarse por el carril de reparación sin esperar a esta feature; se especifica aquí para que
quede fijada la regresión y para que la matriz la absorba.

**Independent Test**: en un directorio de persona sin configuración previa, pedir el registro de
un solo agente y comprobar el inventario resultante.

**Acceptance Scenarios**:

1. **Given** un directorio de persona sin configuración previa, **When** se pide el registro de
   ámbito global únicamente para un agente, **Then** solo se crean artefactos de ese agente.
2. **Given** una petición para varios agentes, **When** se ejecuta, **Then** cada agente pedido
   recibe su configuración y ninguno más.
3. **Given** un agente que no fue pedido, **When** termina la operación, **Then** su directorio
   de configuración no se ha creado.

---

### Edge Cases

- Un agente declara un canal pero la persona nunca lo usó: la celda vacía es un estado normal,
  no un incumplimiento.
- Dos agentes comparten el mismo nombre de archivo de instrucciones con esquemas distintos: la
  matriz debe distinguir el esquema, no solo la ruta.
- Un artefacto pertenece a dos agentes a la vez: la matriz debe decidir a quién se atribuye, y
  la desinstalación no puede retirarlo mientras el otro lo siga necesitando.
- Un agente se retira del registro pero quedan sus artefactos instalados en máquinas existentes:
  la actividad de retirada de artefactos de versiones anteriores debe seguir cubriéndolos.
- La matriz crece y la verificación empieza a exigir mucho por cada agente nuevo: el costo de
  añadir un agente debe seguir siendo declarativo, no una lista de tareas manuales.
- Un artefacto de ámbito de usuario lo comparten varios proyectos y solo uno desinstala: la
  información entregada debe dejar claro que retirarlo afecta a los demás.

## Requirements *(mandatory)*

### Functional Requirements

**Declaración única (Historia 1)**

- **FR-001**: El sistema DEBE mantener una declaración única de qué artefacto corresponde a cada
  combinación de canal, agente y ámbito.
- **FR-002**: Cada celda de esa declaración DEBE indicar su ámbito, y ese ámbito DEBE ser
  consultable por cualquier actividad del ciclo de vida.
- **FR-003**: Cada actividad del ciclo de vida DEBE derivar de esa declaración qué artefactos le
  corresponden, en lugar de mantener su propia lista.
- **FR-004**: Una celda que deliberadamente no aplique a un agente DEBE declararlo con su
  motivo. Un motivo declarado es un estado válido; una ausencia sin motivo no lo es.
- **FR-005**: La verificación DEBE fallar cuando exista una celda que alguna actividad del ciclo
  de vida no cubra ni por definición ni por motivo declarado, nombrando la celda y la actividad.
- **FR-006**: Añadir un agente DEBE consistir en añadir sus filas a la declaración. Ninguna
  actividad del ciclo de vida DEBE requerir edición adicional para cubrirlo, o bien la
  verificación DEBE exigirla explícitamente.
- **FR-007**: El diagnóstico de estado DEBE derivarse de la misma declaración, de modo que un
  agente nuevo aparezca en él sin trabajo adicional.

**Simetría entre instalar y desinstalar (Historia 2)**

- **FR-008**: Todo artefacto de ámbito de proyecto que la instalación escriba DEBE ser retirado
  por la desinstalación.
- **FR-009**: La desinstalación DEBE conservar íntegro el contenido ajeno a gomemory que
  comparta archivo con lo suyo.
- **FR-010**: La desinstalación DEBE manejar el esquema de configuración propio de cada agente,
  y ese esquema DEBE formar parte de la declaración, no del flujo que la recorre.
- **FR-011**: Los artefactos que generaban versiones anteriores DEBEN retirarse a partir de la
  misma declaración que el resto, no de una lista mantenida aparte.
- **FR-012**: La verificación DEBE comparar el inventario que produce la instalación contra el
  que retira la desinstalación, y fallar ante cualquier diferencia en el ámbito del proyecto.

**Contención del alcance (Historia 3)**

- **FR-013**: Una actividad cuyo alcance declarado sea un proyecto NO DEBE crear, modificar ni
  eliminar artefactos de ámbito de usuario.
- **FR-014**: Cuando exista un artefacto de ámbito de usuario relacionado con una desinstalación
  de proyecto, el sistema DEBE informar su existencia, su ámbito, el efecto de retirarlo sobre
  los demás proyectos, y cómo hacerlo.
- **FR-015**: El alcance de cada actividad DEBE ser explícito en su definición, de forma que la
  contención sea verificable sin leer su implementación.
- **FR-016**: La verificación DEBE ejercer cada actividad de alcance de proyecto y fallar si el
  entorno de la persona resulta modificado.

**Cobertura por agente solicitado (Historia 4)**

- **FR-017**: Toda actividad que reciba una selección de agentes DEBE limitar sus efectos a los
  agentes seleccionados.
- **FR-018**: Ninguna actividad DEBE crear el directorio de configuración de un agente que no
  fue solicitado.
- **FR-019**: La correspondencia entre un agente y el mecanismo que le escribe su configuración
  DEBE formar parte de la declaración única, no estar fijada en el flujo.

### Key Entities

- **Celda de la matriz**: la unidad de la declaración. Une un canal, un agente y un ámbito con
  el artefacto que le corresponde, su esquema cuando aplica, y el motivo por el que no aplica
  cuando ese es el caso.
- **Canal**: la vía por la que gomemory entrega una capacidad al agente — protocolo, entrada a
  modo plan, guardia de plan, recordatorio por turno, registro del servidor, permisos,
  envoltorio nativo.
- **Ámbito**: el alcance de una celda. Determina quién puede tocarla: proyecto o persona.
- **Actividad del ciclo de vida**: instalar, actualizar, desinstalar, retirar artefactos de
  versiones anteriores y diagnosticar. Cada una declara su alcance y se deriva de la matriz.
- **Motivo declarado**: la razón por la que una celda no aplica. Es un dato de la matriz y
  alimenta el diagnóstico; su ausencia hace fallar la verificación.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Añadir un agente al sistema requiere editar una sola declaración; cualquier
  actividad que quede sin cubrir hace fallar la verificación con un mensaje que la nombra.
- **SC-002**: El número de declaraciones independientes de artefactos por agente baja de catorce
  a una, más las traducciones que cada agente necesite, todas derivadas de ella.
- **SC-003**: Instalar en un proyecto limpio y después desinstalar deja cero artefactos de
  gomemory en el ámbito del proyecto, para todos los agentes soportados.
- **SC-004**: Ejecutar la batería completa de pruebas deja el entorno de quien la ejecuta sin
  modificar, comprobado por inventario antes y después.
- **SC-005**: Pedir el registro de ámbito global para un solo agente deja cero artefactos de
  cualquier otro agente.
- **SC-006**: Una celda declarada sin definición y sin motivo hace fallar la verificación, y el
  mensaje permite identificarla sin leer código.
- **SC-007**: El diagnóstico de estado enumera exactamente las celdas de la matriz, sin listas
  propias.

## Assumptions

- El registro de capacidades por agente que ya existe es el punto de partida de la declaración
  única. Esta feature lo extiende con lo que hoy vive disperso, no lo reemplaza.
- Los agentes que solo reciben el registro del servidor, sin canales adicionales, ocupan celdas
  con motivo declarado. No se les añade funcionalidad como parte de esta feature.
- La corrección de la Historia 4 puede adelantarse por el carril de reparación. Su presencia
  aquí fija la regresión y asegura que la matriz la absorba.
- Los defectos verificados citados en el contexto provienen de mediciones del 2026-08-23 sobre
  la versión publicada 2.9.0 y sobre el árbol de trabajo posterior.
- La verificación de la matriz corre con la batería de pruebas habitual y no requiere entorno
  especial.

## Out of Scope

- Añadir agentes nuevos al registro de capacidades.
- Añadir canales nuevos. Esta feature consolida los existentes.
- Medir o reducir el costo en contexto de los canales. Corresponde a la especificación 023.
- Detectar que un canal dejó de ejercerse y reportarlo. Corresponde a la especificación 024.
- Modificar el contenido del protocolo de memoria o del documento de reglas.
