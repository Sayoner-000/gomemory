# uninstall --all no borra binarios fuera de HOME

ID: HCD-011
Tipo: fix
Carril: fast
Motivo: Fix con reproducción clara y decisión tomada por la persona

## Intención
mem uninstall --all solo elimina binarios dentro del HOME de la ejecución; uno fuera se conserva y se indica cómo borrarlo a mano

## Reproducción
Con `HOME` temporal y dos copias del binario fuera de él, una en `PATH` y otra ejecutada: `mem uninstall --all --yes` borra ambas y sale con 0. Aislar `HOME` no protege el binario, porque `planBinaries` lo resuelve por `PATH`.

## Alcance
Incluido: `mem uninstall --all` omite cualquier binario fuera del `HOME` de la ejecución, también el que se está ejecutando, e indica cómo borrarlo a mano.
Excluido: el resto del alcance de `--all`; el binario del proyecto; explicar el incidente de las 16:31, que no se puede demostrar.

## ¿Qué pasa si no lo hacemos?
Cualquier ejecución con `HOME` aislado, como una revisión o una prueba, puede borrar el binario real y dejar sin servicio a todos los agentes.

## Alternativa más barata
Documentar que hay que aislar también `PATH`: no cuesta código, pero el fallo depende de recordarlo. A cambio, quien instaló en `/usr/local/bin` deberá borrar el binario a mano.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/primary/cli/ -run 'TestPlanBinaries_' -count=1`
- [x] 2. Comprobar con el binario real que las copias fuera de HOME se conservan y la de dentro se borra (dep: 1) → guion `repro11b` con HOME y PATH temporales

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-31 22:50 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestPlanBinaries_' -count=1` (salida 1)
      --- FAIL: TestPlanBinaries_SoloDentroDeHome (0.00s)
          uninstall_binary_scope_test.go:19: un binario fuera de HOME debe conservarse
      FAIL
      FAIL	mem/adapters/primary/cli	0.397s
      FAIL
- 2026-10-31 22:55 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestPlanBinaries_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.360s
- 2026-10-31 22:55 · tarea 2 · verde · vigente · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad/repro11b.sh /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad` (salida 0)
        ℹ️  R/ejecutado/mem está fuera de HOME y se conserva: bórralo a mano si quieres retirarl
      salida=0 PATH-fuera=       1 ejecutado-fuera=       1 dentro-de-HOME=       0
      ejecutando desde fuera: enruta=       1

## Siguiente
Cerrada con aprobación sobre la revisión 46283d4c4f1e.
