# Contrato: estado del hook OpenCode en `mem doctor`

Origen: FR-004 (US2). Punto de código: `toolOutputHookStates` en `adapters/primary/cli/cmd_doctor_compression.go`.

## Forma (estable, no cambia)

- Texto: línea `hook de salidas (opencode): <estado>` en la sección `Compresión`.
- JSON (`doctor --json`, clave `compression.tool_output_hooks.opencode`): string.

## Valores admitidos

- Compresión de salidas desactivada: `inactivo`.
- Activada: el estado DEBE describir emisión best-effort y contener la palabra `best-effort`, advirtiendo que la sustitución final depende del runtime.
- PROHIBIDO: `no soportado` o `sin verificar` como estado de emisión (contradicen la verificación [378]).

## Verificación

- Test unitario junto a `TestToolOutputHookStates_CodexSoloEnOrigen`: `toolOutputHookStates(dir, true)["opencode"]` contiene `best-effort`.
- Manual: binario fresco con el plugin instalado → `mem doctor` muestra el estado sin marcarlo como problema.
