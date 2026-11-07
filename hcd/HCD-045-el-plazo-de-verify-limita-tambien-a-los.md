# El plazo de verify limita también a los descendientes

ID: HCD-045
Tipo: fix
Carril: fast
Motivo: Fix de disponibilidad con reproducción clara y precedente en el repo (`adapters/primary/cli/install_process_unix.go` y `adapters/primary/cli/install_process_windows.go`); sin decisiones abiertas (auditoría 2026-11-06 H-02)

## Intención
Que `mem hcd verify` vuelva dentro del plazo más un margen fijo aunque la comprobación deje procesos hijos con la salida heredada, y que esos hijos terminen.

## Reproducción
`Workspace.Run` (`adapters/secondary/hcdfiles/workspace.go:152`) usa `exec.CommandContext`: al vencer el plazo mata solo el shell, y `cmd.Run` sigue esperando los pipes que conserva un hijo. No configura `WaitDelay` ni la cancelación del árbol de procesos.
Programa de la auditoría: `docs/audits/evidence/2026-11-06-project/timeout.go.txt` con plazo de 100 ms y `sleep 2 & wait` → `elapsed=2.011s exit=-1`.

## Alcance
Incluido: en `Workspace.Run`, ejecutar la comprobación en su propio grupo de procesos (Unix: `Setpgid` y `SIGKILL` al grupo; Windows: `taskkill /T /F`), como hace el instalador, y fijar `WaitDelay` (1 s) como margen máximo de espera de los pipes después de cancelar. Se conserva `exit=-1` con el aviso «tiempo límite agotado».
Excluido: cambiar el plazo por defecto (10 min); Job Objects en Windows; el contenido del extracto (HCD-043).

## ¿Qué pasa si no lo hacemos?
Una comprobación que arranca un servidor, un watcher o cualquier proceso en segundo plano deja `mem hcd verify` colgado hasta que ese hijo termine, sin límite real, y el hijo sigue corriendo después del plazo.

## Alternativa más barata
Solo `cmd.WaitDelay`: corta la espera, pero deja vivos a los descendientes, que pueden seguir escribiendo en el repositorio después del plazo. La auditoría lo señala («WaitDelay por sí solo no demuestra que los hijos hayan terminado»). Se descarta como cierre completo; se usa como red de seguridad.

## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla: con plazo de 100 ms, `Run` vuelve en menos de 1,5 s con `exit=-1` y el hijo en segundo plano no llega a escribir su canario → `go test ./adapters/secondary/hcdfiles/ -run TestRunTerminaLosDescendientesAlVencerElPlazo -count=1`
- [x] 2. El programa de la auditoría vuelve en milisegundos, no en segundos → `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/timeout.go.txt $d/main.go && go run $d/main.go | grep -Eq 'elapsed=[0-9]+ms exit=-1'`
- [x] 3. La variante de Windows compila y pasa vet → `GOOS=windows go build ./... && GOOS=windows go vet ./adapters/secondary/hcdfiles/`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 07:31 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/hcdfiles/ -run TestRunTerminaLosDescendientesAlVencerElPlazo -count=1` (salida 1)
      --- FAIL: TestRunTerminaLosDescendientesAlVencerElPlazo (2.02s)
          workspace_test.go:156: Run tardó 2.0188195s con un plazo de 100ms
      FAIL
      FAIL	mem/adapters/secondary/hcdfiles	2.131s
      FAIL
- 2026-11-06 07:31 -05:00 · tarea 2 · rojo observado · caducada · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/timeout.go.txt $d/main.go && go run $d/main.go | grep -Eq 'elapsed=[0-9]+ms exit=-1'` (salida 1)
- 2026-11-06 07:49 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/secondary/hcdfiles/ -run TestRunTerminaLosDescendientesAlVencerElPlazo -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	2.821s
- 2026-11-06 07:49 -05:00 · tarea 2 · verde · vigente · `d=$(mktemp -d) && cp docs/audits/evidence/2026-11-06-project/timeout.go.txt $d/main.go && go run $d/main.go | grep -Eq 'elapsed=[0-9]+ms exit=-1'` (salida 0)
- 2026-11-06 07:49 -05:00 · tarea 3 · verde · vigente · `GOOS=windows go build ./... && GOOS=windows go vet ./adapters/secondary/hcdfiles/` (salida 0)

## Siguiente
Cerrada con aprobación sobre la revisión ec2b04ede42b.
