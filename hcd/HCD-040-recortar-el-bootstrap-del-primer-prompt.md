# Recortar el bootstrap del primer prompt

ID: HCD-040
Tipo: technical
Carril: fast
Motivo: mejora técnica medible y acotada al hook user-prompt-submit; qué se recorta se decide en clarify

## Intención
Reducir lo que el primer prompt de cada sesión inyecta (medido con v2.49.1: 4.864 caracteres con Octopus apagado y 5.978 encendido, ~1.500 tokens) sin perder la carga de tools, get_context ni la regla de delegación. Desglose: ToolSearch ~1.580, Octopus 986, protocolo 462, juez 381, privacidad 145, «no confundir sistemas» 787 (repite los nombres de las tools), grafo externo 1.633.


Emisor: `hookUserPromptSubmit` (`adapters/primary/cli/cmd_hook.go`) junta `buildMemoryToolBootstrap` (ToolSearch + get_context/get_plan_context + regla de Octopus) y `memoryProtocolReminder` (guardado, juez, privacidad, «no confundir sistemas», grafo externo). El guardado, el juez y la privacidad ya viajan en las instrucciones del servidor MCP (`buildIntegrationBlock`, `adapters/primary/cli/cmd_install.go`), así que en Claude llegan repetidas.

## Alcance
Incluido: el texto del primer prompt de `hookUserPromptSubmit` (`buildMemoryToolBootstrap` y `memoryProtocolReminder`) según lo que decida Q1, y una prueba que fije el tope de caracteres del primer prompt.
Excluido: los prompts siguientes (ya pasivos); las instrucciones del servidor MCP y el bloque de instalación (`buildIntegrationBlock`); la lista de tools del ToolSearch, que hace falta para cargar las tools diferidas; el arranque de SessionStart (HCD-037).
Decidido en Q1: compactar en todos los runtimes. Se mantienen todas las reglas, reescritas cortas: «no confundir sistemas» sin la lista de tools, el grafo externo en 2-3 frases, y el protocolo, el juez y la privacidad en una línea cada uno.

## ¿Qué pasa si no lo hacemos?
Cada sesión paga ~1.200–1.500 tokens en el primer prompt, y una parte llega repetida porque ya estaba en las instrucciones MCP y en el CLAUDE.md global. Como el texto queda en el historial, el coste se arrastra en todos los turnos hasta la compactación.

## Alternativa más barata
No hacer nada: el coste es moderado (~1.500 tokens por sesión). Pero el recorte toca solo dos cadenas y una prueba, y el ahorro se repite en cada sesión de cada proyecto, así que compensa.


## Aclaraciones
- [x] Q1. ¿Qué se recorta del primer prompt: compactar los textos en todos los runtimes, quitar lo que ya llega por las instrucciones MCP, o ambas cosas? → Compactar en todos (Recommended): mantener todas las reglas en el primer prompt, reescritas cortas: «no confundir sistemas» sin la lista de tools, grafo externo resumido a 2-3 frases, y protocolo, juez y privacidad en una línea cada uno. Sin depender de que el runtime muestre las instrucciones MCP.
- [x] Q2. La feature 038 exige que memoryProtocolReminder contenga literal domain.CodeRetrievalPolicy (~1.600 caracteres, igual que las instrucciones MCP; code_retrieval_policy_test.go). ¿Cómo se compacta el grafo externo sin romper esa coherencia? → Versión corta canónica (Recommended): añadir domain.CodeRetrievalPolicyShort (2-3 frases, mismo orden de prioridad) junto a la larga en domain/code_retrieval_policy.go. Las instrucciones MCP siguen con la larga y el recordatorio usa la corta. La prueba de 038 pasa a exigir la corta en el recordatorio y la larga en MCP.

## Checklist
- [x] 1. Añadir domain.CodeRetrievalPolicyShort y usarla en memoryProtocolReminder (domain/code_retrieval_policy.go, adapters/primary/cli/cmd_hook.go, code_retrieval_policy_test.go) → go test ./adapters/primary/cli -run 'TestMCPInitialize_EntregaPoliticaCanonica|TestRecordatorio_SigueLaMismaPoliticaCanonica' -count=1
- [x] 2. Compactar memoryProtocolReminder (guardado, juez, privacidad y «no confundir» sin lista de tools) bajo un tope probado (adapters/primary/cli/cmd_hook.go, cmd_hook_bootstrap_test.go) → go test ./adapters/primary/cli -run TestPrimerPrompt_CabeEnElTope -count=1
- [x] 3. Comprobar que los contratos de hooks y get_context siguen en verde (tests/contract, tests/integration) → go vet ./... && go test ./adapters/primary/cli/... ./domain/... ./tests/contract/... ./tests/integration/... -count=1
- [x] 4. Medir el primer prompt con el binario real de la rama en un proyecto desechable → manual: caracteres con Octopus apagado y encendido frente a la base 4.864 / 5.978

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-06 05:39 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run 'TestMCPInitialize_EntregaPoliticaCanonica|TestRecordatorio_SigueLaMismaPoliticaCanonica' -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/code_retrieval_policy_test.go:38:54: undefined: domain.CodeRetrievalPolicyShort
      adapters/primary/cli/code_retrieval_policy_test.go:44:26: undefined: domain.CodeRetrievalPolicyShort
      adapters/primary/cli/code_retrieval_policy_test.go:50:31: undefined: domain.CodeRetrievalPolicyShort
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-06 05:40 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestPrimerPrompt_CabeEnElTope -count=1` (salida 1)
      --- FAIL: TestPrimerPrompt_CabeEnElTope (0.00s)
          cmd_hook_bootstrap_test.go:206: octopus=false: el primer prompt mide 4565 caracteres; quiero como mucho 3000
          cmd_hook_bootstrap_test.go:206: octopus=true: el primer prompt mide 5679 caracteres; quiero como mucho 4000
          cmd_hook_bootstrap_test.go:217: el recordatorio no debe repetir la lista de tools de gomemory
      FAIL
      FAIL	mem/adapters/primary/cli	0.485s
      FAIL
- 2026-11-06 05:40 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run 'TestMCPInitialize_EntregaPoliticaCanonica|TestRecordatorio_SigueLaMismaPoliticaCanonica' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.254s
- 2026-11-06 05:40 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestPrimerPrompt_CabeEnElTope -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.285s
- 2026-11-06 05:40 -05:00 · tarea 3 · falló · caducada · `go vet ./... && go test ./adapters/primary/cli/... ./domain/... ./tests/contract/... ./tests/integration/... -count=1` (salida 1)
        ○ .claude/plugins/gomemory: no encontrado
        ○ .claude/settings.json: no encontrado
        ○ .claude/settings.json: sin permisos de gomemory
        ○ opencode.json: sin permisos de gomemory
        ○ envoltorios nativos: no encontrados
      Resumen:
        ✓ Integración de gomemory en /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition2197640837/003
        ✓ /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition2197640837/003/mem (copia v2.49.1)
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ Memoria 1 eliminada
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) de el proyecto "001-71ebe58d7f3afee3".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) por garbage collection de el proyecto "001-71ebe58d7f3afee3".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      FAIL
      FAIL	mem/tests/contract	98.280s
      ok  	mem/tests/integration	50.854s
      FAIL
- 2026-11-06 05:44 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run 'TestMCPInitialize_EntregaPoliticaCanonica|TestRecordatorio_SigueLaMismaPoliticaCanonica' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.312s
- 2026-11-06 05:44 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestPrimerPrompt_CabeEnElTope -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.293s
- 2026-11-06 05:44 -05:00 · tarea 3 · verde · caducada · `go vet ./... && go test ./adapters/primary/cli/... ./domain/... ./tests/contract/... ./tests/integration/... -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	17.517s
      ok  	mem/adapters/primary/cli/clikit	0.178s
      ok  	mem/adapters/primary/cli/hcd	2.467s
      ok  	mem/adapters/primary/cli/review	1.890s
      ok  	mem/adapters/primary/cli/socratica	1.616s
      ok  	mem/domain	0.238s
      ok  	mem/tests/contract	98.513s
      ok  	mem/tests/integration	49.649s
- 2026-11-06 05:46 -05:00 · tarea 4 · manual · caducada
      Binario de la rama (go build ./infrastructure) en un proyecto desechable, mem hook user-prompt-submit en una conversación nueva: 3.044 caracteres con Octopus apagado (base v2.49.1: 4.864, −37 %) y 4.158 con Octopus encendido (base: 5.978, −30 %). Se mantienen todas las secciones: ToolSearch, get_context/get_plan_context, Octopus, guardado, JUEZ IMPARCIAL, PRIVACIDAD, no confundir sistemas y grafo. El mismo recordatorio cierra get_context y get_plan_context, así que allí también se ahorran ~1.800 caracteres por llamada.
- 2026-11-06 05:46 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run 'TestMCPInitialize_EntregaPoliticaCanonica|TestRecordatorio_SigueLaMismaPoliticaCanonica' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.319s
- 2026-11-06 05:46 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/primary/cli -run TestPrimerPrompt_CabeEnElTope -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.288s
- 2026-11-06 05:46 -05:00 · tarea 3 · verde · vigente · `go vet ./... && go test ./adapters/primary/cli/... ./domain/... ./tests/contract/... ./tests/integration/... -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	18.104s
      ok  	mem/adapters/primary/cli/clikit	0.223s
      ok  	mem/adapters/primary/cli/hcd	2.843s
      ok  	mem/adapters/primary/cli/review	2.155s
      ok  	mem/adapters/primary/cli/socratica	1.888s
      ok  	mem/domain	0.271s
      ok  	mem/tests/contract	96.225s
      ok  	mem/tests/integration	48.652s
- 2026-11-06 05:48 -05:00 · tarea 4 · manual · vigente
      Binario de la rama (go build ./infrastructure) en un proyecto desechable, mem hook user-prompt-submit en una conversación nueva: 3.044 caracteres con Octopus apagado (base v2.49.1: 4.864, −37 %) y 4.158 con Octopus encendido (base: 5.978, −30 %). Se mantienen todas las secciones: ToolSearch, get_context/get_plan_context, Octopus, guardado, JUEZ IMPARCIAL, PRIVACIDAD, no confundir sistemas y grafo. El mismo recordatorio cierra get_context y get_plan_context, así que allí también se ahorran ~1.800 caracteres por llamada.

## Siguiente
Cerrada con aprobación sobre la revisión eb4bdeb77405.
