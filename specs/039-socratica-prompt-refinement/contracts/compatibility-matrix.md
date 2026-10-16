# Matriz de compatibilidad (plantilla a completar con evidencia)

Cada celda se rellena solo con una prueba ejecutada: la orden y la salida quedan en `specs/039-socratica-prompt-refinement/evidence/`. Hasta entonces, el valor es «pendiente» y el código trata la capacidad como **no soportada**.

| Capacidad | Claude Code 2.1.300 | Codex 0.162.1 | OpenCode 2.0.27 | Alternativa en texto |
|---|---|---|---|---|
| Habilidad descubierta | pendiente | pendiente | pendiente | — |
| `/socratica` con nombre | pendiente (habilidad nativa) | pendiente (`$socratica`) | pendiente (comando) | lenguaje natural |
| `/socratic-q` con nombre | pendiente (comando) | no previsto | pendiente (comando) | «socratica en modo socratic-q: …» |
| Intercepción pre-envío | pendiente (`UserPromptSubmit` + additionalContext) | pendiente (`UserPromptSubmit` JSON) | pendiente (`chat.message`) | invocación explícita |
| Eliminar `>` inicial | **por directiva** (el hook no puede mutar el texto) | **por directiva** | **garantizado** (mutación de la parte) | — |
| Panel nativo | no (R10) | no | no | revisión en texto |
| Portapapeles | no | no | no | bloque delimitado |
| Devolver el original al campo de entrada | no | no | pendiente | original repetido literalmente |
| Historial en interfaz | TUI `mem` | TUI `mem` | TUI `mem` | `mem socratica history` |
| Generador no interactivo (CLI/TUI) | pendiente (`claude -p --json-schema --tools ""`) | pendiente (`codex exec --sandbox read-only --output-schema`) | **no soportado**: `run` no tiene modo sin herramientas; `--agent` existe pero no se demostró un agente de solo lectura (T066, `evidence/opencode.md`) | error código 2 |
| Marca de origen al lanzar desde CLI/TUI | pendiente (`GOMEMORY_SOCRATICA_DISPATCH`) | pendiente | pendiente | — |
