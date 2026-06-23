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
mem install .        # Cablea memoria + MCP + hooks para todos los agentes
mem --help
```

### Opción B — Desde el fuente

```bash
go build -o mem ./infrastructure/
./mem setup opencode      # o claude-code
./mem --help
```

### Hooks portables

Los hooks de Claude Code son subcomandos del binario (`mem hook session-start`,
`session-end`, `pre-compact`, `user-prompt-submit`): no usan `bash`/`curl` ni
servidor HTTP y funcionan igual en Windows. Se configuran solos con
`mem install .` o `mem setup claude-code`.

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

1. Copia `infrastructure/plugin/opencode/plugin.ts` a `~/.config/opencode/plugins/gomemory/`
2. Crea/actualiza `~/.config/opencode/opencode.json` con la referencia al plugin
3. El plugin se activa automáticamente al iniciar OpenCode

### Verificación

```bash
ls ~/.config/opencode/plugins/gomemory/
# Debería ver: plugin.ts

cat ~/.config/opencode/opencode.json
# Buscar "plugins" conteniendo "gomemory"
```

### Qué Hace el Plugin

- **Auto-server**: Inicia `mem serve` en background si no está corriendo
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

Esto instala en `.claude/plugins/gomemory/` dentro del proyecto:

1. `.claude-plugin/plugin.json` — Manifest del plugin
2. `hooks/hooks.json` — Hooks registrados para `SessionStart`, `PreCompact`, `UserPromptSubmit`, `SessionEnd`
3. `scripts/session-start.sh` — Inicia el servidor HTTP si no está corriendo, crea sesión
4. `scripts/session-stop.sh` — Cierra sesión HTTP
5. `scripts/user-prompt-submit.sh` — ToolSearch en primer prompt
6. `scripts/post-compaction.sh` — Recuperación post-compactación
7. `skills/memory/SKILL.md` — Memory Protocol skill

La configuración MCP real (`.mcp.json` con la ruta absoluta del binario) se escribe en la raíz del proyecto, no dentro del directorio del plugin.

### Verificación

```bash
ls -la .claude/plugins/gomemory/
# hooks/, scripts/, skills/, .claude-plugin/

# Verificar que los hooks están registrados
cat .claude/plugins/gomemory/hooks/hooks.json
cat .claude/settings.json   # hooks efectivos para Claude Code
```

---

## 4. Servidor HTTP

El servidor HTTP corre en `127.0.0.1:9735` y es auto-iniciado por los plugins.
También puede iniciarse manualmente:

```bash
./mem serve                # Puerto default 9735
./mem serve --port 19735   # Puerto personalizado
```

### Endpoints

| Endpoint | Método | Body | Respuesta |
|----------|--------|------|-----------|
| `POST /session/start` | — | `{"session_id": "...", "created_at": "..."}` |
| `POST /session/end` | `{"summary": "..."}` | `{"session_id": "...", "ended_at": "..."}` |
| `GET /context` | — | Contexto en markdown de sesiones recientes |
| `GET /health` | — | `{"status": "ok", "project": "..."}` |

### Healthcheck

```bash
curl http://127.0.0.1:9735/health
# {"status":"ok","project":"/ruta/al/proyecto"}
```

---

## 5. Verificación

### Test Rápido

```bash
# 1. Verificar que el servidor HTTP funciona
./mem serve &
sleep 1
curl http://127.0.0.1:9735/health

# 2. Crear una sesión
curl -X POST http://127.0.0.1:9735/session/start

# 3. Obtener contexto
curl http://127.0.0.1:9735/context

# 4. Cerrar sesión
curl -X POST http://127.0.0.1:9735/session/end \
  -H "Content-Type: application/json" \
  -d '{"summary":"Prueba manual completada"}'

# 5. Detener servidor
kill %1
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

### Error: "address already in use"

```bash
# Puerto 9735 ocupado — usa otro puerto
./mem setup --port 19735 opencode

# O mata el proceso anterior
lsof -i :9735
kill <PID>
```

### Error: "MCP connection refused"

```bash
# El servidor HTTP debe estar corriendo
./mem serve &
curl http://127.0.0.1:9735/health
```

### El Plugin OpenCode No se Activa

```bash
# Verificar que el plugin está instalado
ls ~/.config/opencode/plugins/gomemory/

# Verificar que opencode.json lo referencia
cat ~/.config/opencode/opencode.json

# Reinstalar si es necesario
./mem setup opencode --force
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

# Servidor HTTP
./mem serve                       # Iniciar servidor (auto por plugins)
./mem serve --port 9735           # Puerto personalizado

# Mantenimiento de memoria
./mem purge --older-than-days 90  # Purgar memorias viejas del proyecto actual
./mem compact                     # Recuperar espacio en disco
./mem gc                          # Garbage collection a demanda (90 días default)
./mem uninstall --yes             # Desinstalación completa (reverso de install)

# Verificación
./mem --help                      # Listar comandos
curl http://127.0.0.1:9735/health # Healthcheck
go test ./...                     # Tests
```

Para más detalles técnicos, ver:
- `docs/PLUGINS.md` — Arquitectura del sistema de plugins
- `docs/MEMORY-PROTOCOL.md` — Referencia técnica del protocolo
