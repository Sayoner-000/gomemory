# Feature Specification: Evolución de la Integración con Grafo de Código Externo

**Feature Branch**: `010-codegraph-integration-evolution`

**Created**: 2026-07-23

**Status**: Draft

**Input**: User description: "revisa que podemos mejorar y evolucionar" a partir del análisis de cómo gomemory se conecta hoy con codebase-memory-mcp (brazo extensor opcional, solo lectura de un resumen agregado de arquitectura, patrón no-bloqueante con snapshot cacheado y refresco detached). Ver `specs/004-code-graph/` (grafo propio Fase 1) como antecedente directo.

## Contexto: qué existe hoy

Hoy la integración con un proveedor externo de grafo de código (p. ej.
codebase-memory-mcp) es **de un solo sentido y agregada**: en cada
`get_context`, si hay un proveedor configurado y disponible, se inyecta un
resumen genérico del proyecto completo (lenguajes, clusters, hotspots) leído
de un snapshot cacheado (TTL 60s, refresco en proceso detached). Esa
información no se dirige a lo que el agente está haciendo en el turno actual,
ni se relaciona con las memorias concretas (`bugfix`, `decision`,
`architecture`) que el agente va guardando. El proveedor tampoco recibe nada
de vuelta: gomemory nunca escribe en el grafo externo.

Esta especificación cubre tres evoluciones independientes sobre esa base,
manteniendo los tres principios ya establecidos y verificados en el código
actual: (1) el proveedor es opcional y su ausencia nunca degrada la
experiencia base, (2) ninguna consulta al proveedor bloquea el guardado de
memoria ni el arranque de sesión, (3) gomemory nunca dispara indexado ajeno.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Advertencia de impacto al guardar una memoria sobre un archivo de riesgo (Priority: P1)

Como agente AI (o persona) que guarda un `bugfix` o una `decision` asociada a
un archivo concreto (`filepath`), quiero saber si ese archivo contiene código
de alto impacto (muchos llamadores, "hotspot" del grafo externo) en el
momento mismo de guardar, para calibrar cuánta cautela aplicar antes de tocar
ese código — sin tener que ir a consultar el grafo por separado.

**Por qué esta prioridad**: hoy el resumen de hotspots existe pero es
agregado y genérico (top-N del proyecto completo en cada `get_context`); no
le dice nada al agente sobre el archivo específico que está a punto de
guardar como relacionado a un bug. Conectar "lo que se está guardando" con
"qué tan riesgoso es ese archivo" es el salto de valor más directo sobre la
integración actual.

**Prueba independiente**: se puede probar guardando una memoria con
`filepath` apuntando a un símbolo que el proveedor externo reporta como
hotspot, y verificando que la memoria persistida (o la respuesta de
`save_memory`) incluye la anotación de impacto — sin que el guardado tarde
perceptiblemente más que hoy, y sin que falle si el proveedor no está
disponible.

**Acceptance Scenarios**:

1. **Given** un proveedor de grafo externo disponible con un snapshot vigente
   que marca un símbolo del archivo `X` como hotspot, **When** se guarda una
   memoria con `filepath=X`, **Then** la memoria persistida incluye una nota
   de impacto (p. ej. "archivo de alto impacto: N llamadores directos") junto
   al resto del contenido.
2. **Given** un archivo que el proveedor no reporta como hotspot (o el
   proveedor no tiene información de ese archivo), **When** se guarda una
   memoria con ese `filepath`, **Then** la memoria se guarda igual, sin
   anotación de impacto y sin mensaje de error.
3. **Given** el proveedor externo no disponible o desactivado, **When** se
   guarda cualquier memoria con `filepath`, **Then** el guardado se completa
   exactamente igual que hoy (sin anotación, sin demora perceptible, sin
   error visible).
4. **Given** una memoria sin `filepath`, **When** se guarda, **Then** nunca
   se consulta el grafo externo para esa memoria (no aplica).

---

### User Story 2 - Sincronización bidireccional de decisiones arquitectónicas como ADR (Priority: P2)

Como persona que mantiene el proyecto, quiero que las memorias de tipo
`architecture`/`decision` que gomemory guarda queden también disponibles
como registro de decisión de arquitectura (ADR) en el grafo de código
externo, **y** que los ADR que ya existan o se creen del lado del proveedor
externo aparezcan también como memoria consultable en gomemory, para tener
un único cuerpo de decisiones coherente en ambas direcciones sin
documentarlas dos veces a mano.

**Por qué esta prioridad**: es valor real pero secundario al de la Historia 1
— depende de que el proveedor externo soporte gestión de ADRs, es opcional
por naturaleza (activable/desactivable), y su ausencia no afecta el flujo
principal de memoria.

**Prueba independiente**: se puede probar activando la sincronización,
guardando una memoria `architecture`, y verificando en el proveedor externo
que aparece el ADR correspondiente; y, en el otro sentido, dando de alta un
ADR nuevo directamente en el proveedor externo y verificando que aparece como
memoria en gomemory tras el siguiente refresco — cada dirección se puede
probar de forma aislada, y ambas deben poder fallar sin afectar el flujo
principal de guardado.

**Acceptance Scenarios**:

1. **Given** la sincronización de ADR activada y el proveedor disponible,
   **When** se guarda una memoria de tipo `architecture` o `decision`,
   **Then** se crea o actualiza un ADR correspondiente en el proveedor
   externo, de forma best-effort y sin bloquear la confirmación del guardado
   al agente.
2. **Given** la sincronización de ADR activada pero el proveedor no
   disponible en el momento del guardado, **When** se guarda una memoria
   `architecture`/`decision`, **Then** la memoria se guarda igual en
   gomemory, y el intento de sincronización queda registrado como pendiente
   o se reintenta en el próximo refresco, sin generar error visible al
   agente.
3. **Given** la sincronización de ADR desactivada (default), **When** se
   guarda cualquier memoria, **Then** el comportamiento es idéntico al actual
   (sin llamadas al proveedor para ADR en ningún sentido).
4. **Given** una memoria `architecture`/`decision` ya sincronizada, **When**
   se guarda de nuevo con el mismo `topic_key` (actualización), **Then** el
   ADR existente se actualiza en vez de crear uno duplicado.
5. **Given** un ADR existente o nuevo del lado del proveedor externo que
   nunca se originó en gomemory, **When** ocurre el siguiente refresco de
   sincronización, **Then** ese ADR se importa a gomemory como memoria
   (`architecture`), quedando disponible en `get_context`, `search_memories`
   y la TUI igual que cualquier otra memoria.
6. **Given** un ADR que sí se originó en gomemory (fue exportado por esta
   misma sincronización), **When** ocurre el siguiente refresco, **Then** NO
   se reimporta como una memoria nueva ni duplicada — se reconoce como propio
   por su registro de sincronización.
7. **Given** una memoria importada desde un ADR externo, **When** esa memoria
   se actualiza más tarde desde gomemory (p. ej. el agente la edita), **Then**
   la actualización se propaga de vuelta al ADR de origen igual que con una
   memoria nativa de gomemory (no se trata distinto por haber sido
   importada).

---

### User Story 3 - Múltiples proveedores de grafo de código con selección automática (Priority: P3)

Como persona que administra gomemory en distintos proyectos/máquinas, quiero
poder declarar más de un proveedor de grafo de código posible (por ejemplo,
si distintos equipos usan herramientas distintas), para que gomemory use
automáticamente el primero disponible sin que tenga que reconfigurar
`--code-graph-command` manualmente cada vez que cambia de entorno.

**Por qué esta prioridad**: valor real para portabilidad entre máquinas/
equipos, pero de menor impacto inmediato que las dos historias anteriores —
hoy solo existe un proveedor concreto adoptado (codebase-memory-mcp), así que
esta historia prepara el terreno más que resuelve un dolor actual.

**Prueba independiente**: se puede probar configurando dos proveedores
candidatos (uno inexistente en el PATH y otro presente), y verificando que
gomemory usa el disponible sin intervención manual y sin fallar por el
ausente.

**Acceptance Scenarios**:

1. **Given** dos o más proveedores declarados en configuración, **When**
   gomemory arma el contexto, **Then** usa el primer proveedor disponible
   según el orden de prioridad declarado, e ignora en silencio los que no
   responden.
2. **Given** ningún proveedor de la lista disponible, **When** gomemory arma
   el contexto, **Then** el comportamiento es idéntico al caso actual sin
   proveedor (sin enriquecimiento, sin error).
3. **Given** el proveedor previamente activo deja de estar disponible y otro
   de la lista sí lo está, **When** ocurre el siguiente refresco de
   snapshot, **Then** gomemory cambia de proveedor automáticamente sin que la
   persona deba tocar configuración.

---

### Edge Cases

- ¿Qué pasa si el proveedor externo devuelve datos de impacto (Historia 1)
  para un `filepath` que no coincide exactamente con ningún símbolo indexado
  (archivo renombrado, ruta relativa distinta)? → Se trata igual que "sin
  información": la memoria se guarda sin anotación, sin error.
- ¿Qué pasa si la sincronización de ADR (Historia 2) se activa sobre un
  proyecto que ya tiene cientos de memorias `architecture`/`decision`
  históricas y, del otro lado, cientos de ADR ya existentes? → La primera
  activación dispara una conciliación inicial best-effort en ambos sentidos
  (exporta lo que gomemory tiene, importa lo que el proveedor tiene) en vez
  de aplicar solo hacia adelante; si el volumen es alto, la conciliación
  puede completarse de forma incremental entre varios refrescos sin bloquear
  el uso normal mientras tanto.
- ¿Qué pasa si el mismo contenido de decisión se edita casi al mismo tiempo
  en gomemory y en el ADR externo (conflicto de edición concurrente)? → Gana
  la versión con timestamp de modificación más reciente; el registro de
  sincronización deja constancia de que hubo un conflicto resuelto, sin
  perder silenciosamente ninguna de las dos versiones (la que pierde queda
  visible en el historial de relaciones, no se descarta).
- ¿Qué pasa si dos proveedores (Historia 3) reportan información
  contradictoria para el mismo símbolo? → Gana el proveedor activo actual
  (el primero disponible en el orden de prioridad); no se mezclan ni
  concilian datos de proveedores distintos en un mismo snapshot.
- ¿Qué pasa si el refresco de snapshot está en curso (proceso detached)
  cuando se guarda una memoria con `filepath` (Historia 1)? → Se usa el
  snapshot cacheado tal cual esté en ese instante (posiblemente desactualizado
  hasta 60s); nunca se espera al refresco.
- ¿Qué pasa si falla la escritura o lectura de ADR (Historia 2) por causas
  ajenas al proveedor (permisos, red)? → Se trata igual que "proveedor no
  disponible": no bloquea ni falla el guardado de la memoria en gomemory ni
  el resto del refresco.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE, al guardar una memoria con `filepath`
  asociado, consultar de forma no bloqueante si ese archivo/símbolo aparece
  como hotspot en el snapshot cacheado del proveedor de grafo externo
  vigente, y anotar el resultado en la memoria persistida cuando aplique.
- **FR-002**: El sistema DEBE completar el guardado de cualquier memoria de
  forma idéntica a hoy (mismo tiempo de respuesta, sin errores visibles)
  cuando el proveedor de grafo externo no está disponible, no reporta datos
  para el archivo, o está desactivado.
- **FR-003**: El sistema DEBE ofrecer una sincronización opcional
  (desactivada por defecto) que refleje las memorias de tipo `architecture` y
  `decision` como ADR en el proveedor externo, activable/desactivable por
  configuración.
- **FR-004**: La sincronización de ADR DEBE ser best-effort en ambos
  sentidos: un fallo al exportar hacia el proveedor o al importar desde él
  nunca impide ni retrasa perceptiblemente la confirmación del guardado de
  una memoria en gomemory ni el resto del refresco de contexto.
- **FR-005**: La sincronización de ADR DEBE actualizar el ADR (o la memoria)
  existente en vez de duplicarlo cuando el origen se actualiza (mismo
  `topic_key` o identidad equivalente a la ya usada por el dedup de
  gomemory).
- **FR-005b**: El sistema DEBE importar como memoria `architecture` en
  gomemory cualquier ADR presente en el proveedor externo que no se haya
  originado en una exportación previa de gomemory, manteniéndolo consultable
  por `get_context`, `search_memories`, `get_memory` y la TUI igual que
  cualquier memoria nativa.
- **FR-005c**: El sistema DEBE distinguir, para cada par memoria↔ADR
  sincronizado, cuál de los dos lados fue el origen de la creación, para
  evitar reimportar como memoria nueva un ADR que en realidad se exportó
  desde gomemory (y viceversa), previniendo bucles de sincronización.
- **FR-005d**: Cuando la misma decisión cambie en ambos lados antes de que
  ocurra la siguiente sincronización, el sistema DEBE resolver el conflicto
  por timestamp de modificación más reciente y dejar constancia consultable
  de que hubo un conflicto resuelto (sin descartar silenciosamente ninguna de
  las dos versiones).
- **FR-006**: El sistema DEBE permitir declarar más de un proveedor de grafo
  de código candidato, en un orden de prioridad explícito.
- **FR-007**: El sistema DEBE seleccionar automáticamente, en cada refresco
  de snapshot, el primer proveedor disponible de la lista declarada, sin
  requerir intervención manual cuando cambia la disponibilidad.
- **FR-008**: Ninguna de las tres capacidades anteriores DEBE, bajo ninguna
  circunstancia, disparar un indexado del proveedor externo (esa operación
  sigue siendo responsabilidad exclusiva de quien administra el proveedor).
- **FR-009**: Cada una de las tres capacidades DEBE poder activarse o
  desactivarse de forma independiente entre sí (activar la Historia 2 no
  requiere tener activa la Historia 1, y viceversa).
- **FR-010**: El sistema DEBE registrar (de forma consultable, no
  necesariamente visible por defecto) el estado de cada intento de
  sincronización de ADR en cada sentido, para distinguir "nunca sincronizado"
  de "sincronización fallida temporalmente" y de "conflicto resuelto".

### Key Entities *(include if feature involves data)*

- **CodeImpactAnnotation**: anotación de impacto derivada del grafo externo
  para un `filepath`/símbolo concreto en el momento de guardar una memoria
  (p. ej. "hotspot con N llamadores directos"); vive adjunta a la memoria, no
  como entidad separada persistente.
- **ADRSyncRecord**: relación entre una memoria de gomemory (`architecture`/
  `decision`) y su ADR correspondiente en el proveedor externo — identifica
  el ADR remoto, **el lado de origen** (creado primero en gomemory o
  importado desde el proveedor), el estado de la última sincronización en
  cada sentido (ok / pendiente / fallida / conflicto-resuelto) y cuándo
  ocurrió.
- **ProviderCandidateList**: lista ordenada por prioridad de proveedores de
  grafo de código declarados, usada para decidir cuál está activo en cada
  refresco de snapshot.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Al guardar una memoria cuyo archivo asociado es un hotspot
  conocido del proveedor externo, la anotación de impacto aparece en el
  100% de los casos donde el snapshot está vigente y disponible.
- **SC-002**: El tiempo de guardado de una memoria no aumenta de forma
  perceptible (mismo orden de magnitud que sin estas capacidades) sin
  importar si hay 0, 1 o 2 proveedores de grafo configurados, ni si la
  sincronización de ADR está activa o no.
- **SC-003**: El 100% de las memorias `architecture`/`decision` guardadas con
  la sincronización de ADR activa y el proveedor disponible en el momento del
  guardado terminan reflejadas como ADR consultable, sin duplicados por
  actualizaciones repetidas de la misma memoria.
- **SC-004**: Si el proveedor activo deja de estar disponible y hay otro
  candidato disponible en la lista, el sistema sigue enriqueciendo el
  contexto sin ninguna intervención manual en el 100% de los casos
  evaluados.
- **SC-005**: En ningún escenario de las tres capacidades el sistema dispara
  un indexado del proveedor externo por sí solo.
- **SC-006**: El 100% de los ADR presentes en el proveedor externo que no se
  originaron en gomemory terminan disponibles como memoria consultable en
  gomemory tras, como máximo, el siguiente ciclo de refresco — sin generar
  duplicados en refrescos posteriores.

## Assumptions

- Las tres capacidades son estrictamente aditivas sobre la integración ya
  existente (snapshot cacheado + refresco detached + `available=false`
  degrada en silencio): ninguna de ellas cambia el contrato ya probado del
  puerto `CodeGraphProvider` (`Name()`, `Snapshot()`, `MaybeRefresh()`), sino
  que lo extiende.
- El proveedor externo de referencia (codebase-memory-mcp) ya expone, vía su
  propio CLI, las operaciones necesarias para consultar impacto por símbolo y
  para gestionar ADR (`manage_adr`, incluida lectura de ADR existentes); si
  un proveedor concreto no soporta alguna de estas operaciones, esa
  capacidad específica se reporta como no disponible para ese proveedor,
  igual que hoy ocurre con `get_architecture`.
- La sincronización de ADR (Historia 2) es **bidireccional por diseño**:
  gomemory exporta decisiones propias hacia el proveedor externo, e importa
  como memoria cualquier ADR del proveedor que no haya sido originado en
  gomemory. El registro de origen (`ADRSyncRecord`) es lo que evita bucles
  (reimportar lo exportado o reexportar lo importado como si fuera nuevo).
- La selección automática de proveedor (Historia 3) usa el orden de prioridad
  tal como lo declara la persona que administra el proyecto; no se asume
  ninguna heurística automática de "mejor proveedor" más allá de
  disponibilidad.
- Estas capacidades se activan/desactivan por proyecto, siguiendo el mismo
  mecanismo de configuración ya usado hoy para `code_graph_disabled` /
  `code_graph_command`.
