# Plan de implementación: Memoria autónoma transversal

**Spec**: [spec.md](./spec.md) · **Rama**: `006-hooks-memoria-autonoma` · **Versión objetivo**: v1.13.0

## Enfoque

Cerrar las dos brechas de autonomía sin renunciar a la ventaja arquitectónica de gomemory (un único binario `mem hook`, sin servidor HTTP). Toda la lógica nueva vive en Go y se cablea en los puntos de inyección por turno de cada agente.

## Comparación de referencia (engram → gomemory)

| Comportamiento activo | engram | gomemory antes | gomemory ahora |
|---|---|---|---|
| Cargar tools diferidas | systemMessage + ToolSearch | `{"tools": true}` (no-op) | systemMessage + ToolSearch ✅ |
| Recordatorio de guardado | UserPromptSubmit, >15 min, debounce | pasivo tras 1er prompt | `computeSaveNudge` transversal ✅ |
| Captura pasiva de subagentes | SubagentStop → server | — | pendiente |
| Re-inyección post-compactación | SessionStart:compact | PreCompact (parcial) | pendiente |

## Diseño

### 1. Carga de tools diferidas (FR-001)
`hookUserPromptSubmit` (primer prompt) emite:
- `systemMessage`: instrucción imperativa `memoryToolBootstrap` con `ToolSearch select:mcp__gomemory__get_context,save_memory,search_memories,list_memories,get_memory,forget_memory,judge_memories,start_session,end_session` + "luego llama a get_context()".
- `hookSpecificOutput.additionalContext`: `memoryProtocolReminder` (forma documentada, antes iba como `additionalContext` top-level).
Se elimina `{"tools": true}`.
**Ámbito**: solo Claude Code registra este hook; OpenCode carga tools por su plugin y el resto por instrucciones MCP nativas — no necesitan bootstrap.

### 2. Recordatorio de guardado transversal (FR-002 a FR-005)
- **Persistencia**: `SecondsSinceLastSave(project)` — consulta SQL que devuelve segundos desde la última memoria con `type != 'checkpoint'` (o inexistencia). El offset `-5 hours` se reusa de `Now` y se cancela en la resta. Expuesta en el puerto `MemoryRepository`.
- **Decisión (única fuente)**: `computeSaveNudge(deps, root, project)` en `adapters/primary/cli/nudge.go`. Gate de edad de sesión (5 min), umbral (15 min), debounce por archivo `.memory/.last-nudge` (15 min). Si no hay guardado real todavía, el reloj es la edad de la sesión.
- **Cableado transversal**:
  - Claude Code: branch de prompt subsiguiente de `hookUserPromptSubmit` → `{"systemMessage": <nudge>}` o `{}`.
  - Evento `mem hook nudge` → imprime el texto en plano o nada.
  - OpenCode: `experimental.chat.system.transform` invoca `mem hook nudge` y lo empuja al system prompt si no está vacío.

## Archivos afectados

- `adapters/secondary/persistence/memory.go` — `SecondsSinceLastSave`.
- `application/ports/memory_repository.go` — método en el puerto.
- `adapters/secondary/persistence/repositories.go` — implementación.
- `adapters/primary/cli/nudge.go` — **nuevo**: decisión + umbrales + texto.
- `adapters/primary/cli/cmd_hook.go` — fix del primer prompt, branch subsiguiente con nudge, evento `nudge`, `hookNudge`.
- `infrastructure/plugin/opencode/gomemory.ts` — nudge en `system.transform`.
- `adapters/secondary/persistence/memory_test.go` — test de `SecondsSinceLastSave`.

## Verificación

- Unit: `TestSecondsSinceLastSave` (exclusión de checkpoints, caso vacío, edad).
- Integración: `HookMarker` sigue verde (primer prompt trae additionalContext, subsiguiente pasivo cuando no toca).
- End-to-end contra binario real con timestamps retrasados en SQLite: recordatorio dispara con sesión vieja sin guardar; debounce silencia repetición; guardado reciente silencia; checkpoint reciente NO reinicia el reloj; vía Claude (`user-prompt-submit`) emite `systemMessage`.

## Riesgos y decisiones

- **Offset horario `-5 hours`**: se reusa el mismo de `Now` para que se cancele en la resta; no se introduce un segundo criterio de tiempo.
- **Debounce entre sesiones**: el marcador `.memory/.last-nudge` persiste entre sesiones a propósito, para que reiniciar la sesión no dispare un recordatorio inmediato (el gate de edad de sesión igual lo evita en los primeros 5 min).
- **Transversalidad de agentes sin hook por turno** (Cursor/Windsurf/Cline/Codex): no tienen punto de inyección por turno propio en gomemory; siguen cubiertos por las instrucciones MCP nativas y el checkpoint automático. El recordatorio activo aplica donde hay hook por turno (Claude Code y OpenCode).
