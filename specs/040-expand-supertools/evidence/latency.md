# Evidencia T074: latencia de `mem hook expand` (5 eventos)

- **Fecha**: 2026-10-21
- **Binario**: `mem` recompilado desde el árbol de trabajo, macOS (darwin)
- **Montaje**: proyecto temporal con **64 reglas regex propias** en `.gomemory/rules/`, `GOMEMORY_DATA_HOME` aislado y 100 invocaciones por evento (proceso completo: arranque, stdin, decisión y salida), medidas desde fuera con `Time::HiRes`. Para `stop`, la cola tenía **500 observaciones** antes de cada lote.

## Resultado en condiciones limpias

| Evento | p50 | p95 | máx. | Objetivo | Veredicto |
|---|---|---|---|---|---|
| referencia: `mem version` (solo arranque) | 21,5 ms | 23,1 ms | 54,6 ms | — | — |
| `pre-tool` | 22,3 ms | 24,3 ms | 36,0 ms | p95 < 150 ms | PASS |
| `prompt` | 22,0 ms | 22,6 ms | 23,8 ms | p95 < 150 ms | PASS |
| `post-tool` | 21,8 ms | 22,4 ms | 22,8 ms | < 20 ms | ver nota |
| `session-start` | 26,6 ms | 30,1 ms | 31,7 ms | p95 < 150 ms | PASS |
| `stop` (cola de 500, plegado incluido) | 51,1 ms | 72,6 ms | 74,4 ms | p95 < 150 ms | PASS; la cola quedó a 0 |

**Nota sobre `post-tool`**: el objetivo de 20 ms del plan no se puede cumplir con un proceso por evento, porque solo arrancar el binario (`mem version`) ya cuesta 23,1 ms de p95. El coste propio de expand en `post-tool` es inferior a 1 ms sobre esa referencia. SC-002 (p95 < 150 ms) se cumple en los 5 eventos.

**Cruce del umbral de `strategic-compact`**: 150 invocaciones seguidas de `pre-tool` en una misma sesión (que emiten la sugerencia al llegar a 50 y abren la base de forma perezosa para registrarla) dieron un máximo de 56,5 ms.

## Incidencia durante la primera medición

La primera medición dio atípicos de **25 s y 638 s** en `pre-tool`, con la máquina en load average 137. La causa eran **10 procesos `yes` huérfanos** (PPID 1, arrancados entre las 07:05 y las 07:14 durante el trabajo de otra sesión, sin relación con este repositorio) que consumían ~95 % de CPU cada uno desde hacía 5 horas. En esa misma franja, una prueba de OpenCode ajena a expand (`execFileExec cancela un subproceso real`) tardó 926 s. Tras terminar esos procesos, los atípicos no se reproducen en 150 + 500 invocaciones. No se atribuyen a expand.
