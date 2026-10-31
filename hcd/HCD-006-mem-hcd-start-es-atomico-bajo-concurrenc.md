# mem hcd start es atómico bajo concurrencia

ID: HCD-006
Tipo: fix
Carril: fast
Motivo: Fix con reproducción clara en binario real, sin decisiones interdependientes

## Intención
Dos mem hcd start concurrentes terminan con números distintos, sin error del driver, sin docs huérfanos y sin que un perdedor sobrescriba el doc de otra unidad (C-003, C-004, H-01)

## Reproducción
Lanzar 8 `mem hcd start` a la vez sobre un proyecto limpio. Hoy falla: con títulos distintos sale `constraint failed: UNIQUE constraint failed: hcd_units.project, hcd_units.number (2067)` con código `internal` y queda un doc sin fila (`HCD-002-paralelo-2.md`). Con título igual, `HCD-012` respondió `ok:true` y su doc contenía `intent-b5`, el intent de otro proceso. La prueba `TestHCDStartConcurrente` lo fija contra SQLite real.

## Alcance
Incluido: número, inserción, activación y escritura del doc en una sola operación del store (C-003, C-004, H-01).
Excluido: que `Start` desactive sin avisar la unidad activa anterior; el comportamiento se conserva y se decide aparte. Un fallo del propio `Commit` puede dejar el doc escrito: riesgo residual aceptado, limitado a fallos de disco.

## ¿Qué pasa si no lo hacemos?
Sesiones o subagentes en paralelo ven un error interno del driver, acumulan docs huérfanos y, con título igual, una unidad aceptada puede quedarse con la intención de otra sin que nadie lo note.

## Alternativa más barata
Reintentar ante UNIQUE sin mover `WriteDoc` eliminaría el error, pero no la sobrescritura ni los huérfanos. La propuesta de H-01 (crear primero y compensar después) exige un borrado compensatorio que puede fallar. Escribir el doc dentro de la transacción es más simple y no añade ninguna operación nueva al store salvo `CreateActive`.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./application/usecases/ -run 'TestHCDStartConcurrente$' -count=3`
- [x] 2. Un fallo al escribir el doc no deja unidad y el número sigue libre (dep: 1) → `go test ./application/usecases/ -run 'TestHCDStartFalloDeDoc$' -count=1`
- [x] 3. 8 procesos reales del binario dan 8 unidades con su propio intent (dep: 1) → observación manual con `scripts` en HOME aislado

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-31 15:02 · tarea 1 · rojo observado · caducada · `go test ./application/usecases/ -run 'TestHCDStartConcurrente$' -count=3` (salida 1)
      --- FAIL: TestHCDStartConcurrente (0.11s)
          hcd_start_atomic_test.go:69: HCD-001 no contiene su intent "intent-0-fin"
      --- FAIL: TestHCDStartConcurrente (0.09s)
          hcd_start_atomic_test.go:58: competidor 0: constraint failed: UNIQUE constraint failed: hcd_units.project, hcd_units.number (2067)
      --- FAIL: TestHCDStartConcurrente (0.09s)
          hcd_start_atomic_test.go:58: competidor 0: constraint failed: UNIQUE constraint failed: hcd_units.project, hcd_units.number (2067)
      FAIL
      FAIL	mem/application/usecases	0.472s
      FAIL
- 2026-10-31 15:03 · tarea 1 · verde · vigente · `go test ./application/usecases/ -run 'TestHCDStartConcurrente$' -count=3` (salida 0)
      ok  	mem/application/usecases	0.580s
- 2026-10-31 15:03 · tarea 2 · verde · vigente · `go test ./application/usecases/ -run 'TestHCDStartFalloDeDoc$' -count=1` (salida 0)
      ok  	mem/application/usecases	0.237s
- 2026-10-31 15:03 · tarea 3 · verde · vigente · `/private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad/repro8.sh /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad` (salida 0)
      unidades 8 docs 8 fallos 0

## Siguiente
Cerrada con aprobación sobre la revisión 7a995d87497a.
