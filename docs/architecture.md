# Arquitectura de gomemory

## Visión General

`gomemory` es un CLI + TUI + MCP server en Go que persiste contexto de agentes AI por proyecto. Usa SQLite embebido (sin CGO) como almacenamiento y expone la memoria como herramientas nativas vía MCP (Model Context Protocol) para múltiples agentes.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         gomemory                                       │
│                                                                          │
│  ┌─────────┐  ┌──────────┐  ┌─────────┐  ┌────────────┐  ┌──────────┐  │
│  │  CLI    │  │   TUI    │  │  Wrap   │  │  Context   │  │MCP Server│  │
│  │  cmds   │  │ Bubbletea│  │ wrapper │  │  builder   │  │ (stdio)  │  │
│  └────┬────┘  └────┬─────┘  └────┬────┘  └─────┬──────┘  └────┬─────┘  │
│       │            │             │              │              │        │
│       └────────────┴─────────────┴──────────────┴──────────────┘        │
│                            │                                             │
│                    ┌───────┴───────┐                                     │
│                    │    Store     │                                     │
│                    │  (SQLite)    │                                     │
│                    └───────┬───────┘                                     │
│                            │                                             │
│                    ┌───────┴───────┐                                     │
│                    │ .memory/mem.db│                                     │
│                    └───────────────┘                                     │
└──────────────────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
   opencode    Claude    Cursor/Windsurf/Cline    Codex
   (.opencode.json) (.mcp.json) (.cursor/mcp.json, etc.) (~/.codex/config.toml)
```

## Componentes

### 1. CLI (`adapters/primary/cli/`)

Dispatcher central. Enruta subcomandos a handlers según `os.Args[1]`.

| Subcomando | Archivo | Función |
|---|---|---|
| `init` | `adapters/primary/cli/cmd_init.go` | Crea `.memory/` y tablas SQLite |
| `save` | `adapters/primary/cli/cmd_save.go` | Inserta una memoria con tipo, título, contenido |
| `capture` | `adapters/primary/cli/cmd_capture.go` | Guarda aprendizaje estructurado con formato What/Why/Where/Learned |
| `compare` | `adapters/primary/cli/cmd_compare.go` | Compara dos memorias ([flags] id1 id2) y persiste veredicto semántico |
| `compare list` | `adapters/primary/cli/cmd_compare.go` | Lista relaciones guardadas entre memorias |
| `project` | `adapters/primary/cli/cmd_project.go` | Detecta el proyecto actual (read-only: nombre, raíz, BD, conteo) |
| `list` / `log` | `adapters/primary/cli/cmd_list.go` | Lista memorias recientes en formato tabla |
| `search` | `adapters/primary/cli/cmd_search.go` | Busca por LIKE en título + contenido con ranking (título primero) |
| `context` | `adapters/primary/cli/cmd_context.go` | Genera contexto markdown para el agente AI (read-only por defecto) |
| `session` | `adapters/primary/cli/cmd_session.go` | Gestiona sesiones de trabajo (start/end/list) |
| `install` | `adapters/primary/cli/cmd_install.go` | Copia binario + init + .gitignore + AGENTS + configura MCP para todos los agentes |
| `wrap` | `adapters/primary/cli/cmd_wrap.go` | Ejecuta comando y pregunta si guardar al terminar |
| `mcp` | `adapters/primary/cli/cmd_mcp.go` | Servidor MCP sobre stdio con 7 tools y 2 recursos. Acepta `--root <dir>` |
| `setup` | `adapters/primary/cli/cmd_setup.go` | Instala el plugin + hooks de un agente (`opencode`, `claude-code`) |
| `setup-mcp` / `mcp-setup` | `adapters/primary/cli/cmd_mcp_setup.go` | Configura MCP para opencode, Claude, Cursor, Windsurf, Cline y/o Codex |
| `serve` | `adapters/primary/cli/cmd_serve.go` | Servidor HTTP de plugins (`127.0.0.1:9735`). Lo usa el plugin de OpenCode |
| `hook` | `adapters/primary/cli/cmd_hook.go` | Entrypoint portable de hooks de Claude Code (`session-start`, `session-end`, `pre-compact`, `user-prompt-submit`) |
| `settings` | `adapters/primary/cli/cmd_settings.go` | Ver o cambiar auto-approve de las tools MCP (`--auto-approve`, `--show`) |
| `purge` | `adapters/primary/cli/cmd_purge.go` | Borra memorias (proyecto actual por defecto; `--all`/`--type`/`--older-than-days`) |
| `compact` | `adapters/primary/cli/cmd_compact.go` | `VACUUM` de `.memory/mem.db` (recupera espacio, no borra nada) |
| `gc` | `adapters/primary/cli/cmd_gc.go` | Garbage collection por antigüedad a demanda (90 días por defecto) |
| `uninstall` | `adapters/primary/cli/cmd_uninstall.go` | Reverso de `install`: quita binario, hooks, MCP, bloques en AGENTS/CLAUDE y datos |
| `tui` | `adapters/primary/cli/cli.go:LaunchTUI()` | Abre interfaz TUI explícitamente |
| *(sin args)* | `adapters/primary/cli/dispatcher.go` | Abre TUI automáticamente |

Flujo:

```
os.Args → infrastructure/main.go → NewContainer() → cli.Run(cmd, args, deps)
                                                       ↓
                                               cli.Dispatcher switch
                                                       ↓
                                              CmdXxx(deps, args)
                                                       ↓
                                              deps.MemoryRepo / deps.ProjectRepo
                                                       ↓
                                          adapters/secondary/persistence/
```

### 2. TUI (`adapters/primary/tui/tui.go`)

Interfaz de terminal con [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles).

**Estados (screens):**
- `screenList` — listado de memorias agrupadas por tipo (arquitectura, decisión, patrón, bugfix, learning, discovery), con cursor de navegación y búsqueda en vivo
- `screenDetail` — vista detalle de una memoria seleccionada con tipo, fecha, archivo relacionado y sesión
- `screenSave` — formulario multi-campo (título, tipo, contenido, archivo) con validación

**Arquitectura del modelo:**

```
model
├── memRepo, sessRepo, root, project  ← contexto de la persistencia
├── screen                        ← estado actual (list/detail/save)
├── memories / cursor             ← lista completa con cursor de navegación
├── searching / search            ← búsqueda en vivo (filtrado por título/tipo/contenido)
├── selected                      ← memoria seleccionada para detalle
└── save{Title,Type,Content,Filepath}  ← formulario con 4 campos + focus
```

**Atajos de teclado:**
| Tecla | Acción |
|---|---|
| `↑↓` o `jk` | Navegar |
| `Enter` | Ver detalle |
| `s` | Guardar nueva memoria |
| `/` | Activar/desactivar búsqueda en vivo |
| `Tab` / `↑↓` | Cambiar campo en formulario |
| `Esc` | Volver / cancelar / salir de búsqueda |
| `q` / `Ctrl+C` | Salir |

**Visual:**
- Memorias agrupadas por tipo con cabeceras (ej. "▲ Arquitectura (3)")
- Item seleccionado con fondo gris y tag de tipo coloreado
- Búsqueda en vivo: filtra mientras escribes por título, contenido o tipo
- Pantalla alternativa (`tea.WithAltScreen()`) para no ensuciar el historial

### 3. Persistence (`adapters/secondary/persistence/`)

Capa de persistencia sobre SQLite usando [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) (SQLite puro Go, sin CGO).

```
adapters/secondary/persistence/
├── db.go
│   ├── FindRoot()        ← busca .memory/ desde CWD hacia padres
│   ├── EnsureDir()        ← crea .memory/ si no existe
│   ├── DbPath()           ← path completo a .memory/mem.db
│   ├── Open()             ← abre DB + migraciones automáticas (WAL mode, busy timeout 5s)
│   ├── Init()             ← EnsureDir + Open
│   └── migrate()          ← CREATE TABLE IF NOT EXISTS (memories, sessions, memory_relations)
│
├── memory.go
│   ├── InsertMemory()     ← INSERT con timestamps UTC-5
│   ├── ListMemories()     ← SELECT by project, ordenado DESC, limitable (máx 200)
│   └── SearchMemories()   ← SELECT con LIKE en title/content, ranking: title match primero
│
├── session.go
│   ├── StartSession()     ← INSERT con UUID aleatorio (hex aleatorio 16 bytes)
│   ├── EndSession()       ← UPDATE ended_at + summary
│   ├── ActiveSession()    ← busca sesión sin ended_at
│   └── RecentSessions()   ← últimas N sesiones
│
├── relation.go
│   ├── InsertRelation()   ← INSERT de veredicto entre dos memorias
│   ├── UpdateRelation()   ← UPDATE de veredicto existente (idempotente)
│   ├── GetRelation()      ← SELECT por ID
│   ├── GetRelationByPair()← SELECT por par (memory_id_a, memory_id_b) en cualquier orden
│   └── ListRelations()    ← últimas N relaciones
│
├── settings.go            ← Config local (auto-approve, etc.)
│
└── repositories.go        ← Wrappers que implementan ports.*Repository
```

La conexión SQLite usa WAL mode y busy timeout de 5s para mejor concurrencia. El archivo `repositories.go` envuelve las funciones CRUD raw en structs que implementan las interfaces definidas en `application/ports/`, permitiendo que la capa de aplicación dependa solo de contratos.

### 4. Context Builder (`application/usecases/build_context.go`)

Usa las interfaces `MemoryLister` + `SessionQuerier` definidas en `application/ports/context_builder.go` en lugar de depender directamente de `*sql.DB`.

Genera un markdown estructurado con toda la memoria del proyecto, agrupado por tipo:

```
mem context        → imprime en stdout
mem context --write → escribe .memory/context.md
```

**Estructura del contexto generado:**

```
# Memoria del Proyecto

## Decisiones de Arquitectura
- **título**: contenido (→ archivo relacionado)

## Decisiones Técnicas
- **título**: contenido

## Patrones y Convenciones
- **título**: contenido

## Bugfixes
- **título**: contenido (→ archivo relacionado)

## Aprendizajes Recientes
- **título**: contenido (archivo relacionado)
...

## Sesión Activa
- Iniciada: timestamp

## Sesiones Recientes
- inicio → fin: resumen
```

El archivo `.memory/context.md` es leído por los agentes AI al inicio de cada sesión.

### 5. Wrap (`adapters/primary/cli/cmd_wrap.go`)

Wrapper interactivo que envuelve cualquier comando:

1. Auto-inicia sesión si no hay activa (flag `-s true`)
2. Ejecuta el comando con stdin/stdout/stderr completo (passthrough)
3. Captura el código de salida del comando hijo
4. Al terminar pregunta interactivamente: `¿Guardar algo en memoria? (s/N)`
5. Si acepta, recolecta título/tipo/contenido y persiste
6. Exit code propagado: el wrap termina con el mismo código que el comando envuelto

### 6. MCP Server (`adapters/primary/mcp/server.go`)

Servidor MCP (Model Context Protocol) sobre transporte stdio. Usa la SDK oficial [`github.com/modelcontextprotocol/go-sdk`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk). Ahora usa las interfaces `MemoryRepository` + `SessionRepository` en lugar de depender directamente de `*sql.DB`.

**Herramientas:**

| Tool | Input | Descripción |
|---|---|---|
| `save_memory` | title, type, content, filepath | Guarda una memoria con sesión activa si existe |
| `search_memories` | query, limit | Busca en la memoria con ranking por relevancia |
| `list_memories` | limit | Lista memorias recientes |
| `get_memory` | id | Obtiene una por ID (tipo, título, fecha, sesión, contenido) |
| `start_session` | — | Inicia sesión de trabajo (valida que no haya activa) |
| `end_session` | summary | Finaliza sesión activa |
| `get_context` | — | Contexto markdown completo del proyecto |

**Recursos:**

| URI | Descripción |
|---|---|
| `mem://context` | Contexto markdown del proyecto |
| `mem://memory/{id}` | Memoria específica por ID (resource template) |

### 7. Capture (`adapters/primary/cli/cmd_capture.go`)

Guarda aprendizajes con formato estructurado:

```
mem capture -w "implementar auth JWT" -y "seguridad stateless" -f "middleware.go" -l "usar refresh tokens"
mem capture -i  # modo interactivo
```

El contenido se construye como markdown estructurado:

```
**What**: implementar auth JWT
**Why**: seguridad stateless
**Where**: middleware.go
**Learned**: usar refresh tokens
```

Se asocia automáticamente a la sesión activa si existe.

### 8. Compare (`adapters/primary/cli/cmd_compare.go`)

Compara dos memorias y persiste un veredicto semántico en la tabla `memory_relations`:

```
mem compare -r supersedes -c 0.9 -m "la nueva decisión reemplaza a la anterior" 1 2
```

**Tipos de relación:**

| Relación | Descripción |
|---|---|
| `related` | Las memorias están semanticamente relacionadas |
| `compatible` | Las memorias son compatibles entre sí |
| `scoped` | Una memoria es caso específico de la otra |
| `conflicts_with` | Las memorias entran en conflicto |
| `supersedes` | Una memoria reemplaza a la otra |
| `not_conflict` | Se evaluaron y no hay conflicto |

Características:
- Idempotente: si ya existe una relación para el par, la actualiza
- Validación: verifica que ambas memorias existan en el proyecto
- Listado: `mem compare list [-n N]` muestra relaciones recientes

### 9. Project (`adapters/primary/cli/cmd_project.go`)

Comando read-only para detectar el proyecto actual:

```
mem project
```

Salida:
```
Proyecto: gomemory
Raíz:     /home/user/proyecto
BD:       /home/user/proyecto/.memory/mem.db
Memorias: 12
Sesión:   Activa desde 2026-06-18 10:00:00
```

### 10. MCP Setup Multi-Agente (`adapters/primary/cli/cmd_mcp_setup.go`)

Configura la integración MCP para múltiples agentes AI desde un solo comando:

```
mem setup-mcp [--target dir] [--agents opencode,claude,cursor,windsurf,cline,codex,all]
```

Soporta 6 agentes:

| Agente | Archivo destino | Configuración |
|---|---|---|
| `opencode` | `.opencode.json` | `mcpServers.gomemory = { command, args: ["mcp", "--root", absRoot] }` |
| `claude` | `.mcp.json` (proyecto) | `mcpServers.gomemory` — servidor MCP sobre stdio |
| `cursor` | `.cursor/mcp.json` | `mcpServers.gomemory` |
| `windsurf` | `.windsurf/mcp_config.json` | `mcpServers.gomemory` |
| `cline` | `.cline/mcp_settings.json` | `mcpServers.gomemory` con `disabled: false` |
| `codex` | `~/.codex/config.toml` (global, una tabla por proyecto) | `[mcp_servers."gomemory_<proyecto>"]` con `command`, `args`, `cwd` |

Cada función de setup (`setupOpenCode`, `setupClaude`, `setupCursor`, `setupWindsurf`, `setupCline`, `setupCodex`) es idempotente: detecta si la configuración ya existe y la salta o actualiza. `setupCodex` nunca reescribe el archivo completo — solo hace `append` de su propia tabla TOML, para no arriesgar corromper otras entradas ya presentes en `~/.codex/config.toml`.

El flag `--agents all` configura los 6 agentes en un solo comando.

### El flag `--root`: por qué existe

`mem mcp` resuelve el proyecto vía `ProjectRepo.FindRoot()`, que sube directorios desde el `cwd` del proceso buscando `.memory/`. Cuando un agente (Claude, Cursor, etc.) lanza el servidor MCP, **no garantiza qué `cwd` usará** para el subproceso — puede ser el directorio desde donde se abrió el editor, no el proyecto instalado. Por eso, cada configuración generada por `setupX`/`setupCodex` incluye `args: ["mcp", "--root", absRoot]`: el servidor recibe la raíz del proyecto explícitamente, sin depender del `cwd` real. Si `--root` no se pasa (por ejemplo, al ejecutar `./mem mcp` manualmente desde dentro del proyecto), se mantiene el comportamiento anterior basado en `FindRoot()`.

### 11. Hooks portables de Claude Code (`adapters/primary/cli/cmd_hook.go`)

`mem hook <evento>` es el entrypoint único de los hooks de Claude Code. Reemplaza los scripts `bash` + `curl` legados (`session-start.sh`, etc.) por subcomandos del binario que hablan directo a los repositorios: **sin shell, sin `curl`, sin servidor HTTP**. Funcionan igual en Linux, macOS y Windows.

**Regla de oro:** un hook NUNCA aborta el arranque del agente. Ante cualquier error sale con código 0 y, como mucho, sin salida.

Se registran en `.claude/settings.json` (`mem setup claude-code` o `mem hook`), mapeando cada evento de Claude Code a su subcomando:

| Evento Claude Code | Subcomando | Para qué sirve |
|---|---|---|
| `SessionStart` | `session-start` | Abre una sesión si no hay activa **e inyecta el contexto de sesiones previas** como `additionalContext`. El agente arranca recordando el proyecto sin que se lo pidan |
| `SessionEnd` | `session-end` | Cierra la sesión activa como **red de seguridad** (acepta un `summary` opcional por stdin). Evita sesiones colgadas aunque el modelo no llame `end_session` |
| `PreCompact` | `pre-compact` | Antes de compactar el contexto, inyecta **instrucciones de recuperación + el contexto previo** para que la compactación no borre el estado de trabajo |
| `UserPromptSubmit` | `user-prompt-submit` | En el **primer** prompt de la sesión activa las tools MCP de memoria e inyecta un recordatorio del protocolo; en los prompts siguientes es pasivo (un marcador `.session-tools-injected` evita overhead por prompt) |

Los hooks son el mecanismo que hace que la memoria "tome todo bien" en Claude Code: sin ellos, las tools MCP existen pero nadie abre/cierra sesiones ni recupera contexto automáticamente.

### 12. Servidor HTTP de plugins (`adapters/primary/cli/cmd_serve.go`)

`mem serve` levanta un servidor HTTP en `127.0.0.1:9735` (flag `--port`). Lo **auto-inicia el plugin de OpenCode** (`plugin.ts`) para gestionar sesiones y contexto vía HTTP. Los hooks de Claude Code **no** lo usan (van directo a los repos vía `mem hook`).

| Endpoint | Método | Descripción |
|---|---|---|
| `/session/start` | POST | Crea (o reusa) la sesión activa |
| `/session/end` | POST | Cierra la sesión con `{"summary": "..."}` |
| `/context` | GET | Contexto markdown de sesiones previas |
| `/health` | GET | Healthcheck (`{"status":"ok","project":"..."}`) |

### 13. Plugin Setup (`adapters/primary/cli/cmd_setup.go` + `adapters/primary/setup/`)

`mem setup <agente>` instala el plugin **y sus hooks** para un agente concreto:

- `mem setup opencode` → copia `plugin.ts` a `~/.config/opencode/plugins/gomemory/` y lo referencia en `opencode.json`.
- `mem setup claude-code` → copia el plugin a `.claude/plugins/gomemory/`, escribe `.mcp.json` y registra los hooks portables (`mem hook <evento>`) en `.claude/settings.json`.

La referencia al binario es portable (`BinRef`/`binRefFor` en `cmd_install.go`/`binref.go`): se usa `mem` por PATH, nunca una ruta absoluta de máquina. El fallback por-proyecto de los hooks de Claude usa `${CLAUDE_PROJECT_DIR}/mem`, que Claude expande en runtime.

> **Importante:** `mem install` configura el **MCP** de los 6 agentes, pero **no** registra los hooks/plugins. Los hooks se instalan con `mem setup claude-code` / `mem setup opencode`.

### 14. Mantenimiento de memoria (`cmd_purge.go`, `cmd_compact.go`, `cmd_gc.go` + `adapters/secondary/persistence/maintenance.go`)

Operaciones destructivas que exigen confirmación humana y **no se exponen vía MCP** (para que un agente no pueda borrar memoria por su cuenta). La lógica de borrado/VACUUM vive en `persistence/maintenance.go` (`PurgeMemories`, `CompactDB`, `StatsQuery`, `FileSize`).

| Comando | Qué hace |
|---|---|
| `mem purge` | Borra memorias del proyecto actual por defecto; `--all` (todos los proyectos), `--type`, `--older-than-days`, `--yes`. Al borrar una memoria limpia también sus relaciones (`mem compare`) |
| `mem compact` | `VACUUM` de `.memory/mem.db`: recupera el espacio liberado por borrados. Nunca elimina memorias; reporta tamaño antes/después |
| `mem gc` | Garbage collection por antigüedad a demanda (90 días por defecto), reutilizando la lógica de `purge`. Solo corre cuando el usuario lo pide — nunca en segundo plano |

También disponibles desde la TUI (tecla `m`), salvo la desinstalación.

### 15. Desinstalación (`adapters/primary/cli/cmd_uninstall.go`)

`mem uninstall [dir] [--yes]` es el reverso exacto de `install`: remueve el binario `mem`, los hooks de `.claude/settings.json`, el registro MCP en `.mcp.json` y configs equivalentes, los bloques inyectados en `AGENTS.md`/`CLAUDE.md` y los datos (`.memory/`). Reporta los componentes que no encontró sin fallar. El archivo global `~/.codex/config.toml` no se toca automáticamente: se informa al usuario para que lo edite si usó el agente Codex.

## Instalador universal de consola (`scripts/install.sh`, `scripts/install.ps1`)

Para que toda la config de agentes pueda referenciar `mem` por nombre (no por ruta absoluta), el binario debe estar en el PATH. Los instaladores de consola lo dejan ahí sin compilar ni clonar, en Linux, macOS y Windows:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh | bash
# Windows (PowerShell)
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
```

Descargan el binario del release de GitHub correspondiente al SO/arquitectura y lo instalan (Linux/macOS: `GOMEMORY_VERSION`, `GOMEMORY_BIN_DIR` para fijar versión/destino; `--uninstall`/`-Uninstall` para remover). Los releases los publica GoReleaser vía el workflow `.github/workflows/release.yml` al pushear un tag `v*`.

## Flujo de Instalación (`adapters/primary/cli/cmd_install.go`)

```
mem install /ruta/a/proyecto
  │
  ├─ 1. Copiar binario → /ruta/a/proyecto/mem
  │
  ├─ 2. Verificar/Inicializar .memory/
  │    ├─ .memory/mem.db existe y es válido → verificado ✅
  │    ├─ .memory/mem.db existe y corrupto → init --force
  │    └─ No existe → init (crea .memory/ + tablas)
  │
  ├─ 3. Actualizar .gitignore (añade .memory/ y /mem)
  │
  ├─ 4. AGENTS.md, CLAUDE.md, .cursorrules, .windsurfrules — el "pack" de trabajo
  │    ├─ No existen → se crean con [reglas de trabajo] + [protocolo de memoria]
  │    ├─ Existen sin integración → inyecta el preámbulo de reglas ANTES del
  │    │                            bloque ## Memoria Persistente (idempotente)
  │    └─ Existen con ambos marcadores → saltar
  │         · Preámbulo: templates/agent-preamble.md (go:embed), marcador
  │           <!-- gomemory-workrules-v1 -->
  │         · Protocolo: buildIntegrationBlock(), marcador
  │           <!-- gomemory-protocol-v2 -->
  │
  ├─ 4b. Constitución → copia templates/speckit-constitution-gen.md a la raíz
  │      del proyecto (solo si no existe; nunca sobrescribe)
  │
  └─ 5. MCP server config para TODOS los agentes (NO instala hooks/plugins;
       │  eso lo hace `mem setup claude-code` / `mem setup opencode`)
       ├─ opencode → .opencode.json
       ├─ claude → .mcp.json
       ├─ cursor → .cursor/mcp.json
       ├─ windsurf → .windsurf/mcp_config.json
       ├─ cline → .cline/mcp_settings.json
       └─ codex → ~/.codex/config.toml (tabla por proyecto)
```

El contenido inyectado se versiona con marcadores HTML para upgrades idempotentes: si una instalación previa dejó un bloque viejo (sin el marcador de versión vigente), `mem install` lo reemplaza en lugar de duplicarlo. Tanto el preámbulo de reglas como la constitución viven embebidos en el binario (`infrastructure/templates/`, `go:embed`), así que `mem install` no depende de archivos presentes en disco ni del `cwd`.

## Modelo de Datos

### `memories`

| Columna | Tipo | Descripción |
|---|---|---|
| `id` | INTEGER PK | Auto-incremental |
| `project` | TEXT | Nombre del proyecto (basename del root) |
| `session_id` | TEXT? | Sesión asociada (UUID) |
| `type` | TEXT | `learning`, `decision`, `architecture`, `bugfix`, `pattern`, `discovery` |
| `title` | TEXT | Título descriptivo |
| `content` | TEXT | Cuerpo del aprendizaje |
| `filepath` | TEXT? | Archivo relacionado |
| `created_at` | TEXT | Timestamp UTC-5 (Colombia) |
| `updated_at` | TEXT | Timestamp UTC-5 (Colombia) |

Índices: `project`, `type`, `created_at DESC`.

### `sessions`

| Columna | Tipo | Descripción |
|---|---|---|
| `id` | TEXT PK | UUID v4 (hex 16 bytes: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`) |
| `project` | TEXT | Nombre del proyecto |
| `summary` | TEXT | Resumen de la sesión |
| `created_at` | TEXT | Inicio de sesión (UTC-5) |
| `ended_at` | TEXT? | Fin de sesión (UTC-5) |

### `memory_relations`

| Columna | Tipo | Descripción |
|---|---|---|
| `id` | INTEGER PK | Auto-incremental |
| `project` | TEXT | Nombre del proyecto |
| `memory_id_a` | INTEGER | FK → memories(id) |
| `memory_id_b` | INTEGER | FK → memories(id) |
| `relation` | TEXT | `related`, `compatible`, `scoped`, `conflicts_with`, `supersedes`, `not_conflict` |
| `confidence` | REAL | Confianza del veredicto (0.0-1.0) |
| `reasoning` | TEXT | Razonamiento del veredicto |
| `created_at` | TEXT | Timestamp UTC-5 (Colombia) |

Índices: `project`, `memory_id_a`, `memory_id_b`.

### Tipos de memoria

| Tipo | Icono | Constante | Uso |
|---|---|---|---|
| `architecture` | ▲ | `types.Architecture` | Decisiones de arquitectura |
| `decision` | ◆ | `types.Decision` | Decisiones técnicas importantes |
| `pattern` | ■ | `types.Pattern` | Patrones y convenciones |
| `bugfix` | ✕ | `types.Bugfix` | Bugs corregidos y causa raíz |
| `learning` | ● | `types.Learning` | Descubrimientos y aprendizajes |
| `discovery` | ◇ | `types.Discovery` | Hallazgos sin categoría |

Validación vía `ValidMemoryType()`: si el tipo no es válido, default a `learning`.

## Tipos Go (`domain/`)

Definidos en `domain/memory.go`, `domain/session.go`, `domain/relation.go` y `domain/errors.go`.

```go
type MemoryType string

const (
    Learning     MemoryType = "learning"
    Decision     MemoryType = "decision"
    Architecture MemoryType = "architecture"
    Bugfix       MemoryType = "bugfix"
    Pattern      MemoryType = "pattern"
    Discovery    MemoryType = "discovery"
)

type Memory struct {
    ID        int64
    Project   string
    SessionID string
    Type      MemoryType
    Title     string
    Content   string
    Filepath  string
    CreatedAt string
    UpdatedAt string
}

type Session struct {
    ID        string
    Project   string
    Summary   string
    CreatedAt string
    EndedAt   *string
}

type RelationType string

const (
    Related       RelationType = "related"
    Compatible    RelationType = "compatible"
    Scoped        RelationType = "scoped"
    ConflictsWith RelationType = "conflicts_with"
    Supersedes    RelationType = "supersedes"
    NotConflict   RelationType = "not_conflict"
)

type Confidence float64

type Relation struct {
    ID         int64
    Project    string
    MemoryID_A int64
    MemoryID_B int64
    Relation   RelationType
    Confidence Confidence
    Reasoning  string
    CreatedAt  string
}
```

## Dependencias Externas

| Librería | Propósito |
|---|---|
| `modernc.org/sqlite` | SQLite embebido sin CGO |
| `charmbracelet/bubbletea` | Framework TUI (Elm-style) con alt screen |
| `charmbracelet/bubbles` | Componentes TUI (textinput) |
| `charmbracelet/lipgloss` | Estilos de terminal (colores adaptativos light/dark) |
| `github.com/modelcontextprotocol/go-sdk` | SDK oficial MCP para servidor con tools y resources |

Zero dependencias en runtime para el usuario final. El binario compilado es autocontenido (~16MB).

## Variables de Entorno

No requiere variables de entorno para operación normal. Toda la configuración es implícita (directorio actual + `.memory/`).

## Decisiones Técnicas

1. **SQLite sin CGO**: `modernc.org/sqlite` evita depender de gcc/libsqlite3. Binario portátil.

2. **Búsqueda de `.memory/` hacia arriba**: `FindRoot()` sube directorios hasta encontrar `.memory/`, permitiendo ejecutar `mem` desde cualquier subdirectorio.

3. **WAL mode**: `_pragma=journal_mode(WAL)` permite lectores concurrentes sin bloqueo. Busy timeout de 5s.

4. **Timestamps en UTC-5**: `datetime('now', '-5 hours')` para zona horaria Bogotá/Colombia (sin DST).

5. **MCP como integración primaria**: `mem mcp` expone la memoria como herramientas nativas. Los agentes MCP pueden invocar `save_memory`, `search_memories`, etc. sin depender de instrucciones en texto.

6. **Multi-agente MCP**: `setup-mcp` configura opencode, Claude, Cursor, Windsurf, Cline y Codex desde un solo comando. Cada función de setup es idempotente. El `cwd` real del proceso que lanza `mem mcp` no es confiable entre agentes, así que la raíz del proyecto se pasa explícita vía `--root` en `args` (ver sección "El flag `--root`").

7. **Doble vía de integración**: MCP (automática, para agentes compatibles) + instrucciones en AGENTS.md (fallback, para cualquier agente).

8. **AlphaScreen en TUI**: Bubbletea usa pantalla alternativa (`tea.WithAltScreen()`) para no ensuciar el historial.

9. **Idempotencia en install**: Verifica DB existente, detecta integración ya presente, no duplica config MCP, salta agentes ya configurados.

10. **Búsqueda con ranking**: `SearchMemories()` ordena resultados priorizando matches en título sobre contenido, luego por fecha descendente.

11. **Wrap con propagación de exit code**: `mem wrap` termina con el mismo código de salida que el comando envuelto.

12. **Sesiones con UUID aleatorio**: `newID()` genera IDs hex aleatorios de 16 bytes (formato UUID-like) sin depender de `google/uuid`.

13. **Tipos de memoria con fallback**: `ValidMemoryType()` valida el tipo y cae a `learning` si no es reconocido, evitando errores de parseo.

14. **Capture estructurado**: El comando `capture` usa formato What/Why/Where/Learned para capturas completas, mientras `save` es para notas rápidas. Ambos conviven.

15. **Relaciones semánticas idempotentes**: `mem compare` detecta si ya existe una relación para el par (a,b) y la actualiza en lugar de duplicar. La búsqueda por par funciona en cualquier orden (a,b) o (b,a).

16. **Project como comando read-only**: `mem project` solo lee el sistema de archivos y la BD, nunca escribe. Ideal para verificar contexto antes de operar.

## Estructura de Directorios del Proyecto Instalado

```
proyecto/
├── .memory/                    ← DB + contexto (gitignorado)
│   ├── mem.db                  ← SQLite (WAL mode)
│   └── context.md              ← Contexto markdown generado
├── AGENTS.md                   ← Instrucciones de integración
├── CLAUDE.md                   ← Ídem para Claude Code
├── .opencode.json              ← MCP server config (opencode)
├── .mcp.json                   ← MCP server config (Claude)
├── .cursor/
│   └── mcp.json                ← MCP server config (Cursor)
├── .windsurf/
│   └── mcp_config.json         ← MCP server config (Windsurf)
├── .cline/
│   └── mcp_settings.json       ← MCP server config (Cline)
├── mem                         ← Binario (gitignorado)
├── .gitignore                  ← .memory/ y /mem ignorados
                                 (Codex no usa un archivo dentro del proyecto: registra
                                  una tabla [mcp_servers."gomemory_<proyecto>"] en el
                                  archivo global ~/.codex/config.toml)
└── ...
```

## Arquitectura Hexagonal

`gomemory` sigue una arquitectura hexagonal (puertos y adaptadores) con 4 capas:

1. **`domain/`** — Capa más interna. Define tipos, validación y errores de dominio. Zero dependencias del proyecto.
2. **`application/`** — Casos de uso y puertos (interfaces). Solo importa `domain/`. Define contratos vía `ports/*.go`.
3. **`adapters/`** — Implementaciones concretas:
   - `primary/` — Adaptadores driving (CLI, TUI, MCP, setup). Reciben input del exterior.
   - `secondary/` — Adaptadores driven (persistence SQLite). Implementan los ports.
4. **`infrastructure/`** — Composition root. `main.go` + `container.go` cablean todas las dependencias.

**Reglas de dependencia:**
- `domain` → nada del proyecto
- `application` → solo `domain`
- `adapters/primary` → `application` (via interfaces)
- `adapters/secondary` → `application` + `domain` (implementa interfaces)
- `infrastructure` → todo (cablea)

Ninguna capa externa depende directamente de adapters primarios o secundarios. Todo pasa por interfaces en `application/ports/`.

## Estructura del Repositorio Fuente

```
gomemory/
├── domain/                          # Capa más interna — 0 dependencias del proyecto
│   ├── memory.go                    #   tipos Memory, MemoryType, validación
│   ├── session.go                   #   tipos Session
│   ├── relation.go                  #   tipos Relation, RelationType, Confidence
│   └── errors.go                    #   errores de dominio (ErrNotFound, ErrValidation)
│
├── application/                     # Capa de aplicación — solo importa domain/
│   ├── ports/                       #   Puertos (interfaces) que definen contratos
│   │   ├── memory_repository.go     #     MemoryRepository interface
│   │   ├── session_repository.go    #     SessionRepository interface
│   │   ├── relation_repository.go   #     RelationRepository interface
│   │   ├── settings_repository.go   #     SettingsRepository interface
│   │   ├── project_repository.go    #     ProjectRepository interface
│   │   └── context_builder.go       #     ContextBuilder interface
│   └── usecases/                    #   Casos de uso
│       └── build_context.go         #     Genera .memory/context.md
│
├── adapters/                        # Capa de adaptadores
│   ├── primary/                     #   Adaptadores primarios (driving)
│   │   ├── cli/                     #     Comandos CLI
│   │   │   ├── cli.go              #       LaunchTUI, Usage
│   │   │   ├── deps.go             #       Deps struct (inyección de dependencias)
│   │   │   ├── dispatcher.go       #       Run(): dispatcher central
│   │   │   ├── binref.go           #       BinRef/binRefFor: referencia portable a `mem`
│   │   │   ├── cmd_init.go         #       mem init [--force]
│   │   │   ├── cmd_save.go         #       mem save -t "tit" -y tipo "cuerpo"
│   │   │   ├── cmd_capture.go      #       mem capture
│   │   │   ├── cmd_compare.go      #       mem compare
│   │   │   ├── cmd_list.go         #       mem list [-n N]
│   │   │   ├── cmd_search.go       #       mem search "consulta" [-n N]
│   │   │   ├── cmd_context.go      #       mem context [-w|--write]
│   │   │   ├── cmd_session.go      #       mem session start|end|list
│   │   │   ├── cmd_install.go      #       mem install [dir] (pack: reglas + protocolo + constitución)
│   │   │   ├── cmd_uninstall.go    #       mem uninstall [dir] [--yes]
│   │   │   ├── cmd_project.go      #       mem project
│   │   │   ├── cmd_wrap.go         #       mem wrap <comando> [args...]
│   │   │   ├── cmd_mcp.go          #       mem mcp — servidor MCP (tools + resources)
│   │   │   ├── cmd_mcp_setup.go    #       mem setup-mcp
│   │   │   ├── cmd_serve.go        #       mem serve — servidor HTTP (plugin OpenCode)
│   │   │   ├── cmd_hook.go         #       mem hook <evento> — hooks portables Claude Code
│   │   │   ├── cmd_setup.go        #       mem setup <agent> — plugin + hooks
│   │   │   ├── cmd_settings.go     #       mem settings — auto-approve MCP
│   │   │   ├── cmd_purge.go        #       mem purge
│   │   │   ├── cmd_compact.go      #       mem compact
│   │   │   └── cmd_gc.go           #       mem gc
│   │   ├── tui/                     #     TUI (Bubbletea)
│   │   │   └── tui.go
│   │   ├── mcp/                     #     Servidor MCP
│   │   │   ├── server.go           #       HTTP + MCP handlers
│   │   │   └── server_compat.go    #       Compatibilidad con tests legacy
│   │   └── setup/                   #     Setup de plugins (plugin + hooks)
│   │       ├── setup.go
│   │       ├── opencode_setup.go
│   │       └── claude_code_setup.go #       writeClaudeHooks: hooks portables en settings.json
│   └── secondary/                   #   Adaptadores secundarios (driven)
│       └── persistence/             #     Persistencia SQLite
│           ├── db.go                #       Conexión, migraciones, FindRoot
│           ├── memory.go            #       CRUD memorias
│           ├── session.go           #       CRUD sesiones
│           ├── relation.go          #       CRUD relaciones
│           ├── maintenance.go       #       Purge/Compact/GC (no expuesto vía MCP)
│           ├── settings.go          #       Config local
│           └── repositories.go     #       Wrappers ports.*Repository
│
├── infrastructure/                  # Composition root
│   ├── main.go                      #   Entry point, go:embed, dispatch
│   ├── container.go                 #   NewContainer(): wiring de dependencias
│   ├── plugin/                      #   Plugins embebidos (go:embed)
│   │   ├── opencode/
│   │   │   └── plugin.ts
│   │   └── claude-code/
│   │       ├── hooks/
│   │       ├── scripts/
│   │       └── skills/
│   └── templates/                   #   Templates embebidos (go:embed)
│       ├── agent-preamble.md        #     Reglas de trabajo + orquestación + tareas
│       └── speckit-constitution-gen.md  # Constitución copiada por `mem install`
│
├── tests/                           # Tests
│   ├── contract/
│   │   └── memory_protocol_test.go
│   └── integration/
│       └── plugin_integration_test.go
│
├── docs/
│   ├── architecture.md       # Este documento
│   ├── PLUGINS.md
│   ├── MEMORY-PROTOCOL.md
│   ├── MANUAL.md
│   ├── todo.md
│   └── lessons.md
├── scripts/                   # Instaladores universales de consola
│   ├── install.sh             #   Linux / macOS
│   └── install.ps1            #   Windows (PowerShell)
├── specs/                     # SDD specs (001..005)
├── .github/workflows/         # CI: release.yml (GoReleaser al pushear tag v*)
├── AGENTS.md
├── CLAUDE.md
├── go.mod / go.sum
└── mem                       # Binario compilado (gitignorado)
```
