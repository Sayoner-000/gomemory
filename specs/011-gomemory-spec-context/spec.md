# Feature Specification: gomemory como brazo extensor de contexto histórico para /speckit

**Feature Branch**: `011-gomemory-spec-context`

**Created**: 2026-08-03

**Status**: Draft

**Input**: User description: "deseo revisar que gomemory se pueda obtener como brazo extensor con /specs para que no se tenga barrer toda las especificacion. Y que se sepa en cada espcifacion de que se ha hecho el proyecto, sin necesidad del scan del directorio, y complemenentando los nodos de code base memory."

## Contexto: qué existe hoy

Hoy, cuando una persona (o un agente) redacta una nueva especificación con
`/speckit-specify`, la única forma de saber qué funcionalidades y decisiones
ya existen en el proyecto es leyendo manualmente los `spec.md` de cada
carpeta bajo `specs/` — un barrido de directorio que crece con cada feature
nueva y que nadie hace de forma sistemática en la práctica.

gomemory ya arma, para otros flujos (arranque de sesión, hooks de agente),
un resumen del proyecto (`get_context`) con memorias recientes, sesiones,
sinapsis (relaciones entre memorias) y — cuando hay un proveedor externo de
grafo de código conectado (p. ej. codebase-memory-mcp) — un resumen agregado
de esa estructura (lenguajes, módulos, hotspots). Ese mecanismo hoy no está
conectado al flujo de `/speckit-specify` ni al resto de comandos de
spec-kit: cada especificación nueva arranca "en blanco" respecto al
historial del proyecto.

Esta especificación cubre conectar ese resumen ya existente al flujo de
creación de especificaciones, para que cada `spec.md` nuevo se redacte con
conocimiento de lo que ya se hizo, sin exigir un barrido manual del
directorio `specs/` y sin duplicar la información estructural que ya aporta
el grafo de código externo.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Contexto histórico automático al crear una especificación (Priority: P1)

Como persona (o agente) que ejecuta `/speckit-specify` para una funcionalidad
nueva, quiero recibir automáticamente un resumen de qué features y
decisiones ya existen en el proyecto — sin tener que abrir manualmente cada
`spec.md` bajo `specs/` — para redactar la nueva especificación con
conocimiento del historial y evitar proponer algo ya implementado o
contradictorio con una decisión previa.

**Por qué esta prioridad**: es el corazón del pedido — hoy el barrido manual
del directorio es el único mecanismo disponible y nadie lo hace de forma
consistente, así que cada especificación nueva corre el riesgo de duplicar
o contradecir trabajo ya hecho.

**Prueba independiente**: se puede probar creando una nueva especificación
en un proyecto con historial previo en gomemory (memorias de tipo
`decision`/`architecture`, especificaciones previas) y verificando que el
resumen de historial aparece disponible antes de que se redacte el borrador,
sin que la persona haya tenido que pedirlo explícitamente ni abrir archivos
de `specs/`.

**Acceptance Scenarios**:

1. **Given** el proyecto tiene memorias y especificaciones previas
   registradas en gomemory, **When** se invoca `/speckit-specify` con una
   descripción de feature nueva, **Then** el flujo incorpora automáticamente
   un resumen de features/decisiones previas relevantes antes de completar
   el borrador de especificación.
2. **Given** el proyecto es nuevo y no tiene memorias guardadas aún,
   **When** se invoca `/speckit-specify`, **Then** el flujo continúa
   normalmente sin el resumen, sin bloquear ni pedir pasos adicionales.
3. **Given** gomemory no está disponible o la consulta de contexto falla,
   **When** se invoca `/speckit-specify`, **Then** el flujo continúa con el
   comportamiento actual (sin el resumen), sin error visible para quien lo
   usa.

---

### User Story 2 - Distinguir historia de decisiones vs. estructura de código (Priority: P1)

Como persona que mantiene el proyecto, quiero que el resumen de historial
que aporta gomemory distinga con claridad qué es "historia y decisiones del
proyecto" (memorias, ADR) y qué es "estructura del código" (nodos y
relaciones del grafo externo tipo codebase-memory), para no leer información
duplicada y saber a qué fuente acudir según lo que necesito verificar.

**Por qué esta prioridad**: sin esta distinción, el resumen mezclaría dos
fuentes de naturaleza distinta (una narrativa/histórica, otra estructural y
verificable contra el código) y la persona no sabría cuál confiar para cada
tipo de pregunta — el pedido original menciona explícitamente
"complementando", no "reemplazando" los nodos de grafo de código.

**Independent Test**: se puede probar revisando el resumen entregado en una
especificación nueva y confirmando que las secciones de historial/decisiones
y la de estructura de código (cuando el proveedor externo está disponible)
están claramente separadas y rotuladas.

**Acceptance Scenarios**:

1. **Given** hay tanto memorias de gomemory como un grafo de código externo
   disponible, **When** se genera el resumen para una especificación nueva,
   **Then** el resumen presenta ambas fuentes en secciones diferenciadas,
   cada una con su origen identificado.
2. **Given** solo gomemory tiene datos (sin proveedor de grafo de código
   conectado), **When** se genera el resumen, **Then** el resumen incluye
   únicamente la sección de historial/decisiones, sin mencionar una fuente
   estructural que no está disponible.

---

### User Story 3 - Mismo contexto disponible en planificación y aclaración (Priority: P3)

Como persona que continúa una especificación ya creada mediante
`/speckit-plan` o `/speckit-clarify`, quiero que el mismo resumen de
historial esté disponible en esas fases, para no perder la ventaja obtenida
al inicio si esas fases ocurren en una sesión distinta a la de
`/speckit-specify`.

**Por qué esta prioridad**: es una extensión natural del mismo mecanismo,
pero de menor impacto que la Historia 1 — en la mayoría de los casos
`/speckit-plan` y `/speckit-clarify` ocurren en la misma sesión donde ya se
cargó el contexto al hacer `/speckit-specify`.

**Independent Test**: se puede probar invocando `/speckit-plan` o
`/speckit-clarify` en una sesión nueva (sin el contexto ya cargado por
`/speckit-specify`) y verificando que el resumen de historial también está
disponible ahí.

**Acceptance Scenarios**:

1. **Given** una especificación ya existe y se abre una sesión nueva,
   **When** se invoca `/speckit-plan` o `/speckit-clarify` sobre ella,
   **Then** el resumen de historial del proyecto está disponible igual que
   en `/speckit-specify`.

---

### User Story 4 - Encendido/apagado del brazo extensor sin depender de spec-kit (Priority: P2)

Como persona que usa gomemory en un proyecto que **no** tiene spec-kit
instalado, o que simplemente no quiere que gomemory intente actuar como
brazo extensor de especificaciones, quiero poder activar o desactivar esta
integración desde la propia TUI de gomemory, para que gomemory no intente
detectar ni conectarse a spec-kit cuando no aplica, sin tener que tocar
configuración de spec-kit para lograrlo.

**Por qué esta prioridad**: la integración depende de que el proyecto tenga
spec-kit instalado (`.specify/`), algo que no es cierto para todos los
proyectos donde corre gomemory; sin un apagador propio, gomemory tendría que
adivinar en cada arranque si debe intentar la integración, igual que ya
resuelve hoy para el proveedor externo de grafo de código
(`CodeGraphDisabled`).

**Independent Test**: se puede probar entrando a la pantalla de
configuración de la TUI, alternando el interruptor de esta integración, y
verificando que el estado persiste entre sesiones y que gomemory respeta esa
elección (no intenta el brazo extensor si está apagado, aunque `.specify/`
exista).

**Acceptance Scenarios**:

1. **Given** un proyecto sin `.specify/` (sin spec-kit instalado), **When**
   gomemory arranca, **Then** la integración permanece inactiva por defecto,
   sin errores ni intentos de conexión visibles.
2. **Given** un proyecto con spec-kit instalado, **When** la persona apaga
   el interruptor en la TUI, **Then** gomemory deja de actuar como brazo
   extensor para ese proyecto hasta que se vuelva a activar, aunque
   `.specify/` siga presente.
3. **Given** la persona activa el interruptor en un proyecto con spec-kit
   instalado, **When** se invoca `/speckit-specify`, **Then** el resumen de
   historial (Historia 1) vuelve a incorporarse normalmente.

---

### Edge Cases

- ¿Qué pasa cuando el proyecto acumula cientos de memorias y decenas de
  especificaciones previas? El resumen debe acotarse a lo más reciente o
  relevante, nunca volcar el historial completo en cada especificación
  nueva.
- ¿Qué pasa si gomemory registra una feature como "hecha" pero el código
  actual ya no la tiene (memoria desactualizada, feature revertida)? El
  resumen se presenta como referencia a verificar, no como verdad absoluta
  del estado actual del código.
- ¿Qué pasa si tanto gomemory como el proveedor externo de grafo de código
  están desconectados al mismo tiempo? La creación de la especificación
  continúa exactamente igual que hoy, sin ninguna de las dos secciones de
  resumen.
- ¿Qué pasa si el resumen de historial y el resumen del grafo de código
  parecen contradecirse entre sí sobre una misma parte del proyecto? Ambas
  secciones se muestran igual, rotuladas por su origen; no se intenta
  reconciliar automáticamente — la persona decide cuál verificar.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El flujo de `/speckit-specify` DEBE incorporar automáticamente
  un resumen del historial del proyecto (features previas registradas,
  decisiones arquitectónicas) antes de completar el borrador de la
  especificación, sin que la persona tenga que solicitarlo explícitamente.
- **FR-002**: El resumen de historial DEBE poder obtenerse sin leer ni
  recorrer manualmente el contenido de los archivos `spec.md` existentes
  bajo `specs/`.
- **FR-003**: El resumen DEBE distinguir, en secciones separadas y
  rotuladas por origen, la información que proviene del historial de
  decisiones del proyecto (gomemory) de la que proviene de la estructura de
  código (grafo externo tipo codebase-memory).
- **FR-004**: Si gomemory no está disponible, no tiene datos guardados, o la
  consulta del resumen falla, el flujo de creación de especificación DEBE
  continuar sin el resumen y sin mostrar un error a la persona
  (degradación transparente).
- **FR-005**: El resumen incorporado DEBE mantenerse acotado a un tamaño
  manejable (priorizado/resumido) en vez de volcar el historial completo del
  proyecto, para no saturar la especificación nueva con contenido
  irrelevante.
- **FR-006**: El mismo mecanismo de resumen de historial DEBE poder
  reutilizarse en `/speckit-plan` y `/speckit-clarify` cuando se ejecutan
  sobre una especificación existente en una sesión nueva.
- **FR-007**: El resumen entregado NO DEBE presentarse como verdad absoluta
  del estado actual del código — su función es orientar, no reemplazar la
  verificación contra el código o el grafo de código externo cuando haya
  duda.
- **FR-008**: La integración NO DEBE requerir que gomemory escriba o
  modifique el grafo de código externo como parte de este flujo — la
  relación con el grafo de código se mantiene de solo lectura.
- **FR-009**: gomemory DEBE ofrecer, desde su propia interfaz (TUI), un
  interruptor para activar o desactivar el brazo extensor hacia spec-kit,
  independiente de cualquier configuración del lado de spec-kit, cuya
  elección persista entre sesiones.
- **FR-010**: En un proyecto sin spec-kit instalado, la integración DEBE
  permanecer inactiva por defecto y no DEBE intentar detectarlo ni
  conectarse en cada arranque de forma perceptible, siguiendo el mismo
  criterio ya usado para el proveedor externo de grafo de código.

### Key Entities

- **Resumen de historial de especificación**: contenido generado a partir
  de gomemory (y, si está disponible, del grafo de código externo) que se
  incorpora al flujo de `/speckit-specify` (y opcionalmente `/speckit-plan`
  / `/speckit-clarify`) antes de redactar o continuar una especificación.
- **Historial de decisiones del proyecto**: memorias, sesiones y decisiones
  ya registradas en gomemory — el "por qué" y el "qué se hizo" del proyecto.
- **Estructura de código externa**: nodos, relaciones y resumen agregado
  (lenguajes, módulos, símbolos de alto impacto) que aporta un proveedor de
  grafo de código externo ya conectado a gomemory, cuando existe.
- **Interruptor del brazo extensor**: preferencia persistida por proyecto,
  visible y editable desde la TUI de gomemory, que activa o desactiva la
  integración con spec-kit de forma independiente a la configuración propia
  de spec-kit.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Al iniciar una especificación nueva, la persona dispone de un
  resumen de lo que ya existe en el proyecto sin abrir manualmente ningún
  archivo dentro de `specs/`.
- **SC-002**: Incorporar el resumen de historial no añade una demora
  perceptible al inicio de una especificación frente al flujo actual sin
  resumen.
- **SC-003**: En un proyecto sin memorias guardadas o con gomemory no
  disponible, la creación de la especificación se completa exactamente
  igual que hoy, sin pasos ni errores adicionales visibles.
- **SC-004**: Una persona que revisa el resumen puede identificar sin
  ambigüedad, en menos de lo que tarda en leerlo una vez, qué parte proviene
  del historial de decisiones y cuál de la estructura de código.
- **SC-005**: Las especificaciones nuevas dejan de requerir que la persona
  busque manualmente en `specs/` si una funcionalidad similar ya fue
  implementada antes de redactar el borrador.
- **SC-006**: Una persona en un proyecto sin spec-kit nunca percibe efecto
  alguno de esta integración (ni demoras, ni mensajes, ni intentos de
  conexión), sin tener que configurar nada.
- **SC-007**: Una persona puede activar o desactivar el brazo extensor desde
  la TUI de gomemory en un solo paso, y esa elección se mantiene en
  siguientes sesiones sin repetirla.

## Assumptions

- El alcance principal (MVP) de esta especificación es el flujo de creación
  (`/speckit-specify`, Historia 1 y 2); extender el mismo resumen a
  `/speckit-plan` y `/speckit-clarify` (Historia 3) es deseable pero de
  menor prioridad, dado que en la mayoría de los casos ocurren en la misma
  sesión donde ya se cargó el contexto.
- El resumen de historial reutiliza el mecanismo de contexto de proyecto que
  gomemory ya construye para otros flujos (memorias recientes, sesiones,
  relaciones, y el resumen del grafo de código externo cuando está
  disponible), acotado a un tamaño razonable — no se asume la necesidad de
  un motor de búsqueda semántica nuevo para esta primera versión.
- La integración es opcional y de degradación transparente, siguiendo el
  mismo principio ya vigente en el proyecto para proveedores externos: su
  ausencia nunca bloquea ni degrada el flujo base de especificación.
- gomemory mantiene su rol de solo lectura respecto al grafo de código
  externo también en este flujo: no escribe ni modifica esos nodos como
  parte de crear o continuar una especificación.
- La separación entre "historial de decisiones" (gomemory) y "estructura de
  código" (grafo externo) sigue el criterio ya establecido en el proyecto:
  gomemory aporta el "por qué", el grafo externo aporta el "qué/cómo".
- El interruptor de activación/desactivación (Historia 4) sigue el mismo
  patrón ya usado en la TUI de gomemory para otros proveedores externos
  opcionales (p. ej. el grafo de código externo): ausente/apagado no rompe
  nada, y la detección de spec-kit (presencia de `.specify/`) es solo la
  señal para el valor por defecto, no un requisito para que el interruptor
  exista.
