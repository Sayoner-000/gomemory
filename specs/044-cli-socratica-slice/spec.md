# Feature Specification: Entradas CLI y MCP de Socrática como slice

**Feature Branch**: `044-cli-socratica-slice`  
**Created**: 2026-10-26  
**Status**: Complete  
**Input**: Continuación del H-04 opcional de `specs/043-vertical-slices-assessment`: extraer las entradas CLI y MCP de Socrática siguiendo el piloto de Review.

## User Scenarios & Testing

### User Story 1 - Usar Socrática desde la CLI (Priority: P1)

Como persona que usa `mem socratica`, quiero que el comando siga ofreciendo sus operaciones actuales después de aislar su entrada, para usar Socrática con el mismo comportamiento y sin acoplarla a toda la CLI.

**Why this priority**: La CLI es la interfaz principal del slice y contiene la mayor parte del comportamiento que debe conservarse.

**Independent Test**: Ejecutar las pruebas existentes de Socrática y comparar salidas y códigos de salida representativos del binario anterior y el nuevo.

**Acceptance Scenarios**:

1. **Given** una invocación válida de Socrática, **When** se despacha desde la CLI, **Then** produce la misma salida observable y código de salida.
2. **Given** `expand evolve` solicita elegir un harness, **When** usa el selector compartido, **Then** conserva las opciones disponibles, exclusiones y cancelación.
3. **Given** una entrada por stdin supera el límite, **When** se procesa, **Then** falla con el mismo error y no se trunca ni se envía al generador.

### User Story 2 - Usar el flujo MCP de Socrática (Priority: P2)

Como cliente MCP, quiero seguir invocando `socratica_flow` con su contrato actual después de aislar el registro de herramientas, para preparar y operar flujos sin cambios de protocolo.

**Why this priority**: MCP es otra entrada pública del mismo slice; se extrae tras estabilizar la entrada CLI y el kit compartido.

**Independent Test**: Ejecutar las pruebas de integración del servidor MCP y verificar el nombre y las respuestas de `socratica_flow`.

**Acceptance Scenarios**:

1. **Given** el servidor MCP se inicia, **When** registra sus herramientas, **Then** `socratica_flow` aparece exactamente una vez.
2. **Given** una petición MCP válida para Socrática, **When** se ejecuta, **Then** conserva las respuestas, errores y protecciones actuales.
3. **Given** un flujo `specify` en cualquier etapa, **When** se opera a través de MCP, **Then** validación, revisión, aprobación, ack y report conservan sus contratos actuales.

### Edge Cases

- No hay generador disponible o la persona cancela la selección de harness.
- El proceso no tiene TTY, stdin excede el límite o contiene texto de terminal.
- Se solicitan acciones MCP con revisión obsoleta, entrada inválida, flujo ausente o autorización ya consumida.
- `expand evolve` debe seguir eligiendo el harness aunque el paquete Socrática ya no sea parte del paquete padre.

## Requirements

### Functional Requirements

- **FR-001**: El sistema MUST ofrecer `mem socratica` con los mismos subcomandos, resultados, errores y códigos de salida que antes de la extracción.
- **FR-002**: El sistema MUST ofrecer la herramienta MCP `socratica_flow` una sola vez y preservar su esquema y comportamiento.
- **FR-003**: La selección de harness MUST conservar catálogo, exclusiones, mensajes y cancelación en las entradas CLI que la usan.
- **FR-004**: La lectura acotada MUST mantener el límite, error y rechazo sin truncamiento para CLI y hooks.
- **FR-005**: Los paquetes del slice Socrática MUST depender únicamente de `cli/clikit` para utilidades de CLI compartidas; no dependerán del paquete padre ni de otros slices.
- **FR-006**: Los casos de uso, reglas de dominio, persistencia, TUI y comportamiento de hooks MUST conservar sus contratos existentes.
- **FR-007**: El kit compartido MUST permanecer libre de dependencias en paquetes de slices.

## Success Criteria

### Measurable Outcomes

- **SC-001**: El 100 % de las pruebas actuales de CLI y MCP de Socrática pasan tras mover las pruebas internas al slice.
- **SC-002**: La guarda de capas pasa y no detecta dependencias del slice hacia `cli` padre u otros slices.
- **SC-003**: En los escenarios comparativos acordados, el 100 % de salidas y códigos de salida coinciden tras normalizar IDs y marcas de tiempo variables.
- **SC-004**: La batería de tests, race, vet y build del proyecto termina satisfactoriamente.

## Assumptions

- La extracción es una refactorización sin cambio de comportamiento ni migración de datos.
- El piloto de `cli/review` y `cli/clikit` es el patrón arquitectónico aceptado.
- Se incluye la adaptación mínima de `expand evolve` al selector compartido porque hoy lo llama mediante una estructura privada de Socrática.
- La extracción de otras entradas CLI y la publicación quedan fuera de esta entrega.
