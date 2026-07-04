# Contratos CLI: comandos nuevos y modificados

gomemory es un CLI/TUI/MCP server — el "contrato" externo relevante es la interfaz de comandos (flags, exit codes, salida esperada) y las herramientas MCP expuestas. Ninguna herramienta MCP nueva se añade en esta feature (ver Constitution Check: registro/migración quedan fuera de MCP).

## `mem migrate`

```text
mem migrate [--force]
```

- Sin flags: si existe `<root>/.memory/mem.db` y no existe el archivo correspondiente en el store global, lo mueve (incluye `-wal`/`-shm`) y normaliza la columna `project` de las filas migradas a la nueva clave. Imprime conteo de memorias migradas. Exit code `0`.
- Si ya existe el archivo en el store global y no hay legado que migrar: no-op, mensaje informativo, exit code `0`.
- Si existen **ambos** (legado y global) con datos: no sobrescribe nada, imprime advertencia con las rutas de ambos archivos, exit code distinto de `0` (falla explícita) salvo que se pase `--force`.
- `--force`: fuerza la migración incluso si el destino global ya existe, sobrescribiéndolo — requiere el mismo patrón de confirmación explícita que ya usan `mem purge`/`mem gc` (no se expone vía MCP).

## `mem init` (comportamiento nuevo)

```text
mem init [--force]
```

- Deja de ser obligatorio antes de usar gomemory en un proyecto. Se convierte en no-op informativo: imprime la clave de proyecto resuelta y la ruta del `mem.db` en el store global para el cwd actual (creándolo si no existe, igual que hará el init perezoso en cualquier otro comando).
- Si detecta `.memory/mem.db` legado en el cwd, dispara automáticamente el mismo flujo que `mem migrate` (sin necesidad de flag adicional) y lo reporta.
- `--force`: sin cambio de intención respecto a hoy (reinicializar), pero ya no falla si no había nada previo — ahora es siempre válido.

## `mem mcp` (comportamiento modificado)

```text
mem mcp
```

- Ya no exige que `.memory/` exista en ningún directorio padre del cwd. Resuelve el proyecto vía `FindProjectRoot()` (git root o cwd) y abre/crea el `mem.db` correspondiente en el store global (init perezoso).
- El flag `--root` se mantiene por compatibilidad para forzar una ruta explícita, pero deja de exigir `.memory/` preexistente en esa ruta — aplica la misma resolución de clave y store global.
- Nunca termina con `log.Fatalf` por ausencia de instalación previa (ese código de error se elimina).

## `mem setup-mcp` (flag nuevo)

```text
mem setup-mcp --agents <lista> [--scope project|global] [--target <dir>]
```

- `--scope global` (nuevo): registra el/los agente(s) indicados en su mecanismo de configuración de usuario (`~/.claude.json` para Claude Code, `~/.codex/config.toml` para Codex, equivalente para OpenCode si aplica). No requiere `--target`.
- `--scope project` (comportamiento actual, sigue siendo el default para Cursor/Windsurf/Cline): registra en el repo indicado por `--target` (o cwd).
- Para agentes sin soporte de scope de usuario conocido, `--scope global` retorna error explícito indicando que ese agente solo soporta `--scope project`.
- Antes de escribir en `--scope global`, si detecta una entrada existente con la clave `gomemory` que no apunta al binario `mem` actual, se detiene con un mensaje pidiendo confirmación/limpieza manual (no sobrescribe silenciosamente) — cubre el caso de colisión de nombre ya detectado en este entorno.

## `mem install <dir>` (re-scope, comportamiento modificado)

```text
mem install <dir>
```

- Dejan de ejecutarse: copia de binario al repo, creación de `.memory/` en el repo, escritura de `.mcp.json`/`.cursor/mcp.json`/`.windsurf/mcp_config.json`/`.cline/mcp_settings.json` locales, inyección de bloques de protocolo en `AGENTS.md`/`CLAUDE.md`, copia de `speckit-constitution-gen.md`.
- Pasa a ejecutar: `mem setup-mcp --scope global --agents claude,codex` (u otros agentes que ya soporten scope global) + `mem migrate` sobre `<dir>` si corresponde.
- Imprime aviso de deprecación explicando el cambio de comportamiento y remitiendo a `mem setup-mcp`/`mem migrate` como comandos directos preferidos.

## Herramientas MCP expuestas (sin cambios)

`save_memory`, `search_memories`, `list_memories`, `get_memory`, `forget_memory`, `judge_memories`, `start_session`, `end_session`, `get_context` — sin cambios en su firma ni comportamiento observable; solo cambia, de forma transparente para quien las invoca, dónde se resuelve el proyecto y dónde vive el archivo subyacente.
