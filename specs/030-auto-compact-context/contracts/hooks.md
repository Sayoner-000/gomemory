# Contrato: subcomandos `mem hook` de la feature 030

Todos los subcomandos son best-effort. Terminan con código 0 aunque fallen, y
un fallo nunca interrumpe el turno ni la compactación del cliente. El dialecto
de salida se elige con `--emit=claude|json|text` (por defecto `claude`), igual
que en los hooks vigentes.

Los textos se definen en [texts.md](texts.md). Ningún subcomando nombra
agentes, clientes ni comandos de cliente en lo que emite (FR-018).

---

## `mem hook compaction-context` *(nuevo · C1 genérico)*

Emite el contexto de compactación y la orden de persistir el resumen, para
integraciones que añaden texto al compresor por su cuenta.

- **Entrada**: ninguna.
- **Salida (`text`)**: `CompactionContext` + `\n\n` + `CompactorPersistOrder`.
  Sin sesión activa: solo la orden.
- **Consumidor**: la integración con C1 por lista de contexto.

`mem hook pre-compact` no cambia: sigue siendo el manejador heredado
(research.md R12).

## `mem hook post-compact` *(contrato ampliado · C2)*

- **Entrada**: ninguna.
- **Efectos**, en este orden:
  1. Si no hay sesión activa, abre una (FR-023).
  2. Reinicia la huella, el marcador de sesión, el marcador de plan, el
     refuerzo de preferencias y `.pending-agent-notice`.
- **Salida**: `RecoverySteps` + `\n\n` + `CompactionContext` + `\n\n` + contexto
  de proyecto en modo índice, con tope total ≤ `Settings.Budget` (FR-004, FR-005).
- **Dialecto**: texto plano en stdout, como hoy. Tanto el `SessionStart` de
  claude como el de codex inyectan ese stdout al agente, así que no hace falta
  un sobre JSON.

## `mem hook compact-summary` *(nuevo · C6)*

- **Entrada (stdin JSON)**: `{"compact_summary": "<texto>"}`, o `{"summary": "<texto>"}`
  para integraciones que no usan el campo del cliente. Se aceptan ambas claves.
  **Precedencia** si llegan las dos: gana `compact_summary`; si viene vacío o
  solo con espacios, se usa `summary`.
- **Efecto**: `UpdateSummary(sesiónActiva, texto)`. Sin sesión activa, la abre
  primero. Texto vacío: no hace nada.
- **Stdin malformado** (JSON inválido): igual que texto vacío — no hace nada y
  sale con código 0.
- **Salida**: vacía (`json`: `{}`).
- **Idempotencia**: el mismo texto dos veces deja el mismo estado (FR-009).

## `mem hook subagent-stop` *(contrato ampliado · C3)*

- **Entrada (stdin JSON)**: `{"last_assistant_message": "<texto>", ...}`.
- **Efectos**, en este orden:
  1. `CaptureLearnings` sobre `last_assistant_message` (si viene).
  2. Checkpoint de actividad vigente (`recordActivityCheckpoint`).
- **Salida**: vacía; con `--emit=json`, `{}`. El esquema de codex exige JSON en
  este evento.
- **Restricción**: el paso 1 NO DEBE consumir stdin de forma que el paso 2
  pierda el payload. El payload se lee una sola vez y se pasa a ambos.

## `mem hook turn-end` *(contrato ampliado · US4)*

Sin cambios si `CompactAgentNotice == false` (FR-015).

Con la opción activa, cuando `computeCompactNudge` dispara:

| Dialecto | Salida |
|----------|--------|
| `claude` | `{"systemMessage": <CompactNudge>, "hookSpecificOutput": {"hookEventName": "Stop", "additionalContext": <AgentPrepareNotice>}}` |
| `json` | `{"systemMessage": <CompactNudge>}` + escribe `.pending-agent-notice` |
| `text` | `<CompactNudge>` + escribe `.pending-agent-notice` |

Nunca emite `decision`, `continue: false` ni ninguna forma de prolongar o
bloquear el turno (FR-017).

## `mem hook user-prompt-submit` y `mem hook agent-notice` *(C4 diferido)*

- `user-prompt-submit`: si existe `.pending-agent-notice`, antepone
  `AgentPrepareNotice` al contexto que ya emite y borra el archivo.
- `agent-notice` *(nuevo)*: emite `AgentPrepareNotice` si existe el archivo y
  lo borra; si no, salida vacía. Lo usa la integración que entrega C4 por las
  instrucciones de sistema del turno.
