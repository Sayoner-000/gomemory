## Orquestación del Flujo de Trabajo

### 1. Modo Plan por Defecto
- Entra en modo plan para CUALQUIER tarea no trivial (3+ pasos o decisiones arquitectónicas)
- Si algo se desvía, DETENTE y replantea inmediatamente — no sigas avanzando a la fuerza
- Usa el modo plan también para pasos de verificación, no solo para construir
- Escribe especificaciones detalladas desde el inicio para reducir ambigüedad

### 2. Estrategia de Subagentes
- Usa subagentes libremente para mantener limpio el contexto principal
- Delega investigación, exploración y análisis paralelo a subagentes
- Para problemas complejos, utiliza más cómputo mediante subagentes
- Un enfoque por subagente para una ejecución enfocada

### 3. Bucle de Mejora Continua
- Después de CUALQUIER corrección del usuario: actualiza docs/lessons.md con el patrón
- Escribe reglas para ti mismo que prevengan el mismo error
- Itera agresivamente sobre estas lecciones hasta reducir la tasa de errores
- Revisa las lecciones al inicio de la sesión para el proyecto relevante

### 4. Verificación Antes de Dar por Terminado
- Nunca marques una tarea como completa sin demostrar que funciona
- Compara (diff) el comportamiento entre el estado original y tus cambios cuando aplique
- Pregúntate: "¿Un ingeniero senior aprobaría esto?"
- Ejecuta pruebas, revisa logs, demuestra la corrección

### 5. Exige Elegancia (Balanceada)
- Para cambios no triviales: pausa y pregunta "¿hay una forma más elegante?"
- Si una solución se siente improvisada: "Sabiendo todo lo que sé ahora, implementa la solución elegante"
- Omite esto para arreglos simples y evidentes — no sobre-ingenierizar
- Cuestiona tu propio trabajo antes de presentarlo

### 6. Corrección Autónoma de Bugs
- Cuando recibas un reporte de bug: arréglalo. No pidas guía paso a paso
- Señala logs, errores, pruebas fallidas — luego resuélvelos
- Cero necesidad de cambiar el contexto del usuario
- Arregla fallos en CI sin que te indiquen cómo hacerlo

## Gestión de Tareas

1. *Planifica Primero*: Escribe el plan en docs/todo.md con ítems verificables
2. *Verifica el Plan*: Confirma antes de comenzar la implementación
3. *Haz Seguimiento*: Marca los ítems como completados a medida que avanzas
4. *Explica los Cambios*: Resume a alto nivel en cada paso
5. *Documenta Resultados*: Añade una sección de revisión en docs/todo.md
6. *Captura Lecciones*: Actualiza docs/lessons.md después de correcciones

## Arquitectura del Proyecto

### Descripción

`gomemory` es un CLI + TUI + MCP server en Go que persiste contexto de agentes AI por proyecto.
Usa SQLite embebido (`modernc.org/sqlite`, sin CGO) y se integra con opencode/Claude/Cursor/Windsurf/Cline
mediante MCP (Model Context Protocol) o instrucciones en `AGENTS.md`.

### Estructura de archivos

```
gomemory/
├── main.go                   # Dispatcher CLI + launchTUI() + usage()
├── cmd_init.go               # mem init [--force]
├── cmd_save.go               # mem save -t "tit" -y tipo "cuerpo"
├── cmd_capture.go            # mem capture — aprendizaje estructurado What/Why/Where/Learned
├── cmd_compare.go            # mem compare — veredictos semánticos entre memorias
├── cmd_project.go            # mem project — detecta proyecto actual (read-only)
├── cmd_list.go               # mem list [-n N]
├── cmd_search.go             # mem search "consulta" [-n N]
├── cmd_context.go            # mem context [-w|--write]
├── cmd_session.go            # mem session start|end|list
├── cmd_install.go            # mem install [dir] + MCP auto-config (5 agentes)
├── cmd_wrap.go               # mem wrap <comando> [args...]
├── cmd_mcp.go                # mem mcp — servidor MCP (7 tools + 2 resources)
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
│   ├── architecture.md       # Documentación de arquitectura
│   ├── todo.md               # Plan de tareas
│   └── lessons.md            # Lecciones aprendidas
├── README.md                 # Guía de inicio rápido
├── AGENTS.md                 # Instrucciones para el agente AI
├── CLAUDE.md                 # Instrucciones para Claude Code
├── go.mod / go.sum           # Dependencias Go
└── mem                       # Binario compilado (gitignorado)
```

### Comandos

| Comando | Descripción |
|---------|-------------|
| `mem` | Abrir TUI interactiva (Bubbletea) |
| `mem init [--force]` | Inicializar `.memory/` en el proyecto |
| `mem save -t "título" -y tipo "cuerpo"` | Guardar aprendizaje |
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
| `mem setup-mcp` | Configurar MCP para opencode, claude, cursor, windsurf, cline |
| `mem tui` | Abrir TUI explícitamente |
| `mem help` | Mostrar ayuda |

### Stack tecnológico

- **Lenguaje**: Go 1.25+
- **Base de datos**: SQLite embebido via `modernc.org/sqlite` (sin CGO, ~11MB binario)
- **TUI**: `charmbracelet/bubbletea` + `bubbles` + `lipgloss`
- **MCP SDK**: `github.com/modelcontextprotocol/go-sdk`
- **Timestamps**: UTC-5 (Bogotá/Colombia, sin DST)
- **Sin dependencias runtime**: binario autocontenido, portable Linux/macOS/Windows

### Flujo de datos

```
Usuario → CLI/TUI → store.Open() → SQLite (WAL mode)
                         ↓
               context.Builder → .memory/context.md → agente AI lo lee al iniciar
                         ↓
               cmd_mcp.go → MCP stdio server → tools/resources para agentes MCP
```

<!-- gomemory-protocol-v2 -->
## Memoria Persistente (`mem`) — Protocolo Activo

Este proyecto tiene el servidor MCP `gomemory` conectado. Este protocolo es OBLIGATORIO
y SIEMPRE ACTIVO — no esperes a que el usuario lo pida explícitamente.

### Herramientas MCP disponibles
- `save_memory(title, type, content, filepath?)` — guarda una memoria
- `search_memories(query, limit?)` — busca en memorias del proyecto
- `list_memories(limit?)` — lista memorias recientes
- `get_memory(id)` — obtiene una memoria específica
- `start_session()` / `end_session(summary?)` — gestiona la sesión de trabajo
- `get_context()` — contexto completo del proyecto en markdown

Si el MCP no está disponible en el agente actual, usa el CLI equivalente:
`./mem save -t "título" -y tipo "contenido"`, `./mem search "tema"`, `./mem context`, `./mem session start|end`.

### GUARDAR PROACTIVAMENTE — no esperes a que el usuario lo pida
Llama a `save_memory` (o `./mem save`) INMEDIATAMENTE después de:
- Una decisión técnica o de arquitectura
- Un bug corregido (incluye causa raíz)
- Un patrón o convención establecida
- Un descubrimiento no obvio sobre el código
- El usuario confirma o rechaza un enfoque propuesto

Autochequeo después de CADA tarea: "¿Tomé una decisión, corregí un bug, descubrí algo
o establecí una convención? Si sí → `save_memory` AHORA."

### Al inicio de cada sesión:
1. Llama `get_context()` (o `./mem context`) para cargar el contexto histórico
2. Si no hay sesión activa, llama `start_session()` (o `./mem session start`)

### Al cerrar la sesión (antes de decir "listo"):
Llama `end_session(summary)` (o `./mem session end -s "..."`) con un resumen de lo realizado.

### Consultar memoria:
- `search_memories(query)` (o `./mem search "tema"`) cuando el usuario pregunte por trabajo previo
- `./mem` abre la TUI interactiva
