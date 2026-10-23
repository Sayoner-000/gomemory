# Medición de apertura en Windows

Línea base: Windows CI, workflow `diag-windows-timing`, run [39941668694](https://github.com/Sayoner-000/gomemory/actions/runs/39941668694), 2026-10-23, cinco repeticiones por prueba. Las columnas posteriores se completan con mediciones del mismo runner e instrumentación.

| Métrica | Línea base (p50 / máximo) | Después (transacción) | Después (con R9 e índice) |
|---|---|---|---|
| `db.open`, base nueva | 554 / 1.760 ms | | |
| `db.open`, base preparada | 6 / 8 ms | | |
| `fold`, cierre del primer turno | 581 / 1.809 ms | | |
| Registro de sugerencia o bloqueo (`flush`) | 42 / 1.592 ms | | |
| `toolout` | Sin agregado en la tabla original; máximo de referencia 71 ms | | |
| `close`, por comando y canal | Sin línea base | | |
| Apertura de base al día con escritor abierto (`contention.open`, SC-010) | Sin línea base | | |
| `TestHookExpandInstinctsTwoProjects`, PASS/FAIL | Cinco repeticiones; sin recuento en la tabla original | | |
| `TestHookExpand_SuggestionOncePerEpisodeAndRecorded`, PASS/FAIL | Cinco repeticiones; sin recuento en la tabla original | | |

Pendiente: medir 20 repeticiones en la rama de diagnóstico; estos espacios vacíos no representan resultados locales ni objetivos cumplidos.
