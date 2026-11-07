# Cubrir la omisión de concesiones del hook tras el plazo

ID: HCD-054
Tipo: technical
Carril: fast
Motivo: Mejora de cobertura acotada, con mutación reproducida por Splash y sin decisiones abiertas

## Intención
Demostrar que una compresión que termina con el presupuesto ya vencido no inicia la concesión de referencias, y que eliminar la guarda hace fallar la prueba


## Alcance
Incluido: extraer el callback de compresión y concesión de `hookToolOutput` a una función privada que el propio hook use, conservando su comportamiento, para probar ese mismo camino sin ejecutar `os.Exit`. Añadir pruebas con un compresor controlado y un repositorio espía de concesiones. El compresor se bloquea mediante canales; la prueba hace vencer el plazo lógico antes de liberarlo, sin esperas arbitrarias. Cuando devuelve referencias válidas después del vencimiento, no debe iniciarse `GrantAutomaticOriginals` ni invocarse `Grant`, y el callback debe rechazar la salida. Un caso de control dentro del plazo debe conceder las referencias, para evitar que una preparación inválida haga pasar el caso negativo. Comprobar mediante una mutación temporal que retirar exclusivamente la guarda `expired()` hace fallar la prueba nueva por una concesión indebida.

Excluido: cambiar comportamiento de producción, contratos públicos, `domain.HookBudget`, `domain.StoreTimeout`, almacenamiento de originales, política de recuperación o reintentos de HCD-053. No se exige cancelar ni revertir una concesión iniciada antes del vencimiento. No se cambia la firma de `ports.Compressor`, no se añaden dependencias y no se reabre HCD-053. La mutación vive en un directorio temporal y se aplica mediante `go test -overlay`; no se entrega código de producción con la guarda eliminada.

## Antecedentes y evidencia inicial
HCD-053, cerrada sobre `d6376ceecc13`, fijó en Q2 la omisión de concesiones cuando el presupuesto ya venció. Esta unidad preserva esa decisión y mejora su prueba. La revisión Splash posterior no demostró un bug funcional: detectó que `TestToolOutputHint` comprueba un callback sintético y `expired()`, sin observar el camino de concesión del hook.

Splash retiró únicamente la guarda `if expired() { return "", false }` de `adapters/primary/cli/hook_tool_output.go` mediante un archivo temporal y ejecutó `go test -overlay /private/tmp/hcd053-splash-overlay.json ./adapters/primary/cli -run 'TestToolOutputHint|TestRetrieveHintOnce|TestWithBudget' -count=1`: las pruebas siguieron verdes. Ese archivo temporal documenta la observación inicial; la implementación debe generar su propio overlay a partir del código vigente. El criterio de éxito es que la prueba nueva detecte esa mutación y pase con la guarda presente.

Normas aplicables: Constitución §10 (pruebas de comportamiento y determinismo) y §18 (documento HCD único); Playbook `HCD-TASK-01`, `HCD-EVIDENCE-01` y `HCD-TDD-01`. No se declara un fallo funcional de producción para justificar un tipo `fix`.

## ¿Qué pasa si no lo hacemos?
Una regresión que elimine la protección de concesiones tras el plazo puede pasar inadvertida, aun con las pruebas actuales en verde. El contrato acordado en HCD-053 quedaría sin una prueba que observe su efecto real sobre el repositorio de concesiones.

## Alternativa más barata
Conservar la prueba de `expired()` y revisar la guarda manualmente. Evita la extracción privada y las pruebas nuevas, pero la mutación de Splash ya mostró que esa cobertura no detecta su eliminación. La prueba con espías y control positivo aporta una protección automática acotada.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Extraer el callback de compresión y concesión a una función privada compartida por el hook y sus pruebas, conservando su comportamiento y sus dependencias actuales → `go test ./adapters/primary/cli -run 'TestToolOutput|TestRetrieveHintOnce|TestRewriteToolOutput|TestWithBudget' -count=1`
- [x] 2. Añadir `TestToolOutputExpiredCompressionSkipsGrant` y `TestToolOutputWithinBudgetGrants` con compresor controlado, vencimiento lógico y repositorio espía; exigir cero concesiones tras el plazo y una concesión válida dentro de él → `go test -race ./adapters/primary/cli -run 'TestToolOutput(ExpiredCompressionSkipsGrant|WithinBudgetGrants)$' -count=20`
- [x] 3. Demostrar que la prueba nueva detecta la eliminación de la guarda de `adapters/primary/cli/hook_tool_output.go` mediante una mutación aislada en un overlay temporal → ejecutar con `mem hcd verify --expect-fail` la prueba `TestToolOutputExpiredCompressionSkipsGrant` usando `go test -overlay` con solo la guarda eliminada; debe compilar y fallar por llamadas indebidas a `Grant`, y el mismo test sin overlay debe pasar; registrar ambos comandos concretos y su evidencia en el motor
- [x] 4. Validar formato, regresiones del hook y portabilidad del cambio → `test -z "$(gofmt -l .)" && git diff --check && go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 && go test -race ./adapters/primary/cli -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-07 07:42 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run 'TestToolOutput|TestRetrieveHintOnce|TestRewriteToolOutput|TestWithBudget' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.702s
- 2026-11-07 07:43 -05:00 · tarea 2 · rojo observado · vigente · `go test -overlay /private/tmp/hcd054-expiry-overlay.json ./adapters/primary/cli -run '^TestToolOutputExpiredCompressionSkipsGrant$' -count=1` (salida 1)
      --- FAIL: TestToolOutputExpiredCompressionSkipsGrant (0.00s)
          hook_tool_output_grants_test.go:45: Grant se invocó 1 veces tras el plazo; se esperaban cero concesiones
          hook_tool_output_grants_test.go:45: se inició GrantAutomaticOriginals tras el plazo: 1 lecturas
          hook_tool_output_grants_test.go:45: el callback vencido publicó salida o consumió la pista: {content:reducido
              > Marcas ⟦mem⟧ … ref=X: contenido omitido para ahorrar tokens; recupéralo íntegro con pack_retrieve(ref=X) (CLI: mem pack retrieve X).
               ok:true}, pistas=1
      FAIL
      FAIL	mem/adapters/primary/cli	0.297s
      FAIL
- 2026-11-07 07:43 -05:00 · tarea 2 · verde · vigente · `go test -race ./adapters/primary/cli -run 'TestToolOutput(ExpiredCompressionSkipsGrant|WithinBudgetGrants)$' -count=20` (salida 0)
      ok  	mem/adapters/primary/cli	1.609s
- 2026-11-07 07:44 -05:00 · tarea 2,3 · rojo observado · caducada · `go test -overlay /private/tmp/hcd054-expiry-overlay.json ./adapters/primary/cli -run '^TestToolOutputExpiredCompressionSkipsGrant$' -count=1` (salida 1)
      --- FAIL: TestToolOutputExpiredCompressionSkipsGrant (0.00s)
          hook_tool_output_grants_test.go:45: Grant se invocó 1 veces tras el plazo; se esperaban cero concesiones
          hook_tool_output_grants_test.go:45: se inició GrantAutomaticOriginals tras el plazo: 1 lecturas
          hook_tool_output_grants_test.go:45: el callback vencido publicó salida o consumió la pista: {content:reducido
              > Marcas ⟦mem⟧ … ref=X: contenido omitido para ahorrar tokens; recupéralo íntegro con pack_retrieve(ref=X) (CLI: mem pack retrieve X).
               ok:true}, pistas=1
      FAIL
      FAIL	mem/adapters/primary/cli	0.350s
      FAIL
- 2026-11-07 07:44 -05:00 · tarea 1,2,3 · verde · vigente · `go test ./adapters/primary/cli -run 'TestToolOutput|TestRetrieveHintOnce|TestRewriteToolOutput|TestWithBudget' -count=1 && go test -race ./adapters/primary/cli -run 'TestToolOutput(ExpiredCompressionSkipsGrant|WithinBudgetGrants)$' -count=20` (salida 0)
      ok  	mem/adapters/primary/cli	0.524s
      ok  	mem/adapters/primary/cli	1.556s
- 2026-11-07 07:45 -05:00 · tarea 4 · verde · vigente · `test -z "$(gofmt -l .)" && git diff --check && go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 && go test -race ./adapters/primary/cli -count=1` (salida 0)
      ok  	mem/adapters/secondary/gitdiff	0.629s
      ok  	mem/adapters/secondary/hcdfiles	3.125s
      ok  	mem/adapters/secondary/hostbind	0.268s
      ok  	mem/adapters/secondary/launcher	0.567s
      ok  	mem/adapters/secondary/persistence	33.112s
      ok  	mem/adapters/secondary/release	0.364s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.189s
      ok  	mem/adapters/secondary/speckit	0.147s
      ok  	mem/adapters/secondary/tokens	0.120s
      ok  	mem/adapters/secondary/usage	0.120s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.218s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.321s
      ok  	mem/infrastructure	0.551s
      ok  	mem/tests/contract	91.563s
      ok  	mem/tests/integration	43.127s
      ?   	mem/version	[no test files]
      ok  	mem/adapters/primary/cli	60.048s

## Siguiente
Cerrada con aprobación sobre la revisión e95ae1073f69.
