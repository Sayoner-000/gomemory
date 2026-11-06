# CI Windows: archivos en uso por procesos en segundo plano

ID: HCD-039
Tipo: fix
Carril: fast
Motivo: fix con fallo observado en 3 runs; causa por confirmar con los logs, sin decisiones interdependientes

## Intención
Que retirar una copia local de `mem.exe` en Windows no falle por un bloqueo transitorio del ejecutable recién ejecutado, y que el job Windows del CI deje de fallar al azar con «being used by another process» sin tener que relanzarlo.

Causa (memoria 1857, que corrige la hipótesis de la 1848): `retireLocalCopy` (`adapters/primary/cli/local_copy.go`) ejecuta la copia con `identifyVersion` (`<copia> version`) y la borra al instante con `os.Remove` en `removeBinaryFile` (`adapters/primary/cli/uninstall_plan.go`). En Windows un exe que acaba de terminar puede seguir bloqueado un instante (cierre del proceso, escaneo de Defender), y `os.Remove` no reintenta los errores transitorios 5/32/33, a diferencia de `atomicfs.Replace` y `atomicfs.ReadFile`. `spawnUpdateCheck` no interviene: la copia solo se lanza con `version`.

## Reproducción
En el CI real (job «Windows (Go y binario del release)»), de forma intermitente:
- Run 42154582679, intento 1, commit 78ea15f: `TestInstall_ResumenIncluyeLaCopiaRetirada` → `⚠ Binario: no se pudo retirar …\retirada\mem.exe: remove …: The process cannot access the file because it is being used by another process`.
- Run 42105846103: `TestCicloDeVida_Idempotente` → «la segunda instalación no tiene copia que retirar» y 177 → 176 archivos. Es el mismo síntoma: la 1.ª instalación no pudo retirar la copia y la 2.ª sí.
- Run 41754437738 (`update-check.json` en uso): corría bb6ecbbb, anterior a fbdaa9d (`tryReadAll` con `atomicfs.ReadFile`). Ya está corregido y queda fuera.

Reproducción local portable (macOS no tiene el bloqueo): `go test ./adapters/secondary/atomicfs -run TestRemove_ -count=1`. Simula un borrado que devuelve un error transitorio las primeras veces y comprueba que el borrado de `atomicfs` reintenta hasta lograrlo. Hoy falla porque `atomicfs` no tiene borrado con reintento.

## Alcance
Incluido: `atomicfs.Remove`, con el mismo reintento acotado de errores transitorios que `Replace` (en Windows; en el resto equivale a `os.Remove`); su uso en `removeBinaryFile` para la rama que hoy llama a `os.Remove`; la prueba del reintento; y la validación en el CI Windows real.
Excluido: la rama de `removeBinaryFile` que aparta el ejecutable en curso (`ping … & del`), que no cambia; `spawnUpdateCheck` y `update-check.json`, ya cerrados con fbdaa9d; otros `os.Remove` del proyecto que no han dado fallos observados.

## ¿Qué pasa si no lo hacemos?
Cada PR puede quedarse en rojo por azar en Windows y hay que relanzar con `gh run rerun --failed`, lo que oculta fallos reales. Además le pasa a la persona: el hook SessionStart y `mem install` en Windows dejan sin retirar la copia local y avisan «no se pudo retirar», de modo que conviven dos versiones hasta otra sesión.

## Alternativa más barata
Endurecer solo los dos tests (esperar o reintentar en la prueba) sale más barato, pero deja el fallo en el producto: la retirada del hook y de install seguiría fallando a usuarios de Windows. Tampoco basta con relanzar el CI, porque no corrige nada. Se elige el reintento en el producto, que es el patrón ya probado en `atomicfs`, como hace cmd/go con internal/robustio.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → re-ejecutar la reproducción
- [x] 2. removeBinaryFile borra con atomicfs.Remove y todo el proyecto compila y pasa en Windows y Unix → go vet ./... && GOOS=windows go vet ./adapters/... && go test ./adapters/primary/cli/... ./adapters/secondary/atomicfs/... -count=1
- [ ] 3. El job Windows del CI real pasa sin relanzar → manual: job «Windows (Go y binario del release)» en verde en 3 runs del PR, sin gh run rerun

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 04:53 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/atomicfs -run TestRemove_ -count=1` (salida 1)
      # mem/adapters/secondary/atomicfs [mem/adapters/secondary/atomicfs.test]
      adapters/secondary/atomicfs/remove_test.go:19:9: undefined: retry
      adapters/secondary/atomicfs/remove_test.go:35:9: undefined: retry
      adapters/secondary/atomicfs/remove_test.go:43:9: undefined: retry
      adapters/secondary/atomicfs/remove_test.go:54:12: undefined: Remove
      adapters/secondary/atomicfs/remove_test.go:60:12: undefined: Remove
      FAIL	mem/adapters/secondary/atomicfs [build failed]
      FAIL
- 2026-11-06 04:53 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/atomicfs -run TestRemove_ -count=1` (salida 0)
      ok  	mem/adapters/secondary/atomicfs	0.496s
- 2026-11-06 04:53 -05:00 · tarea 2 · verde · vigente · `go vet ./... && GOOS=windows go vet ./adapters/... && go test ./adapters/primary/cli/... ./adapters/secondary/atomicfs/... -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	19.217s
      ok  	mem/adapters/primary/cli/clikit	0.228s
      ok  	mem/adapters/primary/cli/hcd	1.920s
      ok  	mem/adapters/primary/cli/review	1.385s
      ok  	mem/adapters/primary/cli/socratica	1.609s
      ok  	mem/adapters/secondary/atomicfs	0.221s
- 2026-11-06 04:54 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/secondary/atomicfs -run TestRemove_ -count=1` (salida 0)
      ok  	mem/adapters/secondary/atomicfs	0.172s

## Siguiente
pendiente
