# Fixtures de `tool_response` (feature 033, US3)

| Archivo | Origen |
|---|---|
| `claude-bash.json`, `claude-grep.json` | **Capturados** con Claude Code 2.1.282 (`claude -p` + hook PostToolUse que vuelca stdin), 2026-09-25. Forma real; `stdout`/`content` sustituidos por muestras del corpus e identificadores anonimizados. |
| `claude-read.json` | Forma de Read según el binario 2.1.282 (`type` + `file.content`); solo se usa para comprobar que Read se excluye. |
| `claude-mcp.json` | **Derivado** del contrato MCP (array de bloques de contenido): no capturado. |
| `codex-mcp.json`, `codex-shell.json` | **Derivados**: Codex 0.157.0 solo admite `updatedMCPToolOutput` (verificado en el binario); la forma de `tool_response` sigue el `CallToolResult` de MCP. No capturados. |
| `opencode-bash.json` | Forma de `tool.execute.after` según `@opencode-ai/plugin` 1.18.20 (`output.output: string`). |

Si un runtime cambia su forma, se vuelve a capturar y se anota aquí la versión.
