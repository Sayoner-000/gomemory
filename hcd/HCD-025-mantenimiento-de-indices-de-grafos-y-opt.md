# Mantenimiento de índices de grafos y optimización de relaciones en mem optimize

ID: HCD-025
Tipo: feature
Carril: full
Motivo: Afecta múltiples almacenes de grafos (gomemory, codebase, codegraph), contratos CLI y políticas de depuración de huérfanos con decisiones abiertas interdependientes

## Intención
Incorporar dentro del módulo de optimización, el comando `mem optimize` y la interfaz interactiva TUI (`mem tui`), el mantenimiento integral de índices de grafos y relaciones:
1. Depurar relaciones huérfanas y desasociadas en `memory_relations`, deduplicar aristas entre pares y compactar el índice FTS5 `memory_search`.
2. Saneamiento del grafo de código interno (`code_nodes`, `code_edges`, `code_files`), eliminando nodos y aristas huérfanas de archivos eliminados y optimizando `code_search`.
3. Mantenimiento y verificación de índices de proveedores externos (`codebase-memory-mcp` y `codegraph`), limpiando snapshots o artefactos temporales huérfanos y comprobando su estado de indexado.
4. Optimización a nivel de almacenamiento SQLite (`PRAGMA optimize`, análisis y verificación de integridad).
5. Integración en la TUI: añadir la acción de optimización de grafos e índices en el menú de mantenimiento interactivo con feedback visual de progreso y resultados.

## Alcance
Incluido: nuevo comando CLI `mem optimize` (con opciones como `--dry-run`, `--yes`, `--target`); acción de optimización interactiva en la pantalla de Mantenimiento de la TUI (`adapters/primary/tui`), con flujo de confirmación y reporte de estado; caso de uso y métodos en repositorios para detectar y purgar relaciones huérfanas (`memory_id_a` o `memory_id_b` inexistentes), deduplicar relaciones duplicadas entre mismos pares de memoria, y purgar nodos/aristas huérfanas en el grafo de código interno de SQLite; optimización de índices FTS5 (`memory_search`, `code_search`); comprobación de salud y purga de snapshots/bloqueos temporales huérfanos de `codebase-memory-mcp` y `codegraph`; salida formateada en consola usando `console.NewFlow` con símbolos uniformes de lipgloss; pruebas unitarias, de contrato, de TUI e integración.
Excluido: modificar la gramática o motor del AST indexador existente; disparar reindexados externos bloqueantes de larga duración por defecto sin confirmación o flag explícito; alterar la semántica de compresión de contexto de `mem pack`; modificar los esquemas DDL de forma no aditiva; rediseñar la navegación global de pestañas de la TUI.

## ¿Qué pasa si no lo hacemos?
Con el tiempo y tras sucesivos turnos de trabajo, borrados parciales (`mem forget`, `gc`), refactorizaciones de código y eliminaciones de archivos, se acumulan relaciones huérfanas en `memory_relations`, nodos/aristas huérfanas en `code_nodes`/`code_edges` e inconsistencias en los índices de búsqueda FTS5. Además, los snapshots de grafos externos pueden quedar desincronizados o huérfanos en disco sin que exista un comando unificado o acción en la TUI para verificar y optimizar el estado de los grafos del proyecto.

## Alternativa más barata
Ejecutar manualmente `mem gc`, `mem compact` y `mem index --force`. Esta alternativa no detecta ni elimina relaciones huérfanas entre memorias, no limpia nodos huérfanos en el grafo de código ni optimiza las relaciones existentes, ni diagnostica ni mantiene los grafos de `codebase-memory-mcp` o `codegraph`, ni ofrece la funcionalidad de optimización desde la TUI.

## Historias
### US1. Mantenimiento y saneamiento del grafo y relaciones de gomemory
Como desarrollador o agente que usa gomemory, quiero un comando `mem optimize` que limpie relaciones huérfanas, deduplique conexiones y optimice los índices internos, para mantener la base de datos íntegra, compacta y con consultas eficientes.
- C-01: Dado un proyecto con relaciones en `memory_relations` cuyos extremos apuntan a memorias inexistentes o con aristas duplicadas entre los mismos IDs, cuando se ejecuta `mem optimize`, entonces las relaciones huérfanas se eliminan, los duplicados se consolidan preservando la mayor confianza, y se reporta el conteo de elementos saneados.
- C-02: Dado un grafo de código interno con nodos (`code_nodes`) o aristas (`code_edges`) desvinculados de archivos presentes en `code_files`, cuando se ejecuta `mem optimize`, entonces se purgan los nodos y aristas huérfanas y se reconstruyen/optimizan los índices FTS5 (`code_search` y `memory_search`).

### US2. Mantenimiento y diagnóstico de índices de grafos externos
Como desarrollador o agente que cuenta con proveedores externos de grafo (`codebase-memory-mcp`, `codegraph`), quiero que `mem optimize` inspeccione su estado y limpie snapshots o artefactos temporales obsoletos, para garantizar que la información de contexto de código sea fidedigna y libre de residuos.
- C-03: Dado un proyecto con proveedores externos configurados, cuando se ejecuta `mem optimize`, entonces se verifica la disponibilidad y estado de sincronización de cada proveedor externo (`codebase-memory-mcp` y `codegraph`), eliminando snapshots inválidos o huérfanos en el directorio de memoria.

### US3. Optimización y mantenimiento de grafos e índices desde la TUI
Como usuario interactivo de `mem tui`, quiero disponer de una opción de optimización dentro del menú de mantenimiento, para poder sanear los índices, huérfanos y relaciones directamente sin recurrir a la línea de comandos externa.
- C-04: Dada la pantalla de Mantenimiento de la TUI, cuando el usuario navega a la opción «Optimizar grafos e índices» y confirma la ejecución, entonces se invocan los saneamientos de relaciones, nodos huérfanos e índices, y se reporta el resumen de la operación en la línea de estado de la TUI.

## Aclaraciones
- [x] Q1. ¿Cuál debe ser la interfaz CLI y comportamiento por defecto de `mem optimize` (flags `--dry-run`, `--yes`, `--target=all|internal|external`)? → Comando mem optimize que optimiza todo por defecto con flags --dry-run, --yes y --target=all|internal|external
- [x] Q2. ¿Qué reglas de consolidación deben aplicarse al optimizar relaciones existentes (eliminar bidireccionales redundantes, umbral de confianza mínimo, o solo duplicados exactos y huérfanos)? → Purgar huérfanas, eliminar auto-referencias y consolidar duplicados con la mayor confianza (sin poda destructiva por umbral)
- [x] Q3. ¿Cómo debe proceder `mem optimize` con respecto al reindexado de proveedores externos (solo diagnóstico y limpieza de snapshots, o invocar reindexado completo si se detectan desactualizados)? → Diagnóstico de salud y purga de residuos por defecto; reindexado externo completo solo con flag --reindex-external

## Slices
### VS-01. Mantenimiento y saneamiento del grafo interno y relaciones de gomemory
Tareas: 1, 2, 3
Criterios: C-01, C-02

### VS-02. Mantenimiento, diagnóstico y saneamiento de proveedores externos de grafos
Tareas: 4, 5
Criterios: C-03

### VS-03. Integración de la optimización de grafos e índices en la TUI
Tareas: 6, 7
Criterios: C-04

## Checklist
- [x] 1. Implementar en persistence la purga de relaciones huérfanas, consolidación de duplicados y saneamiento de nodos/aristas huérfanas de código en SQLite → go test ./adapters/secondary/persistence -run TestOptimizeStorage_RelationsAndCodeGraph -count=1
- [x] 2. Crear el caso de uso OptimizeGraph que orquesta el saneamiento de relaciones, grafos e índices FTS5/SQLite con reporte de métricas → go test ./application/usecases -run TestOptimizeGraph_UseCase -count=1
- [x] 3. Añadir el comando CLI mem optimize con flags --dry-run, --yes y --target con salida estructurada en console → go test ./adapters/primary/cli -run TestCmdOptimize -count=1
- [x] 4. Implementar en codegraph y codebasememory la verificación de salud, purga de snapshots obsoletos y soporte de reindexado externo → go test ./adapters/secondary/codegraph/... -run TestCodeProviders_OptimizeMaintenance -count=1
- [x] 5. Integrar el mantenimiento de proveedores externos y flag --reindex-external en el caso de uso y comando CLI mem optimize → go test ./adapters/primary/cli -run TestCmdOptimize_ExternalProviders -count=1
- [x] 6. Añadir la opción de optimización de grafos e índices y su confirmación en el menú de mantenimiento de la TUI → go test ./adapters/primary/tui -run TestTUIMaintenance_OptimizeOption -count=1
- [x] 7. Ejecutar la optimización desde la TUI y reportar el resumen de saneamiento en la línea de estado → go test ./adapters/primary/tui -run TestTUIMaintenance_ExecuteOptimize -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-04 19:53 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/persistence -run TestOptimizeStorage_RelationsAndCodeGraph -count=1` (salida 1)
      # mem/adapters/secondary/persistence [mem/adapters/secondary/persistence.test]
      adapters/secondary/persistence/repositories.go:368:9: cannot use &MaintenanceRepository{…} (value of type *MaintenanceRepository) as ports.MaintenanceRepository value in return statement: *MaintenanceRepository does not implement ports.MaintenanceRepository (missing method OptimizeStorage)
      adapters/secondary/persistence/repositories.go:395:37: cannot use (*MaintenanceRepository)(nil) (value of type *MaintenanceRepository) as ports.MaintenanceRepository value in variable declaration: *MaintenanceRepository does not implement ports.MaintenanceRepository (missing method OptimizeStorage)
      adapters/secondary/persistence/optimize_storage_test.go:17:40: undefined: domain.MemoryLearning
      adapters/secondary/persistence/optimize_storage_test.go:18:40: undefined: domain.MemoryLearning
      FAIL	mem/adapters/secondary/persistence [build failed]
      FAIL
- 2026-11-04 20:00 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/persistence -run TestOptimizeStorage_RelationsAndCodeGraph -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.187s
- 2026-11-04 20:01 -05:00 · tarea 2 · rojo observado · caducada · `go test ./application/usecases -run TestOptimizeGraph_UseCase -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/optimize_graph_test.go:42:17: undefined: usecases.NewOptimizeGraph
      application/usecases/optimize_graph_test.go:43:56: undefined: usecases.OptimizeGraphInput
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-04 20:02 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/persistence -run TestOptimizeStorage_RelationsAndCodeGraph -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.229s
- 2026-11-04 20:03 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run TestOptimizeGraph_UseCase -count=1` (salida 0)
      ok  	mem/application/usecases	0.215s
- 2026-11-04 20:05 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestCmdOptimize -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_optimize_test.go:38:3: undefined: CmdOptimize
      adapters/primary/cli/cmd_optimize_test.go:55:20: cannot use &optimizeTestProjectRepo{…} (value of type *optimizeTestProjectRepo) as ports.ProjectRepository value in struct literal: *optimizeTestProjectRepo does not implement ports.ProjectRepository (missing method DbPath)
      adapters/primary/cli/cmd_optimize_test.go:60:3: undefined: CmdOptimize
      adapters/primary/cli/cmd_optimize_test.go:84:20: cannot use &optimizeTestProjectRepo{…} (value of type *optimizeTestProjectRepo) as ports.ProjectRepository value in struct literal: *optimizeTestProjectRepo does not implement ports.ProjectRepository (missing method DbPath)
      adapters/primary/cli/cmd_optimize_test.go:89:3: undefined: CmdOptimize
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-04 20:09 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdOptimize -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.315s
- 2026-11-04 20:10 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/persistence -run TestOptimizeStorage_RelationsAndCodeGraph -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.246s
- 2026-11-04 20:10 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run TestOptimizeGraph_UseCase -count=1` (salida 0)
      ok  	mem/application/usecases	0.206s
- 2026-11-04 20:12 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/secondary/codegraph/... -run TestCodeProviders_OptimizeMaintenance -count=1` (salida 1)
      --- FAIL: TestCodeProviders_OptimizeMaintenance (0.00s)
          optimize_maintenance_test.go:25: codebasememory.Provider debe implementar ports.CodeGraphMaintainer
      FAIL
      FAIL	mem/adapters/secondary/codegraph	0.551s
      ok  	mem/adapters/secondary/codegraph/codebasememory	0.547s [no tests to run]
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.546s [no tests to run]
      FAIL
- 2026-11-04 20:16 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/codegraph/... -run TestCodeProviders_OptimizeMaintenance -count=1` (salida 0)
      ok  	mem/adapters/secondary/codegraph	0.126s
      ok  	mem/adapters/secondary/codegraph/codebasememory	0.118s [no tests to run]
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.124s [no tests to run]
- 2026-11-04 20:17 -05:00 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestCmdOptimize_ExternalProviders -count=1` (salida 1)
      --- FAIL: TestCmdOptimize_ExternalProviders (0.00s)
          cmd_optimize_test.go:152: se esperaba mención al proveedor externo en salida, got:
              ✓ El almacén y grafos ya estaban optimizados — nada que sanear.
          cmd_optimize_test.go:155: se esperaba reporte de snapshots purgados, got:
              ✓ El almacén y grafos ya estaban optimizados — nada que sanear.
          cmd_optimize_test.go:158: se esperaba reporte de reindexado completo, got:
              ✓ El almacén y grafos ya estaban optimizados — nada que sanear.
      FAIL
      FAIL	mem/adapters/primary/cli	0.518s
      FAIL
- 2026-11-04 20:19 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdOptimize_ExternalProviders -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.272s
- 2026-11-04 20:21 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/tui -run TestTUIMaintenance_OptimizeOption -count=1` (salida 1)
      # mem/adapters/primary/tui
      adapters/primary/tui/tui_optimize_test.go:33:1: expected declaration, found '}'
      FAIL	mem/adapters/primary/tui [setup failed]
      FAIL
- 2026-11-04 20:23 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/tui -run TestTUIMaintenance_OptimizeOption -count=1` (salida 1)
      --- FAIL: TestTUIMaintenance_OptimizeOption (0.00s)
          tui_optimize_test.go:41: se esperaba una opción 'Optimizar' en maintenanceOptions, got: [Purgar Compactar Garbage Collection Consolidar (topic_key + actividad duplicada)]
      FAIL
      FAIL	mem/adapters/primary/tui	0.247s
      FAIL
- 2026-11-04 20:27 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/persistence -run TestOptimizeStorage_RelationsAndCodeGraph -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.219s
- 2026-11-04 20:28 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run TestOptimizeGraph_UseCase -count=1` (salida 0)
      ok  	mem/application/usecases	0.216s
- 2026-11-04 20:29 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdOptimize -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.368s
- 2026-11-04 20:29 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/codegraph/... -run TestCodeProviders_OptimizeMaintenance -count=1` (salida 0)
      ok  	mem/adapters/secondary/codegraph	0.171s
      ok  	mem/adapters/secondary/codegraph/codebasememory	0.169s [no tests to run]
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.171s [no tests to run]
- 2026-11-04 20:30 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdOptimize_ExternalProviders -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.257s
- 2026-11-04 20:31 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/tui -run TestTUIMaintenance_OptimizeOption -count=1` (salida 0)
      ok  	mem/adapters/primary/tui	0.296s
- 2026-11-04 20:32 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/tui -run TestTUIMaintenance_ExecuteOptimize -count=1` (salida 0)
      ok  	mem/adapters/primary/tui	0.220s
- 2026-11-04 22:04 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/secondary/persistence -run TestOptimizeStorage_RelationsAndCodeGraph -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.330s
- 2026-11-04 22:05 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases -run TestOptimizeGraph_UseCase -count=1` (salida 0)
      ok  	mem/application/usecases	0.237s
- 2026-11-04 22:06 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli -run TestCmdOptimize -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.314s
- 2026-11-04 22:06 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/secondary/codegraph/... -run TestCodeProviders_OptimizeMaintenance -count=1` (salida 0)
      ok  	mem/adapters/secondary/codegraph	0.191s
      ok  	mem/adapters/secondary/codegraph/codebasememory	0.158s [no tests to run]
      ok  	mem/adapters/secondary/codegraph/codegraphcli	0.182s [no tests to run]
- 2026-11-04 22:07 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli -run TestCmdOptimize_ExternalProviders -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.346s
- 2026-11-04 22:09 -05:00 · tarea 6 · verde · vigente · `go test ./adapters/primary/tui -run TestTUIMaintenance_OptimizeOption -count=1` (salida 0)
      ok  	mem/adapters/primary/tui	0.726s
- 2026-11-04 22:10 -05:00 · tarea 7 · verde · vigente · `go test ./adapters/primary/tui -run TestTUIMaintenance_ExecuteOptimize -count=1` (salida 0)
      ok  	mem/adapters/primary/tui	0.264s

## Siguiente
Cerrada con aprobación sobre la revisión 87678fb3a53b.
