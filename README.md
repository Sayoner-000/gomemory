# gomemory

**Memoria colectiva para agentes AI.** Tu proyecto nunca olvida lo que aprendiste.

`gomemory` es un CLI + TUI + MCP server en Go que guarda decisiones, arquitectura, bugfixes y aprendizajes de tu proyecto en SQLite. Cuando vuelves a trabajar (días o semanas después), **el agente AI recuerda todo el contexto**.

> `go` + `memory` → **gomemory**

## ✨ ¿Qué hace?

- **Persiste contexto entre sesiones** de opencode, Claude, Cursor, Windsurf, Cline, Codex o cualquier agente AI
- **Organiza** lo aprendido por tipo: `architecture`, `decision`, `bugfix`, `pattern`, `learning`, `discovery`
- **TUI interactiva** con Bubbletea para navegar, buscar y guardar
- **`mem wrap`** envuelve cualquier comando y pregunta si guardar al terminar
- **MCP server** (`mem mcp`) — expone la memoria como herramientas nativas para cualquier agente MCP
- **Configuración multi-agente** (`mem setup-mcp`) — configura MCP para opencode, Claude, Cursor, Windsurf, Cline y Codex automáticamente
- **Pesa ~16MB**, sin dependencias runtime. Linux, macOS, Windows.

## 🚀 Inicio rápido

```bash
# Desde el repo de gomemory, instalar en tu proyecto:
/path/a/gomemory/mem install /ruta/a/tu/proyecto

# Entrar al proyecto y usar:
cd /ruta/a/tu/proyecto
./mem                    # Abrir TUI
```

## 📦 Guía paso a paso

### 1. Compilar (una sola vez)

Requisito: Go 1.25+ (ver `go.mod`). No necesitas CGO ni librerías de sistema —
`modernc.org/sqlite` es SQLite implementado en Go puro.

```bash
git clone <repo> && cd gomemory
go build -o mem .
```

O descarga el binario precompilado de [releases](https://github.com/tuusuario/gomemory/releases).

### 2. Instalar en tu proyecto

```bash
./mem install /ruta/a/tu/proyecto
```

Esto hace automáticamente:

```
✅ Copia el binario a /proyecto/mem
✅ Crea .memory/ con SQLite (o verifica DB existente)
✅ Actualiza .gitignore
✅ Crea/actualiza AGENTS.md y CLAUDE.md con integración
✅ Configura MCP para opencode, Claude, Cursor, Windsurf, Cline y Codex
```

### 3. Usar

```bash
cd /ruta/a/tu/proyecto

./mem                    # TUI interactiva
./mem save -t "API REST" -y decision "Usamos Fiber para rutas"
./mem search "API"
./mem context --write    # genera .memory/context.md
```

### 4. (Opcional) Wrapper automático

```bash
alias opencode='./mem wrap opencode'
```

Cada vez que termines una interacción, te preguntará si guardar algo en memoria.

## 🌍 Portabilidad

`gomemory` es un único binario Go estático (sin CGO, sin dependencias de runtime),
así que es trivial moverlo entre sistemas y máquinas.

### Compilar para otra plataforma (cross-compile)

```bash
# macOS (Apple Silicon)
GOOS=darwin  GOARCH=arm64 go build -o mem-darwin-arm64 .

# macOS (Intel)
GOOS=darwin  GOARCH=amd64 go build -o mem-darwin-amd64 .

# Linux
GOOS=linux   GOARCH=amd64 go build -o mem-linux-amd64 .

# Windows
GOOS=windows GOARCH=amd64 go build -o mem-windows-amd64.exe .
```

No se requiere ningún build tag adicional ni toolchain de C — el cross-compile
funciona "out of the box" porque todo el árbol de dependencias es Go puro.

### Mover una instalación entre máquinas/equipos

- `.memory/mem.db` es un único archivo SQLite (WAL mode) — cópialo y listo, no
  hay estado externo ni conexión a un servicio.
- El binario `mem` es autocontenido: puedes copiarlo a otra máquina con el mismo
  SO/arquitectura sin instalar nada más.
- Los timestamps se almacenan en **UTC-5 (Bogotá/Colombia, sin DST)**, así que el
  historial se mantiene consistente sin importar la zona horaria de la máquina
  donde corra el binario.
- Las configuraciones MCP (`.mcp.json`, `.cursor/mcp.json`, etc.) usan **rutas
  absolutas** vía `--root <dir>` — si mueves el proyecto a otra ruta, vuelve a
  ejecutar `./mem setup-mcp --agents all` para regenerarlas.
- Haz backup de `.memory/mem.db` de vez en cuando (está gitignorado por diseño,
  para no versionar la memoria junto al código).

## ✅ Pruebas y verificación

El proyecto no tiene (todavía) una suite de tests automatizados en Go; la
verificación se hace compilando y ejecutando un flujo de humo. Antes de dar por
buena una build:

```bash
# 1. Compila y verifica estáticamente
go build -o mem .
go vet ./...

# 2. Flujo de humo en un directorio temporal (no toca tu proyecto real)
TMPD=$(mktemp -d) && cp mem "$TMPD/mem" && cd "$TMPD"

./mem init                                          # crea .memory/
./mem save -t "prueba" -y learning "smoke test"      # guarda una memoria
./mem search "smoke"                                 # debe encontrarla
./mem context                                        # debe listarla en el contexto
./mem project                                        # debe mostrar el proyecto y 1 memoria
./mem session start && ./mem session end -s "ok"    # ciclo de sesión completo

cd - && rm -rf "$TMPD"
```

Checklist mínimo antes de publicar un cambio:

- [ ] `go build -o mem .` compila sin errores
- [ ] `go vet ./...` no reporta problemas
- [ ] El flujo de humo anterior corre sin errores en un directorio limpio
- [ ] `./mem install <dir-de-prueba>` genera `.memory/`, `.gitignore`, `AGENTS.md`/`CLAUDE.md` y configs MCP
- [ ] `./mem mcp --root <dir>` levanta el servidor sin abortar (Ctrl+C para salir)
- [ ] La TUI (`./mem`) abre, navega, busca (`/`) y guarda (`s`) sin trazas de error

## 🎨 Interfaz TUI

```bash
./mem    # ← lanza la interfaz de terminal
```

La TUI (Bubbletea) necesita una **terminal real (TTY)** que envíe el tamaño de
ventana — funciona en una terminal local, sobre SSH, o dentro de `tmux`/`zellij`,
pero no si la salida de `mem` se redirige a un pipe o archivo (en ese caso usa
los subcomandos no interactivos: `list`, `search`, `context`).

### Pantalla de lista (inicial)

| Tecla | Acción |
|-------|--------|
| `↑`/`↓` o `j`/`k` | Navegar entre memorias |
| `Enter` | Ver detalle de la memoria seleccionada |
| `s` | Abrir formulario para guardar nueva memoria |
| `a` | Alternar **auto-approve** de las tools MCP (se persiste en settings) |
| `/` | Activar búsqueda en vivo (escribe para filtrar por título/contenido/tipo) |
| `Esc` (en búsqueda) | Salir del modo búsqueda |
| `q` o `Ctrl+C` | Salir de la TUI |

### Pantalla de detalle

| Tecla | Acción |
|-------|--------|
| `Esc`, `q` o `Enter` | Volver a la lista |

### Pantalla de guardado (formulario)

| Tecla | Acción |
|-------|--------|
| `Tab` / `↓` | Siguiente campo |
| `Shift+Tab` / `↑` | Campo anterior |
| `Enter` | Confirmar campo / enviar formulario en el último campo |
| `Esc` | Cancelar y volver a la lista (sin guardar) |
| `Ctrl+C` | Salir de la TUI |

### Notas para entornos remotos / multiplexores

- En `tmux`/`zellij`, asegúrate de que el pane tenga foco para que las teclas
  lleguen a la TUI y de tener soporte de color (256 colores o true color) para
  ver bien los estilos de `lipgloss`.
- Si trabajas sobre SSH con latencia alta, la búsqueda en vivo (`/`) sigue
  funcionando porque el filtrado es local — no hay round-trip de red.
- Si la terminal no soporta `tea.WithAltScreen()` correctamente (terminales muy
  antiguas o emuladores no estándar), usa los comandos CLI equivalentes
  (`mem list`, `mem search`, `mem save`) en vez de la TUI.

## 📋 Comandos

| Comando | Descripción |
|---------|-------------|
| `mem` | Abrir TUI |
| `mem init [--force]` | Inicializar `.memory/` |
| `mem save -t "tit" -y tipo "cuerpo"` | Guardar memoria |
| `mem capture [flags]` | Guardar aprendizaje estructurado (What/Why/Where/Learned) |
| `mem compare [flags] <id1> <id2>` | Comparar memorias y persistir veredicto semántico |
| `mem compare list [-n N]` | Listar relaciones guardadas |
| `mem project` | Detectar proyecto actual (read-only) |
| `mem list [-n 20]` | Listar memorias recientes |
| `mem log [-n 20]` | Alias de list |
| `mem search "consulta"` | Buscar en la memoria |
| `mem context [-w\|--write]` | Ver contexto (read-only por defecto) |
| `mem session start` | Iniciar sesión de trabajo |
| `mem session end -s "resumen"` | Finalizar sesión |
| `mem install [dir]` | Instalar en otro proyecto |
| `mem wrap <comando> [args...]` | Ejecutar y preguntar si guardar |
| `mem mcp [--root <dir>]` | Servidor MCP para agentes AI (`--root` fija el proyecto sin depender del cwd) |
| `mem setup-mcp [--agents a,b,c]` | Configurar MCP para agentes (opencode, claude, cursor, windsurf, cline, codex, all) |
| `mem settings [--auto-approve=true\|false] [--show]` | Ver o cambiar auto-approve de las tools MCP |
| `mem tui` | Abrir TUI explícitamente |
| `mem help` | Mostrar ayuda |

### Capture estructurado

```bash
# Flags individuales:
mem capture -w "implementar auth JWT" -y "seguridad stateless" -f "middleware.go" -l "usar refresh tokens"

# Modo interactivo (guía paso a paso):
mem capture -i
```

El comando `capture` guarda aprendizajes con formato estructurado `**What** / **Why** / **Where** / **Learned**`, ideal para decisiones técnicas complejas.

### Comparar memorias

```bash
# Persistir un veredicto semántico entre dos memorias:
mem compare -r supersedes -c 0.9 -m "la nueva decisión reemplaza a la anterior" 1 2

# Tipos de relación: related, compatible, scoped, conflicts_with, supersedes, not_conflict

# Listar relaciones guardadas:
mem compare list
```

### Detectar proyecto

```bash
mem project   # Muestra nombre, raíz, ruta de BD y conteo de memorias
```

Comando read-only para verificar en qué proyecto estás trabajando.

### Tipos de memoria

| Tipo | Icono | Cuándo usarlo |
|------|-------|---------------|
| `architecture` | ▲ | Decisiones de arquitectura |
| `decision` | ◆ | Decisiones técnicas importantes |
| `pattern` | ■ | Patrones y convenciones |
| `bugfix` | ✕ | Bugs corregidos y causa raíz |
| `learning` | ● | Descubrimientos y aprendizajes |
| `discovery` | ◇ | Hallazgos sin categoría |

## 🔌 Integración MCP (multi-agente)

`mem mcp` expone la memoria del proyecto como herramientas MCP sobre stdio.
Cualquier agente compatible (opencode, Claude Desktop, Cursor, Windsurf, Cline, etc.) puede usar
`mem` como herramienta nativa — sin depender de instrucciones en texto.

### Configuración automática

```bash
# Configurar todos los agentes detectados:
./mem setup-mcp --agents all

# O agentes específicos:
./mem setup-mcp --agents opencode,claude,cursor,codex
```

Esto genera la configuración MCP para cada agente. Todas incluyen `--root <ruta-absoluta>`
en `args` para que el servidor resuelva el proyecto correcto sin depender del `cwd`
con el que el agente lance el proceso:

| Agente | Archivo de configuración |
|--------|--------------------------|
| opencode | `.opencode.json` → `mcpServers.gomemory` |
| Claude | `.mcp.json` → `mcpServers.gomemory` |
| Cursor | `.cursor/mcp.json` → `mcpServers.gomemory` |
| Windsurf | `.windsurf/mcp_config.json` → `mcpServers.gomemory` |
| Cline | `.cline/mcp_settings.json` → `mcpServers.gomemory` |
| Codex | `~/.codex/config.toml` (global) → `[mcp_servers."gomemory_<proyecto>"]` |

### Herramientas MCP

| Herramienta | Descripción |
|---|---|
| `save_memory` | Guarda aprendizaje, decisión, bugfix o patrón |
| `search_memories` | Busca en la memoria del proyecto |
| `list_memories` | Lista memorias recientes |
| `get_memory` | Obtiene una memoria por ID |
| `start_session` | Inicia sesión de trabajo |
| `end_session` | Finaliza sesión con resumen |
| `get_context` | Obtiene contexto completo del proyecto |

### Recursos MCP

- `mem://context` — Contexto markdown completo del proyecto
- `mem://memory/{id}` — Memoria específica por ID

## 🤖 Integración con agentes AI (vía instrucciones)

Si no usas MCP, añade esto al final de `AGENTS.md`:

```markdown
## Memoria Persistente (`mem`)

Este proyecto usa `mem` (Go CLI) para persistir contexto entre sesiones.

### Al inicio de cada sesión:
1. Ejecuta `./mem context` para cargar el contexto histórico
2. Si hay sesión activa, continúa; si no, ejecuta `./mem session start`

### Durante la sesión:
- Decisiones técnicas → `./mem save -t "decisión" -y decision "lo acordado"`
- Bugs → `./mem save -y bugfix "causa raíz y solución"`
- Patrones → `./mem save -t "patrón" -y pattern "descripción"`
- Aprendizajes → `./mem save -t "título" "aprendizaje"`

### Al final de cada sesión:
1. `./mem session end -s "resumen de lo realizado"`
2. `./mem context --write` para regenerar `.memory/context.md`
```

## 🏗️ Estructura del proyecto instalado

```
proyecto/
├── .memory/               # ← Base de datos (gitignorada)
│   ├── mem.db             # SQLite (WAL mode, timestamps UTC-5 Bogotá)
│   └── context.md         # Contexto generado para el agente
├── AGENTS.md              # Instrucciones de integración
├── CLAUDE.md              # Ídem para Claude Code
├── .opencode.json         # MCP server config para opencode
├── .mcp.json              # MCP server config para Claude
├── .cursor/mcp.json       # MCP server config para Cursor
├── .windsurf/mcp_config.json # MCP server config para Windsurf
├── .cline/mcp_settings.json  # MCP server config para Cline
├── mem                    # Binario (gitignorado)
├── .gitignore             # .memory/ y mem ignorados
└── ...

# Codex no usa un archivo dentro del proyecto: registra una tabla
# [mcp_servers."gomemory_<proyecto>"] en ~/.codex/config.toml (global)
```

## 📚 Documentación

- [`docs/architecture.md`](docs/architecture.md) — Arquitectura completa del proyecto

## 💡 Tips

- **`mem context --write`** regenera `.memory/context.md` — el agente lo lee rápido sin ejecutar nada
- **`mem mcp`** expone todo como herramientas MCP — los agentes lo usan sin pensar
- **`mem setup-mcp`** configura MCP para opencode, Claude, Cursor, Windsurf, Cline y Codex en un solo comando
- **`mem install`** ya configura el MCP automáticamente para todos los agentes
- Usa **`mem wrap opencode`** como alias para capturar aprendizajes manualmente
- **`mem session start/end`** agrupa aprendizajes por sesión de trabajo
- **`mem search "algo"`** busca en todo el historial con ranking por relevancia (título primero)
- La TUI tiene **búsqueda en vivo** — presiona `/` y empieza a escribir para filtrar
- Ver [🌍 Portabilidad](#-portabilidad) para mover `.memory/mem.db` entre máquinas y compilar para otra plataforma

## 🧠 Filosofía

> "Un agente sin memoria empieza de cero cada vez. Un agente con memoria construye sobre el pasado."

`gomemory` trata la memoria del proyecto como un ciudadano de primera clase. No más repetir decisiones, no más olvidar por qué se hizo algo. Cada sesión suma.

## 📝 Licencia

MIT

## 👤 Autor

Arquitecto y diseñador del proyecto: **Jose Gomez**
