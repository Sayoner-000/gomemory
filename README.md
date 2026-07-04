# gomemory

[![GitHub Release](https://img.shields.io/github/v/release/Sayoner-000/gomemory?style=flat&color=blue)](https://github.com/Sayoner-000/gomemory/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Platform](https://img.shields.io/badge/macOS_%7C_Linux_%7C_Windows-supported-lightgrey)](https://github.com/Sayoner-000/gomemory/releases/latest)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-9_tools-blueviolet)](https://modelcontextprotocol.io/)
[![SQLite](https://img.shields.io/badge/SQLite-embebido-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org/)

Servidor MCP y CLI en Go que proporciona memoria persistente a agentes de código (Claude Code, Cursor, OpenCode, Cline). Guarda contexto, decisiones de arquitectura y bugfixes en una base de datos SQLite embebida local, permitiendo recuperar el contexto entre sesiones sin depender de archivos en el repositorio.

## Inicio Rápido

Instala el binario de forma global:

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
```

## Configuración del Agente

### Registro Global (Claude Code / Codex / OpenCode)
Ejecuta esto una sola vez en cualquier directorio. Todos los proyectos nuevos usarán gomemory automáticamente:

```bash
mem setup-mcp --scope global --agents claude,codex,opencode
```

### Registro por Proyecto (Cursor / Windsurf / Cline)
Ejecuta esto en la raíz del proyecto específico:

```bash
cd /ruta/a/tu/proyecto
mem setup-mcp --scope project --agents cursor,windsurf,cline --target .
```

*Nota: La base de datos `mem.db` se guarda en `~/.local/share/gomemory/` o `%LOCALAPPDATA%\gomemory`. No ensucia tu repositorio con archivos adicionales.*

## Uso y Características Principales

Una vez configurado, el agente interactúa con la memoria automáticamente vía MCP. Puedes gestionarla manualmente mediante el CLI:

```bash
# Interfaz visual de terminal (TUI)
mem

# Guardar una decisión manualmente
mem save -t "API REST" -y decision "Usamos Fiber para el enrutamiento"

# Buscar en el historial
mem search "API"
```

* **8 Tipos de memoria:** `architecture`, `decision`, `bugfix`, `pattern`, `learning`, `discovery`, `preference`, `checkpoint`.
* **Privacidad por diseño:** El contenido envuelto en `<private>...</private>` se redacta y no llega a la base de datos.
* **Auto-Checkpoints:** En Claude Code y OpenCode, los turnos con actividad real se registran automáticamente como `checkpoint` sin consumir tokens del agente.
* **Resolución de conflictos:** `judge_memories` resuelve colisiones entre memorias obsoletas y nuevas con veredictos semánticos obligatorios.

## Herramientas MCP Expuestas

| Tool / Resource | Descripción |
| :--- | :--- |
| `save_memory` | Registra una nueva memoria estructurada. |
| `search_memories` | Búsqueda por ranking (título y contenido). |
| `list_memories` | Devuelve las memorias recientes del proyecto. |
| `get_memory` | Retorna el contenido de un ID específico. |
| `get_context` | Contexto completo del proyecto en markdown, para arrancar sesión. |
| `start_session` / `end_session` | Abre y cierra una sesión de trabajo con resumen. |
| `forget_memory` | Elimina un registro por ID (requiere aprobación manual). |
| `judge_memories` | Resuelve conflictos semánticos entre dos registros. |
| `mem://context` | Recurso: Contexto completo en markdown. |
| `mem://memory/{id}` | Recurso: Lectura directa de un ID. |

> El servidor también expone 5 herramientas adicionales para indexar y consultar
> el grafo de código fuente (`index_project`, `search_code`, `get_symbol`,
> `list_dependencies`, `graph_status`) — ver [`docs/architecture.md`](docs/architecture.md).

## CLI

Comandos principales para la gestión manual:

| Comando | Acción |
| :--- | :--- |
| `mem` | Abre la TUI interactiva (Bubbletea). |
| `mem init [--force]` | Inicializa `.memory/` explícitamente. |
| `mem context [-w]` | Muestra o escribe el contexto actual. |
| `mem capture` | Formulario guiado (What/Why/Where/Learned). |
| `mem update` | Actualiza el binario de forma idempotente. |
| `mem gc` / `mem compact`| Limpieza de registros antiguos (>90 días) y optimización de BD. |
| `mem settings` | Configuración general y auto-approve de MCP. |

*Ejecuta `mem help` para ver los subcomandos disponibles.*

## Arquitectura

- **Base de datos:** SQLite embebido vía `modernc.org/sqlite` (sin CGO).
- **Servidor HTTP:** Auto-iniciado en background (`127.0.0.1:9735`) para mantener sesiones activas entre reinicios del agente.
- **Portabilidad:** Cross-compile nativo. Los timestamps usan UTC-5 por defecto.

```text
gomemory/
├── domain/         # Modelos (Memory, Session, Relation, Code, Redact)
├── application/    # Casos de uso (BuildContext, IndexProject, GoParse)
├── adapters/       # CLI, MCP Server, TUI y persistencia SQLite
├── infrastructure/ # Orquestación, plugins de agentes y main
└── scripts/        # Instaladores shell/powershell
```

### Compilación Manual
Requiere Go 1.25+ instalado:
```bash
git clone https://github.com/Sayoner-000/gomemory.git
cd gomemory
go build -o mem ./infrastructure/
./mem install .
```

## Más Documentación

| Documento | Descripción |
|-----------|-------------|
| [`docs/MANUAL.md`](docs/MANUAL.md) | Guía completa: multi-agente, troubleshooting, seguridad, stack, portabilidad |
| [`docs/architecture.md`](docs/architecture.md) | Arquitectura interna a fondo |
| [`docs/PLUGINS.md`](docs/PLUGINS.md) | Sistema de plugins multi-agente |
| [`docs/MEMORY-PROTOCOL.md`](docs/MEMORY-PROTOCOL.md) | Protocolo de memoria (referencia técnica) |

---
**Autor:** Jose Gomez ([@Sayoner-000](https://github.com/Sayoner-000))
**Licencia:** MIT
*Inspirado en la arquitectura base de [Engram](https://github.com/Gentleman-Programming/engram).*
