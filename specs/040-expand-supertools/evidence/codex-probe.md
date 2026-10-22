# Evidencia: sonda real de Codex (T004)

- **Fecha**: 2026-10-21
- **Codex**: `codex-cli 0.163.0`, modelo de la configuración de la persona
- **Aislamiento**: `CODEX_HOME` temporal en el scratchpad de la sesión. La credencial se enlazó (no se copió) y el enlace se borró al terminar. `~/.codex/config.toml` de la persona no se tocó.
- **Modo**: sesión **interactiva** (TUI en una PTY con `expect`, respondiendo a las consultas OSC 10/11 de la terminal) y una ejecución de control con `codex exec`.
- **Hooks de sonda**:
  - `PreToolUse` con matcher `.*`: añade el stdin a un log y responde `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"probe deny"}}` si el payload contiene `PROBE_DENY` o `probe_denied.txt`; en el resto de casos responde `{}`.
  - `PostToolUse` con matcher `.*`: solo registra.

## Resultados

| # | Petición | `tool_name` en PreToolUse | PostToolUse | Resultado observado |
|---|---|---|---|---|
| 0 | `codex exec "… ls"` (control) | — | — | **ningún hook disparado** (los dos logs vacíos): confirma que `codex exec` no ejecuta hooks |
| 1 | shell `echo PROBE_DENY` | `Bash` | no | **bloqueado**. Codex: «El comando `echo PROBE_DENY` fue bloqueado por el hook `PreToolUse` con el motivo `probe deny`. No se ejecutó.» |
| 2 | shell `ls` | `Bash` | sí | permitido con `{}` |
| 3 | editar `a.txt` con apply_patch | `apply_patch` (×2: un primer intento `Delete+Add` sin PostToolUse y luego `Update File`) | sí (`apply_patch`) | permitido; `a.txt` = `adios` |
| 4 | delegar a un subagente | `collaborationspawn_agent`, luego `collaborationwait_agent` | sí | permitido; el subagente respondió |
| 5 | crear `probe_denied.txt` con apply_patch | `apply_patch` | no | **bloqueado**. Codex: «`Command blocked by PreToolUse hook: probe deny.` El archivo no se creó.» Comprobado: el archivo no existe |

## Forma del payload (PreToolUse, rutas anonimizadas)

```json
{"session_id":"01a15ddb-…","turn_id":"…","transcript_path":"<scratch>","cwd":"<scratch>","hook_event_name":"PreToolUse","model":"…","permission_mode":"bypassPermissions","tool_name":"Bash","tool_input":{"command":"echo PROBE_DENY"},"tool_use_id":"exec-…"}
```

En `apply_patch`, `tool_input.command` contiene el parche completo (`*** Begin Patch` / `*** Update File: <ruta>` / `*** Add File: <ruta>` / `*** Delete File: <ruta>` …). En `collaborationspawn_agent`, `tool_input` lleva `task_name` y `message`.

## Conclusiones para el diseño (aplicadas en research R3/R4/R7 y tasks)

1. **Matcher de Codex en `PreToolUse`**: `Bash|apply_patch|collaborationspawn_agent`. Los tres nombres se observaron literalmente. `PostToolUse` usa `Bash`.
2. **La denegación funciona** con el sobre `hookSpecificOutput.permissionDecision: "deny"`, tanto en `Bash` como en `apply_patch`. El dialecto `json` de `pre-tool` debe emitir ese sobre (igual que claude), no `{"decision":"deny"}`.
3. **Detección de dialecto**: el payload de Codex trae `hook_event_name` y `tool_name`, así que `detectDialect` lo clasificaría como `claude`. Los hooks de Codex siguen fijando `--emit=json` y la salida de ambos dialectos en `pre-tool` es idéntica, de modo que no hay divergencia.
4. **Reglas `file` en Codex**: la ruta y el contenido se extraen de las cabeceras `*** (Add|Update|Delete) File:` del parche, y el cuerpo del parche se toma como contenido.
5. **`ClassifyTool`**: `apply_patch` → file; `collaborationspawn_agent` → delegate; `collaborationwait_agent` → other.
6. SC-001 se mantiene en **3 de 3**: Codex bloquea de verdad. Queda pendiente la verificación de extremo a extremo con `mem hook expand` real (T032).
