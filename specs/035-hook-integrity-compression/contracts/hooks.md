# Contrato: hooks de gomemory (cambios de la feature 035)

Contrato observable de los subcomandos `mem hook …` que cambian. Lo que no aparece aquí se comporta como en v2.27.1.

## Reglas comunes (todos los eventos de inyección)

- **C1. Tope**: el texto que llega al modelo (`additionalContext` en el dialecto claude/json; stdout en text/neutral) tiene ≤ 10 000 runas.
- **C2. Recorte**: si hubo que recortar, el texto termina en `… contexto recortado: llama get_context() para el resto` y se registra `budget_trimmed` con `orig=N,emit=M`.
- **C3. Reentrada**: una segunda invocación del mismo evento con el mismo stdin **mientras la primera conserva el lock del SO** produce la salida neutra de su dialecto (`{}` en claude/json, vacío en text/neutral), no tiene efectos (sin checkpoint, sin escrituras de estado) y registra `duplicate_dropped`. En `user-prompt-submit`, si hay `prompt_id`, la misma salida neutra se aplica a una copia que llegue hasta 1 h después de que el handler haya completado. Un `prompt_id` diferente se procesa aunque el texto sea idéntico. Sin `prompt_id`, las invocaciones secuenciales se procesan con normalidad.
- **C4. Fallo**: cualquier error interno produce la salida neutra y código 0 (sin cambios respecto a lo vigente).

## `mem hook session-start`

| Entrada | Efecto | Salida |
|---|---|---|
| `session_id` distinto del de `.conversation`, o sin archivo | `BeginConversation` (reinicio del estado por turno y rotación si la sesión lleva > 4 h inactiva) | contexto ajustado (C1) |
| mismo `session_id` (reanudación) | sin reinicio | contexto ajustado (C1) |
| sin `session_id`, salvo `source=resume` | nueva conversación local; se limpia el estado por turno | contexto ajustado (C1) |

`mem hook session-end` y `mem session end` eliminan `.conversation`; el siguiente inicio sin id crea una conversación local nueva. Si el hook de fin no llega, `SessionStart` sin id también abre una nueva, salvo `source=resume`.
| subcomandos de gomemory presentes a la vez en el `settings.json` global y en el del proyecto (Claude) | ninguna escritura en la configuración del host | además, `systemMessage`: «gomemory: hooks duplicados en usuario y proyecto (cada evento corre dos veces). Corrígelo con: mem update» |

- Solo se anota como entregado lo que se emitió.

## `mem hook user-prompt-submit [--emit=json|text]`

- **Primer prompt de la conversación**: secciones bootstrap > protocolo > Octopus (si está activo) > plan-doc (si `permission_mode=plan`) > aviso pendiente de esta conversación, ajustadas con C1.
  - El plan-doc omite la «Memoria del Proyecto» si el arranque de esta conversación ya la entregó.
- **Prompts siguientes**:
  - aviso pendiente (solo si su `conv_id` = conversación actual);
  - plan-doc si se acaba de entrar en plan;
  - recordatorio de guardado (con debounce, medido desde el inicio de la conversación);
  - recordatorio de modo plan: dialecto claude solo si no se emitió en esta conversación y se acaba de entrar en plan; json (Codex) en cada turno;
  - Octopus solo si `octopus_enabled` cambió respecto al último valor emitido.
- Un aviso pendiente de otra conversación se borra sin emitirse.

## `mem hook nudge` (OpenCode)

- Sin cambios en el recordatorio de modo plan (cada turno).
- Octopus sigue la regla de cambio de estado.
- El recordatorio de guardado se mide desde el inicio de la conversación.

## `mem hook turn-end [--emit=…]`

- Sin cambios de salida.
- El aviso pendiente se escribe como `{conv_id, text}` con la conversación actual.
- Se aplica C3.

## `mem hook tool-output <claude|opencode|codex>`

| Caso | Salida |
|---|---|
| Herramienta excluida: Read/Edit/Write/MultiEdit/NotebookEdit, `mcp__gomemory__*`, `mcp__codebase-memory-mcp__get_code_snippet`, o Bash/bash cuyo comando empiece por `sed `, `cat `, `head `, `tail `, `git diff`, `git show`, `mem doctor` | vacía + `tool_output_excluded` |
| Texto < 2 000 tokens aproximados | vacía |
| Bloque de prosa | intacto |
| Código (incluido Go sin `package` y código indentado) | cada línea conservada idéntica, con su indentación; nunca una marca en lugar de una sentencia |
| Array JSON con ≤ 50 elementos (a cualquier profundidad) | completo |
| Listado (≥ 30 líneas, ≥ 70 % `ruta:línea:` o ruta) | 15 primeras + 5 últimas + líneas de error, y marca ⟦mem⟧ con la ref y el conteo por archivo |
| Codex | vacía (sin cambios: Codex no admite la reescritura) |

- OpenCode envía `{"tool","output","command"}`; `command` es opcional.

## `mem doctor`

- ⚠ «hooks de gomemory duplicados en usuario y proyecto (<agente>): cada evento corre dos veces → mem update».
- Línea «protecciones (7 días)» por agente con los conteos de `duplicate_dropped`, `budget_trimmed` y `tool_output_excluded`; ⚠ si `duplicate_dropped` o `budget_trimmed` > 0.
- Codex: «hook de salidas (codex): solo en origen (Codex no permite reescribir salidas)».

## `mem install` / `mem update`

- En Claude Code, cada subcomando de hook de gomemory queda registrado **una sola vez** entre `~/.claude/settings.json` y `<proyecto>/.claude/settings.json`. El global (que sirve a todos los proyectos) nunca se toca desde un proyecto: el proyecto solo registra lo que el global no cubre (p. ej. `tool-output`) y retira los duplicados de su propio archivo. Los hooks ajenos quedan intactos.
- `mem update` aplica esa limpieza también cuando ya estás en la última versión.
- Codex (solo ámbito de usuario) y OpenCode (plugin único global) no tienen duplicación posible.

## `mem pack savings`

- Columnas `sin ganancia` y `degradaciones` separadas. `no_gain` solo suma en «sin ganancia».
