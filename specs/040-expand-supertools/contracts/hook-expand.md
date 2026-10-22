# Contrato: `mem hook expand <event> [--emit=<dialecto>]`

## Entrada

- `<event>`: `session-start | prompt | pre-tool | post-tool | stop`. Cualquier otro valor → exit 0 sin salida.
- stdin: el payload del harness, leído con un tope de 256 KiB. Si no es JSON, se trata como texto (`prompt` en el evento prompt; en los demás, se permite sin salida).
- Campos reconocidos: `session_id`, `hook_event_name`, `source`, `tool_name`, `tool_input.{command,file_path,path,content,new_string}`, `prompt`, `last_assistant_message`. En OpenCode, el plugin envía `{tool, args, prompt, session_id, source}` y el adaptador lo normaliza; `source` vale `startup` en `session.created` y `compact` en `session.compacted`, para que `counter.reset` y `episode.reset` también ocurran en OpenCode (3.ª pasada, U1).

## Proceso

1. Leer los ajustes. Con `ExpandDisabled`, salida de allow del dialecto y exit 0.
2. Construir `ExpandSignals` (R6: contador y huella desde archivos, sin SQLite en pre-tool ni prompt).
3. `RouteSupertools` una sola vez.
4. Ejecutar las acciones dentro de `withBudget(domain.HookBudget)`.
5. Filtrar las sugerencias por episodio y marcarlas.
6. Si algo emite (warn, block o suggest), `recordGuard`.
7. `renderExpandOutput(dialecto, evento, outcome)`.

## Garantías

- **G1 Fallo abierto**: un pánico, un timeout, un payload inválido o un error de E/S producen la salida de allow y exit 0, salvo en el dialecto neutral cuando el bloqueo ya está decidido. Un timeout registra `expand.timeout` y un error o pánico registra `expand.error`. La base se abre de forma perezosa, solo para registrar.
- **G1b Interruptor**: `ExpandDisabled` → allow sin nada más. `ExpandOff` → `ExpandSwitches.Off`; las supertools apagadas no actúan y las demás sí.
- **G1c Visibilidad**: cada `block`, `warn` o `suggest` emitido queda registrado y aparece en `mem expand status` (activaciones de 7 días).
- **G2 Sin salida espuria**: `post-tool` nunca escribe en stdout ni en stderr (instincts es silencioso). La única excepción sería un bloqueo, que este evento no produce.
- **G3 Presupuesto**: p95 < 150 ms en 100 invocaciones de `pre-tool` con 64 reglas cargadas (quickstart §5).
- **G4 Idempotencia por episodio**: cada `suggest:<skill>` se emite como mucho una vez entre dos `episode.reset`.
- **G6 Nunca aprobar**: la salida de `pre-tool` no contiene `"allow"` en ningún dialecto. El módulo solo puede denegar o informar; nunca relaja el control de permisos del harness.
- **G7 Contexto y plazo**: `ctx` con `context.WithTimeout(domain.HookBudget)` se propaga a todas las acciones. `withBudget` queda como red de seguridad.
- **G5 Privacidad**: ningún contenido del payload llega a `hook_guard_events`. Lo que se escribe en la cola de instincts pasa por `RedactSecrets` y la limpieza de `<private>`.

## Salida por dialecto (R4)

| Resultado | claude / json | text (OpenCode) | neutral |
|---|---|---|---|
| allow, sin contexto | `{}` | `` | `` (exit 0) |
| allow + contexto (warn/suggest) en prompt o session-start | `{"hookSpecificOutput":{"hookEventName":"<Evento>","additionalContext":"…"}}` | texto | texto en stdout |
| allow + contexto en pre-tool | `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"…"}}` (**sin** `permissionDecision`: `"allow"` saltaría la confirmación de permisos del harness) | texto | texto en stdout |
| block en pre-tool | `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"…"}}` | `BLOCK: …` (el plugin lanza `Error`) | stderr, exit 1 |
| block en prompt o stop | `{"decision":"block","reason":"…"}` | `BLOCK: …` | stderr, exit 1 |

El texto de contexto lleva el prefijo `[gomemory expand]`, una sugerencia por línea, y ocupa como máximo 600 caracteres en total.

## Formato de la sugerencia

```text
[gomemory expand] Antes de confirmar cambios, aplica la skill `verify-loop` (build, vet, test -race, lint, diff y bugs previos sobre los archivos tocados).
```

## Cableado (R3)

El instalador de cada harness registra la tabla R3. `hookCommandIsGomemory` reconoce `hook expand` a través del registro `domain.GomemoryHookSubcommands` (R2).
