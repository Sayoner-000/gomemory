# Antigravity en setup-mcp global y en la política de recuperación; hora local en la evidencia HCD

ID: HCD-021
Tipo: fix
Carril: fast
Motivo: Tres fallos con reproducción clara y sin decisiones abiertas: la hora local del host ya la pidió la persona

## Intención
Que mem setup-mcp --scope global registre agy por defecto, que la política de recuperación oriente a agy hacia las tools MCP de gomemory que llegan comprimidas, y que la evidencia HCD muestre la hora local del host con su desfase

## Reproducción
1. `mem setup-mcp --scope global` (sin `--agents`) registra opencode, claude y codex, pero no agy: `defaultAgentList = "opencode,claude,codex"` en `adapters/primary/cli/cmd_mcp_setup.go:56`, aunque `globalScopeAgents` sí declara `agy`. Salida real de la persona con gomemory 2.44.4: ninguna línea `agy`.
2. `domain.CodeRetrievalPolicy` (`domain/code_retrieval_policy.go`) solo nombra a Codex entre los agentes sin compresión de resultados externos; agy 1.4.12 tampoco la tiene (PostToolUse sin salida) y la política no le dice qué tools de gomemory llegan comprimidas en origen.
3. La sección Evidencia de los documentos HCD muestra la hora en UTC sin zona: `application/usecases/hcd.go:502` usa `e.CreatedAt.UTC().Format("2006-01-02 15:04")`; en un host UTC-5 un registro de las 19:42 aparece como `00:42` del día siguiente.

## Alcance
Incluido: añadir `agy` a `defaultAgentList` (y a la ayuda de `--agents`), sin crear `~/.gemini/config` en máquinas sin agy, con la invariante del test de agentes por defecto ampliada; nombrar a Antigravity en `domain.CodeRetrievalPolicy` y precisar que `get_context`, `search_memories` y `search_code` llegan comprimidas en origen y que `get_symbol` es compacta sin compresión, con prueba; mostrar la hora de la evidencia HCD en la zona local del host con desfase (`2006-01-02 15:04 -07:00`), con prueba que fija la zona.
Excluido: cambiar cómo se guardan las horas (siguen en UTC en la base); comprimir `get_symbol`; cambiar el registro por proyecto o `mem install`; tocar otros formatos de fecha fuera de la Evidencia HCD.

## ¿Qué pasa si no lo hacemos?
Quien registra gomemory en global sin `--agents` deja agy sin MCP ni hooks y nada se lo advierte; agy sigue leyendo archivos y haciendo grep sin compresión aunque gomemory ofrece tools que sí comprimen; y la evidencia HCD confunde fechas y horas para quien no trabaja en UTC.

## Alternativa más barata
Pasar siempre `--agents opencode,claude,codex,agy`, recordar a mano qué tools comprimen y restar el desfase al leer la evidencia. Depende de la memoria de la persona y no protege a nadie más.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. `defaultAgentList` incluye agy y sigue contenido en `globalScopeAgents` → `go test ./adapters/primary/cli -run TestAgentesPorDefecto -count=1`
- [x] 2. `setupAgyGlobal` no crea `~/.gemini/config` si agy no está instalado (ni binario ni carpeta) y lo informa → `go test ./adapters/primary/cli -run TestSetupAgyGlobal_SinAgyNoCreaConfig -count=1`
- [x] 3. `CodeRetrievalPolicy` nombra a Antigravity y distingue las tools comprimidas en origen de `get_symbol` → `go test ./domain ./adapters/primary/cli -run 'TestCodeRetrievalPolicy' -count=1`
- [x] 4. La Evidencia HCD muestra la hora local del host con desfase → `go test ./application/usecases -run TestHCDEvidence_HoraLocal -count=1`
- [x] 5. `mem setup-mcp --scope global` sin `--agents` registra agy en un HOME temporal con agy → evidencia manual
- [x] 6. Verificación completa → `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-03 21:05 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestAgentesPorDefecto -count=1` (salida 1)
      --- FAIL: TestAgentesPorDefecto_IncluyenCodex (0.00s)
          hook_prompt_dialect_test.go:145: el valor por defecto de --agents ("opencode,claude,codex") no incluye "agy"
      FAIL
      FAIL	mem/adapters/primary/cli	0.586s
      FAIL
- 2026-11-03 21:05 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run TestAgentesPorDefecto -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.359s
- 2026-11-03 21:05 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestSetupAgyGlobal_SinAgyNoCreaConfig -count=1` (salida 1)
        ✅ agy: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupAgyGlobal_SinAgyNoCreaConfig1628094236/001/.gemini/config/mcp_config.json
        ✅ agy: /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestSetupAgyGlobal_SinAgyNoCreaConfig1628094236/001/.gemini/config/hooks.json
      --- FAIL: TestSetupAgyGlobal_SinAgyNoCreaConfig (0.00s)
          hcd021_agy_global_test.go:17: sin agy instalado no hay registro que informar
      FAIL
      FAIL	mem/adapters/primary/cli	0.305s
      FAIL
- 2026-11-03 21:05 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestSetupAgyGlobal_SinAgyNoCreaConfig -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.426s
- 2026-11-03 21:06 -05:00 · tarea 3 · rojo observado · caducada · `go test ./domain ./adapters/primary/cli -run 'TestCodeRetrievalPolicy' -count=1` (salida 1)
      --- FAIL: TestCodeRetrievalPolicy_Antigravity (0.00s)
          code_retrieval_policy_test.go:14: la política no menciona "Antigravity"
          code_retrieval_policy_test.go:14: la política no menciona "get_context"
          code_retrieval_policy_test.go:14: la política no menciona "search_memories"
          code_retrieval_policy_test.go:18: la política debe aclarar que get_symbol no se comprime
      FAIL
      FAIL	mem/domain	0.179s
      ok  	mem/adapters/primary/cli	0.323s [no tests to run]
      FAIL
- 2026-11-03 21:06 -05:00 · tarea 3 · verde · caducada · `go test ./domain ./adapters/primary/cli -run 'TestCodeRetrievalPolicy' -count=1` (salida 0)
      ok  	mem/domain	0.143s
      ok  	mem/adapters/primary/cli	0.274s [no tests to run]
- 2026-11-03 21:07 -05:00 · tarea 4 · rojo observado · caducada · `go test ./application/usecases -run TestHCDEvidence_HoraLocal -count=1` (salida 1)
      --- FAIL: TestHCDEvidence_HoraLocal (0.00s)
          hcd021_evidence_time_test.go:20: la hora debe ir en la zona local con su desfase:
              <!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
              - 2026-11-04 00:42 · tarea 1 · manual · vigente
      FAIL
      FAIL	mem/application/usecases	0.266s
      FAIL
- 2026-11-03 21:07 -05:00 · tarea 4 · verde · vigente · `go test ./application/usecases -run TestHCDEvidence_HoraLocal -count=1` (salida 0)
      ok  	mem/application/usecases	0.203s
- 2026-11-03 21:07 -05:00 · tarea 5 · manual · vigente
      Binario de este árbol (HCD-021), HOME temporal. Con agy (binario en PATH y ~/.gemini/config): 'mem setup-mcp --scope global' sin --agents imprime '✅ agy: …/.gemini/config/mcp_config.json' y '…/hooks.json'; mcpServers = [gomemory]; 3 registros. Sin agy (PATH sin agy, HOME vacío): imprime 'ℹ️ agy: no instalado (ni agy en el PATH ni ~/.gemini/config); se omite' y no crea ~/.gemini (HOME queda con .claude .codex .config de los otros agentes).
- 2026-11-03 21:07 -05:00 · tarea 6 · verde · vigente · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh` (salida 0)
      ok  	mem/adapters/secondary/expandstate	0.352s
      ok  	mem/adapters/secondary/generator	1.260s
      ok  	mem/adapters/secondary/hcdfiles	0.925s
      ok  	mem/adapters/secondary/launcher	0.677s
      ok  	mem/adapters/secondary/persistence	31.394s
      ok  	mem/adapters/secondary/release	0.401s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.232s
      ok  	mem/adapters/secondary/speckit	0.201s
      ok  	mem/adapters/secondary/tokens	0.156s
      ok  	mem/adapters/secondary/usage	0.155s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	18.060s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.395s
      ok  	mem/infrastructure	0.634s
      ok  	mem/tests/contract	99.266s
      ok  	mem/tests/integration	45.475s
      ?   	mem/version	[no test files]
      OK: --uninstall solo retira binarios propios
- 2026-11-03 21:09 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run TestAgentesPorDefecto -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.298s
- 2026-11-03 21:09 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/primary/cli -run TestSetupAgyGlobal_SinAgyNoCreaConfig -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.280s
- 2026-11-03 21:09 -05:00 · tarea 3 · verde · vigente · `go test ./domain ./adapters/primary/cli -run 'TestCodeRetrievalPolicy' -count=1` (salida 0)
      ok  	mem/domain	0.198s
      ok  	mem/adapters/primary/cli	0.250s [no tests to run]

## Siguiente
Cerrada con aprobación sobre la revisión c28d085023b4.
