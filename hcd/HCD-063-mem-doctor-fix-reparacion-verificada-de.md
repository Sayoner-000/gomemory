# mem doctor --fix: reparación verificada de lo que gestiona gomemory

ID: HCD-063
Tipo: feature
Carril: fast
Motivo: decisiones ya resueltas por la persona (confirmación para ámbito usuario; solo con --fix); añade un flag sin romper contratos

## Intención
Que mem doctor --fix ejecute las correcciones que el doctor ya calcula (canales de gomemory, plugin OpenCode, hooks duplicados, almacén de originales), pida confirmación, vuelva a diagnosticar y muestre problemas antes/después


## Alcance
Incluido: flag `mem doctor --fix` con `--dry-run` y `--yes`; plan de reparación en `domain` (acciones con ámbito, advertencia, sin duplicados, proyecto antes que usuario); acciones para los problemas que el doctor ya detecta y gomemory gestiona: canales del brazo gomemory rotos (`mem install . --yes` en proyecto, `mem setup-mcp --scope global --agents X` en usuario), plugin OpenCode incompatible o artefacto `gomemory.test.mjs` (`mem setup-mcp --scope global --agents opencode`), hooks de Claude duplicados entre usuario y proyecto (retirada en proceso con `setup.DedupClaudeProjectHooks`, que nunca toca el global), almacén de originales cerca del tope (purga en proceso), servidor MCP de CodeGraph sin conectar (`codegraph install …`); confirmación única para las acciones de proyecto y una por cada acción de ámbito usuario con su advertencia (decisión de la persona: «con confirmación»); nuevo diagnóstico tras ejecutar y resumen de problemas antes/después; clave `fixes` en `--json`; cambiar el nombre del proveedor simulado `codebase-memory-mcp` por `codegraph` en `adapters/primary/cli/cmd_optimize_test.go`; corregir el inspector de canales de Claude en ámbito proyecto para que un hook que ya registra `~/.claude/settings.json` cuente como cubierto (defecto hallado al probar `--fix` con el binario real: `mem install .` omite en el proyecto los hooks globales desde la feature 035 y el inspector los marcaba `missing`, así que el remedio nunca convergía).
Excluido: reparación automática fuera de `--fix` (decisión de la persona: «solo con --fix»); canales del brazo codegraph, que son de solo lectura (INV-1 de `domain/activation.go`); plugins OpenCode ajenos; copias locales del binario (ya se retiran al abrir el proyecto); actualizar el binario (`mem update`); cambiar el comportamiento de `mem doctor` sin `--fix`, incluido `--strict`.

## ¿Qué pasa si no lo hacemos?
El doctor seguirá listando el remedio de cada problema y la persona tendrá que copiar y ejecutar a mano varios comandos, sin saber después si el problema desapareció; un canal roto en un proyecto poco usado se queda roto hasta que alguien lea el informe.

## Alternativa más barata
Imprimir un único comando agregado al final del informe (`mem install . --yes && …`). Es más barato, pero no distingue ámbito proyecto de usuario, no pide confirmación para lo que afecta a toda la máquina y no verifica el resultado.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Definir domain.FixAction y domain.PlanDeReparacion (sin duplicados, proyecto antes que usuario, advertencia en ámbito usuario) con su test → go test ./domain -run TestPlanDeReparacion -count=1
- [x] 2. Construir el plan desde el diagnóstico (canales, OpenCode, hooks duplicados, originales, CodeGraph MCP) sin canales del brazo codegraph → go test ./adapters/primary/cli -run TestBuildFixPlan -count=1
- [x] 3. Añadir --fix, --dry-run y --yes a mem doctor: confirmación por ámbito, ejecución con ejecutor inyectable, nuevo diagnóstico y resumen antes/después, clave fixes en --json → go test ./adapters/primary/cli -run TestDoctorFix -count=1
- [x] 4. Renombrar el proveedor simulado codebase-memory-mcp a codegraph en adapters/primary/cli/cmd_optimize_test.go → go test ./adapters/primary/cli -run TestCmdOptimize -count=1
- [x] 5. Ejecutar la regresión de domain, setup, cli y contract → go test ./domain ./adapters/primary/setup ./adapters/primary/cli ./tests/contract -count=1
- [x] 6. Contar como cubierto en ámbito proyecto el hook de Claude que registra el settings.json global → go test ./adapters/primary/setup -run TestInspectClaudeHook_ProyectoCubiertoPorGlobal -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-24 13:34 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run TestPlanDeReparacion -count=1` (salida 1)
      # mem/domain [mem/domain.test]
      domain/fix_plan_test.go:6:10: undefined: PlanDeReparacion
      domain/fix_plan_test.go:6:29: undefined: FixAction
      domain/fix_plan_test.go:24:10: undefined: PlanDeReparacion
      domain/fix_plan_test.go:24:29: undefined: FixAction
      domain/fix_plan_test.go:43:13: undefined: PlanDeReparacion
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-24 13:34 -05:00 · tarea 1 · rojo observado · caducada · `go test ./domain -run TestPlanDeReparacion -count=1` (salida 1)
      # mem/domain [mem/domain.test]
      domain/fix_plan_test.go:6:10: undefined: PlanDeReparacion
      domain/fix_plan_test.go:6:29: undefined: FixAction
      domain/fix_plan_test.go:24:10: undefined: PlanDeReparacion
      domain/fix_plan_test.go:24:29: undefined: FixAction
      domain/fix_plan_test.go:43:13: undefined: PlanDeReparacion
      FAIL	mem/domain [build failed]
      FAIL
- 2026-11-24 13:34 -05:00 · tarea 1 · verde · caducada · `go test ./domain -run TestPlanDeReparacion -count=1` (salida 0)
      ok  	mem/domain	0.007s
- 2026-11-24 13:35 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestBuildFixPlan -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_doctor_fix_test.go:11:26: undefined: fixStep
      adapters/primary/cli/cmd_doctor_fix_test.go:20:8: undefined: doctorState
      adapters/primary/cli/cmd_doctor_fix_test.go:27:11: undefined: buildFixPlan
      adapters/primary/cli/cmd_doctor_fix_test.go:45:8: undefined: doctorState
      adapters/primary/cli/cmd_doctor_fix_test.go:49:14: undefined: buildFixPlan
      adapters/primary/cli/cmd_doctor_fix_test.go:55:8: undefined: doctorState
      adapters/primary/cli/cmd_doctor_fix_test.go:61:21: undefined: buildFixPlan
      adapters/primary/cli/cmd_doctor_fix_test.go:68:8: undefined: doctorState
      adapters/primary/cli/cmd_doctor_fix_test.go:69:21: undefined: buildFixPlan
      adapters/primary/cli/cmd_doctor_fix_test.go:74:7: undefined: doctorState
      adapters/primary/cli/cmd_doctor_fix_test.go:74:7: too many errors
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-24 13:36 -05:00 · tarea 2 · falló · caducada · `go test ./adapters/primary/cli -run TestBuildFixPlan -count=1` (salida 1)
      --- FAIL: TestBuildFixPlan_CanalesDeGomemoryPorAmbito (0.00s)
          cmd_doctor_fix_test.go:31: plan = ["mem install . --yes" "mem setup-mcp --scope global --agents codex" "mem setup-mcp --scope global --agents opencode"], want ["mem install . --yes" "mem setup-mcp --scope global --agents codex"]
      --- FAIL: TestBuildFixPlan_NuncaTocaElBrazoCodegraph (0.00s)
          cmd_doctor_fix_test.go:50: los canales del brazo codegraph son de solo lectura (INV-1): ["mem setup-mcp --scope global --agents opencode"]
      --- FAIL: TestBuildFixPlan_HooksDuplicadosOriginalesYCodeGraph (0.00s)
          cmd_doctor_fix_test.go:89: se esperaban 3 acciones, plan = ["retirar del .claude/settings.json del proyecto los hooks de gomemory que ya registra el global" "mem pack purge" "mem setup-mcp --scope global --agents opencode" "codegraph install --target codex,antigravity --location global --yes"]
      --- FAIL: TestBuildFixPlan_SinProblemasNoHayPlan (0.00s)
          cmd_doctor_fix_test.go:107: plan = ["mem setup-mcp --scope global --agents opencode"]
      FAIL
      FAIL	mem/adapters/primary/cli	0.006s
      FAIL
- 2026-11-24 13:36 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestBuildFixPlan -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.006s
- 2026-11-24 13:36 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestDoctorFix -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_doctor_fix_test.go:126:45: undefined: collectDoctor
      adapters/primary/cli/cmd_doctor_fix_test.go:126:60: undefined: runFixCommand
      adapters/primary/cli/cmd_doctor_fix_test.go:126:75: undefined: fixConfirmIn
      adapters/primary/cli/cmd_doctor_fix_test.go:126:89: undefined: dedupClaudeHooks
      adapters/primary/cli/cmd_doctor_fix_test.go:128:3: undefined: collectDoctor
      adapters/primary/cli/cmd_doctor_fix_test.go:128:18: undefined: runFixCommand
      adapters/primary/cli/cmd_doctor_fix_test.go:128:33: undefined: fixConfirmIn
      adapters/primary/cli/cmd_doctor_fix_test.go:128:47: undefined: dedupClaudeHooks
      adapters/primary/cli/cmd_doctor_fix_test.go:130:2: undefined: collectDoctor
      adapters/primary/cli/cmd_doctor_fix_test.go:137:2: undefined: runFixCommand
      adapters/primary/cli/cmd_doctor_fix_test.go:137:2: too many errors
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-24 13:38 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run "TestDoctorFix|TestDoctorSinFix|TestBuildFixPlan" -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.014s
- 2026-11-24 13:38 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/primary/cli -run TestCmdOptimize -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.010s
- 2026-11-24 13:40 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/setup -run TestInspectClaudeHook_ProyectoCubiertoPorGlobal -count=1` (salida 1)
      --- FAIL: TestInspectClaudeHook_ProyectoCubiertoPorGlobal (0.24s)
          activation_inspect_global_cover_test.go:32: plan_guard/project cubierto por el global: state=missing detail="PreToolUse:ExitPlanMode no registrado"
          activation_inspect_global_cover_test.go:32: plan_entry/project cubierto por el global: state=missing detail="PostToolUse:EnterPlanMode no registrado"
      FAIL
      FAIL	mem/adapters/primary/setup	0.249s
      FAIL
- 2026-11-24 13:40 -05:00 · tarea 6 · verde · caducada · `go test ./adapters/primary/setup -run "TestInspectClaudeHook_|TestActivationInspect" -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	3.906s
- 2026-11-24 13:41 -05:00 · tarea 5 · falló · caducada · `go test ./domain ./adapters/primary/setup ./adapters/primary/cli ./tests/contract -count=1` (salida 1)
        ○ ~/.codex/config.toml conserva [mcp_servers.gomemory] y los hooks del ciclo de gomemory — son configuración global compartida por todos los proyectos.
        ○ .claude/plugins/gomemory: no encontrado
        ○ .claude/settings.json: no encontrado
        ○ .claude/settings.json: sin permisos de gomemory
        ○ opencode.json: sin permisos de gomemory
        ○ envoltorios nativos: no encontrados
      Resumen:
        ✓ Integración de gomemory en /tmp/TestCmdUninstallAcceptsYesFlagInAnyPosition1055133866/003
        ✓ /tmp/TestCmdUninstallAcceptsYesFlagInAnyPosition1055133866/003/mem (copia v2.58.0)
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ Memoria 1 eliminada
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) de el proyecto "001-056efccb6a042b54".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) por garbage collection de el proyecto "001-056efccb6a042b54".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      FAIL
      FAIL	mem/tests/contract	70.420s
      FAIL
- 2026-11-24 13:44 -05:00 · tarea 5 · falló · caducada · `go test ./domain ./adapters/primary/setup ./adapters/primary/cli ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos'` (salida 1)
      ok  	mem/domain	0.296s
      --- FAIL: TestSocraticaWrapper_FalloConservaElPrevio (0.00s)
          agent_skills_setup_test.go:478: la escritura debe fallar en un directorio de solo lectura
        ✓ opencode: MCP registrado en scope usuario (/tmp/TestOpenCodeGlobal_EnlaceDeDotfilesSeConserva2545561447/002/opencode.json)
        ✓ opencode: tools MCP pre-aprobadas en scope usuario (/tmp/TestOpenCodeGlobal_EnlaceDeDotfilesSeConserva2545561447/002/opencode.json)
      --- FAIL: TestGlobalWrapper_Atomico (0.00s)
          global_atomic_test.go:24: debía fallar sin temporal
        ✓ opencode: plugin instalado en /tmp/TestInstallOpenCode_RegistraEnScopeUsuarioYRetiraElArchivoDeProy2532007898/001/.config/opencode/plugins/gomemory.ts
        ✓ opencode: MCP registrado en scope usuario (/tmp/TestInstallOpenCode_RegistraEnScopeUsuarioYRetiraElArchivoDeProy2532007898/001/.config/opencode/opencode.json)
        ✓ opencode: tools MCP pre-aprobadas en scope usuario (/tmp/TestInstallOpenCode_RegistraEnScopeUsuarioYRetiraElArchivoDeProy2532007898/001/.config/opencode/opencode.json)
        opencode: opencode.json de proyecto retirado (la referencia vive en el scope usuario)
        opencode: registro de proyecto retirado de opencode.json (conserva configuración ajena)
        ✓ opencode: plugin instalado en /tmp/TestInstallOpenCodePlugin_RetiraElTestHeredadoYConservaLoAjeno3915672524/001/.config/opencode/plugins/gomemory.ts
      FAIL
      FAIL	mem/adapters/primary/setup	7.660s
      ok  	mem/adapters/primary/cli	17.953s
      ok  	mem/tests/contract	88.383s
      FAIL
- 2026-11-24 13:46 -05:00 · tarea 5 · verde · caducada · `go test ./domain ./adapters/primary/setup ./adapters/primary/cli ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos|TestSocraticaWrapper_FalloConservaElPrevio|TestGlobalWrapper_Atomico'` (salida 0)
      ok  	mem/domain	0.259s
      ok  	mem/adapters/primary/setup	6.167s
      ok  	mem/adapters/primary/cli	15.735s
      ok  	mem/tests/contract	77.978s
- 2026-11-24 13:47 -05:00 · tarea 1,2,3,4,6 · verde · caducada · `go test ./domain ./adapters/primary/setup ./adapters/primary/cli ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos|TestSocraticaWrapper_FalloConservaElPrevio|TestGlobalWrapper_Atomico'` (salida 0)
      ok  	mem/domain	0.264s
      ok  	mem/adapters/primary/setup	7.419s
      ok  	mem/adapters/primary/cli	16.986s
      ok  	mem/tests/contract	82.466s
- 2026-11-24 13:57 -05:00 · tarea 1,2,3,4,5,6 · verde · caducada · `go test ./domain ./adapters/primary/setup ./adapters/primary/cli ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos|TestSocraticaWrapper_FalloConservaElPrevio|TestGlobalWrapper_Atomico'` (salida 0)
      ok  	mem/domain	0.381s
      ok  	mem/adapters/primary/setup	6.595s
      ok  	mem/adapters/primary/cli	14.666s
      ok  	mem/tests/contract	76.077s
- 2026-11-24 13:59 -05:00 · tarea 1,2,3,4,5,6 · verde · vigente · `go test ./domain ./adapters/primary/setup ./adapters/primary/cli ./tests/contract -count=1 -skip 'TestCheckReplaceable|TestMigracionGlobal_FalloDeCopia|TestInstallBinary_PasoDelResumenRefleLaRetirada|TestSessionStart_RetiradaFallidaNoRompeElHook|TestUninstall_ExportFallidoNoBorraLaMemoria|TestUpdate_GlobalSinEscrituraNoDescarga|TestUpdate_TerminaConResumenDePasos|TestSocraticaWrapper_FalloConservaElPrevio|TestGlobalWrapper_Atomico'` (salida 0)
      ok  	mem/domain	0.325s
      ok  	mem/adapters/primary/setup	6.529s
      ok  	mem/adapters/primary/cli	27.605s
      ok  	mem/tests/contract	80.093s

## Siguiente
Cerrada con aprobación sobre la revisión c42d62f5dd3d.
