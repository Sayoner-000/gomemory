# Cerrar hallazgos ADM-01..05 de la auditoría v2.43.1

ID: HCD-017
Tipo: fix
Carril: fast
Motivo: Cuatro correcciones con reproducción determinista y cierre propuesto en el informe; sin decisiones abiertas interdependientes

## Intención
Corregir ADM-01 (copia viva con forma título: contenido), ADM-02/03 (TopicKey explícito en decisiones de plan), ADM-04 (uninstall reporta y sale !=0 si rm falla) y ADM-05 (DeletedCount real en consolidación) según docs/audits/2026-11-02-acr-adm-v2.43.1.md

## Reproducción
Informe `docs/audits/2026-11-02-acr-adm-v2.43.1.md`, §4:
- ADM-01: purgar la memoria «T»/«abc» con una memoria viva cuyo contenido es «T: abc» retira el original «T: abc» (`TestReproCopiaVivaFormaCorta` del informe falla).
- ADM-02/03: dos decisiones «Plan aprobado: Plan» con contenidos distintos comparten id; la segunda reescribe la primera. Con el dedup desactivado, un reenvío exacto duplica la fila.
- ADM-04: con el directorio del binario sin permiso de escritura, `install.sh --uninstall` imprime «No se encontró el binario» y sale con 0.
- ADM-05: si `applyGroup` falla a mitad, `DeletedCount` sigue contando todos los grupos.

## Alcance
Incluido: ADM-01 en `maintenance.go` (cada candidata lleva la forma que produjo su hash y se comprueba esa forma; para la forma con título se sigue aceptando también el contenido plano vivo, para no retirar lo que el caso existente «corta viva» conserva) y el caso «corta viva con la forma»; ADM-02/03 con `TopicKey` `plan:` + 16 hex de sha256 del plan en `hookPlanApproved` y pruebas de coexistencia y de reenvío idempotente; ADM-04 en `install.sh` con estado de `rm` capturado, aviso y salida 1, y caso nuevo en `scripts/test-install-uninstall.sh`; ADM-05 con `DeletedCount` igual a lo que devuelve `DeleteMany`.
Excluido: la prueba del plugin de OpenCode para el reenvío de `plan-approved` (el reenvío sigue ocurriendo; la identidad por `topic_key` lo vuelve idempotente en Go); migrar decisiones de plan ya guardadas; el veredicto ACR formal con dos revisores; publicar una versión.

## ¿Qué pasa si no lo hacemos?
Se rompe la garantía publicada de copias vivas (refs de pack rotas), se pierden en silencio planes aprobados que empiezan igual, un fallo de desinstalación pasa por éxito y la TUI muestra una cifra falsa de filas fundidas.

## Alternativa más barata
Documentar las limitaciones en `CHANGELOG.md` y `docs/DATABASE.md`. No evita la pérdida de planes ni el código de salida falso, y contradice C-14/C-28 de HCD-013.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. ADM-01: la copia viva con la forma «título: contenido» se conserva → `go test ./adapters/secondary/persistence -run 'CopiaViva|PurgaCopias' -count=1`
- [x] 2. ADM-02/03: dos planes con el mismo título coexisten y un reenvío exacto no crea fila → `go test ./adapters/primary/cli ./adapters/secondary/persistence -run 'Plan' -count=1`
- [x] 3. ADM-04: `--uninstall` sale distinto de 0 y avisa si `rm` falla → `bash scripts/test-install-uninstall.sh`
- [x] 4. ADM-05: `DeletedCount` refleja lo realmente borrado → `go test ./application/usecases -run Consolidate -count=1`
- [x] 5. Verificación completa → `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash scripts/test-purge-physical.sh && bash -n scripts/install.sh`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-03 01:07 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/persistence -run 'CopiaViva|PurgaCopias' -count=1` (salida 1)
      --- FAIL: TestCopiaViva_ConservaSoloLoQueSigueVivo (0.07s)
          --- FAIL: TestCopiaViva_ConservaSoloLoQueSigueVivo/corta_viva_con_la_forma (0.01s)
              borrado_memorias_test.go:555: retirar=false y el original existe=false
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.772s
      FAIL
- 2026-11-03 01:07 · tarea 1 · verde · caducada · `go test ./adapters/secondary/persistence -run 'CopiaViva|PurgaCopias' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.442s
- 2026-11-03 01:08 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli ./adapters/secondary/persistence -run 'Plan' -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_hook_plan_test.go:79:7: undefined: planMemory
      adapters/primary/cli/cmd_hook_plan_test.go:80:7: undefined: planMemory
      adapters/primary/cli/cmd_hook_plan_test.go:90:14: undefined: planMemory
      FAIL	mem/adapters/primary/cli [build failed]
      ok  	mem/adapters/secondary/persistence	0.283s
      FAIL
- 2026-11-03 01:08 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli ./adapters/secondary/persistence -run 'Plan' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.469s
      ok  	mem/adapters/secondary/persistence	0.240s
- 2026-11-03 01:09 · tarea 3 · rojo observado · caducada · `bash scripts/test-install-uninstall.sh` (salida 1)
      FALLO: un rm fallido sale con 0
- 2026-11-03 01:09 · tarea 3 · verde · caducada · `bash scripts/test-install-uninstall.sh` (salida 0)
      OK: --uninstall solo retira binarios propios
- 2026-11-03 01:10 · tarea 4 · rojo observado · caducada · `go test ./application/usecases -run Consolidate -count=1` (salida 1)
      --- FAIL: TestConsolidate_FalloParcialInformaLoBorrado (0.01s)
          consolidate_memories_test.go:283: DeletedCount = 2, borradas de verdad = 1 (quiero 1)
      FAIL
      FAIL	mem/application/usecases	1.272s
      FAIL
- 2026-11-03 01:10 · tarea 4 · verde · vigente · `go test ./application/usecases -run Consolidate -count=1` (salida 0)
      ok  	mem/application/usecases	0.781s
- 2026-11-03 01:10 · tarea 5 · falló · vigente · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash scripts/test-purge-physical.sh && bash -n scripts/install.sh` (salida 1)
        ℹ️  .claude/settings.json: no encontrado
        ℹ️  .claude/settings.json: sin permisos de gomemory
        ℹ️  opencode.json: sin permisos de gomemory
        ℹ️  envoltorios nativos: no encontrados
      Resumen:
        ✓ Integración de gomemory en /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition239375867/003
        ✓ /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition239375867/003/mem (copia v2.44.1)
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ Memoria 1 eliminada
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✅ 1 memoria(s) eliminada(s) de el proyecto "001-71c36aed4f39d407".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✅ 1 memoria(s) eliminada(s) por garbage collection de el proyecto "001-71c36aed4f39d407".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      FAIL
      FAIL	mem/tests/contract	121.566s
      ok  	mem/tests/integration	36.425s
      ?   	mem/version	[no test files]
      FAIL
- 2026-11-03 01:16 · tarea 1,2,3,4,5 · verde · vigente · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash scripts/test-purge-physical.sh && bash -n scripts/install.sh` (salida 0)
      ok  	mem/adapters/secondary/generator	2.336s
      ok  	mem/adapters/secondary/hcdfiles	1.407s
      ok  	mem/adapters/secondary/launcher	1.769s
      ok  	mem/adapters/secondary/persistence	20.963s
      ok  	mem/adapters/secondary/release	0.616s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.417s
      ok  	mem/adapters/secondary/speckit	0.418s
      ok  	mem/adapters/secondary/tokens	0.390s
      ok  	mem/adapters/secondary/usage	0.354s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	7.656s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.673s
      ok  	mem/infrastructure	0.535s
      ok  	mem/tests/contract	124.517s
      ok  	mem/tests/integration	39.280s
      ?   	mem/version	[no test files]
      OK: --uninstall solo retira binarios propios
      OK: el texto purgado no está en disco y la superviviente se encuentra

## Siguiente
Cerrada con aprobación sobre la revisión 8d5c66e5ff67.
