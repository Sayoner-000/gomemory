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
- [ ] 1. La reproducción ya no falla: reindexar `b.go` conserva la arista A→B desde `a.go` → `go test -count=1 ./adapters/secondary/persistence -run TestReplaceFileNodes_ConservaAristasEntrantes`
- [ ] 2. Un símbolo que desaparece al reindexar no deja aristas colgantes ni reapunta a otro símbolo → `go test -count=1 ./adapters/secondary/persistence -run TestReplaceFileNodes_DescartaAristasSinDestino`
- [ ] 3. Sin regresiones → `go test -count=1 ./domain/... ./application/... ./adapters/... ./tests/...`
- [ ] 4. En el repositorio real, tras un reindexado incremental de `domain/rules.go`, `get_symbol EvaluateRules` conserva sus llamadores de otros archivos → manual (comparar antes y después con el binario de la rama)

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 06:09 -05:00 · tarea 1 · rojo observado · vigente · `go test -count=1 ./adapters/secondary/persistence -run TestReplaceFileNodes_ConservaAristasEntrantes` (salida 1)
      --- FAIL: TestReplaceFileNodes_ConservaAristasEntrantes (0.02s)
          code_graph_test.go:229: M debe conservar a A como llamador tras reindexar b.go: []
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.641s
      FAIL
- 2026-11-05 06:09 -05:00 · tarea 2 · rojo NO observado (el comando pasó) · vigente · `go test -count=1 ./adapters/secondary/persistence -run TestReplaceFileNodes_DescartaAristasSinDestino` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.170s

## Siguiente
pendiente
