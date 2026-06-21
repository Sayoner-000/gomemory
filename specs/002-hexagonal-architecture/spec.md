# Feature Specification: Reorganización a Arquitectura Hexagonal

**Feature Branch**: `002-hexagonal-architecture`

**Created**: 2026-06-21

**Status**: Draft

**Input**: User description: "organicemos los archivos de raíz del .go, y darle una estructura hexagonal para este proyecto ya que todo está regado"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Migración con Preservación Funcional (Priority: P1)

Como mantenedor del proyecto, quiero reorganizar los archivos Go en una estructura hexagonal sin romper la funcionalidad existente, para que el código sea más mantenible y las dependencias estén claramente delimitadas.

**Why this priority**: Sin esta migración, el proyecto mezcla capas en archivos planos en la raíz, violando el Principio I de la constitución (Arquitectura Hexagonal). La prioridad máxima es preservar toda funcionalidad existente durante la migración.

**Independent Test**: Todos los comandos CLI (`mem init`, `mem save`, `mem search`, etc.) y el servidor MCP funcionan exactamente igual después de la migración. Los tests existentes pasan sin modificaciones.

**Acceptance Scenarios**:

1. **Given** el proyecto en su estado actual (archivos planos en raíz), **When** se ejecuta la reorganización hexagonal, **Then** todos los comandos CLI existentes funcionan idénticamente.
2. **Given** la nueva estructura hexagonal, **When** se compila el proyecto con `go build`, **Then** el binario se genera sin errores.
3. **Given** la nueva estructura hexagonal, **When** se ejecutan los tests existentes (`go test ./...`), **Then** todos pasan sin modificaciones.
4. **Given** la nueva estructura hexagonal, **When** se intenta importar un adaptador concreto desde la capa de dominio, **Then** el compilador rechaza la dependencia.

---

### User Story 2 — Separación Clara por Capas (Priority: P2)

Como desarrollador del proyecto, quiero que cada archivo Go tenga un lugar inequívoco según su rol hexagonal, para que sea obvio dónde agregar nuevo código y qué dependencias están permitidas.

**Why this priority**: La razón principal de la reorganización es la mantenibilidad. Sin una estructura clara, los nuevos contribuidores (incluyendo agentes AI) no saben dónde poner cada cosa y se violan las reglas de dependencia.

**Independent Test**: La estructura de directorios resultante refleja las 4 capas hexagonales (dominio, aplicación, adaptadores, infraestructura). Cada paquete tiene una responsabilidad única y verificable.

**Acceptance Scenarios**:

1. **Given** el proyecto reorganizado, **When** se inspecciona la estructura de directorios, **Then** existen las 4 capas hexagonales con nombres estándar del proyecto.
2. **Given** la nueva estructura, **When** se examina el archivo `go.mod`, **Then** el módulo raíz no ha cambiado.
3. **Given** la capa de dominio, **When** se revisan sus imports, **Then** no importa nada de infraestructura, adaptadores ni store.

---

### User Story 3 — Composition Root Centralizado (Priority: P3)

Como arquitecto del proyecto, quiero que el wiring de dependencias ocurra en un solo lugar (composition root), para que intercambiar implementaciones (ej. mock adapters vs reales) sea trivial, tal como lo exige la constitución.

**Why this priority**: Sin un composition root, las dependencias se wirean ad-hoc en cada comando, dificultando el testing y el intercambio de adaptadores. No es blocker de la migración inicial pero es necesario para cumplir la constitución.

**Independent Test**: Cambiando una variable de entorno `USE_MOCK_ADAPTERS=true` se intercambia todo el stack de adaptadores sin cambiar código.

**Acceptance Scenarios**:

1. **Given** la nueva estructura, **When** se busca un solo punto de wiring de dependencias, **Then** existe una función/clase central que construye todas las dependencias.
2. **Given** el composition root, **When** se revisan los comandos CLI, **Then** ninguno construye sus propias dependencias directamente; todas vienen del composition root.

---

### Edge Cases

- ¿Qué pasa con archivos que mezclan responsabilidades de múltiples capas (ej. un `cmd_*.go` que hace llamadas directas a BD)? Deben dividirse en múltiples archivos.
- ¿Cómo manejar paquetes existentes como `store/` que actualmente son implementación concreta pero deberían ser un adaptador? Se mueven a `adapters/secondary/` como implementación, con una interfaz en `application/ports/`.
- ¿Qué ocurre con `types/types.go` que es dominio puro pero está en un paquete separado? Se mueve a `domain/`.
- ¿Qué pasa con `main.go` que actualmente es dispatcher CLI y composition root? Se refactoriza para que sea únicamente composition root.
- ¿Cómo migrar sin romper el historial git de los archivos? Se prioriza `git mv` sobre crear nuevos archivos desde cero.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE reorganizar los archivos Go en 4 capas hexagonales: `domain/`, `application/`, `adapters/`, `infrastructure/`.
- **FR-002**: La capa `domain/` DEBE contener solo tipos puros, reglas de negocio y errores de dominio, sin imports de infraestructura ni I/O.
- **FR-003**: La capa `application/` DEBE contener puertos (interfaces) y casos de uso, y solo puede importar `domain/`.
- **FR-004**: La capa `adapters/` DEBE contener adaptadores primarios (CLI, TUI, HTTP/MCP server) y secundarios (persistencia, setup), implementando las interfaces de `application/ports/`.
- **FR-005**: La capa `infrastructure/` DEBE contener el composition root (wiring de dependencias), configuración global, y el archivo `main.go` reducido a bootstrap mínimo.
- **FR-006**: El wiring de dependencias DEBE ocurrir en UN SOLO LUGAR dentro de `infrastructure/`, sin frameworks de DI.
- **FR-007**: Los 14 archivos `cmd_*.go` actuales en la raíz DEBEN migrarse a `adapters/primary/cli/` como un solo paquete Go o submódulos, según conveniencia de compilación.
- **FR-008**: El paquete `store/` actual DEBE migrarse a `adapters/secondary/persistence/` implementando interfaces definidas en `application/ports/`.
- **FR-009**: El paquete `types/` actual DEBE migrarse a `domain/` ya que contiene tipos puros de dominio.
- **FR-010**: El paquete `tui/` actual DEBE migrarse a `adapters/primary/tui/`.
- **FR-011**: El paquete `context/` actual DEBE migrarse (según su responsabilidad: si es lógica de negocio va a `application/`, si es adaptador va a `adapters/`).
- **FR-012**: El paquete `internal/server/` actual DEBE migrarse a `adapters/primary/mcp/` como adaptador de servidor MCP.
- **FR-013**: El paquete `internal/setup/` actual DEBE migrarse a `adapters/primary/setup/` o `adapters/secondary/` según su responsabilidad.
- **FR-014**: El archivo `main.go` DEBE reducirse al mínimo: parseo de flags, carga de configuración, wiring de dependencias y dispatch al adaptador correspondiente.
- **FR-015**: La migración DEBE usar `git mv` para preservar el historial de archivos.
- **FR-016**: La migración NO DEBE modificar tests existentes ni su organización actual en `tests/`.
- **FR-017**: El sistema DEBE compilar y pasar todos los tests existentes después de la migración.
- **FR-018**: El archivo `go.mod` NO DEBE cambiar su módulo raíz.
- **FR-019**: Los adaptadores primarios (CLI, TUI, MCP) DEBEN usar las interfaces de `application/ports/` para acceder a la capa de aplicación, nunca imports directos a adaptadores secundarios.

### Key Entities *(include if feature involves data)*

- **Dominio**: Tipos puros (`Memory`, `Session`, `MemoryType`, etc.) y reglas de negocio sin dependencias externas.
- **Puerto (interfaz)**: Contrato que define cómo la aplicación interactúa con el exterior (ej. `MemoryRepository`, `SessionManager`).
- **Caso de Uso**: Operación orquestada que coordina puertos para cumplir un objetivo de negocio.
- **Adaptador Primario (Driver)**: Implementación de un puerto que recibe input del exterior (CLI, TUI, HTTP, MCP).
- **Adaptador Secundario (Driven)**: Implementación de un puerto que ejecuta operaciones hacia el exterior (SQLite, sistema de archivos, comandos).
- **Composition Root**: Punto único donde se construyen y wirean todas las dependencias del sistema.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Los 14 comandos CLI existentes (`mem init`, `mem save`, `mem search`, etc.) funcionan exactamente igual que antes de la migración.
- **SC-002**: El binario se compila sin errores con `go build ./...`.
- **SC-003**: Todos los tests existentes pasan sin modificaciones: `go test ./...` retorna éxito.
- **SC-004**: La estructura de directorios refleja las 4 capas hexagonales: `domain/`, `application/`, `adapters/`, `infrastructure/`.
- **SC-005**: No existe ningún archivo `.go` en la raíz del proyecto después de la migración (excepto `main.go` que se mueve a `infrastructure/`).
- **SC-006**: Ningún archivo en `domain/` importa paquetes fuera de la stdlib que no sean también de dominio.
- **SC-007**: No hay código duplicado: cada tipo, función o constante existe en exactamente un lugar.
- **SC-008**: El historial git de los archivos migrados se preserva mediante `git mv`.

## Assumptions

- La migración es puramente estructural: no se agrega nueva funcionalidad ni se cambia lógica existente.
- Los archivos se mueven con `git mv` para preservar el historial.
- Los tests existentes en `tests/` no se modifican ni mueven.
- El módulo Go (`go.mod`) mantiene su nombre actual.
- Los packages `store/`, `types/`, `context/`, `tui/` existen actualmente como directorios y se migran completos (no se dividen en sub-paquetes a menos que su tamaño lo justifique).
- No se introducen frameworks de DI ni librerías externas nuevas.
- El `main.go` actual contiene lógica de dispatcher que puede coexistir temporalmente con la nueva estructura hasta una refactorización futura del composition root.
- Los archivos `cmd_*.go` comparten el mismo package `main` y dependen entre sí — se migran como un solo paquete dentro de `adapters/primary/cli/`.
- La estructura de `internal/` se deshace: `internal/server/` → `adapters/primary/mcp/`, `internal/setup/` → `adapters/primary/setup/`.
