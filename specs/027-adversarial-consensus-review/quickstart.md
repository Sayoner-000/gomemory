# Guía de validación: Revisión Adversarial por Consenso

Escenarios ejecutables para comprobar el protocolo de punta a punta, referenciando los contratos (`contracts/mcp-tools.md`, `contracts/cli-contracts.md`) y el modelo de datos (`data-model.md`) en vez de repetirlos.

## Prerrequisitos

- Proyecto con `gomemory` instalado y `.memory/` inicializado (`mem init` o inicialización perezosa habitual).
- Un target de prueba con un defecto deliberado y reproducible (por ejemplo, un `git diff` local con una condición de carrera introducida a propósito).
- Acceso a un agente/skill orquestador capaz de ejecutar dos evaluaciones independientes (pueden ser dos llamadas aisladas al mismo modelo si no hay diversidad de proveedor disponible — ver `research.md`, Decisión 3).

## Escenario 1 — Flujo aprobado (Historia de usuario 1 y 2; AC-001, AC-004, AC-005)

1. `mem review --diff` sobre el cambio con el defecto deliberado → anota el `review_id` impreso.
2. El agente ejecuta el Revisor A (solo lectura) sobre el target congelado y reporta sus hallazgos con `review_submit` (`reviewer: A`).
3. El agente ejecuta el Revisor B de forma aislada (sin ver el resultado de A) y reporta con `review_submit` (`reviewer: B`).
4. El agente compara ambos conjuntos de hallazgos y llama a `review_consensus` proponiendo el match correspondiente al defecto compartido como `CONFIRMED`.
5. **Resultado esperado**: `review_consensus` devuelve un `consensus_finding` `CONFIRMED` de severidad HIGH/CRITICAL referenciando los dos hallazgos de origen (AC-001).
6. El agente ejecuta el Fix Actor recibiendo únicamente ese hallazgo confirmado, su evidencia y las reglas del proyecto, corrige el mínimo necesario y llama a `review_fix_record` (AC-004).
7. El agente ejecuta una nueva ronda de Revisor A y Revisor B, acotada a verificar la resolución (`review_submit` con `round: 1`), y confirma la resolución con `review_consensus`/estado `RESOLVED`.
8. `mem review status <review_id>` / `review_finalize` → **verdict: APPROVED**.
9. `mem review show <review_id>` → la cadena completa (hallazgo → consenso → fix → re-revisión → veredicto) es reconstruible sin acceso directo a `mem.db`.
10. Consultar `get_context()` en una sesión nueva → el patrón de fallo aparece como conocimiento reutilizable (`review_learning`), sin transcript ni cadena de razonamiento (Historia de usuario 3; AC-008).

## Escenario 2 — Flujo escalado (AC-006)

1. Repetir los pasos 1–6 del Escenario 1, pero con una corrección deliberadamente insuficiente.
2. La re-revisión (`round: 1`) marca el hallazgo como `UNRESOLVED`.
3. Repetir una segunda ronda de corrección (`round: 2`, el `max_fix_rounds` por defecto) igualmente insuficiente.
4. Intentar una tercera ronda vía `review_fix_record` → **rechazada** (excede `max_fix_rounds`, INV-009).
5. `review_finalize` → **verdict: ESCALATED**, sin que se haya añadido ninguna ronda fuera del presupuesto.

## Escenario 3 — Incompleto por fallo de revisor (AC-007)

1. Iniciar una revisión y reportar éxito para el Revisor A.
2. Reportar `status: "failure"` para el Revisor B (o dejarlo sin responder).
3. `review_finalize` → **verdict: INCOMPLETE**, nunca `APPROVED` (FR-030).

## Escenario 4 — Hallazgo sospechoso no se autocorrige (AC-002)

1. Reportar un hallazgo solo desde el Revisor A, sin equivalente en B.
2. `review_consensus` lo clasifica como `SUSPECT`.
3. Intentar `review_fix_record` referenciándolo sin `explicit_authorization` → **rechazado**.

## Escenario 5 — Deduplicación de memoria (AC-009)

1. Completar el Escenario 1 una vez (memoria `review_learning` promovida con `topic_key` derivado de categoría+componente).
2. Repetir el Escenario 1 con un defecto del mismo patrón (misma categoría/componente).
3. **Resultado esperado**: la memoria existente se actualiza/refuerza (mismo `id`), no se crea una segunda fila — verificable con `search_memories`/`list_memories`.

## Verificación técnica de soporte

- `go test ./domain/... ./application/usecases/...` — cubre `DeriveVerdict`, elegibilidad de corrección y presupuesto de rondas como funciones puras.
- `go test ./tests/contract/...` — esquemas de las tools MCP y comandos CLI de este documento.
- `go test ./tests/integration/...` — Escenarios 1 y 2 completos (`review_approved_flow_test.go`, `review_escalated_flow_test.go`).
