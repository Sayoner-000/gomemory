# Fixtures de `tool_response` (feature 033, US3)

| Archivo | Origen |
|---|---|
| `claude-bash.json`, `claude-grep.json` | **Capturados** con Claude Code 2.1.282 (`claude -p` + hook PostToolUse que vuelca stdin), 2026-09-25. Forma real; `stdout`/`content` sustituidos por muestras del corpus e identificadores anonimizados. |
| `claude-read.json` | Forma de Read según el binario 2.1.282 (`type` + `file.content`); solo se usa para comprobar que Read se excluye. |
| `claude-mcp.json` | **Derivado** del contrato MCP (array de bloques de contenido): no capturado. |
| `codex-mcp.json`, `codex-shell.json` | **Derivados**: Codex rechaza siempre `updatedMCPToolOutput` (0.159.0, `output_parser.rs`), así que el hook debe responder con salida vacía; la forma de `tool_response` sigue el `CallToolResult` de MCP. No capturados. |
| `opencode-bash.json` | Forma de `tool.execute.after` según `@opencode-ai/plugin` 1.18.20 (`output.output: string`). |

## Feature 035 (compresión que no destruye)

Envoltura PostToolUse de Claude Code igual a la de `claude-bash.json`; el contenido es **real**, generado el 2026-10-04 directamente con la herramienta (sin pasar por el agente), y las rutas de la carpeta personal se anonimizaron como `/home/user`.

| Archivo | Origen |
|---|---|
| `claude-bash-sed-go.json` | `sed -n 1,80p adapters/secondary/persistence/compression_stats.go`. Es el caso que el hook destruyó en vivo (indentación borrada y sentencias omitidas). Debe quedar excluido. |
| `claude-bash-awk-go.json` | `awk 'NR<=200' adapters/primary/cli/cmd_hook.go`: código Go sin cabecera `package` y con un comando no excluido; las líneas conservadas deben salir idénticas. |
| `claude-bash-doctor.json` | Salida real de `mem doctor`. Debe quedar excluido. |
| `claude-mcp-search30.json` | `search_code` de codebase-memory-mcp con 30 resultados (forma compact; nombres y líneas reales del repositorio). Debe llegar completo. |
| `claude-grep-listing.json` | Grep en modo content con 400 líneas `ruta:línea:` reales (`grep -rn "func " --include='*.go' .`). Es un listado. |
| `listing-find.txt` | `find . -name '*.go'` del repositorio (599 líneas). Es un listado de rutas. |

Si un runtime cambia su forma, se vuelve a capturar y se anota aquí la versión.
