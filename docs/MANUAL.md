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

---

## 1. Instalación Rápida

```bash
# Compilar gomemory
go build -o mem .

# Instalar plugin para OpenCode (recomendado si usas OpenCode)
./mem setup opencode

# O para Claude Code
./mem setup claude-code

# Verificar que funciona
./mem --help
```

### Prerrequisitos

- Go 1.22+
- OpenCode 0.70+ (para plugin OpenCode)
- Claude Code (para plugin Claude Code)
- No se necesita CGO

---

## 2. Plugin para OpenCode

### Instalación

```bash
./mem setup opencode
```

Esto:

1. Copia `plugin/opencode/plugin.ts` a `~/.config/opencode/plugins/gomemory/`
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

Esto instala en `.memory/plugins/claude-code/` dentro del proyecto:

1. `.mcp.json` — Configuración MCP apuntando a `mem mcp`
2. `.claude-plugin/plugin.json` — Manifest del plugin
3. `hooks/hooks.json` — Hooks registrados para startup, compact, submit, shutdown
4. `scripts/session-start.sh` — Inicia sesión HTTP, importa git-sync
5. `scripts/session-stop.sh` — Cierra sesión HTTP
6. `scripts/user-prompt-submit.sh` — ToolSearch en primer prompt
7. `scripts/post-compaction.sh` — Recuperación post-compactación
8. `skills/memory/SKILL.md` — Memory Protocol skill

### Verificación

```bash
ls -la .memory/plugins/claude-code/
# hooks.json, scripts/, skills/, .mcp.json, .claude-plugin/

# Verificar que los hooks están registrados
cat .memory/plugins/claude-code/hooks/hooks.json
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
go build -o mem . && echo "OK"
go vet ./... && echo "OK"
```

---

## 6. Solución de Problemas

### Error: "plugin directory not found"

```bash
# Asegúrate de compilar desde la raíz del proyecto
ls plugin/opencode/plugin.ts   # debe existir
ls plugin/claude-code/         # debe existir
```

### Error: "address already in use"

```bash
# Puerto 9735 ocupado — usa otro puerto
./mem setup opencode --port 19735

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
ls .memory/plugins/claude-code/

# Reinstalar
./mem setup claude-code
```

---

## 7. Memory Protocol

El Memory Protocol es un conjunto de reglas inyectadas en el system prompt del
agente. No requiere configuración adicional — se activa automáticamente con
cualquier plugin.

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

## Referencia Rápida

```bash
# Instalación
./mem setup opencode              # Plugin para OpenCode
./mem setup claude-code           # Plugin para Claude Code

# Servidor HTTP
./mem serve                       # Iniciar servidor (auto por plugins)
./mem serve --port 9735           # Puerto personalizado

# Verificación
./mem --help                      # Listar comandos
curl http://127.0.0.1:9735/health # Healthcheck
go test ./...                     # Tests
```

Para más detalles técnicos, ver:
- `docs/PLUGINS.md` — Arquitectura del sistema de plugins
- `docs/MEMORY-PROTOCOL.md` — Referencia técnica del protocolo
