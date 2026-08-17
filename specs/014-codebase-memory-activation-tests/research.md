# Research: codebase-memory-mcp activation regression tests

**Date**: 2026-08-08
**Feature**: 014-codebase-memory-activation-tests

## R1: ¿Cómo se prueba el hook de Claude Code sin un agente real?

**Decision**: Invocar el binario `mem hook user-prompt-submit <dir>` como subproceso y parsear el JSON que imprime a stdout.

**Rationale**: El handler `hookUserPromptSubmit` termina con `os.Exit(0)`, así que solo funciona como subproceso real (no in-process). La salida es JSON con `hookSpecificOutput.additionalContext` o `systemMessage`. Este es el mismo patrón que usan los tests de integración existentes (`tests/integration/hook_marker_integration_test.go`).

**Alternatives considered**:
- Test Go unitario: ya existe en `cmd_hook_bootstrap_test.go`, pero no cubre el JSON completo que Claude Code recibe
- Mock del stdin: innecesario — el hook lee un marker file, no stdin para decidir la rama

## R2: ¿Cómo se valida el plugin OpenCode sin un agente OpenCode?

**Decision**: Búsqueda de strings (`grep -qF`) en `infrastructure/plugin/opencode/gomemory.ts`.

**Rationale**: El plugin inyecta el protocolo como texto plano vía `chat.system.transform`. No hay runtime que ejecutar — basta verificar que los nombres de tools aparecen con el prefijo correcto (`codebase-memory-mcp_`).

**Alternatives considered**:
- Parsear el TypeScript: sobre-ingeniería — el protocolo es texto constante, no lógica condicional
- Test de contrato TypeScript: no existe infraestructura para ello en el proyecto

## R3: ¿Cómo se prueba el canal AGENTS.md/.cursorrules?

**Decision**: Verificar que `cmd_install.go` contiene la referencia a `codebase-memory-mcp` y la variable Go `CodebaseMemoryMCPDiscoveryTools`.

**Rationale**: `buildIntegrationBlock()` genera el texto que se inserta en AGENTS.md. Si la referencia existe en el código fuente, el texto generado la incluirá. No es necesario compilar e inspeccionar el output — el test de contrato existente (`mcp_tool_sync_test.go`) ya verifica que la lista de tools coincide.

**Alternatives considered**:
- Ejecutar `mem install` y verificar el archivo generado: lento, frágil, requiere directorio temporal con permisos

## R4: ¿Cuántos checks en total?

**Decision**: 50 checks distribuidos en 7 secciones.

**Rationale**: Cada sección valida un canal o contrato. El conteo se deriva de:
- Claude Code hook: 1 (JSON válido) + 1 (systemMessage ausente) + 1 (additionalContext) + 6 (tools) + 4 (admin ausentes) + 1 (get_plan_context) = 14
- Subagentes: 1 + 1 + 6 = 8
- OpenCode: 1 + 6 + 1 + 1 = 9
- Integración: 3
- MCP: 1
- Contrato gomemory: 15
- Total: 14 + 8 + 9 + 3 + 1 + 15 = 50

## R5: ¿Qué pasa si se añade una tool nueva?

**Decision**: El script falla con mensaje descriptivo indicando qué tool falta.

**Rationale**: Es el comportamiento deseado de una regresión — detectar cambios no intencionados. Si se añade una tool deliberadamente, se actualiza el script (o se añade al array `TOOLS`/`gomemory_tools`).

**Alternatives considered**:
- Ignorar tools nuevas: perdería la capacidad de detectar regressions
- Modo "warn" en vez de "fail": debilitaría la barrera de calidad
