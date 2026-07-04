# gomemory

[![GitHub Release](https://img.shields.io/github/v/release/Sayoner-000/gomemory?style=flat&color=blue)](https://github.com/Sayoner-000/gomemory/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Platform](https://img.shields.io/badge/macOS_%7C_Linux_%7C_Windows-supported-lightgrey)](https://github.com/Sayoner-000/gomemory/releases/latest)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-9_tools-blueviolet)](https://modelcontextprotocol.io/)
[![SQLite](https://img.shields.io/badge/SQLite-embebido-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org/)

**Memoria colectiva para agentes de código — persistente, portable, plug-and-play.**

`gomemory` es un CLI + TUI + MCP server + sistema de plugins en Go que guarda
decisiones, arquitectura, bugfixes y aprendizajes de tu proyecto en SQLite.
Cuando vuelves a trabajar (días o semanas después), **el agente recuerda
todo el contexto** sin que tengas que pedírselo.

> `go` + `memory` → **gomemory**
>
> Inspirado en [Engram](https://github.com/Gentleman-Programming/engram) de
> Gentleman-Programming — la idea original de memoria persistente para agentes
> de código nació de ese proyecto.

---

## Por qué gomemory

- **Contexto entre sesiones** — OpenCode, Claude Code, Cursor, Windsurf, Cline, Codex o cualquier agente compatible con MCP recuerda todo el historial del proyecto.
- **Sin dependencias runtime** — binario autocontenido (~16MB). SQLite embebido via `modernc.org/sqlite` (sin CGO). Descarga, ejecuta, listo.
- **Sin instalación por proyecto (Claude Code, Codex, OpenCode)** — `mem setup-mcp --scope global` registra gomemory una sola vez por máquina; cada proyecto nuevo guarda y consulta memoria desde el primer uso, sin tocar el repo (ni `.mcp.json`/`opencode.json`, ni binario copiado, ni `AGENTS.md`).
- **Multi-agente** — `mem setup-mcp --scope project` registra el MCP por proyecto para Cursor/Windsurf/Cline (sin registro global conocido); `mem install` sigue disponible como pack completo opcional (MCP + hooks + `AGENTS.md`/`CLAUDE.md` + constitución) para cualquier agente.
- **8 tipos de memoria** — `architecture`, `decision`, `bugfix`, `pattern`, `learning`, `discovery`, `preference`, `checkpoint` (auto por turno).
- **Búsqueda con ranking** — relevancia por título primero, contenido después. Búsqueda semántica y por patrón de nombre.
- **CLI completo** — 24 subcomandos para gestionar memoria desde terminal, más una TUI interactiva con Bubbletea.
- **Servidor HTTP background** — auto-iniciado por los plugins, sesiones persistentes, contexto optimizado (< 200 tokens por request).
- **Checkpoints automáticos** — cada turno con actividad real se registra como memoria `checkpoint` sin gastar tokens del agente.
- **Juez imparcial** — `judge_memories` resuelve conflictos entre memorias con veredicto semántico y reasoning obligatorio.
- **Privacidad** — contenido envuelto en `<private>...</private>` se redacta antes de guardar, sin excepciones.

---

## Inicio rápido

```bash
# Linux / macOS — instalar el binario en el PATH
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh | bash
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
```

**Opción recomendada — Claude Code / Codex / OpenCode, sin instalar nada por proyecto:**

```bash
# Una sola vez, en cualquier directorio
mem setup-mcp --scope global --agents claude,codex,opencode

# Ya funciona en cualquier proyecto nuevo, sin mem install:
cd /ruta/a/cualquier/proyecto
mem save -t "API REST" -y decision "Usamos Fiber para rutas"
mem search "API"
```

El store de memoria (`mem.db`) se crea solo, por proyecto, en un directorio
global del usuario (`~/.local/share/gomemory/`), identificado por la raíz de
git — cero archivos nuevos en el repo. Si ya tenías proyectos instalados a la
manera antigua, su historial se migra solo al primer uso (o con `mem migrate`).

**Cursor / Windsurf / Cline — registro MCP por proyecto** (todavía sin registro global conocido):

```bash
cd /ruta/a/tu/proyecto
mem setup-mcp --scope project --agents cursor,windsurf,cline --target .
```

Solo escribe la config MCP de cada agente (`.cursor/mcp.json`, `.windsurf/mcp.json`,
`.cline/mcp.json`) — el store de memoria se crea solo al primer uso, igual que en
el flujo global.

**`mem install .` — pack completo (opcional, para cualquier agente):**

```bash
cd /ruta/a/tu/proyecto
mem install .

# Guardar y buscar
mem save -t "API REST" -y decision "Usamos Fiber para rutas"
mem search "API"
mem context --write
```

Además del MCP de los 6 agentes (Claude, OpenCode, Cursor, Windsurf, Cline,
Codex), `mem install .`:
- crea `.memory/` y actualiza `.gitignore`;
- genera/actualiza `AGENTS.md` y `CLAUDE.md` con el **pack de trabajo**: reglas de
  trabajo (lecciones de campo) + orquestación + el Memory Protocol;
- copia la **constitución** (`speckit-constitution-gen.md`) a la raíz del proyecto.

Úsalo si quieres ese pack de instrucciones en el repo, no solo la conexión MCP.

> Los **hooks** de los agentes no se instalan con `install` — usa
> `mem setup claude-code` o `mem setup opencode` para registrarlos.
>
> Desde el fuente: `go build -o mem ./infrastructure/` y luego `./mem install .`.

---

## Características

### Memoria Persistente

- **8 tipos de memoria**: `architecture`, `decision`, `bugfix`, `pattern`, `learning`, `discovery`, `preference`, `checkpoint` (auto, ver checkpoints por turno)
- **Búsqueda con ranking** por relevancia (título primero, contenido después)
- **Relaciones semánticas** entre memorias: `related`, `compatible`, `scoped`, `conflicts_with`, `supersedes`, `not_conflict`
- **Agrupación por sesiones** de trabajo con UUID y resumen markdown
- **Capture estructurado** con campos What / Why / Where / Learned (flags individuales o modo interactivo)

### Sistema de Plugins Multi-Agente

Los plugins inyectan memoria automáticamente en cada inferencia del agente,
sin invocación manual de herramientas MCP.

| Agente | Plugin | Instalación |
|--------|--------|-------------|
| **OpenCode** | `plugin/opencode/plugin.ts` | `mem setup opencode` |
| **Claude Code** | `plugin/claude-code/` (hooks + scripts + skill) | `mem setup claude-code` |

### Servidor HTTP Background

- **Auto-iniciado** por los plugins — sin configuración manual
- **Sesiones persistentes** — sobreviven reinicios del agente
- **Contexto optimizado** — payload < 200 tokens por request
- **Endpoint**: `127.0.0.1:9735`

### CLI Completo

`mem` + 24 subcomandos para gestionar memoria desde terminal.

| Comando | Descripción |
|---------|-------------|
| `mem` | Abrir TUI interactiva (Bubbletea) |
| `mem init [--force]` | Inicializar `.memory/` en el proyecto |
| `mem save -t "título" -y tipo "cuerpo"` | Guardar memoria |
| `mem capture [flags]` | Guardar aprendizaje estructurado (What/Why/Where/Learned) |
| `mem compare [flags] <id1> <id2>` (alias `mem judge`) | Comparar memorias y persistir veredicto semántico |
| `mem compare list [-n N]` | Listar relaciones guardadas |
| `mem forget <id>` | Borrar una memoria puntual por ID (irreversible) |
| `mem project` | Detectar proyecto actual (read-only) |
| `mem list [-n N]` | Listar memorias recientes |
| `mem log` | Alias de `list` |
| `mem search "consulta"` | Buscar en la memoria |
| `mem context [-w\|--write]` | Mostrar contexto o escribirlo a `.memory/context.md` |
| `mem session start` | Iniciar sesión de trabajo |
| `mem session end -s "resumen"` | Finalizar sesión |
| `mem install [dir]` | Instalar gomemory en otro proyecto |
| `mem uninstall [dir] [--yes]` | Desinstalar gomemory por completo (reverso de `install`) |
| `mem update [--check] [--version vX.Y.Z]` | Actualizar el binario y refrescar hooks/MCP/permisos del proyecto |
| `mem version` | Mostrar la versión instalada |
| `mem purge [flags]` | Purgar memorias (proyecto actual por defecto, `--all`/`--type`/`--older-than-days`) |
| `mem compact` | Compactar `.memory/mem.db` (recupera espacio, no borra nada) |
| `mem gc [flags]` | Garbage collection a demanda (90 días de retención por defecto) |
| `mem wrap <comando> [args...]` | Ejecutar comando y preguntar si guardar |
| `mem mcp` | Servidor MCP para agentes de código |
| `mem hook <evento>` | Entrypoint portable de hooks (`session-start`, `session-end`, `pre-compact`, `user-prompt-submit`, `turn-end`) |
| `mem settings [--auto-approve\|--show]` | Ver/cambiar auto-approve de las tools MCP |
| `mem setup-mcp [--agents a,b,c]` | Configurar MCP multi-agente |
| `mem serve [--port N]` | Servidor HTTP de plugins (auto-inicia sesiones y contexto) |
| `mem setup opencode\|claude-code` | Instalar plugin de memoria para agente específico |
| `mem tui` | Abrir TUI explícitamente |
| `mem help` | Mostrar ayuda |

### TUI Interactiva (Bubbletea)

- Navegación con `↑`/`↓` o `j`/`k`
- Búsqueda en vivo con `/`
- Guardado rápido con formulario guiado
- Vista de detalle con contenido completo
- Auto-approve de herramientas MCP

### MCP Server (Model Context Protocol)

- **9 herramientas MCP**: `save_memory`, `search_memories`, `list_memories`, `get_memory`, `forget_memory`, `judge_memories`, `start_session`, `end_session`, `get_context`
- **2 recursos MCP**: `mem://context`, `mem://memory/{id}`
- `--root` flag para resolver proyecto sin depender del `cwd`
- Configuración multi-agente vía `mem setup-mcp`

### Checkpoints Automáticos por Turno

En Claude Code (hook `Stop`) y OpenCode (evento `session.idle`), cada turno
con actividad real (archivos editados, comandos corridos) se registra solo
como memoria `checkpoint`, sin gastar tokens del agente ni depender de que
decida llamar `save_memory`. Turnos de puro chat no generan ruido.

El agente sigue usando `save_memory` para lo que requiere síntesis
(decisiones, causas raíz, patrones); ver sección `## Actividad Reciente
(auto)` del contexto para lo capturado automáticamente.

### Juez Imparcial de Memorias en Conflicto

- `judge_memories` (MCP) / `mem judge` (CLI, alias de `mem compare`) deja que
  el agente registre un veredicto — `related|compatible|scoped|conflicts_with|supersedes|not_conflict` — con `reasoning` obligatorio explicando qué hechos
  verificó contra el código actual.
- El contexto (`get_context`/`mem context`) muestra proactivamente una sección
  `## Conflictos sin resolver` con los pares de memorias en `conflicts_with`
  para que el agente no las ignore.

### Privacidad

Cualquier contenido envuelto en `<private>...</private>` se redacta antes de
guardar — nunca llega a la base de datos, sin importar por qué comando o
tool se guardó (`save_memory`, `mem save`, `mem capture`, `mem wrap`, TUI).

### Testeado

- Tests unitarios: servidor HTTP, instalador de plugins
- Tests de integración: session lifecycle, plugin structure
- Tests de contrato: Memory Protocol, progressive disclosure
- CI listo: `go build`, `go vet`, `go test ./...`

---

## Instalación

### Binarios Pre-compilados

Un solo comando instala el binario `mem` en el PATH, sin compilar ni clonar.

| Plataforma | Comando |
|------------|---------|
| **Linux / macOS** | `curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh \| bash` |
| **Windows** | `irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 \| iex` |

Opciones del instalador: `--uninstall` (Linux/macOS), `-Uninstall` (Windows).

> **Por qué un binario en el PATH?** Toda la config de agentes referencia `mem`
> por nombre — nunca una ruta absoluta de tu máquina. Así la config es portable
> entre equipos y SO, y los hooks corren igual en Claude, OpenCode, Cursor, etc.

### Actualización

```bash
mem update --check              # muestra versión actual vs. disponible, sin instalar
mem update                      # descarga la última versión, reemplaza el binario
mem update --version v1.8.0     # instala una versión específica
```

`mem update` reemplaza el binario de forma atómica y, si se ejecuta dentro de
un proyecto con `.memory/`, además vuelve a correr `mem install` con el
binario nuevo para refrescar hooks, config MCP y permisos pre-aprobados de
forma idempotente — así una instalación vieja con bugs ya corregidos (como los
permisos MCP faltantes o el recordatorio de protocolo que no se re-inyectaba)
queda al día sin pasos manuales.

En Windows, el binario en ejecución no se puede sobrescribir: `mem update`
deja el binario nuevo listo junto al actual y muestra el comando exacto para
completar el reemplazo manualmente una vez cerrado el proceso.

### Desinstalación

```bash
mem uninstall [dir] [--yes]
```

Reverso completo de `mem install`: quita el binario, los hooks, la config
MCP en todos los agentes soportados, los permisos `mcp__gomemory__*`
pre-aprobados, los bloques de protocolo en `AGENTS.md`/`CLAUDE.md` (preserva
el resto del contenido del usuario) y `.memory/` con toda la memoria
guardada. `~/.codex/config.toml` no se toca automáticamente por ser un
archivo global compartido entre proyectos — se avisa para removerlo a mano.

### Build desde el Fuente

<details>
<summary>Prerrequisitos</summary>

| Requisito | Verificar | Instalar |
|-----------|-----------|----------|
| **Go 1.25+** | `go version` | https://go.dev/dl/ |
| **Git** | `git --version` | Pre-instalado en la mayoría de sistemas |

</details>

```bash
git clone https://github.com/Sayoner-000/gomemory.git
cd gomemory
go build -o mem ./infrastructure/
./mem install .
```

### Configuración Manual MCP

<details>
<summary>Si prefieres no usar el instalador</summary>

Con `mem` en el PATH, `claude mcp add -s user gomemory mem mcp` registra el
mismo resultado que `mem setup-mcp --scope global --agents claude` (delegar
en el CLI de Claude Code es más seguro que editar `~/.claude.json` a mano:
es un archivo grande con formato propio). Si prefieres editarlo directamente,
la entrada global vive en `~/.claude.json` → `mcpServers.gomemory`:

```json
{
  "mcpServers": {
    "gomemory": {
      "command": "/ruta/a/mem",
      "args": ["mcp"]
    }
  }
}
```

Para scope de proyecto en vez de global, la misma entrada va en `.mcp.json`
en la raíz del repo. Reiniciar el agente. Verificar con `/mcp` — deberías ver
`gomemory` con 9 tools.

> Nota: si existen **ambos** (un `.mcp.json` de proyecto y una entrada global
> con la misma clave `gomemory`), el de proyecto tiene precedencia — confirmado
> empíricamente. Si registraste el scope global y no ves el cambio, revisa que
> el repo actual no tenga su propio `.mcp.json` residual.

</details>

---

## Multi-Agente

Dos formas de configurar agentes, según si soportan registro MCP a nivel de usuario:

**Global (una vez por máquina)** — `mem setup-mcp --scope global --agents claude,codex,opencode`:

| Agente | Config MCP | Notas |
|--------|-----------|-------|
| **Claude Code** | `~/.claude.json` → `mcpServers.gomemory` (scope `user`) | Registrado vía `claude mcp add`, con detección de colisión de nombre |
| **Codex** | `~/.codex/config.toml` → `[mcp_servers.gomemory]` | Tabla única, sin `cwd` por proyecto |
| **OpenCode** | `~/.config/opencode/opencode.json` → `mcp.gomemory` | OpenCode mergea la config de usuario con la del proyecto activo (confirmado con `opencode debug config`); el plugin se instala en el mismo paso |

**Por proyecto (`mem install`, o `mem setup-mcp --scope project`)** — necesario para el resto, opcional para Claude/Codex/OpenCode:

| Agente | Config MCP | Instrucciones | Hooks |
|--------|-----------|---------------|-------|
| **Claude Code** | `.mcp.json` | `AGENTS.md` + `CLAUDE.md` | SessionStart, SessionEnd, PreCompact, UserPromptSubmit, Stop |
| **OpenCode** | `opencode.json` | `AGENTS.md` | `plugin/opencode/plugin.ts` (auto-inicio, ya es global) |
| **Cursor** | `.cursor/mcp.json` | — | — |
| **Windsurf** | `.windsurf/mcp.json` | — | — |
| **Cline** | `.cline/mcp.json` | — | — |
| **Codex** | `~/.codex/config.toml` (tabla por proyecto, `gomemory_<proyecto>`) | `.codex/AGENTS.md` | SessionStart |

> Los hooks son subcomandos del binario (`mem hook <evento>`), no scripts shell:
> no dependen de `bash`/`curl` ni de un servidor HTTP, y corren igual en Windows.
>
> Regla de oro: un hook nunca aborta el arranque del agente — ante cualquier
> error sale silencioso con código 0.

---

## Herramientas MCP

### Indexación

| Tool | Descripción |
|------|-------------|
| `save_memory` | Guardar una memoria (learning, decision, bugfix, pattern, discovery, architecture, preference) |
| `list_memories` | Listar las memorias más recientes del proyecto |
| `search_memories` | Buscar en todas las memorias por texto |
| `get_memory` | Obtener una memoria específica por ID |

### Consulta

| Tool | Descripción |
|------|-------------|
| `get_context` | Obtener el contexto completo del proyecto como markdown |
| `get_symbol` | Obtener la definición de un símbolo (función/método/tipo) con callers y callees |
| `search_code` | Buscar símbolos de código por nombre, firma o paquete |
| `list_dependencies` | Recorrer el grafo de dependencias de un símbolo |

### Sesión

| Tool | Descripción |
|------|-------------|
| `start_session` | Iniciar una nueva sesión de trabajo |
| `end_session` | Finalizar la sesión activa con un resumen |

### Gestión

| Tool | Descripción |
|------|-------------|
| `forget_memory` | Borrar una memoria específica por ID (irreversible) |
| `judge_memories` | Comparar dos memorias en conflicto y registrar veredicto semántico |

### Recursos MCP

| Recurso | URI | Descripción |
|---------|-----|-------------|
| Contexto | `mem://context` | Contexto completo del proyecto |
| Memoria | `mem://memory/{id}` | Contenido de una memoria específica |

---

## Arquitectura

```
gomemory/
├── domain/                    Modelos de negocio
│   ├── memory.go              Memory (tipo, título, contenido, sesiones)
│   ├── session.go             Session (UUID, resumen, timestamps)
│   ├── relation.go            Relation (tipos semánticos entre memorias)
│   ├── code.go                Code (indexado de símbolos)
│   ├── errors.go              Errores del dominio
│   └── redact.go              Redacción de contenido <private>
│
├── application/               Lógica de negocio
│   ├── ports/                 Interfaces (puertos)
│   │   ├── memory_repository.go
│   │   ├── session_repository.go
│   │   ├── relation_repository.go
│   │   ├── context_builder.go
│   │   ├── code_graph_repository.go
│   │   ├── project_repository.go
│   │   ├── maintenance_repository.go
│   │   └── settings_repository.go
│   └── usecases/              Casos de uso
│       ├── build_context.go   Construcción de contexto para el agente
│       ├── index_project.go   Indexación de código fuente
│       ├── record_verdict.go  Veredicto semántico entre memorias
│       └── goparse.go         Parsing de código Go
│
├── adapters/                  Adaptadores (entrada/salida)
│   ├── primary/               Entradas
│   │   ├── cli/               Comandos de terminal
│   │   ├── mcp/               Servidor MCP (9 tools)
│   │   ├── tui/               Interfaz terminal (Bubbletea)
│   │   └── setup/             Instalación y configuración
│   └── secondary/             Salidas
│       └── persistence/       SQLite (lectura/escritura)
│
├── infrastructure/            Orquestación
│   ├── main.go                Punto de entrada (CLI + MCP + HTTP)
│   ├── container.go           Contenedor de dependencias
│   ├── plugin/                Plugins multi-agente
│   │   ├── opencode/          plugin.ts para OpenCode
│   │   └── claude-code/       Hooks + skill para Claude Code
│   └── templates/             Plantillas de configuración
│
├── scripts/                   Instaladores
│   ├── install.sh             Linux / macOS
│   └── install.ps1            Windows (PowerShell)
│
├── tests/                     Pruebas
│   ├── unit/                  Unitarias
│   ├── integration/           Integración
│   └── contract/              Contrato (Memory Protocol)
│
├── docs/                      Documentación
│   ├── architecture.md        Arquitectura completa
│   ├── MANUAL.md              Guía paso a paso
│   ├── PLUGINS.md             Sistema de plugins
│   └── MEMORY-PROTOCOL.md     Protocolo técnico
│
└── version/                   Constante de versión
```

**Flujo de datos**: `domain` define los modelos → `application/ports` define las interfaces → `adapters/primary` (CLI/MCP/TUI) recibe input del usuario → `adapters/secondary/persistence` escribe en SQLite → `infrastructure` orquesta todo.

---

## Configuración

### Ajustes

```bash
mem settings --show             # ver configuración actual
mem settings --auto-approve     # togglear auto-approve de tools MCP
```

### Variables de Entorno

No requiere ninguna para operación normal — el proyecto se identifica por su
git root y el store de datos se resuelve solo. Una variable opcional:

| Variable | Valor por defecto | Descripción |
|----------|-------------------|-------------|
| `GOMEMORY_DATA_HOME` | `$XDG_DATA_HOME/gomemory` o `~/.local/share/gomemory` (Linux/macOS); `%LOCALAPPDATA%\gomemory` (Windows) | Override del directorio de datos donde vive `projects/<clave>/mem.db` |

---

## Pruebas

```bash
go build -o mem ./infrastructure/      # compilar
go vet ./...                           # verificación estática
go test ./... -v                       # tests unitarios + integración + contrato
```

---

## Solución de Problemas

| Problema | Solución |
|----------|----------|
| `/mcp` no muestra el servidor | Verificar que `.mcp.json` tenga la ruta absoluta a `mem`. Reiniciar el agente. |
| `mem install` falla | Verificar que `mem` esté en el PATH: `which mem` (Linux/macOS) o `where mem` (Windows). |
| Hooks no se ejecutan | Verificar que estén configurados: `cat .claude/settings.json \| jq .hooks`. Usar `mem setup claude-code` para re-instalar. |
| Memoria no se persiste entre sesiones | Verificar que `.memory/` exista y tenga permisos de escritura. Ejecutar `mem init --force`. |
| Binario no encontrado después de instalar | Agregar al PATH: `export PATH="$HOME/.local/bin:$PATH"` (Linux/macOS). |
| `mem update` en Windows | El binario en ejecución no se sobrescribe. Cerrar el proceso y ejecutar el comando que `mem update` sugiere. |
| Contexto muy grande | Ejecutar `mem gc` para limpiar memorias antiguas (retención de 90 días por defecto). |
| Base de datos corrupta | Ejecutar `mem compact` para recuperar espacio. Si persiste, borrar `.memory/mem.db` y re-indexar. |

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

## Seguridad

- **Sin telemetría** — gomemory no envía datos a ningún servidor. Todo ocurre localmente.
- **Binario autocontenido** — sin dependencias compartidas que puedan ser comprometidas.
- **Redacción automática** — contenido envuelto en `<private>...</private>` se elimina antes de llegar a la base de datos.
- **SQLite WAL** — integridad ACID con Write-Ahead Logging. Los datos sobreviven cortes de energía.
- **Permisos MCP granulares** — `forget_memory` queda fuera de auto-approve por ser destructivo/irreversible.

---

## Documentación

| Documento | Descripción |
|-----------|-------------|
| [`docs/architecture.md`](docs/architecture.md) | Arquitectura completa del proyecto |
| [`docs/PLUGINS.md`](docs/PLUGINS.md) | Sistema de plugins multi-agente |
| [`docs/MEMORY-PROTOCOL.md`](docs/MEMORY-PROTOCOL.md) | Protocolo de memoria (referencia técnica) |
| [`docs/MANUAL.md`](docs/MANUAL.md) | Guía paso a paso para usuarios |

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
de código es viable y valiosa — gomemory lleva esa idea a un stack autocontenido en
Go con SQLite embebido, multi-agente, y sin dependencias runtime.
