# Contrato — `mem hook plan-guard` (borde de salida del plan)

Exige el contrato de forma del plan **antes** de que el plan llegue a la persona. Es el único
mecanismo determinista de la feature (Historia 1, FR-001..FR-004).

> **Este documento describe la traducción a Claude Code.** La definición de la capacidad, neutral y
> válida para cualquier agente, está en [agent-integration.md](./agent-integration.md); lo de aquí es
> uno de sus dialectos. Si los dos documentos discrepan, manda el neutral.

## Registro

| Agente | Evento | Matcher | Comando |
|---|---|---|---|
| Claude Code | `PreToolUse` | `ExitPlanMode` | `mem hook plan-guard` |
| Cualquier otro | invocación directa antes de presentar el plan | — | `mem hook plan-guard` |

## Dialectos de salida

El veredicto es el mismo en todos; solo cambia cómo se expresa «devolver el plan».

| Dialecto | Selección | Devolver | Permitir |
|---|---|---|---|
| `neutral` | **por defecto**, cuando no hay envoltura reconocible | código ≠ 0 + motivo por stderr | código 0, sin salida |
| `claude` | payload con la envoltura de eventos de Claude Code, o `--emit=claude` | JSON de `deny` (abajo), código 0 | `{}`, código 0 |
| `json` | `--emit=json` | `{"decision":"deny","reason":"…"}`, código 0 | `{"decision":"allow"}`, código 0 |
| `text` | `--emit=text` | motivo por stdout, código 0 | sin salida, código 0 |

**Regla de neutralidad**: ante la duda se responde en `neutral`, nunca en el dialecto de un agente
concreto.

Se registra en el ámbito de proyecto (`<root>/.claude/settings.json`) **y** en el de usuario
(`~/.claude/settings.json`), preservando toda entrada ajena. El subcomando debe quedar reconocido por
el filtro de idempotencia en la misma tarea que lo registra, o cada reinstalación duplicará la
entrada.

## Entrada (stdin, JSON)

Se aceptan las dos formas, igual que hace `plan-approved`:

```json
{ "hook_event_name": "PreToolUse", "tool_name": "ExitPlanMode", "tool_input": { "plan": "…" } }
```

```json
{ "plan": "…" }
```

**Resolución**: `tool_input.plan` primero; si no existe, `plan` de nivel superior. Si ninguno aporta
texto → permitir en silencio.

## Salida (stdout, JSON) — exit code **siempre 0**

### Caso A — permitir (silencio)

Cuando el veredicto es `ShapeOK` o `ShapeNotApplicable`, cuando el contador de episodio ya registra
una devolución, cuando la exigencia está apagada, o ante cualquier error interno:

```json
{}
```

### Caso B — devolver el plan

Solo cuando el veredicto es `ShapeMissing` **y** el contador de episodio está en cero:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "El plan no cumple el contrato de forma del proyecto: falta el árbol de tareas atómicas. Cada hoja debe ser un verbo + objeto con un resultado verificable (`[1.1] verbo + objeto → resultado`). Llama a get_plan_context() si necesitas el método y el historial, y presenta el plan otra vez. Este aviso se emite una sola vez por plan."
  }
}
```

**Requisitos del motivo**:

- Dice qué falta y qué hacer, en una sola lectura. No cita reglas por número.
- Menciona la capacidad que resuelve el problema (`get_plan_context()`).
- Declara que no se repetirá, para que el agente no entre en un bucle defensivo.
- Cabe holgadamente en el tope de 10 000 caracteres del canal.

## Efectos de estado

| Situación | Efecto en `PlanEpisodeState` |
|---|---|
| Caso A | ninguno |
| Caso B | `denials := 1` |

## Invariantes verificables

1. **Nunca falla hacia el bloqueo**: entrada ilegible, marcador corrupto, error de disco o veredicto
   dudoso → Caso A.
2. **Como máximo una devolución por episodio** (FR-002).
3. **Nunca sobre planes triviales** (FR-003).
4. **Apagable**: con `plan_guard_disabled` o `atomic_plan_disabled` activos, siempre Caso A y sin
   escrituras de estado (FR-004).
5. **Sin base de datos**: la ruta de este hook no abre SQLite. Presupuesto < 50 ms.
6. **Código de salida**: 0 en todos los caminos de los dialectos que transportan la decisión en la
   salida (`claude`, `json`, `text`), incluido el Caso B. Distinto de 0 **solo** en el dialecto
   `neutral`, donde el código *es* el vehículo de la decisión por contrato. Ningún fallo ambiental
   produce nunca un código distinto de 0.
7. **No toca nada del brazo extensor**: no lee ni escribe su configuración, y no restringe ninguna
   herramienta de exploración (INV-1, INV-3).

## Pruebas de contrato exigidas

- Tabla de veredictos: plan en árbol con glifos → A; plan con identificadores jerárquicos → A; plan en
  prosa larga → B; plan de dos líneas → A; plan vacío → A; plan en inglés con árbol → A.
- Payload en forma `tool_input.plan` y en forma `plan` de nivel superior → mismo resultado.
- Segunda invocación consecutiva con el mismo plan en prosa → A (idempotencia por episodio).
- Tras `plan-approved`, un plan en prosa vuelve a producir B (episodio nuevo).
- Con el interruptor apagado → A y sin marcador escrito.
- Payload basura (JSON inválido) → A y exit 0.
- Texto plano por stdin (sin JSON) → mismo veredicto que la forma `{"plan":"…"}`.
- Sin envoltura reconocible y sin `--emit` → respuesta en `neutral` (código ≠ 0 + stderr), no en el
  dialecto de Claude Code.
- El mismo plan evaluado con `--emit=neutral`, `--emit=json`, `--emit=text` y `--emit=claude` produce
  el mismo veredicto en las cuatro formas documentadas.
