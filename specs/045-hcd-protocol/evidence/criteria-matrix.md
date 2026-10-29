# Matriz de criterios 045 (T121)

Fecha: 2026-10-28. «Resultado» refleja `go test ./...` completo de esta fecha (ver `full-suite.md`) y, donde se indica, evidencia de ejecución. Una fila cuyo resultado es **Parcial** o **Pendiente** declara qué falta; ninguna queda sin cobertura asignada.

## Criterios C-01 … C-24

| ID | Qué exige | Prueba o escenario que lo cubre | Evidencia | Resultado |
|---|---|---|---|---|
| C-01 | Ceremonia de Especificación ofrece HCD y Spec Kit; se inicializa solo la opción elegida | `adapters/primary/tui` (pantalla `specification`, T095) · `tests/integration/hcd_adapters_equivalence_test.go` · `TestSocraticaSpecifyDoesNotTouchHCD` (la rama Spec Kit no crea filas `hcd_*`) | suite completa | Cubierto |
| C-02 | Unidad Spec Kit existente continúa en Spec Kit, sin migración automática | `application/usecases/hcd_approach_test.go` · `TestHCDMigratePreviewClassifiesEveryItem…` | `evidence/speckit-untouched.md` §3 (45/45 `speckit`) | Cubierto |
| C-03 | Definición incompleta: la validación enumera cada faltante y sigue en `draft` | `domain/hcd_definition_test.go` (incl. `…TemplatePlaceholdersArePending`) · `application/usecases/hcd_flow_test.go` · `TestHCDKindsEndToEnd` | `evidence/kinds-e2e.md` (`DEF-PENDING@campo`, estado `draft`) | Cubierto |
| C-04 | Aprobación vinculada a la revisión exacta; unidad pasa a `ready` | `hcd_flow_test.go` (`…ApproveRequiresExactValidatedRevisionAndQuote`) | `evidence/kinds-e2e.md` (`approve → ready`) | Cubierto |
| C-05 | Tarea sin vínculos se rechaza indicando referencias | `application/usecases/hcd_delivery_test.go` (`…CheckDetectsOrphan…`) · `domain/hcd_delivery_test.go` | suite completa | Cubierto |
| C-06 | Tarea con varios resultados o decisiones pendientes no se admite | `hcd_delivery_test.go` · `domain/hcd_delivery_test.go` (`ATOM-*`) | suite completa | Cubierto |
| C-07 | Slice preparado se ejecuta sin detallar los futuros | `hcd_delivery_test.go` (`…RegistersNextSlice…`) | suite completa | Cubierto |
| C-08 | Tarea implementada sin evidencia suficiente no pasa a verificada | `domain/hcd_evidence_test.go` · `application/usecases/hcd_verify_test.go` | suite completa | Cubierto |
| C-09 | Cambio de una entrada relevante deja la verificación pendiente de revalidación | `hcd_evidence_test.go` (`…EvidenceFreshness`) · `hcd_verify_test.go` (`…DefinitionChangeKeepsLateReportStale`, `…ChangedSubjectPreventsClose`) · `hcd_execution_test.go` (`TestHCDRunRevalidatesOnlyStaleEvidence`) · `tests/contract/hcd_engine_contract_test.go` | suite completa | Cubierto |
| C-10 | Retomar desde otra sesión recupera estado, reglas y decisiones | `hcd_flow_test.go` · `build_context_hcd_test.go` · `tests/contract/hcd_connected_test.go` | `evidence/focus-heartbeat.md` (Claude Code retomó la unidad de OpenCode; Codex recibió «HCD activo») | Cubierto |
| C-11 | Las consultas no ejecutan tareas ni consumen autorizaciones | `hcd_flow_test.go` (`…QueriesAndResume…`) · `tests/contract/hcd_engine_contract_test.go` | suite completa | Cubierto |
| C-12 | Decisión aceptada se persiste en memoria con su contexto | `application/usecases/hcd_memory_test.go` (`…DecisionAcceptanceAndSupersedesMirror`) | suite completa | Cubierto |
| C-13 | Fallo de persistencia en memoria es visible y el reintento no duplica | `hcd_memory_test.go` (`…MemorySyncRetriesWithoutDuplicates`) | suite completa | Cubierto |
| C-14 | Excepciones con responsable y vencimiento; la vencida bloquea el cierre | `application/usecases/hcd_governance_test.go` · `domain/hcd_governance_test.go` | suite completa | Cubierto |
| C-15 | Una revisión nueva del Playbook no cambia la unidad sin `adopt` | `hcd_governance_test.go` · `tests/integration/hcd_playbook_adopt_test.go` | suite completa (el integration reveló y cerró el bug de `--expected-revision`) | Cubierto |
| C-16 | Dos adaptadores aplican las mismas precondiciones y producen el mismo resultado | `tests/integration/hcd_adapters_equivalence_test.go` | suite completa | Cubierto |
| C-17 | Mutación con revisión esperada antigua se rechaza sin pérdida | `hcd_flow_test.go` · `hcd_verify_test.go` (`ErrRevisionConflict`) · `tests/contract/hcd_engine_contract_test.go` (exit 4) | suite completa | Cubierto |
| C-18 | Operación repetida devuelve el resultado original; contenido distinto → conflicto | `hcd_execution_test.go` · `hcd_verify_test.go` (`ErrOperationConflict`) · `hcd_delivery_test.go` | suite completa | Cubierto |
| C-19 | Ejecución con respuesta perdida queda incierta, sin éxito ni repetición automática | `application/usecases/hcd_uncertain_test.go` | suite completa. No se forzó en los agentes reales: la concesión mínima es de 1 hora | Cubierto solo por pruebas automáticas |
| C-20 | Propuesta de migración no activa HCD; la unidad sigue en Spec Kit | `application/usecases/hcd_migrate_test.go` (`…PreviewClassifiesEveryItem…`) | `evidence/migration-044.md` pasos 0–1 | Cubierto |
| C-21 | Propuesta aprobada se activa con las fuentes conservadas y un solo enfoque | `hcd_migrate_test.go` (`…ActivateKeepsSourcesAndSingleApproach`, `…FailureLeavesSingleRecoverableApproach`) | `evidence/migration-044.md` pasos 3–5 | Cubierto |
| C-22 | Bloqueo crítico, control fallido o excepción vencida impiden cerrar | `domain/hcd_evidence_test.go` (`…ClosureBlockers`) · `hcd_verify_test.go` | suite completa | Cubierto |
| C-23 | Cierre solo con controles satisfechos y aceptación de la revisión exacta | `hcd_verify_test.go` (`…CloseRequiresApprovedDefinitionRevision`, `…VerifyPassDoesNotCloseAndCloseReleasesFocus`) | `evidence/kinds-e2e.md` (`close → completed`) | Cubierto |
| C-24 | Spec Kit no cambió | `git diff` contra `eb26453` · huellas `.opencode` · fechas y huellas de `.claude`/`.agents` | `evidence/speckit-untouched.md` · `evidence/constitution-18-applied.md` | Cubierto (la constitución cambia solo por la §18/§25.2 aprobadas, excepción de FR-006; la plantilla distribuida recibe la misma §18 sin la §25.2) |

## Criterios de éxito SC-001 … SC-011

| ID | Qué exige | Prueba o escenario que lo cubre | Evidencia | Resultado |
|---|---|---|---|---|
| SC-001 | Camino mínimo `fix` con `/hcd` y skills `hcd-*` en un agente | `TestHCDBinaryMinimalFlow` · escenario 1 | `evidence/mvp-claude.md` (Claude Code, 13 turnos, `start` → `completed`) · `evidence/env-codex.md` (Codex) | Cubierto (2026-10-28) |
| SC-002 | Los seis escenarios de extremo a extremo | `TestHCDKindsEndToEnd` · `TestHCDBinaryMinimalFlow` · migración | `kinds-e2e.md` · `migration-044.md` · `focus-heartbeat.md` (unidad retomada desde otro entorno) | Cubierto |
| SC-003 | Los 24 criterios con prueba vinculada y evidencia | esta matriz | — | Cubierto (C-19 solo con pruebas automáticas) |
| SC-004 | Unidades Spec Kit abiertas con su enfoque, cero bytes modificados | `hcd_approach_test.go` · `git diff` | `evidence/speckit-untouched.md` §1 y §3 | Cubierto |
| SC-005 | Ninguna consulta cambia estado, revisión ni autorizaciones | `hcd_flow_test.go` (`…QueriesAndResume…`) · contrato | suite completa | Cubierto |
| SC-006 | Idempotencia y rechazo de revisión obsoleta sin pérdida | pruebas de C-17 y C-18 | suite completa | Cubierto |
| SC-007 | Retomar con una sola invocación presenta paso, decisiones y siguiente acción | `build_context_hcd_test.go` · `hcd_flow_test.go` | `focus-heartbeat.md` (paso 1b) | Cubierto |
| SC-008 | Solo se declara compatible un entorno con evidencia real | `README.md`, guía técnica y guía SDD_HCD declaran Claude Code y Codex con su evidencia; OpenCode según `env-opencode.md` | `evidence/mvp-claude.md` · `evidence/env-codex.md` · `evidence/env-opencode.md` | Cubierto |
| SC-009 | Migración con 100 % de elementos clasificados y 0 tareas verificadas | `hcd_migrate_test.go` · `domain/hcd_migrate_test.go` | `evidence/migration-044.md` (105/105, 0 verificadas) | Cubierto |
| SC-010 | Menos artefactos obligatorios que Spec Kit en una corrección pequeña | Por construcción: HCD exige `definition.md`, `delivery.md` y `verification.md` generado (3) frente a spec, plan, tasks y checklist (4) | `TestHCDBinaryMinimalFlow` solo crea esos tres | Cubierto por construcción |
| SC-011 | «HCD activo» en el 100 % de los arranques con unidad activa en los tres agentes; nada fuera de `hcd/` cambia | `build_context_hcd_test.go` · `tests/integration/hcd_no_agent_files_test.go` | `focus-heartbeat.md` (Codex lo copió de su contexto de arranque; `git status` sin cambios por HCD fuera de `hcd/`) | Cubierto |
