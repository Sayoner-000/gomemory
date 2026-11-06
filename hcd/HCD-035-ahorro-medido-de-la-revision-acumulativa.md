# Ahorro medido de la revisión acumulativa: review_start --since informa los tokens que el revisor evita leer

ID: HCD-035
Tipo: evolution
Carril: fast
Motivo: salida aditiva de review_start; proxy determinista sin decisiones interdependientes

## Intención
Que cada revisión iniciada con --since informe, sin gastar tokens de modelo, cuántos tokens del alcance evita releer el revisor (alcance completo frente a delta, descontando la lista de heredados), para comprobar con datos si la revisión acumulativa ahorra lo que motivó HCD-032.


## Alcance
Incluido: un bloque `savings` en la respuesta de `review_start` (MCP) y una línea «ahorro estimado» en `mem review --commit … --since …` (CLI) cuando la revisión tiene `--since` y su delta salió de git: `scope_tokens` (archivos de texto del alcance en la revisión del objetivo), `delta_tokens` (los del alcance que están en el delta), `inherited_tokens` (la lista de heredados que el revisor recibe de más) y `avoided_tokens` = scope − delta − inherited, que puede ser negativo. El cálculo puro vive en `domain/review_savings.go`; el contenido en una revisión se lee con un port nuevo (`application/ports/revision_files.go`) implementado en `adapters/secondary/gitdiff/gitdiff.go` con una sola llamada `git cat-file --batch`; se cuenta con el `ports.TokenCounter` existente. Los archivos binarios no cuentan. Con delta de origen `scope` (objetivo que no es commit) el bloque dice `measurable: false` y el motivo.
Excluido: medir los tokens que de verdad gasta un modelo (proxy de entrada, no consumo real); persistir la medición en el ledger; mostrarla en `review_status`; cambiar qué se hereda o cómo se calcula el delta (HCD-032).

## ¿Qué pasa si no lo hacemos?
La revisión acumulativa existe pero nunca se ha usado en real (0 revisiones con `since_review_id` en el ledger) y no hay forma de saber si ahorra: seguiríamos construyendo sobre una hipótesis. Medirlo con dos ACR reales cuesta justo el gasto que se quiere evitar.

## Alternativa más barata
Un script puntual en el scratchpad que recree dos revisiones encadenadas sobre commits reales y cuente tokens, sin código en el producto. Se descarta porque mide una vez y no queda: con el bloque en `review_start`, cada revisión futura trae su propio dato sin coste de modelo.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Crear el cálculo puro de `savings` en `domain/review_savings.go` → go test ./domain/ -run TestReviewSavings
- [x] 2. Crear el port `application/ports/revision_files.go` e implementarlo en `adapters/secondary/gitdiff/gitdiff.go` (alcance en una revisión, solo texto, un `git cat-file --batch`) → go test ./adapters/secondary/gitdiff/ -run TestTextFilesAt
- [x] 3. Medir el ahorro de una revisión enlazada en `application/usecases/review_savings.go` con el port y el `TokenCounter` → go test ./application/usecases/ -run TestReviewSinceSavings
- [x] 4. Exponer `savings` en `adapters/primary/cli/review/review_since.go` para `review_start` y `mem review --since`, cableado en `adapters/primary/cli/clikit/deps.go` e `infrastructure/container.go` → go test ./adapters/primary/cli/review/ -run TestReviewSince
- [x] 5. Comprobar con el binario real dos revisiones encadenadas sobre git en `tests/integration/review_acumulativa_test.go` → go test ./tests/integration/ -run TestReviewAcumulativa

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 20:10 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain/ -run TestReviewSavings` (salida 1)
      # mem/domain [mem/domain.test]
      domain/review_savings_test.go:9:9: undefined: MeasureSinceSavings
      domain/review_savings_test.go:10:10: undefined: SinceSavings
      domain/review_savings_test.go:15:12: undefined: MeasureSinceSavings
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 20:10 -05:00 · tarea 1 · verde · caducada · `go test ./domain/ -run TestReviewSavings` (salida 0)
      ok  	mem/domain	0.330s
- 2026-11-05 20:10 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/secondary/gitdiff/ -run TestTextFilesAt` (salida 1)
      # mem/adapters/secondary/gitdiff [mem/adapters/secondary/gitdiff.test]
      adapters/secondary/gitdiff/gitdiff_test.go:86:16: d.TextFilesAt undefined (type Diff has no field or method TextFilesAt)
      adapters/secondary/gitdiff/gitdiff_test.go:93:16: d.TextFilesAt undefined (type Diff has no field or method TextFilesAt)
      adapters/secondary/gitdiff/gitdiff_test.go:97:17: d.TextFilesAt undefined (type Diff has no field or method TextFilesAt)
      FAIL	mem/adapters/secondary/gitdiff [build failed]
      FAIL
- 2026-11-05 20:11 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/gitdiff/ -run TestTextFilesAt` (salida 0)
      ok  	mem/adapters/secondary/gitdiff	0.247s
- 2026-11-05 20:11 -05:00 · tarea 3 · rojo observado · caducada · `go test ./application/usecases/ -run TestReviewSinceSavings` (salida 1)
      # mem/application/usecases [mem/application/usecases.test]
      application/usecases/review_savings_test.go:36:9: undefined: ReviewSinceSavings
      application/usecases/review_savings_test.go:51:8: undefined: ReviewSinceSavings
      application/usecases/review_savings_test.go:56:10: undefined: ReviewSinceSavings
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-05 20:11 -05:00 · tarea 3 · verde · caducada · `go test ./application/usecases/ -run TestReviewSinceSavings` (salida 0)
      ok  	mem/application/usecases	0.214s
- 2026-11-05 20:12 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/primary/cli/review/ -run TestReviewSince` (salida 1)
      # mem/adapters/primary/cli/review [mem/adapters/primary/cli/review.test]
      adapters/primary/cli/review/review_since_test.go:79:23: unknown field RevisionFiles in struct literal of type clikit.Deps
      adapters/primary/cli/review/review_since_test.go:80:7: undefined: ahorroDeRevision
      adapters/primary/cli/review/review_since_test.go:81:22: undefined: cargaHeredada
      adapters/primary/cli/review/review_since_test.go:86:10: undefined: ahorroDeRevision
      FAIL	mem/adapters/primary/cli/review [build failed]
      FAIL
- 2026-11-05 20:13 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/primary/cli/review/ -run TestReviewSince` (salida 0)
      ok  	mem/adapters/primary/cli/review	0.158s
- 2026-11-05 20:14 -05:00 · tarea 1 · verde · vigente · `go test ./domain/ -run TestReviewSavings` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 20:14 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/secondary/gitdiff/ -run TestTextFilesAt` (salida 0)
      ok  	mem/adapters/secondary/gitdiff	(cached)
- 2026-11-05 20:14 -05:00 · tarea 3 · verde · vigente · `go test ./application/usecases/ -run TestReviewSinceSavings` (salida 0)
      ok  	mem/application/usecases	(cached)
- 2026-11-05 20:14 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/primary/cli/review/ -run TestReviewSince` (salida 0)
      ok  	mem/adapters/primary/cli/review	(cached)
- 2026-11-05 20:14 -05:00 · tarea 5 · verde · vigente · `go test ./tests/integration/ -run TestReviewAcumulativa` (salida 0)
      ok  	mem/tests/integration	1.403s

## Siguiente
Cerrada con aprobación sobre la revisión 7a05508ef5db.
