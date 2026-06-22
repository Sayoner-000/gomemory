# gomemory v1.3.0

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
`mem` + 18 subcomandos para gestionar memoria desde terminal.

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
| `mem wrap <comando> [args...]` | Ejecutar comando y preguntar si guardar |
| `mem mcp` | Servidor MCP para agentes AI |
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

## Inicio rápido

```bash
# Compilar
go build -o mem ./infrastructure/

# Instalar plugin para OpenCode (recomendado)
./mem setup opencode

# O para Claude Code
./mem setup claude-code

# Servidor HTTP (auto-iniciado por plugins, o manual)
./mem serve &

# Guardar y buscar
./mem save -t "API REST" -y decision "Usamos Fiber para rutas"
./mem search "API"
./mem context --write
```

---

## Instalación en proyecto

```bash
./mem install /ruta/a/tu/proyecto
```

Esto crea `.memory/`, configura MCP para todos los agentes, y genera
`AGENTS.md`/`CLAUDE.md` con integración.

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
./mem setup claude-code
```

Instala hooks, scripts y skill en `.claude/plugins/gomemory/`. El plugin:
- Crea sesión al iniciar (hook SessionStart)
- Cierra sesión al terminar (hook SessionEnd)
- Inyecta contexto post-compactación
- Skill de memoria (`skills/memory/SKILL.md`) siempre disponible para el agente

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

**Jose Gomez** — Arquitecto y desarrollador

---

## Inspiración

Este proyecto nació como una adaptación y evolución de
[Engram](https://github.com/Gentleman-Programming/engram) de
Gentleman-Programming. Engram demostró que la memoria persistente para agentes
AI es viable y valiosa — gomemory lleva esa idea a un stack autocontenido en
Go con SQLite embebido, multi-agente, y sin dependencias runtime.
