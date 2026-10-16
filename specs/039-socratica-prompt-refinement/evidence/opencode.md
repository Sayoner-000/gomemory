# Evidencia OpenCode (039)

## T066 — ¿OpenCode puede ser generador de solo lectura?

- Fecha: 2026-10-16 · macOS (Darwin 27.0.0)
- Binario: `opencode --version` → `opencode v2.0.27`

Orden ejecutada (solo lectura de la ayuda; no se abrió ninguna sesión ni se tocó la configuración):

```sh
opencode run --help
```

Salida literal relevante:

```text
USAGE
  opencode run [flags] [<message...>]

FLAGS
  --standalone            Run with a private server instead of the background service
  --server string         Connect to a server URL instead of the background service
  --continue, -c          Continue the last session
  --session, -s string    Session ID to continue, or to create if it does not exist
  --fork                  Fork the session before continuing
  --model, -m string      Model to use in the format provider/model#variant
  --agent string          Agent to use
  --format choice         Output format (choices: default, json)
  --file, -f string       File to attach to the message
  --title string          Session title
  --thinking              Show thinking blocks
  --auto                  Auto-approve permissions that are not explicitly denied
```

Resultado:

- `--agent <nombre>` existe. No hay ningún indicador propio de «sin herramientas» ni de solo lectura (a diferencia de `claude --tools ""` o `codex exec --sandbox read-only`).
- Demostrar que un agente definido por la persona **no puede modificar archivos** exige crear ese agente en su configuración de OpenCode y ejecutar una sesión real. No se hizo: cambia la configuración de la persona y consume su cuota, y la tarea solo admite el adaptador «si queda demostrado».

Veredicto: **no demostrado → OpenCode sigue «no soportado» como generador** (research R3). No se añade `adapters/secondary/generator/opencode.go` y `socratica_generators` sigue ignorando `opencode` en el composition root.

## Aceptación por agente (T063)

Pendiente: requiere una sesión real de OpenCode 2.0.27 con la persona (quickstart §6).
