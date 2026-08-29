# Contratos — Tools MCP nuevas (Fase 1)

gomemory expone su funcionalidad vía MCP (Principio V.9 de la constitución). Estas tools son nuevas — no modifican ninguna existente — y se registran junto a las demás en `adapters/primary/cli/cmd_mcp_review_tools.go`, siguiendo el mismo patrón que `registerCodeTools`. Ninguna ejecuta un modelo por sí misma: cada llamada persiste y valida un paso del protocolo que el agente orquestador ya resolvió.

## `review_start`

Congela un target y abre una revisión (FR-001..004).

- **Entrada**: `target_type`, `revision`, `digest`, `scope[]`, opcional `reviewer_a{provider,model}`, `reviewer_b{provider,model}`, opcional `max_fix_rounds`, `auto_fix_severities[]`.
- **Salida**: `review_id`, `independence{level,reason}`.
- **Invariante de contrato**: `digest` vacío ⇒ error, ninguna fila se crea. Con `reviewer_a.provider+model == reviewer_b.provider+model` ⇒ `independence.level = "degraded"`, `reason = "same-model"` (nunca `"full"`).

## `review_submit`

Registra el resultado de un revisor (éxito con hallazgos, o fallo) para la ronda activa (FR-005..013, FR-039, FR-030).

- **Entrada**: `review_id`, `reviewer` (`A`|`B`), `target_digest` (debe igualar al congelado), `status` (`success`|`failure`), `findings[]` (`local_id, location, severity, category, claim, evidence_class, evidence[], confidence`).
- **Salida**: confirmación de almacenamiento; si ambos revisores de la ronda ya tienen resultado, indica que la revisión está lista para `review_consensus`.
- **Invariante de contrato**: `target_digest` distinto al congelado ⇒ error `"target changed"`, la ronda no avanza (AC-003). Reenviar el mismo `(review_id, reviewer, round, local_id)` actualiza esa fila, nunca duplica (idempotencia, FR-039). `status: "failure"` de cualquiera de los dos revisores fija de inmediato el camino hacia `INCOMPLETE` (FR-030) — no se acepta un tercer intento del mismo reviewer en la misma ronda.

## `review_consensus`

El agente orquestador propone la clasificación de equivalencia entre hallazgos de A y B; esta tool valida y persiste (FR-014..018, INV-002, INV-004).

- **Entrada**: `review_id`, `matches[]` de `{status: CONFIRMED|CONTRADICTION, finding_id_a, finding_id_b, severity, claim}` y `unmatched[]` de `{status: SUSPECT|INFO, finding_id}`.
- **Salida**: `consensus_findings[]` persistidos con su `consensus_local_id`, o los rechazos con motivo.
- **Invariante de contrato**: un `CONFIRMED`/`CONTRADICTION` cuyos dos `finding_id` no pertenezcan a revisores distintos (`A` y `B`) de la misma ronda del mismo `review_id` se **rechaza** (INV-002/INV-004) — el motor de consenso nunca fabrica una equivalencia que el agente no sustentó con dos fuentes reales. No requiere ambos resultados de revisor completos, salvo que sea así por diseño de `review_submit` (ver arriba).

## `review_fix_record`

Registra una corrección aplicada fuera de GoMemory, atada a hallazgos confirmados y autorizados (FR-019..022, INV-006, INV-007, INV-009).

- **Entrada**: `review_id`, `addressed_consensus_ids[]`, `base_target_digest`, `fixed_target_digest`, `modified_paths[]`, `verification[]`, `diff_digest`, opcional `explicit_authorization: true` (solo para autorizar un `consensus_finding` fuera de `auto_fix_severities`).
- **Salida**: `round` asignado.
- **Invariante de contrato**: rechaza si algún `addressed_consensus_ids` no es `CONFIRMED`, si su severidad no está en `auto_fix_severities` sin `explicit_authorization`, o si el siguiente número de ronda excede `max_fix_rounds` (INV-009 — no hay forma de pedir una ronda extra desde la entrada).

## `review_finalize`

Deriva el estado terminal a partir de lo persistido (FR-026..030, FR-042).

- **Entrada**: `review_id`.
- **Salida**: `verdict` (`APPROVED`|`ESCALATED`|`INCOMPLETE`), `metrics{duration, findings_total, findings_confirmed, findings_suspect, contradictions, fix_rounds, memory_promoted, memory_deduplicated}`.
- **Invariante de contrato**: el veredicto es siempre el resultado de `domain.DeriveVerdict` sobre el estado ya persistido — la tool no acepta un `verdict` como parámetro de entrada bajo ninguna forma (INV-010: una revisión incompleta nunca puede convertirse en `APPROVED` porque el agente lo pida).

## `review_status`

Consulta de solo lectura (Historia de usuario 4).

- **Entrada**: `review_id`.
- **Salida**: `status`, `round`, resumen de hallazgos por estado, sin ejecutar ninguna transición.

## Exclusiones deliberadas (FR-031, FR-041)

Ninguna tool de este contrato acepta ni persiste: prompts en bruto, cadena de razonamiento, transcripts completos de revisor, secretos o credenciales. Un campo de texto libre (`claim`, `verification`) que contenga un secreto detectable se redacta igual que en `save_memory` (mismo helper de redacción ya existente).
