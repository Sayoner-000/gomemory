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

### 1. CLI (`main.go` + `cmd_*.go`)

Dispatcher central. Enruta subcomandos a handlers según `os.Args[1]`.

| Subcomando | Archivo | Función |
|---|---|---|
| `init` | `cmd_init.go` | Crea `.memory/` y tablas SQLite |
| `save` | `cmd_save.go` | Inserta una memoria con tipo, título, contenido |
| `capture` | `cmd_capture.go` | Guarda aprendizaje estructurado con formato What/Why/Where/Learned |
| `compare` | `cmd_compare.go` | Compara dos memorias ([flags] id1 id2) y persiste veredicto semántico |
| `compare list` | `cmd_compare.go` | Lista relaciones guardadas entre memorias |
| `project` | `cmd_project.go` | Detecta el proyecto actual (read-only: nombre, raíz, BD, conteo) |
| `list` / `log` | `cmd_list.go` | Lista memorias recientes en formato tabla |
| `search` | `cmd_search.go` | Busca por LIKE en título + contenido con ranking (título primero) |
| `context` | `cmd_context.go` | Genera contexto markdown para el agente AI (read-only por defecto) |
| `session` | `cmd_session.go` | Gestiona sesiones de trabajo (start/end/list) |
| `install` | `cmd_install.go` | Copia binario + init + .gitignore + AGENTS + configura MCP para todos los agentes |
| `wrap` | `cmd_wrap.go` | Ejecuta comando y pregunta si guardar al terminar |
| `mcp` | `cmd_mcp.go` | Servidor MCP sobre stdio con 7 tools y 2 recursos. Acepta `--root <dir>` |
| `setup-mcp` / `mcp-setup` | `cmd_mcp_setup.go` | Configura MCP para opencode, Claude, Cursor, Windsurf, Cline y/o Codex |
| `settings` | `cmd_settings.go` | Ver o cambiar auto-approve de las tools MCP (`--auto-approve`, `--show`) |
| `tui` | `main.go:launchTUI()` | Abre interfaz TUI explícitamente |
| *(sin args)* | `main.go` | Abre TUI automáticamente |

Flujo:

```
os.Args → main() → switch cmd → cmdXxx()
                          ↓
                  store.FindRoot()  ← busca .memory/ hacia arriba
                          ↓
                    store.Open()    ← abre SQLite + migraciones
                          ↓
                  operación (insert/search/list/context/mcp)
                          ↓
                    db.Close()
```

### 2. TUI (`tui/tui.go`)

Interfaz de terminal con [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles).

**Estados (screens):**
- `screenList` — listado de memorias agrupadas por tipo (arquitectura, decisión, patrón, bugfix, learning, discovery), con cursor de navegación y búsqueda en vivo
- `screenDetail` — vista detalle de una memoria seleccionada con tipo, fecha, archivo relacionado y sesión
- `screenSave` — formulario multi-campo (título, tipo, contenido, archivo) con validación

**Arquitectura del modelo:**

```
model
├── db, root, project             ← contexto de la DB
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

### 3. Store (`store/db.go`, `store/memory.go`, `store/session.go`)

Capa de persistencia sobre SQLite usando [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) (SQLite puro Go, sin CGO).

```
store/
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
└── relation.go
    ├── InsertRelation()   ← INSERT de veredicto entre dos memorias
    ├── UpdateRelation()   ← UPDATE de veredicto existente (idempotente)
    ├── GetRelation()      ← SELECT por ID
    ├── GetRelationByPair()← SELECT por par (memory_id_a, memory_id_b) en cualquier orden
    └── ListRelations()    ← últimas N relaciones
```

La conexión SQLite usa WAL mode y busy timeout de 5s para mejor concurrencia.

### 4. Context Builder (`context/builder.go`)

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

### 5. Wrap (`cmd_wrap.go`)

Wrapper interactivo que envuelve cualquier comando:

1. Auto-inicia sesión si no hay activa (flag `-s true`)
2. Ejecuta el comando con stdin/stdout/stderr completo (passthrough)
3. Captura el código de salida del comando hijo
4. Al terminar pregunta interactivamente: `¿Guardar algo en memoria? (s/N)`
5. Si acepta, recolecta título/tipo/contenido y persiste
6. Exit code propagado: el wrap termina con el mismo código que el comando envuelto

### 6. MCP Server (`cmd_mcp.go`)

Servidor MCP (Model Context Protocol) sobre transporte stdio. Usa la SDK oficial [`github.com/modelcontextprotocol/go-sdk`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk).

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

### 7. Capture (`cmd_capture.go`)

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

### 8. Compare (`cmd_compare.go`)

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

### 9. Project (`cmd_project.go`)

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

### 10. MCP Setup Multi-Agente (`cmd_mcp_setup.go`)

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

`mem mcp` resuelve el proyecto vía `store.FindRoot()`, que sube directorios desde el `cwd` del proceso buscando `.memory/`. Cuando un agente (Claude, Cursor, etc.) lanza el servidor MCP, **no garantiza qué `cwd` usará** para el subproceso — puede ser el directorio desde donde se abrió el editor, no el proyecto instalado. Por eso, cada configuración generada por `setupX`/`setupCodex` incluye `args: ["mcp", "--root", absRoot]`: el servidor recibe la raíz del proyecto explícitamente, sin depender del `cwd` real. Si `--root` no se pasa (por ejemplo, al ejecutar `./mem mcp` manualmente desde dentro del proyecto), se mantiene el comportamiento anterior basado en `FindRoot()`.

## Flujo de Instalación (`cmd_install.go`)

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
  ├─ 4. AGENTS.md, CLAUDE.md, .cursorrules, .windsurfrules
  │    ├─ No existen → copia plantilla base del proyecto origen
  │    ├─ Existen sin integración → agrega bloque ## Memoria Persistente
  │    └─ Existen con integración → saltar (idempotente)
  │
  └─ 5. MCP server config para TODOS los agentes
       ├─ opencode → .opencode.json
       ├─ claude → .mcp.json
       ├─ cursor → .cursor/mcp.json
       ├─ windsurf → .windsurf/mcp_config.json
       ├─ cline → .cline/mcp_settings.json
       └─ codex → ~/.codex/config.toml (tabla por proyecto)
```

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

## Tipos Go (`types/types.go`)

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

## Estructura del Repositorio Fuente

```
gomemory/
├── main.go                   # Dispatcher CLI + launchTUI() + usage()
├── cmd_init.go               # mem init [--force]
├── cmd_save.go               # mem save -t "tit" -y tipo "cuerpo"
├── cmd_list.go               # mem list [-n N]
├── cmd_search.go             # mem search "consulta" [-n N]
├── cmd_context.go            # mem context [-w|--write]
├── cmd_session.go            # mem session start|end|list
├── cmd_capture.go            # mem capture — aprendizaje estructurado What/Why/Where/Learned
├── cmd_compare.go            # mem compare — veredictos semánticos entre memorias
├── cmd_project.go            # mem project — detecta proyecto actual (read-only)
├── cmd_install.go            # mem install [dir] + MCP auto-config para 5 agentes
├── cmd_wrap.go               # mem wrap <comando> [args...]
├── cmd_mcp.go                # mem mcp — servidor MCP (tools + resources)
├── cmd_mcp_setup.go          # mem setup-mcp — configura MCP multi-agente
├── tui/
│   └── tui.go                # Bubbletea TUI (list/detail/save screens)
├── store/
│   ├── db.go                 # SQLite connection, FindRoot, migrations, Now()
│   ├── memory.go             # CRUD memorias con search ranking
│   ├── session.go            # CRUD sesiones con UUID
│   └── relation.go           # CRUD relaciones entre memorias
├── context/
│   └── builder.go            # Genera .memory/context.md agrupado por tipo
├── types/
│   └── types.go              # Memory, Session, MemoryType, ValidMemoryType()
├── docs/
│   ├── architecture.md       # Este documento
│   ├── todo.md               # Plan de tareas
│   └── lessons.md            # Lecciones aprendidas
├── README.md                 # Guía de inicio rápido
├── AGENTS.md                 # Instrucciones para el agente AI
├── CLAUDE.md                 # Instrucciones para Claude Code
├── go.mod / go.sum           # Dependencias Go
└── mem                       # Binario compilado (gitignorado)
```
