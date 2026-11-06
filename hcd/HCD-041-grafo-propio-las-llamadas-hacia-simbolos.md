# Grafo propio: las llamadas hacia símbolos nuevos quedan sin resolver

ID: HCD-041
Tipo: fix
Carril: fast
Motivo: fix con reproducción determinista; el límite residual de HCD-029; la forma del arreglo se decide en clarify

## Intención
Que una llamada desde un archivo no tocado hacia un símbolo que aparece después en otro archivo se resuelva en el reindexado incremental, sin mem index -force. Hoy resolveEdges (application/usecases/index_project.go) solo resuelve hacia símbolos que ya existen y no guarda las llamadas sin resolver. Memoria 1753.

## Reproducción
Con el binario real v2.49.1, en un proyecto desechable y con un almacén aislado (`GOMEMORY_DATA_HOME`):
1. `a.go` define `func A() { F() }` y `F` no existe: `mem index -skip-graph` → 0 aristas `calls` (lo esperado).
2. Añadir `b.go` con `func F() {}` y volver a ejecutar `mem index -skip-graph` (incremental: `a.go` no cambia) → **0 aristas A→F**.
3. `mem index -skip-graph -force` → 1 arista A→F.

Reproducción automatizada (para el rojo y el verde): una prueba del indexador en `application/usecases` que indexa `a.go`, añade `b.go`, reindexa en modo incremental y exige la arista A→F. La comprobación exacta se fija en las tareas.

## Alcance
Incluido: que el reindexado incremental (`IndexProject` e `IndexFiles`) resuelva las llamadas de archivos no tocados hacia símbolos que aparecen después, con las mismas reglas de `resolveIdentCall` y `resolveSelectorCall`; sus pruebas; y la verificación con el binario real.
Excluido: el grafo externo (codegraph, codebase-memory-mcp); la resolución con tipos (sigue siendo best-effort); la conservación de aristas entrantes al reindexar, que ya está cerrada en HCD-029.
Decidido en Q1: una tabla de pendientes, `code_pending_calls`, creada con una migración aditiva. Al indexar un archivo se reemplazan sus pendientes. Al insertar símbolos nuevos se resuelven las pendientes con ese nombre, con las mismas reglas, y se borran. Las bases existentes se completan con un único `mem index -force`.

## ¿Qué pasa si no lo hacemos?
`get_symbol`, `list_dependencies` y el contexto de edición (HCD-028) muestran menos llamadores de los reales para todo símbolo creado después de que se indexara su llamador. Es justo el caso de escribir primero la llamada y después la función. El grafo solo se corrige con `mem index -force`, que nadie ejecuta en el día a día.

## Alternativa más barata
Ejecutar `mem index -force` de vez en cuando. No basta: el defecto vuelve en el siguiente símbolo nuevo, y un reindexado completo cuesta segundos en cada ejecución (es la misma razón por la que se descartó en HCD-029).


## Aclaraciones
- [x] Q1. ¿Cómo se recuerdan las llamadas sin resolver para resolverlas cuando aparezca el símbolo en otro archivo? → Tabla de pendientes (Recommended): nueva tabla code_pending_calls (archivo origen, nodo que llama, nombre/alias). Al indexar un archivo se reemplazan sus pendientes; al insertar símbolos nuevos se resuelven las pendientes con ese nombre usando las mismas reglas (resolveIdentCall/resolveSelectorCall) y se borran. Migración aditiva (CREATE TABLE IF NOT EXISTS); las bases existentes se completan con un mem index -force una sola vez.

## Checklist
- [x] 1. La reproducción ya no falla → re-ejecutar la reproducción
- [x] 2. Una llamada pendiente desaparece cuando su archivo la quita o se borra, y las llamadas a builtins no se guardan (application/usecases/index_project_test.go) → go test ./application/usecases -run TestIndexIncremental_PendienteRetiradaNoSeResuelve -count=1
- [x] 3. Una llamada con selector hacia un paquete importado se resuelve cuando aparece el símbolo (application/usecases/index_project_test.go) → go test ./application/usecases -run TestIndexIncremental_ResuelveSelectorHaciaSimboloNuevo -count=1
- [x] 4. Sin regresiones en el indexador, la persistencia y los contratos (application, adapters/secondary/persistence, tests) → go vet ./... && go test ./application/... ./adapters/secondary/persistence/... ./adapters/primary/cli/... ./tests/... -count=1
- [x] 5. La reproducción con el binario real da 1 arista A→F sin -force → manual: los mismos pasos 1 y 2 de la Reproducción con el binario de la rama

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 05:55 -05:00 · tarea 1 · rojo observado · caducada · `go test ./application/usecases -run TestIndexIncremental_ResuelveLlamadaHaciaSimboloNuevo -count=1` (salida 1)
      --- FAIL: TestIndexIncremental_ResuelveLlamadaHaciaSimboloNuevo (0.05s)
          --- FAIL: TestIndexIncremental_ResuelveLlamadaHaciaSimboloNuevo/IndexProject (0.03s)
              index_pending_calls_test.go:53: esperaba A→F tras indexar b.go, got []
          --- FAIL: TestIndexIncremental_ResuelveLlamadaHaciaSimboloNuevo/IndexFiles (0.02s)
              index_pending_calls_test.go:53: esperaba A→F tras indexar b.go, got []
      FAIL
      FAIL	mem/application/usecases	0.496s
      FAIL
- 2026-11-06 05:55 -05:00 · tarea 3 · rojo observado · caducada · `go test ./application/usecases -run TestIndexIncremental_ResuelveSelectorHaciaSimboloNuevo -count=1` (salida 1)
      --- FAIL: TestIndexIncremental_ResuelveSelectorHaciaSimboloNuevo (0.02s)
          index_pending_calls_test.go:108: esperaba UseHelper→Helper tras indexar sub.go, got []
      FAIL
      FAIL	mem/application/usecases	0.202s
      FAIL
- 2026-11-06 05:56 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run TestIndexIncremental_ResuelveLlamadaHaciaSimboloNuevo -count=1` (salida 0)
      ok  	mem/application/usecases	0.216s
- 2026-11-06 05:56 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run TestIndexIncremental_PendienteRetiradaNoSeResuelve -count=1` (salida 0)
      ok  	mem/application/usecases	0.195s
- 2026-11-06 05:56 -05:00 · tarea 3 · verde · caducada · `go test ./application/usecases -run TestIndexIncremental_ResuelveSelectorHaciaSimboloNuevo -count=1` (salida 0)
      ok  	mem/application/usecases	0.168s
- 2026-11-06 05:56 -05:00 · tarea 4 · falló · caducada · `go vet ./... && go test ./application/... ./adapters/secondary/persistence/... ./adapters/primary/cli/... ./tests/... -count=1` (salida 1)
              col|usage_records|0|id|INTEGER|0||1
              col|usage_records|1|project|TEXT|1||0
              col|usage_records|2|session_id|TEXT|0||0
              col|usage_records|3|operation|TEXT|1||0
              col|usage_records|4|channel|TEXT|1||0
              col|usage_records|5|baseline_tokens|INTEGER|1|0|0
              col|usage_records|6|emitted_tokens|INTEGER|1|0|0
              col|usage_records|7|created_at|TEXT|1|datetime('now', '-5 hours')|0
      --- FAIL: TestSchemaGolden_VersionBumpsWithCatalog (0.01s)
          db_schema_golden_test.go:124: schema changed without bumping schemaVersion
      FAIL
      FAIL	mem/adapters/secondary/persistence	28.349s
      ok  	mem/adapters/primary/cli	18.937s
      ok  	mem/adapters/primary/cli/clikit	0.202s
      ok  	mem/adapters/primary/cli/hcd	2.710s
      ok  	mem/adapters/primary/cli/review	2.059s
      ok  	mem/adapters/primary/cli/socratica	1.837s
      ok  	mem/tests/contract	97.974s
      ok  	mem/tests/integration	50.438s
      FAIL
- 2026-11-06 05:59 -05:00 · tarea 1 · verde · vigente · `go test ./application/usecases -run TestIndexIncremental_ResuelveLlamadaHaciaSimboloNuevo -count=1` (salida 0)
      ok  	mem/application/usecases	0.242s
- 2026-11-06 05:59 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases -run TestIndexIncremental_PendienteRetiradaNoSeResuelve -count=1` (salida 0)
      ok  	mem/application/usecases	0.204s
- 2026-11-06 05:59 -05:00 · tarea 3 · verde · vigente · `go test ./application/usecases -run TestIndexIncremental_ResuelveSelectorHaciaSimboloNuevo -count=1` (salida 0)
      ok  	mem/application/usecases	0.206s
- 2026-11-06 05:59 -05:00 · tarea 4 · verde · vigente · `go vet ./... && go test ./application/... ./adapters/secondary/persistence/... ./adapters/primary/cli/... ./tests/... -count=1` (salida 0)
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	17.386s
      ok  	mem/adapters/secondary/persistence	31.204s
      ok  	mem/adapters/primary/cli	19.748s
      ok  	mem/adapters/primary/cli/clikit	0.256s
      ok  	mem/adapters/primary/cli/hcd	3.585s
      ok  	mem/adapters/primary/cli/review	2.748s
      ok  	mem/adapters/primary/cli/socratica	2.450s
      ok  	mem/tests/contract	104.612s
      ok  	mem/tests/integration	59.327s
- 2026-11-06 06:01 -05:00 · tarea 5 · manual · vigente
      Binario de la rama (go build ./infrastructure), almacén aislado con GOMEMORY_DATA_HOME. Escenario A: indexar a.go (A llama a F), añadir b.go con F y mem index -skip-graph incremental → A→F = 1, 0 pendientes restantes, user_version = 11 (antes de la rama: 0). Escenario B (actualización): una base creada con v2.49.1 (user_version = 10) y abierta con la rama migra a 11 y crea code_pending_calls (exit 0); A→F = 0 porque v2.49.1 no guardó la pendiente; tras un único mem index -force, A→F = 1, como se acordó en Q1.

## Siguiente
Cerrada con aprobación sobre la revisión 0a94945455a3.
