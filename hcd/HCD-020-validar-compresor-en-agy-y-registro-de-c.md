# Validar compresor en Agy y registro de codegraph y codebase-memory-mcp en mcp_config.json de Agy

ID: HCD-020
Tipo: fix
Carril: fast
Motivo: Fix con reproducción clara; la única decisión (cbm solo se diagnostica) ya la tomó la persona

## Intención
Demostrar en sesión real que la compresión propia del MCP funciona con agy, y que mem install/mem update dejan o diagnostican codegraph y codebase-memory-mcp en ~/.gemini/config/mcp_config.json

## Reproducción
1. G1 · codegraph: con un HOME cuyo `~/.gemini/config/mcp_config.json` no tiene `mcpServers.codegraph` (y Claude/Codex/OpenCode sí lo tienen), `ensureCodeGraphMCP` (`adapters/primary/cli/codegraph_mcp.go`) no detecta la falta porque `codeGraphMCPState`, `codeGraphMissing` y `codeGraphRegistered` solo revisan claude/codex/opencode: `mem install --agents agy` o `mem update` terminan sin codegraph en Agy y `mem doctor` («CodeGraph MCP») no lista agy.
2. G2 · codebase-memory-mcp: gomemory no comprueba si `mcpServers.codebase-memory-mcp` está en el `mcp_config.json` de Agy; si falta, `mem doctor` no avisa ni da el comando manual.
3. Compresor: no hay evidencia de punta a punta de que la compresión propia del MCP (`own_output`) llegue comprimida y recuperable a una sesión real de agy 1.4.12; solo hay eventos `agy budget_trimmed` en `mem doctor`.

## Alcance
Incluido: añadir agy (`~/.gemini/config/mcp_config.json`, `mcpServers.codegraph`) a `codeGraphMCPState`, `codeGraphMissing`, `codeGraphRegistered` y `printDoctorCodeGraphMCP`, con pruebas; que `ensureCodeGraphMCP` y su comando manual usen `--target` explícito con solo los agentes faltantes que gomemory gestiona (claude, codex, opencode, antigravity) en vez de `--target auto`, para no crear `~/.gemini/GEMINI.md` ni `~/.gemini/settings.json` (Q1), con pruebas; un aviso de `mem doctor` cuando falte `codebase-memory-mcp` en el `mcp_config.json` de Agy y su binario esté en PATH, con el comando manual `codebase-memory-mcp install --clients=antigravity` y la nota de que ese instalador escribe `~/.gemini/GEMINI.md`, con pruebas; evidencia real en agy 1.4.12 de compresión propia del MCP con ref ⟦mem⟧ recuperable; validación en HOME temporal de `mem install --agents agy` y del camino de `mem update` (conserva entradas ajenas, idempotente); comprobar que agy resuelve `"command":"mem"` relativo al PATH.
Excluido: que gomemory escriba la entrada de codebase-memory-mcp o lance su instalador (decisión de la persona: solo diagnosticar); escribir `GEMINI.md`; cambiar el motor de compresión, el hook de salidas o el esquema de `hooks.json`; tocar el `~/.gemini/config` real salvo lectura.

## ¿Qué pasa si no lo hacemos?
Una instalación desde cero o un `mem update` puede dejar a Agy sin codegraph o sin codebase-memory-mcp sin que nadie lo note: el protocolo le pide usar el grafo y el agente cae a lecturas manuales. Además, seguiríamos afirmando que la compresión funciona en Agy sin haberlo demostrado en una sesión real.

## Alternativa más barata
Ejecutar a mano `codegraph install --target auto --location global --yes` y `codebase-memory-mcp install` tras cada instalación y revisar el JSON. Funciona hoy en esta máquina, pero no protege instalaciones nuevas, depende de recordarlo, y el instalador de cbm escribe `GEMINI.md`, contra la preferencia registrada.


## Aclaraciones
- [x] Q1. ensureCodeGraphMCP lanza 'codegraph install --target auto', que además del MCP de agy crea ~/.gemini/GEMINI.md y ~/.gemini/settings.json (target Gemini CLI), contra la preferencia [1543]; el arreglo de G1 lo dispara también cuando solo falta en agy. ¿Cómo debe invocar gomemory a codegraph? → Targets explícitos (Recommended)

## Checklist
- [x] 1. G1: `codeGraphMCPState` lee el `mcp_config.json` de agy y `codeGraphMissing`/`codeGraphRegistered` incluyen agy → `go test ./adapters/primary/cli -run TestCodeGraphMCP_Agy -count=1`
- [x] 2. G1: `ensureCodeGraphMCP` lanza el instalador de codegraph cuando solo falta en agy → `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_SoloFaltaAgy -count=1`
- [x] 3. G2: `mem doctor` avisa, con el comando manual y la nota de GEMINI.md, si falta codebase-memory-mcp en el `mcp_config.json` de agy y su binario está en PATH; sin aviso si está → `go test ./adapters/primary/cli -run TestDoctorCodebaseMemoryMCP_Agy -count=1`
- [x] 4. `InstallAgyGlobal` conserva codegraph, codebase-memory-mcp y claves ajenas de `mcp_config.json` y es idempotente → `go test ./adapters/primary/setup -run TestInstallAgyGlobal_ConservaMCPAjenos -count=1`
- [x] 5. Q1: `ensureCodeGraphMCP` invoca `codegraph install --target <faltantes> --location global --yes` (agy → `antigravity`) y el comando manual nombra los mismos targets → `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_TargetsExplicitos -count=1`
- [x] 6. Sandbox: `mem install --yes --agents agy` y el camino de `mem update` sobre un HOME temporal dejan gomemory y codegraph en `mcp_config.json`, sin `~/.gemini/GEMINI.md` ni `~/.gemini/settings.json`, y `mem doctor` avisa por cbm → evidencia manual con el JSON resultante
- [x] 7. Compresor en agy 1.4.12 real: `agy -p` que llama a get_context/search_memories devuelve salida comprimida con ref ⟦mem⟧ y `mem pack retrieve <ref>` recupera el original; `mem doctor` suma eventos agy → evidencia manual con transcript y ref
- [x] 8. agy resuelve `"command":"mem"` relativo al PATH (tools de gomemory listadas en la sesión real) y PostToolUse sigue sin entregar la salida → evidencia manual
- [x] 9. Verificación completa → `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-04 00:34 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestCodeGraphMCP_Agy -count=1` (salida 1)
      --- FAIL: TestCodeGraphMCP_Agy (0.00s)
          codegraph_mcp_test.go:155: agy sin codegraph: presente=false registrado=false
      FAIL
      FAIL	mem/adapters/primary/cli	0.645s
      FAIL
- 2026-11-04 00:34 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run TestCodeGraphMCP_Agy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.293s
- 2026-11-04 00:34 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_SoloFaltaAgy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.507s
- 2026-11-04 00:34 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestDoctorCodebaseMemoryMCP_Agy -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/codegraph_mcp_test.go:206:16: undefined: agyCodebaseMemoryWarning
      adapters/primary/cli/codegraph_mcp_test.go:213:15: undefined: agyCodebaseMemoryWarning
      adapters/primary/cli/codegraph_mcp_test.go:218:16: undefined: agyCodebaseMemoryWarning
      adapters/primary/cli/codegraph_mcp_test.go:223:16: undefined: agyCodebaseMemoryWarning
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-04 00:35 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run TestDoctorCodebaseMemoryMCP_Agy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.396s
- 2026-11-04 00:35 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run TestDoctorCodebaseMemoryMCP_Agy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.394s
- 2026-11-04 00:35 · tarea 4 · verde · caducada · `go test ./adapters/primary/setup -run TestInstallAgyGlobal_ConservaMCPAjenos -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.180s
- 2026-11-04 00:38 · tarea 5 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_TargetsExplicitos -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/codegraph_mcp_test.go:231:25: undefined: codeGraphInstallArgs
      adapters/primary/cli/codegraph_mcp_test.go:234:12: undefined: codeGraphManual
      adapters/primary/cli/codegraph_mcp_test.go:237:34: undefined: codeGraphInstallArgs
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-04 00:39 · tarea 5 · verde · caducada · `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_TargetsExplicitos -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.317s
- 2026-11-04 00:39 · tarea 6 · manual · caducada
      Sandbox HOME temporal con binario de este árbol (gomemory 2.44.3+HCD-020), agy/codegraph 1.6.2/cbm 0.12.0 reales. 1) mem install --yes --agents agy desde 0: exit 0, 'CodeGraph MCP: activo en agy', mcp_config.json = gomemory+codegraph, ~/.gemini solo contiene config/ (sin GEMINI.md ni settings.json). 2) mem update --version v2.44.3 --yes con mcp_config sin codegraph y con clave ajena: exit 0, codegraph añadido, 'ajeno' conservado, sin GEMINI.md. 3) mem doctor: aviso 'agy sin codebase-memory-mcp → codebase-memory-mcp install --clients=antigravity (…escribe ~/.gemini/GEMINI.md)' y 'agy: conectado' en CodeGraph MCP. Antes del arreglo Q1, el mismo paso 2 creaba ~/.gemini/GEMINI.md y settings.json.
- 2026-11-04 00:42 · tarea 7 · manual · caducada
      Sesión real agy 1.4.12 (conversation 6f05d193-3392-4b3b-829e-f7eab6e34a28), agy --mode plan -p con --settings de sesión que solo permite mcp(gomemory/get_context) y mcp(gomemory/search_memories): status SUCCESS, sin denied_actions. La salida de search_memories llegó comprimida: '⟦mem⟧ ×24 filas · ref=ed50029a47f7' (~500 caracteres según el agente). mem pack retrieve ed50029a47f7 → exit 0, original íntegro de 30 líneas / 1810 bytes (memorias [1567], [1569], …). mem doctor: 'agy budget_trimmed' pasó de 4 a 7 (último memory orig=16834, emit=9797).
- 2026-11-04 00:42 · tarea 8 · manual · caducada
      1) command relativo: ~/.gemini/config/mcp_config.json registra gomemory como 'command: mem' (sin ruta absoluta); agy mcp list lo muestra 'enabled' y en la sesión real 6f05d193-3392-4b3b-829e-f7eab6e34a28 get_context y search_memories respondieron, así que agy resuelve mem por el PATH. 2) PostToolUse sin salida: agy --version = 1.4.12, la misma versión medida en docs/evidence/hcd015/agy-native.md:15-19 (PostToolUse recibe toolCall/stepIdx/error, no la salida). No se re-midió con un hook espía para no tocar ~/.gemini/config/hooks.json real (excluido del alcance); el estado 'no disponible' de mem doctor sigue respaldado por esa evidencia.
- 2026-11-04 00:42 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run TestCodeGraphMCP_Agy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.456s
- 2026-11-04 00:42 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_SoloFaltaAgy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.659s
- 2026-11-04 00:42 · tarea 3 · verde · caducada · `go test ./adapters/primary/cli -run TestDoctorCodebaseMemoryMCP_Agy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.365s
- 2026-11-04 00:42 · tarea 4 · verde · caducada · `go test ./adapters/primary/setup -run TestInstallAgyGlobal_ConservaMCPAjenos -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.198s
- 2026-11-04 00:42 · tarea 9 · verde · caducada · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh` (salida 0)
      ok  	mem/adapters/secondary/expandstate	0.352s
      ok  	mem/adapters/secondary/generator	1.263s
      ok  	mem/adapters/secondary/hcdfiles	0.607s
      ok  	mem/adapters/secondary/launcher	0.775s
      ok  	mem/adapters/secondary/persistence	32.761s
      ok  	mem/adapters/secondary/release	0.401s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.202s
      ok  	mem/adapters/secondary/speckit	0.144s
      ok  	mem/adapters/secondary/tokens	0.137s
      ok  	mem/adapters/secondary/usage	0.136s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	13.136s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.224s
      ok  	mem/infrastructure	0.329s
      ok  	mem/tests/contract	93.684s
      ok  	mem/tests/integration	41.199s
      ?   	mem/version	[no test files]
      OK: --uninstall solo retira binarios propios
- 2026-11-04 02:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run TestCodeGraphMCP_Agy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.636s
- 2026-11-04 02:00 · tarea 2 · verde · vigente · `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_SoloFaltaAgy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.450s
- 2026-11-04 02:00 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli -run TestDoctorCodebaseMemoryMCP_Agy -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.326s
- 2026-11-04 02:00 · tarea 4 · verde · vigente · `go test ./adapters/primary/setup -run TestInstallAgyGlobal_ConservaMCPAjenos -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.224s
- 2026-11-04 02:00 · tarea 5 · verde · vigente · `go test ./adapters/primary/cli -run TestEnsureCodeGraphMCP_TargetsExplicitos -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.322s
- 2026-11-04 02:00 · tarea 6 · manual · vigente
      Re-ejecutado sobre HEAD fbdaa9d con binario gomemory 2.44.4 del árbol, HOME temporal, agy/codegraph/cbm reales. mem install --yes --agents agy desde 0: exit 0, 'CodeGraph MCP: activo en agy', ~/.gemini solo con config/. mem update --version v2.44.4 --yes con mcp_config sin codegraph y con clave 'ajeno': exit 0, mcpServers = ajeno, codegraph, gomemory; sin GEMINI.md ni settings.json. mem doctor: aviso 'agy sin codebase-memory-mcp → codebase-memory-mcp install --clients=antigravity (…GEMINI.md)' y 'agy: conectado'.
- 2026-11-04 02:01 · tarea 7 · manual · vigente
      Re-ejecutado sobre HEAD fbdaa9d: sesión real agy 1.4.12 (conversation 4ddf061c-a2a1-44b8-9470-09b89ca75733), agy --mode plan -p con --settings de sesión que solo permite mcp(gomemory/get_context) y mcp(gomemory/search_memories): SUCCESS, sin denied_actions. search_memories llegó comprimida: '⟦mem⟧ ×24 filas · ref=7e79bf4d0193' (~580 caracteres). mem pack retrieve 7e79bf4d0193 → original íntegro de 30 líneas / 1813 bytes (memorias [1567], …).
- 2026-11-04 02:01 · tarea 8 · manual · vigente
      Re-ejecutado sobre HEAD fbdaa9d. 1) command relativo: ~/.gemini/config/mcp_config.json registra gomemory como 'command: mem'; en la sesión real 4ddf061c-a2a1-44b8-9470-09b89ca75733 search_memories respondió, así que agy resuelve mem por el PATH. 2) PostToolUse sin salida: agy --version = 1.4.12, la misma versión medida en docs/evidence/hcd015/agy-native.md:15-19; no se re-midió con un hook espía para no tocar ~/.gemini/config/hooks.json real (excluido del alcance).
- 2026-11-04 02:01 · tarea 9 · verde · vigente · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh` (salida 0)
      ok  	mem/adapters/secondary/expandstate	0.315s
      ok  	mem/adapters/secondary/generator	1.201s
      ok  	mem/adapters/secondary/hcdfiles	0.729s
      ok  	mem/adapters/secondary/launcher	0.693s
      ok  	mem/adapters/secondary/persistence	33.175s
      ok  	mem/adapters/secondary/release	0.417s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.221s
      ok  	mem/adapters/secondary/speckit	0.148s
      ok  	mem/adapters/secondary/tokens	0.143s
      ok  	mem/adapters/secondary/usage	0.162s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.410s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.212s
      ok  	mem/infrastructure	0.394s
      ok  	mem/tests/contract	96.050s
      ok  	mem/tests/integration	42.249s
      ?   	mem/version	[no test files]
      OK: --uninstall solo retira binarios propios

## Siguiente
Cerrada con aprobación sobre la revisión f25854ec19ad.
