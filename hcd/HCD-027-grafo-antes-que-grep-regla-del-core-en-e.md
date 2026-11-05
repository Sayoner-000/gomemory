# Grafo antes que grep: regla del core en el PreToolUse de los cuatro runtimes

ID: HCD-027
Tipo: feature
Carril: full
Motivo: Decisiones interdependientes (avisar o bloquear; qué es código cubierto) y cambia el comportamiento de herramientas en los cuatro runtimes

## Intención
Que gomemory, y no la persona, haga cumplir que el agente consulte código con los grafos (codegraph, codebase-memory-mcp, search_code/get_symbol) en lugar de grep/rg/find cuando el grafo cubre la consulta, mediante el PreToolUse de mem expand en Claude, Codex, agy y OpenCode


## Alcance
Incluido: una segunda regla de serie del motor de `mem expand` (`domain.BuiltinRules`, junto a `destructive-shell`), evaluada en el `PreToolUse` de shell que ya llega desde los cuatro runtimes (Claude `Bash`, Codex `Bash`, agy `run_command`, OpenCode `tool.execute.before`). Detecta búsquedas de código por shell (`grep`, `rg`, `ag`, `ack`, `git grep`, `find … -name`) y, si el proyecto tiene un grafo que cubre la consulta, responde con la consulta de grafo equivalente. La señal de cobertura la calcula el adaptador y la recibe el dominio (sin E/S en `domain`). Se puede apagar o sobrescribir como cualquier regla (`mem expand off rules`, `overrides:`).
Excluido: las herramientas nativas de búsqueda del host (Grep/Glob de Claude, búsqueda de OpenCode), que no pasan por el `PreToolUse` de shell (unidad posterior si la evidencia lo pide); búsquedas sobre lo que el grafo no indexa (docs, configuración, logs, salida de comandos); la exclusión de AGENTS.md/CLAUDE.md (Q3 de HCD-026).

## ¿Qué pasa si no lo hacemos?
La regla vive solo en una memoria y depende de que el agente la recuerde. Hoy el instinto más usado del proyecto es `git diff` (554 usos) y `git grep` suma 87: el agente lee código a grep, paga la salida entera en tokens y pierde las relaciones (llamadores, radio de impacto) que el grafo ya tiene calculadas.

## Alternativa más barata
Reforzar el texto del protocolo inyectado. Ya existe («prioriza herramientas de grafo frente a lecturas manuales») y no basta: es una recomendación que el agente ignora sin consecuencia. La regla en el hook es la única vía que actúa en el momento de la llamada.

## Historias
### US1. El agente que busca código con grep recibe la consulta de grafo
Como persona que usa gomemory, quiero que cuando el agente lance `grep`/`rg`/`find` sobre código cubierto por un grafo, gomemory intervenga en el `PreToolUse`, para que use el grafo sin que yo tenga que recordárselo.
- C-01: Dado un proyecto con grafo de código disponible, cuando el agente ejecuta `grep -rn "Foo" --include=*.go .` por shell, entonces la regla `graph-before-grep` bloquea el comando (Q1) y el motivo nombra la consulta de grafo equivalente y la salida de escape (`mem expand off rules`) cuando el grafo cubre la búsqueda: existe un grafo del proyecto (`.codegraph/` o el índice propio de gomemory con archivos) y la búsqueda apunta a código de un lenguaje indexado, por filtro (`--include`, `-g`, `-t`) o por rutas de código (Q2).
- C-02: Dado un proyecto SIN grafo de código, cuando el agente ejecuta la misma búsqueda, entonces la regla permite el comando sin intervenir.
- C-03: Dado cualquier proyecto, cuando el agente busca en archivos que el grafo no indexa (`grep foo README.md`, `grep ERROR app.log`, salida canalizada `cmd | grep x`), entonces la regla no interviene.

### US2. La regla se comporta igual en los cuatro runtimes
Como persona que alterna Claude, Codex, agy y OpenCode, quiero el mismo veredicto en todos, para no depender del harness.
- C-04: Dado el mismo comando y proyecto, cuando llega por el `PreToolUse` de Claude, Codex, agy u OpenCode, entonces el veredicto y el motivo son idénticos en el dialecto de cada uno.
- C-05: Dado `mem expand off rules` o una regla propia con `overrides: graph-before-grep`, cuando el agente ejecuta la búsqueda, entonces la regla de serie no actúa.

## Aclaraciones
- [x] Q1. Q1: ¿la regla graph-before-grep avisa (warn) o bloquea (block) una búsqueda de código por shell cuando hay grafo? → Bloquear (Recommended): deniega el comando y el motivo nombra la consulta de grafo equivalente; escape con mem expand off rules u overrides
- [x] Q2. Q2: ¿qué cuenta como grafo que cubre la búsqueda para que graph-before-grep bloquee? → Grafo + lenguaje indexado (Recommended): bloquea si existe un grafo del proyecto (.codegraph/ o índice propio de gomemory con archivos) y la búsqueda apunta a código de un lenguaje indexado (filtro --include/-g/-t de código o rutas de código); README, logs y tuberías pasan

## Slices
### VS-01. La regla de serie graph-before-grep bloquea búsquedas de código por shell (dominio puro)
Tareas: 1, 2, 3
Criterios: C-01, C-03, C-05

### VS-02. El adaptador calcula la cobertura del grafo y la regla actúa igual en los cuatro runtimes
Tareas: 4, 5, 6, 7
Criterios: C-02, C-04

## Checklist
- [x] 1. Añadir la regla de serie `graph-before-grep` (`domain.BuiltinRules`) que bloquea `grep`/`egrep`/`rg`/`ag`/`ack`/`git grep`/`find -name|-iname|-path` cuando un filtro (`--include`, `-g`, `-t`, `-name`) o una ruta de archivo apuntan a una extensión de `RuleInput.IndexedExtensions`, con motivo que nombra la consulta de grafo → `go test ./domain -run TestGraphBeforeGrep_Bloquea`
- [x] 2. La regla no interviene en tuberías (`cmd | grep x`), archivos no indexados (`README.md`, `app.log`) ni con `IndexedExtensions` vacío → `go test ./domain -run TestGraphBeforeGrep_NoInterviene`
- [x] 3. Una regla propia con `overrides: graph-before-grep` anula la de serie → `go test ./domain -run TestGraphBeforeGrep_Override`
- [x] 4. Calcular en el adaptador las extensiones indexadas del proyecto: `.codegraph/` presente → lenguajes de codegraph; si no, índice propio de gomemory con archivos → `.go`; sin grafo → vacío → `go test ./adapters/primary/cli -run TestGraphIndexedExtensions`
- [x] 5. `ExpandRules` propaga las extensiones indexadas a `RuleInput` en el evento bash → `go test ./application/usecases -run TestExpandRules_GraphCoverage`
- [x] 6. El mismo comando produce el mismo bloqueo en los dialectos de Claude, Codex (json), agy y OpenCode, y `mem expand off rules` lo desactiva → `go test ./adapters/primary/cli -run TestGraphBeforeGrep_CuatroRuntimes`
- [x] 7. El binario real bloquea `grep -rn Foo --include=*.go .` en este repo y deja pasar `grep foo README.md` → `go test ./tests/integration -run TestGraphBeforeGrep_BinarioReal`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 05:36 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run TestGraphBeforeGrep_Bloquea` (salida 1)
      # mem/domain [mem/domain.test]
      domain/rules_graph_test.go:15:62: unknown field IndexedExtensions in struct literal of type RuleInput
      domain/rules_graph_test.go:71:88: unknown field IndexedExtensions in struct literal of type RuleInput
      domain/rules_graph_test.go:76:70: unknown field IndexedExtensions in struct literal of type RuleInput
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 05:36 -05:00 · tarea 2 · rojo observado · caducada · `go test ./domain -run TestGraphBeforeGrep_NoInterviene` (salida 1)
      # mem/domain [mem/domain.test]
      domain/rules_graph_test.go:15:62: unknown field IndexedExtensions in struct literal of type RuleInput
      domain/rules_graph_test.go:71:88: unknown field IndexedExtensions in struct literal of type RuleInput
      domain/rules_graph_test.go:76:70: unknown field IndexedExtensions in struct literal of type RuleInput
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 05:36 -05:00 · tarea 3 · rojo observado · caducada · `go test ./domain -run TestGraphBeforeGrep_Override` (salida 1)
      # mem/domain [mem/domain.test]
      domain/rules_graph_test.go:15:62: unknown field IndexedExtensions in struct literal of type RuleInput
      domain/rules_graph_test.go:71:88: unknown field IndexedExtensions in struct literal of type RuleInput
      domain/rules_graph_test.go:76:70: unknown field IndexedExtensions in struct literal of type RuleInput
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-05 05:37 -05:00 · tarea 1 · verde · caducada · `go test ./domain -run TestGraphBeforeGrep_Bloquea` (salida 0)
      ok  	mem/domain	0.182s
- 2026-11-05 05:37 -05:00 · tarea 2 · verde · caducada · `go test ./domain -run TestGraphBeforeGrep_NoInterviene` (salida 0)
      ok  	mem/domain	0.125s
- 2026-11-05 05:37 -05:00 · tarea 3 · verde · caducada · `go test ./domain -run TestGraphBeforeGrep_Override` (salida 0)
      ok  	mem/domain	0.131s
- 2026-11-05 05:38 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestGraphIndexedExtensions` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hook_expand_graph_test.go:20:12: undefined: graphIndexedExtensions
      adapters/primary/cli/hook_expand_graph_test.go:23:12: undefined: graphIndexedExtensions
      adapters/primary/cli/hook_expand_graph_test.go:31:9: undefined: graphIndexedExtensions
      adapters/primary/cli/hook_expand_graph_test.go:69:5: unknown field IndexedExtensions in struct literal of type usecases.ExpandRules
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-05 05:38 -05:00 · tarea 5 · rojo observado · caducada · `go test ./application/usecases -run TestExpandRules_GraphCoverage` (salida 1)
      # mem/application/usecases [mem/application/usecases.test]
      application/usecases/expand_rules_test.go:48:65: unknown field IndexedExtensions in struct literal of type ExpandRules
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-05 05:38 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestGraphBeforeGrep_CuatroRuntimes` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hook_expand_graph_test.go:20:12: undefined: graphIndexedExtensions
      adapters/primary/cli/hook_expand_graph_test.go:23:12: undefined: graphIndexedExtensions
      adapters/primary/cli/hook_expand_graph_test.go:31:9: undefined: graphIndexedExtensions
      adapters/primary/cli/hook_expand_graph_test.go:69:5: unknown field IndexedExtensions in struct literal of type usecases.ExpandRules
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-05 05:38 -05:00 · tarea 7 · rojo observado · caducada · `go test ./tests/integration -run TestGraphBeforeGrep_BinarioReal` (salida 1)
      --- FAIL: TestGraphBeforeGrep_BinarioReal (1.16s)
          hook_expand_test.go:394: mem hook expand [pre-tool --emit=claude] tardó 358.008625ms
          hook_expand_test.go:396: grep de código con grafo debe bloquearse: "{}" 0
      FAIL
      FAIL	mem/tests/integration	1.430s
      FAIL
- 2026-11-05 05:39 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/primary/cli -run TestGraphIndexedExtensions` (salida 0)
      ok  	mem/adapters/primary/cli	0.292s
- 2026-11-05 05:39 -05:00 · tarea 5 · verde · caducada · `go test ./application/usecases -run TestExpandRules_GraphCoverage` (salida 0)
      ok  	mem/application/usecases	0.223s
- 2026-11-05 05:39 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/cli -run TestGraphBeforeGrep_CuatroRuntimes` (salida 0)
      ok  	mem/adapters/primary/cli	0.289s
- 2026-11-05 05:39 -05:00 · tarea 7 · verde · caducada · `go test ./tests/integration -run TestGraphBeforeGrep_BinarioReal` (salida 0)
      ok  	mem/tests/integration	(cached)
- 2026-11-05 05:42 -05:00 · tarea 1 · verde · vigente · `go test ./domain -run TestGraphBeforeGrep_Bloquea` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 05:42 -05:00 · tarea 2 · verde · vigente · `go test ./domain -run TestGraphBeforeGrep_NoInterviene` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 05:42 -05:00 · tarea 3 · verde · vigente · `go test ./domain -run TestGraphBeforeGrep_Override` (salida 0)
      ok  	mem/domain	(cached)
- 2026-11-05 05:42 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/primary/cli -run TestGraphIndexedExtensions` (salida 0)
      ok  	mem/adapters/primary/cli	0.262s
- 2026-11-05 05:42 -05:00 · tarea 5 · verde · vigente · `go test ./application/usecases -run TestExpandRules_GraphCoverage` (salida 0)
      ok  	mem/application/usecases	(cached)
- 2026-11-05 05:42 -05:00 · tarea 6 · verde · vigente · `go test ./adapters/primary/cli -run TestGraphBeforeGrep_CuatroRuntimes` (salida 0)
      ok  	mem/adapters/primary/cli	0.267s
- 2026-11-05 05:42 -05:00 · tarea 7 · verde · vigente · `go test ./tests/integration -run TestGraphBeforeGrep_BinarioReal` (salida 0)
      ok  	mem/tests/integration	(cached)

## Siguiente
Cerrada con aprobación sobre la revisión b6ffd04159e2.
