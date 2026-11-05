# Subagentes: aviso mínimo en lugar del protocolo de memoria completo

ID: HCD-026
Tipo: fix
Carril: fast
Motivo: Fallo con reproducción clara (hook inyecta ~6 KB y órdenes de memoria a cada subagente), sin decisiones abiertas

## Intención
Que cada subagente reciba solo un aviso breve de herramientas disponibles, sin bootstrap de ToolSearch, sin protocolo de guardado ni end_session, para reducir tokens en ACR y delegaciones

## Reproducción
`mem hook subagent-start </dev/null` devuelve hoy un `additionalContext` de 5.900 caracteres que, en cada subagente de Claude Code:
- ordena «PRIMERA ACCIÓN — ejecuta este ToolSearch» con ~40 herramientas (incluidas `save_memory`, `forget_memory`, `start_session`, `end_session`, `review_*`, `pack_*`, `octopus_*`);
- repite el protocolo completo de memoria («Guarda proactivamente…», «Antes de cerrar, llama a end_session(summary)»);
- inyecta la política de delegación de Octopus, aunque un subagente de Claude Code no puede crear subagentes.
Efecto observado: los revisores de ACR guardaron memorias propias (memorias 1693, 1694) y cada revisor paga ese bootstrap y los esquemas que carga, en cada ronda.

## Alcance
Incluido: el contenido que emite `hookSubagentStart` (`adapters/primary/cli/cmd_hook.go`) y sus pruebas (`tests/integration/hook_marker_integration_test.go`). El subagente conserva el aviso de que existen herramientas de grafo de código, motivo original del hook (que Explore no recurra a grep).
Por petición de la persona, aplica a los subagentes de los cuatro runtimes (Claude Code, Codex, OpenCode, agy), con una única fuente del aviso en Go:
- Claude Code: hook `SubagentStart` → `hookSubagentStart`.
- OpenCode: `experimental.chat.system.transform` (`infrastructure/plugin/opencode/gomemory.ts`) inyecta hoy en TODA sesión, hija incluida, el protocolo completo + `mem context` entero + Octopus + nudge en cada turno; la sesión hija (con `parentID`) debe recibir solo el aviso mínimo.
- Codex y agy: no se ha verificado si sus subagentes (`spawn_agent`, `invoke_subagent`) disparan `SessionStart`/`UserPromptSubmit`/`PreInvocation` ni si el payload los distingue (ver Q2).
Excluido: el bootstrap del agente principal (`user-prompt-submit`, `session-start`); la revisión acumulativa de ACR y las lentes deterministas (unidad posterior); los archivos estáticos de protocolo (AGENTS.md, GEMINI.md), que leen igual el agente raíz y sus subagentes.

## ¿Qué pasa si no lo hacemos?
Cada subagente (Explore, web-fetch, revisores A/B de ACR) sigue pagando ~1.500 tokens de inyección más los esquemas de ~40 herramientas, y recibe órdenes de escribir memoria y de cerrar sesión que solo corresponden al agente principal: ruido en la memoria y riesgo de cerrar la sesión activa.

## Alternativa más barata
Desactivar el hook `SubagentStart` en la configuración. Se descarta: el subagente volvería a ignorar el grafo de código y a explorar con grep, que es el fallo que el hook corrigió.


## Aclaraciones
- [x] Q1. ¿Qué herramientas precarga el aviso mínimo del subagente? → Solo lectura (Recommended): grafo de código (codebase-memory-mcp, search_code/get_symbol de gomemory) más search_memories/get_memory; sin save/forget/session/review/pack/octopus
- [x] Q2. Codex y agy: ¿se verifican ahora en sesión real si sus subagentes disparan hooks, o quedan para una unidad posterior? → Sondear ahora (Recommended): una sesión real corta en Codex y otra en agy con un subagente trivial para registrar qué hooks disparan y su payload; los cuatro runtimes quedan en esta unidad
- [x] Q3. ¿Impedir que los subagentes carguen AGENTS.md/CLAUDE.md entra en esta unidad, y con qué mecanismo por runtime? → Unidad aparte (Recommended): HCD-026 cierra solo la fuga que controla gomemory (aviso mínimo en los cuatro runtimes); excluir AGENTS.md/CLAUDE.md de los subagentes se investiga por runtime en otra unidad sin quitárselos al agente raíz

## Checklist
- [x] 1. El aviso del subagente no ordena escribir memoria, gestionar la sesión ni delegar (sin save_memory, forget_memory, judge_memories, start_session, end_session, save_session_summary, review_*, pack_*, octopus_*, «PRIMERA ACCIÓN») → `go test ./adapters/primary/cli -run TestSubagentNotice_SinEscrituraNiSesion`
- [x] 2. El aviso precarga solo lectura: search_memories, get_memory, search_code, get_symbol, list_dependencies y las tools de descubrimiento de codebase-memory-mcp (si el proveedor está activo) → `go test ./adapters/primary/cli -run TestSubagentNotice_PrecargaSoloLectura`
- [x] 3. El aviso cabe en 1.200 caracteres (hoy 5.900) → `go test ./adapters/primary/cli -run TestSubagentNotice_Acotado`
- [x] 4. El hook real del binario emite el aviso mínimo y, con Octopus encendido, solo el agente raíz recibe la política de delegación → `go test ./tests/integration -run 'TestHookSubagentStart|TestHookOctopusEncendido'`
- [x] 6. agy: tras `PreToolUse(invoke_subagent)` del raíz, el `SessionStart` con prompt vacío se clasifica como subagente y solo emite el aviso mínimo (sin `session-start` ni bootstrap en `PreInvocation`); sin marcador o con marcador caducado se trata como raíz → `go test ./adapters/primary/cli -run TestAgySubagent`
- [x] 7. OpenCode: una sesión con `parentID` recibe en `system.transform` solo el aviso mínimo (sin protocolo, sin `mem context`, sin Octopus ni nudge) → `node --test infrastructure/plugin/opencode/gomemory.test.mjs`
- [x] 8. Codex: el sondeo real muestra que el subagente no dispara hooks de arranque; gomemory no le inyecta nada y lo que hereda del raíz pasa a la unidad de Q3 → manual (registro del sondeo, memoria 1719)
- [x] 5. Sin regresiones en los paquetes tocados → `go test ./adapters/primary/cli/... ./tests/integration/... ./tests/contract/...`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 05:09 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestSubagentNotice_SinEscrituraNiSesion` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/cmd_hook_subagent_test.go:17:13: undefined: subagentNotice
      adapters/primary/cli/cmd_hook_subagent_test.go:41:12: undefined: subagentNotice
      adapters/primary/cli/cmd_hook_subagent_test.go:55:12: undefined: subagentNotice
      adapters/primary/cli/cmd_hook_subagent_test.go:62:33: undefined: subagentNotice
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-05 05:17 -05:00 · tarea 6 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestAgySubagent` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hook_agy_subagent_test.go:51:5: undefined: classifyAgySubagent
      adapters/primary/cli/hook_agy_subagent_test.go:56:6: undefined: classifyAgySubagent
      adapters/primary/cli/hook_agy_subagent_test.go:59:19: undefined: planAgyHookFor
      adapters/primary/cli/hook_agy_subagent_test.go:65:6: undefined: classifyAgySubagent
      adapters/primary/cli/hook_agy_subagent_test.go:68:22: undefined: planAgyHookFor
      adapters/primary/cli/hook_agy_subagent_test.go:76:5: undefined: classifyAgySubagent
      adapters/primary/cli/hook_agy_subagent_test.go:79:19: undefined: planAgyHookFor
      adapters/primary/cli/hook_agy_subagent_test.go:90:2: undefined: classifyAgySubagent
      adapters/primary/cli/hook_agy_subagent_test.go:92:5: undefined: classifyAgySubagent
      adapters/primary/cli/hook_agy_subagent_test.go:92:56: undefined: agySubagentWindow
      adapters/primary/cli/hook_agy_subagent_test.go:92:56: too many errors
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-05 05:19 -05:00 · tarea 7 · rojo observado · caducada · `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 1)
        gomemory_review_start INPUT: provide target_type, revision, and digest. Omit scope
        unless the review must be narrowed. If scope is needed, it is a JSON array of
        repository-relative paths — use scope: ["."] for the repository root, never
        scope: . or any other bare path.
        SESSION CLOSE: before saying "done", call gomemory_end_session(summary) with Goal /
        Discoveries / Accomplished / Next Steps / Relevant Files.
        ## Project Memory (gomemory)
        CONTEXTO-COMPLETO
            at TestContext.<anonymous> (file:///Users/josegomezj/home/rcw/gomemory/infrastructure/plugin/opencode/gomemory.test.mjs:868:10)
            at async Test.run (node:internal/test_runner/test:1409:7)
            at async Test.processPendingSubtests (node:internal/test_runner/test:974:7) {
          generatedMessage: false,
          code: 'ERR_ASSERTION',
          actual: false,
          expected: true,
          operator: '==',
          diff: 'simple'
        }
- 2026-11-05 05:24 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run TestSubagentNotice_SinEscrituraNiSesion` (salida 0)
      ok  	mem/adapters/primary/cli	0.282s
- 2026-11-05 05:24 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/primary/cli -run TestSubagentNotice_PrecargaSoloLectura` (salida 0)
      ok  	mem/adapters/primary/cli	0.246s
- 2026-11-05 05:24 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli -run TestSubagentNotice_Acotado` (salida 0)
      ok  	mem/adapters/primary/cli	0.287s
- 2026-11-05 05:24 -05:00 · tarea 4 · verde · vigente · `go test ./tests/integration -run 'TestHookSubagentStart|TestHookOctopusEncendido'` (salida 0)
      ok  	mem/tests/integration	1.252s
- 2026-11-05 05:24 -05:00 · tarea 6 · verde · vigente · `go test ./adapters/primary/cli -run TestAgySubagent` (salida 0)
      ok  	mem/adapters/primary/cli	0.294s
- 2026-11-05 05:24 -05:00 · tarea 7 · verde · vigente · `node --test infrastructure/plugin/opencode/gomemory.test.mjs` (salida 0)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.66175ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.242125ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.369167ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.227125ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.199209ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.310292ms)
      ✔ expand: stop sends the last assistant text (0.231084ms)
      ✔ expand: stop without messages sends an empty last message (0.1065ms)
      ✔ expand: post-tool observes bash only (16.542625ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.965042ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.656291ms)
      ✔ child session (parentID) gets only the minimal subagent notice (0.415125ms)
      ℹ tests 67
      ℹ suites 0
      ℹ pass 67
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 2028.768708
- 2026-11-05 05:24 -05:00 · tarea 8 · manual · vigente
      Sondeo real codex-cli 0.166.0 (codex exec) con shim de mem en PATH, proyecto desechable: al crear el subagente con spawn_agent solo llegaron 'expand pre-tool' (raíz) y 'subagent-stop'; ningún session-start/user-prompt-submit del hijo. gomemory no inyecta nada al subagente de Codex; lo que hereda del raíz (AGENTS.md, protocolo) pasa a la unidad de Q3. Memoria 1719.
- 2026-11-05 05:24 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli/... ./tests/integration/... ./tests/contract/...` (salida 0)
      ok  	mem/adapters/primary/cli	17.659s
      ok  	mem/adapters/primary/cli/clikit	(cached)
      ok  	mem/adapters/primary/cli/hcd	(cached)
      ok  	mem/adapters/primary/cli/review	(cached)
      ok  	mem/adapters/primary/cli/socratica	(cached)
      ok  	mem/tests/integration	38.070s
      ok  	mem/tests/contract	89.860s

## Siguiente
Cerrada con aprobación sobre la revisión 99946e6dd1e6.
