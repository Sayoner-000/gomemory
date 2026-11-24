# Las tareas HCD caducan durante la implementación

ID: HCD-064
Tipo: fix
Carril: fast
Motivo: fix con diagnóstico verificado en el código (huella con HEAD y --stale con un solo comando); una decisión de gate se consulta antes de implementarla

## Intención
Que editar o commitear durante la implementación no obligue a reverificar las tareas ya hechas: huella por contenido del árbol, --stale que reejecuta el comando propio de cada tarea y gate que no desvía a verify mientras queden tareas pendientes; la frescura se sigue exigiendo al cerrar

## Reproducción
`Workspace.Fingerprint` (`adapters/secondary/hcdfiles/workspace.go`) combina HEAD con el contenido de los archivos cambiados y sin rastrear: un commit sin cambiar contenido produce otra huella y caduca toda la evidencia, y editar cualquier archivo para la tarea k caduca las tareas 1..k-1. `HCD.Verify --stale` (`application/usecases/hcd.go`) exige un único comando para todas las tareas caducadas en lugar de reejecutar el de cada una. Datos de pi-gomemory: HCD-062 acumula 50 evidencias para 15 tareas y HCD-076, 55 para 22; HCD-063 tuvo que revalidar sus 6 tareas dos veces tras editar documentación.

## Alcance
Incluido: huella calculada sobre el contenido del árbol de trabajo (ids de blob de git por ruta, sin HEAD), conservando la exclusión de `hcd/` y el caso `nogit`; `mem hcd verify --stale` sin comando, que reejecuta el último comando verde de cada tarea caducada (agrupando las que comparten comando) y registra una evidencia por comando; con `--stale` y comando explícito se conserva el comportamiento actual; test que fije que con tareas pendientes y caducadas el paso es implement y la siguiente acción no pide reverificar; según Q1 (A), mientras queden pendientes una tarea caducada cuenta como progreso (fuera de `Unverified` y sin hallazgo en `analyze`), y sin pendientes el motor pide `verify --stale` y el cierre exige frescura; actualizar la skill `hcd-verify` embebida y la ayuda de `mem hcd verify`.
Excluido: relajar la frescura en el cierre (la evidencia final debe corresponder al código final); cambiar `WorkRevision` (editar el texto del checklist sigue caducando la evidencia); publicar la release sin aprobación; los cambios visuales de pi-gomemory (`extensions/gomemory-tasks.ts`), que se hacen en su repositorio.

## ¿Qué pasa si no lo hacemos?
Cada edición o commit intermedio marca como caducadas (↻) tareas ya demostradas, el motor desvía a verify y el agente reejecuta comprobaciones una y otra vez: más tiempo, más tokens y paneles en alerta que no indican ningún fallo real.

## Alternativa más barata
Solo cambiar las skills para que el agente ignore los ↻ hasta el final. No corrige el desvío del motor a verify tras un commit ni evita que `--stale` exija un comando común que no prueba lo mismo que el check de cada tarea.


## Aclaraciones
- [x] Q1. ¿Una tarea stale cuenta como progreso hasta el cierre (fuera de Unverified y sin alerta mientras queden tareas pendientes), con el gate de close exigiendo frescura? → A

## Checklist
- [x] 1. Calcular la huella sobre el contenido del árbol de trabajo para que un commit sin cambios de contenido no la altere → go test ./adapters/secondary/hcdfiles -run TestFingerprint -count=1
- [x] 2. Hacer que verify --stale sin comando reejecute el último comando verde de cada tarea caducada → go test ./application/usecases -run TestHCDVerifyStale -count=1
- [x] 3. Aceptar en la CLI mem hcd verify --stale sin comando → go test ./adapters/primary/cli/hcd -run TestVerifyStaleSinComando -count=1
- [x] 4. Fijar que con tareas pendientes y caducadas el paso es implement y la siguiente acción no pide reverificar → go test ./domain -run TestHCD064 -count=1
- [x] 5. Actualizar la skill hcd-verify embebida y la ayuda de mem hcd verify → go test ./tests/contract -run 'TestSkillSourcesInSync|TestHCD064' -count=1
- [x] 6. Ejecutar la regresión de domain, usecases, hcdfiles, cli y contract → go test ./domain ./application/usecases ./adapters/secondary/hcdfiles ./adapters/primary/cli/... ./tests/contract -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-24 14:03 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/hcdfiles -run TestFingerprint -count=1` (salida 1)
      --- FAIL: TestFingerprintIgnoraCommitsSinCambioDeContenido (0.07s)
          workspace_test.go:91: un commit sin cambiar contenido cambió la huella: a713915a8792+dbdafb7ab7f5 → a113b815630f+e3b0c44298fc
      FAIL
      FAIL	mem/adapters/secondary/hcdfiles	0.183s
      FAIL
- 2026-11-24 14:04 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/hcdfiles -run TestFingerprint -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	0.281s
- 2026-11-24 14:04 -05:00 · tarea 2 · rojo observado · caducada · `go test ./application/usecases -run TestHCDVerifyStale -count=1` (salida 1)
      --- FAIL: TestHCDVerifyStaleSinComandoReejecutaElDeCadaTarea (0.22s)
          hcd_test.go:419: verify --stale sin comando: validation error: usa un comando o --manual, no ambos
      FAIL
      FAIL	mem/application/usecases	0.669s
      FAIL
- 2026-11-24 14:05 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run TestHCDVerifyStale -count=1` (salida 0)
      ok  	mem/application/usecases	0.430s
- 2026-11-24 14:05 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli/hcd -run TestVerifyStaleSinComando -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.184s
- 2026-11-24 14:39 -05:00 · tarea 4 · rojo observado · caducada · `go test ./domain -run TestHCD064 -count=1` (salida 1)
      --- FAIL: TestHCD064StaleCuentaComoProgresoConPendientes (0.00s)
          hcd064_stale_progress_test.go:28: Unverified() = [1 2], want [2]: la caducada cuenta como progreso
      FAIL
      FAIL	mem/domain	0.006s
      FAIL
- 2026-11-24 14:39 -05:00 · tarea 4 · verde · caducada · `go test ./domain -run "TestHCD064|Stale|Evidence|Stage" -count=1` (salida 0)
      ok  	mem/domain	0.011s
- 2026-11-24 14:39 -05:00 · tarea 5 · rojo observado · caducada · `go test ./tests/contract -run 'TestSkillSourcesInSync|TestHCD064' -count=1` (salida 1)
      --- FAIL: TestHCD064StaleSinComandoDocumentado (0.00s)
          hcd064_stale_docs_test.go:32: ../../infrastructure/templates/hcd/hcd-verify/SKILL.md no contiene "`mem hcd verify --stale --json --client <agente>`"
          hcd064_stale_docs_test.go:32: ../../infrastructure/templates/hcd/hcd-verify/SKILL.md no contiene "último comando que pasó"
          hcd064_stale_docs_test.go:32: ../../infrastructure/templates/hcd/hcd-verify/SKILL.md no contiene "cuentan como progreso"
          hcd064_stale_docs_test.go:32: ../../infrastructure/templates/hcd/hcd-implement/SKILL.md no contiene "`mem hcd verify --stale --json --client <agente>`"
          hcd064_stale_docs_test.go:32: ../../adapters/primary/cli/hcd/cmd_hcd.go no contiene "verify  [unidad] --stale [-- 'comando']"
      FAIL
      FAIL	mem/tests/contract	0.016s
      FAIL
- 2026-11-24 14:40 -05:00 · tarea 5 · verde · caducada · `go test ./tests/contract -run 'TestSkillSourcesInSync|TestHCD064' -count=1` (salida 0)
      ok  	mem/tests/contract	0.019s
- 2026-11-24 14:40 -05:00 · tarea 5 · verde · caducada · `go test ./tests/contract -run 'TestSkillSourcesInSync|TestHCD064' -count=1` (salida 0)
      ok  	mem/tests/contract	0.014s
- 2026-11-24 14:41 -05:00 · tarea 6 · verde · caducada · `go test ./domain ./application/usecases ./adapters/secondary/hcdfiles ./adapters/primary/cli/... ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos'` (salida 0)
      ok  	mem/domain	0.621s
      ok  	mem/application/usecases	18.734s
      ok  	mem/adapters/secondary/hcdfiles	8.333s
      ok  	mem/adapters/primary/cli	21.212s
      ok  	mem/adapters/primary/cli/clikit	0.011s
      ok  	mem/adapters/primary/cli/hcd	2.956s
      ok  	mem/adapters/primary/cli/review	0.954s
      ok  	mem/adapters/primary/cli/socratica	4.639s
      ok  	mem/tests/contract	86.001s
- 2026-11-24 14:43 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/hcdfiles -run TestFingerprint -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	0.177s
- 2026-11-24 14:43 -05:00 · tarea 2 · verde · caducada · `go test ./application/usecases -run TestHCDVerifyStale -count=1` (salida 0)
      ok  	mem/application/usecases	1.038s
- 2026-11-24 14:43 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli/hcd -run TestVerifyStaleSinComando -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.322s
- 2026-11-24 14:43 -05:00 · tarea 4 · verde · caducada · `go test ./domain -run "TestHCD064|Stale|Evidence|Stage" -count=1` (salida 0)
      ok  	mem/domain	0.029s
- 2026-11-24 14:43 -05:00 · tarea 5 · verde · caducada · `go test ./tests/contract -run 'TestSkillSourcesInSync|TestHCD064' -count=1` (salida 0)
      ok  	mem/tests/contract	0.027s
- 2026-11-24 14:43 -05:00 · tarea 6 · verde · caducada · `go test ./domain ./application/usecases ./adapters/secondary/hcdfiles ./adapters/primary/cli/... ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos'` (salida 0)
      ok  	mem/domain	1.315s
      ok  	mem/application/usecases	23.610s
      ok  	mem/adapters/secondary/hcdfiles	5.918s
      ok  	mem/adapters/primary/cli	24.067s
      ok  	mem/adapters/primary/cli/clikit	0.016s
      ok  	mem/adapters/primary/cli/hcd	3.878s
      ok  	mem/adapters/primary/cli/review	1.187s
      ok  	mem/adapters/primary/cli/socratica	6.166s
      ok  	mem/tests/contract	88.972s
- 2026-11-24 14:45 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/secondary/hcdfiles -run TestFingerprint -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	0.145s
- 2026-11-24 14:45 -05:00 · tarea 2 · verde · vigente · `go test ./application/usecases -run TestHCDVerifyStale -count=1` (salida 0)
      ok  	mem/application/usecases	0.451s
- 2026-11-24 14:45 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli/hcd -run TestVerifyStaleSinComando -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.144s
- 2026-11-24 14:45 -05:00 · tarea 4 · verde · vigente · `go test ./domain -run "TestHCD064|Stale|Evidence|Stage" -count=1` (salida 0)
      ok  	mem/domain	0.014s
- 2026-11-24 14:45 -05:00 · tarea 5 · verde · vigente · `go test ./tests/contract -run 'TestSkillSourcesInSync|TestHCD064' -count=1` (salida 0)
      ok  	mem/tests/contract	0.020s
- 2026-11-24 14:45 -05:00 · tarea 6 · verde · vigente · `go test ./domain ./application/usecases ./adapters/secondary/hcdfiles ./adapters/primary/cli/... ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos'` (salida 0)
      ok  	mem/domain	1.715s
      ok  	mem/application/usecases	36.111s
      ok  	mem/adapters/secondary/hcdfiles	7.135s
      ok  	mem/adapters/primary/cli	34.305s
      ok  	mem/adapters/primary/cli/clikit	0.026s
      ok  	mem/adapters/primary/cli/hcd	5.550s
      ok  	mem/adapters/primary/cli/review	1.540s
      ok  	mem/adapters/primary/cli/socratica	9.193s
      ok  	mem/tests/contract	102.872s

## Siguiente
Cerrada con aprobación sobre la revisión ce94d8d70e71.
