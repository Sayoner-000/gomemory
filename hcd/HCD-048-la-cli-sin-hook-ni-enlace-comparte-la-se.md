# La CLI sin hook ni enlace comparte la sesión común del proyecto (C-001)

ID: HCD-048
Tipo: fix
Carril: full
Motivo: La persona pidió ceremonia completa: revierte una decisión aprobada (HCD-036 Q2) y toca la identidad de sesión que comparten CLI, MCP y hooks

## Intención
Que mem session start y el resto de la CLI/TUI, sin id de hook ni enlace del host, usen la misma sesión del proyecto que el MCP y los hooks, de modo que las entregas se supriman y post-compact restablezca el estado también cuando hay TTY; la identidad de terminal deja de deducirse del TTY y solo se fija por entorno

## Reproducción
Con un TTY controlador (macOS, pseudo-terminal `ttys017`):
`go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1 -v` dentro de un pty.
Falla `tests/integration/plan_context_suppression_max_test.go:63` («get_plan_context debería suprimir el historial ya entregado por mcp/hook») y `tests/integration/session_delta_test.go:52/55` («tras post-compact no debe quedar nada marcado como ya entregado»). Sin TTY (`</dev/null`, CI) las dos pasan.
Causa: `infrastructure/container.go` `conversationFor` da a la CLI/TUI `hostbind.TerminalID()` («terminal-ttysNNN») como id local; `mem session start` abre la sesión con ese `conversation_id` y el MCP y los hooks sin id, que buscan `conversation_id IS NULL`, no la ven: no anotan entregas y post-compact no restablece nada.

## Alcance
Incluido: `conversationFor` deja de usar el TTY; la CLI/TUI sin id de hook ni enlace del host usa la sesión común del proyecto (`conversation_id IS NULL`), igual que el MCP y los hooks; la identidad propia de terminal solo se fija con `GOMEMORY_TERMINAL` (id «terminal-<valor>», Q1); `hostbind.TerminalID` deja de consultar `ps` y lee esa variable, y sus pruebas se ajustan; la verificación final corre con y sin TTY.
Excluido: el enlace del host y el id de hook de HCD-036 (Q1) no cambian; el flag `--conversation=` de `mem session` no cambia; no se migran las sesiones ya abiertas con `terminal-…` (las cierra `closeStaleSessions` por inactividad); C-002 va en otra unidad.

## ¿Qué pasa si no lo hacemos?
Cualquier persona que use `mem` desde su terminal deja al MCP y a los hooks sin sesión: `get_plan_context` reenvía el historial completo en cada turno (miles de tokens) y post-compact no restablece las entregas. La suite de integración falla en cualquier máquina con TTY, aunque CI pase.

## Alternativa más barata
Forzar en las pruebas un entorno sin TTY tapa el rojo pero deja el fallo real para la persona: se descarta. Cambiar solo `cmd_session start` dejaría al resto de la CLI/TUI en otra sesión: también se descarta. La opción elegida es el cambio mínimo en el único punto de origen (`conversationFor`).

## Historias
### US1. La terminal y el agente comparten la sesión del proyecto
Como persona que usa `mem` en su terminal mientras su agente trabaja, quiero que la CLI use la misma sesión del proyecto que el MCP y los hooks, para que el contexto ya entregado no se reenvíe ni se pierda tras compactar.
- C-01: Dado un proceso con TTY controlador y sin hook ni enlace del host, cuando ejecuto `TestPlanContextSuppression_Max` y `TestSessionDelta20Turns`, entonces pasan igual que sin TTY.
- C-02: Dado el canal `cli` o `tui` sin variable de entorno de terminal, cuando `conversationFor` resuelve la conversación, entonces `Current()` devuelve "" con o sin TTY.

### US2. Identidad de terminal explícita
Como persona que quiere una sesión aislada en un terminal, quiero fijarla por una variable de entorno, para no depender de qué TTY tenga el proceso.
- C-03: Dado el canal `cli` o `tui` con la variable de entorno fijada, cuando `conversationFor` resuelve la conversación, entonces `Current()` devuelve el id derivado de esa variable; y los canales `mcp` y `hook` la ignoran.

## Aclaraciones
- [x] Q1. ¿Cómo se llama y qué formato tiene la variable de entorno que fija la identidad de terminal de la CLI/TUI? → GOMEMORY_TERMINAL (Recommended): valor libre; el id queda «terminal-<valor>»; vacía o ausente = sesión común del proyecto; los canales mcp y hook la ignoran.

## Slices
<!-- ### VS-01. comportamiento entregable
Tareas: 1, 2
Criterios: C-01 -->
### VS-01. La CLI/TUI comparte la sesión del proyecto salvo GOMEMORY_TERMINAL
Tareas: 1, 2, 3, 4, 5
Criterios: C-01, C-02, C-03

## Checklist
- [x] 1. La reproducción ya no falla → `TestPlanContextSuppression_Max` y `TestSessionDelta20Turns` pasan sin TTY (`</dev/null`) y dentro de un pseudo-terminal (`python3 -I scripts/with-tty.py`)
- [x] 2. Derivar `hostbind.TerminalID` solo de `GOMEMORY_TERMINAL` → `go test ./adapters/secondary/hostbind -run TestTerminalID -count=1`
- [x] 3. Limitar `GOMEMORY_TERMINAL` a los canales `cli` y `tui` en `conversationFor` → `go test ./infrastructure -run TestConversationFor -count=1`
- [x] 4. Comprobar la suite completa sin TTY, `go vet` y la compilación de Windows → `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 </dev/null`
- [x] 5. Comprobar la suite completa con TTY → `python3 -I scripts/with-tty.py go test ./... -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-07 05:06 -05:00 · tarea 1 · rojo observado · caducada · `sh -c go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1 </dev/null && python3 -I -c 'import os,pty,sys; sys.exit(os.waitstatus_to_exitcode(pty.spawn(sys.argv[1:])))' go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1` (salida 2)
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
- 2026-11-07 05:14 -05:00 · tarea 1 · rojo observado · caducada · `go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1 </dev/null && python3 -I scripts/with-tty.py go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1` (salida 1)
      ok  	mem/tests/integration	12.538s
      --- FAIL: TestPlanContextSuppression_Max (3.04s)
          --- FAIL: TestPlanContextSuppression_Max/max/mcp (0.35s)
              plan_context_suppression_max_test.go:63: get_plan_context debería suprimir el historial ya entregado por mcp con max
          --- FAIL: TestPlanContextSuppression_Max/max/hook (0.31s)
              plan_context_suppression_max_test.go:63: get_plan_context debería suprimir el historial ya entregado por hook con max
          --- FAIL: TestPlanContextSuppression_Max/structural/mcp (0.35s)
              plan_context_suppression_max_test.go:63: get_plan_context debería suprimir el historial ya entregado por mcp con structural
      --- FAIL: TestSessionDelta20Turns (8.41s)
          session_delta_test.go:38: turnos 2-20: structural=194494 max=24223 → -87.5%
          session_delta_test.go:52: tras post-compact no debe quedar nada marcado como ya entregado
          session_delta_test.go:55: tras compactar, el método de planificación debe volver completo
      FAIL
      FAIL	mem/tests/integration	11.702s
      FAIL
- 2026-11-07 05:15 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/secondary/hostbind -run TestTerminalID -count=1` (salida 1)
      # mem/adapters/secondary/hostbind [mem/adapters/secondary/hostbind.test]
      adapters/secondary/hostbind/terminal_test.go:18:12: undefined: TerminalEnv
      adapters/secondary/hostbind/terminal_test.go:20:56: undefined: TerminalEnv
      FAIL	mem/adapters/secondary/hostbind [build failed]
      FAIL
- 2026-11-07 05:15 -05:00 · tarea 3 · rojo observado · caducada · `go test ./infrastructure -run TestConversationFor -count=1` (salida 1)
      # mem/infrastructure [mem/infrastructure.test]
      infrastructure/container_terminal_test.go:21:20: undefined: hostbind.TerminalEnv
      infrastructure/container_terminal_test.go:24:90: undefined: hostbind.TerminalEnv
      infrastructure/container_terminal_test.go:28:20: undefined: hostbind.TerminalEnv
      infrastructure/container_terminal_test.go:31:75: undefined: hostbind.TerminalEnv
      FAIL	mem/infrastructure [build failed]
      FAIL
- 2026-11-07 05:15 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/secondary/hostbind -run TestTerminalID -count=1` (salida 1)
      --- FAIL: TestTerminalID (0.01s)
          terminal_test.go:20: TerminalID() con GOMEMORY_TERMINAL="pane-1" = ""; quería "terminal-pane-1"
          terminal_test.go:20: TerminalID() con GOMEMORY_TERMINAL=" pane-1 " = ""; quería "terminal-pane-1"
      FAIL
      FAIL	mem/adapters/secondary/hostbind	0.155s
      FAIL
- 2026-11-07 05:15 -05:00 · tarea 3 · rojo observado · caducada · `go test ./infrastructure -run TestConversationFor -count=1` (salida 1)
      --- FAIL: TestConversationFor (0.19s)
          container_terminal_test.go:31: C-03: canal cli con GOMEMORY_TERMINAL=pane-1 = ""; quería "terminal-pane-1"
          container_terminal_test.go:31: C-03: canal tui con GOMEMORY_TERMINAL=pane-1 = ""; quería "terminal-pane-1"
      FAIL
      FAIL	mem/infrastructure	0.445s
      FAIL
- 2026-11-07 05:15 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/hostbind -count=1` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.175s
- 2026-11-07 05:15 -05:00 · tarea 3 · verde · caducada · `go test ./infrastructure -count=1` (salida 0)
      ok  	mem/infrastructure	0.523s
- 2026-11-07 05:15 -05:00 · tarea 1 · verde · caducada · `go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1 </dev/null && python3 -I scripts/with-tty.py go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1` (salida 0)
      ok  	mem/tests/integration	9.714s
      ok  	mem/tests/integration	9.760s
- 2026-11-07 05:16 -05:00 · tarea 4 · verde · caducada · `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 </dev/null` (salida 0)
      ok  	mem/adapters/secondary/generator	1.125s
      ok  	mem/adapters/secondary/gitdiff	0.692s
      ok  	mem/adapters/secondary/hcdfiles	3.390s
      ok  	mem/adapters/secondary/hostbind	0.123s
      ok  	mem/adapters/secondary/launcher	0.631s
      ok  	mem/adapters/secondary/persistence	32.392s
      ok  	mem/adapters/secondary/release	0.456s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.199s
      ok  	mem/adapters/secondary/speckit	0.127s
      ok  	mem/adapters/secondary/tokens	0.121s
      ok  	mem/adapters/secondary/usage	0.121s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.551s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.259s
      ok  	mem/infrastructure	0.802s
      ok  	mem/tests/contract	95.023s
      ok  	mem/tests/integration	47.460s
      ?   	mem/version	[no test files]
- 2026-11-07 05:17 -05:00 · tarea 5 · verde · caducada · `python3 -I scripts/with-tty.py go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/generator	1.127s
      ok  	mem/adapters/secondary/gitdiff	0.604s
      ok  	mem/adapters/secondary/hcdfiles	3.129s
      ok  	mem/adapters/secondary/hostbind	0.144s
      ok  	mem/adapters/secondary/launcher	0.586s
      ok  	mem/adapters/secondary/persistence	34.417s
      ok  	mem/adapters/secondary/release	0.331s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.167s
      ok  	mem/adapters/secondary/speckit	0.113s
      ok  	mem/adapters/secondary/tokens	0.090s
      ok  	mem/adapters/secondary/usage	0.095s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.878s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.226s
      ok  	mem/infrastructure	0.898s
      ok  	mem/tests/contract	96.578s
      ok  	mem/tests/integration	47.860s
      ?   	mem/version	[no test files]
- 2026-11-07 05:20 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/secondary/hostbind -count=1` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.323s
- 2026-11-07 05:20 -05:00 · tarea 3 · verde · vigente · `go test ./infrastructure -count=1` (salida 0)
      ok  	mem/infrastructure	0.528s
- 2026-11-07 05:21 -05:00 · tarea 1 · verde · vigente · `go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1 </dev/null && python3 -I scripts/with-tty.py go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -count=1` (salida 0)
      ok  	mem/tests/integration	9.876s
      ok  	mem/tests/integration	10.002s
- 2026-11-07 05:21 -05:00 · tarea 4 · verde · vigente · `go vet ./... && GOOS=windows go build ./... && go test ./... -count=1 </dev/null` (salida 0)
      ok  	mem/adapters/secondary/generator	1.160s
      ok  	mem/adapters/secondary/gitdiff	0.582s
      ok  	mem/adapters/secondary/hcdfiles	3.188s
      ok  	mem/adapters/secondary/hostbind	0.134s
      ok  	mem/adapters/secondary/launcher	0.548s
      ok  	mem/adapters/secondary/persistence	33.131s
      ok  	mem/adapters/secondary/release	0.367s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.186s
      ok  	mem/adapters/secondary/speckit	0.128s
      ok  	mem/adapters/secondary/tokens	0.110s
      ok  	mem/adapters/secondary/usage	0.108s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	15.139s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.238s
      ok  	mem/infrastructure	0.803s
      ok  	mem/tests/contract	95.767s
      ok  	mem/tests/integration	48.681s
      ?   	mem/version	[no test files]
- 2026-11-07 05:23 -05:00 · tarea 5 · verde · vigente · `python3 -I scripts/with-tty.py go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/generator	1.167s
      ok  	mem/adapters/secondary/gitdiff	0.930s
      ok  	mem/adapters/secondary/hcdfiles	3.317s
      ok  	mem/adapters/secondary/hostbind	0.157s
      ok  	mem/adapters/secondary/launcher	0.577s
      ok  	mem/adapters/secondary/persistence	35.021s
      ok  	mem/adapters/secondary/release	0.377s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.176s
      ok  	mem/adapters/secondary/speckit	0.137s
      ok  	mem/adapters/secondary/tokens	0.101s
      ok  	mem/adapters/secondary/usage	0.107s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	18.844s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.279s
      ok  	mem/infrastructure	1.503s
      ok  	mem/tests/contract	103.106s
      ok  	mem/tests/integration	56.405s
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión afde9fb70b0a.
