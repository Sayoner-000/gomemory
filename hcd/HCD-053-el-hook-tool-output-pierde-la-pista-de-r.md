# El hook tool-output pierde la pista de recuperación y sus pruebas fallan bajo carga

ID: HCD-053
Tipo: fix
Carril: fast
Motivo: Fix con reproducción en CI y causa localizada; la persona eligió la Vía rápida

## Intención
Que la pista de recuperación del hook tool-output solo se marque como mostrada cuando la salida se entrega de verdad, y que las pruebas que lo ejecutan y la del tope de originales no fallen por la carga del runner sin relajar el presupuesto de 150 ms del hook

## Reproducción
El CI de 7112dd9 (2026-11-07) falló en `TestHCD014CompactionContinuity/recovery_hint` (`tests/integration/hcd014_compaction_test.go:38`): el hook `tool-output` devolvió la salida vacía, sin `ref=`. En `adapters/primary/cli/hook_tool_output.go`, `withBudget(domain.HookBudget, …)` descarta la salida si pasan 150 ms, y en un runner cargado pasa. Además, `retrieveHintOnce` escribe el marcador de pista mostrada dentro de la compresión, antes de saber si la salida se entrega: si el plazo vence después de escribirlo, la pista nunca llega al agente en esa conversación. `TestCompressKeepsEveryRefWithSmallConfiguredCap` (`adapters/secondary/persistence`) falla de forma intermitente por la misma familia: guarda unos 1,8 MB de originales dentro de `domain.StoreTimeout` (500 ms). Prueba del defecto de la pista: `go test ./adapters/primary/cli -run TestToolOutputHint -count=1`.

## Alcance
Incluido: la pista de recuperación se calcula sin escribir nada. El marcador se escribe solo si la salida se entrega con la pista, justo antes de escribirla en stdout. La pista va en un único campo: el primero que se comprime al recorrer los campos en orden alfabético, para que sea determinista (hallazgo 3 de splash). Si el presupuesto ya venció, la concesión de referencias (`GrantAutomaticOriginals`) se omite; los originales ya guardados quedan, son contenido válido que el LRU retira, y se documenta (Q2). Las pruebas que ejecutan el hook real y exigen compresión (`tests/contract/hook_tool_output_test.go`, `tests/integration/hcd014_compaction_test.go` y `tests/integration/hcd014_isolation_test.go`) repiten el hook hasta 5 veces cuando la salida llega vacía por el presupuesto (Q1). `TestCompressKeepsEveryRefWithSmallConfiguredCap` da al motor un `StoreTimeout` amplio, porque mide el tope de tamaño, no la latencia.
Excluido: el presupuesto de 150 ms (`domain.HookBudget`) y el `StoreTimeout` de producción no cambian, y no se añade ningún ajuste. `tests/contract/hook_tool_output_safety_test.go` no se repite: allí una salida vacía es válida y reintentar no cambia el veredicto (hallazgo 7). Si dos hooks corren en paralelo, los dos pueden mostrar la pista; repetirla es inocuo y se acepta (hallazgo 4). Pasar contexto a `Compress` para cancelar el guardado tocaría la excepción E-002 de la Constitución y queda para otra unidad.

## ¿Qué pasa si no lo hacemos?
El CI de Linux falla de vez en cuando sin cambios de código, como en 7112dd9, y cada fallo obliga a investigar o a relanzar. En uso real, si el hook se pasa del plazo justo después de marcar la pista, el agente no sabrá cómo recuperar lo omitido en esa conversación.

## Alternativa más barata
Relanzar el job cuando falle. No cuesta nada en código, pero deja el CI poco fiable y no corrige la pista perdida, que es un defecto de producto.


## Aclaraciones
- [x] Q1. ¿Cómo hacen las pruebas que ejecutan el hook tool-output para no fallar cuando el runner se pasa del presupuesto de 150 ms? → Reintentar en las pruebas (Recommended): las pruebas repiten el hook hasta 5 veces cuando la salida llega vacía por el presupuesto; sin ajuste nuevo en producción, el plazo real de 150 ms sigue probado, y el arreglo de la pista hace seguro el reintento.
- [x] Q2. Cuando vence el presupuesto del hook y la salida se descarta, ¿qué se hace con los originales y las concesiones que la compresión ya guardó? → No conceder tras el plazo (Recommended): si el presupuesto ya venció, la concesión de referencias se omite, para no conceder referencias que nadie vio; los originales ya guardados quedan (contenido válido que el LRU retira) y se documenta.

## Checklist
- [x] 1. La reproducción ya no falla: la pista solo se marca al entregarse, va en un único campo determinista y no se concede tras el plazo; la prueba nueva `TestToolOutputHint` falla antes del arreglo → `go test ./adapters/primary/cli -run TestToolOutputHint -count=1` en `adapters/primary/cli/hook_tool_output_test.go`
- [x] 2. Reintentar el hook en las pruebas de `tests/contract` y `tests/integration` que exigen compresión cuando la salida llega vacía por el presupuesto (depende de la tarea 1: sin ella, un intento perdido consume la pista) → `go test ./tests/contract -run ToolOutput -count=3 && go test ./tests/integration -run HCD014 -count=3`
- [x] 3. Dar a `TestCompressKeepsEveryRefWithSmallConfiguredCap` en `adapters/secondary/persistence` un `StoreTimeout` amplio → `go test ./adapters/secondary/persistence -run TestCompressKeepsEveryRefWithSmallConfiguredCap -count=5`
- [x] 4. Mantener en verde la suite, el detector de carreras, vet y la compilación de Windows → `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 && go test -race ./adapters/primary/cli -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-07 07:24 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestToolOutputHint -count=1` (salida 1)
      --- FAIL: TestToolOutputHint (0.15s)
          hook_tool_output_test.go:50: la nota perdida por el plazo debe llegar en la siguiente entrega: "salida"
      FAIL
      FAIL	mem/adapters/primary/cli	0.815s
      FAIL
- 2026-11-07 07:26 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run 'TestToolOutputHint|TestRetrieveHintOnce' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.434s
- 2026-11-07 07:26 -05:00 · tarea 2 · verde · caducada · `go test ./tests/contract -run ToolOutput -count=3 && go test ./tests/integration -run HCD014 -count=3` (salida 0)
      ok  	mem/tests/contract	5.250s
      ok  	mem/tests/integration	3.492s
- 2026-11-07 07:27 -05:00 · tarea 2 · verde · caducada · `go test ./tests/contract -run ToolOutput -count=3 && go test ./tests/integration -run HCD014 -count=3` (salida 0)
      ok  	mem/tests/contract	5.199s
      ok  	mem/tests/integration	3.535s
- 2026-11-07 07:27 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/secondary/persistence -run TestCompressKeepsEveryRefWithSmallConfiguredCap -count=5` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.679s
- 2026-11-07 07:27 -05:00 · tarea 1,2 · verde · vigente · `go test ./adapters/primary/cli -run 'TestToolOutputHint|TestRetrieveHintOnce' -count=1 && go test ./tests/contract -run ToolOutput -count=3 && go test ./tests/integration -run HCD014 -count=3` (salida 0)
      ok  	mem/adapters/primary/cli	0.452s
      ok  	mem/tests/contract	5.800s
      ok  	mem/tests/integration	3.409s
- 2026-11-07 07:28 -05:00 · tarea 4 · verde · vigente · `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 && go test -race ./adapters/primary/cli -count=1` (salida 0)
      ok  	mem/adapters/secondary/gitdiff	0.615s
      ok  	mem/adapters/secondary/hcdfiles	3.100s
      ok  	mem/adapters/secondary/hostbind	0.270s
      ok  	mem/adapters/secondary/launcher	0.571s
      ok  	mem/adapters/secondary/persistence	33.121s
      ok  	mem/adapters/secondary/release	0.376s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.175s
      ok  	mem/adapters/secondary/speckit	0.138s
      ok  	mem/adapters/secondary/tokens	0.144s
      ok  	mem/adapters/secondary/usage	0.149s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.517s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.228s
      ok  	mem/infrastructure	0.518s
      ok  	mem/tests/contract	91.865s
      ok  	mem/tests/integration	44.185s
      ?   	mem/version	[no test files]
      ok  	mem/adapters/primary/cli	59.347s

## Siguiente
Cerrada con aprobación sobre la revisión d6376ceecc13.
