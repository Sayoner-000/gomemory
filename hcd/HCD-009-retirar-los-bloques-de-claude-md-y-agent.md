# Retirar los bloques de CLAUDE.md y AGENTS.md del ámbito global

ID: HCD-009
Tipo: evolution
Carril: full
Motivo: Cambia el contrato público de setup-mcp --scope global y toca archivos personales, con decisiones interdependientes

## Intención
gomemory entrega protocolo y baseline por MCP, hooks y contexto, sin escribir en los archivos de instrucciones de usuario, y la instalación deja esos archivos solo con el contenido de la persona


## Alcance
Incluido: retirar el refuerzo de preferencias de los hooks, conservando el aviso de compactar; reordenar el bloque de protocolo para que las reglas precedan a la lista de herramientas, de modo que el truncado de las instrucciones MCP solo afecte a esa lista; quitar la escritura del bloque en `setup-mcp --scope global`, conservando `atomic-decomposition`; entregar el baseline universal como documento fijado, con la plantilla embebida de respaldo para proyectos ya instalados; actualizar `domain/channel_matrix.go` y el inspector, dejando los nombres solo como Legacy; retirar los bloques marcados de los tres archivos globales con copia de seguridad; depurar código, pruebas y documentación que traten esos archivos como canal vigente.
Excluido: ejecutar la compactación, porque gomemory no tiene canal para ordenarla en los agentes; el texto propio de la persona fuera de los marcadores; el marcador `gomemory-workrules-v1` de `~/.claude/CLAUDE.md`, que ningún código de producción gestiona; `uninstall` y la limpieza de heredados, que conservan los nombres para retirar instalaciones antiguas; `.cursorrules` y `.windsurfrules`.

## ¿Qué pasa si no lo hacemos?
El protocolo sigue duplicado entre MCP y unas 75 líneas por archivo global, gomemory sigue escribiendo en archivos personales y cada versión del bloque compite con la que ya llega por MCP y hooks.

## Alternativa más barata
Dejar de escribir el bloque sin migrar ni tocar la matriz: unas pocas líneas, pero `doctor` daría alarmas falsas, los bloques quedarían congelados en `v10` y el baseline universal se perdería en instalaciones nuevas.

## Historias
### US1. Entrega sin archivos de instrucciones
Como persona que usa gomemory, quiero recibir protocolo y baseline sin depender de CLAUDE.md ni AGENTS.md, para no mantener una copia que diverge.
- C-01: Dado un agente sin bloques de gomemory en sus archivos globales, cuando abre una sesión real en Claude, Codex u OpenCode, entonces recibe el protocolo y el baseline universal.
- C-02: Dado un proyecto instalado antes de este cambio, cuando se construye su contexto sin reinstalar, entonces incluye el baseline desde la plantilla embebida.
- C-10: Dado que Claude trunca las instrucciones MCP hacia el carácter 3900, cuando se recorta el bloque en ese punto, entonces conserva guardado, juez, privacidad, modo plan, inicio, cierre y consulta.
- C-11: Dado una sesión larga con preferencias guardadas, cuando termina un turno o llega un prompt, entonces ningún hook reinyecta preferencias y, al superar el umbral, solo aparece el aviso de compactar.
- C-03: Dado que el baseline llega por contexto, cuando se mide el contexto de sesión, entonces se mantiene dentro de su presupuesto vigente.

### US2. Instalación global sin bloques
Como persona que instala gomemory, quiero que la instalación global no escriba en mis instrucciones, para conservar esos archivos como míos.
- C-04: Dado un HOME con directorios de los tres agentes, cuando se ejecuta `mem setup-mcp --scope global`, entonces no se escribe ningún bloque y `atomic-decomposition` sigue instalada en Claude y OpenCode.
- C-05: Dado un agente sin bloques, cuando se ejecuta el diagnóstico, entonces no informa de un canal de instrucciones ausente ni desactualizado.

### US3. Retirada segura de los bloques
Como persona con una instalación anterior, quiero que se retiren los bloques con copia de seguridad, para recuperar mi archivo si algo sale mal.
- C-06: Dado un archivo global con bloques marcados y texto propio, cuando se migra, entonces queda una copia del original y el texto propio se conserva byte a byte.
- C-07: Dado un archivo ya migrado, cuando se migra otra vez, entonces no cambia nada y no se crea otra copia.
- C-08: Dado que la copia de seguridad falla, cuando se intenta migrar, entonces el archivo original no se modifica y se informa del error.

### US4. Depuración de referencias
Como mantenedor, quiero que el repositorio trate esos archivos solo como heredados, para no reprocesar este cambio.
- C-09: Dado el repositorio, cuando se buscan `CLAUDE.md` y `AGENTS.md`, entonces solo aparecen como heredados, en limpieza o desinstalación, o en documentación que explica la retirada.

## Aclaraciones
- [x] Q1. ¿Quién ejecuta la comprobación en sesión real de C-01? Codex no ejecuta hooks en codex exec, así que exige sesión interactiva. → Tú, con guion (Recomendado)
- [x] Q2. ¿Cuándo se retiran los bloques de los archivos globales: automáticamente al ejecutar setup-mcp --scope global, o con un comando explícito? → Con setup-mcp global (Recomendado)
- [x] Q3. Las instrucciones MCP llegan truncadas en Claude hacia el carácter 3900 de 5358: se pierden privacidad, modo plan, inicio, cierre y consulta. ¿Cómo se garantiza el protocolo sin el bloque de CLAUDE.md? → Reordenar el bloque (Recomendado)

## Slices
### VS-01. El baseline llega por contexto, con o sin semilla
Tareas: 1, 2, 3, 14, 15, 16, 20
Criterios: C-02, C-03, C-10, C-11

### VS-02. Control previo en sesión real de los tres agentes
Tareas: 4, 5, 6
Criterios: C-01

### VS-03. La instalación global deja de escribir bloques
Tareas: 7, 8, 18, 21
Criterios: C-04, C-05

### VS-04. Retirada segura de los bloques
Tareas: 9, 10, 11, 12, 17
Criterios: C-06, C-07, C-08, C-01

### VS-05. Depuración de referencias
Tareas: 13, 19
Criterios: C-09

## Checklist
- [x] 1. Añadir el baseline universal al catálogo de documentos fijados → `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1`
- [x] 2. Emitir el baseline en el contexto con plantilla embebida de respaldo (dep: 1) → `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1`
- [x] 3. Comprobar que el contexto de sesión cabe en su presupuesto con el baseline (dep: 2) → `go test ./tests/contract/ -run 'TestHookBudget' -count=1`
- [x] 4. Observar el encabezado del baseline y el protocolo en una sesión real de Claude (dep: 2) → observación manual de la persona con el guion
- [x] 5. Observar lo mismo en una sesión interactiva real de Codex (dep: 2) → observación manual de la persona con el guion
- [x] 6. Observar lo mismo en una sesión real de OpenCode (dep: 2) → observación manual de la persona con el guion
- [x] 7. Quitar la escritura del bloque en `setup-mcp --scope global` conservando `atomic-decomposition` (dep: 4, 5, 6) → `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestRunGlobalScopeSetup|TestInstallAtomicPlanGlobal' -count=1`
- [x] 8. Marcar los archivos de instrucciones de usuario como Legacy en la matriz y el inspector (dep: 7) → `go test ./domain/ ./adapters/primary/setup/ -run 'Matrix|Activation|Channel' -count=1`
- [x] 9. Copiar el archivo antes de retirar sus bloques en `setup-mcp --scope global` (dep: 7) → `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_ConservaTextoYCopia$' -count=1`
- [x] 10. Hacer la retirada idempotente y abortarla si falla la copia (dep: 9) → `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_(Idempotente|FalloDeCopia)$' -count=1`
- [x] 11. Migrar los tres archivos reales de la persona (dep: 10) → observación manual: copias presentes y `diff` que solo muestra los bloques retirados
- [x] 12. Repetir la observación en sesión real de los tres agentes, ya sin bloques (dep: 11) → observación manual de la persona con el guion
- [x] 13. Depurar las referencias vigentes a esos archivos (dep: 8) → `grep` documentado: solo quedan usos heredados, de limpieza o de retirada
- [x] 14. Retirar el refuerzo de preferencias de los hooks (dep: 1) → `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1`
- [x] 15. Mover las reglas por delante de la lista de herramientas en el bloque de protocolo (dep: 1) → `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado$' -count=1`
- [x] 16. Conservar el baseline entero en el contexto que OpenCode reinyecta cada turno (dep: 2) → `go test ./application/usecases/ -run 'TestSessionDelta_ConservaBaseline' -count=1 && go test ./tests/integration/ -run 'TestContextKeepBaseline' -count=1 && node --test infrastructure/plugin/opencode/gomemory.test.mjs`
- [x] 17. Retirar solo bloques con inicio y fin marcados, todos, sin tocar el resto del archivo (dep: 9) → `go test ./adapters/primary/cli/ -run 'TestMigracionEstricta_|TestMigracionGlobal_' -count=1`
- [x] 18. Limitar la migración a los agentes elegidos y dar el remedio de `doctor` según el ámbito (dep: 8) → `go test ./adapters/primary/setup/ -run 'TestGlobalInstructionPaths_|TestActivationInspect_InstruccionesDeUsuarioSoloComoHeredadas|TestActivationInspect_RemedioPorAmbito' -count=1`
- [x] 19. Actualizar la documentación del refuerzo retirado y del canal de nivel usuario (dep: 13) → `! grep -nE 'last-preference-nudge|Bloque de protocolo de nivel usuario|archivos de instrucciones de \*\*ámbito usuario' docs/MEMORY-PROTOCOL.md docs/AGENT-INTEGRATION.md docs/architecture.md`
- [x] 20. Respetar el presupuesto al emitir el baseline y conservarlo también en OpenCode 1.x (dep: 16) → `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1 && go test ./tests/contract/ -run 'TestOpenCodeV2_FabricaV1IdenticaALaReferencia' -count=1 && node --test infrastructure/plugin/opencode/gomemory.test.mjs`
- [x] 21. Dar en `doctor` un comando solo si puede resolver el bloque; en otro caso, pedir retirarlo a mano (dep: 18) → `go test ./adapters/primary/setup/ -run 'TestActivationInspect_RemedioSeguro' -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-31 15:31 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.169s
- 2026-10-31 15:31 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.169s
- 2026-10-31 15:31 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	5.971s
      ok  	mem/tests/integration	4.481s
- 2026-10-31 15:39 · tarea 6 · manual · caducada
      Sesión interactiva real de OpenCode (Build, DeepSeek V4.1 Flash) con el binario de HCD-009, observada por la persona: la sección Baseline universal (memoria fijada) aparece en el bloque de memoria del proyecto y cita su primera frase; el protocolo llega por las instrucciones MCP, AGENTS.md global y memoria inyectada; describe get_plan_context antes de planificar y nombra get_context, save_memory y search_memories. Autodeclarado por el agente, con los bloques aún presentes.
- 2026-10-31 15:43 · tarea 2 · falló · caducada · `sh -c go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 2)
      	buildjson       build -json encoding
      	buildmode       build modes
      	c               calling between Go and C
      	cache           build and test caching
      	environment     environment variables
      	filetype        file types
      	goauth          GOAUTH environment variable
      	go.mod          the go.mod file
      	gopath          GOPATH environment variable
      	goproxy         module proxy protocol
      	importpath      import path syntax
      	modules         modules, module versions, and more
      	module-auth     module authentication using go.sum
      	packages        package lists and patterns
      	private         configuration for downloading non-public code
      	testflag        testing flags
      	testfunc        testing functions
      	vcs             controlling version control with GOVCS
      Use "go help <topic>" for more information about that topic.
- 2026-10-31 15:43 · tarea 3 · falló · caducada · `sh -c go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 2)
      	buildjson       build -json encoding
      	buildmode       build modes
      	c               calling between Go and C
      	cache           build and test caching
      	environment     environment variables
      	filetype        file types
      	goauth          GOAUTH environment variable
      	go.mod          the go.mod file
      	gopath          GOPATH environment variable
      	goproxy         module proxy protocol
      	importpath      import path syntax
      	modules         modules, module versions, and more
      	module-auth     module authentication using go.sum
      	packages        package lists and patterns
      	private         configuration for downloading non-public code
      	testflag        testing flags
      	testfunc        testing functions
      	vcs             controlling version control with GOVCS
      Use "go help <topic>" for more information about that topic.
- 2026-10-31 15:43 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.214s
- 2026-10-31 15:43 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.104s
      ok  	mem/tests/integration	4.555s
- 2026-10-31 15:43 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.136s
- 2026-10-31 15:44 · tarea 14 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestComputePreferenceReinforcement' -count=1` (salida 1)
      --- FAIL: TestComputePreferenceReinforcement_OmiteDocumentosFijados (0.01s)
          cmd_footprint_test.go:147: un documento fijado no es una preferencia: "REFUERZO DE PREFERENCIAS (sesión larga sin compactar): estas reglas del usuario siguen activas, no las pierdas de vista:\n\n- **Baseline**: cuerpo del baseline\n- **Reglas**: cuerpo de las reglas\n- **Tuteo**: usa tuteo\n"
      FAIL
      FAIL	mem/adapters/primary/cli	0.316s
      FAIL
- 2026-10-31 15:45 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.149s
- 2026-10-31 15:45 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.176s
- 2026-10-31 15:45 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.059s
      ok  	mem/tests/integration	4.596s
- 2026-10-31 15:45 · tarea 14 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestComputePreferenceReinforcement' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.375s
- 2026-10-31 15:46 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.134s
- 2026-10-31 15:46 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.192s
- 2026-10-31 15:46 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.055s
      ok  	mem/tests/integration	4.485s
- 2026-10-31 15:47 · tarea 14 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestComputePreferenceReinforcement' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.311s
- 2026-10-31 15:54 · tarea 15 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado$' -count=1` (salida 1)
      --- FAIL: TestIntegrationBlock_ReglasSobrevivenAlTruncado (0.00s)
          cmd_install_protocol_test.go:217: "### Juez imparcial" queda después del corte
          cmd_install_protocol_test.go:217: "### Privacidad" queda después del corte
          cmd_install_protocol_test.go:217: "### Al entrar en modo plan" queda después del corte
          cmd_install_protocol_test.go:217: "### Al inicio de cada sesión" queda después del corte
          cmd_install_protocol_test.go:217: "### Al cerrar la sesión" queda después del corte
          cmd_install_protocol_test.go:217: "### Consultar memoria" queda después del corte
      FAIL
      FAIL	mem/adapters/primary/cli	0.420s
      FAIL
- 2026-10-31 15:54 · tarea 14 · rojo observado · caducada · `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1` (salida 1)
      # mem/tests/integration
      found packages main (agent_notice_hook_integration_test.go) and integration (hook_sin_refuerzo_test.go) in /Users/josegomezj/home/rcw/gomemory/tests/integration
      FAIL	mem/tests/integration [setup failed]
      FAIL
- 2026-10-31 15:55 · tarea 14 · rojo observado · caducada · `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1` (salida 1)
      --- FAIL: TestHook_SinRefuerzoDePreferencias (1.01s)
          hook_sin_refuerzo_test.go:28: nudge reinyecta preferencias: "Si vas a entrar en modo plan (o la tarea pide un plan/enfoque antes de tocar código), llama a get_plan_context() ANTES de redactar: trae el método de descomposición atómica y el historial del proyecto. Usa el grafo de código para explorar y el árbol de tareas atómicas para presentar el resultado.\n\nREFUERZO DE PREFERENCIAS (sesión larga sin compactar): estas reglas del usuario siguen activas, no las pierdas de vista:\n\n- **Regla**: preferencia centinela\n"
          hook_sin_refuerzo_test.go:28: turn-end reinyecta preferencias: "{\"hookSpecificOutput\":{\"additionalContext\":\"REFUERZO DE PREFERENCIAS (sesión larga sin compactar): estas reglas del usuario siguen activas, no las pierdas de vista:\\n\\n- **Regla**: preferencia centinela\\n\",\"hookEventName\":\"Stop\"}}"
      FAIL
      FAIL	mem/tests/integration	1.294s
      FAIL
- 2026-10-31 15:59 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.136s
- 2026-10-31 15:59 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.207s
- 2026-10-31 15:59 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.034s
      ok  	mem/tests/integration	4.534s
- 2026-10-31 15:59 · tarea 14 · verde · caducada · `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1` (salida 0)
      ok  	mem/tests/integration	1.106s
- 2026-10-31 15:59 · tarea 15 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.277s
- 2026-10-31 20:26 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo$' -count=1` (salida 1)
      mem/adapters/primary/setup.InstallPlugin({0x0, 0x0}, {0x100bfedad, 0xf}, {0x1d3a6527c630, 0x90}, 0x1d3a65451450)
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/setup/setup.go:23 +0x40
      mem/adapters/primary/setup.installOpenCodePlugin({0x0?, 0x101733818?}, {{0x100bf2954, 0x3}, {0x100bf2954, 0x3}, {0x1d3a6512ba10, 0x1, 0x1}})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/setup/opencode_setup.go:260 +0x218
      mem/adapters/primary/setup.InstallOpenCodeGlobal({{0x100bf2954, 0x3}, {0x100bf2954, 0x3}, {0x1d3a6512ba10, 0x1, 0x1}})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/setup/opencode_setup.go:38 +0x54
      mem/adapters/primary/cli.setupOpenCodeGlobal({{0x100bf2954, 0x3}, {0x1d3a6512ba10, 0x1, 0x1}, {0x100bf2954, 0x3}, 0x1})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup.go:345 +0x70
      mem/adapters/primary/cli.runGlobalScopeAgent({0x100bf616d?, 0x1d3a65451b78?}, {{0x100bf2954, 0x3}, {0x1d3a6512ba10, 0x1, 0x1}, {0x100bf2954, 0x3}, 0x1})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup.go:331 +0x114
      mem/adapters/primary/cli.runGlobalScopeSetup({0x1d3a65451ef0, 0x3, 0x1d3a000001ed?})
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup.go:170 +0x220
      mem/adapters/primary/cli.TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo(0x1d3a6525d688)
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_mcp_setup_hooks_test.go:26 +0x25c
      testing.tRunner(0x1d3a6525d688, 0x1017d9698)
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2195 +0xc4
      created by testing.(*T).Run in goroutine 1
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2260 +0x3b8
      FAIL	mem/adapters/primary/cli	1.066s
      FAIL
- 2026-10-31 20:27 · tarea 7 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo$' -count=1` (salida 1)
      loquesYConservaElMetodo727596270/001/.claude/skills/atomic-decomposition/SKILL.md
        ✅ planificación atómica: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.config/opencode/AGENTS.md
        ✅ planificación atómica: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.config/opencode/commands/atomic-decomposition.md
        ✅ planificación atómica: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.codex/AGENTS.md
        ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.codex/skills/atomic-decomposition/SKILL.md
        ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.config/opencode/skills/atomic-decomposition/SKILL.md
        ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.claude/skills/constitution/SKILL.md
        ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.codex/skills/constitution/SKILL.md
        ✅ habilidad: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo727596270/001/.config/opencode/skills/constitution/SKILL.md
      ✅ 1 registro(s) global(es) completados. Reinicia el agente para que los detecte.
      --- FAIL: TestRunGlobalScopeSetup_NoEscribeBloquesYConservaElMetodo (0.26s)
          cmd_mcp_setup_hooks_test.go:30: .claude/CLAUDE.md recibió un bloque de gomemory
          cmd_mcp_setup_hooks_test.go:30: .config/opencode/AGENTS.md recibió un bloque de gomemory
          cmd_mcp_setup_hooks_test.go:30: .codex/AGENTS.md recibió un bloque de gomemory
      FAIL
      FAIL	mem/adapters/primary/cli	0.575s
      FAIL
- 2026-10-31 20:27 · tarea 9 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_ConservaTextoYCopia$' -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_install_expand_test.go:57:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_expand_test.go:61:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:79:18: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:108:18: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:131:13: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:133:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:147:19: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:155:22: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:176:18: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:200:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:200:20: too many errors
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-10-31 20:27 · tarea 10 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_(Idempotente|FalloDeCopia)$' -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_install_expand_test.go:57:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_expand_test.go:61:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:79:18: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:108:18: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:131:13: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:133:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:147:19: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:155:22: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:176:18: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:200:20: undefined: composeAgentFile
      adapters/primary/cli/cmd_install_protocol_test.go:200:20: too many errors
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-10-31 20:38 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.145s
- 2026-10-31 20:39 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.217s
- 2026-10-31 20:39 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.822s
      ok  	mem/tests/integration	4.858s
- 2026-10-31 20:39 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestRunGlobalScopeSetup|TestInstallAtomicPlanGlobal' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	1.437s
      ok  	mem/adapters/primary/setup	0.185s
- 2026-10-31 20:39 · tarea 8 · verde · caducada · `go test ./domain/ ./adapters/primary/setup/ -run 'Matrix|Activation|Channel|TestC7' -count=1` (salida 0)
      ok  	mem/domain	0.126s
      ok  	mem/adapters/primary/setup	0.136s
- 2026-10-31 20:39 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_ConservaTextoYCopia$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.290s
- 2026-10-31 20:39 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_(Idempotente|FalloDeCopia)$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.278s
- 2026-10-31 20:39 · tarea 13 · verde · caducada · `! grep -rnE 'composeAgentFile|writeGlobalInstructions' --include='*.go' adapters domain application tests infrastructure && test $(grep -rlE 'CLAUDE\.md|AGENTS\.md' --include='*.go' adapters domain application infrastructure | grep -v _test | grep -vE 'atomic_plan_global|activation_inspect|cmd_uninstall|cmd_install_cleanup|cmd_install.go|agentconfig/reader|channel_matrix|cmd_mcp.go' | wc -l) -eq 0` (salida 0)
- 2026-10-31 20:39 · tarea 14 · verde · caducada · `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1` (salida 0)
      ok  	mem/tests/integration	1.162s
- 2026-10-31 20:39 · tarea 15 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.270s
- 2026-10-31 20:41 · tarea 11 · manual · caducada
      Migración real ejecutada por la persona con mem setup-mcp --scope global sobre b066236: retirados los bloques de ~/.claude/CLAUDE.md, ~/.config/opencode/AGENTS.md y ~/.codex/AGENTS.md. Comprobado: una copia por archivo, cero marcadores restantes y texto propio igual salvo líneas vacías en el empalme.
- 2026-10-31 20:54 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.255s
- 2026-10-31 20:54 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.215s
- 2026-10-31 20:54 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.317s
      ok  	mem/tests/integration	4.639s
- 2026-10-31 20:54 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestRunGlobalScopeSetup|TestInstallAtomicPlanGlobal' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	1.725s
      ok  	mem/adapters/primary/setup	0.181s
- 2026-10-31 20:54 · tarea 8 · verde · caducada · `go test ./domain/ ./adapters/primary/setup/ -run 'Matrix|Activation|Channel|TestC7' -count=1` (salida 0)
      ok  	mem/domain	0.109s
      ok  	mem/adapters/primary/setup	0.142s
- 2026-10-31 20:54 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_ConservaTextoYCopia$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.283s
- 2026-10-31 20:54 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_(Idempotente|FalloDeCopia)$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.306s
- 2026-10-31 20:54 · tarea 13 · verde · caducada · `! grep -rnE 'composeAgentFile|writeGlobalInstructions' --include='*.go' adapters domain application tests infrastructure` (salida 0)
- 2026-10-31 20:54 · tarea 14 · verde · caducada · `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1` (salida 0)
      ok  	mem/tests/integration	1.217s
- 2026-10-31 20:54 · tarea 15 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.283s
- 2026-10-31 20:54 · tarea 16 · verde · caducada · `go test ./application/usecases/ -run 'TestSessionDelta_ConservaBaseline' -count=1 && go test ./tests/integration/ -run 'TestContextKeepBaseline' -count=1 && node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (0.802084ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.312875ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.132166ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.261667ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.171791ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.165292ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.292084ms)
      ✔ expand: stop sends the last assistant text (0.196834ms)
      ✔ expand: stop without messages sends an empty last message (0.1895ms)
      ✔ expand: post-tool observes bash only (15.661834ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.694458ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.353792ms)
      ℹ tests 62
      ℹ suites 0
      ℹ pass 62
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1856.31775
- 2026-10-31 20:59 · tarea 5 · manual · caducada
      Sesión interactiva real de Codex sobre b71abce, ya sin bloques en AGENTS.md, observada por la persona: baseline completo en el mensaje developer del hook antes de la primera herramienta, comprobado en el rollout; protocolo v11 completo por MCP; plan, privacidad y cierre correctos. Llamó a end_session pese a la instrucción de lectura.
- 2026-10-31 20:59 · tarea 4 · manual · caducada
      Sesión interactiva real de Claude sobre b71abce, ya sin bloques marcados, observada por la persona: baseline por SessionStart y get_context; protocolo v11 por MCP, cortado en Prioridad de recuperación tras todas las reglas; plan, privacidad y cierre correctos. Autodeclarado. CLAUDE.md conserva texto propio sin marcadores con reglas similares, por lo que no aísla del todo la vía MCP.
- 2026-10-31 21:02 · tarea 11 · manual · caducada
      Migración vigente sobre b71abce: los tres archivos globales siguen con cero marcadores y una sola copia; setup-mcp repetido no creó copias nuevas y actualizó el plugin de OpenCode con keep-baseline.
- 2026-10-31 21:02 · tarea 6 · manual · caducada
      Sesión real de OpenCode 2.2.1 con plugin actualizado, observada por la persona: en el primer turno ve la sección Baseline universal con su primera frase sin llamar a full=true; protocolo v11 por instrucciones MCP y contexto inyectado, sin bloques en AGENTS.md. Autodeclarado; consultó el repositorio pese a la instrucción.
- 2026-10-31 21:02 · tarea 12 · manual · caducada
      Ronda final sobre b71abce, sin bloques en los archivos globales, observada por la persona en sesiones reales: Claude y Codex reciben baseline y protocolo v11 por hooks y MCP; en OpenCode 2.2.1 el baseline sigue completo en el segundo turno, sin marcador ya entregado. Respuestas autodeclaradas; Codex comprobado además en su rollout.
- 2026-10-31 21:20 · tarea 17 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionEstricta_|TestMigracionGlobal_' -count=1` (salida 1)
      --- FAIL: TestMigracionEstricta_BloqueSinCierreNoSeToca (0.01s)
          global_instructions_migration_test.go:116: baseline: archivo alterado: "", open /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestMigracionEstricta_BloqueSinCierreNoSeToca1991271413/001/AGENTS.md: no such file or directory
          global_instructions_migration_test.go:119: baseline: retired=[/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestMigracionEstricta_BloqueSinCierreNoSeToca1991271413/001/AGENTS.md] errs=[]
          global_instructions_migration_test.go:122: baseline: creó copia sin migrar
          global_instructions_migration_test.go:119: cierre sin inicio: retired=[] errs=[]
          global_instructions_migration_test.go:116: protocolo: archivo alterado: "# Mis reglas\n", <nil>
          global_instructions_migration_test.go:119: protocolo: retired=[/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestMigracionEstricta_BloqueSinCierreNoSeToca1991271413/003/AGENTS.md] errs=[]
          global_instructions_migration_test.go:122: protocolo: creó copia sin migrar
      --- FAIL: TestMigracionEstricta_EncabezadoSinMarcadorEsDeLaPersona (0.01s)
          global_instructions_migration_test.go:132: got="# Mío\n" retired=[/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestMigracionEstricta_EncabezadoSinMarcadorEsDeLaPersona163529284/001/AGENTS.md] errs=[]
      --- FAIL: TestMigracionEstricta_RetiraTodosLosBloques (0.01s)
          global_instructions_migration_test.go:141: got="antes\n\nmedio\n\n<!-- gomemory-protocol-v10 -->\nuno\n<!-- gomemory-protocol-end -->\n\ndespués\n" retired=[/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestMigracionEstricta_RetiraTodosLosBloques2887114205/001/AGENTS.md] errs=[]
      --- FAIL: TestMigracionEstricta_PreservaCRLF (0.01s)
          global_instructions_migration_test.go:152: got="uno\r\n\r\n\n\r\n\r\ndos\r\n" errs=[]
      FAIL
      FAIL	mem/adapters/primary/cli	0.329s
      FAIL
- 2026-10-31 21:20 · tarea 20 · rojo observado · caducada · `go test ./application/usecases/ -run 'TestBuilder_BaselineRespetaElPresupuesto' -count=1` (salida 1)
      --- FAIL: TestBuilder_BaselineRespetaElPresupuesto (0.01s)
          build_baseline_test.go:100: debe omitirse con su puntero: 6092 bytes
              # Memoria del Proyecto
              ## Baseline universal (memoria fijada)
              baseline largo baseline largo baseline largo baseline largo baseline largo baseline largo baseline largo baseline largo baseline largo b
      FAIL
      FAIL	mem/application/usecases	0.180s
      FAIL
- 2026-10-31 21:22 · tarea 1 · verde · caducada · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.197s
- 2026-10-31 21:22 · tarea 2 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.231s
- 2026-10-31 21:22 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.295s
      ok  	mem/tests/integration	5.011s
- 2026-10-31 21:22 · tarea 7 · verde · caducada · `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestRunGlobalScopeSetup|TestInstallAtomicPlanGlobal' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	1.358s
      ok  	mem/adapters/primary/setup	0.151s
- 2026-10-31 21:22 · tarea 8 · verde · caducada · `go test ./domain/ ./adapters/primary/setup/ -run 'Matrix|Activation|Channel|TestC7' -count=1` (salida 0)
      ok  	mem/domain	0.133s
      ok  	mem/adapters/primary/setup	0.153s
- 2026-10-31 21:22 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_ConservaTextoYCopia$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.275s
- 2026-10-31 21:22 · tarea 10 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_(Idempotente|FalloDeCopia)$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.277s
- 2026-10-31 21:22 · tarea 13 · verde · caducada · `! grep -rnE 'composeAgentFile|writeGlobalInstructions' --include='*.go' adapters domain application tests infrastructure` (salida 0)
- 2026-10-31 21:22 · tarea 14 · verde · caducada · `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1` (salida 0)
      ok  	mem/tests/integration	1.208s
- 2026-10-31 21:23 · tarea 15 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.367s
- 2026-10-31 21:23 · tarea 16 · verde · caducada · `go test ./application/usecases/ -run 'TestSessionDelta_ConservaBaseline' -count=1 && go test ./tests/integration/ -run 'TestContextKeepBaseline' -count=1 && node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (1.092083ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.508959ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.176417ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.301291ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.200458ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.179417ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.271458ms)
      ✔ expand: stop sends the last assistant text (0.206958ms)
      ✔ expand: stop without messages sends an empty last message (0.565292ms)
      ✔ expand: post-tool observes bash only (15.974792ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.855833ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.745417ms)
      ℹ tests 62
      ℹ suites 0
      ℹ pass 62
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1871.741917
- 2026-10-31 21:23 · tarea 17 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionEstricta_|TestMigracionGlobal_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.309s
- 2026-10-31 21:23 · tarea 18 · verde · caducada · `go test ./adapters/primary/setup/ -run 'TestGlobalInstructionPaths_|TestActivationInspect_InstruccionesDeUsuarioSoloComoHeredadas|TestActivationInspect_RemedioPorAmbito' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.144s
- 2026-10-31 21:23 · tarea 19 · verde · caducada · `! grep -nE 'last-preference-nudge|Bloque de protocolo de nivel usuario|archivos de instrucciones de \*\*ámbito usuario' docs/MEMORY-PROTOCOL.md docs/AGENT-INTEGRATION.md docs/architecture.md` (salida 0)
- 2026-10-31 21:23 · tarea 20 · verde · caducada · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1 && go test ./tests/contract/ -run 'TestOpenCodeV2_FabricaV1IdenticaALaReferencia' -count=1 && node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (0.88275ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.321292ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.138375ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.261083ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.17075ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.152792ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.235458ms)
      ✔ expand: stop sends the last assistant text (0.186042ms)
      ✔ expand: stop without messages sends an empty last message (0.154333ms)
      ✔ expand: post-tool observes bash only (16.681833ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.538459ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.41925ms)
      ℹ tests 62
      ℹ suites 0
      ℹ pass 62
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1841.150834
- 2026-10-31 21:38 · tarea 1 · verde · vigente · `go test ./domain/ -run 'TestPinnedDocs_IncluyeBaseline$' -count=1` (salida 0)
      ok  	mem/domain	0.486s
- 2026-10-31 21:38 · tarea 2 · verde · vigente · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1` (salida 0)
      ok  	mem/application/usecases	0.224s
- 2026-10-31 21:38 · tarea 3 · verde · vigente · `go test ./tests/contract/ -run 'TestHookBudget' -count=1 && go test ./tests/integration/ -run 'TestSessionDelta20Turns$' -count=1` (salida 0)
      ok  	mem/tests/contract	6.424s
      ok  	mem/tests/integration	4.708s
- 2026-10-31 21:38 · tarea 7 · verde · vigente · `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestRunGlobalScopeSetup|TestInstallAtomicPlanGlobal' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	1.353s
      ok  	mem/adapters/primary/setup	0.167s
- 2026-10-31 21:38 · tarea 8 · verde · vigente · `go test ./domain/ ./adapters/primary/setup/ -run 'Matrix|Activation|Channel|TestC7' -count=1` (salida 0)
      ok  	mem/domain	0.122s
      ok  	mem/adapters/primary/setup	0.156s
- 2026-10-31 21:38 · tarea 9 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_ConservaTextoYCopia$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.278s
- 2026-10-31 21:38 · tarea 10 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_(Idempotente|FalloDeCopia)$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.273s
- 2026-10-31 21:38 · tarea 13 · verde · vigente · `! grep -rnE 'composeAgentFile|writeGlobalInstructions' --include='*.go' adapters domain application tests infrastructure` (salida 0)
- 2026-10-31 21:38 · tarea 14 · verde · vigente · `go test ./tests/integration/ -run 'TestHook_SinRefuerzoDePreferencias$' -count=1` (salida 0)
      ok  	mem/tests/integration	1.186s
- 2026-10-31 21:38 · tarea 15 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.275s
- 2026-10-31 21:38 · tarea 16 · verde · vigente · `go test ./application/usecases/ -run 'TestSessionDelta_ConservaBaseline' -count=1 && go test ./tests/integration/ -run 'TestContextKeepBaseline' -count=1 && node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (0.851542ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.324042ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.139416ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.238958ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.158916ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.14475ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.24725ms)
      ✔ expand: stop sends the last assistant text (0.169583ms)
      ✔ expand: stop without messages sends an empty last message (0.66325ms)
      ✔ expand: post-tool observes bash only (15.667ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.573792ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.152958ms)
      ℹ tests 62
      ℹ suites 0
      ℹ pass 62
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1857.847834
- 2026-10-31 21:38 · tarea 17 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestMigracionEstricta_|TestMigracionGlobal_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.290s
- 2026-10-31 21:38 · tarea 18 · verde · vigente · `go test ./adapters/primary/setup/ -run 'TestGlobalInstructionPaths_|TestActivationInspect_InstruccionesDeUsuarioSoloComoHeredadas' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.145s
- 2026-10-31 21:38 · tarea 19 · verde · vigente · `! grep -nE 'last-preference-nudge|Bloque de protocolo de nivel usuario|archivos de instrucciones de \*\*ámbito usuario' docs/MEMORY-PROTOCOL.md docs/AGENT-INTEGRATION.md docs/architecture.md` (salida 0)
- 2026-10-31 21:38 · tarea 20 · verde · vigente · `go test ./application/usecases/ -run 'TestBuilder_Baseline' -count=1 && go test ./tests/contract/ -run 'TestOpenCodeV2_FabricaV1IdenticaALaReferencia' -count=1 && node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ v2 socratica: the decision reaches the real prompt text (0.868625ms)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.322ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.134875ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.28825ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.200667ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.160584ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.309958ms)
      ✔ expand: stop sends the last assistant text (0.18775ms)
      ✔ expand: stop without messages sends an empty last message (0.171125ms)
      ✔ expand: post-tool observes bash only (16.853917ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.708834ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.336458ms)
      ℹ tests 62
      ℹ suites 0
      ℹ pass 62
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1844.403
- 2026-10-31 21:38 · tarea 21 · verde · vigente · `go test ./adapters/primary/setup/ -run 'TestActivationInspect_RemedioSeguro' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.147s
- 2026-10-31 21:43 · tarea 11 · manual · vigente
      Migración vigente sobre dd5cbb5: los tres archivos globales con cero marcadores y una sola copia; plugin de OpenCode y binario de las 16:39 con keep-baseline en la fábrica común.
- 2026-10-31 21:43 · tarea 4 · manual · vigente
      Sesión real de Claude sobre dd5cbb5, observada por la persona, sin herramientas: baseline por SessionStart; protocolo v11 por MCP cortado en Prioridad de recuperación tras todas las reglas; plan, privacidad y cierre correctos. Autodeclarado; CLAUDE.md conserva texto propio similar sin marcadores.
- 2026-10-31 21:43 · tarea 5 · manual · vigente
      Sesión real de Codex sobre dd5cbb5, observada por la persona: baseline completo en mensaje developer antes de la primera herramienta, comprobado en el rollout; protocolo v11 completo; plan, privacidad y cierre correctos. Llamó a get_context pese a la instrucción.
- 2026-10-31 21:43 · tarea 6 · manual · vigente
      Sesión real de OpenCode 2.2.1 sobre dd5cbb5, observada por la persona: tras varios turnos el baseline sigue completo sin marcador ya entregado y el protocolo llega completo con cierre. El guion de 6 preguntas no se le pasó: recibió por error la respuesta de Claude, y usó herramientas y guardó una memoria; plan, herramientas, privacidad y cierre se observaron en la ronda sobre b71abce.
- 2026-10-31 21:43 · tarea 12 · manual · vigente
      Ronda final sobre dd5cbb5, sin bloques en los archivos globales: Claude y Codex reciben baseline y protocolo v11 por hooks y MCP; OpenCode conserva el baseline entero en turnos sucesivos. Respuestas autodeclaradas; Codex comprobado en su rollout; OpenCode sin el guion completo en esta ronda.

## Siguiente
Cerrada con aprobación sobre la revisión a1a60599cd4d.
