# Especificación de funcionalidad: Operación agnóstica de gomemory

**Rama de funcionalidad**: `025-gomemory-agnostic-operation`

**Creado**: 2026-08-24

**Estado**: Borrador

**Entrada**: Formalizar la mejora agnóstica de gomemory publicada en v2.11.0.

## Contexto del problema

gomemory se utiliza desde proyectos, agentes y sistemas operativos distintos. Tres comportamientos
impedían que esa promesa fuera consistente:

1. El registro MCP de Codex quedaba asociado al directorio de cada proyecto. Al retirar o mover ese
   directorio, Codex intentaba iniciar servidores desde rutas inexistentes y el arranque quedaba
   incompleto.
2. La interfaz de terminal no entregaba correctamente el contenido pegado a todos sus campos y no
   permitía recorrer memorias cuyo detalle excedía el área visible. Copiar una vista tampoco era una
   capacidad uniforme.
3. La constitución predeterminada incluía nombres y atribuciones particulares, aunque debía servir
   como base reutilizable para proyectos sin relación con esas referencias.

La mejora debe hacer que estas capacidades dependan de la intención de la persona y del ámbito real
de la configuración, no de un proyecto, ruta, agente, organización o plataforma particular.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Iniciar gomemory desde cualquier proyecto registrado (Priority: P1)

Una persona que usa Codex necesita que gomemory esté disponible en todos sus proyectos mediante un
único registro de ámbito personal. Mover o eliminar un proyecto previamente utilizado no debe romper
el arranque MCP ni dejar intentos de inicio asociados a rutas obsoletas.

**Why this priority**: un arranque MCP incompleto impide usar la memoria y muestra errores en cada
sesión. Es la pérdida funcional de mayor impacto.

**Independent Test**: preparar varios registros antiguos asociados a proyectos, incluidos proyectos
que ya no existen, ejecutar la actualización y comprobar que queda un solo registro compartido que
funciona desde un proyecto distinto.

**Acceptance Scenarios**:

1. **Given** varios registros antiguos de gomemory asociados a proyectos diferentes, **When** se
   actualiza la configuración, **Then** queda un único registro compartido y desaparecen todos los
   registros antiguos de gomemory.
2. **Given** un registro antiguo que apunta a un proyecto eliminado, **When** Codex inicia después de
   la actualización, **Then** gomemory inicia sin depender de esa ruta y no aparece un error por
   directorio inexistente.
3. **Given** una configuración que también contiene servidores ajenos a gomemory, **When** se realiza
   la migración, **Then** esos servidores y sus valores permanecen sin cambios.
4. **Given** una configuración que requiere migración, **When** se reemplaza, **Then** existe una copia
   recuperable del estado anterior y se conservan sus restricciones de acceso.
5. **Given** una configuración que ya contiene el registro compartido correcto, **When** se ejecuta de
   nuevo la actualización, **Then** no se crean duplicados ni se altera contenido ajeno.
6. **Given** varios proyectos en la misma cuenta, **When** cualquiera de ellos inicia Codex, **Then**
   todos reutilizan el mismo servidor gomemory sin registros por proyecto.

---

### User Story 2 - Trabajar con texto completo desde la interfaz de terminal (Priority: P1)

Una persona necesita copiar información visible, pegar texto en el campo que está editando y recorrer
el detalle completo de una memoria extensa sin abandonar la interfaz de terminal.

**Why this priority**: copiar, pegar y leer el contenido completo son operaciones básicas. Si fallan,
la interfaz impide completar tareas habituales aunque los datos existan.

**Independent Test**: abrir una memoria que exceda ampliamente el área visible, recorrerla hasta el
final, copiarla y pegar texto en cada tipo de campo editable, verificando que no se pierdan caracteres.

**Acceptance Scenarios**:

1. **Given** cualquier vista con contenido textual, **When** la persona solicita copiar, **Then** el
   contenido correspondiente queda disponible en el portapapeles y la interfaz confirma la acción.
2. **Given** una memoria cuyo detalle tiene más líneas que el área visible, **When** se abre, **Then**
   la persona puede avanzar, retroceder, saltar por páginas e ir al inicio o al final.
3. **Given** una memoria extensa desplazada a cualquier posición, **When** se copia su detalle,
   **Then** se copia la memoria completa y no solo el fragmento visible.
4. **Given** un campo editable con el foco activo, **When** se pega texto mediante las capacidades del
   terminal, **Then** el campo recibe exactamente ese texto en la posición activa.
5. **Given** una memoria corta, **When** se usan acciones de desplazamiento, **Then** el contenido
   permanece estable y la interfaz no presenta errores ni posiciones inválidas.
6. **Given** una terminal compatible en cualquiera de las plataformas soportadas, **When** se realizan
   las acciones de copiar, pegar y desplazar, **Then** la experiencia observable es equivalente.

---

### User Story 3 - Reutilizar una constitución sin identidad impuesta (Priority: P2)

Una persona que inicia un proyecto necesita una constitución predeterminada que pueda adoptar sin
retirar primero nombres de personas, equipos, organizaciones o proyectos ajenos.

**Why this priority**: no impide ejecutar gomemory, pero afecta la reutilización correcta del producto
y puede atribuir normas a quien no corresponde.

**Independent Test**: restaurar o inicializar la constitución predeterminada y comprobar que conserva
sus principios aplicables sin contener identidades o atribuciones particulares.

**Acceptance Scenarios**:

1. **Given** un proyecto nuevo sin constitución administrada, **When** se inicializa el documento
   predeterminado, **Then** su encabezado y contenido no identifican a una persona, organización o
   proyecto particular.
2. **Given** una constitución predeterminada restaurada, **When** se compara con la plantilla vigente,
   **Then** ambas expresan la misma base agnóstica.
3. **Given** una constitución que un equipo ya personalizó, **When** se actualiza gomemory, **Then** la
   personalización no se reemplaza automáticamente.

### Edge Cases

- El archivo de configuración no existe: debe poder crearse con un único registro compartido sin
  exigir un proyecto previo.
- El archivo de configuración contiene comentarios, secciones no relacionadas o distintas formas
  válidas de nombrar las secciones: la migración debe conservar todo lo ajeno a gomemory.
- La actualización se interrumpe mientras reemplaza la configuración: el estado previo debe seguir
  siendo recuperable y no debe quedar un archivo parcialmente escrito como configuración activa.
- Dos solicitudes de actualización ocurren casi al mismo tiempo: el resultado debe contener un solo
  registro compartido.
- La memoria contiene líneas muy largas, caracteres multibyte, saltos de línea o más contenido que
  varias páginas: copiar y desplazarse no debe truncar ni corromper el texto.
- El identificador opcional de una memoria es más corto que la forma resumida mostrada: la vista no
  debe fallar al presentarlo.
- El portapapeles no está disponible en la terminal: la interfaz debe permanecer utilizable y no debe
  perder ni modificar el contenido de la memoria.
- La constitución predeterminada contiene ejemplos técnicos: un ejemplo no debe introducir nombres o
  atribuciones que conviertan el documento en específico de una organización.

## Requirements *(mandatory)*

### Functional Requirements

**Registro MCP compartido**

- **FR-001**: Codex DEBE disponer de un único registro gomemory de ámbito personal, reutilizable por
  todos los proyectos de la misma cuenta.
- **FR-002**: El registro compartido NO DEBE depender del directorio de un proyecto concreto para
  iniciar gomemory.
- **FR-003**: La actualización DEBE identificar y retirar todos los registros gomemory antiguos de
  ámbito por proyecto, incluso si sus rutas ya no existen.
- **FR-004**: La migración DEBE conservar sin cambios semánticos toda configuración que no pertenezca
  a gomemory.
- **FR-005**: Antes de reemplazar una configuración con registros antiguos, el sistema DEBE crear una
  copia exacta y recuperable del estado anterior.
- **FR-006**: El reemplazo de configuración DEBE evitar que un fallo intermedio deje contenido parcial
  como estado activo.
- **FR-007**: La configuración resultante DEBE conservar las restricciones de acceso del archivo que
  reemplaza.
- **FR-008**: La actualización DEBE ser idempotente y segura ante solicitudes concurrentes: cualquier
  cantidad de ejecuciones debe producir exactamente un registro compartido.
- **FR-009**: La desinstalación desde un proyecto NO DEBE retirar el registro compartido mientras pueda
  ser utilizado por otros proyectos y DEBE explicar este alcance a la persona.

**Interacción textual en la TUI**

- **FR-010**: La persona DEBE poder copiar el contenido textual correspondiente desde todas las vistas
  de la interfaz de terminal mediante una acción consistente.
- **FR-011**: La interfaz DEBE confirmar de forma visible cuando recibe una solicitud válida de copia.
- **FR-012**: Al copiar el detalle de una memoria, el resultado DEBE incluir todos sus metadatos
  presentables y el contenido completo, con independencia del fragmento visible.
- **FR-013**: Todo campo editable con foco DEBE aceptar íntegramente el contenido pegado que entregue
  la terminal, sin omitir ni alterar sus caracteres.
- **FR-014**: El detalle de una memoria que exceda el área visible DEBE admitir desplazamiento por
  línea, por página, al inicio y al final.
- **FR-015**: La vista DEBE limitar el contenido presentado al espacio disponible e indicar cuando hay
  contenido adicional arriba o abajo.
- **FR-016**: Las acciones de desplazamiento DEBEN mantener siempre una posición válida para memorias
  vacías, cortas, extensas o con líneas que requieran ajuste visual.
- **FR-017**: Las ayudas visibles DEBEN informar las acciones disponibles de copia y desplazamiento sin
  impedir el uso de las acciones existentes.
- **FR-018**: Copiar, pegar y desplazarse DEBEN ofrecer resultados equivalentes en las plataformas y
  terminales soportadas, sin requerir rutas ni utilidades específicas de una plataforma.
- **FR-019**: La indisponibilidad del portapapeles NO DEBE cerrar la interfaz, corromper datos ni impedir
  la navegación y edición restantes.

**Constitución agnóstica**

- **FR-020**: La constitución predeterminada NO DEBE contener nombres de personas, equipos,
  organizaciones o proyectos particulares como título, autoría o ámbito de pertenencia.
- **FR-021**: La plantilla distribuida y la constitución predeterminada restaurable DEBEN mantener el
  mismo contenido base agnóstico.
- **FR-022**: Una actualización NO DEBE sobrescribir automáticamente una constitución que el equipo ya
  personalizó.
- **FR-023**: La constitución predeterminada DEBE conservar reglas aplicables a proyectos compatibles
  sin atribuirlas a una identidad concreta.

**Agnosticismo transversal**

- **FR-024**: Los comportamientos de esta funcionalidad DEBEN depender de capacidades declaradas y del
  ámbito de la persona, no del nombre del proyecto, su ruta, el agente anfitrión o el sistema operativo.
- **FR-025**: La documentación y los mensajes dirigidos a la persona DEBEN describir el alcance
  compartido y evitar instrucciones exclusivas de una plataforma cuando exista una acción común.

### Key Entities

- **Registro MCP compartido**: configuración personal que permite iniciar gomemory desde cualquier
  proyecto; tiene una identidad única y no mantiene una ruta de proyecto como requisito de arranque.
- **Registro MCP antiguo**: configuración creada para un proyecto concreto; puede incluir una ruta que
  ya no existe y debe ser retirada durante la migración.
- **Respaldo de configuración**: copia exacta del estado anterior a una migración, identificable y
  recuperable sin reemplazar respaldos previos.
- **Vista textual**: contenido que la interfaz presenta en una pantalla y que puede copiarse.
- **Detalle de memoria**: metadatos y contenido completo de una memoria, del cual solo una ventana puede
  estar visible simultáneamente.
- **Campo activo**: entrada editable que recibe acciones de escritura y pegado mientras tiene el foco.
- **Constitución predeterminada**: documento base administrado por gomemory para proyectos que todavía
  no tienen una versión personalizada.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: En una prueba con al menos 20 registros antiguos, incluidas 8 rutas inexistentes, el
  100 % se migra a un único registro compartido y ningún arranque posterior falla por esas rutas.
- **SC-002**: Tras la migración, gomemory inicia correctamente desde al menos 10 proyectos distintos sin
  agregar configuración específica para ninguno de ellos.
- **SC-003**: El 100 % del contenido ajeno a gomemory y sus restricciones de acceso permanece
  equivalente después de la migración, verificado contra una copia anterior.
- **SC-004**: Diez actualizaciones consecutivas y ocho solicitudes simultáneas producen siempre un solo
  registro compartido y ningún registro antiguo.
- **SC-005**: En memorias de hasta 10.000 caracteres y 500 líneas, la persona puede alcanzar el inicio y
  el final, y la copia conserva el 100 % de caracteres y saltos de línea del contenido original.
- **SC-006**: El 100 % de los tipos de campo editable acepta texto pegado, incluidos caracteres
  multibyte y múltiples líneas cuando el campo los admite, sin pérdida ni duplicación.
- **SC-007**: Una persona que desconoce la interfaz identifica cómo copiar y desplazar contenido en su
  primer intento usando únicamente las ayudas visibles.
- **SC-008**: Las mismas pruebas de copia, pegado y desplazamiento entregan resultados equivalentes en
  Linux, macOS y Windows dentro de las terminales soportadas.
- **SC-009**: La constitución predeterminada contiene cero títulos, autorías o declaraciones de alcance
  que identifiquen a una persona, organización o proyecto particular.
- **SC-010**: El 100 % de las constituciones personalizadas usadas en pruebas conserva su contenido tras
  actualizar gomemory.

## Assumptions

- La funcionalidad formaliza el comportamiento publicado en `v2.11.0`; no solicita una segunda
  implementación independiente.
- La cuenta de la persona es el ámbito adecuado para compartir el registro MCP de Codex entre
  proyectos.
- Las plataformas soportadas por la distribución vigente son Linux, macOS y Windows.
- La terminal anfitriona es responsable de ofrecer el canal de portapapeles; gomemory debe usar esa
  capacidad sin depender de utilidades externas específicas de una plataforma.
- Los respaldos de configuración permanecen junto al archivo administrado y no reemplazan respaldos
  existentes.
- Una constitución ya presente se considera personalizada y no se sustituye salvo una acción explícita
  de restauración de la persona.

## Out of Scope

- Crear registros compartidos para agentes que no tengan integración con gomemory.
- Sincronizar una configuración personal entre máquinas o cuentas diferentes.
- Proporcionar un portapapeles propio cuando la terminal anfitriona no ofrece esa capacidad.
- Incorporar edición completa del contenido desde la pantalla de detalle de una memoria.
- Reescribir automáticamente constituciones que los equipos ya personalizaron.
- Cambiar los principios técnicos de la constitución; esta funcionalidad solo elimina identidad y
  atribución específicas de su contenido predeterminado.
