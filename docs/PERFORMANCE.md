# Rendimiento de duplicados y contexto

Desde v2.55.2, la detección de duplicados usa un índice de tokens para evitar pares sin coincidencias. La decisión sigue usando Jaccard exacto y conserva los grupos transitivos, la separación por tipo y la memoria sugerida para conservar. En conjuntos densos vuelve a recorrer los pares cuando consultar el índice deja de ahorrar trabajo.

El contexto mantiene los cuerpos de las 100 memorias recientes y lee una proyección del historial para títulos, sesión, sinapsis y anclas. Los títulos vacíos requieren leer y sanear el cuerpo antes de obtener el extracto. Los repositorios sin el puerto opcional, o que devuelvan un error, conservan la lectura completa anterior. No cambia el esquema SQLite ni se eliminan memorias.

## Resultados y límites

Medianas de cinco muestras con Go 1.27.4 en Apple M4, 10.000 memorias sintéticas. Los bytes son asignaciones acumuladas por operación, no memoria residente ni pico de uso.

| Operación | Referencia | v2.55.2 |
|---|---:|---:|
| Duplicados con vocabulario disperso | 21,63 s | 119 ms |
| Bytes asignados por esos duplicados | 52,86 MB | 105,52 MB |
| Construcción de contexto con título | 49,08 ms | 35,54 ms |
| Bytes asignados por ese contexto | 27,87 MB | 11,11 MB |

El índice intercambia memoria por tiempo: en este corpus los duplicados son unas 182 veces más rápidos y asignan aproximadamente el doble de bytes. No hay una mejora equivalente en corpus densos: en la comparación completa tardaron 15,79 s frente a 15,27 s. El ahorro de contexto con títulos vacíos fue del 6,6%, frente al 60,1% del caso con título. Estas mediciones no son garantías de latencia para datos reales.

## Reproducir

```bash
go test ./application/usecases -run '^$' -bench '^BenchmarkHCD059(Duplicates|Context)(Reference)?$' -benchtime=1x -benchmem -count=5 -timeout=9m
GOMEMORY_PERF_ACCEPTANCE=1 go test ./application/usecases -run '^TestHCD059PerformanceAcceptance$' -v -count=1 -timeout=10m
```

La aceptación exige al menos 5 veces menos tiempo en duplicados dispersos y como máximo el 70% de los bytes de referencia en contexto con título. Es optativa y queda fuera de la suite ordinaria para no imponer tiempos de una máquina a CI. Las pruebas diferenciales comparan resultados, títulos Unicode, conflictos antiguos, métricas y rutas de fallback.

Las muestras completas, los comandos y las limitaciones están en [HCD-059](../hcd/HCD-059-optimizar-duplicados-y-construccion-de-c.md). Para usar el código publicado, actualiza con `mem update` y reinicia los servidores MCP que continúen ejecutando el binario anterior.
