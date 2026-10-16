# Contrato MCP: `socratica_flow`

Una sola herramienta con un campo `action`. No se publica una colección de herramientas (FR-031). Se registra junto a las de revisión en `cmd_mcp_*`, con su esquema en `cmd_mcp_schemas.go`. Los textos de entrada pasan por `ProtectText` antes de persistirse y las respuestas también.

## Entrada

```json
{
  "action": "start | answer | rewrite | edit | approve | ack | cancel | fail | retry | send_original | resume | mark_dispatched | force_resend | get | history | settings",
  "flow_id": "string (todas salvo start/history/settings)",
  "dispatch_id": "string (ack, mark_dispatched, force_resend)",
  "ack_nonce": "string aleatorio generado por el agente (ack); repetir el ack con el mismo valor es idempotente",
  "client": "claude | codex | opencode (obligatorio en start/history/settings)",
  "mode": "refine | scrutiny (start)",
  "prompt": "texto original (start) | texto editado (edit)",
  "questions": [{"text": "…", "options": ["…", "…"]}],
  "answers": [{"index": 0, "choice": "…"} , {"index": 1, "custom": "…"}, {"index": 2, "skipped": true}],
  "skip_rest": false,
  "proposed": "texto (rewrite)",
  "additions": ["…"],
  "error": "texto llano (fail)",
  "scrutiny": {"restatement": "", "assumptions": [{"text": "…", "kind": "fact|hypothesis"}], "strongest_support": "", "strongest_objection": "", "alternatives": [], "missing_evidence": [], "decision_criteria": [], "hypotheses": []},
  "limit": 20
}
```

## Salida

```json
{
  "flow_id": "…",
  "phase": "captured | questions | rewriting | review | approved | dispatched | failed | cancelled",
  "original": "…",
  "proposed": "…",
  "dispatch_id": "… | null",
  "duplicate": false,
  "acked": false,
  "already_dispatched": false,
  "in_flight": false,
  "recovered": false,
  "uncertain": false,
  "summary": {"words_before": 0, "words_after": 0, "additions": []},
  "scrutiny": { … } ,
  "next": "instrucción breve para el agente sobre el siguiente paso permitido",
  "history": [ … ]   // solo action=history
}
```

## Reglas

- `start` con un flujo no terminal ya abierto para el mismo `(project, client)` devuelve ese flujo con `recovered=true`. No crea uno nuevo.
- Como máximo hay un flujo no terminal por `(project, client)`, garantizado por el índice único parcial. Para empezar de cero, la persona cancela primero (`action=cancel`).
- `start` con `mode=scrutiny` y `rewrite` aceptan el campo `scrutiny` (objeto de `generation.schema.json`), que se persiste en `scrutiny_json` y se devuelve en `get`.
- `questions` se valida con el parser (R6). Lo que no se puede reparar devuelve un error y el flujo pasa a `failed`.
- **Protocolo de despacho desde el agente (I8):** (1) `approve` → `dispatch_id`; (2) `ack(flow_id, dispatch_id, ack_nonce)` **antes** de ejecutar: es la compuerta atómica (`owner` vacío, exige que no haya reclamo local) y solo quien recibe `acked=true` ejecuta, una vez, exactamente `approved_prompt`; (3) si `ack` responde `already_dispatched=true` o un error de rechazo, **NO ejecutes**. Si el agente cae entre approve y ack, no ejecutó nada: `resume` devuelve el flujo a review. Si pierde la respuesta del ack, lo repite con el **mismo** `ack_nonce` y recibe otra vez `acked=true` sin consumir otra autorización. Si cae después del ack, la autorización ya está consumida (`dispatched`): otra sesión recibe `already_dispatched=true` y no ejecuta; si la persona confirma que la tarea no llegó a empezar, puede pedir `force_resend` desde dispatched (autorización nueva, ver «Garantía de despacho» en `data-model.md`). gomemory nunca afirma que la tarea se ejecutó.
- `approve` es idempotente. Sobre approved sin ack devuelve `duplicate=true`, `acked=false` y el mismo `dispatch_id`, y `next` dirige al `ack`, sin prohibir. Sobre dispatched devuelve `duplicate=true`, `already_dispatched=true` y `next` dice «NO ejecutes: ya fue enviado». `send_original` desde `failed` aprueba con el texto original.
- `ack` exige que el `dispatch_id` coincida; con uno antiguo (anterior a un `force_resend`) se rechaza.
- Un flujo approved con reclamo y sin ack se devuelve con `uncertain=true`; `next` dice «NO ejecutes: puede que ya se haya enviado; pregunta a la persona» y solo admite `mark_dispatched`, `force_resend` (elección explícita, genera un `dispatch_id` nuevo) o `cancel`, todas con el `dispatch_id` vigente y solo a petición explícita de la persona. Con el lock retenido responden `in_flight=true` sin cambios. Tras `force_resend`, el agente sigue el protocolo normal: `approve` (duplicate con `acked=false`) → `ack` con el `dispatch_id` nuevo → ejecutar.
- `get` es de **solo lectura**: devuelve el estado tal cual, con `recoverable=true` si el flujo está `approved` sin `ack` ni reclamo, y con `uncertain=true` si tiene reclamo sin `ack`.
- `resume` aplica el evento `resume` de `data-model.md`: approved sin ack **ni reclamo** → review con `recovered=true`; approved con reclamo sin ack → approved con `in_flight=true` (lock retenido) o `uncertain=true` (lock libre), nunca review (nunca reenvía); captured/rewriting → `next` pide **al agente activo** que vuelva a generar preguntas o reescritura. Por MCP nunca se llama a `SocraticGenerator` (FR-009).
- **`next` por estado (I11)**, el mismo en `get`, `resume` y `approve`:
  - **revisión recuperable** (review con `recovered=true`, o approved sin ack ni reclamo): «ofrece Ejecutar (approve → ack → ejecutar) o Cancelar; cancelar libera el pendiente» (G1).
  - **envío en curso** (`in_flight=true`): «NO ejecutes ni ofrezcas acciones: otro proceso está despachando; vuelve a consultar con get».
  - **envío incierto** (`uncertain=true`): «NO ejecutes: puede que ya se haya enviado; pregunta a la persona: marcar como enviado, reenviar o cancelar».
  - **autorización consumida** (`dispatched`, `already_dispatched=true`): «NO ejecutes: ya fue enviado; si la persona confirma que la tarea no empezó, ofrece reenviar (`force_resend`)».
  - **listo para confirmar** (approved sin ack, recién aprobado o duplicado con `acked=false`): «llama a ack con este dispatch_id y un ack_nonce antes de ejecutar».
- `settings` es de solo lectura: devuelve intercepción, capacidades y modo privado del cliente. Cambiar la intercepción no se expone por MCP (FR-018/FR-031). Se hace por CLI o TUI; `/socratica on|off` dentro del agente devuelve la orden exacta `mem socratica on|off --client <agente>`. Así se evita que un modelo active la captura por su cuenta.
- Con `socratica_private=true` no se escribe historial. `ack` **redacta** la fila (`EffectRedactFlow`: vacía los textos y conserva `dispatch_id`, `approved_hash` y `dispatch_consumed` para que el hook reconozca una vez el prompt aprobado; se purga con `TerminalRetention`). `cancel` borra la fila, porque no hay despacho que reconocer.
- Las acciones de generación nunca llaman a `SocraticGenerator` desde MCP: el caso de uso se invoca con `OriginMCP` y devuelve `next` para el agente activo (FR-009).
- Un ajuste `socratica_*` inválido devuelve un error MCP con la clave y el valor (`ErrInvalidSocraticaSettings`); no se aplican defaults sobre valores presentes.
- Si no hay memoria inicializada, devuelve `{"phase":"unavailable"}` y la habilidad sigue en modo texto sin persistencia.
