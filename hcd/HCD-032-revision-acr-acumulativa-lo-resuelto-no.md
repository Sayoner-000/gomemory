# Revisión ACR acumulativa: lo resuelto no se reabre y se revisa solo el delta

ID: HCD-032
Renumerada: antes HCD-030 (colisión con HCD-030 de origin/main); el cuerpo conserva el número original para no alterar la revisión aprobada. HCD-030 → HCD-032, HCD-031 → HCD-033
Tipo: feature
Carril: full
Motivo: Cambia el contrato de review_start y del veredicto ACR, con decisiones interdependientes (qué se hereda, qué es delta, criterio de parada, revisor no ejecutado)

## Intención
Reducir el coste en tokens de ACR: una revisión nueva sobre el mismo objetivo hereda los hallazgos ya cerrados sin reabrirlos, los revisores examinan solo el delta desde el último cierre, la revisión se detiene cuando no quedan CRITICAL/HIGH abiertos y un revisor que no entregó informe nunca cuenta como limpio


## Alcance
Incluido: enlazar una revisión ACR nueva con una revisión anterior del mismo trabajo para que (a) herede los hallazgos ya asentados, que los revisores reciben como «no reportar» y que, si reaparecen en el mismo alcance sin cambios, no se vuelven a adjudicar; (b) los revisores examinen solo el delta desde la revisión anterior. Un hallazgo reportado se reconoce como heredado solo por referencia explícita: `review_start` devuelve la lista de heredados con su id y el revisor que lo reporta lo cita con `settled_ref: <id>`; el motor comprueba que su archivo no está en el delta. Sin referencia, el hallazgo se adjudica como hoy (sin comparación aproximada de textos). Superficies: `review_start` (MCP), `mem review start` (CLI), el dominio de revisión y el ledger (aditivo).
Excluido: «revisor no ejecutado ≠ limpio» y «parar sin CRITICAL/HIGH abiertos», que ya existen (`domain.DeriveVerdict`: INCOMPLETE si falta o falla A o B; APPROVED sin severos pendientes) y se conservan intactos; las rondas de corrección dentro de una misma revisión, que ya son solo de revalidación; el contenido de la skill ACR más allá de usar el nuevo enlace.

## ¿Qué pasa si no lo hacemos?
Cada `review_start` empieza de cero aunque el objetivo sea la misma unidad tras unas correcciones: los dos revisores vuelven a leer todo el alcance y vuelven a encontrar, reportar y adjudicar hallazgos ya cerrados. En HCD-025 hubo cuatro revisiones seguidas (memorias 1691, 1697, 1700, 1706) y cada una re-auditó lo anterior; es el consumo de tokens que la persona señaló como problema.

## Alternativa más barata
Pegar a mano en el prompt de los revisores la lista de hallazgos cerrados y el diff. No basta: depende de que el agente lo recuerde y lo haga bien, el motor no impide re-adjudicar un hallazgo asentado y nada queda en el ledger para auditar qué se heredó.

## Historias
### US1. Una revisión de seguimiento no reabre lo ya asentado
Como persona que pide revisiones ACR sucesivas sobre el mismo trabajo, quiero que la nueva herede lo que la anterior ya asentó, para no pagar dos veces el mismo hallazgo.
- C-01: Dado una revisión anterior terminada, cuando se inicia una nueva con `since_review_id` (CLI `--since <id>`; Q1), entonces la nueva registra en el ledger de qué revisión hereda y qué hallazgos heredó.
- C-02: Dado un hallazgo heredado —CONFIRMED con re-juicio RESOLVED vigente, o SUSPECT/INFO ya clasificado; nunca un CONFIRMED severo sin resolver ni una CONTRADICTION (Q2)—, cuando un revisor lo vuelve a reportar sobre un archivo que no cambió desde la revisión anterior, entonces no se crea una adjudicación nueva y el veredicto lo trata como ya asentado.
- C-03: Dado un hallazgo heredado cuyo alcance cambió desde la revisión anterior, cuando reaparece, entonces se adjudica de nuevo con normalidad y queda enlazado al hallazgo heredado, para que la regresión sea visible (Q4).

### US2. Los revisores examinan solo el delta
Como persona que paga los tokens de cada revisor, quiero que la revisión de seguimiento se limite a lo que cambió desde la anterior.
- C-04: Dado una revisión enlazada, cuando se inicia, entonces, si ambas tienen revisión git, gomemory calcula el alcance como los archivos cambiados entre la revisión anterior y la nueva (`git diff --name-only`), lo guarda y `review_status` lo muestra; sin git o sin commit usa el scope del llamador y lo declara (Q3).
- C-05: Dado una revisión sin enlace, cuando se inicia, entonces se comporta exactamente como hoy (compatibilidad).
- C-06: Las garantías actuales del veredicto no cambian: un revisor ausente o fallido sigue dando INCOMPLETE y un CRITICAL/HIGH confirmado sin resolver sigue impidiendo APPROVED.

## Aclaraciones
- [x] Q1. Q1: ¿cómo se enlaza una revisión nueva con la anterior del mismo trabajo? → Explícito con --since (Recommended): review_start acepta since_review_id (CLI --since <id>); sin él todo funciona como hoy
- [x] Q2. Q2: ¿qué hallazgos de la revisión anterior cuentan como asentados y se heredan? → Resueltos + no severos (Recommended): CONFIRMED con re-juicio RESOLVED vigente, más SUSPECT e INFO ya clasificados; un CONFIRMED severo sin resolver o una CONTRADICTION no se hereda y sigue abierto
- [x] Q3. Q3: ¿quién calcula el delta que examinan los revisores en una revisión enlazada? → gomemory con git (Recommended): con revisión git en ambas, git diff --name-only anterior..nueva da el alcance guardado y mostrado en review_status; sin git o sin commit, se usa el scope del llamador y se declara
- [x] Q4. Q4: un hallazgo heredado que reaparece en un archivo que cambió desde la revisión anterior, ¿se reabre o sigue asentado? → Se reabre como hallazgo nuevo (Recommended): si el archivo cambió (está en el delta) se adjudica de nuevo enlazado al heredado; si no cambió, sigue asentado y no se adjudica

## Slices
### VS-01. El dominio decide qué se hereda, cuándo un asentado sigue valiendo y cómo lo trata el veredicto
Tareas: 1, 2, 3
Criterios: C-02, C-03, C-06

### VS-02. Una revisión iniciada con --since hereda de la anterior y examina solo el delta de git
Tareas: 4, 5, 6
Criterios: C-01, C-04, C-05

### VS-03. Los revisores citan lo heredado con settled_ref y la revisión aprueba sin re-adjudicarlo
Tareas: 7, 8, 9, 10
Criterios: C-02, C-03, C-06

## Checklist
- [x] 1. Seleccionar los hallazgos heredables de una revisión terminada: CONFIRMED con re-juicio RESOLVED vigente, SUSPECT e INFO; nunca un severo sin resolver ni una CONTRADICTION (Q2) → `go test ./domain -run TestInheritableFindings`
- [x] 2. Decidir si un asentado sigue valiendo: sí si ninguno de sus archivos (de `Location`) está en el delta; no si alguno cambió (Q4) → `go test ./domain -run TestSettledStillValid`
- [x] 3. El veredicto no exige clasificar un hallazgo con asentamiento vigente; uno reabierto sí; INCOMPLETE y la regla de severos siguen intactos (C-06) → `go test ./domain -run TestDeriveVerdict_Settled`
- [x] 4. Persistir de forma aditiva el enlace, el delta, los heredados y `settled_ref`, con ida y vuelta completa → `go test -count=1 ./adapters/secondary/persistence -run TestReviewInheritance_RoundTrip`
- [x] 5. Calcular con git los archivos cambiados entre dos commits, con error claro si alguno no es un commit → `go test -count=1 ./adapters/secondary/gitdiff -run TestChangedFiles`
- [x] 6. `StartReview` con `SinceReviewID`: exige una revisión anterior terminada del mismo proyecto, hereda y fija el delta de git o, sin git/commit, el scope del llamador declarado → `go test ./application/usecases -run TestStartReview_Since`
- [x] 7. `SubmitReviewerResult` acepta `settled_ref`: rechaza una referencia que no es un heredado; marca asentado si su archivo no cambió y lo reabre enlazado si cambió → `go test ./application/usecases -run TestSubmitReviewerResult_SettledRef`
- [x] 8. `review_start` (MCP) acepta `since_review_id` y devuelve heredados y delta; `review_submit` acepta `settled_ref`; `mem review start --since`; `review_status` muestra enlace y delta → `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince`
- [x] 9. Dos revisiones encadenadas sobre un repositorio git real: la segunda hereda, un asentado en archivo sin cambios aprueba sin consenso y uno en archivo cambiado se reabre → `go test -count=1 ./tests/integration -run TestReviewAcumulativa`
- [x] 10. Sin regresiones → `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 07:01 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./domain -run TestInheritableFindings` (salida 1)
      # mem/domain [mem/domain.test]
      domain/review_inheritance_test.go:32:9: undefined: InheritableFindings
      domain/review_inheritance_test.go:49:7: undefined: SettledFinding
      domain/review_inheritance_test.go:50:6: undefined: SettledStillValid
      domain/review_inheritance_test.go:53:5: undefined: SettledStillValid
      domain/review_inheritance_test.go:56:6: undefined: SettledStillValid
      domain/review_inheritance_test.go:59:5: undefined: SettledStillValid
      domain/review_inheritance_test.go:59:23: undefined: SettledFinding
      domain/review_inheritance_test.go:72:83: unknown field SettledRef in struct literal of type Finding
      domain/review_inheritance_test.go:72:99: unknown field Settled in struct literal of type Finding
      domain/review_inheritance_test.go:77:11: reopened.Settled undefined (type Finding has no field or method Settled)
      domain/review_inheritance_test.go:77:11: too many errors
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 07:01 -05:00 · tarea 2 · rojo observado · caducada · `go test -count=1 ./domain -run TestSettledStillValid` (salida 1)
      # mem/domain [mem/domain.test]
      domain/review_inheritance_test.go:32:9: undefined: InheritableFindings
      domain/review_inheritance_test.go:49:7: undefined: SettledFinding
      domain/review_inheritance_test.go:50:6: undefined: SettledStillValid
      domain/review_inheritance_test.go:53:5: undefined: SettledStillValid
      domain/review_inheritance_test.go:56:6: undefined: SettledStillValid
      domain/review_inheritance_test.go:59:5: undefined: SettledStillValid
      domain/review_inheritance_test.go:59:23: undefined: SettledFinding
      domain/review_inheritance_test.go:72:83: unknown field SettledRef in struct literal of type Finding
      domain/review_inheritance_test.go:72:99: unknown field Settled in struct literal of type Finding
      domain/review_inheritance_test.go:77:11: reopened.Settled undefined (type Finding has no field or method Settled)
      domain/review_inheritance_test.go:77:11: too many errors
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 07:01 -05:00 · tarea 3 · rojo observado · caducada · `go test -count=1 ./domain -run TestDeriveVerdict_Settled` (salida 1)
      # mem/domain [mem/domain.test]
      domain/review_inheritance_test.go:32:9: undefined: InheritableFindings
      domain/review_inheritance_test.go:49:7: undefined: SettledFinding
      domain/review_inheritance_test.go:50:6: undefined: SettledStillValid
      domain/review_inheritance_test.go:53:5: undefined: SettledStillValid
      domain/review_inheritance_test.go:56:6: undefined: SettledStillValid
      domain/review_inheritance_test.go:59:5: undefined: SettledStillValid
      domain/review_inheritance_test.go:59:23: undefined: SettledFinding
      domain/review_inheritance_test.go:72:83: unknown field SettledRef in struct literal of type Finding
      domain/review_inheritance_test.go:72:99: unknown field Settled in struct literal of type Finding
      domain/review_inheritance_test.go:77:11: reopened.Settled undefined (type Finding has no field or method Settled)
      domain/review_inheritance_test.go:77:11: too many errors
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 07:02 -05:00 · tarea 1 · verde · caducada · `go test -count=1 ./domain -run TestInheritableFindings` (salida 0)
      ok  	mem/domain	0.133s
- 2026-11-05 07:02 -05:00 · tarea 2 · verde · caducada · `go test -count=1 ./domain -run TestSettledStillValid` (salida 0)
      ok  	mem/domain	0.124s
- 2026-11-05 07:02 -05:00 · tarea 3 · verde · caducada · `go test -count=1 ./domain -run TestDeriveVerdict_Settled` (salida 0)
      ok  	mem/domain	0.123s
- 2026-11-05 07:03 -05:00 · tarea 4 · rojo observado · caducada · `go test -count=1 ./adapters/secondary/persistence -run TestReviewInheritance_RoundTrip` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/review_inheritance_test.go:27:3: unknown field SinceReviewID in struct literal of type domain.Review
      adapters/secondary/persistence/review_inheritance_test.go:27:30: unknown field DeltaScope in struct literal of type domain.Review
      adapters/secondary/persistence/review_inheritance_test.go:27:71: unknown field DeltaSource in struct literal of type domain.Review
      adapters/secondary/persistence/review_inheritance_test.go:27:91: undefined: domain.DeltaSourceGit
      adapters/secondary/persistence/review_inheritance_test.go:28:3: unknown field Inherited in struct literal of type domain.Review
      adapters/secondary/persistence/review_inheritance_test.go:38:9: got.SinceReviewID undefined (type *domain.Review has no field or method SinceReviewID)
      adapters/secondary/persistence/review_inheritance_test.go:38:44: got.DeltaSource undefined (type *domain.Review has no field or method DeltaSource)
      adapters/secondary/persistence/review_inheritance_test.go:38:66: undefined: domain.DeltaSourceGit
      adapters/secondary/persistence/review_inheritance_test.go:38:102: got.DeltaScope undefined (type *domain.Review has no field or method DeltaScope)
      adapters/secondary/persistence/review_inheritance_test.go:41:28: repo.InheritedFindings undefined (type ports.ReviewRepository has no field or method InheritedFindings)
      adapters/secondary/persistence/review_inheritance_test.go:41:28: too many errors
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-05 07:07 -05:00 · tarea 4 · verde · caducada · `go test -count=1 ./adapters/secondary/persistence -run TestReviewInheritance_RoundTrip` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.194s
- 2026-11-05 07:07 -05:00 · tarea 5 · rojo observado · caducada · `go test -count=1 ./adapters/secondary/gitdiff -run TestChangedFiles` (salida 1)
      # mem/adapters/secondary/gitdiff [mem/adapters/secondary/gitdiff.test]
      adapters/secondary/gitdiff/gitdiff_test.go:51:7: undefined: New
      adapters/secondary/gitdiff/gitdiff_test.go:67:15: undefined: New
      FAIL	mem/adapters/secondary/gitdiff [build failed]
      FAIL
- 2026-11-05 07:07 -05:00 · tarea 5 · verde · caducada · `go test -count=1 ./adapters/secondary/gitdiff -run TestChangedFiles` (salida 0)
      ok  	mem/adapters/secondary/gitdiff	0.284s
- 2026-11-05 07:08 -05:00 · tarea 6 · rojo observado · caducada · `go test -count=1 ./application/usecases -run TestStartReview_Since` (salida 1)
      # mem/application/usecases [mem/application/usecases.test]
      application/usecases/start_review_since_test.go:61:28: unknown field SinceReviewID in struct literal of type StartReviewInput
      application/usecases/start_review_since_test.go:63:17: undefined: StartReviewSince
      application/usecases/start_review_since_test.go:79:18: undefined: StartReviewSince
      application/usecases/start_review_since_test.go:89:82: unknown field SinceReviewID in struct literal of type StartReviewInput
      application/usecases/start_review_since_test.go:90:82: unknown field SinceReviewID in struct literal of type StartReviewInput
      application/usecases/start_review_since_test.go:92:16: undefined: StartReviewSince
      application/usecases/start_review_since_test.go:97:15: undefined: StartReviewSince
      application/usecases/start_review_since_test.go:107:8: plain.SinceReviewID undefined (type StartReviewInput has no field or method SinceReviewID)
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-05 07:08 -05:00 · tarea 4 · verde · caducada · `go test -count=1 ./adapters/secondary/persistence -run TestReviewInheritance_RoundTrip` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.221s
- 2026-11-05 07:09 -05:00 · tarea 5 · verde · caducada · `go test -count=1 ./adapters/secondary/gitdiff -run TestChangedFiles` (salida 0)
      ok  	mem/adapters/secondary/gitdiff	0.278s
- 2026-11-05 07:09 -05:00 · tarea 6 · verde · caducada · `go test -count=1 ./application/usecases -run TestStartReview_Since` (salida 0)
      ok  	mem/application/usecases	0.151s
- 2026-11-05 07:09 -05:00 · tarea 1 · verde · caducada · `go test -count=1 ./domain -run TestInheritableFindings` (salida 0)
      ok  	mem/domain	0.120s
- 2026-11-05 07:09 -05:00 · tarea 2 · verde · caducada · `go test -count=1 ./domain -run TestSettledStillValid` (salida 0)
      ok  	mem/domain	0.146s
- 2026-11-05 07:09 -05:00 · tarea 3 · verde · caducada · `go test -count=1 ./domain -run TestDeriveVerdict_Settled` (salida 0)
      ok  	mem/domain	0.245s
- 2026-11-05 07:09 -05:00 · tarea 7 · rojo observado · caducada · `go test -count=1 ./application/usecases -run TestSubmitReviewerResult_SettledRef` (salida 1)
      --- FAIL: TestSubmitReviewerResult_SettledRef (0.00s)
          --- FAIL: TestSubmitReviewerResult_SettledRef/una_referencia_que_no_es_un_heredado_se_rechaza (0.00s)
              submit_settled_test.go:40: settled_ref debe citar un hallazgo heredado de esta revisión
          --- FAIL: TestSubmitReviewerResult_SettledRef/archivo_sin_cambios:_asentado_y_la_revisión_aprueba_sin_consenso (0.00s)
              submit_settled_test.go:55: el motor debe marcar asentado el hallazgo de A: {ID:1 ReviewerResultID:1 LocalID:A-1 Location:domain/rules.go:42 Severity:HIGH Category:c Claim:resuelto EvidenceClass:static-analysis Evidence:[e] Confidence:high SettledRef:1 Settled:false}
          --- FAIL: TestSubmitReviewerResult_SettledRef/el_consenso_no_exige_clasificar_los_asentados (0.00s)
              submit_settled_test.go:117: clasificar solo el nuevo debe bastar: quedan 1 hallazgos sin clasificar: 1
      FAIL
      FAIL	mem/application/usecases	0.219s
      FAIL
- 2026-11-05 07:11 -05:00 · tarea 7 · verde · caducada · `go test -count=1 ./application/usecases -run TestSubmitReviewerResult_SettledRef` (salida 0)
      ok  	mem/application/usecases	0.195s
- 2026-11-05 07:12 -05:00 · tarea 8 · rojo observado · caducada · `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince` (salida 1)
      # mem/adapters/primary/cli/review [mem/adapters/primary/cli/review.test]
      adapters/primary/cli/review/review_since_test.go:15:26: undefined: extraerModificadores
      adapters/primary/cli/review/review_since_test.go:19:25: undefined: extraerModificadores
      adapters/primary/cli/review/review_since_test.go:22:21: undefined: extraerModificadores
      adapters/primary/cli/review/review_since_test.go:25:29: undefined: extraerModificadores
      adapters/primary/cli/review/review_since_test.go:31:8: undefined: findingsDesdeEntrada
      adapters/primary/cli/review/review_since_test.go:32:58: unknown field SettledRef in struct literal of type reviewFindingInput
      adapters/primary/cli/review/review_since_test.go:44:12: undefined: enlaceDeRevision
      adapters/primary/cli/review/review_since_test.go:59:10: undefined: enlaceDeRevision
      FAIL	mem/adapters/primary/cli/review [build failed]
      FAIL
- 2026-11-05 07:13 -05:00 · tarea 8 · verde · caducada · `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince` (salida 0)
      ok  	mem/adapters/primary/cli/review	0.171s
- 2026-11-05 07:13 -05:00 · tarea 9 · rojo observado · caducada · `go test -count=1 ./tests/integration -run TestReviewAcumulativa` (salida 1)
      --- FAIL: TestReviewAcumulativa (0.11s)
          review_acumulativa_test.go:123: archivo sin cambios: asentado: {ID:3 ReviewerResultID:3 LocalID:A-1 Location:domain/rules.go:1 Severity:HIGH Category:c Claim:sospecha en domain/rules.go:1 EvidenceClass:static-analysis Evidence:[e] Confidence:medium SettledRef:1 Settled:false}
      FAIL
      FAIL	mem/tests/integration	0.404s
      FAIL
- 2026-11-05 07:17 -05:00 · tarea 1 · verde · vigente · `go test -count=1 ./domain -run TestInheritableFindings` (salida 0)
      ok  	mem/domain	0.148s
- 2026-11-05 07:17 -05:00 · tarea 2 · verde · vigente · `go test -count=1 ./domain -run TestSettledStillValid` (salida 0)
      ok  	mem/domain	0.121s
- 2026-11-05 07:17 -05:00 · tarea 3 · verde · vigente · `go test -count=1 ./domain -run TestDeriveVerdict_Settled` (salida 0)
      ok  	mem/domain	0.145s
- 2026-11-05 07:17 -05:00 · tarea 4 · verde · vigente · `go test -count=1 ./adapters/secondary/persistence -run TestReviewInheritance_RoundTrip` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.172s
- 2026-11-05 07:17 -05:00 · tarea 5 · verde · vigente · `go test -count=1 ./adapters/secondary/gitdiff -run TestChangedFiles` (salida 0)
      ok  	mem/adapters/secondary/gitdiff	0.278s
- 2026-11-05 07:17 -05:00 · tarea 6 · verde · vigente · `go test -count=1 ./application/usecases -run TestStartReview_Since` (salida 0)
      ok  	mem/application/usecases	0.155s
- 2026-11-05 07:17 -05:00 · tarea 7 · verde · vigente · `go test -count=1 ./application/usecases -run TestSubmitReviewerResult_SettledRef` (salida 0)
      ok  	mem/application/usecases	0.156s
- 2026-11-05 07:17 -05:00 · tarea 8 · verde · vigente · `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince` (salida 0)
      ok  	mem/adapters/primary/cli/review	0.138s
- 2026-11-05 07:17 -05:00 · tarea 9 · verde · vigente · `go test -count=1 ./tests/integration -run TestReviewAcumulativa` (salida 0)
      ok  	mem/tests/integration	0.376s
- 2026-11-05 07:17 -05:00 · tarea 10 · verde · vigente · `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...` (salida 0)
      ok  	mem/adapters/secondary/codegraph	0.133s
      ok  	mem/adapters/secondary/codegraph/codebasememory	1.047s
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.646s
      ok  	mem/adapters/secondary/compression	0.096s
      ok  	mem/adapters/secondary/compression/native	6.850s
      ?   	mem/adapters/secondary/configpath	[no test files]
      ok  	mem/adapters/secondary/expandstate	0.219s
      ok  	mem/adapters/secondary/generator	1.080s
      ok  	mem/adapters/secondary/gitdiff	0.480s
      ok  	mem/adapters/secondary/hcdfiles	0.600s
      ok  	mem/adapters/secondary/launcher	0.516s
      ok  	mem/adapters/secondary/persistence	31.100s
      ok  	mem/adapters/secondary/release	0.352s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.150s
      ok  	mem/adapters/secondary/speckit	0.099s
      ok  	mem/adapters/secondary/tokens	0.092s
      ok  	mem/adapters/secondary/usage	0.105s
      ok  	mem/tests/contract	85.772s
      ok  	mem/tests/integration	36.945s

## Siguiente
Cerrada con aprobación sobre la revisión 8baa7d881aa5.
