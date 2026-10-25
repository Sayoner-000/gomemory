# Matriz de aceptación de 042 (provisional)

**Estado: aceptación PENDIENTE.** Falta la evidencia real de los tres agentes (T063 Claude Code, T064 Codex, T065 OpenCode), y con ella el cierre de T068 y T069. Este documento recoge lo que **sí** está demostrado de forma automática y deja explícito lo que no lo está. No declara completa la feature.

Evidencia automática y cifras: [`automated.md`](automated.md). Línea base previa: [`baseline.md`](baseline.md).

Leyenda: **Automático** = cubierto por pruebas deterministas con un agente de Spec Kit sintético; **Skill** = depende de que el modelo siga las instrucciones de la skill (se comprueba su texto, no el comportamiento del modelo); **Pendiente** = requiere una ejecución real que no se ha hecho.

## Requisitos funcionales

| FR | Estado | Dónde se comprueba |
| --- | --- | --- |
| FR-001 cuatro clases de solicitud, original conservado | Automático | `application/usecases/socratica_specify_draft_test.go` (`FourRequestClasses…`), `tests/integration/socratica_specify_dispatch_test.go` (caso A) |
| FR-002 coexistencia con `refine`/`scrutiny` | Automático | `…PreviousModesKeepTheirContract`, `…PreviousModesKeepApprove`, `…PreviousModesKeepTheirAckAndHaveNoReport` (CLI MCP), `TestCmdSocratica` |
| FR-003 contexto antes de preguntar, tratado como datos | Skill + Automático parcial | Texto de la skill (`socratica_specify_skill_test.go`); instrucciones incrustadas no autorizan ni responden (`…EmbeddedInstructionsAreOnlyData`). La calidad de la conversación real es **Pendiente** |
| FR-004 tres preguntas acumuladas, omitir no resuelve | Automático | `domain/socratica_specify_questions_test.go`, `…QuestionsAccumulateAndNeverFailTheFlow`, caso B de integración |
| FR-005 contenido mínimo y pendientes bloqueantes | Automático | `domain/socratica_specify_validation_test.go` |
| FR-006 entregas completas y verificables | Automático | validadores (`delivery_gap`, `foreign_criterion`, `story_without_delivery`); la elección de entregas es del agente y de la persona |
| FR-007 etapas, sin saltos, indica etapa y siguiente paso | Automático | `domain/socratica_specify_stages_test.go`, `…StageAndRevisionGuards`, `SpecifyNext` |
| FR-008 mínimos, listas declaradas, marcadores | Automático | `domain/socratica_specify_validation_test.go` |
| FR-009 identificadores únicos y referencias | Automático | ídem |
| FR-010 no declarar lista con faltantes ni bloqueos | Automático | `…OmissionsNeverResolveABlock`, caso B |
| FR-011 invalidación al editar | Automático | `domain/socratica_specify_review_test.go`, `…EditingForcesANewCheckAndReview` |
| FR-012 texto final desde la preparación, idéntico | Automático | `domain/socratica_specify_render_test.go`, `…BindsToTheExactReviewedText` |
| FR-013 revisión con límites explicados y acciones | Skill + Automático | `next` de revisión (`…ShowsTheCompleteTextWithoutSideEffects`); texto de la skill |
| FR-014 solo la aprobación explícita autoriza | Automático | `…ApprovalIsExplicitAndBoundToTheShownRevision`, `…ClosedRoutes` |
| FR-015 cancelar conserva el original | Automático | `…CancelReturnsTheOriginalAndWithdrawsTheDraft`, `…ExplicitCancelFreesTheSlot…` |
| FR-016 comprobar Spec Kit antes de consumir | Skill + Automático parcial | Orden preflight→ack en `adapters/primary/setup/socratica_specify_skill_test.go`; sin `ack` no se consume nada (caso C). Que el modelo lo haga de verdad es **Pendiente** |
| FR-017 una sola concesión, confirmaciones simultáneas | Automático | `…ConcurrentAcksGrantExactlyOnce` (MCP real), `socratica_specify_ack_test.go` (SQLite, `-race`) |
| FR-018 respuesta perdida, incierto, reenvío explícito | Automático | `…ReplayLostResponseAndOtherNonce`, `…ReconciliationIsExplicit…` |
| FR-019 Spec Kit recibe solo la entrada aprobada; spec, checklist y trazabilidad | Automático (control) + **Pendiente** (generación real) | Caso C entrega el texto exacto al agente sintético y valida trazabilidad; spec/checklist reales: T063–T065 |
| FR-020 resultado comprobado y diferenciado | Automático | `report` (`socratica_specify_report_test.go`, `…ReportCommunicatesAnOutcomeNeverAPermission`); es información del agente, rotulada como tal |
| FR-021 recuperación aislada por proyecto y agente | Automático | `application/usecases/socratica_specify_recovery_test.go` |
| FR-022 protección y modo privado | Automático | `socratica_ephemeral_test.go`, `tests/integration/socratica_specify_private_test.go` (escáner de residuos con control) |
| FR-023 borrado y retención del agregado; fallo conserva el último estado | Automático | `socratica_specify_retention_test.go`, `socratica_specify_migration_test.go` (reversión) |
| FR-024 CLI y TUI reconocen el modo sin generar | Automático | `cmd_socratica_specify_test.go`, `tui_socratica_specify_test.go` (pureza a ocho anchos) |
| FR-025 skill canónica y copias coherentes, alias finos | Automático | paridad en `agent_skills_setup_test.go` y `tests/contract/skill_sources_sync_test.go`; `…EnvoltorioSocraticaHU_EsFino…` |
| FR-026 alias y entradas por agente | Automático (reconocimiento e instalación) + **Pendiente** (invocación real) | `domain/socratica_interception_test.go`, `hook_socratica_test.go`, instalación; que cada agente la ejecute: T063–T065 |
| FR-027 instalación idempotente, sin crear agentes, desinstalación | Automático | `socratica_install_test.go`, `tests/contract/lifecycle_*_test.go` (binario real) |
| FR-028 flujos existentes y Spec Kit independiente | Automático | suites previas verdes; Spec Kit no se modifica |

## Resultados de éxito

| SC | Estado | Observación |
| --- | --- | --- |
| SC-001 cuatro clases, ≤ 3 preguntas | Automático | control de flujo; la calidad de lo que propone el modelo es **Pendiente** |
| SC-002 100 % bloqueos sin aprobación | Automático | casos B y de validación |
| SC-003 edición exige comprobar y revisar; coincidencia carácter a carácter | Automático | huella de bytes exactos |
| SC-004 como máximo una generación por autorización | Automático | concurrencia, replay, rollback, reenvío explícito |
| SC-005 falta de generador no consume; cancelar recupera el original | Automático (control) | la comprobación de disponibilidad la hace el agente (**Skill**) |
| SC-006 recorrido real en los tres agentes con spec y checklist generados | **Pendiente** | T063, T064, T065 |
| SC-007 persistencia y cero apariciones del contenido excluido | Automático | recuperación, privacidad entre procesos, sanitización |
| SC-008 instalación idempotente y desinstalación limpia en los tres agentes | Automático | pruebas aisladas y binario real |
| SC-009 la persona confirma mediante la aprobación; cero especificaciones antes | Automático (control) + **Pendiente** (demostración real) | casos A–C; confirmación humana real: T063–T065 |

## Casos del quickstart

| Caso | Estado |
| --- | --- |
| A preparación sin generar | Automático |
| B bloqueos, edición y preguntas | Automático |
| C disponibilidad, aprobación y replay | Automático |
| D resultado real en los tres agentes | **Pendiente** |
| E recuperación, privacidad y borrado | Automático |
| F instalación, compatibilidad y migración | Automático |

## Qué falta y por qué

Los casos D (T063–T065) necesitan, por agente: sesión autenticada, un proyecto de aceptación desechable con Spec Kit, y a una persona que elija «Aprobar y generar especificación» sobre el texto mostrado. Si el agente aprobara por sí mismo, la demostración dejaría de probar el control que debe probar. Hasta que existan `claude.md`, `codex.md` y `opencode.md` en esta carpeta, SC-006 y la parte real de SC-009 están pendientes y la feature no se declara completa.
