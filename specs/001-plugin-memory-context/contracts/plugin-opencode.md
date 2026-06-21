# Contrato: Plugin OpenCode

## Propósito

Plugin TypeScript que se integra con OpenCode para inyectar el Memory
Protocol en cada inferencia, gestionar sesiones automáticamente y
proporcionar contexto de memoria sin invocación manual de herramientas MCP.

## Interfaces

### Plugin API (TypeScript)

```typescript
// Punto de entrada del plugin
export function activate(context: PluginContext): void {
  // 1. Auto-iniciar servidor HTTP de gomemory si no corre
  // 2. Crear sesión via HTTP API
  // 3. Inyectar Memory Protocol en system prompt
  // 4. Registrar hooks de ciclo de vida
}

export function deactivate(): void {
  // 1. Cerrar sesión via HTTP API
  // 2. Detener servidor HTTP (opcional)
}
```

### Hooks

| Hook | Evento | Acción |
|------|--------|--------|
| `chat.system.transform` | Cada inferencia | Concatena Memory Protocol al system message existente |
| `session.start` | Nueva sesión | Crea sesión via HTTP, inyecta contexto previo |
| `compact.after` | Post-compactación | Inyecta instrucciones de recuperación + contexto previo |
| `session.end` | Fin de sesión | Cierra sesión vía MCP `end_session` |

### HTTP API (gomemory serve)

Endpoint usado por el plugin para operaciones de sesión:

```
POST /session/start      → { session_id, created_at }
POST /session/end        → { session_id, summary, ended_at }
GET  /context            → { session_id, recent_memories[], recent_sessions[] }
GET  /health             → { status: "ok" }
```

### Configuración Generada

Archivo `.opencode.json`:

```json
{
  "mcpServers": {
    "gomemory": {
      "command": "/path/to/mem",
      "args": ["mcp", "--root", "/path/to/project"]
    }
  },
  "plugins": [
    "gomemory-plugin"
  ]
}
```

### Instalación

```
mem setup opencode [--target dir] [--port 9735]
```

Acciones:
1. Copiar `plugin/opencode/plugin.ts` a `~/.config/opencode/plugins/gomemory.ts`
2. Configurar MCP server en `.opencode.json`
3. No sobrescribir si ya existe (idempotente)
