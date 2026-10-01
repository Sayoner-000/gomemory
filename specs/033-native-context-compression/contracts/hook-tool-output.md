# Contrato: `mem hook tool-output <runtime>`

Comprime la salida de una herramienta antes de que la vea el modelo (FR-022).
Es **opt-in**: solo se instala y solo actúa con `tool_output_compression=true`.

## Invariantes comunes
- **H1. Nunca rompe la herramienta.** Si hay cualquier error, se supera el
  tiempo (150 ms), la herramienta está excluida o la salida queda por debajo
  del umbral (400 tokens), escribe **salida vacía** y termina con código 0.
  El runtime usa entonces la salida original.
- **H2. Misma forma.** Solo sustituye valores de **texto** dentro de
  `tool_response`; no añade, quita ni renombra claves, ni cambia tipos.
- **H3. Exclusiones fijas:** `Read`, `Edit`, `Write`, `MultiEdit`,
  `NotebookEdit`, `mcp__gomemory__*` y las herramientas propias de gomemory en
  otros runtimes.
- **H4. Privacidad:** si el texto contiene `<private>` o secretos detectados,
  se comprime sin pérdida (sin refs) y no se guarda ningún original.
- **H5. Estadísticas:** registra el uso con `origin=tool_output`.

## Claude Code (verificado en 2.1.282)
Registro en la instalación, **solo con el ajuste activo**:
`PostToolUse`, matcher `Bash|Grep|Glob|WebFetch|WebSearch|mcp__.*` → `mem hook tool-output claude`.

Entrada (stdin): el evento PostToolUse (`tool_name`, `tool_input`,
`tool_response`, `session_id`, …).

Salida (stdout), solo si hay ganancia:
```json
{"hookSpecificOutput":{"hookEventName":"PostToolUse","updatedToolOutput": <tool_response con los campos de texto comprimidos>}}
```
Campos de texto que se reescriben, según la forma recibida: `stdout`,
`stderr`, `output`, `content` (string) o `content[].text`. Cualquier otra
forma → H1. Si la forma no coincide, Claude Code descarta
`updatedToolOutput` por sí mismo ("does not match … output shape"); H2 evita
llegar a ese caso.

## Codex: no soportado
Codex rechaza **siempre** `hookSpecificOutput.updatedMCPToolOutput`: su parser
(`codex-rs/hooks/src/engine/output_parser.rs`,
`unsupported_post_tool_use_hook_specific_output`) marca el hook como fallido con
«PostToolUse hook returned unsupported updatedMCPToolOutput» y deja pasar la
salida original (comprobado en codex-cli 0.159.0). La premisa de que 0.157.0 lo
admitía era falsa: el campo se deserializa, pero se rechaza.
Por eso `mem hook tool-output codex` no emite nada (H1), la instalación ya no
registra el hook y retira el que dejaron versiones anteriores. `mem doctor`
informa «no soportado».

## OpenCode (plugin `infrastructure/plugin/opencode/gomemory.ts`)
- **v1 (1.18.x)**: dentro del `tool.execute.after` que ya existe, y además
  de la captura de `task`, si el ajuste está activo y la herramienta no está
  excluida, llama a `mem hook tool-output opencode` con
  `{"tool", "output": output.output}` por stdin y, si recibe
  `{"output": "<comprimido>"}`, asigna `output.output`. Se valida con el
  binario real en quickstart Q7.
- **v2 (2.x)**: se habilita solo si Q7 demuestra que mutar el resultado en
  `ctx.tool.hook("execute.after")` cambia lo que ve el modelo. Si no, se queda
  en «no soportado» en `mem doctor` y el plugin no lo invoca.
- El plugin lee el ajuste una vez por sesión (`mem hook tool-output --enabled`
  → `true|false`), para no lanzar un proceso por herramienta cuando está
  apagado.

## Pruebas de contrato (`tests/contract/`)
- Fixtures reales de `tool_response` capturados de cada runtime: Bash, Grep,
  WebFetch y MCP en Claude; MCP en Codex (salida vacía esperada); y la salida de `bash` en OpenCode.
- Para cada fixture: la forma de salida es igual a la de entrada (mismas
  claves y tipos) y el texto tiene menos tokens.
- Para cada exclusión: salida vacía.
- Con el tiempo agotado (se inyecta un reloj lento): salida vacía y código 0.
