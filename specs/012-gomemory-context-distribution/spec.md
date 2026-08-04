# Feature Specification: Distribuir el brazo extensor gomemory-context vía `mem install`, transversal a agentes

**Feature Branch**: `012-gomemory-context-distribution`

**Created**: 2026-08-03

**Status**: Draft

**Input**: User description: "Actualmente el brazo extensor gomemory-context (spec 011) solo existe instalado manualmente dentro de este mismo repositorio (go_memory) y solo quedó activo para Claude Code, porque `mem install` no distribuye la extensión a otros proyectos ni genera los artefactos para otros agentes. Quiero que sea transversal: que `mem install` deje la extensión gomemory-context lista para cualquier proyecto que tenga spec-kit instalado, y que funcione en todos los agentes que gomemory ya soporta de forma nativa — con OpenCode como caso explícito además de Claude Code. Debe seguir el mismo patrón ya usado para distribuir speckit-constitution-gen.md (plantilla embebida, copiada solo si el proyecto destino la necesita y no sobrescribiendo si ya existe)."

## Contexto: qué existe hoy

La especificación 011 conectó gomemory con el flujo de `/speckit-specify` a
través de una extensión de spec-kit (`gomemory-context`), verificada en
vivo dentro de este mismo repositorio. Pero esa verificación se hizo
instalando la extensión **a mano**, con la CLI de terceros `specify`, y
solo quedó activa para **Claude Code** — la única integración de spec-kit
instalada en este repositorio.

`mem install` (el instalador que sí usan todas las personas que adoptan
gomemory) hoy no sabe nada de esta extensión: no la copia a proyectos
nuevos, y no genera los artefactos que cada agente necesita para
reconocerla. Como resultado, hoy el brazo extensor hacia spec-kit solo
existe, en la práctica, en `go_memory` — nadie más lo tiene, y quien sí lo
tuviera solo lo vería funcionar si usa Claude Code.

Esta especificación cubre cerrar esa brecha: que `mem install` deje el
brazo extensor listo de fábrica, en cualquier proyecto que ya use spec-kit,
para los agentes que gomemory ya instala de forma completa hoy (Claude Code
y OpenCode), sin exigir que la persona tenga instalada la CLI de `specify`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - El brazo extensor queda listo con solo `mem install` (Priority: P1)

Como persona que instala o actualiza gomemory (`mem install`) en un
proyecto que ya usa spec-kit, quiero que el brazo extensor hacia
`/speckit-specify` quede funcionando de inmediato, sin instalar ni
configurar nada aparte, para no tener que repetir a mano lo que hoy solo
existe dentro de `go_memory`.

**Por qué esta prioridad**: es el corazón del pedido — hoy la única forma
de tener esta capacidad es replicar manualmente lo que se hizo en este
repo, algo que nadie más puede hacer de forma realista.

**Prueba independiente**: se puede probar corriendo `mem install` en un
proyecto de prueba que ya tenga `.specify/` (spec-kit inicializado, sin la
extensión gomemory-context) y verificando que, al terminar, `/speckit-specify`
incorpora el resumen de historial automáticamente — igual que se verificó
en `go_memory` para la spec 011, pero sin haber tocado nada a mano.

**Acceptance Scenarios**:

1. **Given** un proyecto con `.specify/` ya presente (spec-kit
   inicializado) y sin la extensión gomemory-context, **When** se ejecuta
   `mem install`, **Then** al terminar existen los archivos de la extensión
   y el artefacto que el agente activo necesita para reconocer el hook
   `before_specify`, sin pasos manuales adicionales.
2. **Given** el proyecto del escenario anterior, **When** se invoca
   `/speckit-specify`, **Then** el resumen de historial se incorpora
   automáticamente, igual que se verificó en `go_memory` para la spec 011.
3. **Given** la persona no tiene instalada ninguna herramienta de terceros
   de spec-kit (la CLI `specify`) más allá del propio spec-kit ya
   inicializado en el proyecto, **When** se ejecuta `mem install`,
   **Then** el brazo extensor queda igual de funcional que si la CLI
   estuviera instalada.

---

### User Story 2 - Paridad real con OpenCode (Priority: P1)

Como persona que usa OpenCode (no Claude Code) para trabajar con spec-kit,
quiero que `mem install` deje el brazo extensor igual de funcional para
OpenCode, para no quedar en desventaja frente a quienes usan Claude Code.

**Por qué esta prioridad**: el pedido explícito fue que esto sea
transversal, con OpenCode como caso nombrado — gomemory ya instala OpenCode
de forma completa (plugin + hooks) igual que Claude Code, así que dejar
fuera a OpenCode aquí sería una asimetría injustificada.

**Prueba independiente**: se puede probar corriendo `mem install` en un
proyecto con `.specify/` usando OpenCode como agente, y verificando que
`/speckit-specify` (ejecutado desde OpenCode) también incorpora el resumen
de historial automáticamente.

**Acceptance Scenarios**:

1. **Given** un proyecto con `.specify/` ya presente y OpenCode como agente
   configurado, **When** se ejecuta `mem install`, **Then** el artefacto
   que OpenCode necesita para reconocer el hook `before_specify` queda
   creado, con el mismo comportamiento (resumen automático, degradación
   transparente) ya verificado para Claude Code en la spec 011.
2. **Given** un proyecto con ambos agentes configurados (Claude Code y
   OpenCode), **When** se ejecuta `mem install`, **Then** ambos quedan
   funcionales, sin que uno dependa de que el otro se haya usado primero.

---

### User Story 3 - Proyectos sin spec-kit no se ven afectados (Priority: P2)

Como persona que usa gomemory en un proyecto que no usa spec-kit, quiero
que `mem install` no cree ningún archivo relacionado con esta extensión,
para no ensuciar mi proyecto con algo que no voy a usar.

**Por qué esta prioridad**: es la continuación directa del principio ya
establecido en la spec 011 (Historia 4): la integración con spec-kit nunca
debe imponerse a quien no lo usa.

**Independent Test**: se puede probar corriendo `mem install` en un
proyecto sin `.specify/` y confirmando que no aparece ningún archivo nuevo
relacionado con la extensión gomemory-context.

**Acceptance Scenarios**:

1. **Given** un proyecto sin `.specify/` (spec-kit no inicializado),
   **When** se ejecuta `mem install`, **Then** no se crea ningún archivo
   bajo `.specify/extensions/gomemory-context/` ni ningún artefacto de
   agente relacionado con esta extensión.

---

### User Story 4 - Las correcciones futuras llegan solas (Priority: P3)

Como persona que ya tiene el brazo extensor instalado y actualiza
gomemory a una versión más nueva, quiero que las correcciones o mejoras
del brazo extensor lleguen a mi proyecto en el siguiente `mem install`, sin
tener que reinstalar la extensión a mano.

**Por qué esta prioridad**: valor incremental sobre la Historia 1 — sin
esto, cada corrección futura del brazo extensor requeriría instrucciones
manuales de actualización, igual que pasaba hoy antes de esta feature.

**Independent Test**: se puede probar cambiando el contenido de la
plantilla embebida (simulando una corrección), reconstruyendo el binario, y
corriendo `mem install` de nuevo sobre un proyecto que ya tenía la versión
anterior — el archivo debe quedar actualizado.

**Acceptance Scenarios**:

1. **Given** un proyecto con una versión anterior del brazo extensor ya
   instalada, **When** se ejecuta `mem install` con una versión de
   gomemory que trae una corrección al brazo extensor, **Then** los
   archivos que cambiaron respecto a la versión embebida se actualizan.
2. **Given** un archivo del brazo extensor que ya coincide exactamente con
   la versión embebida actual, **When** se ejecuta `mem install` de nuevo,
   **Then** ese archivo no se reescribe (sin ruido innecesario).

---

### Edge Cases

- ¿Qué pasa si el proyecto tiene `.specify/` pero nunca se instaló ninguna
  extensión antes (sin `.specify/extensions/` todavía)? Debe poder crearse
  la carpeta completa igual, sin depender de que ya exista.
- ¿Qué pasa si la persona había personalizado a mano un archivo del brazo
  extensor (por ejemplo, cambió el hook `before_specify` de mandatorio a
  opcional en su copia) y ese archivo ya no coincide con la versión
  embebida? Se sobrescribe con la versión actual — mismo comportamiento ya
  aceptado hoy para el resto de artefactos que `mem install` gestiona
  (p. ej. el plugin de OpenCode): son archivos del framework, no contenido
  propio del proyecto como la constitución.
- ¿Qué pasa en un agente que gomemory hoy solo configura de forma parcial
  (Cursor, Windsurf, Cline, Codex — solo reciben configuración MCP, no
  plugin completo)? Queda fuera del alcance de esta iteración; no reciben
  el brazo extensor todavía.
- ¿Qué pasa si `mem install` se ejecuta dos veces seguidas sin cambios de
  por medio? La segunda corrida no debe reportar ni reescribir nada
  distinto a la primera (idempotente).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `mem install` DEBE, cuando el proyecto destino tenga
  spec-kit inicializado, colocar los archivos fuente de la extensión
  gomemory-context en el proyecto destino, sin exigir que la persona tenga
  instalada ninguna herramienta externa de spec-kit para lograrlo.
- **FR-002**: `mem install` DEBE dejar el brazo extensor operativo para
  Claude Code inmediatamente después de instalar, sin pasos manuales
  adicionales.
- **FR-003**: `mem install` DEBE dejar el brazo extensor igualmente
  operativo para OpenCode inmediatamente después de instalar, con el mismo
  nivel de soporte que Claude Code (mismo comportamiento automático,
  misma degradación transparente si gomemory no tiene datos).
- **FR-004**: En un proyecto sin spec-kit inicializado, `mem install` NO
  DEBE crear ningún archivo relacionado con esta extensión — el
  comportamiento debe ser idéntico al que existe hoy sin esta feature.
- **FR-005**: Las actualizaciones de gomemory DEBEN poder propagar
  correcciones del brazo extensor a proyectos que ya lo tenían instalado,
  en la siguiente ejecución de `mem install`, sin intervención manual.
- **FR-006**: Un archivo del brazo extensor ya instalado que coincide
  exactamente con la versión embebida actual NO DEBE reescribirse en
  corridas repetidas de `mem install`.
- **FR-007**: El funcionamiento del brazo extensor en Claude Code y
  OpenCode NO DEBE depender de que la persona tenga instalada la CLI de
  terceros `specify` — solo de tener `mem install` corriendo, igual que el
  resto de la instalación de gomemory.
- **FR-008**: Esta feature NO DEBE alterar el comportamiento ya
  establecido en la spec 011 para el interruptor de encendido/apagado del
  brazo extensor (`SpeckitContextDisabled`) ni para su degradación
  transparente cuando gomemory no está disponible o no tiene datos.

### Key Entities

- **Plantillas embebidas del brazo extensor**: copia versionada de los
  archivos de la extensión gomemory-context (fuente + artefactos ya listos
  para Claude Code y OpenCode) empaquetada dentro del binario de gomemory,
  análoga a la plantilla de constitución ya embebida hoy.
- **Artefacto por agente**: la forma final que cada agente reconoce para
  ejecutar el hook `before_specify` — una para Claude Code, otra para
  OpenCode — derivada de la misma extensión fuente.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Una persona que corre `mem install` en un proyecto con
  spec-kit ya instalado obtiene el resumen de historial automáticamente en
  su primera especificación nueva con Claude Code, sin pasos manuales.
- **SC-002**: La misma persona, usando OpenCode en vez de Claude Code,
  obtiene el mismo resultado, sin diferencias perceptibles entre un agente
  y otro.
- **SC-003**: Una persona en un proyecto sin spec-kit no percibe ningún
  archivo ni cambio adicional tras `mem install` relacionado con esta
  capacidad.
- **SC-004**: Tras actualizar gomemory, los proyectos que ya tenían el
  brazo extensor instalado reciben las correcciones más recientes en su
  siguiente `mem install`, sin intervención manual.
- **SC-005**: Nadie necesita instalar herramientas adicionales, fuera de
  gomemory mismo, para que el brazo extensor funcione en Claude Code u
  OpenCode.

## Assumptions

- El alcance de "todos los agentes que gomemory soporta de forma nativa"
  para esta iteración son **Claude Code y OpenCode**: los dos únicos que
  hoy reciben instalación completa (plugin + hooks) vía `mem install`. Los
  demás agentes soportados (Cursor, Windsurf, Cline, Codex) hoy solo
  reciben configuración MCP y quedan fuera de esta iteración.
- Se reutiliza el mecanismo ya existente de plantillas embebidas en el
  binario (el mismo usado hoy para `speckit-constitution-gen.md` y para el
  plugin de OpenCode) — no se introduce un mecanismo de distribución
  nuevo.
- El criterio de actualización sigue el mismo ya establecido y verificado
  hoy en el instalador para artefactos gestionados por el framework
  (p. ej. el plugin de OpenCode): si el archivo instalado coincide con la
  plantilla embebida, no se toca; si difiere (versión anterior o editado a
  mano), se sobrescribe con la versión actual. A diferencia de la
  constitución (contenido propio del proyecto, nunca se sobrescribe), los
  archivos de esta extensión son del framework.
- El registro formal de la extensión ante herramientas de terceros de
  spec-kit (por ejemplo, que `specify extension list` la reconozca) sigue
  siendo un mecanismo aparte y opcional — esta feature no lo reemplaza, y
  el brazo extensor debe funcionar en Claude Code/OpenCode con o sin ese
  registro.
- Los cambios de esta feature son aditivos sobre la spec 011: no se
  modifica el contrato de comportamiento del script del hook
  (`speckit_context_disabled`, degradación transparente, solo lectura),
  solo cómo llegan sus archivos a cada proyecto.
