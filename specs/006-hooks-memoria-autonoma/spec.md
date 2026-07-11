# Feature Specification: Memoria autónoma transversal (hooks dinámicos por turno)

**Feature Branch**: `006-hooks-memoria-autonoma`

**Created**: 2026-07-11

**Status**: In Progress

**Input**: User description: "A diferencia de `engram` (Gentleman-Programming), gomemory se sentía menos dinámico al invocar la memoria: había que mencionarla en el chat para que el agente la usara, en vez de que la memoria se activara de forma autónoma. El análisis comparó ambos: engram registra 5 hooks de Claude Code con comportamientos ACTIVOS por turno (forzar carga de tools diferidas vía ToolSearch, recordatorio periódico de guardado, captura pasiva de subagentes, re-inyección post-compactación). gomemory ya tenía los hooks registrados pero con dos brechas de autonomía: (1) el primer prompt emitía `{"tools": true}`, campo NO soportado por Claude Code — no-op silencioso que dejaba las tools MCP diferidas sin cargar; (2) tras el primer prompt, los siguientes eran totalmente pasivos, sin recordar al agente que guardara. Se decide cerrar estas brechas manteniendo la ventaja arquitectónica de gomemory (un único binario `mem hook`, sin servidor HTTP ni dependencia de bash/curl) y garantizando que las mejoras sean TRANSVERSALES a todos los agentes, no solo Claude Code."

## Contexto y motivación

`engram` y `gomemory` son arquitectónicamente casi idénticos (Go + SQLite + MCP stdio). La diferencia percibida de "dinamismo" no venía del motor de memoria sino de los **comportamientos activos dentro de los hooks por turno**. gomemory ya reemplaza los scripts bash+curl de engram por un único binario portable (`mem hook <evento>`), lo cual es una ventaja: funciona igual en Linux/macOS/Windows y no necesita un daemon HTTP. Pero ese binario tenía dos huecos de comportamiento que hacían sentir la memoria "manual".

**Restricción transversal (obligatoria)**: gomemory es multiagente (Claude Code, OpenCode, Cursor, Windsurf, Cline, Codex). Toda mejora de autonomía debe implementarse una sola vez (lógica en Go) y cablearse en cada punto de inyección por turno que gomemory controla, no quedar atada a un solo agente.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Las tools de memoria quedan disponibles sin que el usuario las mencione (Priority: P1)

Como usuario que abre una sesión en un proyecto con gomemory, quiero que el agente pueda invocar las herramientas de memoria desde el primer turno sin que yo tenga que nombrarlas, para que la memoria se sienta autónoma y no como algo que hay que "despertar" a mano.

**Why this priority**: Es la causa raíz del problema reportado. En Claude Code las tools de un MCP server llegan **diferidas** (existen por nombre pero sin esquema cargado); si el hook no las materializa, el agente sabe que hay memoria pero no puede llamarla hasta que el usuario la menciona.

**Independent Test**: Abrir una sesión nueva y, sin mencionar la memoria, comprobar que el agente carga y usa las tools de gomemory en su primer turno.

**Acceptance Scenarios**:

1. **Given** una sesión recién iniciada en Claude Code, **When** el usuario envía su primer mensaje sobre cualquier tema, **Then** el hook inyecta una instrucción explícita de carga de tools (ToolSearch con los nombres reales de gomemory) y el agente puede invocar la memoria en ese mismo turno.
2. **Given** ese mismo primer prompt, **When** se inspecciona la salida del hook, **Then** NO contiene el campo no soportado `{"tools": true}` y SÍ el recordatorio de protocolo como contexto adicional en la forma documentada.

---

### User Story 2 - El agente recibe recordatorios de guardado a lo largo de la sesión (Priority: P1)

Como usuario en una sesión larga, quiero que, si el agente lleva rato trabajando sin registrar decisiones ni hallazgos, algo le recuerde guardar en memoria, para no perder contexto por olvido del agente.

**Why this priority**: Sin recordatorios periódicos, la única señal de "guardá" ocurría al inicio de la sesión; en sesiones largas el agente se "olvidaba" y la memoria quedaba incompleta.

**Independent Test**: Simular una sesión de más de 15 minutos sin guardados reales y verificar que el siguiente turno recibe un recordatorio; verificar que un guardado reciente lo silencia; verificar que no se repite antes del período de enfriamiento.

**Acceptance Scenarios**:

1. **Given** una sesión activa de más de 5 minutos, **When** pasaron más de 15 minutos desde el último guardado real (o no hubo ninguno y la sesión ya superó ese umbral), **Then** el turno siguiente recibe un recordatorio para llamar a `save_memory`.
2. **Given** que acaba de recordarse, **When** llegan turnos inmediatos, **Then** no se repite el recordatorio durante el período de enfriamiento (15 min).
3. **Given** que el agente guardó una memoria real hace menos de 15 minutos, **When** llega un nuevo turno, **Then** no se emite recordatorio.
4. **Given** que solo se insertaron checkpoints automáticos (actividad de turno), **When** se evalúa el recordatorio, **Then** los checkpoints NO cuentan como "guardado real" y no silencian el recordatorio.

---

### User Story 3 - Los recordatorios funcionan igual en todos los agentes (Priority: P1)

Como usuario que puede usar gomemory desde distintos agentes, quiero que el comportamiento de recordatorio sea consistente y no exclusivo de Claude Code, para tener la misma experiencia sin importar la herramienta.

**Why this priority**: gomemory se define como multiagente; una mejora que solo aplique a Claude Code rompería esa promesa.

**Independent Test**: Verificar que la decisión del recordatorio (umbral + enfriamiento) vive una sola vez en Go y que tanto el hook de Claude Code como el plugin de OpenCode la consumen, produciendo el mismo texto bajo las mismas condiciones.

**Acceptance Scenarios**:

1. **Given** las mismas condiciones de sesión y guardado, **When** se evalúa el recordatorio desde el hook de Claude Code y desde `mem hook nudge` (usado por OpenCode), **Then** ambos deciden lo mismo y emiten el mismo texto.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El primer prompt de cada sesión DEBE forzar la carga de las tools MCP de gomemory mediante el único mecanismo que Claude Code respeta (systemMessage con `ToolSearch select:` y los nombres reales), y NO usar campos no soportados.
- **FR-002**: El sistema DEBE recordar al agente que guarde cuando lleve más de 15 minutos sin un guardado real y la sesión tenga más de 5 minutos de vida, con un enfriamiento de 15 minutos entre recordatorios.
- **FR-003**: Los checkpoints automáticos (actividad de turno) NO DEBEN contar como guardado real a efectos del recordatorio.
- **FR-004**: La decisión del recordatorio DEBE residir en una única función en Go, compartida por todos los agentes; cada integración por turno la consume (hook de Claude Code y plugin de OpenCode vía `mem hook nudge`).
- **FR-005**: Ningún hook DEBE abortar el turno del agente ante error; toda operación es best-effort y degrada en silencio.

### Key Entities

- **Memoria real**: cualquier memoria cuyo tipo no sea `checkpoint`. Es la señal usada para medir "tiempo desde el último guardado".
- **Marcador de debounce** (`.memory/.last-nudge`): epoch del último recordatorio emitido, para respetar el enfriamiento entre turnos y entre sesiones.

## Alcance

**Incluido en esta iteración (implementado):**
- Fix de carga de tools diferidas (FR-001).
- Recordatorio periódico de guardado, transversal Claude Code + OpenCode (FR-002 a FR-005).

**Fuera de alcance de esta iteración (pendiente, ver plan.md):**
- Captura pasiva de subagentes (hook `SubagentStop`).
- Re-inyección de protocolo/contexto post-compactación (`SessionStart` matcher `compact`), hoy cubierto parcialmente por `PreCompact`.
- Persistencia del prompt originante junto al guardado.

## Success Criteria *(mandatory)*

- **SC-001**: En una sesión nueva de Claude Code, el agente invoca la memoria en el primer turno sin que el usuario la mencione.
- **SC-002**: En una sesión larga sin guardados, el agente recibe exactamente un recordatorio por período de enfriamiento.
- **SC-003**: El mismo escenario evaluado desde Claude Code y desde OpenCode produce la misma decisión y texto.
