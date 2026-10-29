# Evidencia T122 — quickstart con el binario de la rama

Fecha: 2026-10-28. Binario compilado desde la rama (`go build ./infrastructure`), `HOME` y proyecto temporales. Ampliado el 2026-10-28 con las ejecuciones reales de Claude Code, Codex y OpenCode. Dos subpuntos solo se cubren con pruebas automáticas y se indican en la tabla.

| Escenario | Qué se ejecutó | Cómo | Resultado |
|---|---|---|---|
| 1 — Recorrido mínimo (SC-001) | `start → … → close` | `TestHCDBinaryMinimalFlow` (binario) y agentes reales: Claude Code (`mvp-claude.md`), Codex (`env-codex.md`), OpenCode (`env-opencode.md`) | Pasa |
| 2 — Invariantes (C-11, C-17, C-18) | `define` repetido con mismo id → mismo resultado sin avanzar; otro contenido → `operation_conflict` (exit 4); revisión antigua → `revision_conflict` (exit 4) sin cambiar el archivo; `status/next/show/resume` no cambian la revisión | `TestHCDQuickstartInvariantsStaleClarificationAndCoexistence` | Pasa |
| 2 — C-19 (ejecución incierta) | Vencimiento de concesión y reconciliación | Pruebas automáticas (`hcd_uncertain_test.go`). Con el binario exige esperar la concesión mínima de 1 hora | Cubierto solo por pruebas automáticas |
| 3 — Evidencia que caduca (C-09, C-22) | Edición manual de `definition.md`: la consulta no escribe; la primera mutación responde `revision_conflict` y deja la unidad en `draft` sin aprobación; `close` se rechaza | mismo test | Pasa |
| 4 — Aclaración (FR-016a) | Pregunta bloqueante → `approve` se rechaza; tras responderla, `approve` → `ready` | mismo test | Pasa (sin comprobar la memoria `type=decision` con `topic_key`; la cubre `hcd_memory_test.go`) |
| 5 — Memoria y retomar (C-10, C-12, C-13) | Retomar en otra sesión y otro agente; fallo de memoria y `memory sync` | Retomar: Claude Code retomó con `/hcd` la unidad abierta por OpenCode, con paso, decisiones y siguiente acción en una respuesta (`focus-heartbeat.md`). Fallo de memoria: `TestHCDMemorySyncRetriesWithoutDuplicates`; con el binario no se aísla porque memoria y control comparten base | Pasa (fallo de memoria solo con pruebas automáticas) |
| 6 — Coexistencia (C-02, C-24) | `define` sobre `specs/044-legado` → `approach_mismatch`; `specs/` sin cambios | mismo test + `evidence/speckit-untouched.md` | Pasa |
| 7 — Entornos reales (SC-008) | Recorrido completo en Codex y OpenCode | `env-codex.md`, `env-opencode.md` | Pasa |
| 8 — Unidad activa y heartbeat (SC-011) | Los cinco pasos alternando los tres agentes | `focus-heartbeat.md` | Pasa (`$hcd` en Codex limitado por el `mem` global del entorno) |
| 9 — Los tipos de unidad (SC-002) | feature, evolution, technical hasta `close` | `TestHCDKindsEndToEnd` → `evidence/kinds-e2e.md` | Pasa |
| Migración (SC-009) | preview + activate sobre copia de `specs/044-…` | binario → `evidence/migration-044.md` | Pasa |

## Diferencias entre el quickstart y el motor

Resueltas el 2026-10-28: el quickstart ya documenta `DEF-PENDING` (escenario 1, paso 2), que `status` es de solo lectura y la edición la detecta la primera mutación (escenario 3) y que `--timeout` recibe horas enteras (escenario 2). No queda ninguna diferencia conocida.
