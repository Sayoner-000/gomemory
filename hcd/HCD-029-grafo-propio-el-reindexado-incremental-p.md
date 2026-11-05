# Grafo propio: el reindexado incremental pierde las aristas entrantes

ID: HCD-029
Tipo: fix
Carril: fast
Motivo: Fallo con reproducción clara (get_symbol EvaluateRules: 3 llamadores incremental vs 15 tras index -force) y arreglo acotado a ReplaceFileNodes, sin decisiones abiertas

## Intención
Que reindexar un archivo conserve las aristas que llegan desde archivos no reindexados, reapuntándolas al nodo nuevo con la misma identidad

## Reproducción
1. `ReplaceFileNodes(db, "proj", "a.go", …, [A])`, `ReplaceFileNodes(db, "proj", "b.go", …, [B])` e `InsertCodeEdges(db, "proj", "a.go", [A→B calls])`.
2. Reindexar solo `b.go` (hash nuevo, mismo símbolo B): `ReplaceFileNodes(db, "proj", "b.go", "h2", [B])`.
3. `Neighbors(nuevo B, calls, "in")` devuelve vacío: la arista A→B desapareció aunque `a.go` no cambió.
En el repositorio real: `get_symbol EvaluateRules` listaba 3 llamadores; tras `mem index -force`, 15 (memoria 1746).

## Alcance
Incluido: `ReplaceFileNodes` (`adapters/secondary/persistence/code_graph.go`): antes de borrar los nodos del archivo guarda las aristas entrantes cuyo origen es otro archivo, con la identidad del destino (kind, name, receiver), y tras insertar los nodos nuevos las reapunta al nodo con la misma identidad; las que ya no casan (símbolo borrado o renombrado) se descartan sin dejar aristas colgantes.
Excluido: la resolución de llamadas del indexador (`resolveIdentCall`/`resolveSelectorCall`); `DeleteCodeFile` (si el archivo desaparece, sus aristas entrantes deben irse); el grafo externo (CodeGraph, codebase-memory-mcp).

## ¿Qué pasa si no lo hacemos?
Cada reindexado incremental (lo hace turn-end en cada turno sobre los .go tocados) borra los llamadores que vienen de otros archivos. `get_symbol` y `list_dependencies` dan llamadores incompletos y el contexto de edición de HCD-028 omite casi todas las pruebas que cubren el archivo: justo el archivo que se está editando es el que pierde sus aristas.

## Alternativa más barata
Forzar `mem index -force` periódicamente. No basta: el grafo vuelve a degradarse en el siguiente turno que toque un archivo, y un reindexado completo cuesta segundos en cada ejecución.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla: reindexar `b.go` conserva las aristas entrantes desde `a.go` con su confianza, y un símbolo que desaparece no deja aristas colgantes ni las cede a otro (guarda del propio arreglo, que hoy pasa porque se borra todo) → `go test -count=1 ./adapters/secondary/persistence -run 'TestReplaceFileNodes_ConservaAristasEntrantes|TestReplaceFileNodes_DescartaAristasSinDestino'`
- [x] 2. En el repositorio real, tras un reindexado incremental de `domain/rules.go`, `get_symbol EvaluateRules` conserva sus llamadores de otros archivos → `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/hcd029/check.sh` (compila el binario desde el árbol de trabajo): proyecto desechable donde a.go llama a B de b.go; se reindexa solo b.go y debe quedar 1 arista calls (rojo sin el arreglo de ReplaceFileNodes; verde con él)
- [x] 3. Sin regresiones → `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 06:09 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./adapters/secondary/persistence -run TestReplaceFileNodes_ConservaAristasEntrantes` (salida 1)
      --- FAIL: TestReplaceFileNodes_ConservaAristasEntrantes (0.02s)
          code_graph_test.go:229: M debe conservar a A como llamador tras reindexar b.go: []
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.641s
      FAIL
- 2026-11-05 06:09 -05:00 · tarea 2 · rojo NO observado (el comando pasó) · caducada · `go test -count=1 ./adapters/secondary/persistence -run TestReplaceFileNodes_DescartaAristasSinDestino` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.170s
- 2026-11-05 06:10 -05:00 · tarea 1 · rojo observado · caducada · `go test -count=1 ./adapters/secondary/persistence -run 'TestReplaceFileNodes_ConservaAristasEntrantes|TestReplaceFileNodes_DescartaAristasSinDestino'` (salida 1)
      --- FAIL: TestReplaceFileNodes_ConservaAristasEntrantes (0.01s)
          code_graph_test.go:229: M debe conservar a A como llamador tras reindexar b.go: []
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.189s
      FAIL
- 2026-11-05 06:11 -05:00 · tarea 1 · verde · caducada · `go test -count=1 ./adapters/secondary/persistence -run 'TestReplaceFileNodes_ConservaAristasEntrantes|TestReplaceFileNodes_DescartaAristasSinDestino'` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.212s
- 2026-11-05 06:11 -05:00 · tarea 2 · verde · caducada · `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...` (salida 0)
      ?   	mem/adapters/secondary/clock	[no test files]
      ok  	mem/adapters/secondary/codegraph	0.146s
      ok  	mem/adapters/secondary/codegraph/codebasememory	0.882s
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.649s
      ok  	mem/adapters/secondary/compression	0.122s
      ok  	mem/adapters/secondary/compression/native	6.842s
      ?   	mem/adapters/secondary/configpath	[no test files]
      ok  	mem/adapters/secondary/expandstate	0.257s
      ok  	mem/adapters/secondary/generator	1.092s
      ok  	mem/adapters/secondary/hcdfiles	0.692s
      ok  	mem/adapters/secondary/launcher	0.538s
      ok  	mem/adapters/secondary/persistence	32.891s
      ok  	mem/adapters/secondary/release	0.377s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.196s
      ok  	mem/adapters/secondary/speckit	0.128s
      ok  	mem/adapters/secondary/tokens	0.114s
      ok  	mem/adapters/secondary/usage	0.116s
      ok  	mem/tests/contract	85.195s
      ok  	mem/tests/integration	37.390s
- 2026-11-05 06:13 -05:00 · tarea 2 · rojo observado · caducada · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/hcd029/check.sh /Users/josegomezj/.local/bin/mem` (salida 1)
      aristas calls tras reindexar solo b.go: 0
- 2026-11-05 06:13 -05:00 · tarea 2 · verde · caducada · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/hcd029/check.sh /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/mem-029` (salida 0)
      aristas calls tras reindexar solo b.go: 1
- 2026-11-05 06:13 -05:00 · tarea 1 · verde · caducada · `go test -count=1 ./adapters/secondary/persistence -run 'TestReplaceFileNodes_ConservaAristasEntrantes|TestReplaceFileNodes_DescartaAristasSinDestino'` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.207s
- 2026-11-05 06:13 -05:00 · tarea 3 · verde · caducada · `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...` (salida 0)
      ?   	mem/adapters/secondary/clock	[no test files]
      ok  	mem/adapters/secondary/codegraph	0.133s
      ok  	mem/adapters/secondary/codegraph/codebasememory	0.977s
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.602s
      ok  	mem/adapters/secondary/compression	0.103s
      ok  	mem/adapters/secondary/compression/native	6.892s
      ?   	mem/adapters/secondary/configpath	[no test files]
      ok  	mem/adapters/secondary/expandstate	0.265s
      ok  	mem/adapters/secondary/generator	1.058s
      ok  	mem/adapters/secondary/hcdfiles	0.704s
      ok  	mem/adapters/secondary/launcher	0.520s
      ok  	mem/adapters/secondary/persistence	34.235s
      ok  	mem/adapters/secondary/release	0.369s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.155s
      ok  	mem/adapters/secondary/speckit	0.107s
      ok  	mem/adapters/secondary/tokens	0.100s
      ok  	mem/adapters/secondary/usage	0.098s
      ok  	mem/tests/contract	86.345s
      ok  	mem/tests/integration	37.480s
- 2026-11-05 06:15 -05:00 · tarea 2 · rojo observado · caducada · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/hcd029/check.sh` (salida 1)
      aristas calls tras reindexar solo b.go: 0
- 2026-11-05 06:15 -05:00 · tarea 2 · verde · caducada · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/hcd029/check.sh` (salida 0)
      aristas calls tras reindexar solo b.go: 1
- 2026-11-05 06:15 -05:00 · tarea 1 · verde · caducada · `go test -count=1 ./adapters/secondary/persistence -run 'TestReplaceFileNodes_ConservaAristasEntrantes|TestReplaceFileNodes_DescartaAristasSinDestino'` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.211s
- 2026-11-05 06:15 -05:00 · tarea 3 · verde · caducada · `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...` (salida 0)
      ?   	mem/adapters/secondary/clock	[no test files]
      ok  	mem/adapters/secondary/codegraph	0.132s
      ok  	mem/adapters/secondary/codegraph/codebasememory	0.983s
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.636s
      ok  	mem/adapters/secondary/compression	0.115s
      ok  	mem/adapters/secondary/compression/native	6.658s
      ?   	mem/adapters/secondary/configpath	[no test files]
      ok  	mem/adapters/secondary/expandstate	0.283s
      ok  	mem/adapters/secondary/generator	1.118s
      ok  	mem/adapters/secondary/hcdfiles	0.656s
      ok  	mem/adapters/secondary/launcher	0.535s
      ok  	mem/adapters/secondary/persistence	32.077s
      ok  	mem/adapters/secondary/release	0.347s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.156s
      ok  	mem/adapters/secondary/speckit	0.103s
      ok  	mem/adapters/secondary/tokens	0.084s
      ok  	mem/adapters/secondary/usage	0.086s
      ok  	mem/tests/contract	87.616s
      ok  	mem/tests/integration	37.381s
- 2026-11-05 06:17 -05:00 · tarea 2 · rojo observado · caducada · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/hcd029/check.sh` (salida 1)
      aristas calls tras reindexar solo b.go: 0
- 2026-11-05 06:17 -05:00 · tarea 2 · verde · vigente · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/f3396a6b-475e-48fd-a227-857fac2634b5/scratchpad/hcd029/check.sh` (salida 0)
      aristas calls tras reindexar solo b.go: 1
- 2026-11-05 06:17 -05:00 · tarea 1 · verde · vigente · `go test -count=1 ./adapters/secondary/persistence -run 'TestReplaceFileNodes_ConservaAristasEntrantes|TestReplaceFileNodes_DescartaAristasSinDestino'` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.221s
- 2026-11-05 06:17 -05:00 · tarea 3 · verde · vigente · `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...` (salida 0)
      ?   	mem/adapters/secondary/clock	[no test files]
      ok  	mem/adapters/secondary/codegraph	0.150s
      ok  	mem/adapters/secondary/codegraph/codebasememory	1.124s
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.623s
      ok  	mem/adapters/secondary/compression	0.128s
      ok  	mem/adapters/secondary/compression/native	6.861s
      ?   	mem/adapters/secondary/configpath	[no test files]
      ok  	mem/adapters/secondary/expandstate	0.244s
      ok  	mem/adapters/secondary/generator	1.047s
      ok  	mem/adapters/secondary/hcdfiles	0.657s
      ok  	mem/adapters/secondary/launcher	0.537s
      ok  	mem/adapters/secondary/persistence	31.816s
      ok  	mem/adapters/secondary/release	0.385s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.249s
      ok  	mem/adapters/secondary/speckit	0.193s
      ok  	mem/adapters/secondary/tokens	0.112s
      ok  	mem/adapters/secondary/usage	0.119s
      ok  	mem/tests/contract	86.607s
      ok  	mem/tests/integration	37.392s

## Siguiente
Cerrada con aprobación sobre la revisión 69ac3bf141e6.
