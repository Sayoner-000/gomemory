# Data Model: Plugin de Memoria con Contexto Automático

## Entidades

### PluginConfig

Representa la configuración de un plugin para un agente específico.

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `agent` | string | Nombre del agente: `opencode`, `claude-code` |
| `enabled` | bool | Plugin activo o no |
| `auto_start_server` | bool | Iniciar servidor HTTP automáticamente |
| `server_port` | int | Puerto del servidor HTTP (default: 9735) |
| `inject_context` | bool | Inyectar contexto previo al iniciar sesión |
| `protocol_version` | string | Versión del Memory Protocol |

### PluginFile

Archivo embehido en el binario via go:embed.

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `agent` | string | `opencode` o `claude-code` |
| `path` | string | Ruta relativa dentro de `plugin/` |
| `content` | []byte | Contenido del archivo |
| `mode` | string | Permisos del archivo destino |
| `template` | bool | Si contiene placeholders a reemplazar |

### AgentSession

Sesión activa de un agente, gestionada por el servidor HTTP.

| Campo | Tipo | Origen | Descripción |
|-------|------|--------|-------------|
| `id` | string | store/session | UUID de la sesión |
| `project` | string | store/session | Nombre del proyecto |
| `agent` | string | servidor | Agente que inició la sesión |
| `created_at` | string | store/session | Inicio (UTC-5) |
| `ended_at` | *string | store/session | Fin (UTC-5) |
| `summary` | *string | store/session | Resumen de la sesión |

### MemoryProtocol

Reglas inyectadas en el system prompt del agente. No es una entidad
persistente — es texto generado.

| Componente | Descripción | Tokens aprox |
|------------|-------------|--------------|
| Save triggers | Cuándo guardar (bugfix, decisión, descubrimiento) | ~80 |
| Search triggers | Cuándo buscar (reactivo + proactivo) | ~60 |
| Session close | Protocolo obligatorio de cierre | ~100 |
| Compaction recovery | Pasos post-compactación | ~80 |
| Progressive disclosure | Estrategia de 3 capas | ~60 |
| **Total** | | **~380** |

### ContextInjection

Payload de contexto inyectado al inicio de cada sesión. Generado por
el servidor HTTP.

| Campo | Tipo | Fuente | Tokens aprox |
|-------|------|--------|-------------|
| `active_session` | bool | servidor | ~10 |
| `recent_sessions` | []SessionSummary | store/session | ~50 |
| `recent_memories` | []MemorySummary | store/memory | ~100 |
| `project_info` | ProjectInfo | store/db | ~20 |
| **Total** | | | **~180** |

### ProgressiveDisclosureResult

Resultado de una búsqueda con revelación progresiva.

| Capa | Descripción | Tool MCP |
|------|-------------|----------|
| Layer 1 | Lista de resultados con ID, título, tipo, score | `search_memories` |
| Layer 2 | Eventos antes/después de una memoria específica | `timeline` |
| Layer 3 | Contenido completo de una memoria | `get_memory` |

## Relaciones

```
PluginConfig 1──* PluginFile     (un agente puede tener múltiples archivos)
AgentSession 1──* Memory         (una sesión puede tener múltiples memorias)
AgentSession 1──1 SessionSummary (una sesión tiene un resumen)
MemoryProtocol ── injecta en ──> SystemPrompt (no persistente)
PluginConfig ── genera ──> MCPConfig (archivo .opencode.json / .mcp.json)
```

## Estados

### Plugin Setup

```
not-installed → installed (via mem setup <agent>)
installed → configured (via restart del agente)
configured → active (sesión iniciada)
active → error (si servidor HTTP falla, degrade a solo MCP stdio)
```

### Servidor HTTP

```
stopped → starting (auto-inicio desde plugin)
starting → running (puerto 9735)
running → error (puerto ocupado, permiso denegado)
error → stopped (limpieza)
```

## Validaciones

- `server_port`: 1024-65535, único por proyecto
- `agent`: valor válido en `opencode`, `claude-code`
- `protocol_version`: semver (ej. `1.0.0`)
- Instalación de plugin: idempotente (no sobrescribe si ya existe)
- Sesiones: no duplicar activas (una por proyecto/agente)
