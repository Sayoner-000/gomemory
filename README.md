# gomemory

[![GitHub Release](https://img.shields.io/github/v/release/Sayoner-000/gomemory?style=flat&color=blue)](https://github.com/Sayoner-000/gomemory/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Platform](https://img.shields.io/badge/macOS_%7C_Linux_%7C_Windows-supported-lightgrey)](https://github.com/Sayoner-000/gomemory/releases/latest)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-14_tools-blueviolet)](https://modelcontextprotocol.io/)
[![SQLite](https://img.shields.io/badge/SQLite-embebido-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org/)

Servidor MCP y CLI en Go que proporciona memoria persistente a agentes de código (Claude Code, Cursor, OpenCode, Windsurf, Cline, Codex). Guarda contexto, decisiones de arquitectura y bugfixes en una base de datos SQLite embebida local, permitiendo recuperar el contexto entre sesiones sin depender de archivos en el repositorio.

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

> **`mem setup-mcp` vs `mem setup`:** `setup-mcp` (arriba) solo registra las **tools MCP** — funciona para los 6 agentes soportados. Los **auto-checkpoints** y la **captura de planes aprobados** (ver más abajo) requieren además los **hooks/plugin** de `mem setup <agent>`, disponible hoy solo para `opencode` y `claude-code`. En Cursor/Windsurf/Cline/Codex tienes memoria vía MCP, pero sin esa captura automática por turno.

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
* **Privacidad por diseño:** El contenido envuelto en `<private>...</private>` se redacta y no llega a la base de datos. Como segunda capa, patrones de secretos conocidos (claves de AWS, tokens de GitHub, claves de proveedores de IA, tokens de Slack, JWT, bloques de clave privada PEM) se redactan igual aunque el usuario olvide envolverlos. El archivo `mem.db` y sus directorios se crean con permisos restringidos al propietario (`0600`/`0700`).
* **Auto-Checkpoints:** En Claude Code y OpenCode, los turnos con actividad real se registran automáticamente como `checkpoint` sin consumir tokens del agente.
* **Captura de planes aprobados:** Al aprobar un plan (Claude Code `ExitPlanMode` / modo `plan` de OpenCode), sus decisiones se guardan automáticamente como `decision` — de forma determinista, sin depender de que el modelo lo recuerde. Cada aprobación (incluidos planes revisados) se acumula, así la evolución de las decisiones no se pierde.
* **Consolidación sináptica ("siempre sinapsis"):** Cada memoria que se guarda se enlaza automáticamente con el engrama sustantivo más reciente de su sesión, tejiendo un grafo de decisiones que se re-inyecta en cada `get_context`. Determinista y transversal a todos los agentes (vive en el choke point de guardado, no en cada agente).
* **Grafo de código externo (brazo extensor, opcional):** si detecta un grafo de código ya indexado por [`codebase-memory-mcp`](https://github.com/DeusData/codebase-memory-mcp), gomemory enriquece `get_context` con un resumen estructural (módulos de facto, hotspots, lenguajes) para que la memoria "entienda" el código. **No es una dependencia dura**: si el proveedor no está, todo funciona igual. Es **no-bloqueante** (el contexto lee un snapshot cacheado al instante; el refresco corre en segundo plano) y **agnóstico al agente**. Se enciende/apaga con `mem settings --code-graph=true|false`.
* **Anotación de impacto al guardar:** si el archivo asociado a una memoria (`bugfix`/`decision`) es un hotspot conocido del grafo externo, la memoria queda anotada con el símbolo y sus llamadores directos — sin latencia extra (solo lee el snapshot ya cacheado). `mem settings --code-impact-annotation=true|false` (default activado).
* **Memoria conectada a código activo (recalculada en vivo):** a diferencia de la anotación anterior (que se congela al guardar), `get_context` cruza el archivo de cada memoria contra los hotspots vigentes del grafo externo **en cada llamada** — si el código se reindexa y cambian los hotspots, qué memorias son relevantes se actualiza solo, sin tocar lo ya guardado.
* **Sincronización bidireccional de ADR (opcional):** las memorias `architecture`/`decision` se reflejan como bloques marcados en el documento de ADR del proveedor externo, y los bloques que el proveedor tenga sin marcar se importan como memoria — sin bucles de resincronización. Consultable con `mem adr-sync status`. `mem settings --adr-sync=true|false` (default apagado).
* **Múltiples proveedores de grafo con fallback automático:** `mem settings --code-graph-providers=cmd1,cmd2` declara candidatos en orden de prioridad; gomemory usa el primero disponible sin reconfigurar al cambiar de máquina/entorno.
* **Resolución de conflictos:** `judge_memories` resuelve colisiones entre memorias obsoletas y nuevas con veredictos semánticos obligatorios.
* **Memoria portable (export/import):** `mem export` vuelca las memorias **+ sus relaciones** (sinapsis y veredictos) a un JSON UTF-8 autocontenido, apto para moverlas entre proyectos y máquinas con distinto S.O. `mem import` las trae al proyecto actual con **append + dedup por contenido** (no duplica), **preservando los timestamps** originales, remapeando el proyecto y los ids de relación, y **sin generar sinapsis espurias**. Disponible también desde la TUI (tecla `c` → Configuración).
* **Backup automático local:** cada cierre de sesión genera, en modo best-effort, un snapshot con el mismo formato que `mem export` en `<directorio de datos>/backups/<proyecto>/`, conservando por defecto los últimos 10 (ajustable con `GOMEMORY_BACKUP_KEEP`). Para llevarlo entre máquinas, apunta esa carpeta de backups a tu propia herramienta de sincronización (Dropbox, iCloud, Syncthing, un repo git privado). **No sincronices el `mem.db` crudo directamente**: es una base SQLite en modo WAL y una sincronización parcial o fuera de orden puede corromperla; el bundle JSON del backup sí es un snapshot atómico y seguro para mover.

## Herramientas MCP Expuestas

| Tool / Resource | Descripción |
| :--- | :--- |
| `save_memory` | Registra una nueva memoria estructurada. Con `topic_key` opcional: si el tópico ya existe, actualiza esa memoria en vez de duplicar. |
| `search_memories` | Búsqueda por ranking (título y contenido); devuelve extractos compactos. |
| `list_memories` | Devuelve las memorias recientes del proyecto (extractos compactos). |
| `get_memory` | Retorna el contenido íntegro de un ID específico (detalle bajo demanda). |
| `get_context` | Contexto del proyecto en markdown, acotado por presupuesto, para arrancar sesión. |
| `start_session` / `end_session` | Abre y cierra una sesión de trabajo con resumen. |
| `forget_memory` | Elimina un registro por ID (requiere aprobación manual). |
| `judge_memories` | Resuelve conflictos semánticos entre dos registros. |
| `mem://context` | Recurso: Contexto completo en markdown. |
| `mem://memory/{id}` | Recurso: Lectura directa de un ID. |

> El servidor también expone 5 herramientas adicionales para indexar y consultar
> el grafo de código fuente propio (`index_project`, `search_code`, `get_symbol`,
> `list_dependencies`, `graph_status`) — el CLI equivalente para poblar ese
> índice es `mem index [--force]` (ver tabla CLI). Además, de forma **opcional**,
> puede apoyarse en un grafo de código externo ya indexado (codebase-memory-mcp)
> como brazo extensor — ver [`docs/architecture.md`](docs/architecture.md).

## CLI

Comandos principales para la gestión manual:

| Comando | Acción |
| :--- | :--- |
| `mem` | Abre la TUI interactiva (Bubbletea). |
| `mem save [flags] <texto>` | Guarda una memoria manualmente. `-t` título opcional, `-y` tipo (`learning\|decision\|architecture\|bugfix\|pattern\|discovery\|preference`, default `learning`), `-f` archivo relacionado. **Las flags van antes del texto** (usan el paquete `flag` de Go, que deja de leer flags en el primer argumento posicional). |
| `mem search <query>` | Busca en la memoria del proyecto (título y contenido). |
| `mem list [-n N]` / `mem log` | Lista las memorias más recientes (`-n` cantidad, default 20). |
| `mem init [--force]` | Inicializa `.memory/` explícitamente. |
| `mem context [-w]` | Muestra o escribe el contexto actual. |
| `mem capture` | Formulario guiado (What/Why/Where/Learned). |
| `mem project` | Detecta el proyecto actual (clave, raíz) y muestra su información. |
| `mem index [--force]` | Indexa el código Go del propio proyecto (grafo de símbolos interno — ver "Herramientas MCP Expuestas"). |
| `mem update` | Actualiza el binario de forma idempotente. |
| `mem uninstall [--yes]` | Desinstala gomemory por completo: reverso de `mem install`. |
| `mem purge` | Vacía memorias (por tipo, antigüedad o proyecto completo) — requiere confirmación salvo `--yes`. |
| `mem gc` / `mem compact`| Limpieza de registros antiguos (>90 días) y optimización de BD. |
| `mem settings` | Configuración general: auto-approve de MCP, toggle del grafo de código externo (`--code-graph=true\|false`, `--code-graph-command`/`--code-graph-providers`), anotación de impacto (`--code-impact-annotation`) y sincronización de ADR (`--adr-sync`). |
| `mem adr-sync status` | Estado de la sincronización de ADR (solo lectura): qué memorias están vinculadas a qué bloque del documento del proveedor, y su estado (ok/pendiente/fallido/conflicto). |
| `mem export` / `mem import` | Exporta la memoria (memorias + relaciones) a un JSON portable e impórtala en otro proyecto/máquina con dedup por contenido. También desde la TUI (tecla `c`). |
| `mem hook <evento>` | Entrypoint interno de hooks de agentes (`mem hook turn-end`, etc.) — no se invoca a mano, lo llaman los plugins de Claude Code/OpenCode. |

*Ejecuta `mem help` para ver los subcomandos disponibles.*

## Arquitectura

- **Base de datos:** SQLite embebido vía `modernc.org/sqlite` (sin CGO). Vive en un store global del usuario (`~/.local/share/gomemory/projects/<clave>/mem.db`), no dentro del repositorio.
- **Transporte MCP:** `stdio` (JSON-RPC por stdin/stdout). El cliente lanza `mem mcp` como subproceso; **no se abre ningún puerto TCP**. El proceso vive lo que dura la sesión del agente.
- **Hooks portables:** cada evento del agente invoca `mem hook <evento>`, un binario que habla directo a los repositorios — sin scripts de shell ni `curl`. Idéntico en Linux, macOS y Windows.
- **Grafo de código externo enchufable:** el puerto `CodeGraphProvider` (arquitectura hexagonal) permite traer la fuerza de un grafo ya indexado por otra herramienta sin acoplarse. El hot path solo lee un snapshot cacheado; el refresco corre en un proceso detached (`mem code-refresh`) con timeout corto — nunca bloquea el guardado ni el contexto, y nunca dispara indexado. Ver [`docs/architecture.md`](docs/architecture.md).
- **Portabilidad:** Cross-compile nativo. Los timestamps usan UTC-5 por defecto.

### Huella de contexto (bajo costo de tokens)

gomemory está diseñado para **no inflar la ventana del agente**. Como los resultados de las tools MCP persisten en el contexto toda la sesión, gomemory **emite lo mínimo** desde el inicio y **señala** (nunca ejecuta) cuándo conviene compactar — de forma agnóstica al agente (Claude Code, Cursor, otros clientes MCP o el CLI):

- **`get_context` acotado por presupuesto:** el contexto de arranque se limita a un techo de caracteres, truncando lo largo con un puntero `get_memory <id>` para el detalle bajo demanda. Protocolo y conflictos nunca se recortan.
- **Revelación progresiva:** `search_memories`/`list_memories` devuelven extractos compactos; el contenido íntegro queda en `get_memory`.
- **Dedup en la fuente:** guardar una memoria equivalente (mismo tipo+título, o el mismo `topic_key`) **actualiza** la existente en vez de crear otra.
- **Recordatorio de compactación:** al cerrar el turno, si la huella emitida por gomemory supera un umbral, sugiere de forma neutral compactar el contexto.
- **Refuerzo periódico de preferencias:** las preferencias del usuario (`type=preference`) solo se reinyectaban al iniciar sesión y tras compactar; en sesiones largas que no llegan a compactar se diluían. Ahora, al superar un tercio del umbral de compactación, el hook de fin de turno reinyecta el contenido real (no un recordatorio genérico) de las preferencias más recientes, con enfriamiento de 20 min.

Ajustable en `.memory/settings.json` (valores por defecto entre paréntesis):

| Clave | Efecto | Default |
|-------|--------|---------|
| `budget` | Techo de `get_context` en caracteres (`<0` = sin límite) | `24000` (~6k tokens) |
| `compact_threshold` | Huella emitida/sesión que dispara el recordatorio (`<=0` = off) | `48000` |
| `dedup_window_days` | Ventana del dedup por identidad (`<=0` = off; el `topic_key` sigue) | `7` |

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

## Mitigación de Riesgos Operativos

Como proyecto de un mes/un autor con la memoria aún en evolución, se atendieron cuatro riesgos operativos concretos (detalle de diseño en [`specs/009-mitigacion-riesgos/`](specs/009-mitigacion-riesgos/)):

1. **Búsqueda por relevancia real** (FTS5 + `bm25()`, con fallback automático a `LIKE` si el build no soporta FTS5) en vez de solo balde título/contenido + recencia.
2. **Backup automático local** al cerrar sesión (ver "Backup automático local" arriba), para no depender de que el usuario recuerde exportar antes de perder datos.
3. **Redacción de secretos en dos capas** + permisos de archivo restringidos (ver "Privacidad por diseño" arriba).
4. **Convención de compatibilidad documentada en el código**: migraciones de esquema solo-aditivas y versionado explícito del bundle de export (`domain.ExportVersion`), para que cambios futuros no rompan datos ya persistidos.

## Más Documentación

| Documento | Descripción |
|-----------|-------------|
| [`docs/MANUAL.md`](docs/MANUAL.md) | Guía completa: multi-agente, troubleshooting, seguridad, stack, portabilidad |
| [`docs/architecture.md`](docs/architecture.md) | Arquitectura interna a fondo |
| [`docs/MEMORY-PROTOCOL.md`](docs/MEMORY-PROTOCOL.md) | Protocolo de memoria (referencia técnica) |

---
**Autor:** Jose Gomez ([@Sayoner-000](https://github.com/Sayoner-000))
**Licencia:** MIT
*Inspirado en la arquitectura base de [Engram](https://github.com/Gentleman-Programming/engram).*
