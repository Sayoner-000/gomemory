# HCD-015: integración comprobada en agy real

Fecha: 2026-11-02. `agy` 1.4.12, `-p`, `gemini-3.8-flash-medium`, autenticación nativa. Binario temporal con el código de esta unidad, SHA-256 `42b7fef5faacf852…`. Proyecto y datos de memoria temporales bajo `/private/tmp/hcd015-agy`.

## Procedimiento

`mem install --yes --agents agy` se ejecutó con un `HOME` temporal y escribió `mcp_config.json`, `hooks.json` y `skills.json`. En ese `HOME`, `mem doctor` informó `plan_entry` correcto y los límites de agy.

Con ese `HOME` agy exige iniciar sesión, y no se copiaron credenciales. Por eso las sesiones usaron el perfil real y el grupo generado se copió a `.agents/hooks.json` del proyecto temporal, cambiando el comando por un script que registra entrada y salida de `mem hook agy`. Los hooks globales de la persona también se ejecutaron, con datos temporales.

## Resultado

| Comprobación | Observado |
|---|---|
| Contexto | El modelo citó `HCD015_MEMORIA_REAL`, guardada antes de la sesión; llegó por `injectSteps` |
| Guarda | `git reset --hard` recibió `deny`; el modelo citó el motivo exacto de `destructive-shell` |
| Prompt | Una sola inyección por turno; la invocación siguiente respondió `{}` |
| Actividad | `Stop` creó el checkpoint `Comandos: printf HCD015_LIVE` |
| Permisos | `mem hook agy` responde `allow`; el script de prueba lo cambió a `auto_approve` solo para ese fixture, porque `allow` no ejecuta en `-p` |

La primera sesión destapó dos defectos, corregidos con prueba en rojo: el primer prompt llegaba con el sobre JSON de Claude, y `Stop` no registraba actividad porque el transcript de agy no tiene el formato de Claude.

## Límites

- No se instaló en el perfil real: el grupo manual sigue activo y `mem doctor` lo marca para reemplazarlo. Debe hacerse con el binario publicado; `~/.local/bin/mem` no conoce `mem hook agy`.
- Los hooks globales de la persona corrieron junto a los del proyecto, así que no se midió la ausencia de duplicados en el perfil real. Esa garantía procede de `TestHCD015MigrateManualGroup`.
- La recomendación inicial del primer prompt nombra `ToolSearch`, una herramienta de Claude Code.
- Modalidad `-p`, sin sesión interactiva, subagentes ni herramientas de edición.
