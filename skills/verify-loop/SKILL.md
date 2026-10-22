---
name: verify-loop
description: Aplica este flujo para verificar cambios antes de confirmar.
---

## Capacidades
Prefiere las herramientas MCP de gomemory cuando estén disponibles. Como alternativa usa la CLI `mem`; sin ambas, registra los mismos datos en un documento local y realiza las comprobaciones manualmente.

## Procedimiento
1. Detecta el stack por go.mod, package.json/tsconfig y archivos *.ps1. Usa las órdenes de compilación, lint y pruebas declaradas por el proyecto.
2. Consulta `search_memories` sobre los archivos tocados y filtra memorias de tipo bugfix. Alternativa: `mem search`; sin memoria revisa el historial y las pruebas de regresión.
3. En Go ejecuta `go build ./...`, `go vet ./...` y `go test -race ./...`. En TypeScript y PowerShell ejecuta los controles existentes en el workflow. No instales dependencias sin necesidad.
4. Revisa `git diff --stat` y el diff completo. La CLI es la vía principal para rules: `git diff | mem expand rules --check`. Si mem no existe, inspecciona manualmente cambios destructivos, secretos y reglas del proyecto.
5. Si el riesgo es alto, inicia `review_start`; alternativa `mem review --diff`, o revisión independiente manual.
6. Guarda PASS/FAIL y la evidencia con `save_memory` de tipo decision, o bugfix si corregiste un defecto; alternativa `mem save`. Criterio de salida: veredicto guardado con fallos pendientes explícitos.

## Por harness
| Acción | Claude Code | Codex | OpenCode |
|---|---|---|---|
| Controles | Shell del agente | Shell del agente | Shell del agente |

Inspirada en verification-loop de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.
