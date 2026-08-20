# Feature Specification: Snapshot de optimización de contexto en la interfaz interactiva

**Feature Branch**: `017-context-snapshot-tui`

**Created**: 2026-08-15

**Status**: Superseded by 020-token-usage-benchmark

> Superada el 2026-08-20 por la feature 020: su alcance se absorbe íntegro como segunda
> sección (Snapshot) de la pantalla de uso, junto al benchmark de tokens por sesión, para no
> construir dos pantallas que muestran la misma medición. Todos sus requisitos funcionales
> (FR-001 a FR-008), incluido el de no persistir snapshots, se conservan en la spec 020.

**Input**: User description: "Pantalla de snapshot de optimización de contexto en la TUI (Retrieval → Compression → Token Budget): el usuario quiere, desde la interfaz interactiva, escribir una tarea y un presupuesto de tokens, disparar el mismo proceso de optimización de contexto que ya existe (selección de memorias relevantes, compresión, ajuste al presupuesto), y ver el resultado — tokens antes/después, % de reducción, conteo de elementos por importancia — para esa tarea puntual, sin histórico acumulado entre sesiones."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ver de un vistazo cuánto optimiza el sistema para una tarea concreta (Priority: P1)

Como usuario de la interfaz interactiva, quiero escribir la descripción de una tarea y un
presupuesto de tokens, y ver de inmediato cuánto se redujo el contenido relevante para esa
tarea (tokens antes/después, porcentaje de ahorro, cuántos elementos entraron como críticos,
relevantes u opcionales), sin salir a una terminal aparte.

**Why this priority**: Es el valor central de la funcionalidad — hoy esta información solo es
accesible desde fuera de la interfaz interactiva. Resolver esto entrega el beneficio completo
con el menor alcance posible.

**Independent Test**: Puede probarse completamente escribiendo una tarea y un presupuesto
válidos desde la pantalla nueva y verificando que el resumen de optimización (tokens,
porcentaje, conteo por categoría) aparece en pantalla, sin usar ningún otro comando.

**Acceptance Scenarios**:

1. **Given** el usuario está en la pantalla de snapshot de optimización, **When** escribe una
   tarea no vacía y un presupuesto de tokens entero positivo y confirma, **Then** la pantalla
   muestra tokens antes y después de optimizar, el porcentaje de reducción, los tokens
   ahorrados, y el conteo de elementos incluidos como críticos, relevantes, opcionales,
   duplicados removidos y descartados.
2. **Given** el resultado de un snapshot ya está en pantalla, **When** el usuario vuelve a la
   pantalla anterior, **Then** no queda ningún registro ni rastro de ese snapshot — la próxima
   vez que entre a esta pantalla, empieza desde cero.
3. **Given** el proyecto todavía no tiene ninguna memoria guardada, **When** el usuario corre un
   snapshot para cualquier tarea, **Then** el resultado se muestra igual, con los conteos en
   cero, sin que la operación falle.

---

### User Story 2 - Entender con claridad cuándo el presupuesto es insuficiente (Priority: P2)

Como usuario que está probando distintos presupuestos de tokens, quiero que, si el contenido
imprescindible para mi tarea no cabe en el presupuesto que indiqué, el sistema me lo diga con
un mensaje claro y me deje ajustar el presupuesto o la tarea y volver a intentarlo sin salir de
la pantalla.

**Why this priority**: Es el caso límite más probable al experimentar con presupuestos chicos —
sin un mensaje claro, el usuario no sabría si el snapshot falló por un error o porque el
presupuesto es, en efecto, insuficiente. Depende de que la Historia 1 ya exista.

**Independent Test**: Puede probarse completamente indicando un presupuesto de tokens
deliberadamente muy bajo para una tarea con contenido imprescindible, y verificando que
aparece un mensaje claro (sin jerga técnica) en vez de un resultado parcial o un error crudo,
y que el usuario puede corregir el presupuesto o la tarea sin salir de la pantalla.

**Acceptance Scenarios**:

1. **Given** el usuario indicó un presupuesto de tokens demasiado bajo para el contenido
   imprescindible de la tarea, **When** confirma, **Then** la pantalla muestra un mensaje claro
   explicando que el presupuesto no alcanza para lo imprescindible de esa tarea, sin mostrar un
   resultado parcial ni un error técnico crudo.
2. **Given** ese mensaje está en pantalla, **When** el usuario aumenta el presupuesto o acorta
   la tarea y confirma de nuevo, **Then** el snapshot se ejecuta de nuevo con los nuevos datos,
   sin necesidad de salir de la pantalla.

---

### Edge Cases

- ¿Qué pasa si el usuario intenta confirmar con la tarea vacía o el presupuesto vacío/no
  numérico/cero o negativo? Debe rechazarse con un mensaje de error claro, sin ejecutar nada,
  permaneciendo en la pantalla para corregir.
- ¿Qué pasa si el proyecto no tiene memorias guardadas todavía? El snapshot se ejecuta igual y
  muestra un resultado con los conteos en cero (ver Historia 1, escenario 3).
- ¿Qué pasa si el usuario corre varios snapshots seguidos, uno tras otro, con tareas distintas?
  Cada uno se calcula de forma independiente; el resultado más reciente reemplaza al anterior en
  pantalla, sin mezclarlos ni acumularlos.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE permitir, desde la interfaz interactiva, iniciar un snapshot de
  optimización de contexto indicando una descripción de tarea y un presupuesto de tokens, sin
  salir a una terminal aparte.
- **FR-002**: El sistema DEBE calcular el snapshot aplicando el mismo proceso de selección de
  memorias relevantes, compresión y ajuste al presupuesto que ya usa la funcionalidad de
  optimización de contexto existente, para el proyecto actual.
- **FR-003**: El sistema DEBE mostrar, tras calcular el snapshot, un resumen con: tokens antes y
  después de optimizar, porcentaje de reducción, tokens ahorrados, y el conteo de elementos
  incluidos por categoría (críticos, relevantes, opcionales, duplicados removidos,
  descartados).
- **FR-004**: El sistema DEBE validar, antes de ejecutar el snapshot, que la tarea no esté vacía
  y que el presupuesto de tokens sea un número entero positivo; si no lo son, DEBE mostrar un
  mensaje de error claro y permanecer en la pantalla sin ejecutar nada.
- **FR-005**: Cuando el contenido imprescindible para la tarea exceda el presupuesto indicado,
  el sistema DEBE comunicarlo con un mensaje claro y comprensible (sin errores técnicos crudos),
  y permitir ajustar la tarea o el presupuesto y reintentar sin salir de la pantalla.
- **FR-006**: El sistema NO DEBE conservar, acumular ni persistir snapshots entre ejecuciones —
  cada uno es independiente del anterior y del siguiente.
- **FR-007**: El usuario DEBE poder volver a la pantalla anterior en cualquier momento antes de
  confirmar la ejecución de un snapshot, sin que se ejecute ni se guarde nada.
- **FR-008**: Ejecutar un snapshot desde la interfaz interactiva NO DEBE alterar el
  comportamiento de la funcionalidad de optimización de contexto ya existente fuera de la
  interfaz interactiva.

### Key Entities

- **Snapshot de optimización**: resultado puntual, no persistido, de calcular la optimización
  de contexto para una tarea y un presupuesto de tokens dados. Incluye tokens antes/después,
  porcentaje de reducción, tokens ahorrados, y el conteo de elementos por categoría
  (críticos, relevantes, opcionales, duplicados removidos, descartados).
- **Tarea**: descripción de texto libre, provista por el usuario, que determina qué memorias del
  proyecto son relevantes para ese snapshot.
- **Presupuesto de tokens (del snapshot)**: número entero positivo, provisto por el usuario para
  cada snapshot, que acota el tamaño del resultado optimizado. Es independiente del ajuste de
  "presupuesto" ya existente en la huella de contexto del proyecto (ver Assumptions).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un usuario puede ver el resultado de optimización de contexto para una tarea
  concreta en menos de 30 segundos desde que decide hacerlo, sin salir de la interfaz
  interactiva.
- **SC-002**: El 100% de los intentos con una tarea vacía o un presupuesto no válido (vacío, no
  numérico, cero o negativo) se rechazan con un mensaje claro, sin ejecutar nada.
- **SC-003**: El 100% de los casos en los que el contenido imprescindible no cabe en el
  presupuesto indicado se comunican con un mensaje comprensible que permite reintentar sin salir
  de la pantalla, en vez de un error técnico crudo.
- **SC-004**: Un usuario puede identificar, sin ayuda externa, cuántos tokens se ahorraron y qué
  porcentaje de reducción logró la optimización para su tarea, leyendo únicamente el resultado en
  pantalla.

## Assumptions

- El punto de entrada a esta pantalla es una acción nueva, accesible por atajo de teclado desde
  la pantalla principal de memorias, siguiendo el mismo patrón que las acciones rápidas ya
  existentes ahí (guardar, mantenimiento, optimizar duplicados) — no se agrega como fila del
  menú de Configuración, porque no es un ajuste persistente sino una acción puntual.
- El snapshot muestra únicamente el resumen de estadísticas (tokens, porcentaje, conteo por
  categoría), no el contenido completo de cada elemento incluido — mantiene la pantalla acotada
  a la altura de una terminal. Ver el contenido completo de cada elemento queda fuera de alcance
  de esta funcionalidad.
- El presupuesto de tokens de esta funcionalidad es un parámetro puntual por snapshot,
  independiente del ajuste "Presupuesto get_context" (Budget) ya existente en la huella de
  contexto del proyecto — miden cosas distintas y no se mezclan.
- Esta funcionalidad no introduce ningún histórico ni persistencia nueva: cada snapshot es
  independiente, igual que el comportamiento ya existente de la funcionalidad de optimización de
  contexto que reutiliza.
- El cálculo del snapshot es una operación local del proyecto (sin depender de procesos externos
  lentos), así que no requiere indicadores de progreso prolongados ni ejecución en segundo plano.
