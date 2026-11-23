# Optimizar duplicados y construcción de contexto con evidencia de rendimiento

ID: HCD-059
Tipo: technical
Carril: fast
Motivo: Costes medidos reproducibles; preservar contratos sin decisiones abiertas ni cambios de comportamiento

## Intención
Reducir la latencia de detección de duplicados y la memoria asignada al construir contexto, priorizando resultados medidos y preservando grupos, títulos, anclas y contabilidad


## Alcance
Incluido: P1, reducir comparaciones de duplicados sin alterar Jaccard, separación por tipo, exclusión de checkpoints, transitividad ni SuggestedKeepID; P2, reducir la materialización de cuerpos al construir contexto sin perder títulos, sinapsis, conflictos ni evidencia fuera de la ventana reciente; benchmarks reproducibles y pruebas diferenciales. Objetivos propuestos en la misma máquina: al menos 5x menos tiempo en duplicados dispersos de 10.000 memorias y al menos 30% menos bytes asignados en contexto de 10.000 memorias. No son SLO de producto.
Excluido: cambios de comportamiento, eliminación de memorias, cambios de umbrales, nuevas dependencias, cachés persistentes, migraciones destructivas y optimización del contador sintético (P3, diferida por bajo impacto observado).

## ¿Qué pasa si no lo hacemos?
La detección sigue comparando todos los pares por tipo: el corpus disperso de 10.000 memorias tarda 21,5 segundos. La construcción de contexto sigue materializando el historial completo incluso con salida presupuestada: 27,8 MB asignados por operación en el escenario medido. No se ha demostrado lentitud de producción ni urgencia en proyectos pequeños.

## Alternativa más barata
Mantener el código y ejecutar duplicados solo bajo demanda. Es razonable para historiales pequeños, pero no resuelve el crecimiento medido. Limitar silenciosamente el historial o aproximar la similitud sería más barato y cambiaría el contrato; se descarta. P3 se difiere: el contador añade unos 32 KiB en el contexto medido, sin regresión temporal distinguible con tres muestras.

## Diagnóstico de rendimiento
Entorno: Go 1.27.4, darwin/arm64, 10 CPU lógicas. Una pasada de calentamiento y tres muestras por caso; mediana de tiempo. runtime.MemStats mide bytes acumulados asignados, no RSS ni pico de memoria. Setup excluido y GC antes de cada muestra. Código de producción llamado desde `/tmp/gomemory-perf-diagnostic/main.go` con `GOMEMORY_NO_CODE_REFRESH=1 go run /tmp/gomemory-perf-diagnostic/main.go`. El harness inicial era temporal. Las tareas 1 y 2 incorporan benchmarks versionados con tests del corpus. El nuevo fixture usa exactamente 1.040 bytes, según el alcance; su línea base debe medirse de nuevo antes de comparar mejoras, pues difiere del diagnóstico temporal.

| Memorias | Duplicados | Contexto sin métricas | ListAll |
|---|---:|---:|---:|
| 100 | 4,571 ms | 0,821 ms | 0,268 ms |
| 1.000 | 213,282 ms | 4,908 ms | 2,434 ms |
| 10.000 | 21.527,665 ms | 47,983 ms | 23,786 ms |

A 10.000: duplicados asigna aproximadamente 53,18 MB/op; contexto 27,78 MB/op; ListAll 26,46 MB/op. Contexto con métricas: 48,791 ms y 27,813 MB/op. Contador sintético aislado de 10 MB: 5,171 ms y 10,002 MB/op; este tamaño NO representa el coste normal de contexto.

Corpus duplicados: un tipo learning, 24 términos disjuntos por memoria más título único, threshold 0,09. Corpus contexto del diagnóstico temporal: SQLite real temporal con 100/1.000/10.000 filas, cuerpos de 1.080 bytes (corregido: la descripción inicial decía erróneamente 1.040), Budget=12000; sin relaciones, proveedores, anclas de archivo ni HCD. Fixture insertado en transacción SQL únicamente en la base temporal; no mide guardado ni mantenimiento FTS. Recorder noop para aislar cálculo, sin persistencia de métricas. No se accede a la base del usuario. Estas muestras no son percentiles ni un perfil de producción; antes del cambio deben añadirse casos densos, multítipo, Unicode, relaciones y anclas.

## Mapa de impacto
- `DetectDuplicateGroups → clusterBySimilarity → jaccardSimilarity` en `application/usecases/detect_duplicates.go`; consumidores `DetectProjectDuplicates`, `BuildContextPack` y TUI. El pack usa candidatos acotados.
- `Builder.Build → build → MemoryLister.List / MemoryFullLister.ListAll`; `application/usecases/build_context.go`, `application/ports/context_builder.go`, `adapters/secondary/persistence/repositories.go` y `adapters/secondary/persistence/memory.go`. Preservar consumidores de todos los títulos, sinapsis y evidencia de anclas.
- Solución P1: índice invertido por token con conjunto de candidatos por memoria (no almacenar todos los pares globalmente), Jaccard exacto y union-find existentes; conservar evaluación exhaustiva para umbrales no positivos y resultados de NaN/infinito. Sin prometer mejora asintótica en corpus denso.
- Solución P2: DTO `domain.ContextMemorySummary` con ID, Type, SessionID, DisplayTitle y Filepath; puerto opcional `ports.ContextMemorySummaryLister.ListContextSummaries(project)` sin ensanchar MemoryRepository. Adaptador ordenado por ID ascendente y acotado por proyecto; consulta explícita de metadatos y contenido únicamente cuando el título almacenado está vacío (o requiere fallback tras saneamiento). DisplayTitle debe ser idéntico a displayTitle de la lectura completa: saneamiento terminal/privacidad vigente antes de recortar por runas; no hacer substr SQL antes de sanear. El caso sin título puede requerir leer el cuerpo completo y se medirá aparte.
- Builder prefiere resúmenes para el historial global; los convierte localmente a los metadatos que usan títulos, masa y anclas, sin exponer cuerpos parciales como memorias completas. Conserva List(100) para contenido reciente y la cadena de fallback actual ListAll/List si no hay puerto o devuelve error. Se preservan ByTopicKey, exportación, mass CLI y ListAll genérico. DTO sin timestamps: consumidores globales inspeccionados usan ID, tipo, sesión, título y ruta; las métricas de descarte se calculan sobre la ventana reciente.
- Codebase-memory: generación 2026-11-23T00:41:06Z; trazas inbound/outbound consultadas y sin incidencias registradas en los cinco archivos anteriores. Cobertura best-effort, no garantía exhaustiva. CodeGraph aporta fuentes actuales; sus relaciones pueden omitir enlaces por versión antigua de extracción.

## Normas e historial
Aplican Constitución §3 (capas y SOLID), §5 (persistencia), §10 (pruebas) y §18 (artefacto único); Playbook HCD-TASK-01 y HCD-EVIDENCE-01. No se modifica producción en este diagnóstico. Historial consultado: sin unidades ligadas a detect_duplicates.go; build_context.go enlaza HCD-022 y su auditoría de presentación. Se preservan además los contratos actuales documentados por los tests de títulos y anclas fuera de las 100 recientes. No se replantean decisiones anteriores.

## Árbol de tareas y dependencias
Objetivo: obtener mejora P1 ≥5x en corpus disperso y mejora P2 ≥30% en B/op a 10.000 memorias frente a referencia exhaustiva/completa, sin cambiar resultados.
- Línea base: T1 (duplicados) ∥ T2 (contexto); T3 depende de T1/T2.
- P1: T4 depende de T1; T5 depende de T4; T6 depende de T5.
- P2: T7 depende de T2 (∥ T4); T8 depende de T7; T9 depende de T8; T10 depende de T9/T7; T11 depende de T10.
- Rendimiento: T12 depende de T3/T6/T11; T13 depende de T12.
- Regresión final: T14 depende de T13.
Todas las hojas del checklist son acciones ejecutables con un resultado verificable; no queda una tarea de diseño abierta. Los nombres de tests y benchmarks son contratos por crear, no pruebas que ya existan.

Las referencias exhaustiva y de lectura completa vivirán solo en tests. Se preservará el orden canónico completo de grupos, SuggestedKeepID y resultados de fallback. La medición comparativa usará los mismos fixtures, máquina, GOMAXPROCS y revisión; cinco muestras por ruta, `-benchtime=1x` para evitar calibraciones de minutos. Registrar todas las muestras y sus medianas en este documento fuera del bloque Evidencia; la evidencia HCD se registra solo mediante el motor. No se han implementado ni verificado optimizaciones.


## Línea base versionada previa a optimizar
HEAD: `3847db40e457502ccb5cbf3470046ad02142b860`, Go 1.27.4 darwin/arm64, Apple M4, GOMAXPROCS predeterminado 10. Código de producción sin cambios; fixture versionado de 1.040 bytes. Comando ejecutado con exit 0 en 225,692 s: `go test ./application/usecases -run '^$' -bench '^BenchmarkHCD059(Duplicates|Context)$' -benchtime=1x -benchmem -count=5`. Esta tarea es registro de una medición, no un cambio funcional: no tiene rojo; se declara evidencia manual del comando observado, no una comprobación de optimización.

Medianas por columna (cinco muestras, no percentiles):

| Caso | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Context/SinRelaciones/N100/Metricsfalse-10 | 1086708 | 671584 | 5626 |
| Context/SinRelaciones/N100/Metricstrue-10 | 926875 | 704352 | 5627 |
| Context/SinRelaciones/N1000/Metricsfalse-10 | 5196875 | 2885920 | 29909 |
| Context/SinRelaciones/N1000/Metricstrue-10 | 4693584 | 2918688 | 29910 |
| Context/SinRelaciones/N10000/Metricsfalse-10 | 47166042 | 27870736 | 272975 |
| Context/SinRelaciones/N10000/Metricstrue-10 | 46567584 | 27903472 | 272976 |
| Context/SinTitulo/N100/Metricsfalse-10 | 870250 | 1083776 | 5402 |
| Context/SinTitulo/N100/Metricstrue-10 | 869875 | 1100576 | 5406 |
| Context/SinTitulo/N1000/Metricsfalse-10 | 5950750 | 7771904 | 30575 |
| Context/SinTitulo/N1000/Metricstrue-10 | 6142875 | 7788272 | 30575 |
| Context/SinTitulo/N10000/Metricsfalse-10 | 57333750 | 77468016 | 282640 |
| Context/SinTitulo/N10000/Metricstrue-10 | 58241167 | 77484512 | 282642 |
| Context/Relaciones/N100/Metricsfalse-10 | 982458 | 845136 | 7430 |
| Context/Relaciones/N100/Metricstrue-10 | 912750 | 877904 | 7431 |
| Context/Relaciones/N1000/Metricsfalse-10 | 7355583 | 4708584 | 48418 |
| Context/Relaciones/N1000/Metricstrue-10 | 6906042 | 4741320 | 48419 |
| Context/Relaciones/N10000/Metricsfalse-10 | 77697750 | 49954344 | 462832 |
| Context/Relaciones/N10000/Metricstrue-10 | 77734250 | 49987208 | 462834 |
| Context/Anclas/N100/Metricsfalse-10 | 950833 | 899904 | 8924 |
| Context/Anclas/N100/Metricstrue-10 | 940625 | 932672 | 8925 |
| Context/Anclas/N1000/Metricsfalse-10 | 6241209 | 4600360 | 49412 |
| Context/Anclas/N1000/Metricstrue-10 | 6021000 | 4633128 | 49413 |
| Context/Anclas/N10000/Metricsfalse-10 | 60697167 | 48692832 | 454522 |
| Context/Anclas/N10000/Metricstrue-10 | 65511333 | 48762896 | 454529 |
| Duplicates/Disperso/N100-10 | 2860916 | 514248 | 2027 |
| Duplicates/Disperso/N1000-10 | 215874875 | 5049960 | 20052 |
| Duplicates/Disperso/N10000-10 | 21145572958 | 53109288 | 200132 |
| Duplicates/Denso/N100-10 | 2186959 | 529768 | 1931 |
| Duplicates/Denso/N1000-10 | 159514083 | 5219352 | 19050 |
| Duplicates/Denso/N10000-10 | 14983336042 | 57981800 | 190074 |
| Duplicates/Multitipo/N100-10 | 1405250 | 529736 | 1952 |
| Duplicates/Multitipo/N1000-10 | 83745375 | 4830472 | 19079 |
| Duplicates/Multitipo/N10000-10 | 7532668000 | 55169568 | 190118 |

<details><summary>Muestras completas antes del cambio</summary>

```text
goos: darwin
goarch: arm64
pkg: mem/application/usecases
cpu: Apple M4
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	   1327667 ns/op	  674336 B/op	    5632 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	   1086708 ns/op	  671632 B/op	    5627 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    984500 ns/op	  671168 B/op	    5623 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	   1109208 ns/op	  671584 B/op	    5626 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	   1030250 ns/op	  671584 B/op	    5626 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	   1066000 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	   1009541 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    922083 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    911167 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    926875 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   5294667 ns/op	 2885920 B/op	   29909 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   5196875 ns/op	 2885504 B/op	   29906 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   5288708 ns/op	 2885920 B/op	   29909 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   4958333 ns/op	 2885504 B/op	   29906 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   4591792 ns/op	 2885920 B/op	   29909 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   4698250 ns/op	 2918688 B/op	   29910 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   4606041 ns/op	 2918272 B/op	   29907 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   4711542 ns/op	 2918688 B/op	   29910 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   4693584 ns/op	 2918688 B/op	   29910 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   4642084 ns/op	 2918272 B/op	   29907 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  48292833 ns/op	27876152 B/op	  272982 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  47166042 ns/op	27870704 B/op	  272975 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  47172125 ns/op	27870736 B/op	  272976 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  46484250 ns/op	27870480 B/op	  272973 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  46916500 ns/op	27870736 B/op	  272975 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  46212667 ns/op	27903376 B/op	  272976 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  46567584 ns/op	27903248 B/op	  272974 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  46872667 ns/op	27903600 B/op	  272978 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  47367666 ns/op	27903584 B/op	  272977 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  46062500 ns/op	27903472 B/op	  272976 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    942708 ns/op	 1084192 B/op	    5405 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    870250 ns/op	 1084192 B/op	    5405 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    895167 ns/op	 1083776 B/op	    5402 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    839125 ns/op	 1083776 B/op	    5402 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    843500 ns/op	 1083776 B/op	    5402 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    891792 ns/op	 1100576 B/op	    5406 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    879833 ns/op	 1100576 B/op	    5406 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    869875 ns/op	 1100576 B/op	    5406 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    840209 ns/op	 1100576 B/op	    5406 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    855334 ns/op	 1100576 B/op	    5406 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   6277833 ns/op	 7771888 B/op	   30574 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   6033500 ns/op	 7772000 B/op	   30575 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   5888334 ns/op	 7771888 B/op	   30574 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   5885334 ns/op	 7772000 B/op	   30575 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   5950750 ns/op	 7771904 B/op	   30575 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   6478542 ns/op	 7788272 B/op	   30575 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   5955583 ns/op	 7788272 B/op	   30575 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   6142875 ns/op	 7788272 B/op	   30575 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   6370458 ns/op	 7788320 B/op	   30576 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   6117458 ns/op	 7788272 B/op	   30575 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  59397083 ns/op	77473336 B/op	  282646 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  56761834 ns/op	77468016 B/op	  282640 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  57333750 ns/op	77467936 B/op	  282639 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  57019041 ns/op	77467920 B/op	  282639 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  58661666 ns/op	77468160 B/op	  282641 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  58241167 ns/op	77484288 B/op	  282640 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  57956584 ns/op	77484528 B/op	  282642 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  58341666 ns/op	77484512 B/op	  282642 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  58106542 ns/op	77484544 B/op	  282643 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  58377167 ns/op	77484512 B/op	  282643 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	   1206541 ns/op	  845136 B/op	    7430 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	    965209 ns/op	  845136 B/op	    7430 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	   1019083 ns/op	  845136 B/op	    7430 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	    982458 ns/op	  844512 B/op	    7426 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	    948125 ns/op	  845136 B/op	    7430 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    978542 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    912750 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    913500 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    908416 ns/op	  877280 B/op	    7427 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    861250 ns/op	  877984 B/op	    7432 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   7461833 ns/op	 4708664 B/op	   48419 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   7370292 ns/op	 4708584 B/op	   48418 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   7169750 ns/op	 4708552 B/op	   48418 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   7231375 ns/op	 4708696 B/op	   48419 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   7355583 ns/op	 4708552 B/op	   48418 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   7262375 ns/op	 4741336 B/op	   48419 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   7111084 ns/op	 4741320 B/op	   48419 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   6838458 ns/op	 4741320 B/op	   48419 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   6880083 ns/op	 4741336 B/op	   48419 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   6906042 ns/op	 4741320 B/op	   48419 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  77237916 ns/op	49954200 B/op	  462831 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  77697750 ns/op	49954424 B/op	  462833 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  77081250 ns/op	49954344 B/op	  462832 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  78185291 ns/op	49954440 B/op	  462833 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  77865375 ns/op	49954344 B/op	  462832 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  76407458 ns/op	49987112 B/op	  462833 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  77282209 ns/op	49987224 B/op	  462834 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  77734250 ns/op	49987208 B/op	  462834 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  79908209 ns/op	49987192 B/op	  462834 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  79003958 ns/op	49987224 B/op	  462834 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	   1030833 ns/op	  862416 B/op	    8916 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    983750 ns/op	  899904 B/op	    8924 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    934584 ns/op	  899904 B/op	    8924 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    950833 ns/op	  862416 B/op	    8916 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    908250 ns/op	  899904 B/op	    8924 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    974000 ns/op	  932672 B/op	    8925 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    964125 ns/op	  895312 B/op	    8919 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    940625 ns/op	  932672 B/op	    8925 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    883333 ns/op	  932672 B/op	    8925 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    891625 ns/op	  932672 B/op	    8925 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   6430333 ns/op	 4563880 B/op	   49412 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   6241209 ns/op	 4600792 B/op	   49418 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   6497167 ns/op	 4600776 B/op	   49415 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   5886167 ns/op	 4600360 B/op	   49412 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   6133875 ns/op	 4600360 B/op	   49412 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   6021000 ns/op	 4633656 B/op	   49420 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   5910958 ns/op	 4634000 B/op	   49421 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   6019125 ns/op	 4633128 B/op	   49413 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   6404833 ns/op	 4633128 B/op	   49413 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   6299125 ns/op	 4633128 B/op	   49413 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  61962959 ns/op	48692832 B/op	  454521 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  61367209 ns/op	48619456 B/op	  454521 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  60697167 ns/op	48656192 B/op	  454522 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  60586125 ns/op	48841536 B/op	  454547 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  60617500 ns/op	48693376 B/op	  454528 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  63204792 ns/op	48725824 B/op	  454524 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  64507417 ns/op	48763216 B/op	  454534 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  66213667 ns/op	48689184 B/op	  454525 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  65511333 ns/op	48763536 B/op	  454537 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  66278625 ns/op	48762896 B/op	  454529 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                        	       1	   2977541 ns/op	  514248 B/op	    2027 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                        	       1	   2803375 ns/op	  514248 B/op	    2027 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                        	       1	   2836416 ns/op	  514248 B/op	    2027 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                        	       1	   2860916 ns/op	  514248 B/op	    2027 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                        	       1	   2903375 ns/op	  514248 B/op	    2027 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                       	       1	 222344750 ns/op	 5049960 B/op	   20052 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                       	       1	 215874875 ns/op	 5161352 B/op	   20069 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                       	       1	 214306833 ns/op	 5049848 B/op	   20051 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                       	       1	 217290500 ns/op	 4975816 B/op	   20042 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                       	       1	 212144916 ns/op	 5086920 B/op	   20056 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                      	       1	21145572958 ns/op	53076296 B/op	  200132 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                      	       1	21221258750 ns/op	53109176 B/op	  200130 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                      	       1	21128391208 ns/op	53146424 B/op	  200137 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                      	       1	21755211916 ns/op	53183608 B/op	  200143 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                      	       1	21137749750 ns/op	53109288 B/op	  200131 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                           	       1	   2199833 ns/op	  529768 B/op	    1931 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                           	       1	   2236333 ns/op	  529768 B/op	    1931 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                           	       1	   2165334 ns/op	  529768 B/op	    1931 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                           	       1	   2186959 ns/op	  492696 B/op	    1926 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                           	       1	   2178625 ns/op	  529768 B/op	    1931 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                          	       1	 158735417 ns/op	 5219176 B/op	   19048 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                          	       1	 159691625 ns/op	 5219352 B/op	   19050 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                          	       1	 159760708 ns/op	 5256584 B/op	   19056 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                          	       1	 159514083 ns/op	 5219176 B/op	   19048 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                          	       1	 159370666 ns/op	 5256248 B/op	   19053 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                         	       1	15068149041 ns/op	58095672 B/op	  190091 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                         	       1	14983336042 ns/op	57981800 B/op	  190074 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                         	       1	14959281209 ns/op	57981800 B/op	  190074 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                         	       1	14984293500 ns/op	57907480 B/op	  190062 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                         	       1	14978121250 ns/op	57870408 B/op	  190057 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                       	       1	   1409625 ns/op	  529736 B/op	    1952 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                       	       1	   1405250 ns/op	  529736 B/op	    1952 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                       	       1	   1394917 ns/op	  529736 B/op	    1952 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                       	       1	   1393791 ns/op	  529736 B/op	    1952 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                       	       1	   1417417 ns/op	  529736 B/op	    1952 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                      	       1	  83745375 ns/op	 4793400 B/op	   19074 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                      	       1	  84685500 ns/op	 4830360 B/op	   19078 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                      	       1	  85487500 ns/op	 4867432 B/op	   19083 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                      	       1	  82097209 ns/op	 4830472 B/op	   19079 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                      	       1	  82677542 ns/op	 4904504 B/op	   19088 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                     	       1	7502967417 ns/op	55171272 B/op	  190123 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                     	       1	7532668000 ns/op	55243824 B/op	  190129 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                     	       1	7555663208 ns/op	55095424 B/op	  190108 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                     	       1	7478778291 ns/op	55169568 B/op	  190118 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                     	       1	7709469708 ns/op	55132544 B/op	  190113 allocs/op
PASS
ok  	mem/application/usecases	225.692s
```

</details>

## Avance P1: tareas 4–6
Referencia exhaustiva BFS en tests (tokenización y Jaccard compartidos intencionadamente; enumeración y agrupación independientes), fixtures de transitividad, Unicode, checkpoints, vacíos y 50 corpus pseudoaleatorios de semilla 59, diez umbrales incluidos NaN e infinitos. Se comparan grupos completos e ID conservado. Test adicional verifica poda dispersa, todos los pares para umbral cero y ausencia de pares repetidos en denso. BuildContextPack conserva el ID de la memoria más completa, no mezcla tipos y excluye checkpoints con estadísticas verificadas.

Producción: índice invertido con marcas O(n) por memoria; para filas densas se usa enumeración exhaustiva cuando el recorrido de postings deja de ahorrar trabajo. No se cambia Jaccard ni el umbral. Las asignaciones del índice son un coste explícito, no una mejora de memoria.

Medición provisional posterior (Go1.27.4, Apple M4, tres muestras `-benchtime=1x -benchmem`; no sustituye T12/T13):

| Caso 10k | ns/op (tres muestras) | B/op (tres muestras) |
|---|---|---|
| Disperso | 121069875; 117407000; 116138417 | 105434192; 105497632; 105423840 |
| Denso | 15392403166; 15507621250; 15266973625 | 68482672; 68295912; 68220528 |

Mediana dispersa: 117,407 ms, aproximadamente 180x frente a la línea base 21.145,573 ms, pero asignaciones aproximadamente duplicadas (105,43 MB frente a 53,11 MB). Mediana densa: 15,392 s frente a 14,983 s (~2,7% más en series de distinto tamaño, no conclusión estadística); asignaciones 68,30 MB frente a 57,98 MB. Esta compensación se debe revisar en aceptación final. Tests focalizados con `-race` verdes. Una primera invocación de rojo T5 falló por pérdida de comillas del regex en CLI, no por comportamiento; se repitió correctamente y se observó el rojo de compilación por helper ausente antes de implementarlo.

## Avance P2: tareas 7–11
DTO ContextMemorySummary y puerto opcional separados; MemoryRepository no se amplía. El adaptador proyecta metadatos de todo el proyecto en orden ID; carga cuerpo para títulos vacíos. Si un título no vacío queda vacío tras quitar controles terminales, recupera el cuerpo después de cerrar rows (compatible con pool de una conexión). Se sanea antes del fallback por runas, preservando DisplayTitle y filepath igual que ListAll. La mayoría de títulos evita materializar cuerpos; títulos vacíos conservan un coste mayor por compatibilidad. No hay caché, migración ni eliminación de datos.

Builder prefiere resúmenes, conserva List(100) para las secciones recientes y degrada a ListAll/List ante ausencia o error del puerto. Test diferencial con 130 memorias, conflictos antiguos, títulos vacíos/Unicode/ANSI, sinapsis, anclas y proveedor hotspot, modos índice/presupuestado/ilimitado; comprueba salida byte a byte y contabilidad raw/final y exige que no se llame ListAll en camino optimizado. Test de persistencia verifica aislamiento, orden, metadatos y títulos eliminados por saneamiento. Tests de fallback cubren error de resumen, error de ListAll, ausencia de puertos y propagación del error de List.

Medición provisional (tres muestras, no sustituye T12/T13), contexto SinRelaciones/N10000:
- Metricsfalse ns/op: 36144833; 34931875; 34460208. B/op: 11114552; 11119328; 11113960.
- Metricstrue ns/op: 33486208; 34115250; 34463208. B/op: 11146600; 11146728; 11146728.
Mediana sin métricas: 34,932 ms / 11,115 MB frente a línea base 47,172 ms / 27,871 MB (~60% menos asignaciones). Tests HCD059 focalizados y -race en usecases/persistence verdes; falta regresión integral. Las tareas 7–11 han tenido rojo observado y verde por motor; la evidencia anterior debe revalidarse al finalizar cambios.

## Comparación final T12
Misma máquina Apple M4, Go1.27.4, GOMAXPROCS10, mismo fixture y proceso, cinco muestras por ruta. Referencia de duplicados congela el algoritmo union-find anterior (no el BFS, que tiene coste distinto en denso); referencia contexto oculta solo el puerto opcional. Exit0,356.124s. Comando: `go test ./application/usecases -run '^$' -bench '^BenchmarkHCD059(Duplicates|Context)(Reference)?$' -benchtime=1x -benchmem -count=5 -timeout=9m`. Registro de medición sin rojo funcional.

| Caso | mediana ns/op | mediana B/op | mediana allocs/op |
|---|---:|---:|---:|
| Context/SinRelaciones/N100/Metricsfalse-10 | 816958 | 534872 | 4818 |
| Context/SinRelaciones/N100/Metricstrue-10 | 776041 | 567640 | 4819 |
| Context/SinRelaciones/N1000/Metricsfalse-10 | 3431375 | 1370168 | 21900 |
| Context/SinRelaciones/N1000/Metricstrue-10 | 3558417 | 1402936 | 21901 |
| Context/SinRelaciones/N10000/Metricsfalse-10 | 35392833 | 11113976 | 192965 |
| Context/SinRelaciones/N10000/Metricstrue-10 | 35181625 | 11146632 | 192965 |
| Context/SinTitulo/N100/Metricsfalse-10 | 841250 | 1064280 | 4797 |
| Context/SinTitulo/N100/Metricstrue-10 | 837334 | 1080664 | 4798 |
| Context/SinTitulo/N1000/Metricsfalse-10 | 6112583 | 7424184 | 24566 |
| Context/SinTitulo/N1000/Metricstrue-10 | 5895834 | 7440520 | 24566 |
| Context/SinTitulo/N10000/Metricsfalse-10 | 59224625 | 72391560 | 222633 |
| Context/SinTitulo/N10000/Metricstrue-10 | 58703959 | 72408072 | 222635 |
| Context/Relaciones/N100/Metricsfalse-10 | 877250 | 708424 | 6622 |
| Context/Relaciones/N100/Metricstrue-10 | 833458 | 741192 | 6623 |
| Context/Relaciones/N1000/Metricsfalse-10 | 6161667 | 3192816 | 40409 |
| Context/Relaciones/N1000/Metricstrue-10 | 6042250 | 3225584 | 40410 |
| Context/Relaciones/N10000/Metricsfalse-10 | 70227834 | 33197712 | 382823 |
| Context/Relaciones/N10000/Metricstrue-10 | 69081833 | 33230480 | 382824 |
| Context/Anclas/N100/Metricsfalse-10 | 840458 | 763192 | 8116 |
| Context/Anclas/N100/Metricstrue-10 | 772125 | 758536 | 8110 |
| Context/Anclas/N1000/Metricsfalse-10 | 5321333 | 3086608 | 41411 |
| Context/Anclas/N1000/Metricstrue-10 | 5548875 | 3119376 | 41412 |
| Context/Anclas/N10000/Metricsfalse-10 | 51573084 | 31973824 | 374524 |
| Context/Anclas/N10000/Metricstrue-10 | 52569709 | 31968184 | 374519 |
| ContextReference/SinRelaciones/N100/Metricsfalse-10 | 850000 | 671584 | 5626 |
| ContextReference/SinRelaciones/N100/Metricstrue-10 | 819125 | 704352 | 5627 |
| ContextReference/SinRelaciones/N1000/Metricsfalse-10 | 5564542 | 2885920 | 29909 |
| ContextReference/SinRelaciones/N1000/Metricstrue-10 | 5317334 | 2918688 | 29910 |
| ContextReference/SinRelaciones/N10000/Metricsfalse-10 | 53770083 | 27870736 | 272975 |
| ContextReference/SinRelaciones/N10000/Metricstrue-10 | 54327750 | 27903472 | 272976 |
| ContextReference/SinTitulo/N100/Metricsfalse-10 | 984084 | 1084192 | 5405 |
| ContextReference/SinTitulo/N100/Metricstrue-10 | 970541 | 1100160 | 5403 |
| ContextReference/SinTitulo/N1000/Metricsfalse-10 | 6482500 | 7772000 | 30575 |
| ContextReference/SinTitulo/N1000/Metricstrue-10 | 6646875 | 7788272 | 30575 |
| ContextReference/SinTitulo/N10000/Metricsfalse-10 | 69215417 | 77468032 | 282640 |
| ContextReference/SinTitulo/N10000/Metricstrue-10 | 64967416 | 77484304 | 282640 |
| ContextReference/Relaciones/N100/Metricsfalse-10 | 1025000 | 844512 | 7426 |
| ContextReference/Relaciones/N100/Metricstrue-10 | 1039666 | 877904 | 7431 |
| ContextReference/Relaciones/N1000/Metricsfalse-10 | 8468541 | 4708568 | 48418 |
| ContextReference/Relaciones/N1000/Metricstrue-10 | 8089000 | 4741336 | 48419 |
| ContextReference/Relaciones/N10000/Metricsfalse-10 | 87456250 | 49954440 | 462833 |
| ContextReference/Relaciones/N10000/Metricstrue-10 | 88067500 | 49987208 | 462834 |
| ContextReference/Anclas/N100/Metricsfalse-10 | 1048917 | 900064 | 8924 |
| ContextReference/Anclas/N100/Metricstrue-10 | 965833 | 932832 | 8925 |
| ContextReference/Anclas/N1000/Metricsfalse-10 | 7284667 | 4600824 | 49415 |
| ContextReference/Anclas/N1000/Metricstrue-10 | 6821875 | 4633704 | 49416 |
| ContextReference/Anclas/N10000/Metricsfalse-10 | 70242542 | 48730560 | 454533 |
| ContextReference/Anclas/N10000/Metricstrue-10 | 69438125 | 48800464 | 454538 |
| Duplicates/Disperso/N100-10 | 843625 | 885424 | 4552 |
| Duplicates/Disperso/N1000-10 | 9400333 | 8437344 | 45203 |
| Duplicates/Disperso/N10000-10 | 118427958 | 105521664 | 452194 |
| Duplicates/Denso/N100-10 | 2253083 | 601648 | 2235 |
| Duplicates/Denso/N1000-10 | 162134625 | 5993872 | 20352 |
| Duplicates/Denso/N10000-10 | 15787285541 | 68221768 | 200600 |
| Duplicates/Multitipo/N100-10 | 1462792 | 600760 | 2408 |
| Duplicates/Multitipo/N1000-10 | 85301083 | 5386888 | 20587 |
| Duplicates/Multitipo/N10000-10 | 7711871958 | 63123312 | 200990 |
| DuplicatesReference/Disperso/N100-10 | 2885291 | 511560 | 2026 |
| DuplicatesReference/Disperso/N1000-10 | 220883875 | 5062344 | 20055 |
| DuplicatesReference/Disperso/N10000-10 | 21458985292 | 52974632 | 200144 |
| DuplicatesReference/Denso/N100-10 | 2264667 | 529744 | 1930 |
| DuplicatesReference/Denso/N1000-10 | 160134542 | 5219152 | 19047 |
| DuplicatesReference/Denso/N10000-10 | 15274224917 | 57981600 | 190071 |
| DuplicatesReference/Multitipo/N100-10 | 1404292 | 529688 | 1950 |
| DuplicatesReference/Multitipo/N1000-10 | 83935500 | 4830312 | 19076 |
| DuplicatesReference/Multitipo/N10000-10 | 7691143958 | 55169344 | 190114 |

<details><summary>330 muestras comparación final</summary>

```text
goos: darwin
goarch: arm64
pkg: mem/application/usecases
cpu: Apple M4
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	   1265625 ns/op	  537576 B/op	    4823 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    778458 ns/op	  534872 B/op	    4818 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    835166 ns/op	  534872 B/op	    4818 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    750666 ns/op	  534872 B/op	    4818 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    816958 ns/op	  534872 B/op	    4818 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    776041 ns/op	  567640 B/op	    4819 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	   1821041 ns/op	  567640 B/op	    4819 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    827792 ns/op	  567688 B/op	    4820 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    694666 ns/op	  567640 B/op	    4819 allocs/op
BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    709875 ns/op	  567640 B/op	    4819 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   4031625 ns/op	 1370216 B/op	   21901 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   3534250 ns/op	 1370232 B/op	   21901 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   3431375 ns/op	 1370168 B/op	   21900 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   3349916 ns/op	 1370168 B/op	   21900 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricsfalse-10        	       1	   3391500 ns/op	 1370168 B/op	   21900 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   3532291 ns/op	 1402936 B/op	   21901 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   3558417 ns/op	 1402936 B/op	   21901 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   3588000 ns/op	 1402936 B/op	   21901 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   3572917 ns/op	 1402936 B/op	   21901 allocs/op
BenchmarkHCD059Context/SinRelaciones/N1000/Metricstrue-10         	       1	   3457042 ns/op	 1402520 B/op	   21898 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  38612958 ns/op	11114088 B/op	  192966 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  34518333 ns/op	11119520 B/op	  192973 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  35392833 ns/op	11113864 B/op	  192965 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  35617542 ns/op	11113976 B/op	  192965 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricsfalse-10       	       1	  34431208 ns/op	11113976 B/op	  192965 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  34954541 ns/op	11146840 B/op	  192967 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  35181625 ns/op	11146632 B/op	  192965 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  35053292 ns/op	11146616 B/op	  192965 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  35478042 ns/op	11146616 B/op	  192965 allocs/op
BenchmarkHCD059Context/SinRelaciones/N10000/Metricstrue-10        	       1	  35872333 ns/op	11146712 B/op	  192966 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    892459 ns/op	 1064280 B/op	    4797 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    841250 ns/op	 1064280 B/op	    4797 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    793167 ns/op	 1064296 B/op	    4798 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    873833 ns/op	 1064280 B/op	    4797 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricsfalse-10             	       1	    791583 ns/op	 1064280 B/op	    4797 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    814583 ns/op	 1080664 B/op	    4798 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    837334 ns/op	 1080664 B/op	    4798 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    872625 ns/op	 1080248 B/op	    4795 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    858292 ns/op	 1080664 B/op	    4798 allocs/op
BenchmarkHCD059Context/SinTitulo/N100/Metricstrue-10              	       1	    784959 ns/op	 1080248 B/op	    4795 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   6968542 ns/op	 7424184 B/op	   24566 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   6112583 ns/op	 7424136 B/op	   24565 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   6199667 ns/op	 7424136 B/op	   24565 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   6016875 ns/op	 7424248 B/op	   24566 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricsfalse-10            	       1	   5889334 ns/op	 7424248 B/op	   24566 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   5814584 ns/op	 7440520 B/op	   24566 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   5895834 ns/op	 7440648 B/op	   24567 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   5774208 ns/op	 7440520 B/op	   24566 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   6033791 ns/op	 7440520 B/op	   24566 allocs/op
BenchmarkHCD059Context/SinTitulo/N1000/Metricstrue-10             	       1	   6024625 ns/op	 7440744 B/op	   24568 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  60814667 ns/op	72391720 B/op	  222635 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  59224625 ns/op	72396560 B/op	  222637 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  58658375 ns/op	72391560 B/op	  222633 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  59804416 ns/op	72391368 B/op	  222631 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricsfalse-10           	       1	  58480916 ns/op	72391352 B/op	  222632 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  59713292 ns/op	72407944 B/op	  222634 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  59171542 ns/op	72408200 B/op	  222636 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  58703959 ns/op	72407608 B/op	  222631 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  58412333 ns/op	72408520 B/op	  222639 allocs/op
BenchmarkHCD059Context/SinTitulo/N10000/Metricstrue-10            	       1	  58341375 ns/op	72408072 B/op	  222635 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	   1052750 ns/op	  708424 B/op	    6622 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	    936667 ns/op	  708424 B/op	    6622 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	    850333 ns/op	  708424 B/op	    6622 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	    877250 ns/op	  708424 B/op	    6622 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricsfalse-10            	       1	    813959 ns/op	  708424 B/op	    6622 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    830625 ns/op	  740568 B/op	    6619 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    833458 ns/op	  741192 B/op	    6623 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    865625 ns/op	  741192 B/op	    6623 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    814667 ns/op	  741192 B/op	    6623 allocs/op
BenchmarkHCD059Context/Relaciones/N100/Metricstrue-10             	       1	    841500 ns/op	  741192 B/op	    6623 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   6941459 ns/op	 3192896 B/op	   40410 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   6402917 ns/op	 3192816 B/op	   40409 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   6107375 ns/op	 3192816 B/op	   40409 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   6161667 ns/op	 3192816 B/op	   40409 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricsfalse-10           	       1	   5853084 ns/op	 3192816 B/op	   40409 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   6066708 ns/op	 3225696 B/op	   40411 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   5987583 ns/op	 3225584 B/op	   40410 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   6191458 ns/op	 3225584 B/op	   40410 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   5975583 ns/op	 3225584 B/op	   40410 allocs/op
BenchmarkHCD059Context/Relaciones/N1000/Metricstrue-10            	       1	   6042250 ns/op	 3225696 B/op	   40411 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  72901167 ns/op	33197840 B/op	  382824 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  70227834 ns/op	33197568 B/op	  382822 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  66716625 ns/op	33197696 B/op	  382823 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  70616208 ns/op	33197936 B/op	  382826 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricsfalse-10          	       1	  66390042 ns/op	33197712 B/op	  382823 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  69225375 ns/op	33230480 B/op	  382824 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  67418750 ns/op	33230816 B/op	  382827 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  66655916 ns/op	33230448 B/op	  382824 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  69234542 ns/op	33230608 B/op	  382825 allocs/op
BenchmarkHCD059Context/Relaciones/N10000/Metricstrue-10           	       1	  69081833 ns/op	33230336 B/op	  382823 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    923042 ns/op	  763192 B/op	    8116 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    840458 ns/op	  763192 B/op	    8116 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    874125 ns/op	  763192 B/op	    8116 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    803667 ns/op	  763240 B/op	    8117 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricsfalse-10                	       1	    814625 ns/op	  725704 B/op	    8108 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    772125 ns/op	  758472 B/op	    8109 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    760333 ns/op	  795960 B/op	    8117 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    807625 ns/op	  795960 B/op	    8117 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    763167 ns/op	  758536 B/op	    8110 allocs/op
BenchmarkHCD059Context/Anclas/N100/Metricstrue-10                 	       1	    789000 ns/op	  758472 B/op	    8109 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   5287083 ns/op	 3086848 B/op	   41414 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   5296667 ns/op	 3086608 B/op	   41411 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   5426416 ns/op	 3086608 B/op	   41411 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   5321333 ns/op	 3086608 B/op	   41411 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricsfalse-10               	       1	   5527167 ns/op	 3086192 B/op	   41405 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   5232708 ns/op	 3119376 B/op	   41412 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   5814125 ns/op	 3118960 B/op	   41406 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   5565833 ns/op	 3119376 B/op	   41412 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   5548875 ns/op	 3119376 B/op	   41412 allocs/op
BenchmarkHCD059Context/Anclas/N1000/Metricstrue-10                	       1	   5200416 ns/op	 3118544 B/op	   41403 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  61036208 ns/op	31935528 B/op	  374519 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  51219625 ns/op	32047840 B/op	  374533 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  52371792 ns/op	32120616 B/op	  374541 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  51573084 ns/op	31935208 B/op	  374516 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricsfalse-10              	       1	  51408125 ns/op	31973824 B/op	  374524 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  53199375 ns/op	32042136 B/op	  374527 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  51325500 ns/op	31931208 B/op	  374515 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  54373667 ns/op	31968184 B/op	  374519 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  52569709 ns/op	32043744 B/op	  374531 allocs/op
BenchmarkHCD059Context/Anclas/N10000/Metricstrue-10               	       1	  50768833 ns/op	31893832 B/op	  374507 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricsfalse-10         	       1	    967666 ns/op	  671408 B/op	    5625 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricsfalse-10         	       1	    786458 ns/op	  671584 B/op	    5626 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricsfalse-10         	       1	    822833 ns/op	  671584 B/op	    5626 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricsfalse-10         	       1	    850000 ns/op	  671584 B/op	    5626 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricsfalse-10         	       1	    882083 ns/op	  671584 B/op	    5626 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricstrue-10          	       1	    789417 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricstrue-10          	       1	    809750 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricstrue-10          	       1	    858000 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricstrue-10          	       1	    819125 ns/op	  703936 B/op	    5624 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricstrue-10          	       1	    869833 ns/op	  704352 B/op	    5627 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricsfalse-10        	       1	   6055208 ns/op	 2885792 B/op	   29909 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricsfalse-10        	       1	   5891292 ns/op	 2885920 B/op	   29909 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricsfalse-10        	       1	   5564542 ns/op	 2885920 B/op	   29909 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricsfalse-10        	       1	   5479584 ns/op	 2885920 B/op	   29909 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricsfalse-10        	       1	   5307125 ns/op	 2885920 B/op	   29909 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricstrue-10         	       1	   5519458 ns/op	 2918688 B/op	   29910 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricstrue-10         	       1	   5393791 ns/op	 2918272 B/op	   29907 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricstrue-10         	       1	   5317334 ns/op	 2918688 B/op	   29910 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricstrue-10         	       1	   5281417 ns/op	 2918688 B/op	   29910 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N1000/Metricstrue-10         	       1	   5248708 ns/op	 2918688 B/op	   29910 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricsfalse-10       	       1	  53941000 ns/op	27870496 B/op	  272973 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricsfalse-10       	       1	  54093375 ns/op	27870816 B/op	  272976 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricsfalse-10       	       1	  53112334 ns/op	27870736 B/op	  272975 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricsfalse-10       	       1	  53770083 ns/op	27870848 B/op	  272976 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricsfalse-10       	       1	  53518750 ns/op	27870720 B/op	  272975 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricstrue-10        	       1	  54327750 ns/op	27903472 B/op	  272976 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricstrue-10        	       1	  53711375 ns/op	27903376 B/op	  272975 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricstrue-10        	       1	  55018250 ns/op	27903376 B/op	  272975 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricstrue-10        	       1	  55709208 ns/op	27903472 B/op	  272976 allocs/op
BenchmarkHCD059ContextReference/SinRelaciones/N10000/Metricstrue-10        	       1	  53672417 ns/op	27903584 B/op	  272977 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricsfalse-10             	       1	   1004541 ns/op	 1084192 B/op	    5405 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricsfalse-10             	       1	    984084 ns/op	 1084192 B/op	    5405 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricsfalse-10             	       1	    951292 ns/op	 1084192 B/op	    5405 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricsfalse-10             	       1	    968084 ns/op	 1083776 B/op	    5402 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricsfalse-10             	       1	   1008500 ns/op	 1083776 B/op	    5402 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricstrue-10              	       1	    991583 ns/op	 1100576 B/op	    5406 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricstrue-10              	       1	    970541 ns/op	 1100160 B/op	    5403 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricstrue-10              	       1	    981167 ns/op	 1100160 B/op	    5403 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricstrue-10              	       1	    937042 ns/op	 1100576 B/op	    5406 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N100/Metricstrue-10              	       1	    920125 ns/op	 1100160 B/op	    5403 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricsfalse-10            	       1	   6701375 ns/op	 7771888 B/op	   30574 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricsfalse-10            	       1	   6482500 ns/op	 7772000 B/op	   30575 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricsfalse-10            	       1	   6560417 ns/op	 7771888 B/op	   30574 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricsfalse-10            	       1	   6422583 ns/op	 7772000 B/op	   30575 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricsfalse-10            	       1	   6442000 ns/op	 7772000 B/op	   30575 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricstrue-10             	       1	   6487500 ns/op	 7788272 B/op	   30575 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricstrue-10             	       1	   6646875 ns/op	 7788272 B/op	   30575 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricstrue-10             	       1	   6403708 ns/op	 7788496 B/op	   30577 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricstrue-10             	       1	   7112042 ns/op	 7788272 B/op	   30575 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N1000/Metricstrue-10             	       1	   6772375 ns/op	 7788384 B/op	   30576 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricsfalse-10           	       1	  69215417 ns/op	77468016 B/op	  282640 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricsfalse-10           	       1	  65012292 ns/op	77468384 B/op	  282643 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricsfalse-10           	       1	  70106208 ns/op	77467808 B/op	  282638 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricsfalse-10           	       1	  65087125 ns/op	77468032 B/op	  282640 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricsfalse-10           	       1	  70861708 ns/op	77468240 B/op	  282642 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricstrue-10            	       1	  64947625 ns/op	77484528 B/op	  282642 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricstrue-10            	       1	  64477917 ns/op	77484624 B/op	  282643 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricstrue-10            	       1	  65757250 ns/op	77484304 B/op	  282640 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricstrue-10            	       1	  65775125 ns/op	77484176 B/op	  282639 allocs/op
BenchmarkHCD059ContextReference/SinTitulo/N10000/Metricstrue-10            	       1	  64967416 ns/op	77484160 B/op	  282639 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricsfalse-10            	       1	   1116500 ns/op	  844512 B/op	    7426 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricsfalse-10            	       1	   1015917 ns/op	  845136 B/op	    7430 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricsfalse-10            	       1	   1040000 ns/op	  845136 B/op	    7430 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricsfalse-10            	       1	   1009500 ns/op	  844512 B/op	    7426 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricsfalse-10            	       1	   1025000 ns/op	  844512 B/op	    7426 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricstrue-10             	       1	   1012083 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricstrue-10             	       1	   1143291 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricstrue-10             	       1	   1067333 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricstrue-10             	       1	    950500 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N100/Metricstrue-10             	       1	   1039666 ns/op	  877904 B/op	    7431 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricsfalse-10           	       1	   8690875 ns/op	 4708568 B/op	   48418 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricsfalse-10           	       1	   8163500 ns/op	 4708552 B/op	   48418 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricsfalse-10           	       1	   8684667 ns/op	 4708568 B/op	   48418 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricsfalse-10           	       1	   8226541 ns/op	 4708552 B/op	   48418 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricsfalse-10           	       1	   8468541 ns/op	 4708664 B/op	   48419 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricstrue-10            	       1	   8204375 ns/op	 4741336 B/op	   48419 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricstrue-10            	       1	   8089000 ns/op	 4741352 B/op	   48419 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricstrue-10            	       1	   8212084 ns/op	 4741336 B/op	   48419 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricstrue-10            	       1	   8037583 ns/op	 4741320 B/op	   48419 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N1000/Metricstrue-10            	       1	   7876875 ns/op	 4741320 B/op	   48419 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricsfalse-10          	       1	  91684542 ns/op	49954232 B/op	  462831 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricsfalse-10          	       1	  87456250 ns/op	49954552 B/op	  462834 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricsfalse-10          	       1	  87037875 ns/op	49954552 B/op	  462834 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricsfalse-10          	       1	  86957791 ns/op	49954440 B/op	  462833 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricsfalse-10          	       1	  87537750 ns/op	49954328 B/op	  462832 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricstrue-10           	       1	  91199833 ns/op	49987208 B/op	  462834 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricstrue-10           	       1	  88399042 ns/op	49987208 B/op	  462834 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricstrue-10           	       1	  87710250 ns/op	49987320 B/op	  462835 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricstrue-10           	       1	  88067500 ns/op	49987304 B/op	  462835 allocs/op
BenchmarkHCD059ContextReference/Relaciones/N10000/Metricstrue-10           	       1	  86780000 ns/op	49987096 B/op	  462833 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricsfalse-10                	       1	   1103708 ns/op	  900064 B/op	    8924 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricsfalse-10                	       1	   1072542 ns/op	  900064 B/op	    8924 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricsfalse-10                	       1	   1001000 ns/op	  900064 B/op	    8924 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricsfalse-10                	       1	   1048917 ns/op	  900064 B/op	    8924 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricsfalse-10                	       1	    956583 ns/op	  900064 B/op	    8924 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricstrue-10                 	       1	    979041 ns/op	  932832 B/op	    8925 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricstrue-10                 	       1	   1049791 ns/op	  932832 B/op	    8925 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricstrue-10                 	       1	    965833 ns/op	  932832 B/op	    8925 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricstrue-10                 	       1	    949000 ns/op	  932832 B/op	    8925 allocs/op
BenchmarkHCD059ContextReference/Anclas/N100/Metricstrue-10                 	       1	    964500 ns/op	  932832 B/op	    8925 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricsfalse-10               	       1	   7381500 ns/op	 4601064 B/op	   49419 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricsfalse-10               	       1	   7284667 ns/op	 4601048 B/op	   49419 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricsfalse-10               	       1	   7234209 ns/op	 4600824 B/op	   49415 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricsfalse-10               	       1	   7356083 ns/op	 4600520 B/op	   49412 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricsfalse-10               	       1	   6795042 ns/op	 4600520 B/op	   49412 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricstrue-10                	       1	   6729875 ns/op	 4633704 B/op	   49419 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricstrue-10                	       1	   6821875 ns/op	 4633720 B/op	   49416 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricstrue-10                	       1	   6916792 ns/op	 4633288 B/op	   49413 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricstrue-10                	       1	   6981958 ns/op	 4634136 B/op	   49419 allocs/op
BenchmarkHCD059ContextReference/Anclas/N1000/Metricstrue-10                	       1	   6772625 ns/op	 4633288 B/op	   49413 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricsfalse-10              	       1	  71875666 ns/op	48730560 B/op	  454534 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricsfalse-10              	       1	  69461250 ns/op	48730464 B/op	  454533 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricsfalse-10              	       1	  74138167 ns/op	48878336 B/op	  454547 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricsfalse-10              	       1	  69390083 ns/op	48730448 B/op	  454533 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricsfalse-10              	       1	  70242542 ns/op	48767088 B/op	  454532 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricstrue-10               	       1	  71493417 ns/op	48800464 B/op	  454541 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricstrue-10               	       1	  69432791 ns/op	48726128 B/op	  454529 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricstrue-10               	       1	  69438125 ns/op	48837232 B/op	  454543 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricstrue-10               	       1	  68582250 ns/op	48836928 B/op	  454538 allocs/op
BenchmarkHCD059ContextReference/Anclas/N10000/Metricstrue-10               	       1	  70651458 ns/op	48800160 B/op	  454538 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                                 	       1	    873875 ns/op	  922496 B/op	    4557 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                                 	       1	    876875 ns/op	  885424 B/op	    4552 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                                 	       1	    819917 ns/op	  885424 B/op	    4552 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                                 	       1	    836917 ns/op	  885424 B/op	    4552 allocs/op
BenchmarkHCD059Duplicates/Disperso/N100-10                                 	       1	    843625 ns/op	  922496 B/op	    4557 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                                	       1	   9310000 ns/op	 8437456 B/op	   45203 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                                	       1	   9396833 ns/op	 8405592 B/op	   45203 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                                	       1	   9562708 ns/op	 8437344 B/op	   45202 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                                	       1	   9702750 ns/op	 8400176 B/op	   45196 allocs/op
BenchmarkHCD059Duplicates/Disperso/N1000-10                                	       1	   9400333 ns/op	 8437456 B/op	   45203 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                               	       1	 118427958 ns/op	105534656 B/op	  452207 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                               	       1	 119126958 ns/op	105288256 B/op	  452185 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                               	       1	 118656500 ns/op	105362160 B/op	  452193 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                               	       1	 117853000 ns/op	105521664 B/op	  452194 allocs/op
BenchmarkHCD059Duplicates/Disperso/N10000-10                               	       1	 116222292 ns/op	105670128 B/op	  452216 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                                    	       1	   2252959 ns/op	  601648 B/op	    2235 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                                    	       1	   2283958 ns/op	  601648 B/op	    2235 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                                    	       1	   2279958 ns/op	  601648 B/op	    2235 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                                    	       1	   2253083 ns/op	  601648 B/op	    2235 allocs/op
BenchmarkHCD059Duplicates/Denso/N100-10                                    	       1	   2236041 ns/op	  601648 B/op	    2235 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                                   	       1	 163616875 ns/op	 6031120 B/op	   20359 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                                   	       1	 162134625 ns/op	 5956800 B/op	   20347 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                                   	       1	 163428750 ns/op	 5993872 B/op	   20352 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                                   	       1	 160251000 ns/op	 6068192 B/op	   20364 allocs/op
BenchmarkHCD059Duplicates/Denso/N1000-10                                   	       1	 160643750 ns/op	 5993872 B/op	   20352 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                                  	       1	17418140250 ns/op	68371920 B/op	  200627 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                                  	       1	16163420834 ns/op	68221768 B/op	  200600 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                                  	       1	15787285541 ns/op	68220352 B/op	  200598 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                                  	       1	15604578834 ns/op	68220368 B/op	  200598 allocs/op
BenchmarkHCD059Duplicates/Denso/N10000-10                                  	       1	15498257209 ns/op	68331760 B/op	  200615 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                                	       1	   1502833 ns/op	  600760 B/op	    2408 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                                	       1	   1461083 ns/op	  600760 B/op	    2408 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                                	       1	   1476334 ns/op	  600760 B/op	    2408 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                                	       1	   1462792 ns/op	  600760 B/op	    2408 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N100-10                                	       1	   1459792 ns/op	  600760 B/op	    2408 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                               	       1	  86206292 ns/op	 5386712 B/op	   20585 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                               	       1	  85334625 ns/op	 5497928 B/op	   20600 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                               	       1	  84234667 ns/op	 5386712 B/op	   20585 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                               	       1	  85301083 ns/op	 5423784 B/op	   20590 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N1000-10                               	       1	  83412667 ns/op	 5386888 B/op	   20587 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                              	       1	7711871958 ns/op	63237648 B/op	  201012 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                              	       1	7621406042 ns/op	63123312 B/op	  200990 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                              	       1	7668509459 ns/op	63011920 B/op	  200973 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                              	       1	7724711083 ns/op	63160320 B/op	  200994 allocs/op
BenchmarkHCD059Duplicates/Multitipo/N10000-10                              	       1	7804090334 ns/op	62974848 B/op	  200968 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N100-10                        	       1	   2938500 ns/op	  511560 B/op	    2026 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N100-10                        	       1	   2826458 ns/op	  511560 B/op	    2026 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N100-10                        	       1	   2909542 ns/op	  511560 B/op	    2026 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N100-10                        	       1	   2885291 ns/op	  511560 B/op	    2026 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N100-10                        	       1	   2770500 ns/op	  511560 B/op	    2026 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N1000-10                       	       1	 220839667 ns/op	 4988200 B/op	   20045 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N1000-10                       	       1	 218130708 ns/op	 5136488 B/op	   20065 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N1000-10                       	       1	 220883875 ns/op	 5062344 B/op	   20055 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N1000-10                       	       1	 221157542 ns/op	 5099592 B/op	   20062 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N1000-10                       	       1	 223936625 ns/op	 4988200 B/op	   20045 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N10000-10                      	       1	21490214666 ns/op	52974632 B/op	  200144 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N10000-10                      	       1	21732399666 ns/op	52863528 B/op	  200130 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N10000-10                      	       1	21332235917 ns/op	53048952 B/op	  200156 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N10000-10                      	       1	21458985292 ns/op	52974808 B/op	  200146 allocs/op
BenchmarkHCD059DuplicatesReference/Disperso/N10000-10                      	       1	21251770000 ns/op	52826568 B/op	  200126 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N100-10                           	       1	   2311709 ns/op	  529744 B/op	    1930 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N100-10                           	       1	   2274458 ns/op	  492672 B/op	    1925 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N100-10                           	       1	   2216625 ns/op	  492672 B/op	    1925 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N100-10                           	       1	   2201292 ns/op	  529744 B/op	    1930 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N100-10                           	       1	   2264667 ns/op	  529744 B/op	    1930 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N1000-10                          	       1	 161188000 ns/op	 5182080 B/op	   19042 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N1000-10                          	       1	 160419833 ns/op	 5293296 B/op	   19057 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N1000-10                          	       1	 158707958 ns/op	 5293296 B/op	   19057 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N1000-10                          	       1	 159893000 ns/op	 5219152 B/op	   19047 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N1000-10                          	       1	 160134542 ns/op	 5182192 B/op	   19043 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N10000-10                         	       1	15355810958 ns/op	58170080 B/op	  190103 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N10000-10                         	       1	15274224917 ns/op	57907456 B/op	  190061 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N10000-10                         	       1	15514129542 ns/op	57983416 B/op	  190077 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N10000-10                         	       1	15180175333 ns/op	57907568 B/op	  190062 allocs/op
BenchmarkHCD059DuplicatesReference/Denso/N10000-10                         	       1	15124536625 ns/op	57981600 B/op	  190071 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N100-10                       	       1	   1435042 ns/op	  529688 B/op	    1950 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N100-10                       	       1	   1404292 ns/op	  529688 B/op	    1950 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N100-10                       	       1	   1439875 ns/op	  529688 B/op	    1950 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N100-10                       	       1	   1376708 ns/op	  492616 B/op	    1945 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N100-10                       	       1	   1393875 ns/op	  529688 B/op	    1950 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N1000-10                      	       1	  83439917 ns/op	 4867560 B/op	   19083 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N1000-10                      	       1	  83873709 ns/op	 4793240 B/op	   19071 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N1000-10                      	       1	  84775375 ns/op	 4830424 B/op	   19077 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N1000-10                      	       1	  84529417 ns/op	 4830312 B/op	   19076 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N1000-10                      	       1	  83935500 ns/op	 4793240 B/op	   19071 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N10000-10                     	       1	7691143958 ns/op	55133976 B/op	  190114 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N10000-10                     	       1	7699294750 ns/op	55243600 B/op	  190125 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N10000-10                     	       1	7769485250 ns/op	55169456 B/op	  190115 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N10000-10                     	       1	7650481459 ns/op	55169344 B/op	  190114 allocs/op
BenchmarkHCD059DuplicatesReference/Multitipo/N10000-10                     	       1	7630748750 ns/op	55132272 B/op	  190109 allocs/op
PASS
ok  	mem/application/usecases	356.124s
```

</details>

## Aceptación y regresión T13–T14
Test opt-in con GOMEMORY_PERF_ACCEPTANCE=1: PASS (132,011s), cinco muestras por ruta y comprobación de igualdad antes de medir.
- Duplicados dispersos 10k: referencia 21.634.235.334 ns / 52.862.176 B; optimizado 118.941.875 ns / 105.521.808 B; mejora 181,89x (objetivo ≥5x).
- Contexto SinRelaciones 10k: referencia 49.075.625 ns / 27.870.592 B; optimizado 35.542.209 ns / 11.113.848 B; fracción de asignaciones 0,399 (objetivo ≤0,7).
- El criterio P1 es de latencia en disperso, no de memoria ni del caso denso. La comparación final muestra denso15,787s frente15,274s y asignaciones68,22MB frente57,98MB; multítipo7,712s frente7,691s. No se promete mejora densa. Contexto sin título72,39MB frente77,47MB (~6,6% menos); con relaciones33,20MB frente49,95MB y anclas31,97MB frente48,73MB.
- Regresión integral observada por motor: `go vet ./... && go test -race ./...`, exit0, incluyendo contratos e integración. No se cambian dependencias, se despliega ni se cierra la unidad en este paso.

## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Versionar benchmarks de duplicados dispersos, densos y multítipo a 100/1.000/10.000 elementos en `application/usecases/hcd059_performance_test.go`, con setup fuera del temporizador y ReportAllocs → `go test ./application/usecases -run '^$' -bench '^BenchmarkHCD059Duplicates/Disperso/N100$' -benchtime=1x -benchmem -count=1`
- [x] 2. Versionar benchmarks SQLite de contexto a 100/1.000/10.000 memorias con cuerpos de 1.040 bytes, Budget12000, métricas on/off y variantes con títulos vacíos, relaciones y anclas en `application/usecases/hcd059_performance_test.go` → `go test ./application/usecases -run '^$' -bench '^BenchmarkHCD059Context/SinRelaciones/N100$' -benchtime=1x -benchmem -count=1`
- [x] 3. Registrar la línea base anterior al cambio con cinco muestras para los benchmarks escalados, incluyendo ns/op, B/op, allocs/op, revisión y entorno → `go test ./application/usecases -run '^$' -bench '^BenchmarkHCD059(Duplicates|Context)$' -benchtime=1x -benchmem -count=5`
- [x] 4. Fijar un oráculo exhaustivo de duplicados solo en tests y compararlo con producción para corpus disperso/denso/multítipo, cadenas transitivas, empates, checkpoints, Unicode, tokens vacíos y umbrales negativos/cero/fronterizos/>1/NaN/infinito → `go test ./application/usecases -run '^TestHCD059DuplicateEquivalence$' -count=1`
- [x] 5. Sustituir la enumeración exhaustiva por candidatos de índice invertido para umbrales positivos, deduplicados por memoria, conservando Jaccard exacto y comportamiento de umbrales especiales → `go test ./application/usecases -run '^TestHCD059DuplicateEquivalence$' -count=1`
- [x] 6. Fijar la equivalencia del consumidor BuildContextPack con la detección optimizada, incluidos SuggestedKeepID y candidatos descartados → `go test ./application/usecases -run '^TestHCD059PackDuplicateCompatibility$' -count=1`
- [x] 7. Fijar un test diferencial de contexto frente al camino completo para títulos vacíos/Unicode/controles terminales, conflictos y anclas fuera de las 100 recientes, sinapsis con sesión/hotspots y métricas raw/final, en modo índice/presupuestado/ilimitado → `go test ./application/usecases -run '^TestHCD059ContextEquivalence$' -count=1`
- [x] 8. Declarar ContextMemorySummary y el puerto opcional ContextMemorySummaryLister con los campos y semántica del mapa de impacto, sin ampliar MemoryRepository → `go test ./application/usecases -run '^TestHCD059SummaryPortContract$' -count=1`
- [x] 9. Implementar ListContextSummaries en MemoryRepository con proyección explícita por proyecto, orden ID ascendente y DisplayTitle equivalente a la lectura saneada completa, sin cargar cuerpos de títulos no vacíos → `go test ./adapters/secondary/persistence -run '^TestHCD059ContextSummaries$' -count=1`
- [x] 10. Integrar la lectura opcional de resúmenes en Builder preservando salida y contabilidad de los consumidores globales → `go test ./application/usecases -run '^TestHCD059ContextEquivalence$' -count=1`
- [x] 11. Preservar el fallback de Builder para repositorios sin puerto, errores del resumen y errores de ListAll, sin modificar el error de List reciente → `go test ./application/usecases -run '^TestHCD059ContextSummaryFallback$' -count=1`
- [x] 12. Registrar la comparación posterior de cinco muestras frente a las rutas de referencia exhaustiva/completa bajo los mismos fixtures, con medianas y variantes densas/sin título explícitas → `go test ./application/usecases -run '^$' -bench '^BenchmarkHCD059(Duplicates|Context)$' -benchtime=1x -benchmem -count=5`
- [x] 13. Comprobar automáticamente en harness de aceptación las razones de medianas P1≥5x y B/op P2≤70% de referencia a 10.000 elementos, con guard explícito que omite el test si GOMEMORY_PERF_ACCEPTANCE no vale 1, fuera de CI ordinaria → `GOMEMORY_PERF_ACCEPTANCE=1 go test ./application/usecases -run '^TestHCD059PerformanceAcceptance$' -count=1 -timeout=10m`
- [x] 14. Ejecutar la regresión completa y análisis estático tras los cambios → `go vet ./... && go test -race ./...`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-22 19:50 -05:00 · tarea 1 · rojo observado · caducada · `go test ./application/usecases -run ^TestHCD059DuplicateBenchmarkMatrix$ -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd059_performance_test.go:11:9: undefined: hcd059DuplicateCorpus
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 19:50 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run ^TestHCD059DuplicateBenchmarkMatrix$ -bench ^BenchmarkHCD059Duplicates/Disperso/N100$ -benchtime=1x -benchmem -count=1` (salida 0)
      goos: darwin
      goarch: arm64
      pkg: mem/application/usecases
      cpu: Apple M4
      BenchmarkHCD059Duplicates/Disperso/N100-10    	       1	   2849000 ns/op	  477208 B/op	    2024 allocs/op
      PASS
      ok  	mem/application/usecases	0.346s
- 2026-11-22 19:51 -05:00 · tarea 2 · rojo observado · caducada · `go test ./application/usecases -run ^TestHCD059ContextBenchmarkFixture$ -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd059_performance_test.go:14:13: undefined: hcd059ContextFixture
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 19:51 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run ^TestHCD059DuplicateBenchmarkMatrix$ -bench ^BenchmarkHCD059Duplicates/Disperso/N100$ -benchtime=1x -benchmem -count=1` (salida 0)
      goos: darwin
      goarch: arm64
      pkg: mem/application/usecases
      cpu: Apple M4
      BenchmarkHCD059Duplicates/Disperso/N100-10    	       1	   3679542 ns/op	  514248 B/op	    2027 allocs/op
      PASS
      ok  	mem/application/usecases	0.197s
- 2026-11-22 19:51 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run ^TestHCD059ContextBenchmarkFixture$ -bench ^BenchmarkHCD059Context/SinRelaciones/N100$ -benchtime=1x -benchmem -count=1` (salida 0)
      goos: darwin
      goarch: arm64
      pkg: mem/application/usecases
      cpu: Apple M4
      BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    793333 ns/op	  671168 B/op	    5623 allocs/op
      BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    738833 ns/op	  704352 B/op	    5627 allocs/op
      PASS
      ok  	mem/application/usecases	0.225s
- 2026-11-22 19:57 -05:00 · tarea 1 · verde · caducada · `go test ./application/usecases -run ^TestHCD059DuplicateBenchmarkMatrix$ -bench ^BenchmarkHCD059Duplicates/Disperso/N100$ -benchtime=1x -benchmem -count=1` (salida 0)
      goos: darwin
      goarch: arm64
      pkg: mem/application/usecases
      cpu: Apple M4
      BenchmarkHCD059Duplicates/Disperso/N100-10    	       1	   3823083 ns/op	  514248 B/op	    2027 allocs/op
      PASS
      ok  	mem/application/usecases	0.362s
- 2026-11-22 19:57 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run ^TestHCD059ContextBenchmarkFixture$ -bench ^BenchmarkHCD059Context/SinRelaciones/N100$ -benchtime=1x -benchmem -count=1` (salida 0)
      goos: darwin
      goarch: arm64
      pkg: mem/application/usecases
      cpu: Apple M4
      BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    859542 ns/op	  671584 B/op	    5626 allocs/op
      BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    824042 ns/op	  704352 B/op	    5627 allocs/op
      PASS
      ok  	mem/application/usecases	0.247s
- 2026-11-22 19:57 -05:00 · tarea 3 · manual · caducada
      Medición observada: go test ./application/usecases -run ^$ -bench ^BenchmarkHCD059(Duplicates|Context)$ -benchtime=1x -benchmem -count=5. Exit 0; 225.692s. 33 casos, cinco muestras cada uno; muestras y medianas registradas en la sección Línea base versionada del documento HCD. HEAD3847db40e457, Go1.27.4 AppleM4 arm64 GOMAXPROCS10. Sin cambios productivos; medición sin rojo funcional.
- 2026-11-22 19:58 -05:00 · tarea 4 · rojo observado · caducada · `go test ./application/usecases -run ^TestHCD059DuplicateEquivalence$ -count=1` (salida 1)
      # mem/application/usecases [mem/application/usecases.test]
      application/usecases/hcd059_duplicates_test.go:8:2: "sort" imported and not used
      application/usecases/hcd059_duplicates_test.go:38:9: undefined: hcd059ExhaustiveGroups
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 19:59 -05:00 · tarea 4 · verde · caducada · `go test ./application/usecases -run ^TestHCD059DuplicateEquivalence$ -count=1` (salida 0)
      ok  	mem/application/usecases	0.351s
- 2026-11-22 19:59 -05:00 · tarea 5 · rojo observado · caducada · `go test ./application/usecases -run ^TestHCD059Duplicate(CandidatePruning|Equivalence)$ -count=1` (salida 2)
      sh: -c: line 0: syntax error near unexpected token `('
      sh: -c: line 0: `go test ./application/usecases -run ^TestHCD059Duplicate(CandidatePruning|Equivalence)$ -count=1'
- 2026-11-22 19:59 -05:00 · tarea 5 · rojo observado · caducada · `go test ./application/usecases -run TestHCD059Duplicate -count=1` (salida 1)
      # mem/application/usecases [mem/application/usecases.test]
      application/usecases/hcd059_duplicates_test.go:74:2: undefined: forEachDuplicateCandidate
      application/usecases/hcd059_duplicates_test.go:77:2: undefined: forEachDuplicateCandidate
      application/usecases/hcd059_duplicates_test.go:81:2: undefined: forEachDuplicateCandidate
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 20:00 -05:00 · tarea 4 · verde · caducada · `go test ./application/usecases -run TestHCD059Duplicate -count=1` (salida 0)
      ok  	mem/application/usecases	0.220s
- 2026-11-22 20:00 -05:00 · tarea 5 · verde · caducada · `go test ./application/usecases -run TestHCD059Duplicate -count=1` (salida 0)
      ok  	mem/application/usecases	0.211s
- 2026-11-22 20:01 -05:00 · tarea 4 · verde · caducada · `go test ./application/usecases -run TestHCD059Duplicate -count=1` (salida 0)
      ok  	mem/application/usecases	0.234s
- 2026-11-22 20:01 -05:00 · tarea 5 · verde · caducada · `go test ./application/usecases -run TestHCD059Duplicate -count=1` (salida 0)
      ok  	mem/application/usecases	0.269s
- 2026-11-22 20:02 -05:00 · tarea 6 · rojo observado · caducada · `go test ./application/usecases -run TestHCD059PackDuplicateCompatibility -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd059_pack_test.go:11:8: undefined: hcd059PackCandidates
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 20:02 -05:00 · tarea 6 · verde · caducada · `go test ./application/usecases -run TestHCD059PackDuplicateCompatibility -count=1` (salida 0)
      ok  	mem/application/usecases	0.186s
- 2026-11-22 20:03 -05:00 · tarea 4 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.381s
- 2026-11-22 20:03 -05:00 · tarea 5 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.375s
- 2026-11-22 20:03 -05:00 · tarea 6 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.418s
- 2026-11-22 20:04 -05:00 · tarea 7 · rojo observado · caducada · `go test ./application/usecases -run TestHCD059ContextEquivalence -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd059_context_test.go:8:2: "mem/application/usecases" imported and not used
      application/usecases/hcd059_context_test.go:29:14: undefined: hcd059LegacyBuilder
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 20:04 -05:00 · tarea 7 · falló · caducada · `go test ./application/usecases -run TestHCD059ContextEquivalence -count=1` (salida 1)
      --- FAIL: TestHCD059ContextEquivalence (0.08s)
          --- FAIL: TestHCD059ContextEquivalence/Relaciones (0.02s)
              hcd059_context_test.go:39: insert relation: constraint failed: UNIQUE constraint failed: memory_relations.project, memory_relations.memory_id_a, memory_relations.memory_id_b (2067)
      FAIL
      FAIL	mem/application/usecases	0.288s
      FAIL
- 2026-11-22 20:04 -05:00 · tarea 7 · verde · caducada · `go test ./application/usecases -run TestHCD059ContextEquivalence -count=1` (salida 0)
      ok  	mem/application/usecases	0.294s
- 2026-11-22 20:05 -05:00 · tarea 8 · rojo observado · caducada · `go test ./application/usecases -run TestHCD059SummaryPortContract -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd059_summary_port_test.go:10:63: undefined: domain.ContextMemorySummary
      application/usecases/hcd059_summary_port_test.go:11:18: undefined: domain.ContextMemorySummary
      application/usecases/hcd059_summary_port_test.go:14:14: undefined: ports.ContextMemorySummaryLister
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 20:05 -05:00 · tarea 8 · verde · caducada · `go test ./application/usecases -run TestHCD059SummaryPortContract -count=1` (salida 0)
      ok  	mem/application/usecases	0.205s
- 2026-11-22 20:05 -05:00 · tarea 9 · rojo observado · caducada · `go test ./adapters/secondary/persistence -run TestHCD059ContextSummaries -count=1` (salida 1)
      --- FAIL: TestHCD059ContextSummaries (0.02s)
          hcd059_context_summaries_test.go:19: repository lacks summary port
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.205s
      FAIL
- 2026-11-22 20:06 -05:00 · tarea 9 · verde · caducada · `go test ./adapters/secondary/persistence -run TestHCD059ContextSummaries -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.212s
- 2026-11-22 20:06 -05:00 · tarea 10 · rojo observado · caducada · `go test ./application/usecases -run TestHCD059ContextEquivalence -count=1` (salida 1)
              --- FAIL: TestHCD059ContextEquivalence/Relaciones/budget0/indexfalse (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
              --- FAIL: TestHCD059ContextEquivalence/Relaciones/budget0/indextrue (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
              --- FAIL: TestHCD059ContextEquivalence/Relaciones/budget12000/indexfalse (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
              --- FAIL: TestHCD059ContextEquivalence/Relaciones/budget12000/indextrue (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
          --- FAIL: TestHCD059ContextEquivalence/Anclas (0.02s)
              --- FAIL: TestHCD059ContextEquivalence/Anclas/budget0/indexfalse (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
              --- FAIL: TestHCD059ContextEquivalence/Anclas/budget0/indextrue (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
              --- FAIL: TestHCD059ContextEquivalence/Anclas/budget12000/indexfalse (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
              --- FAIL: TestHCD059ContextEquivalence/Anclas/budget12000/indextrue (0.00s)
                  hcd059_context_test.go:68: summary calls=0 full calls=1
      FAIL
      FAIL	mem/application/usecases	0.324s
      FAIL
- 2026-11-22 20:06 -05:00 · tarea 7 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.376s
- 2026-11-22 20:06 -05:00 · tarea 8 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.525s
- 2026-11-22 20:06 -05:00 · tarea 10 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.357s
- 2026-11-22 20:07 -05:00 · tarea 11 · rojo observado · caducada · `go test ./application/usecases -run TestHCD059ContextSummaryFallback -count=1` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd059_context_fallback_test.go:22:31: undefined: hcd059ListOnly
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 20:07 -05:00 · tarea 11 · verde · caducada · `go test ./application/usecases -run TestHCD059ContextSummaryFallback -count=1` (salida 0)
      ok  	mem/application/usecases	0.217s
- 2026-11-22 20:07 -05:00 · tarea 7 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.383s
- 2026-11-22 20:07 -05:00 · tarea 8 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.368s
- 2026-11-22 20:07 -05:00 · tarea 9 · verde · caducada · `go test ./adapters/secondary/persistence -run TestHCD059 -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.198s
- 2026-11-22 20:08 -05:00 · tarea 10 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.384s
- 2026-11-22 20:08 -05:00 · tarea 11 · verde · caducada · `go test ./application/usecases -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.363s
- 2026-11-22 20:15 -05:00 · tarea 12 · manual · caducada
      Comparación observada exit0 en356.124s: 66casos x5muestras, referencias congeladas union-find y contexto completo; comando go test -bench BenchmarkHCD059 -benchtime=1x -benchmem -count=5, selección exacta y muestras completas/medianas en sección Comparación final T12. Medición sin rojo funcional.
- 2026-11-22 20:15 -05:00 · tarea 13 · rojo observado · caducada · `env GOMEMORY_PERF_ACCEPTANCE=1 go test ./application/usecases -run TestHCD059PerformanceAcceptance -count=1 -timeout=10m` (salida 1)
      # mem/application/usecases_test [mem/application/usecases.test]
      application/usecases/hcd059_acceptance_test.go:10:2: undefined: hcd059CheckPerformance
      FAIL	mem/application/usecases [build failed]
      FAIL
- 2026-11-22 20:16 -05:00 · tarea 13 · verde · vigente · `env GOMEMORY_PERF_ACCEPTANCE=1 go test ./application/usecases -run TestHCD059PerformanceAcceptance -v -count=1 -timeout=10m` (salida 0)
      === RUN   TestHCD059PerformanceAcceptance
          hcd059_acceptance_test.go:43: duplicates reference=21634235334ns/52862176B optimized=118941875ns/105521808B speedup=181.89x
          hcd059_acceptance_test.go:75: context reference=49075625ns/27870592B optimized=35542209ns/11113848B fraction=0.399
      --- PASS: TestHCD059PerformanceAcceptance (131.82s)
      PASS
      ok  	mem/application/usecases	132.011s
- 2026-11-22 20:18 -05:00 · tarea 14 · verde · vigente · `go vet ./... && go test -race ./...` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	6.268s
      ok  	mem/adapters/secondary/hostbind	1.396s
      ok  	mem/adapters/secondary/i18n	1.164s
      ok  	mem/adapters/secondary/launcher	1.673s
      ok  	mem/adapters/secondary/persistence	169.922s
      ok  	mem/adapters/secondary/release	1.515s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	1.285s
      ok  	mem/adapters/secondary/speckit	1.253s
      ok  	mem/adapters/secondary/tokens	1.160s
      ok  	mem/adapters/secondary/usage	1.169s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	139.298s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	1.737s
      ok  	mem/domain/i18n	(cached)
      ok  	mem/infrastructure	9.875s
      ok  	mem/tests/contract	128.206s
      ok  	mem/tests/integration	209.459s
      ?   	mem/version	[no test files]
- 2026-11-22 20:23 -05:00 · tarea 1 · verde · vigente · `go test ./application/usecases -run TestHCD059DuplicateBenchmarkMatrix -bench BenchmarkHCD059Duplicates/Disperso/N100$ -benchtime=1x -benchmem -count=1` (salida 0)
      goos: darwin
      goarch: arm64
      pkg: mem/application/usecases
      cpu: Apple M4
      BenchmarkHCD059Duplicates/Disperso/N100-10         	       1	   1006250 ns/op	  885456 B/op	    4554 allocs/op
      BenchmarkHCD059DuplicatesReference/Disperso/N100-10         	       1	   3207834 ns/op	  511560 B/op	    2026 allocs/op
      PASS
      ok  	mem/application/usecases	0.230s
- 2026-11-22 20:23 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases -run TestHCD059ContextBenchmarkFixture -bench BenchmarkHCD059Context/SinRelaciones/N100$ -benchtime=1x -benchmem -count=1` (salida 0)
      goos: darwin
      goarch: arm64
      pkg: mem/application/usecases
      cpu: Apple M4
      BenchmarkHCD059Context/SinRelaciones/N100/Metricsfalse-10         	       1	    740250 ns/op	  534456 B/op	    4815 allocs/op
      BenchmarkHCD059Context/SinRelaciones/N100/Metricstrue-10          	       1	    649209 ns/op	  567224 B/op	    4816 allocs/op
      BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricsfalse-10         	       1	    825708 ns/op	  671584 B/op	    5626 allocs/op
      BenchmarkHCD059ContextReference/SinRelaciones/N100/Metricstrue-10          	       1	    829459 ns/op	  704400 B/op	    5628 allocs/op
      PASS
      ok  	mem/application/usecases	0.244s
- 2026-11-22 20:23 -05:00 · tarea 4,5,6,7,8,9,10,11 · verde · vigente · `go test ./application/usecases ./adapters/secondary/persistence -run TestHCD059 -count=1` (salida 0)
      ok  	mem/application/usecases	0.368s
      ok  	mem/adapters/secondary/persistence	0.183s
- 2026-11-22 20:23 -05:00 · tarea 3,12 · manual · vigente
      Revalidación documental histórica: T3 contiene165muestras previas de producción sin optimizar (225.692s, exit0); T12 contiene330muestras con rutas congeladas y optimizadas en mismo proceso (356.124s, exit0). Se conserva todo el log y medianas en documento; no se reejecuta retrospectivamente T3 sobre producción modificada. Desde T12 solo se añadió el harness de aceptación, sin cambios de producción ni fixtures comparados. T13 luego confirmó181.89x y fracción0.399, y T14 pasó vet/race integral. Observación manual explícita, no nueva ejecución de los benchmarks.

## Siguiente
Cerrada con aprobación sobre la revisión 8f19b2493f98.
