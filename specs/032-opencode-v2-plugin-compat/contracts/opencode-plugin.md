# Contrato: plugin `gomemory.ts` ↔ OpenCode 1.x / 2.x

## C-1 · Forma del módulo

```text
export const GomemoryPlugin            // fábrica v1: (input) => Promise<V1Hooks>
export default {
  id: "gomemory",
  setup(ctx)  -> Promise<Cleanup>      // OpenCode 2.x
  server(input) -> Promise<V1Hooks>    // OpenCode 1.x (misma fábrica)
}
```

- Sin imports de ejecución fuera de `node:*`. `import type` está permitido.
- El marcador `{{BIN_PATH}}` se sigue sustituyendo en la instalación (sin cambios).

## C-2 · Ganchos por versión

| Capacidad | 1.x (`server`) | 2.x (`setup`) | Llamada a `mem` |
|---|---|---|---|
| Inicio de sesión | `event` `session.created` | evento `session.created` | `session start` |
| Checkpoint | `event` `session.idle` | evento `session.idle` + acumulador | `hook turn-end` |
| Procedencia | `chat.message` | `session.hook("prompt")` | `hook prompt` |
| Inyección | `experimental.chat.system.transform` | `session.hook("context")` | `context`, `hook nudge`, `hook agent-notice`, `hook octopus-delegation-policy opencode` |
| Pre-compactación | `experimental.session.compacting` | `session.hook("compaction")` | `hook compaction-context` |
| Post-compactación | `event` `session.compacted` | evento `session.compaction.ended` | `hook compact-summary`, `hook post-compact` |
| Subagente | `tool.execute.after` (`task`) | `tool.hook("execute.after")` (`task`, `completed`) | `hook subagent-stop` |
| Rastros de canal | `hook channel-fired` / `channel-error` | ídem | ídem |

Los payloads JSON por stdin son **idénticos** en ambas versiones:
`{prompt}`, `{files, commands}`, `{plan}`, `{summary}`, `{last_assistant_message}`.

## C-3 · Invariantes

1. Ningún fallo de `mem` propaga una excepción al host (best-effort, FR-011).
2. En 2.x se ignoran los eventos cuyo `location.directory` no coincide con `ctx.location.directory`.
3. La recuperación post-compactación se inyecta como máximo una vez por sesión.
4. El cleanup de `setup` aborta la suscripción y vacía los acumuladores (FR-007).
5. En 1.x el comportamiento observable es byte a byte el actual (FR-002): la rama v1 no cambia de runner.

## C-4 · Salida de `mem doctor` (sección OpenCode)

Líneas nuevas (texto en español, mismo estilo que el resto del doctor):

```text
OpenCode: v2.0.16 detectado · plugin gomemory: dual (v1+v2) ✅
OpenCode: v2.0.16 detectado · plugin gomemory: solo v1 ❌ → ejecuta `mem setup-mcp --scope global --agents opencode`
OpenCode: versión no detectada · plugin gomemory: dual (v1+v2) ✅
⚠️  plugin ajeno con forma v1 en OpenCode 2.x: ~/.config/opencode/plugins/cbm-augment.ts (no es de gomemory; actualízalo en su proyecto)
🧹 artefacto de pruebas en la carpeta de plugins: gomemory.test.mjs (se retira al reinstalar)
```
