# Skill ACR: encadenar revisiones con since_review_id y citar lo heredado con settled_ref

ID: HCD-033
Renumerada: antes HCD-031 (colisión con HCD-031 de origin/main); el cuerpo conserva el número original para no alterar la revisión aprobada. HCD-030 → HCD-032, HCD-031 → HCD-033
Tipo: evolution
Carril: fast
Motivo: Cambio acotado a la skill ACR (dos copias sincronizadas) que hace usar el contrato ya aprobado en HCD-032; sin decisiones abiertas

## Intención
Que la skill ACR, al encadenar una revisión de seguimiento, inicie con since_review_id, entregue a los revisores el delta y la lista inherited con sus instrucciones, y les haga citar lo heredado con settled_ref, para que el ahorro de HCD-030 se produzca en las revisiones reales


## Alcance
Incluido: la skill ACR, en sus dos copias sincronizadas (`skills/adversarial-consensus-review/SKILL.md` y `infrastructure/templates/adversarial-consensus-review/SKILL.md`): una subsección «Follow-up Review» en la sección 42 y un bloque opcional en la plantilla del revisor (sección 44), redactados de forma neutral respecto del proveedor y con la correspondencia a gomemory (`since_review_id`, `inherited`, `delta`, `settled_ref`). Las descripciones de las herramientas MCP `review_start` y `review_submit`, que es lo que el agente lee al usarlas.
Excluido: cambios en el motor de revisión (cerrado en HCD-030); renumerar la skill; el README de la skill y el manual de usuario (se actualizan en la release).

## ¿Qué pasa si no lo hacemos?
HCD-030 queda sin uso: ningún orquestador sabe pasar `since_review_id` ni que los revisores deben citar lo heredado con `settled_ref`, así que cada revisión de seguimiento sigue re-auditando todo y el ahorro de tokens no se produce.

## Alternativa más barata
Cambiar solo la descripción de `review_start`. No basta: los revisores no ven esa descripción (la ve el orquestador) y la plantilla del revisor es la que fija qué reportan; sin ella siguen re-reportando lo asentado sin cita y el motor lo adjudica de nuevo.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Las dos copias de la skill describen la revisión de seguimiento (since_review_id, delta, inherited, settled_ref) en la sección 42 y en la plantilla del revisor, y siguen idénticas → `go test -count=1 ./tests/contract -run 'TestACRSkill_RevisionDeSeguimiento|TestSkillSourcesInSync'`
- [x] 2. Las descripciones de `review_start` y `review_submit` explican cuándo usar since_review_id y settled_ref → `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince_Descripciones`
- [x] 3. Sin regresiones → `go test -count=1 ./adapters/... ./tests/...`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 07:28 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./tests/contract -run 'TestACRSkill_RevisionDeSeguimiento|TestSkillSourcesInSync'` (salida 1)
      # mem/tests/contract
      found packages contract (acr_skill_followup_test.go) and main (agent_integration_doc_test.go) in /Users/josegomezj/home/rcw/gomemory/tests/contract
      FAIL	mem/tests/contract [setup failed]
      FAIL
- 2026-11-05 07:28 -05:00 · tarea 2 · rojo observado · caducada · `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince_Descripciones` (salida 1)
      # mem/adapters/primary/cli/review [mem/adapters/primary/cli/review.test]
      adapters/primary/cli/review/review_since_test.go:66:23: undefined: reviewStartDescription
      adapters/primary/cli/review/review_since_test.go:67:74: undefined: reviewStartDescription
      adapters/primary/cli/review/review_since_test.go:69:23: undefined: reviewSubmitDescription
      adapters/primary/cli/review/review_since_test.go:70:59: undefined: reviewSubmitDescription
      FAIL	mem/adapters/primary/cli/review [build failed]
      FAIL
- 2026-11-05 07:28 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./tests/contract -run 'TestACRSkill_RevisionDeSeguimiento|TestSkillSourcesInSync'` (salida 1)
      --- FAIL: TestACRSkill_RevisionDeSeguimiento (0.00s)
          acr_skill_followup_test.go:25: skills/adversarial-consensus-review/SKILL.md: falta la subsección «Follow-up Review» en la orquestación
      FAIL
      FAIL	mem/tests/contract	0.259s
      FAIL
- 2026-11-05 07:28 -05:00 · tarea 1 · verde · caducada · `go test -count=1 ./tests/contract -run 'TestACRSkill_RevisionDeSeguimiento|TestSkillSourcesInSync'` (salida 0)
      ok  	mem/tests/contract	0.239s
- 2026-11-05 07:28 -05:00 · tarea 2 · verde · caducada · `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince_Descripciones` (salida 0)
      ok  	mem/adapters/primary/cli/review	0.205s
- 2026-11-05 07:30 -05:00 · tarea 3 · falló · caducada · `go test -count=1 ./adapters/... ./tests/...` (salida 1)
      ok  	mem/adapters/secondary/codegraph/codebasememory	1.199s
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.560s
      ok  	mem/adapters/secondary/compression	0.089s
      ok  	mem/adapters/secondary/compression/native	6.708s
      ?   	mem/adapters/secondary/configpath	[no test files]
      ok  	mem/adapters/secondary/expandstate	0.191s
      ok  	mem/adapters/secondary/generator	1.046s
      ok  	mem/adapters/secondary/gitdiff	0.465s
      ok  	mem/adapters/secondary/hcdfiles	0.664s
      ok  	mem/adapters/secondary/launcher	0.513s
      ok  	mem/adapters/secondary/persistence	32.250s
      ok  	mem/adapters/secondary/release	0.360s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.155s
      ok  	mem/adapters/secondary/speckit	0.103s
      ok  	mem/adapters/secondary/tokens	0.095s
      ok  	mem/adapters/secondary/usage	0.088s
      ok  	mem/tests/contract	89.233s
      ok  	mem/tests/integration	46.085s
      FAIL
- 2026-11-05 07:34 -05:00 · tarea 1 · verde · vigente · `go test -count=1 ./tests/contract -run 'TestACRSkill_RevisionDeSeguimiento|TestSkillSourcesInSync'` (salida 0)
      ok  	mem/tests/contract	0.222s
- 2026-11-05 07:34 -05:00 · tarea 2 · verde · vigente · `go test -count=1 ./adapters/primary/cli/review -run TestReviewSince_Descripciones` (salida 0)
      ok  	mem/adapters/primary/cli/review	0.157s
- 2026-11-05 07:34 -05:00 · tarea 3 · verde · vigente · `go test -count=1 ./adapters/... ./tests/...` (salida 0)
      ok  	mem/adapters/secondary/codegraph	0.139s
      ok  	mem/adapters/secondary/codegraph/codebasememory	1.025s
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.613s
      ok  	mem/adapters/secondary/compression	0.124s
      ok  	mem/adapters/secondary/compression/native	6.430s
      ?   	mem/adapters/secondary/configpath	[no test files]
      ok  	mem/adapters/secondary/expandstate	0.207s
      ok  	mem/adapters/secondary/generator	1.131s
      ok  	mem/adapters/secondary/gitdiff	0.520s
      ok  	mem/adapters/secondary/hcdfiles	0.678s
      ok  	mem/adapters/secondary/launcher	0.526s
      ok  	mem/adapters/secondary/persistence	32.752s
      ok  	mem/adapters/secondary/release	0.364s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.190s
      ok  	mem/adapters/secondary/speckit	0.120s
      ok  	mem/adapters/secondary/tokens	0.085s
      ok  	mem/adapters/secondary/usage	0.111s
      ok  	mem/tests/contract	85.203s
      ok  	mem/tests/integration	36.922s

## Siguiente
Cerrada con aprobación sobre la revisión 868e20725c41.
