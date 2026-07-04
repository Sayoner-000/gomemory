# Lessons Learned

<!-- Capture patterns, gotchas, and rules after each user correction. -->

## 2026-07-04

- **La entrada del 2026-06-21 sobre el plugin de OpenCode quedó obsoleta y es lo contrario de lo correcto.** Aquella decía que OpenCode "requiere un subdirectorio por plugin" (`plugins/gomemory/plugin.ts`). Es al revés: OpenCode auto-descubre plugins como **archivos sueltos** en `~/.config/opencode/plugins/` (`gomemory.ts`, sin subcarpeta) — el código actual (`opencode_setup.go`) instala así y activamente borra la instalación anidada legada (`os.RemoveAll(pluginsDir/gomemory)`) porque OpenCode nunca la cargaba. La documentación (INSTALLATION.md, docs/MANUAL.md, docs/architecture.md) seguía describiendo la ruta anidada meses después del fix real en el código — nadie la sincronizó cuando se revirtió el enfoque. **Regla: cuando una lección quede invalidada por un cambio posterior, no basta con corregir el código — hay que grep por la ruta/comando vieja en TODOS los `.md` del repo (no solo el README), porque cada doc suele repetir el mismo dato de forma independiente.**
- Relacionado: `mem setup opencode` (scope proyecto) escribe el MCP en el `opencode.json` del proyecto actual, no en `~/.config/opencode/opencode.json` — y OpenCode nunca "referencia" el plugin desde ningún JSON, lo auto-descubre por convención de carpeta. Confirmado empíricamente con `opencode debug config` (ver [[bugfix opencode global scope]] / release v1.11.0): ese mismo comando reveló que sí existe un `~/.config/opencode/opencode.json` de scope usuario que se mergea con el del proyecto, lo que permitió agregar `opencode` a `globalScopeAgents`.

## 2026-06-22

- `session-start.sh` resolvía el binario como `BIN="${GOMMY_BIN:-mem}"`. `GOMMY_BIN` nunca se exporta en ningún hook, así que siempre caía al literal `"mem"`, ausente del `PATH` → `command not found`. El mecanismo para hornear la ruta absoluta (`{{BIN_PATH}}` en `replacePlaceholders`) ya existía pero el template nunca lo usaba. Mismo bug en `plugin/opencode/plugin.ts` (`mem serve` literal). **Regla: cuando un template de plugin necesite invocar el propio binario, usar siempre `{{BIN_PATH}}`, nunca un nombre bare que dependa del `PATH`.**
- Los hooks de Claude Code se registraban como `PostStartup`/`PreShutdown` en `claude_code_setup.go` y `hooks/hooks.json` — **esos eventos no existen** en Claude Code (los reales son `SessionStart`/`SessionEnd`). Una instalación nueva nunca dispararía esos hooks (fallarían en silencio, sin error visible). **Regla: verificar contra la documentación real de eventos de hooks del agente, no asumir nombres "parecidos".**
- `//go:embed plugin` (sin el prefijo `all:`) excluye silenciosamente archivos/directorios que empiezan con `.` — por eso `.claude-plugin/plugin.json` y el `.mcp.json` del template nunca se copiaban a proyectos instalados, a pesar de estar documentados. **Regla: cualquier directorio embebido que incluya archivos `.algo` necesita `//go:embed all:dir`, no `//go:embed dir`.**
- `.mcp.json` es local y gitignorado — si el proyecto se clona/copia a otra máquina o usuario, queda con una ruta de binario stale (vimos un path de macOS de otro usuario en un checkout Linux). Reinstalar (`./mem setup claude-code`) lo regenera porque `InstallClaudeCode` sobreescribe la entrada `gomemory` sin condición.
- El flag `--target` de `mem setup` se pierde si se pasa el agente posicional ANTES de los flags (`mem setup claude-code --target X`) — el paquete `flag` de Go deja de parsear flags en el primer argumento no-flag. Hay que pasar `mem setup --target X --port N claude-code` (flags antes del posicional) para que efectivamente se respete.

## 2026-06-21

- `mem setup opencode` instalaba `plugin.ts` como archivo plano en `~/.config/opencode/plugins/` en vez de dentro de `~/.config/opencode/plugins/gomemory/plugin.ts` — OpenCode requiere un subdirectorio por plugin (`plugins/<name>/plugin.ts`). Fijado cambiando el target dir en `opencode_setup.go`. **⚠️ OBSOLETA, ver 2026-07-04**: esta conclusión resultó ser incorrecta y fue revertida — OpenCode sí auto-descubre archivos sueltos, el código actual vuelve a instalar `gomemory.ts` plano.
- El embed `//go:embed` no soporta `..` en la ruta del patrón. Solución: declarar el embed en `main.go` (raíz del módulo) y asignarlo a `setup.PluginFS` via `init()`.
- Los tests de Go deben estar en el mismo package que el código que prueban. `tests/unit/` con `package main` no compila — mover tests a `adapters/primary/mcp/server_test.go` e `adapters/primary/setup/setup_test.go`.
- `replacePlaceholders` en `setup.go` hacía `bytes.ReplaceAll` asignando a `string` en vez de `[]byte` — error de tipos al cambiar entre string y []byte sin conversión explícita.

## 2026-06-17

- AGENTS.md/CLAUDE.md must describe the **actual project architecture**, not a different project — wrong architecture confuses the AI into using wrong tools
- `mem install` message was misleading: said "No se encontró AGENTS.md" when the files existed but already had the integration block → fix: track `found` vs `updated` separately
- Timestamps should be stored in UTC-5 (Bogotá) using `datetime('now', '-5 hours')` in SQLite
- `mem context --write` printed wrong path (`mem.db` instead of `context.md`)
- `parseSubflags` was dead code — remove unused functions
- `mem log` alias existed in code but wasn't documented in help/README
- `mem setup-mcp` and `cmd_mcp_setup.go` existed but weren't documented in any doc file (README, architecture.md, AGENTS.md) — always keep docs in sync with all source files
- AGENTS.md and CLAUDE.md must have identical architecture sections — they diverged (CLAUDE.md was missing the full architecture block)
- The `--agents` flag for `setup-mcp` supports `all` to configure 5 agents at once — document this explicitly

## 2026-06-18

- `mem settings` was implemented (`cmd_settings.go`, registered in `main.go`'s dispatcher) but never added to `usage()`, README.md, or architecture.md — same recurring pattern as `setup-mcp` before it. **Rule: whenever a new `case` is added to the dispatcher switch in `main.go`, add it to `usage()` and the docs command tables in the same commit, not later.**
- MCP server configs generated by `setup-mcp`/`install` only ever wrote `args: ["mcp"]`, with no project root info. `mem mcp` resolves its project via `os.Getwd()` (`store.FindRoot()`), but the MCP client controls the subprocess's actual `cwd` when it spawns the server — never guaranteed to be the installed project. Root cause of "memories not saving after install even though the server shows as connected". **Fix: never rely on the spawning client's cwd for a server that must resolve a specific project — pass the project root explicitly as a CLI arg (`--root`) baked into the generated config.**
- A previous "global Claude config" write (`~/.claude/mcp/gomemory.json`) was dead code — Claude Code does not read per-server files from that path; its real global/user scope lives in `~/.claude.json`, and project scope (which actually works) is `.mcp.json`. **Rule: before writing integration code for an external tool's config format, verify the real schema/location (web search or official docs) instead of assuming a plausible-looking path.**
- Codex CLI's MCP config (`~/.codex/config.toml`) is a single global file shared across all projects with TOML tables `[mcp_servers.<name>]` — unlike the per-project JSON files used by other agents. **When adding a new agent integration, check whether its config is project-scoped or globally shared; globally shared configs need a unique key per project (e.g. `gomemory_<project>`) and must only be appended to, never fully rewritten, to avoid corrupting unrelated entries.**
