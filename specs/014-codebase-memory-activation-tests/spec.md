# Feature Specification: codebase-memory-mcp activation regression tests

**Feature Branch**: `014-codebase-memory-activation-tests`

**Created**: 2026-08-08

**Status**: Draft

**Input**: User description: "Script de regresión que valide que codebase-memory-mcp se activa automáticamente en los 4 canales de distribución (Claude Code hook, OpenCode plugin, integración AGENTS.md, subagentes), como polo a tierra para prevenir regressions futuras."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ejecutar regresión completa (Priority: P1)

Como desarrollador del proyecto gomemory, quiero ejecutar un solo comando que verifique que codebase-memory-mcp se activa correctamente en todos los canales de distribución, para detectar regressions antes de que lleguen a producción.

**Why this priority**: Es la historia principal — sin ella el script no existe. Cubre los 3 bugs históricos (v2.3.0 tools faltantes, v2.3.1 prefijos, v2.3.3 systemMessage vs additionalContext) como regresión permanente.

**Independent Test**: Se puede testear ejecutando `./scripts/test-codebase-memory-activation.sh` y verificando que termina con exit 0 y 50 checks pasados.

**Acceptance Scenarios**:

1. **Given** el binario compilado, **When** se ejecuta el script, **Then** compila mem, ejecuta todos los checks contra los 4 canales, y reporta pasaron/fallaron con exit code 0 si todos pasan.
2. **Given** una regresión en `cmd_hook.go` que remueve codebase-memory-mcp del bootstrap, **When** se ejecuta el script, **Then** falla al menos 1 check del canal Claude Code con mensaje descriptivo.
3. **Given** una regresión en `gomemory.ts` que cambia el prefijo de las tools, **When** se ejecuta el script, **Then** falla al menos 1 check del canal OpenCode.

---

### User Story 2 - Validar canal Claude Code hook (Priority: P1)

El script debe verificar que el hook `user-prompt-submit` del primer prompt de la sesión entrega las 6 tools de codebase-memory-mcp en `hookSpecificOutput.additionalContext`, y que NO usa `systemMessage` (campo que el modelo nunca ve).

**Why this priority**: Este fue el bug más grave (v2.3.3) — el bootstrap viajaba por el campo incorrecto y el agente nunca lo veía.

**Independent Test**: Ejecutar solo la sección 2 del script y verificar que los checks de `additionalContext` y `systemMessage` pasan.

**Acceptance Scenarios**:

1. **Given** una DB de sesión vacía (primer prompt), **When** se invoca `mem hook user-prompt-submit`, **Then** el JSON de salida contiene `hookSpecificOutput.additionalContext` con un `select:` que nombra las 6 tools: `mcp__codebase-memory-mcp__search_graph`, `trace_path`, `get_code_snippet`, `query_graph`, `get_architecture`, `search_code`.
2. **Given** el mismo hook, **When** se parsea la salida, **Then** NO existe la clave `systemMessage` en el JSON.
3. **Given** el mismo hook, **When** se revisa el `select:`, **Then** NO incluye operaciones admin (`index_repository`, `delete_project`, `manage_adr`, `ingest_traces`).
4. **Given** el mismo hook, **When** se revisa el `additionalContext`, **Then** incluye la instrucción `get_plan_context` para modo plan.

---

### User Story 3 - Validar canal subagentes (Priority: P2)

El script debe verificar que el hook `subagent-start` entrega el mismo bootstrap con codebase-memory-mcp en `additionalContext`, para que los subagentes (Explore, etc.) también tengan acceso al grafo de código.

**Why this priority**: Los subagentes arrancan en contexto aislado — sin bootstrap, recurren a grep/sed manual (bug reportado por el usuario en front-go).

**Independent Test**: Ejecutar solo la sección 3 del script y verificar que los 6 tools aparecen en el `select:` del subagente.

**Acceptance Scenarios**:

1. **Given** invocación de `mem hook subagent-start`, **When** se parsea la salida JSON, **Then** contiene `hookSpecificOutput.additionalContext` con las 6 tools de codebase-memory-mcp en un `select:`.
2. **Given** la misma invocación, **When** se revisa el JSON, **Then** NO existe `systemMessage`.

---

### User Story 4 - Validar canal OpenCode (Priority: P2)

El script debe verificar que el plugin `gomemory.ts` contiene la sección `EXTERNAL CODE GRAPH` que nombra las 6 tools con el prefijo `codebase-memory-mcp_` correcto para OpenCode.

**Why this priority**: OpenCode usa un mecanismo distinto (chat.system.transform, no hook JSON) — el prefijo debe ser exacto para que las tools resuelvan.

**Independent Test**: Ejecutar solo la sección 4 del script y verificar que `gomemory.ts` contiene las 6 tools nombradas.

**Acceptance Scenarios**:

1. **Given** el archivo `infrastructure/plugin/opencode/gomemory.ts`, **When** se busca la sección `EXTERNAL CODE GRAPH`, **Then** existe y nombra las 6 tools con prefijo `codebase-memory-mcp_`.
2. **Given** el mismo archivo, **When** se busca `PLAN MODE`, **Then** existe y referencia `get_plan_context`.

---

### User Story 5 - Validar canal integración AGENTS.md (Priority: P3)

El script debe verificar que `cmd_install.go` (generador de AGENTS.md/.cursorrules) referencia codebase-memory-mcp y la variable Go que lista las discovery tools.

**Why this priority**: Este canal cubre agentes sin hook propio (Cursor, Windsurf, Cline, Codex).

**Independent Test**: Ejecutar solo la sección 5 del script.

**Acceptance Scenarios**:

1. **Given** el archivo `adapters/primary/cli/cmd_install.go`, **When** se revisa `buildIntegrationBlock`, **Then** contiene la referencia a `codebase-memory-mcp` y la instrucción "exploración de código".

---

### User Story 6 - Validar contrato de tools gomemory (Priority: P3)

El script debe verificar que el `select:` del bootstrap incluye TODAS las 15 tools registradas por el servidor gomemory (no solo las de codebase-memory-mcp).

**Why this priority**: Si falta una tool de gomemory, el agente no puede invocarla — el test de contrato existente (`mcp_tool_sync_test.go`) ya lo cubre, pero el script lo refuerza como verificación integral.

**Independent Test**: Ejecutar solo la sección 7 del script.

**Acceptance Scenarios**:

1. **Given** el bootstrap del primer prompt, **When** se verifica cada tool de gomemory, **Then** las 15 aparecen en el `select:` con prefijo `mcp__gomemory__`.

---

### Edge Cases

- ¿Qué pasa si `go build` falla? → El script debe abortar con mensaje de error claro.
- ¿Qué pasa si el directorio `.memory` no existe? → El hook `session-start` lo crea; el script maneja esto.
- ¿Qué pasa si se añade una tool de codebase-memory-mcp nueva? → El script fallará indicando qué tool falta — es el comportamiento deseado (regresión).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El script MUST compilar el binario `mem` antes de ejecutar los checks.
- **FR-002**: El script MUST verificar el canal Claude Code hook (`user-prompt-submit`) contra una DB temporal.
- **FR-003**: El script MUST verificar el canal subagente (`subagent-start`).
- **FR-004**: El script MUST verificar el plugin OpenCode (`gomemory.ts`) buscando las 6 tools nombradas.
- **FR-005**: El script MUST verificar `cmd_install.go` (canal integración AGENTS.md).
- **FR-006**: El script MUST verificar `cmd_mcp.go` (canal MCP instructions).
- **FR-007**: El script MUST verificar que las 15 tools de gomemory aparecen en el bootstrap.
- **FR-008**: El script MUST reportar conteo de pasaron/fallaron y terminar con exit code 1 si algún check falla.
- **FR-009**: El script MUST limpiar binarios temporales y DBs al terminar.
- **FR-010**: Los checks MUST validar que `systemMessage` NO aparece en hooks dirigidos al agente (regresión v2.3.3).
- **FR-011**: Los checks MUST validar que operaciones admin de codebase-memory-mcp NO se incluyen en el bootstrap.
- **FR-012**: Los checks MUST validar la presencia de `get_plan_context` para soporte de modo plan.

### Key Entities

- **Canal de distribución**: Cada mecanismo por el que gomemory inyecta instrucciones al agente (hook Claude Code, plugin OpenCode, AGENTS.md, MCP instructions).
- **Tool de descubrimiento**: Las 6 tools de solo lectura de codebase-memory-mcp que se materializan para exploración de código.
- **Bootstrap**: El `select:` de ToolSearch que fuerza la carga de tools diferidas en el primer prompt.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Ejecutar `./scripts/test-codebase-memory-activation.sh` produce 50 checks, todos pasan, exit code 0.
- **SC-002**: Si se rompe el prefijo de codebase-memory-mcp en `gomemory.ts`, al menos 1 check falla en la sección OpenCode.
- **SC-003**: Si se mueve el bootstrap de `additionalContext` a `systemMessage`, al menos 1 check falla en la sección Claude Code.
- **SC-004**: Si se añade una tool de gomemory sin actualizar el bootstrap, al menos 1 check falla en la sección contrato.
- **SC-005**: El script completa en menos de 60 segundos (compilación + checks).

## Assumptions

- El script se ejecuta en el directorio raíz del proyecto go_memory.
- `go` está disponible en PATH.
- `python3` está disponible para validación de JSON (usado por `python3 -m json.tool`).
- Los 6 tools de codebase-memory-mcp son fijos y no cambiarán frecuentemente (son las de solo lectura del proveedor externo).
- Las 15 tools de gomemory son el conjunto estable del servidor MCP propio.
- El interruptor "Grafo de código externo" (CodeGraphDisabled) está por defecto OFF en tests, pero el script prueba la variante ON explícitamente.
