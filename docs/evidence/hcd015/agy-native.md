# HCD-015: contrato observado en agy

Fecha: 2026-11-02. `agy` 1.4.12, `-p`, modelo `gemini-3.8-flash-medium`, autenticación nativa. Proyecto temporal `/private/tmp/hcd015-agy/project` con `.agents/hooks.json` y un hook que registra stdin y responde datos sintéticos. Sin credenciales ni historial capturados. Los hooks globales de la persona también se ejecutaron, con `GOMEMORY_DATA_HOME` temporal.

## Eventos observados

Todos reciben `conversationId`, `workspacePaths`, `transcriptPath`, `artifactDirectoryPath`, `modelName` y `lastUserInput`. Este último no aparece en la documentación de hooks.

| Evento | Campos propios | Salida comprobada |
|---|---|---|
| `SessionStart` | ninguno | `injectSteps` |
| `PreInvocation` | `invocationNum`, `initialNumSteps` | `injectSteps` con `ephemeralMessage` |
| `PostInvocation` | `invocationNum`, `initialNumSteps` | `{}` |
| `PreToolUse` | `toolCall.name`, `toolCall.args`, `stepIdx` | `decision`, `reason` |
| `PostToolUse` | `toolCall`, `stepIdx`, `error` | `{}` |
| `Stop` | `executionNum`, `terminationReason`, `error`, `fullyIdle` | `{}` |

- La terminal se llama `run_command` y su comando va en `toolCall.args.CommandLine`.
- `PostToolUse` no recibe la salida de la herramienta, como dice la documentación.
- `{"decision":"allow"}` no bastó en `-p`: el comando quedó en `denied_actions` por permisos. `auto_approve` sí lo ejecutó. `deny` llegó al modelo como `tool call denied by pre-tool hook: <reason>`, y un comando denegado no dispara `PostToolUse`.
- `--mode auto` no está disponible en esta cuenta.

## Discrepancia con la documentación

`{"systemMessage":"texto"}` en `SessionStart` **se ignora sin aviso**: el transcript no contenía el marcador. `{"systemMessage":{"systemMessage":"texto"}}` y `{"ephemeralMessage":"texto"}` sí llegaron, como pasos `SYSTEM_MESSAGE` y `EPHEMERAL_MESSAGE`. El comprobante es `transcript_full.jsonl`, un registro local del host, no una petición al proveedor.

## Límites

Modalidad `-p` en una sola cuenta y versión. No se observó sesión interactiva, subagentes, `/compact` ni herramientas de edición. `~/.gemini/antigravity-cli/settings.json` cambió de huella durante la prueba y no consta qué modificó; no había copia previa para comparar.
