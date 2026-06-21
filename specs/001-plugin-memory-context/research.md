# Research: Plugin de Memoria con Contexto Automático

## Arquitectura de Plugins

### Decisión: Plugin por agente, embebidos via go:embed

Los plugins (TypeScript para OpenCode, hooks+skills para Claude Code) se
embeben en el binario Go mediante `go:embed` y se instalan en el sistema
del agente via `mem setup <agent>`. Esto mantiene el binario autocontenido
y elimina dependencias externas.

**Rationale**: Engram usa el mismo patrón (`internal/setup/setup.go` con
go:embed). Cada plugin es un directorio dentro de `plugin/<agent>/` que se
empaqueta en el binario en tiempo de compilación.

**Alternativas consideradas**:
- Plugins descargados desde un registry — sobreingeniería para un proyecto
  monousuario
- Scripts de instalación separados — más frágil, dos artefactos que mantener
- Configuración manual de MCP — el usuario tendría que copiar archivos

### Decisión: Servidor HTTP de background (mem serve)

Se añade `cmd_serve.go` que inicia un servidor HTTP en `127.0.0.1:9735`
(puerto por defecto) para:
- Crear/cerrar sesiones sin latencia MCP
- Inyectar contexto de sesiones previas
- Captura pasiva de eventos (user-prompt-submit)
- Auto-inicio desde el plugin del agente

**Rationale**: MCP usa stdio — no puede mantener estado entre invocaciones
del agente. Un servidor HTTP persistente permite tracking de sesiones y
captura de eventos en tiempo real. Engram usa `engram serve` con el mismo
patrón (puerto 7437 por defecto).

**Alternativas consideradas**:
- Todo vía MCP stdio — no permite sesiones persistentes ni captura pasiva
- Archivos de estado en disco — race conditions, sin sincronización
- Socket Unix — menos portable que HTTP

### Decisión: Memory Protocol inyectado en system prompt

El Memory Protocol es un bloque de texto con reglas estrictas que se
concatena al mensaje de sistema existente del agente (no como mensaje
separado). Esto garantiza compatibilidad con modelos que solo aceptan
un bloque de sistema (Qwen, Mistral/Ministral via llama.cpp).

**Rationale**: Engram demostró que este approach funciona. La inyección
via `chat.system.transform` (OpenCode) o vía skill (Claude Code)
garantiza que el protocolo sobrevive a compactaciones.

### Decisión: Revelación progresiva de 3 capas

| Capa | Tool | Tokens | Cuándo |
|------|------|--------|--------|
| 1. Resúmenes | `search_memories` | ~100 | Búsqueda inicial |
| 2. Línea de tiempo | `mem_timeline` | ~200 | Contexto cronológico |
| 3. Contenido completo | `get_memory` | ~500+ | Solo cuando es necesario |

**Rationale**: Engram usa el mismo patrón (Progressive Disclosure). Evita
volcar toda la memoria (~5000+ tokens) cuando solo se necesita contexto
superficial.

### Decisión: Captura pasiva limitada

No se registran tool calls crudos. El agente decide qué es significativo
mediante el Memory Protocol. La captura pasiva se limita a:
- Inicio/fin de sesión (automático)
- Contexto de sesiones previas (al iniciar)
- Salvado de prompt en compactación

**Rationale**: Engram documenta que "raw tool call recording" no es el
camino. El agente debe ser quien decida qué merece persistirse.

### Referencias

- Engram Architecture: https://github.com/Gentleman-Programming/engram
- Engram Plugins: plugin/opencode/engram.ts, plugin/claude-code/
- Engram Memory Protocol: plugin/claude-code/skills/memory/SKILL.md
- Engram Server: internal/server/server.go
- Engram Setup: internal/setup/setup.go
