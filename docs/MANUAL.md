# Manual de Usuario — Plugin System

Guía paso a paso para instalar y usar el sistema de plugins de memoria
automática con OpenCode y Claude Code.

## Índice

1. [Instalación Rápida](#1-instalación-rápida)
2. [Plugin para OpenCode](#2-plugin-para-opencode)
3. [Plugin para Claude Code](#3-plugin-para-claude-code)
4. [Servidor HTTP](#4-servidor-http)
5. [Verificación](#5-verificación)
6. [Solución de Problemas](#6-solución-de-problemas)
7. [Memory Protocol](#7-memory-protocol)
8. [Mantenimiento de Memoria](#8-mantenimiento-de-memoria)
9. [Registro Multi-Agente (detalle completo)](#9-registro-multi-agente-detalle-completo)
10. [Seguridad](#10-seguridad)
11. [Stack Técnico](#11-stack-técnico)
12. [Portabilidad](#12-portabilidad)

---

## 1. Instalación Rápida

### Opción A — Instalador universal de consola (recomendado)

Deja el binario `mem` en el PATH, sin compilar. Linux, macOS y Windows.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.sh | bash
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/Sayoner-000/gomemory/master/scripts/install.ps1 | iex
```

Luego, en tu proyecto:

```bash
cd tu-proyecto
mem install .        # Memoria + MCP (6 agentes) + pack de trabajo + constitución
mem setup claude-code   # (opcional) registra los hooks de Claude Code
mem --help
```

`mem install .` configura el **MCP** de los 6 agentes y escribe el "pack de
trabajo" en `AGENTS.md`/`CLAUDE.md` (reglas de trabajo + orquestación + Memory
Protocol) más la constitución (`speckit-constitution-gen.md`). Los **hooks** de
cada agente se registran aparte con `mem setup claude-code` / `mem setup opencode`.

### Opción B — Desde el fuente

```bash
go build -o mem ./infrastructure/
./mem setup opencode      # o claude-code
./mem --help
```

### Hooks portables

Los hooks de Claude Code son subcomandos del binario (`mem hook session-start`,
`session-end`, `post-compact`, `user-prompt-submit`, `subagent-stop`): no usan
`bash`/`curl` ni servidor HTTP y funcionan igual en Windows. Se configuran solos
con `mem install .` o `mem setup claude-code`.

### Prerrequisitos

- Para la Opción A: ninguno (binario autocontenido).
- Para la Opción B: Go 1.25+.
- OpenCode 0.70+ (para plugin OpenCode), Claude Code (para hooks/plugin).
- No se necesita CGO.

---

## 2. Plugin para OpenCode

### Instalación

```bash
./mem setup opencode
```

Esto:

1. Copia `infrastructure/plugin/opencode/plugin.ts` a `~/.config/opencode/plugins/gomemory.ts` (archivo suelto — OpenCode auto-descubre plugins como archivos en `plugins/`, no en subcarpetas)
2. Crea/actualiza el `opencode.json` **del proyecto actual** con la entrada `mcp.gomemory` (usa `mem setup-mcp --scope global --agents opencode` en vez de esto para registrar el MCP una sola vez, en `~/.config/opencode/opencode.json`, para todos los proyectos)
3. El plugin se activa automáticamente al iniciar OpenCode — OpenCode lo descubre solo, sin que ningún `opencode.json` lo referencie explícitamente

### Verificación

```bash
ls ~/.config/opencode/plugins/
# Debería ver: gomemory.ts

cat opencode.json   # o ~/.config/opencode/opencode.json si usaste --scope global
# Buscar la clave "mcp" conteniendo "gomemory"
```

### Qué Hace el Plugin

- **Hooks directos**: invoca `mem hook <evento>` (sin servidor HTTP ni puerto)
- **Session lifecycle**: Crea sesión al iniciar, cierra al terminar
- **Memory Protocol**: Inyecta reglas de memoria en el system prompt
- **Context injection**: Provee contexto de sesiones previas al arrancar
- **Compaction recovery**: Recupera estado después de compactación
- **Context enrichment**: ToolSearch instruction en el primer prompt

---

## 3. Plugin para Claude Code

### Instalación

```bash
./mem setup claude-code
```

Esto:

1. Copia el skill de memoria a `.claude/plugins/gomemory/skills/memory/SKILL.md`.
2. Escribe `.mcp.json` en la raíz con el server MCP `gomemory` (referencia
   **portable**: `command: "mem"` por PATH, no una ruta absoluta de máquina).
3. Registra **hooks portables** en `.claude/settings.json`: cada evento de Claude
   Code apunta a un subcomando del binario (`mem hook <evento>`), no a scripts
   `.sh`. No dependen de `bash`/`curl` ni del servidor HTTP y corren igual en
   Windows.

### Para qué sirve cada hook

| Evento | Subcomando | Función |
|--------|-----------|---------|
| `SessionStart` (`startup\|resume\|clear`) | `session-start` | Abre sesión si no hay activa **e inyecta el contexto de sesiones previas**. El agente arranca recordando el proyecto. |
| `SessionStart` (`compact`) | `post-compact` | **Después** de compactar, re-inyecta recuperación + contexto y reactiva las tools MCP diferidas. Sobrevive a la compactación (reemplaza al `pre-compact` legado). En OpenCode lo cubre `experimental.session.compacting`. |
| `SessionEnd` | `session-end` | Cierra la sesión activa como **red de seguridad** (acepta `summary` por stdin), aunque el modelo no llame `end_session`. |
| `UserPromptSubmit` | `user-prompt-submit` | En el **primer** prompt activa las tools MCP de memoria e inyecta el recordatorio del protocolo; luego es pasivo. En **cada** prompt persiste el texto del turno como provenance (`origin_prompt`) de lo que se guarde. En OpenCode el equivalente es `chat.message` → `mem hook prompt`. |
| `SubagentStop` | `subagent-stop` | Al terminar un subagente (tool `Task`), registra un **checkpoint** con los archivos y comandos que tocó (actividad que vive solo en el transcript del subagente). |
| `Stop` | `turn-end` | Al terminar cada turno, registra automáticamente (sin gastar tokens del agente) qué archivos se editaron y qué comandos corrieron, como memoria tipo `checkpoint`. Turnos de puro chat no generan nada. En OpenCode el equivalente es el evento `session.idle`. |
| `PostToolUse` (`ExitPlanMode`) | `plan-approved` | Al **aprobar un plan**, guarda el plan como memoria `decision` de forma determinista, sin depender de que el modelo lo recuerde. Cubre el hueco de `turn-end` (un turno de plan mode no toca archivos ni corre comandos). Cada aprobación (incluidos planes revisados) se acumula. En OpenCode el plugin lo activa al detectar un turno en modo `plan`. |

> Regla de oro: un hook nunca aborta el arranque del agente — ante error sale con
> código 0. Los hooks son lo que hace que la memoria "tome todo bien": sin ellos,
> las tools MCP existen pero nadie abre/cierra sesiones ni recupera contexto solo.

### Verificación

```bash
# Skill instalado
ls -la .claude/plugins/gomemory/skills/memory/

# Hooks portables registrados (deben referenciar `mem hook ...`)
cat .claude/settings.json

# MCP configurado (command: "mem")
cat .mcp.json
```

---

## 4. Servidor HTTP (retirado en v1.18.0)

El servidor HTTP legado (`mem serve` en `127.0.0.1:9735`) fue **retirado**. Tanto
el plugin de OpenCode como los hooks de Claude Code hablan directo a los
repositorios vía `mem hook <evento>` — sin shell, sin `curl`, sin puerto TCP,
idéntico en Linux, macOS y Windows. El MCP va por `stdio` (`mem mcp`), sin abrir
ningún puerto.

---

## 5. Verificación

### Test Rápido

```bash
# 1. Crear/gestionar sesión por CLI
./mem session start

# 2. Guardar y buscar una memoria
./mem save -t "Prueba" -y learning "Verificación manual del manual"
./mem search "Prueba"

# 3. Obtener el contexto del proyecto (markdown)
./mem context

# 4. Cerrar sesión con resumen
./mem session end -s "Prueba manual completada"
```

### Tests Automatizados

```bash
go test ./... -v
```

### Verificar Compilación

```bash
go build -o mem ./infrastructure/ && echo "OK"
go vet ./... && echo "OK"
```

---

## 6. Solución de Problemas

### Error: "plugin directory not found"

```bash
# Asegúrate de compilar desde la raíz del proyecto
ls infrastructure/plugin/opencode/plugin.ts   # debe existir
ls infrastructure/plugin/claude-code/         # debe existir
```

### Error: "MCP connection refused"

```bash
# El MCP va por stdio (sin puerto). Verifica que `mem` esté en el PATH
# y que la config del agente apunte a `mem mcp`.
which mem
mem mcp --help
```

### El Plugin OpenCode No se Activa

```bash
# Verificar que el plugin está instalado (archivo suelto, no subcarpeta)
ls ~/.config/opencode/plugins/gomemory.ts

# Verificar la config resuelta (mergea global + proyecto)
opencode debug config

# Reinstalar si es necesario
./mem setup opencode
```

### El Plugin Claude Code No se Activa

```bash
# Verificar archivos instalados
ls .claude/plugins/gomemory/

# Verificar que .mcp.json apunta al binario correcto en esta máquina
cat .mcp.json

# Reinstalar
./mem setup claude-code
```

### Otros problemas comunes

| Problema | Solución |
|----------|----------|
| `/mcp` no muestra el servidor | Verificar que `.mcp.json` tenga la ruta absoluta a `mem`. Reiniciar el agente. |
| `mem install` falla | Verificar que `mem` esté en el PATH: `which mem` (Linux/macOS) o `where mem` (Windows). |
| Memoria no se persiste entre sesiones | Verificar que `.memory/` exista y tenga permisos de escritura. Ejecutar `mem init --force`. |
| Binario no encontrado después de instalar | Agregar al PATH: `export PATH="$HOME/.local/bin:$PATH"` (Linux/macOS). |
| `mem update` en Windows | El binario en ejecución no se sobrescribe. Cerrar el proceso y ejecutar el comando que `mem update` sugiere. |
| Contexto muy grande | Ejecutar `mem gc` para limpiar memorias antiguas (retención de 90 días por defecto). |
| Base de datos corrupta | Ejecutar `mem compact` para recuperar espacio. Si persiste, borrar `.memory/mem.db` y re-indexar. |

---

## 7. Memory Protocol

El Memory Protocol es un conjunto de reglas que le dicen al agente cuándo
guardar, buscar y cerrar sesión de memoria.

- **Con plugin (OpenCode, Claude Code)**: se inyecta automáticamente en el
  system prompt — no requiere configuración adicional.
- **Sin plugin (Cursor, Windsurf, Cline, Codex)**: `mem setup-mcp` solo
  registra el servidor MCP, no inyecta el protocolo por sí solo. La fuente de
  verdad es el bloque que `mem install` agrega a `AGENTS.md`/`CLAUDE.md` (y
  `.cursorrules`/`.windsurfrules` si existen) — ese bloque es el que le indica
  al agente cuándo usar `save_memory`, `search_memories`, etc. Sin ese
  archivo leído por el agente, las tools MCP existen pero nadie las invoca
  proactivamente.

### Save Triggers

El agente guarda memoria automáticamente después de:
- Decisiones de arquitectura
- Corrección de bugs (incluye causa raíz)
- Convenciones o patrones establecidos
- Descubrimientos no obvios sobre el código
- Preferencias o restricciones del usuario

### Search Triggers

El agente busca memoria:
- **Reactivo**: cuando preguntas "recuerdas...?" o similar
- **Proactivo**: al iniciar tareas que podrían solaparse con trabajo previo

### Progressive Disclosure (3 Capas)

1. `search_memories()` — resúmenes compactos (~100 tokens)
2. `get_memory()` — contenido completo solo cuando es necesario
3. Nunca volcar toda la memoria

### Session Close

Al terminar, el agente registra: Goal, Discoveries, Accomplished, Next Steps,
Relevant Files.

### Compaction Recovery

Después de compactación, el agente persiste resumen y recupera estado antes
de continuar.

---

## 8. Mantenimiento de Memoria

Cuando el almacén de memoria (`.memory/mem.db`) crece demasiado, gomemory ofrece
cuatro acciones de mantenimiento — disponibles por CLI y, salvo desinstalar,
también desde la TUI (tecla `m`). Ninguna se expone vía MCP: son operaciones
destructivas que exigen confirmación humana explícita.

### Purgar memorias

```bash
./mem purge                                  # Purga el proyecto actual (pide confirmación)
./mem purge --type bugfix                    # Solo memorias de un tipo
./mem purge --older-than-days 90             # Solo memorias más viejas que N días
./mem purge --all --yes                      # TODOS los proyectos, sin prompt (scripts)
```

Por defecto el alcance es el proyecto actual (FR-003); `--all` requiere pasarse
explícitamente para afectar todos los proyectos del archivo. Al borrar una
memoria también se limpian las relaciones (`mem compare`) que la referencian.

### Compactar el almacenamiento

```bash
./mem compact
```

Ejecuta `VACUUM` para recuperar el espacio en disco liberado por borrados
previos. Nunca elimina memorias — reporta el tamaño antes/después.

### Garbage collection a demanda

```bash
./mem gc                                     # 90 días de retención por defecto
./mem gc --older-than-days 180 --all --yes
```

Limpieza por antigüedad, reutilizando la misma lógica de `purge`. Solo se
ejecuta cuando el usuario lo pide explícitamente — nunca en segundo plano.

### Desinstalación completa

```bash
./mem uninstall                              # reverso exacto de `mem install`
./mem uninstall ~/proyectos/mi-app --yes
```

Además de los datos (`.memory/`), remueve el binario `mem`, los hooks, las
entradas en `AGENTS.md`/`CLAUDE.md` y el registro MCP en `.mcp.json` y
similares. Reporta qué componentes no encontró sin fallar. El archivo global
`~/.codex/config.toml` no se toca automáticamente — se informa al usuario para
que lo edite manualmente si instaló el agente Codex.

Ver también [contracts/cli-tui-contracts.md](../specs/003-memory-maintenance/contracts/cli-tui-contracts.md)
para el detalle completo de flags y comportamiento.

## Referencia Rápida

```bash
# Instalación
./mem setup opencode              # Plugin para OpenCode
./mem setup claude-code           # Plugin para Claude Code

# Configuración
./mem settings --show                 # Ver settings (auto-approve, grafo externo)
./mem settings --code-graph=false     # Apagar el grafo de código externo

# Mantenimiento de memoria
./mem purge --older-than-days 90  # Purgar memorias viejas del proyecto actual
./mem compact                     # Recuperar espacio en disco
./mem gc                          # Garbage collection a demanda (90 días default)
./mem uninstall --yes             # Desinstalación completa (reverso de install)

# Verificación
./mem --help                      # Listar comandos
./mem context                     # Contexto del proyecto (markdown)
go test ./...                     # Tests
```

Para más detalles técnicos, ver:
- `docs/PLUGINS.md` — Arquitectura del sistema de plugins
- `docs/MEMORY-PROTOCOL.md` — Referencia técnica del protocolo

---

## 9. Registro Multi-Agente (detalle completo)

Dos formas de configurar agentes, según si soportan registro MCP a nivel de usuario:

**Global (una vez por máquina)** — `mem setup-mcp --scope global --agents claude,codex,opencode`:

| Agente | Config MCP | Notas |
|--------|-----------|-------|
| **Claude Code** | `~/.claude.json` → `mcpServers.gomemory` (scope `user`) | Registrado vía `claude mcp add`, con detección de colisión de nombre |
| **Codex** | `~/.codex/config.toml` → `[mcp_servers.gomemory]` | Tabla única, sin `cwd` por proyecto |
| **OpenCode** | `~/.config/opencode/opencode.json` → `mcp.gomemory` | OpenCode mergea la config de usuario con la del proyecto activo (confirmado con `opencode debug config`); el plugin se instala en el mismo paso |

**Por proyecto (`mem install`, o `mem setup-mcp --scope project`)** — necesario para Cursor/Windsurf/Cline (sin registro global conocido), opcional para Claude/Codex/OpenCode:

| Agente | Config MCP | `AGENTS.md`/`CLAUDE.md` (refuerzo opcional) | Hooks |
|--------|-----------|---------------------------------------------|-------|
| **Claude Code** | `.mcp.json` | Sí, si se instaló con `mem install` | SessionStart, SessionEnd, PreCompact, UserPromptSubmit, Stop |
| **OpenCode** | `opencode.json` | Sí, si se instaló con `mem install` | `plugin/opencode/plugin.ts` (auto-inicio, ya es global) |
| **Cursor** | `.cursor/mcp.json` | — | — |
| **Windsurf** | `.windsurf/mcp.json` | — | — |
| **Cline** | `.cline/mcp.json` | — | — |
| **Codex** | `~/.codex/config.toml` (tabla por proyecto, `gomemory_<proyecto>`) | Sí, si se instaló con `mem install` | SessionStart |

> Los hooks son subcomandos del binario (`mem hook <evento>`), no scripts shell:
> no dependen de `bash`/`curl` ni de un servidor HTTP, y corren igual en Windows.
>
> Regla de oro: un hook nunca aborta el arranque del agente — ante cualquier
> error sale silencioso con código 0.
>
> El protocolo de memoria (cuándo guardar, buscar, cerrar sesión) ya no depende
> de `AGENTS.md`/`CLAUDE.md`: el servidor `mem mcp` lo declara él mismo en
> `initialize.instructions`, en la descripción de cada tool, y embebido en la
> respuesta de `get_context` — funciona igual con solo el MCP conectado, sin
> `mem install` ni archivos en el repo. El bloque en `AGENTS.md`/`CLAUDE.md`
> queda como refuerzo, no como requisito.

### Configuración Manual MCP

Si prefieres no usar el instalador: con `mem` en el PATH, `claude mcp add -s user gomemory mem mcp` registra el
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

---

## 10. Seguridad

- **Sin telemetría** — gomemory no envía datos a ningún servidor. Todo ocurre localmente.
- **Binario autocontenido** — sin dependencias compartidas que puedan ser comprometidas.
- **Redacción automática** — contenido envuelto en `<private>...</private>` se elimina antes de llegar a la base de datos.
- **SQLite WAL** — integridad ACID con Write-Ahead Logging. Los datos sobreviven cortes de energía.
- **Permisos MCP granulares** — `forget_memory` queda fuera de auto-approve por ser destructivo/irreversible.

---

## 11. Stack Técnico

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

## 12. Portabilidad

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
