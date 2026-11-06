# El arranque registra como entregados los bloques que caben en el recorte

ID: HCD-037
Tipo: technical
Carril: fast
Motivo: Mejora técnica medible y acotada al registro de entregas existente, sin decisiones interdependientes

## Intención
Cuando el contexto de arranque entrega bloques completos dentro del recorte, anotarlos como entregados en el registro de la sesión para que get_context/get_plan_context posteriores no los reenvíen; elimina la otra mitad del contexto duplicado medido.


## Alcance
Incluido: el contexto de arranque (`entregaContextoDeArranque` en `adapters/primary/cli/cmd_hook.go`), cuando la salida supera `domain.HookInlineContextMaxChars` y `FitHookOutput` la recorta. Las entradas de memoria (unidades de `SessionDelta.ApplyContext`) que llegan completas en el texto emitido se anotan en `delivered_blocks`. Las que el recorte deja fuera, o corta a medias, no se anotan. Así, el siguiente `get_context` / `get_plan_context` solo sustituye por `⟦ya entregado⟧` lo que el agente vio de verdad. Va con pruebas que reproducen el recorte real. La comparación normaliza los espacios igual que la compresión estructural, que en nivel max colapsa la sangría del texto emitido. Por Q1 también entra la granularidad del recorte: cuando el párrafo que desborda es una lista de entradas (`- `), `FitHookOutput` corta en la última entrada completa que cabe, en vez de descartar la sección entera.
Excluido: anotar el hash del canal `context` en `DeliveryLog` cuando hay recorte, porque la supresión completa de `get_plan_context` tiene que seguir apagada. Tampoco entran: cambiar el tope del canal o el orden de prioridades de `FitHookOutput`; los niveles de compresión distintos de max (el delta solo actúa en max); el canal post-compact y el del primer prompt; y las ideas pendientes (acortar el bootstrap, unidades B y C).

## ¿Qué pasa si no lo hacemos?
Cuando el contexto de arranque no cabe en el tope del canal (el caso habitual en este proyecto: la salida termina en «… contexto recortado»), hoy no se anota nada. El primer `get_context` de la sesión, que el protocolo exige, vuelve a enviar entero lo que el agente ya recibió al arrancar (baseline, reglas y las primeras secciones de memorias). Es la segunda mitad del contexto duplicado que se midió al empezar (la primera la cerró HCD-036). El coste en tokens se repite en cada sesión.

## Alternativa más barata
1. Recortar el contexto de arranque hasta que quepa siempre. Así se anotaría todo con el código actual, pero el agente perdería memorias al arrancar y eso degrada el arranque.
2. Anotar todo el documento aunque se recorte. Es el defecto que corrigió C-002 de acr_961a1676: se marcaría lo que el agente nunca leyó.
Ninguna sale más barata sin introducir un defecto. El cambio propuesto es pequeño: reutiliza `SessionDelta` y `Mark`, y solo cambia qué texto se anota.


## Aclaraciones
- [x] Q1. El recorte del arranque descarta la sección «Decisiones Técnicas» entera (cutAtParagraph corta solo en párrafos y una sección es un único párrafo), así que en este proyecto solo llega completo el baseline y HCD-037 solo ahorra unos 2.000 caracteres en get_context por MCP. ¿Se amplía el alcance para que el recorte corte por entradas? → Cortar por entradas: si el párrafo que desborda es una lista de entradas, FitHookOutput corta en la última entrada completa que cabe; se añade una tarea con su prueba.

## Checklist
- [x] 1. Añadir `SessionDelta.MarkVisible(ctx, doc, visible)`, que anota solo las unidades de `doc` cuyo texto completo aparece en `visible` → `go test ./application/usecases -run TestSessionDelta_MarkVisible` (incluye una unidad cortada a medias que no se anota)
- [x] 2. Hacer que `entregaContextoDeArranque`, cuando la salida supera el tope, calcule lo emitido con `domain.FitHookOutput` sobre la misma sección que emite `hookSessionStart` y llame a `MarkVisible`, sin registrar `DeliveryLog` → `go test ./adapters/primary/cli -run TestEntregaContextoDeArranque` (la prueba de truncado pasa a exigir: entradas visibles anotadas, entrada recortada sin anotar, canal `context` sin registro)
- [x] 3. Comprobar de punta a punta que, tras un arranque recortado, `get_context` sustituye por `⟦ya entregado⟧` solo lo visible y entrega completo el resto → `go test ./adapters/primary/cli -run TestArranqueRecortado_GetContextSoloSuprimeLoVisto`
- [x] 4. Hacer que el recorte de una sección corte en la última entrada completa de una lista cuando el párrafo que desborda no cabe → `go test ./domain -run TestFitHookOutput_CortaPorEntradas` (las entradas que caben salen completas, ninguna sale a medias y el aviso de recorte se conserva)
- [x] 5. Medir con el binario nuevo sobre una copia aislada del almacén de este proyecto: `mem hook session-start` seguido de `mem context` → se observa un número de marcas `⟦ya entregado⟧` mayor que 0 y la reducción de caracteres frente a `mem context --full`
- [x] 6. Ejecutar la suite completa sin regresiones → `go test ./...`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 21:31 -05:00 · tarea 1 · rojo observado · caducada · `go test ./application/usecases -run TestSessionDelta_MarkVisible` (salida 1)
      # mem/application/usecases [mem/application/usecases.test]
      application/usecases/session_delta_test.go:98:4: d.MarkVisible undefined (type SessionDelta has no field or method MarkVisible)
      application/usecases/session_delta_test.go:109:8: vacio.MarkVisible undefined (type SessionDelta has no field or method MarkVisible)
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-05 21:32 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run TestSessionDelta_MarkVisible` (salida 0)
      ok  	mem/application/usecases	0.379s
- 2026-11-05 21:32 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestEntregaContextoDeArranque` (salida 1)
      --- FAIL: TestEntregaContextoDeArranque_SalidaTruncadaAnotaSoloLoVisible (0.00s)
          cmd_hook_delivery_test.go:174: A y B llegaron completas y deben sustituirse:
              # Memoria del Proyecto
              ## Decisiones Técnicas
              - **A**: uno
              - **B**: dos
              ## Bugfixes
              - **C**: tres xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
      FAIL
      FAIL	mem/adapters/primary/cli	0.322s
      FAIL
- 2026-11-05 21:32 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestEntregaContextoDeArranque` (salida 0)
      ok  	mem/adapters/primary/cli	0.321s
- 2026-11-05 21:33 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run TestArranqueRecortado_GetContextSoloSuprimeLoVisto` (salida 0)
      ok  	mem/adapters/primary/cli	0.316s
- 2026-11-05 21:37 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run TestSessionDelta_MarkVisible` (salida 0)
      ok  	mem/application/usecases	0.222s
- 2026-11-05 21:37 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestEntregaContextoDeArranque` (salida 0)
      ok  	mem/adapters/primary/cli	0.318s
- 2026-11-05 21:37 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run TestArranqueRecortado_GetContextSoloSuprimeLoVisto` (salida 0)
      ok  	mem/adapters/primary/cli	0.308s
- 2026-11-05 21:40 -05:00 · tarea 4 · rojo observado · caducada · `go test ./domain -run TestFitHookOutput_CortaPorEntradas` (salida 1)
      --- FAIL: TestFitHookOutput_CortaPorEntradas (0.00s)
          hook_budget_test.go:86: las entradas que caben deben salir completas: "## Reglas\n\nregla fija\n\n## Decisiones Técnicas\n\n… contexto recortado: llama get_context() para el resto"
      FAIL
      FAIL	mem/domain	0.357s
      FAIL
- 2026-11-05 21:40 -05:00 · tarea 4 · verde · vigente · `go test ./domain -run TestFitHookOutput_CortaPorEntradas` (salida 0)
      ok  	mem/domain	0.161s
- 2026-11-05 21:41 -05:00 · tarea 5 · manual · vigente
      Copia aislada del almacén de este proyecto (GOMEMORY_DATA_HOME en scratchpad, 1298 memorias). Binario 2.49 instalado: arranque 5552 caracteres, 0 entradas de Decisiones Técnicas visibles, 0 bloques anotados; mem context 14724 = full 14724, 0 marcas. Binario nuevo: arranque 9932 caracteres, 35 entradas visibles, 36 bloques anotados (35 + baseline); mem context 8663 frente a full 14724 (-6061 caracteres, -41 %), marca «35 entradas ya entregadas». Las 49 entradas del documento llegan completas en el arranque o enteras en context; el corte cae entre dos entradas.
- 2026-11-05 21:41 -05:00 · tarea 6 · verde · vigente · `go test ./...` (salida 0)
      ok  	mem/adapters/secondary/generator	1.259s
      ok  	mem/adapters/secondary/gitdiff	(cached)
      ok  	mem/adapters/secondary/hcdfiles	0.722s
      ok  	mem/adapters/secondary/hostbind	(cached)
      ok  	mem/adapters/secondary/launcher	0.569s
      ok  	mem/adapters/secondary/persistence	32.675s
      ok  	mem/adapters/secondary/release	0.372s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.190s
      ok  	mem/adapters/secondary/speckit	0.151s
      ok  	mem/adapters/secondary/tokens	(cached)
      ok  	mem/adapters/secondary/usage	0.121s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.445s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.247s
      ok  	mem/infrastructure	0.689s
      ok  	mem/tests/contract	103.465s
      ok  	mem/tests/integration	57.982s
      ?   	mem/version	[no test files]
- 2026-11-05 21:43 -05:00 · tarea 1 · verde · vigente · `go test ./application/usecases -run TestSessionDelta_MarkVisible` (salida 0)
      ok  	mem/application/usecases	(cached)
- 2026-11-05 21:43 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/primary/cli -run TestEntregaContextoDeArranque` (salida 0)
      ok  	mem/adapters/primary/cli	0.284s
- 2026-11-05 21:43 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli -run TestArranqueRecortado_GetContextSoloSuprimeLoVisto` (salida 0)
      ok  	mem/adapters/primary/cli	0.354s
- 2026-11-05 21:46 -05:00 · tarea 5 · manual · vigente
      Medición por MCP (mem mcp por stdio, copia aislada): binario 2.49 → arranque 5552 + get_context 17835 = 23387 caracteres; binario nuevo → arranque 9932 + get_context 11774 = 21706 (−1681, −7,2 % por sesión; get_context −34 %). Por MCP y por CLI se quitan las mismas 68 líneas (6435 caracteres: baseline + 35 entradas) y se añaden 4 de marcadores; la CLI no conserva el baseline.

## Siguiente
Cerrada con aprobación sobre la revisión 024c0cb5b886.
