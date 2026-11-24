# Reducir coste y tokens de los comandos HCD y usar un solo grafo en el protocolo

ID: HCD-061
Tipo: technical
Carril: fast
Motivo: mejora técnica medible sin decisiones interdependientes; codegraph ya elegido como principal (memoria 2217)

## Intención
Que cada comando HCD que modifica una unidad calcule la huella de código una sola vez, que la vista calcule el estado de las tareas una sola vez, que verify --json devuelva solo las tareas que cambiaron, que la CLI acepte el cliente pi y que el protocolo pida codegraph como grafo principal


## Alcance
Incluido: que Clarify, Approve, Close, Cancel, Promote, Rename y RecordDefinitionReview calculen la huella de código (`Workspace.Fingerprint`: 4 procesos git y el hash de los archivos cambiados) una sola vez por comando y reutilicen la de `mutable`/`Show` inicial al volver a mostrar la unidad, porque entre medias solo escriben en `hcd/` o en la base de datos y la huella excluye `hcd/`; que `mem hcd --client pi` sea un cliente válido.
Excluido: `Verify` sigue recalculando la huella tras ejecutar el comando, porque ese comando puede cambiar el código; la respuesta parcial de `verify --json`, que contradice HCD-060 (la skill hcd-verify lee el estado completo devuelto para no repetir `show`); calcular `TaskStatuses` una sola vez en `viewOf` (trabajo en memoria de microsegundos); el protocolo de grafo único, que vive en la extensión gomemory-context de pi-gomemory y va en su propia unidad; retirar el índice de código de gomemory.

## ¿Qué pasa si no lo hacemos?
Cada comando HCD que modifica una unidad sigue lanzando el doble de procesos git y releyendo los archivos cambiados dos veces, coste que crece con el tamaño del worktree y se paga en cada paso de la ceremonia. Pi sigue sin poder identificarse: `mem hcd start --client pi` falla con «cliente inválido: pi» y obliga a registrar sus aprobaciones como `terminal`, perdiendo la procedencia.

## Alternativa más barata
Cachear la huella dentro de `hcdfiles.Workspace` por tiempo; se descarta porque una caché temporal puede servir una huella vieja tras una edición real y debilitar el gate de evidencia. Reutilizarla explícitamente solo dentro de un mismo comando que no ejecuta nada externo es más barato y no cambia la semántica.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Escribir un test que cuente las llamadas a Fingerprint por comando (1 en Clarify, Approve, Close, Cancel, Promote, Rename y RecordDefinitionReview; 2 en Verify) → go test ./application/usecases -run TestHCD061FingerprintPorComando -count=1
- [x] 2. Reutilizar la huella del Show inicial al volver a mostrar la unidad en las mutaciones sin comando externo → go test ./application/usecases -run TestHCD061FingerprintPorComando -count=1
- [x] 3. Escribir un test que acepte --client pi en mem hcd → go test ./adapters/primary/cli/hcd -run TestHCD061ClientePi -count=1
- [x] 4. Añadir pi a los clientes admitidos → go test ./adapters/primary/cli/hcd -run TestHCD061ClientePi -count=1
- [x] 5. Ejecutar la regresión de HCD en usecases, cli/hcd y domain → go test ./application/usecases ./adapters/primary/cli/hcd ./domain -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-23 19:13 -05:00 · tarea 1 · rojo observado · caducada · `go test ./application/usecases -run TestHCD061FingerprintPorComando -count=1` (salida 1)
      --- FAIL: TestHCD061FingerprintPorComando (0.81s)
          hcd061_fingerprint_test.go:61: clarify ask calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:65: clarify answer calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:69: rename calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:78: close calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:88: promote calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:98: review socratic calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:103: review splash calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:107: approve calculó la huella 2 veces, quiero 1
          hcd061_fingerprint_test.go:111: cancel calculó la huella 2 veces, quiero 1
      FAIL
      FAIL	mem/application/usecases	1.229s
      FAIL
- 2026-11-23 19:14 -05:00 · tarea 1,2 · verde · caducada · `go test ./application/usecases -run TestHCD061FingerprintPorComando -count=1` (salida 0)
      ok  	mem/application/usecases	0.799s
- 2026-11-23 19:14 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli/hcd -run TestHCD061ClientePi -count=1` (salida 1)
      --- FAIL: TestHCD061ClientePi (0.00s)
          hcd061_client_test.go:10: parse(start --client pi) = {command:start id: client:terminal run: flags:map[] json:false} cliente inválido: pi
      FAIL
      FAIL	mem/adapters/primary/cli/hcd	0.258s
      FAIL
- 2026-11-23 19:14 -05:00 · tarea 3,4 · verde · caducada · `go test ./adapters/primary/cli/hcd -run TestHCD061ClientePi -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.236s
- 2026-11-23 19:14 -05:00 · tarea 5 · verde · caducada · `go test ./application/usecases ./adapters/primary/cli/hcd ./domain -count=1` (salida 0)
      ok  	mem/application/usecases	21.704s
      ok  	mem/adapters/primary/cli/hcd	3.254s
      ok  	mem/domain	0.329s
- 2026-11-23 19:15 -05:00 · tarea 1,2 · verde · caducada · `go test ./application/usecases -run TestHCD061FingerprintPorComando -count=1` (salida 0)
      ok  	mem/application/usecases	0.821s
- 2026-11-23 19:15 -05:00 · tarea 1,2,3,4,5 · verde · vigente · `go test ./application/usecases ./adapters/primary/cli/hcd ./domain -count=1 -run "." -v 2>&1 | grep -E "^(ok|FAIL|--- FAIL)|TestHCD061" ; test ${PIPESTATUS[0]} -eq 0` (salida 0)
      === RUN   TestHCD061FingerprintPorComando
      --- PASS: TestHCD061FingerprintPorComando (0.58s)
      ok  	mem/application/usecases	16.601s
      === RUN   TestHCD061ClientePi
      --- PASS: TestHCD061ClientePi (0.00s)
      ok  	mem/adapters/primary/cli/hcd	2.918s
      ok  	mem/domain	0.253s

## Siguiente
Cerrada con aprobación sobre la revisión 825b57606870.
