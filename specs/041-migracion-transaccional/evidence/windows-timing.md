# Medición de apertura en Windows

Línea base: Windows CI, workflow `diag-windows-timing`, run [39941668694](https://github.com/Sayoner-000/gomemory/actions/runs/39941668694), 2026-10-23, cinco repeticiones por prueba. Las columnas posteriores se completan con mediciones del mismo runner e instrumentación.

| Métrica | Línea base (p50 / máximo) | Después (transacción) | Después (con R9 e índice) |
|---|---|---|---|
| `db.open`, base nueva | 554 / 1.760 ms | 62,5 / 195 ms en expand stop; 964 ms máx en tool-output (run 39954797737) | 40 / 219 ms (100 aperturas) |
| `db.open`, base preparada | 6 / 8 ms | 5 / 9 ms | 3 / 10 ms (240) |
| `fold`, cierre del primer turno | 581 / 1.809 ms | 134,5 / 335 ms | 87,5 / 264 ms (40) |
| Registro de sugerencia o bloqueo (`flush`) | 42 / 1.592 ms | 41 / 183 ms | 20,5 / 239 ms (pre-tool, 60) |
| `toolout` | Sin agregado en la tabla original; máximo de referencia 71 ms | 34 / 150 ms (Claude, 80) | 17 / 27 ms (Claude, 80) |
| `close`, por comando y canal | Sin línea base | context 27,5 / 63 ms | context 53,5 / 214 ms; `optimize` 1 / 2 ms (run 40046457914) |
| Apertura de base al día con escritor abierto (`contention.open`, SC-010) | Sin línea base | — | `db_migrate_contention_test.go` en verde en Windows (go test, run 40045564958) |
| `TestHookExpandInstinctsTwoProjects`, PASS/FAIL | Cinco repeticiones; sin recuento en la tabla original | 20 / 0 | 20 / 0 (40045565011); 18 / 2 (40046457914, con bloqueos del runner) |
| `TestHookExpand_SuggestionOncePerEpisodeAndRecorded`, PASS/FAIL | Cinco repeticiones; sin recuento en la tabla original | 20 / 0 | 20 / 0 (40045565011); 18 / 2 (40046457914, con bloqueos del runner) |

Pendiente: medir 20 repeticiones en la rama de diagnóstico; estos espacios vacíos no representan resultados locales ni objetivos cumplidos.

## Después de la transacción: run 39954797737

[Run Windows](https://github.com/Sayoner-000/gomemory/actions/runs/39954797737), commit `1b44900`, 20 repeticiones. Resultado global: FAIL (`TestHookToolOutput`, 19 PASS / 1 FAIL). No se publica una release de la rama de diagnóstico.

| Comprobación | Resultado | Estado |
|---|---|---|
| SC-001: `TestHookExpandInstinctsTwoProjects` | 20 PASS / 0 FAIL | PASS |
| SC-001: `TestHookExpand_SuggestionOncePerEpisodeAndRecorded` | 20 PASS / 0 FAIL | PASS |
| Sesión en segundo plano | 20 PASS / 0 FAIL | PASS |
| SC-002: nueva base en expand stop | 62,5 ms p50 / 195 ms máximo (40 aperturas) | PASS en esta ruta |
| SC-002: nueva base en tool-output Claude | 964 ms máximo | FAIL global (< 400 ms) |
| SC-003: fold | 134,5 ms p50 / 335 ms máximo (40) | PASS |
| Base preparada, comando context | 5 ms p50 / 9 ms máximo (20) | Referencia |
| Flush pre-tool | 41 ms p50 / 183 ms máximo (60) | Referencia |
| Cierre context | 27,5 ms p50 / 63 ms máximo (20) | Referencia |
| Tool-output Claude, trabajo acotado | 34 ms p50 / 150 ms máximo (80) | Referencia |

La primera apertura de Claude en la repetición 19 (índice 108 de las 120 aperturas de ese comando, seis por repetición) corresponde a una base nueva: la prueba crea un proyecto `on` nuevo antes de `claude-bash.json`. El span de `pid=4264` va de +62 ms a +1026 ms: 964 ms. No se descarta como ruido. En esa repetición se registra después salida vacía para `claude-mcp.json`; la coincidencia no demuestra una causa común.

Agregados por comando en `windows-transaction-spans.json`. Fuentes descargadas: `/private/tmp/gomemory-041-windows-artifact/{timing.log,timing-tests.log}`. Los spans se emparejan por PID, comando y etiqueta begin/end, usando los milisegundos relativos del proceso; los contadores PASS/FAIL salen del registro completo de pruebas.

**Puerta de T014:** SC-002 no se cumple. Se detiene el avance de producción y se vuelve a research; no se cambian los plazos. La columna final con R9 e índice permanece pendiente.

## Diagnóstico por fase: run 40043142611

[Run](https://github.com/Sayoner-000/gomemory/actions/runs/40043142611), commit `9e87816`, sin cambios de SQL ni configuración. Los agregados reproducibles están en `windows-granular-joint.json` y `windows-granular-isolated.json`.

| Medición | Tanda conjunta | Tool-output aislado |
|---|---|---|
| Ambas pruebas US1 | 20/20 cada una | No ejecutadas |
| Sesión en segundo plano | 20/20 | No ejecutada |
| Tool-output | 19/20 | 20/20 |
| Base nueva, p50 / máximo | 62 / 1.435 ms (100) | 64 / 82 ms (40) |
| Base existente, p50 / máximo | 4 / 8 ms (240) | 4 / 6 ms (140) |
| Fold, p50 / máximo | 134,5 / 282 ms (40) | No ejecutado |

Peor apertura nueva de la tanda conjunta: creación del archivo 1 ms, primera conexión y pragmas 1.035 ms, bloqueo de transacción 0 ms, DDL 16 ms, commit 383 ms, total 1.435 ms. La segunda tarda 850 ms y dedica 664 ms al commit. El tiempo dominante no es ejecutar el DDL ni esperar `BEGIN`. La diferencia entre tandas apunta a sensibilidad a la carga concurrente, pero no identifica por sí sola el mecanismo del sistema operativo. La instrumentación añade escrituras al archivo de marcas y no debe confundirse con una comparación de tiempos sin instrumentación.

Reproducción de agregados:

```sh
python3 specs/041-migracion-transaccional/evidence/aggregate_windows_timing.py timing.log timing-tests.log
python3 specs/041-migracion-transaccional/evidence/aggregate_windows_timing.py timing-isolated.log timing-isolated-tests.log
```

SC-002 sigue en FAIL en la tanda conjunta. La tanda aislada no reemplaza la aceptación conjunta. Siguiente experimento preparado: comparar `synchronous=NORMAL` de R9 contra el control FULL dentro del mismo workflow, sin adelantar cambios a la rama de producción.

## Comparación FULL/NORMAL: run 40043862213

[Run](https://github.com/Sayoner-000/gomemory/actions/runs/40043862213), commit `40e17c6`. Agregados `windows-sync-{full,isolated,normal}.json`. Las tres tandas se ejecutan en ese orden sobre el mismo runner; no es un experimento aleatorizado y no identifica una causa del sistema operativo.

| Métrica | FULL conjunto | FULL aislado | NORMAL conjunto |
|---|---|---|---|
| InstinctsTwoProjects | 19/20 | — | 20/20 |
| SuggestionOncePerEpisodeAndRecorded | 19/20 | — | 20/20 |
| SessionStart | 20/20 | — | 20/20 |
| ToolOutput | 19/20 | 19/20 | 20/20 |
| Base nueva p50 / máximo | 66 / 3.574 ms | 62,5 / 155 ms | 42 / 271 ms |
| Base existente p50 / máximo | 3 / 14 ms | 2 / 5 ms | 2 / 11 ms |
| Fold p50 / máximo | 137 / 2.026 ms | — | 106 / 437 ms |

NORMAL cumple SC-001, SC-002 y SC-003 en esta tanda. Se continúa con la implementación de R9 y la revalidación del conjunto final. El workflow global permanece fallido porque conserva los fallos de control; el paso NORMAL es el resultado experimental favorable. Esto no sustituye el CI final ni las cifras de la columna con todos los pragmas e índice.

## Configuración final (R9 e índice): runs 40045565011 y 40046457914

[Run 40045565011](https://github.com/Sayoner-000/gomemory/actions/runs/40045565011), commit `544ae53`. La tanda conjunta pasa 20/20 en las cuatro pruebas. Agregados: `windows-final-joint.json` y `windows-final-isolated.json`. Resultados: base nueva 219 ms máx (SC-002 < 400, PASS), base preparada 3 ms p50 (SC-004 ≤ 10, PASS) y fold 264 ms máx (SC-003 < 500, PASS). La tanda aislada de tool-output abre una base nueva en 77 ms máx.

[Run 40046457914](https://github.com/Sayoner-000/gomemory/actions/runs/40046457914), commit `0258dd1`. Añade las marcas `close.optimize` y `close.db`; agregado en `windows-close-split-joint.json`. `optimize` cuesta 1 ms p50 / 2 ms máx: FR-013 se cumple, porque las estadísticas suman como mucho 2 ms al cierre. El crecimiento de `close` frente a la transacción (27,5 / 63 ms → 53,5 / 214 ms) está en `db.Close` (48,5 / 203 ms): con `synchronous=NORMAL`, las sincronizaciones a disco se aplazan al checkpoint del WAL que hace la última conexión. Es el coste esperado de R9 y lo pagan los comandos de la CLI, no los hooks medidos, que terminan sin `Close`. En este run hubo bloqueos del runner que afectan también a operaciones sin escritura: una apertura de base existente llegó a 1.559 ms, frente a 10 ms máx en el run anterior. Hubo 2 fallos en cada prueba US1 (18/20). Se registra sin descartarlo: de las tres tandas NORMAL, dos pasan 20/20 y una 18/20 con bloqueos.
