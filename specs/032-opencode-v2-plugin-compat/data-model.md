# Data Model: Compatibilidad del plugin de OpenCode con v2

No hay persistencia nueva ni migraciones de SQLite. Las "entidades" son
estructuras en memoria del plugin y del diagnóstico.

## 1. PluginDefinition (export por defecto de `gomemory.ts`)

| Campo | Tipo | Regla |
|---|---|---|
| `id` | `"gomemory"` | Estable. Identifica el plugin en `opencode plugin list` y en el almacenamiento de v2 |
| `setup` | `(ctx) => Promise<Cleanup>` | Rama v2. Devuelve un cleanup que aborta la suscripción de eventos |
| `server` | `(input) => Promise<V1Hooks>` | Rama v1. Idéntica a la fábrica actual |

Export adicional: `GomemoryPlugin` (fábrica v1, la misma función que usa `server`).

## 2. MemRunner (puerto interno del plugin)

`(args: string[], stdin?: string) => Promise<string | null>`

- `null` indica un fallo del comando; `""` es una respuesta vacía legítima (contrato actual de `run`).
- v1 se implementa con el `$` de Bun y v2 con `node:child_process.execFile`.
- Invariante: ambas implementaciones ejecutan `BIN` con `cwd` = raíz del proyecto.

## 3. GomemoryCore (lógica compartida, independiente de la versión)

Recibe un `MemRunner` y expone operaciones que llaman a `mem`:

| Operación | Llamada a `mem` | La usa |
|---|---|---|
| `sessionStart()` | `session start` | v1 `session.created`, v2 `session.created` |
| `sessionEnd()` | `session end` | fin de sesión |
| `turnEnd({files, commands})` | `hook turn-end` (stdin JSON) | idle |
| `planApproved(plan)` | `hook plan-approved` | idle (modo plan) |
| `prompt(text)` | `hook prompt` | v1 `chat.message`, v2 `prompt` |
| `systemParts(sessionID)` → `string[]` | recuperación pendiente, `channel-fired`, `hook octopus-delegation-policy opencode`, `context`, `hook nudge`, `hook agent-notice` (y `channel-error` si alguna falla) | v1 transform, v2 `context` |
| `compactionContext()` → `string` | `hook compaction-context` | v1 `compacting`, v2 `compaction` |
| `afterCompaction(sessionID, summary?)` | `hook compact-summary`, `hook post-compact` → `pendingRecovery` | v1 `session.compacted`, v2 `session.compaction.ended` |
| `subagentStop(text)` | `hook subagent-stop` | `task` completada |

Estado interno: `pendingRecovery: Map<sessionID, string>` (se consume una
sola vez por sesión, como hoy).

## 4. TurnAccumulator (solo v2)

`Map<sessionID, { files: Set<string>, commands: string[] }>`

- **Alta**: `tool.hook("execute.after")` con `status === "completed"`, según la tabla de nombres (R-6).
- **Vaciado**: en `session.idle` llama a `turnEnd` y borra la entrada.
- **Descarte**: en `session.deleted` y en el cleanup.
- **Regla**: un turno sin archivos ni comandos no llama a `turn-end` (mismo comportamiento que v1).

## 5. OpenCodeInstallStatus (diagnóstico, Go)

| Campo | Tipo | Origen |
|---|---|---|
| `Version` | string (`""` si no se detecta) | `opencode --version` (R-11) |
| `Major` | int (0 = desconocido) | derivado de `Version` |
| `PluginPath` | string | `~/.config/opencode/plugins/gomemory.ts` |
| `PluginShape` | `dual` \| `v1-only` \| `missing` | inspección textual: `export default` + `setup` + `server` |
| `Compatible` | bool | `v1-only` y `Major >= 2` → false; `missing` → false |
| `ForeignV1Plugins` | []string | `.ts`/`.js` ajenos sin `export default` cuando `Major >= 2` |
| `StaleTestArtifact` | bool | existe `gomemory.test.mjs` en la carpeta de plugins |

Transiciones: `v1-only` → (reinstalar) → `dual`. No hay más estados.

## 6. Matriz de canales (existente, `domain/channel_matrix.go`)

Sin filas nuevas: las rutas de usuario no cambian. Cambia **el estado que se
reporta** cuando la forma instalada es `v1-only` en OpenCode 2.x: los canales
del plugin (`plan_entry`, `turn_reminder`) se muestran como rotos con la causa
"plugin v1 en OpenCode 2.x", y no como sanos (FR-005).
