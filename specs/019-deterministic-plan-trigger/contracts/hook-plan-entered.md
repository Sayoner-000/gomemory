# Contrato — `mem hook plan-entered` (borde de entrada al modo plan)

Pone el método de descomposición atómica y el historial del proyecto a disposición del agente en el
momento en que entra en modo plan (Historia 2, FR-005..FR-009). Es **mejor esfuerzo**: si el agente no
ofrece esta señal, la cobertura la da el recordatorio de una línea por turno.

> **Este documento describe la traducción a Claude Code.** La definición neutral del nivel 2, válida
> para cualquier agente, está en [agent-integration.md](./agent-integration.md).

## Registro

| Agente | Evento | Matcher | Comando |
|---|---|---|---|
| Claude Code | `PostToolUse` | `EnterPlanMode` | `mem hook plan-entered` |
| Cualquier otro | invocación directa al entrar en modo plan | — | `mem hook plan-entered` |

## Dialectos de salida

| Dialecto | Selección | Documento |
|---|---|---|
| `neutral` | **por defecto** | documento por stdout, código 0 |
| `claude` | envoltura de eventos reconocida, o `--emit=claude` | `hookSpecificOutput.additionalContext` |
| `json` | `--emit=json` | `{"context":"…"}` |

El presupuesto de recorte es ajustable con `--budget` para agentes cuyo canal tenga otro tope; el
valor por defecto (9 500) corresponde al tope de 10 000 documentado para hooks de Claude Code.

Mismo requisito que el otro hook nuevo: registrar el subcomando en el filtro de idempotencia en la
misma tarea, o la reinstalación duplica la entrada.

## Entrada (stdin, JSON)

```json
{ "hook_event_name": "PostToolUse", "tool_name": "EnterPlanMode", "session_id": "…", "cwd": "…" }
```

No se necesita ningún campo del payload para producir la salida: el documento se compone del método
embebido más el contexto del proyecto resuelto desde el directorio de trabajo. Un payload vacío es
válido.

## Salida (stdout, JSON) — exit code **siempre 0**

### Caso A — inyectar el documento de planificación

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "additionalContext": "<método de descomposición atómica completo>\n\n<historial recortado>\n\n<puntero al resto>"
  }
}
```

### Caso B — silencio

```json
{}
```

Se emite cuando la planificación atómica está apagada (`atomic_plan_disabled`), cuando no se puede
resolver el proyecto, o ante cualquier error interno.

## Presupuesto del canal (FR-007)

| Magnitud | Valor |
|---|---|
| Tope duro del canal | 10 000 caracteres |
| Tope aplicado | **9 500** (margen de 500) |
| Tamaño del método | ~4 200 caracteres |
| `Budget` por defecto del contexto | 24 000 caracteres |

**Orden de prioridad, estricto**:

1. El **método completo**, siempre. Nunca se recorta: su final contiene la autoverificación y el
   formato de salida, que son la parte operativa.
2. El **historial**, con los caracteres que queden.
3. Una línea final indicando cómo recuperar el material omitido (`get_plan_context()`).

**Regla de corte**: el recorte cae en el último límite de párrafo que quepa; si no hay ninguno, en el
último límite de línea. Nunca a mitad de frase. Si el método por sí solo no cupiera, se emite el
método y se omite el historial por completo, con el puntero.

## Efectos de estado

- Reinicia `PlanEpisodeState` a `denials = 0`: entrar en modo plan abre un episodio nuevo.
- No escribe en la base de datos.

## Invariantes verificables

1. La salida **nunca** excede 9 500 caracteres.
2. El método aparece **íntegro** en toda salida del Caso A.
3. Ningún corte a mitad de frase.
4. Exit code 0 en todos los caminos; sin memoria inicializada → Caso B, sin error visible (FR-009).
5. Reentrar en modo plan en la misma sesión no vuelve a emitir el bloque completo (FR-008): la segunda
   entrada consecutiva emite solo el reinicio de episodio y una referencia corta.
6. No toca nada del brazo extensor (INV-1).

## Degradación declarada

Si el agente no acepta contexto en este evento, o entra en modo plan sin llamada a herramienta, el
canal se marca `missing` / `not_applicable` en el reporte de cobertura y la garantía la cubre el
recordatorio por turno. La Historia 1 (determinismo) **no** depende de este hook.

## Pruebas de contrato exigidas

- Documento resultante ≤ 9 500 caracteres con un contexto de 24 000 caracteres simulado.
- El método aparece completo (primera y última línea presentes) en ese caso.
- Sin proyecto resoluble → `{}` y exit 0.
- Con `atomic_plan_disabled` → `{}` y exit 0.
- Segunda invocación consecutiva → salida corta, no el bloque completo.
- El contador de episodio queda en 0 después de invocarlo.
