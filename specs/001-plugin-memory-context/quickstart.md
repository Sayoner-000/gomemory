# Quickstart: Plugin de Memoria con Contexto Automático

## Prerequisitos

- Go 1.22+ instalado
- gomemory compilado (`go build -o mem .`)
- OpenCode instalado (para probar plugin OpenCode)
- Claude Code instalado (para probar plugin Claude Code)

## Escenario 1: Instalación de Plugin OpenCode

```bash
# Compilar gomemory
go build -o mem .

# Instalar plugin para OpenCode
./mem setup opencode --target /tmp/test-project

# Verificar instalación
cat /tmp/test-project/.opencode.json
# → Debe incluir mcpServers.gomemory

# Verificar plugin instalado
ls ~/.config/opencode/plugins/gomemory.ts
# → Debe existir
```

**Resultado esperado**: Plugin instalado, MCP configurado, sin errores.

## Escenario 2: Instalación de Plugin Claude Code

```bash
# Instalar plugin para Claude Code
./mem setup claude-code --target /tmp/test-project

# Verificar MCP config
cat /tmp/test-project/.mcp.json
# → Debe incluir mcpServers.gomemory

# Verificar hooks
ls /tmp/test-project/.claude/plugins/gomemory/hooks/
# → Debe contener hooks.json

# Verificar scripts
ls /tmp/test-project/.claude/plugins/gomemory/scripts/
# → Debe contener session-start.sh, session-stop.sh,
#   user-prompt-submit.sh, post-compaction.sh

# Verificar skill
ls /tmp/test-project/.claude/plugins/gomemory/skills/memory/SKILL.md
# → Debe existir
```

**Resultado esperado**: Plugin instalado con todos los archivos.

## Escenario 3: Servidor HTTP Background

```bash
# Iniciar servidor en puerto por defecto
./mem serve &
sleep 1

# Verificar health
curl http://127.0.0.1:9735/health
# → {"status":"ok"}

# Verificar creación de sesión
curl -X POST http://127.0.0.1:9735/session/start
# → {"session_id": "..."}

# Verificar contexto
curl http://127.0.0.1:9735/context
# → {"recent_memories": [...], "recent_sessions": [...]}

# Verificar cierre de sesión
curl -X POST -d '{"summary":"test session"}' \
  http://127.0.0.1:9735/session/end
# → {"session_id": "...", "ended_at": "..."}

# Detener servidor
kill %1
```

**Resultado esperado**: API HTTP responde en todos los endpoints.

## Escenario 4: Memory Protocol Inyectado

```bash
# Iniciar servidor HTTP
./mem serve &
sleep 1

# Obtener Memory Protocol
curl http://127.0.0.1:9735/context
# → Debe incluir campos de contexto de sesiones previas

# Verificar que el protocolo se puede inyectar
# (El plugin se encarga de la inyección en system prompt)

kill %1
```

**Resultado esperado**: Contexto disponible via HTTP API.

## Escenario 5: Idempotencia de Instalación

```bash
# Instalar dos veces
./mem setup opencode --target /tmp/test-project
./mem setup opencode --target /tmp/test-project

# La segunda instalación no debe duplicar config
# ni sobrescribir archivos existentes
```

**Resultado esperado**: Segunda ejecución no modifica nada (idempotente).

## Escenario 6: Degradación Graceful

```bash
# Ocupar el puerto 9735
python3 -m http.server 9735 &
sleep 1

# Iniciar servidor gomemory (debe fallar en puerto 9735)
./mem serve

# El plugin debe continuar con MCP stdio sin servidor HTTP
# La funcionalidad se degrada: sin captura pasiva, sin sesiones automáticas
```

**Resultado esperado**: Servidor falla gracefulmente, plugin opera en modo
reducido.
