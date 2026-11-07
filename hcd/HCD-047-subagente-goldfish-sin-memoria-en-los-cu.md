# Subagente goldfish sin memoria en los cuatro runtimes

ID: HCD-047
Tipo: evolution
Carril: full
Motivo: Cuatro runtimes con mecanismos distintos (agent_type en Claude, agent en OpenCode, marcador en agy, spawn_agent en Codex) e incógnitas que dependen unas de otras; cambia el contrato del hook de subagentes

## Intención
Que un subagente revisor de tipo `splash` (el «goldfish» de EGM con nombre propio de gomemory: la gota que salpica sin arrastrar memoria) arranque sin memoria del proyecto (sin aviso, sin tools de memoria y sin instrucciones globales) y lea solo el documento y el código vivo, en Claude Code, agy, Codex y OpenCode. Es el requisito de la revisión goldfish antes de aprobar (HCD-048, decisión EGM de 2026-11-06).

## Alcance
Incluido: reconocer al subagente goldfish en cada runtime y no inyectarle nada de gomemory. Claude Code: `hookSubagentStart` (`adapters/primary/cli/cmd_hook.go`) con `agent_type`. agy: `classifyAgySubagent` (`adapters/primary/cli/hook_agy.go`). OpenCode: el plugin `infrastructure/plugin/opencode/gomemory.ts`. Codex: lo que herede su `spawn_agent`. Incluye la definición del agente goldfish que instala `mem setup` en cada runtime donde exista ese concepto (en Claude, con tools solo de lectura de archivos, sin MCP y con `omitClaudeMd`), y una verificación en un harness real (Claude Code, agy, Codex u OpenCode).
Excluido: la skill de revisión y sus lentes (HCD-048); cambiar el aviso de los demás subagentes (HCD-026); los subagentes de ACR.

## ¿Qué pasa si no lo hacemos?
Un revisor goldfish recibe hoy el aviso de subagente («para consultar decisiones previas usa search_memories…», `subagentNoticeFor`) y, en Claude, el CLAUDE.md global con el protocolo de memoria. Ya no tiene contexto cero: arrastra el sesgo del proyecto y la revisión pierde la propiedad que la justifica (memoria 1712).

## Alternativa más barata
Pedirle al goldfish en el prompt que ignore la memoria. Es más barato, pero no quita ni el aviso ni las tools ni el CLAUDE.md, y depende de que el modelo obedezca; no da aislamiento verificable. Se descarta.

## Historias
### US1. El subagente splash arranca sin memoria
Como agente que lanza una revisión splash, quiero que el subagente no reciba nada de gomemory, para que lea solo el documento y el código.
- C-01: Dado un subagente de tipo `splash` en Claude Code, cuando arranca, entonces `SubagentStart` no le añade contexto, y su definición no le da tools MCP ni carga CLAUDE.md.
- C-02: Dado un subagente de cualquier otro tipo, cuando arranca, entonces sigue recibiendo el aviso mínimo de HCD-026.
- C-03: Dado un subagente `splash` en OpenCode, cuando arranca, entonces el plugin no le inyecta el aviso de subagente.
- C-04: Dado un subagente `splash` en agy o Codex, cuando arranca, entonces no recibe el aviso de gomemory, o la limitación queda documentada con la evidencia del sondeo real.
- C-05: Dado un harness real (Claude Code, agy, Codex u OpenCode), cuando se lanza un subagente `splash`, entonces su contexto no contiene «decisiones previas» ni tools de memoria (evidencia manual que nombra el harness).

## Aclaraciones
- [x] Q1. Marca común: ¿el goldfish se reconoce por un tipo de agente llamado `goldfish` en todos los runtimes que tienen tipos de agente, o por otro marcador? → Tipo de agente con nombre propio de gomemory: splash (salpicar, la gota que sale sin arrastrar memoria) en lugar de goldfish; agnóstico al runtime: Claude (agent_type), OpenCode (agent de la tool subagent), agy y Codex con su equivalente, revisados cada uno
- [x] Q2. agy y Codex: si el sondeo real muestra que no se puede identificar al hijo goldfish, ¿se documenta como limitación o se bloquea la revisión goldfish en ese runtime? → Documentar la limitación (Recommended): con la evidencia del sondeo real; la revisión splash avisa en ese runtime de que el aislamiento no está garantizado
- [x] Q3. Instalación: ¿`mem setup` instala la definición del agente goldfish en los runtimes que la soportan, o se documenta para que la cree la persona? → mem setup la instala (Recommended): escribe el agente splash en cada runtime que lo soporta y mem uninstall lo retira

## Slices
### VS-01. agy y Codex reconocen al subagente splash, o su limitación queda documentada
Tareas: 1, 2, 9, 10, 11, 12
Criterios: C-04

### VS-02. En Claude Code, el subagente splash arranca sin aviso, sin tools MCP y sin CLAUDE.md
Tareas: 3, 4, 5
Criterios: C-01, C-02

### VS-03. En OpenCode, el plugin no inyecta el aviso al subagente splash
Tareas: 6, 7
Criterios: C-03

### VS-04. Un harness real confirma que el splash arranca sin memoria
Tareas: 8
Criterios: C-05

## Checklist
- [x] 1. Sondear en agy real qué argumentos trae `PreToolUse(invoke_subagent)` y si agy permite definir un agente `splash` (`adapters/primary/cli/hook_agy.go`) → observación manual con el payload capturado y la versión de agy
- [x] 2. Sondear en Codex real qué contexto e instrucciones hereda un hijo de `spawn_agent` y si admite un agente `splash` (hooks de Codex en `adapters/primary/cli/codex_hooks_migration.go`) → observación manual con la versión de Codex y lo heredado
- [x] 3. Hacer que `hookSubagentStart` (`adapters/primary/cli/cmd_hook.go`) no emita contexto cuando `agent_type` es `splash` y conserve el aviso de HCD-026 en los demás tipos → `go test ./adapters/primary/cli/ -run TestSubagentStart_Splash -count=1`
- [x] 4. Crear la plantilla del agente `infrastructure/templates/agents/claude/splash.md` (Claude: `tools: Read, Grep, Glob`, sin MCP, `omitClaudeMd: true`) → `go test ./infrastructure/ -run TestSplashAgentTemplate -count=1`
- [x] 5. Instalar con `mem setup` el agente en `~/.claude/agents/splash.md` y retirarlo con `mem uninstall` (`adapters/primary/setup/`) → `go test ./adapters/primary/setup/ -run TestClaudeSplashAgent -count=1`
- [x] 6. Hacer que el plugin `infrastructure/plugin/opencode/gomemory.ts` no inyecte el aviso de subagente cuando el agente del hijo es `splash` → `node --test --test-name-pattern splash infrastructure/plugin/opencode/gomemory.test.mjs`
- [x] 7. Instalar con `mem setup` el agente `splash` de OpenCode y retirarlo con `mem uninstall` (`adapters/primary/setup/`) → `go test ./adapters/primary/setup/ -run TestOpenCodeSplashAgent -count=1`
- [x] 8. Lanzar un subagente `splash` en un harness real (Claude Code, agy, Codex u OpenCode) y revisar su contexto inicial contra lo que emite `adapters/primary/cli/cmd_hook.go` → observación manual: nombra el harness y confirma que no recibe «decisiones previas» ni tools de memoria
- [x] 9. Documentar en `docs/MANUAL.md` el agente `splash` por runtime y las limitaciones de agy y Codex según los sondeos → `git grep -q 'splash' -- docs/MANUAL.md`
- [x] 10. Hacer que `classifyAgySubagent` (`adapters/primary/cli/hook_agy.go`) guarde el `TypeName` de `invoke_subagent` con el marcador y que el hijo de tipo `splash` arranque sin aviso → `go test ./adapters/primary/cli/ -run TestAgySubagent_Splash -count=1`
- [x] 11. Avisar en el `expand pre-tool` (`adapters/primary/cli/hook_expand.go`) cuando un `spawn_agent` de Codex lanza un `splash` sin `fork_turns: "none"`, porque heredaría la memoria → `go test ./adapters/primary/cli/ -run TestExpandPreTool_SplashCodexSinFork -count=1`
- [x] 12. Hacer que `domain.CitedPaths` (`domain/hcd_lens.go`) ignore las rutas del HOME (`~/…`), que no son del repositorio y daban un falso PATH-MISSING → `go test ./domain/ -run TestHCDLens_CitedPathsIgnoraHome -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 20:26 -05:00 · tarea 1 · manual · caducada
      agy 1.5.2 (agy -p, proyecto desechable, shim de mem en PATH): el PreToolUse del raíz para invoke_subagent trae toolCall.args = {TypeName: "splash", Role, Prompt}; el SessionStart del hijo llega con conversationId nuevo y lastUserInput vacío, sin campo de padre. agy admite tipos propios (define_subagent) y modelo/workspace opcionales. El hijo respondió que su contexto SÍ contiene 'decisiones previas' (aviso de HCD-026) y NO 'Memoria persistente activa'. Conclusión: agy permite identificar al hijo splash guardando TypeName en el marcador de classifyAgySubagent.
- 2026-11-06 20:27 -05:00 · tarea 2 · manual · caducada
      Codex 0.166.0 (codex exec, proyecto desechable, shim de mem en PATH): spawn_agent acepta task_name y message (obligatorios), agent_type, fork_turns (all|none), model y reasoning_effort. El hijo no dispara session-start ni user-prompt-submit: gomemory solo ve el expand pre-tool del raíz (tool collaborationspawn_agent, tool_input con task_name y fork_turns) y subagent-stop. Con fork_turns=all el hijo hereda la conversación: SÍ contiene 'Memoria persistente activa' y 'decisiones previas'. Con fork_turns=none el hijo NO contiene ninguna de las dos; solo hereda el AGENTS.md global (primer encabezado '## CodeGraph'). Conclusión: en Codex el splash se consigue con spawn_agent(task_name=splash, fork_turns=none); gomemory no puede quitar contexto por hook, pero sí puede vigilar el pre-tool del raíz.
- 2026-11-06 20:28 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run TestSubagentStart_Splash -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_hook_subagent_test.go:69:13: undefined: subagentStartOutput
      adapters/primary/cli/cmd_hook_subagent_test.go:74:10: undefined: subagentStartOutput
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-06 20:29 -05:00 · tarea 4 · rojo observado · caducada · `go test ./infrastructure/ -run TestSplashAgentTemplate -count=1` (salida 1)
      --- FAIL: TestSplashAgentTemplate (0.00s)
          splash_agent_test.go:14: falta la plantilla del agente splash: open templates/agents/claude/splash.md: file does not exist
      FAIL
      FAIL	mem/infrastructure	0.261s
      FAIL
- 2026-11-06 20:29 -05:00 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/setup/ -run TestClaudeSplashAgent -count=1` (salida 1)
      # mem/adapters/primary/setup [mem/adapters/primary/setup.test]
      adapters/primary/setup/splash_agent_test.go:18:22: undefined: InstallSplashAgent
      adapters/primary/setup/splash_agent_test.go:21:19: undefined: InstallSplashAgent
      adapters/primary/setup/splash_agent_test.go:28:19: undefined: InstallSplashAgent
      FAIL	mem/adapters/primary/setup [build failed]
      FAIL
- 2026-11-06 20:31 -05:00 · tarea 6 · rojo observado · caducada · `node --test --test-name-pattern splash infrastructure/plugin/opencode/gomemory.test.mjs` (salida 1)
      ✖ failing tests:
      test at infrastructure/plugin/opencode/gomemory.test.mjs:880:1
      ✖ splash child session gets nothing from gomemory (1.848791ms)
        AssertionError [ERR_ASSERTION]: splash por la sesión: sin nada de gomemory
        + actual - expected
        + 'AVISO-SUBAGENTE'
        - ''
            at TestContext.<anonymous> (file:///Users/josegomezj/home/rcw/gomemory/infrastructure/plugin/opencode/gomemory.test.mjs:902:10)
            at async Test.run (node:internal/test_runner/test:1409:7)
            at async startSubtestAfterBootstrap (node:internal/test_runner/harness:387:3) {
          generatedMessage: false,
          code: 'ERR_ASSERTION',
          actual: 'AVISO-SUBAGENTE',
          expected: '',
          operator: 'strictEqual',
          diff: 'simple'
        }
- 2026-11-06 20:31 -05:00 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/setup/ -run TestOpenCodeSplashAgent -count=1` (salida 1)
      --- FAIL: TestOpenCodeSplashAgent (0.00s)
          splash_agent_test.go:18: instalación = [] <nil>, quiero solo /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestOpenCodeSplashAgent2137088080/001/.config/opencode/agents/splash.md
      FAIL
      FAIL	mem/adapters/primary/setup	0.214s
      FAIL
- 2026-11-06 20:32 -05:00 · tarea 10 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run TestAgySubagent_Splash -count=1` (salida 1)
      --- FAIL: TestAgySubagent_Splash (0.00s)
          hook_agy_subagent_test.go:64: el hijo splash no debe recibir nada de gomemory, se planificó [subagent-start agy --emit=agy]
      FAIL
      FAIL	mem/adapters/primary/cli	0.313s
      FAIL
- 2026-11-06 20:33 -05:00 · tarea 11 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run TestExpandPreTool_SplashCodexSinFork -count=1` (salida 1)
      --- FAIL: TestExpandPreTool_SplashCodexSinFork (0.00s)
          hook_expand_splash_test.go:35: un splash con fork_turns=all debe avisar, salió "{}"
      FAIL
      FAIL	mem/adapters/primary/cli	0.308s
      FAIL
- 2026-11-06 20:35 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli/ -run TestSubagentStart_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.296s
- 2026-11-06 20:35 -05:00 · tarea 4 · verde · caducada · `go test ./infrastructure/ -run TestSplashAgentTemplate -count=1` (salida 0)
      ok  	mem/infrastructure	0.280s
- 2026-11-06 20:35 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/setup/ -run TestClaudeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.232s
- 2026-11-06 20:35 -05:00 · tarea 6 · verde · caducada · `node --test --test-name-pattern splash infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ splash child session gets nothing from gomemory (0.927833ms)
      ℹ tests 1
      ℹ suites 0
      ℹ pass 1
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 143.005916
- 2026-11-06 20:35 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/setup/ -run TestOpenCodeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.186s
- 2026-11-06 20:35 -05:00 · tarea 9 · verde · caducada · `git grep -q 'splash' -- docs/MANUAL.md` (salida 0)
- 2026-11-06 20:35 -05:00 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli/ -run TestAgySubagent_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.390s
- 2026-11-06 20:35 -05:00 · tarea 11 · verde · caducada · `go test ./adapters/primary/cli/ -run TestExpandPreTool_SplashCodexSinFork -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.286s
- 2026-11-06 20:36 -05:00 · tarea 1 · manual · caducada
      agy 1.5.2 (agy -p, proyecto desechable, shim de mem en PATH): PreToolUse(invoke_subagent) del raíz trae toolCall.args = {TypeName: "splash", Role, Prompt}; el SessionStart del hijo llega con conversationId nuevo y lastUserInput vacío, sin campo de padre; los tipos se definen con define_subagent. Con mem v2.49.2 el hijo respondió que SÍ contenía 'decisiones previas'. Conclusión: agy identifica al hijo splash por TypeName.
- 2026-11-06 20:36 -05:00 · tarea 2 · manual · caducada
      Codex 0.166.0 (codex exec, proyecto desechable, shim de mem en PATH): spawn_agent acepta task_name, message, agent_type, fork_turns (all|none), model y reasoning_effort; el hijo no dispara hooks de arranque (solo el expand pre-tool del raíz y subagent-stop). Con fork_turns=all hereda la memoria ('Memoria persistente activa' y 'decisiones previas': sí); con fork_turns=none no contiene ninguna de las dos y solo hereda el AGENTS.md global (## CodeGraph). Limitación documentada: gomemory no quita contexto al hijo por hook; el aislamiento es fork_turns=none y el pre-tool del raíz avisa.
- 2026-11-06 20:36 -05:00 · tarea 8 · manual · caducada
      Harness reales con el binario de desarrollo en PATH, proyecto desechable. (a) Claude Code 2.1.319, claude -p con .claude/agents/splash.md de la plantilla: el subagente splash respondió NO a 'decisiones previas', NO a 'Memoria persistente activa' y que sus únicas herramientas son Read, Grep y Glob (sin MCP). (b) agy 1.5.2, invoke_subagent TypeName splash: el hijo respondió NO a 'decisiones previas' (con v2.49.2 era SÍ) y su conversación quedó en .memory/.agy-splash-sessions.
- 2026-11-06 20:37 -05:00 · tarea 12 · rojo observado · caducada · `go test ./domain/ -run TestHCDLens_CitedPathsIgnoraHome -count=1` (salida 1)
      # mem/domain
      domain/hcd_lens_test.go:13:1: imports must appear before other declarations
      FAIL	mem/domain [setup failed]
      FAIL
- 2026-11-06 20:37 -05:00 · tarea 12 · rojo observado · caducada · `go test ./domain/ -run TestHCDLens_CitedPathsIgnoraHome -count=1` (salida 1)
      --- FAIL: TestHCDLens_CitedPathsIgnoraHome (0.00s)
          hcd_lens_test.go:15: CitedPaths = [adapters/primary/setup/splash_agent.go ~/.claude/agents/splash.md], quiero solo la ruta del repositorio
      FAIL
      FAIL	mem/domain	0.366s
      FAIL
- 2026-11-06 20:38 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli/ -run TestSubagentStart_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.308s
- 2026-11-06 20:38 -05:00 · tarea 4 · verde · caducada · `go test ./infrastructure/ -run TestSplashAgentTemplate -count=1` (salida 0)
      ok  	mem/infrastructure	0.254s
- 2026-11-06 20:38 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/setup/ -run TestClaudeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.225s
- 2026-11-06 20:38 -05:00 · tarea 6 · verde · caducada · `node --test --test-name-pattern splash infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ splash child session gets nothing from gomemory (0.955208ms)
      ℹ tests 1
      ℹ suites 0
      ℹ pass 1
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 134.193791
- 2026-11-06 20:38 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/setup/ -run TestOpenCodeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.182s
- 2026-11-06 20:38 -05:00 · tarea 9 · verde · caducada · `git grep -q 'splash' -- docs/MANUAL.md` (salida 0)
- 2026-11-06 20:38 -05:00 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli/ -run TestAgySubagent_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.756s
- 2026-11-06 20:38 -05:00 · tarea 11 · verde · caducada · `go test ./adapters/primary/cli/ -run TestExpandPreTool_SplashCodexSinFork -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.375s
- 2026-11-06 20:38 -05:00 · tarea 12 · verde · caducada · `go test ./domain/ -run TestHCDLens_CitedPathsIgnoraHome -count=1` (salida 0)
      ok  	mem/domain	0.139s
- 2026-11-06 20:38 -05:00 · tarea 1 · manual · caducada
      agy 1.5.2 (agy -p, proyecto desechable, shim de mem en PATH): PreToolUse(invoke_subagent) del raíz trae toolCall.args = {TypeName: "splash", Role, Prompt}; el SessionStart del hijo llega con conversationId nuevo y lastUserInput vacío, sin campo de padre; los tipos se definen con define_subagent. Con mem v2.49.2 el hijo respondió que SÍ contenía 'decisiones previas'. Conclusión: agy identifica al hijo splash por TypeName.
- 2026-11-06 20:38 -05:00 · tarea 2 · manual · caducada
      Codex 0.166.0 (codex exec, proyecto desechable, shim de mem en PATH): spawn_agent acepta task_name, message, agent_type, fork_turns (all|none), model y reasoning_effort; el hijo no dispara hooks de arranque (solo el expand pre-tool del raíz y subagent-stop). Con fork_turns=all hereda la memoria ('Memoria persistente activa' y 'decisiones previas': sí); con fork_turns=none no contiene ninguna de las dos y solo hereda el AGENTS.md global (## CodeGraph). Limitación documentada: gomemory no quita contexto al hijo por hook; el aislamiento es fork_turns=none y el pre-tool del raíz avisa.
- 2026-11-06 20:38 -05:00 · tarea 8 · manual · caducada
      Harness reales con el binario de desarrollo en PATH, proyecto desechable. (a) Claude Code 2.1.319, claude -p con .claude/agents/splash.md de la plantilla: el subagente splash respondió NO a 'decisiones previas', NO a 'Memoria persistente activa' y que sus únicas herramientas son Read, Grep y Glob (sin MCP). (b) agy 1.5.2, invoke_subagent TypeName splash: el hijo respondió NO a 'decisiones previas' (con v2.49.2 era SÍ) y su conversación quedó en .memory/.agy-splash-sessions.
- 2026-11-06 20:40 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli/ -run TestSubagentStart_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.308s
- 2026-11-06 20:40 -05:00 · tarea 4 · verde · caducada · `go test ./infrastructure/ -run TestSplashAgentTemplate -count=1` (salida 0)
      ok  	mem/infrastructure	0.222s
- 2026-11-06 20:40 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/primary/setup/ -run TestClaudeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.200s
- 2026-11-06 20:41 -05:00 · tarea 6 · verde · caducada · `node --test --test-name-pattern splash infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ splash child session gets nothing from gomemory (1.073042ms)
      ℹ tests 1
      ℹ suites 0
      ℹ pass 1
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 146.800667
- 2026-11-06 20:41 -05:00 · tarea 7 · verde · caducada · `go test ./adapters/primary/setup/ -run TestOpenCodeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.176s
- 2026-11-06 20:41 -05:00 · tarea 9 · verde · caducada · `git grep -q 'splash' -- docs/MANUAL.md` (salida 0)
- 2026-11-06 20:41 -05:00 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli/ -run TestAgySubagent_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.289s
- 2026-11-06 20:41 -05:00 · tarea 11 · verde · caducada · `go test ./adapters/primary/cli/ -run TestExpandPreTool_SplashCodexSinFork -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	1.002s
- 2026-11-06 20:41 -05:00 · tarea 12 · verde · caducada · `go test ./domain/ -run TestHCDLens_CitedPathsIgnoraHome -count=1` (salida 0)
      ok  	mem/domain	0.145s
- 2026-11-06 20:41 -05:00 · tarea 1 · manual · caducada
      agy 1.5.2 (agy -p, proyecto desechable, shim de mem en PATH): PreToolUse(invoke_subagent) del raíz trae toolCall.args = {TypeName: "splash", Role, Prompt}; el SessionStart del hijo llega con conversationId nuevo y lastUserInput vacío, sin campo de padre; los tipos se definen con define_subagent. Con mem v2.49.2 el hijo respondió que SÍ contenía 'decisiones previas'. Conclusión: agy identifica al hijo splash por TypeName.
- 2026-11-06 20:41 -05:00 · tarea 2 · manual · caducada
      Codex 0.166.0 (codex exec, proyecto desechable, shim de mem en PATH): spawn_agent acepta task_name, message, agent_type, fork_turns (all|none), model y reasoning_effort; el hijo no dispara hooks de arranque (solo el expand pre-tool del raíz y subagent-stop). Con fork_turns=all hereda la memoria ('Memoria persistente activa' y 'decisiones previas': sí); con fork_turns=none no contiene ninguna de las dos y solo hereda el AGENTS.md global (## CodeGraph). Limitación documentada: gomemory no quita contexto al hijo por hook; el aislamiento es fork_turns=none y el pre-tool del raíz avisa.
- 2026-11-06 20:41 -05:00 · tarea 8 · manual · caducada
      Harness reales con el binario de desarrollo en PATH, proyecto desechable. (a) Claude Code 2.1.319, claude -p con .claude/agents/splash.md de la plantilla: el subagente splash respondió NO a 'decisiones previas', NO a 'Memoria persistente activa' y que sus únicas herramientas son Read, Grep y Glob (sin MCP). (b) agy 1.5.2, invoke_subagent TypeName splash: el hijo respondió NO a 'decisiones previas' (con v2.49.2 era SÍ) y su conversación quedó en .memory/.agy-splash-sessions.
- 2026-11-06 21:03 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli/ -run TestSubagentStart_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.296s
- 2026-11-06 21:03 -05:00 · tarea 4 · verde · vigente · `go test ./infrastructure/ -run TestSplashAgentTemplate -count=1` (salida 0)
      ok  	mem/infrastructure	0.366s
- 2026-11-06 21:03 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/setup/ -run TestClaudeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.174s
- 2026-11-06 21:03 -05:00 · tarea 6 · verde · vigente · `node --test --test-name-pattern splash infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ splash child session gets nothing from gomemory (1.613334ms)
      ℹ tests 1
      ℹ suites 0
      ℹ pass 1
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 156.617959
- 2026-11-06 21:03 -05:00 · tarea 7 · verde · vigente · `go test ./adapters/primary/setup/ -run TestOpenCodeSplashAgent -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.160s
- 2026-11-06 21:03 -05:00 · tarea 9 · verde · vigente · `git grep -q 'splash' -- docs/MANUAL.md` (salida 0)
- 2026-11-06 21:03 -05:00 · tarea 10 · verde · vigente · `go test ./adapters/primary/cli/ -run TestAgySubagent_Splash -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.298s
- 2026-11-06 21:03 -05:00 · tarea 11 · verde · vigente · `go test ./adapters/primary/cli/ -run TestExpandPreTool_SplashCodexSinFork -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.310s
- 2026-11-06 21:03 -05:00 · tarea 12 · verde · vigente · `go test ./domain/ -run TestHCDLens_CitedPathsIgnoraHome -count=1` (salida 0)
      ok  	mem/domain	0.129s
- 2026-11-06 21:03 -05:00 · tarea 1 · manual · vigente
      agy 1.5.2 (agy -p, proyecto desechable, shim de mem en PATH): PreToolUse(invoke_subagent) del raíz trae toolCall.args = {TypeName: "splash", Role, Prompt}; el SessionStart del hijo llega con conversationId nuevo y lastUserInput vacío, sin campo de padre; los tipos se definen con define_subagent. Con mem v2.49.2 el hijo respondió que SÍ contenía 'decisiones previas'. Conclusión: agy identifica al hijo splash por TypeName.
- 2026-11-06 21:03 -05:00 · tarea 2 · manual · vigente
      Codex 0.166.0 (codex exec, proyecto desechable, shim de mem en PATH): spawn_agent acepta task_name, message, agent_type, fork_turns (all|none), model y reasoning_effort; el hijo no dispara hooks de arranque (solo el expand pre-tool del raíz y subagent-stop). Con fork_turns=all hereda la memoria ('Memoria persistente activa' y 'decisiones previas': sí); con fork_turns=none no contiene ninguna de las dos y solo hereda el AGENTS.md global (## CodeGraph). Limitación documentada: gomemory no quita contexto al hijo por hook; el aislamiento es fork_turns=none y el pre-tool del raíz avisa.
- 2026-11-06 21:03 -05:00 · tarea 8 · manual · vigente
      Harness reales con el binario de desarrollo en PATH, proyecto desechable. (a) Claude Code 2.1.319, claude -p con .claude/agents/splash.md de la plantilla: el subagente splash respondió NO a 'decisiones previas', NO a 'Memoria persistente activa' y que sus únicas herramientas son Read, Grep y Glob (sin MCP). (b) agy 1.5.2, invoke_subagent TypeName splash: el hijo respondió NO a 'decisiones previas' (con v2.49.2 era SÍ) y su conversación quedó en .memory/.agy-splash-sessions.

## Siguiente
Cerrada con aprobación sobre la revisión af62b371cdde.
