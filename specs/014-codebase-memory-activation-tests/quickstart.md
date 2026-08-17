# Quickstart: codebase-memory-mcp activation regression tests

**Date**: 2026-08-08

## Prerrequisitos

- Go toolchain (para compilar `mem`)
- Python3 (para validación de JSON)
- Bash 5+
- Estar en la raíz del proyecto `go_memory`

## Ejecución

```bash
./scripts/test-codebase-memory-activation.sh
```

## Salida esperada

```
═══ Test: activación de codebase-memory-mcp en todos los canales ═══

1. Compilando binario...
  ✓ Binario compilado en /tmp/.../mem-test

2. Claude Code — hook user-prompt-submit (bootstrap)
  ✓ systemMessage ausente (correcto)
  ✓ additionalContext presente
  ✓ Claude Code: search_graph en select:
  ... (14 checks)

3. Claude Code — hook subagent-start (subagentes)
  ✓ subagent: systemMessage ausente
  ... (8 checks)

4. OpenCode — plugin gomemory.ts (EXTERNAL CODE GRAPH)
  ✓ OpenCode: sección EXTERNAL CODE GRAPH
  ... (9 checks)

5. Integración — buildIntegrationBlock (AGENTS.md/.cursorrules)
  ✓ install: referencia a codebase-memory-mcp
  ... (3 checks)

6. MCP instructions — cmd_mcp.go
  ✓ MCP: instructions usa buildIntegrationBlock

7. Contrato — todas las tools de gomemory en bootstrap
  ✓ contrato: gomemory save_memory
  ... (15 checks)

═══ Resultado ═══
  Pasaron: 50
  Fallaron: 0
```

## Verificación de regressions

### Regression v2.3.3: bootstrap en systemMessage

Si `cmd_hook.go` mueve el bootstrap de `additionalContext` a `systemMessage`:
- **Falla**: `systemMessage ausente (correcto)` en sección 2
- **Falla**: `subagent: systemMessage ausente` en sección 3

### Regression v2.3.1: prefijos incorrectos en OpenCode

Si `gomemory.ts` usa `get_context` en vez de `gomemory_get_context`:
- **Falla**: cualquiera de los 6 checks `OpenCode: <tool> nombrado` en sección 4

### Regression v2.3.0: tools faltantes del bootstrap

Si `cmd_hook.go` remueve codebase-memory-mcp del `select:`:
- **Falla**: los 6 checks `Claude Code: <tool> en select:` en sección 2
- **Falla**: los 6 checks `subagent: <tool> en select:` en sección 3

### Regression: tool de gomemory sin actualizar

Si se añade una tool a `domain.MCPAllTools()` sin actualizar el bootstrap:
- **Falla**: el check correspondiente en sección 7 (`contrato: gomemory <tool>`)

## Integración con CI

Para ejecutar como parte de un pipeline:

```bash
./scripts/test-codebase-memory-activation.sh
# exit code 0 = todos pasaron, exit code 1 = al menos uno falló
```

## Troubleshooting

| Problema | Causa | Solución |
|----------|-------|----------|
| `go build` falla | Error de compilación en el proyecto | Corregir el error de Go primero |
| `python3` no encontrado | Python3 no está en PATH | Instalar Python3 o usar jq como alternativa |
| Check de OpenCode falla | `gomemory.ts` fue modificado | Verificar que los nombres de tools tienen el prefijo `codebase-memory-mcp_` |
