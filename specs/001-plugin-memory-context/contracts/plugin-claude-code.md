# Contrato: Plugin Claude Code

## Propósito

Plugin basado en hooks nativos de Claude Code que gestiona el ciclo de
vida de sesiones, inyecta contexto previo, y proporciona un skill de
Memory Protocol para que el agente sepa cuándo y cómo interactuar con
la memoria de gomemory.

## Interfaces

### Hook: `startup`

Script: `scripts/session-start.sh`

Ejecutado al iniciar Claude Code. Acciones:
1. Verificar que el servidor HTTP de gomemory esté corriendo
   (iniciarlo si no)
2. Crear sesión via HTTP API (`POST /session/start`)
3. Importar chunks sincronizados por git desde `.memory/manifest.json`
4. Inyectar contexto de sesiones previas en la conversación inicial

### Hook: `compact`

Script: `scripts/post-compaction.sh`

Ejecutado después de una compactación de contexto. Acciones:
1. Inyectar el resumen de la sesión previa
2. Instruir al agente: "FIRST ACTION REQUIRED — llama
   `end_session(summary)` con este contenido antes de seguir"
3. Inyectar el contexto previo para que el agente continúe sin pérdida

### Hook: `UserPromptSubmit`

Script: `scripts/user-prompt-submit.sh`

Ejecutado antes de cada prompt del usuario. Acciones:
1. En el primer prompt: inyectar ToolSearch para que Claude cargue
   las herramientas MCP de gomemory
2. En prompts posteriores: opcionalmente inyectar recordatorio de
   guardado (solo si el servidor HTTP responde rápido, <50ms)
3. Timeout: 2s (no bloquear la experiencia del usuario)

### Hook: `shutdown`

Script: `scripts/session-stop.sh`

Ejecutado al cerrar Claude Code. Acciones:
1. Si hay sesión activa, cerrarla via HTTP API (`POST /session/end`)
2. Registrar evento de fin de sesión

### Skill: Memory Protocol

Archivo: `skills/memory/SKILL.md`

Inyectado como skill del agente. Contiene:
- Cuándo guardar (bugfix, decisión, descubrimiento, patrón)
- Cuándo buscar (reactivo si usuario pregunta, proactivo al iniciar tarea)
- Protocolo de cierre de sesión obligatorio
- Recuperación post-compactación
- Revelación progresiva de 3 capas

### Configuración Generada

Archivo `.mcp.json`:

```json
{
  "mcpServers": {
    "gomemory": {
      "command": "/path/to/mem",
      "args": ["mcp", "--root", "/path/to/project"]
    }
  }
}
```

Archivo `.claude/settings.json` (hooks):

```json
{
  "hooks": {
    "SessionStart": ["scripts/session-start.sh"],
    "PreCompact": ["scripts/post-compaction.sh"],
    "UserPromptSubmit": ["scripts/user-prompt-submit.sh"],
    "SessionEnd": ["scripts/session-stop.sh"]
  }
}
```

Archivo `CLAUDE.md` (si se requiere "nuclear option"):

```markdown
<!-- SPECKIT START -->
...
<!-- SPECKIT END -->
```

### Instalación

```
mem setup claude-code [--target dir] [--port 9735]
```

Acciones:
1. Crear `.mcp.json` con configuración MCP de gomemory
2. Copiar directorio `plugin/claude-code/` a `.claude/plugins/gomemory/`
3. Configurar hooks en `.claude/settings.json`
4. No sobrescribir si ya existe (idempotente)
