# gomemory v1.6.0

**Memoria colectiva para agentes AI — persistente, portable, plug-and-play.**

`gomemory` es un CLI + TUI + MCP server + sistema de plugins en Go que guarda
decisiones, arquitectura, bugfixes y aprendizajes de tu proyecto en SQLite.
Cuando vuelves a trabajar (días o semanas después), **el agente AI recuerda
todo el contexto** sin que tengas que pedírselo.

> `go` + `memory` → **gomemory**
>
> Inspirado en [Engram](https://github.com/Gentleman-Programming/engram) de
> Gentleman-Programming — la idea original de memoria persistente para agentes
> AI nació de ese proyecto.

---

## Features

### 🧠 Memoria Persistente
- **Contexto entre sesiones** de OpenCode, Claude, Cursor, Windsurf, Cline, Codex
  o cualquier agente AI compatible con MCP
- **6 tipos de memoria**: `architecture`, `decision`, `bugfix`, `pattern`,
  `learning`, `discovery`
- **Búsqueda con ranking** por relevancia (título primero, contenido después)
- **Relaciones semánticas** entre memorias: `related`, `compatible`, `scoped`,
  `conflicts_with`, `supersedes`, `not_conflict`
- **Agrupación por sesiones** de trabajo con UUID y resumen markdown

### 🔌 Sistema de Plugins Multi-Agente
Los plugins inyectan memoria automáticamente en cada inferencia del agente,
sin invocación manual de herramientas MCP.

| Agente | Plugin | Instalación |
|--------|--------|-------------|
| **OpenCode** | `plugin/opencode/plugin.ts` | `mem setup opencode` |
| **Claude Code** | `plugin/claude-code/` (hooks + scripts + skill) | `mem setup claude-code` |

### 🌐 Servidor HTTP Background
- **Auto-iniciado** por los plugins — sin configuración manual
- **Sesiones persistentes** — sobreviven reinicios del agente
- **Contexto optimizado** — payload < 200 tokens por request
- **Endpoint**: `127.0.0.1:9735`

### 📋 CLI Completo
`mem` + 23 subcomandos para gestionar memoria desde terminal.

| Comando | Descripción |
|---------|-------------|
| `mem` | Abrir TUI interactiva (Bubbletea) |
| `mem init [--force]` | Inicializar `.memory/` en el proyecto |
| `mem save -t "título" -y tipo "cuerpo"` | Guardar memoria |
| `mem capture [flags]` | Guardar aprendizaje estructurado (What/Why/Where/Learned) |
| `mem compare [flags] <id1> <id2>` | Comparar memorias y persistir veredicto semántico |
| `mem compare list [-n N]` | Listar relaciones guardadas |
| `mem project` | Detectar proyecto actual (read-only) |
| `mem list [-n N]` | Listar memorias recientes |
| `mem log` | Alias de `list` |
| `mem search "consulta"` | Buscar en la memoria |
| `mem context [-w\|--write]` | Mostrar contexto o escribirlo a `.memory/context.md` |
| `mem session start` | Iniciar sesión de trabajo |
| `mem session end -s "resumen"` | Finalizar sesión |
| `mem install [dir]` | Instalar gomemory en otro proyecto |
| `mem uninstall [dir] [--yes]` | Desinstalar gomemory por completo (reverso de `install`) |
| `mem purge [flags]` | Purgar memorias (proyecto actual por defecto, `--all`/`--type`/`--older-than-days`) |
| `mem compact` | Compactar `.memory/mem.db` (recupera espacio, no borra nada) |
| `mem gc [flags]` | Garbage collection a demanda (90 días de retención por defecto) |
| `mem wrap <comando> [args...]` | Ejecutar comando y preguntar si guardar |
| `mem mcp` | Servidor MCP para agentes AI |
| `mem hook <evento>` | Entrypoint portable de hooks (`session-start`, `session-end`, `pre-compact`, `user-prompt-submit`) |
| `mem settings [--auto-approve\|--show]` | Ver/cambiar auto-approve de las tools MCP |
| `mem setup-mcp [--agents a,b,c]` | Configurar MCP multi-agente |
| `mem serve [--port N]` | Servidor HTTP de plugins (auto-inicia sesiones y contexto) |
| `mem setup opencode\|claude-code` | Instalar plugin de memoria para agente específico |
| `mem tui` | Abrir TUI explícitamente |
| `mem help` | Mostrar ayuda |

### 🎨 TUI Interactiva (Bubbletea)
- Navegación con `↑`/`↓` o `j`/`k`
- Búsqueda en vivo con `/`
- Guardado rápido con formulario guiado
- Vista de detalle con contenido completo
- Auto-approve de herramientas MCP

### 🔗 MCP Server (Model Context Protocol)
- **7 herramientas MCP**: `save_memory`, `search_memories`, `list_memories`,
  `get_memory`, `start_session`, `end_session`, `get_context`
- **2 recursos MCP**: `mem://context`, `mem://memory/{id}`
- `--root` flag para resolver proyecto sin depender del `cwd`
- Configuración multi-agente vía `mem setup-mcp`

### ⚙️ Capture Estructurado
- Campos: What / Why / Where / Learned
- Flags individuales o modo interactivo (`-i`)
- Ideal para decisiones técnicas complejas

### 🧪 Testeado
- Tests unitarios: servidor HTTP, instalador de plugins
- Tests de integración: session lifecycle, plugin structure
- Tests de contrato: Memory Protocol, progressive disclosure
- CI listo: `go build`, `go vet`, `go test ./...`

---

## Instalación universal (consola)

Un solo comando instala el binario `mem` en el PATH, sin compilar ni clonar.
Funciona en Linux, macOS y Windows.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh | bash
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
```

Para desinstalar el binario: `curl -fsSL .../install.sh | bash -s -- --uninstall`
(Linux/macOS) o `... -Uninstall` (Windows).

> ¿Por qué un binario en el PATH? Toda la config de agentes referencia `mem`
> por nombre — nunca una ruta absoluta de tu máquina. Así la config es portable
> entre equipos y SO, y los hooks corren igual en Claude, OpenCode, Cursor, etc.

---

## Inicio rápido

```bash
# Cablear memoria + agentes en tu proyecto (asume `mem` ya en el PATH)
cd /ruta/a/tu/proyecto
mem install .

# Guardar y buscar
mem save -t "API REST" -y decision "Usamos Fiber para rutas"
mem search "API"
mem context --write
```

`mem install .`:
- crea `.memory/` y actualiza `.gitignore`;
- configura el **MCP** de los 6 agentes (Claude, OpenCode, Cursor, Windsurf, Cline, Codex);
- genera/actualiza `AGENTS.md` y `CLAUDE.md` con el **pack de trabajo**: reglas de
  trabajo (lecciones de campo) + orquestación + el Memory Protocol;
- copia la **constitución** (`speckit-constitution-gen.md`) a la raíz del proyecto.

> Los **hooks** de los agentes no se instalan con `install` — usa
> `mem setup claude-code` o `mem setup opencode` para registrarlos.
>
> Desde el fuente: `go build -o mem ./infrastructure/` y luego `./mem install .`.

---

## Plugins

### OpenCode

```bash
./mem setup opencode
```

Instala `plugin/opencode/plugin.ts` en `~/.config/opencode/plugins/gomemory/`
y configura la activación automática. El plugin:
- Inicia `mem serve` en background
- Crea/cierra sesiones automáticamente
- Inyecta Memory Protocol + contexto en cada inferencia
- Recupera estado después de compactación

### Claude Code

```bash
mem setup claude-code
```

Configura hooks portables en `.claude/settings.json` e instala el skill en
`.claude/plugins/gomemory/`. Los hooks son **subcomandos del binario**
(`mem hook <evento>`), no scripts shell: no dependen de `bash`/`curl` ni de un
servidor HTTP, y corren igual en Windows. Para qué sirve cada uno:
- **`SessionStart`** → abre sesión si no hay activa **e inyecta el contexto de
  sesiones previas**: el agente arranca recordando el proyecto.
- **`SessionEnd`** → cierra la sesión activa como **red de seguridad** (aunque el
  modelo no llame `end_session`).
- **`PreCompact`** → antes de compactar, inyecta **instrucciones de recuperación +
  contexto** para que la compactación no borre el estado de trabajo.
- **`UserPromptSubmit`** → en el **primer** prompt activa las tools MCP de memoria
  e inyecta el recordatorio del protocolo; luego es pasivo (sin overhead).
- Skill de memoria (`skills/memory/SKILL.md`) siempre disponible para el agente.

> Regla de oro: un hook nunca aborta el arranque del agente — ante cualquier
> error sale silencioso con código 0.

> Los hooks se escriben referenciando `mem` por PATH (o `${CLAUDE_PROJECT_DIR}/mem`
> como fallback por-proyecto), nunca una ruta absoluta de máquina.

---

## Documentación

| Documento | Descripción |
|-----------|-------------|
| [`docs/architecture.md`](docs/architecture.md) | Arquitectura completa del proyecto |
| [`docs/PLUGINS.md`](docs/PLUGINS.md) | Sistema de plugins multi-agente |
| [`docs/MEMORY-PROTOCOL.md`](docs/MEMORY-PROTOCOL.md) | Protocolo de memoria (referencia técnica) |
| [`docs/MANUAL.md`](docs/MANUAL.md) | Guía paso a paso para usuarios |

---

## Stack

| Componente | Tecnología |
|------------|------------|
| Lenguaje | Go 1.25+ |
| Base de datos | SQLite embebido (`modernc.org/sqlite`, sin CGO) |
| TUI | `charmbracelet/bubbletea` + `bubbles` + `lipgloss` |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` |
| Timestamps | UTC-5 (Bogotá/Colombia, sin DST) |
| Dependencias runtime | 0 — binario autocontenido (~16MB) |
| Portabilidad | Linux, macOS, Windows (cross-compile nativo) |

---

## Portabilidad

```bash
# Cross-compile sin toolchain adicional
GOOS=darwin  GOARCH=arm64 go build -o mem-darwin-arm64 ./infrastructure/
GOOS=darwin  GOARCH=amd64 go build -o mem-darwin-amd64 ./infrastructure/
GOOS=linux   GOARCH=amd64 go build -o mem-linux-amd64 ./infrastructure/
GOOS=windows GOARCH=amd64 go build -o mem-windows-amd64.exe ./infrastructure/
```

- `.memory/mem.db` es SQLite WAL — cópialo entre máquinas sin migraciones
- Timestamps UTC-5 independientes de la zona horaria local
- Las configuraciones MCP usan rutas absolutas — regenera con `setup-mcp`
  después de mover el proyecto

---

## Pruebas

```bash
go build -o mem ./infrastructure/      # compilar
go vet ./...           # verificación estática
go test ./... -v       # tests unitarios + integración + contrato
```

---

## Licencia

MIT

---

## Autor

**Jose Gomez** ([@Sayoner-000](https://github.com/Sayoner-000)) — Arquitecto y desarrollador

---

## Inspiración

Este proyecto nació como una adaptación y evolución de
[Engram](https://github.com/Gentleman-Programming/engram) de
Gentleman-Programming. Engram demostró que la memoria persistente para agentes
AI es viable y valiosa — gomemory lleva esa idea a un stack autocontenido en
Go con SQLite embebido, multi-agente, y sin dependencias runtime.
