# Los hooks sin session_id pierden la conversación en Windows

ID: HCD-038
Tipo: fix
Carril: fast
Motivo: Fix con reproducción clara (CI Windows) y una única causa raíz: hostbind no implementado en Windows

## Intención
Implementar psInfo para Windows (padre, hora de arranque y nombre del proceso con la API de Windows) para que los hooks, el MCP y la CLI encuentren la conversación de su host igual que en macOS y Linux; el job Windows del CI vuelve a verde.

## Reproducción
CI `test`, run 42149534403 (commit 0a6aec3, release v2.49.0), job «Windows (Go y binario del release)»: `go test ./... -count=1` falla en `tests/integration`:
- `TestConversation_AvisoPendienteNoCruzaDeConversacion`: «precondición: la conversación A debía dejar el aviso pendiente: GetFileAttributesEx …\.memory\conversations\559aead08264d579\.pending-agent-notice: The system cannot find the file specified.»
- `TestPlanDoc_NoRepiteLaMemoriaYaEntregada`: «la memoria ya entregada en el arranque no debe repetirse en el plan-doc».
Causa: `psInfo` (`adapters/secondary/hostbind/hostbind.go`) devuelve error en Windows, así que no hay enlace con el host. Un hook sin `session_id` (`turn-end`, `user-prompt-submit`) resuelve la conversación vacía y no encuentra el estado ni la sesión de la conversación que abrió `session-start`. En macOS y Linux pasa porque `ps` enlaza el hook con su host.
Comando de comprobación: el job Windows del run de `test` del commit actual (`git rev-parse HEAD`) termina en `success`.

## Alcance
Incluido: `psInfo` para Windows con la biblioteca estándar (`syscall`): proceso padre y nombre del ejecutable con `CreateToolhelp32Snapshot`/`Process32Next`, y hora de arranque con `OpenProcess`/`GetProcessTimes`, en `adapters/secondary/hostbind/` separado por plataforma. Los shells intermedios de Windows (`cmd`, `powershell`, `pwsh`) se reconocen aunque el nombre lleve ruta con `\` y extensión `.exe`. Va con pruebas portables del recorrido y de `psInfo` sobre el proceso actual.
Excluido: el id de terminal de la CLI en Windows (`TerminalID` sigue vacío); cambiar la regla de HCD-036 de no tomar «la última conversación del proyecto»; tocar las pruebas de integración para esquivar el fallo; el fallo intermitente anterior `TestCicloDeVida_Idempotente`.

## ¿Qué pasa si no lo hacemos?
El CI de Windows sigue en rojo en `main`. Además, en Windows real, los hooks, el MCP y la CLI que no reciben `session_id` no encuentran su conversación: el estado por turno (avisos, huella, debounces) y la supresión de lo ya entregado se rompen. HCD-036 queda a medias en una plataforma que se publica.

## Alternativa más barata
1. Saltar las dos pruebas en Windows (`t.Skip`). Oculta un defecto real del binario de Windows, así que se descarta.
2. Volver en Windows a «la última conversación registrada del proyecto». Contradice HCD-036 (dos agentes en el mismo proyecto se pisarían) y solo arregla los hooks, no el MCP.
La implementación nativa es pequeña (un archivo por plataforma, sin dependencias nuevas) y deja Windows con el mismo comportamiento que las demás plataformas.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → re-ejecutar la reproducción: `id=$(gh run list --workflow test --commit $(git rev-parse HEAD) --limit 1 --json databaseId -q '.[0].databaseId') && test -n "$id" && test "$(gh run view "$id" --json jobs -q '.jobs[]|select(.name|startswith("Windows")).conclusion')" = success`
- [x] 2. Reconocer los shells de Windows (`cmd`, `powershell`, `pwsh`, con ruta `\` y `.exe`) al buscar el host → `go test ./adapters/secondary/hostbind -run TestHost_SaltaShellsDeWindows`
- [x] 3. Separar `psInfo` por plataforma y añadir la implementación de Windows con `syscall` → `GOOS=windows GOARCH=amd64 go vet ./adapters/secondary/hostbind/ && go test ./adapters/secondary/hostbind -run TestPsInfo_ProcesoActual`
- [x] 4. Ejecutar la suite completa local sin regresiones, sin caché como el CI → `go test ./... -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-05 21:51 -05:00 · tarea 1 · rojo observado · caducada · `sh -c id=$(gh run list --workflow test --commit $(git rev-parse HEAD) --limit 1 --json databaseId -q ".[0].databaseId"); test "$(gh run view $id --json jobs -q ".jobs[]|select(.name|startswith(\"Windows\")).conclusion")" = success` (salida 1)
        # View a specific run
        $ gh run view 12345
        # View a specific run with specific attempt number
        $ gh run view 12345 --attempt 3
        # View a specific job within a run
        $ gh run view --job 456789
        # View the full log for a specific job
        $ gh run view --log --job 456789
        # Exit non-zero if a run failed
        $ gh run view 0451 --exit-status && echo "run pending or passed"
      LEARN MORE
        Use `gh <command> <subcommand> --help` for more information about a command.
        Read the manual at https://cli.github.com/manual
        Learn about exit codes using `gh help exit-codes`
        Learn about accessibility experiences using `gh help accessibility`
- 2026-11-05 21:52 -05:00 · tarea 1 · rojo observado · caducada · `id=$(gh run list --workflow test --commit $(git rev-parse HEAD) --limit 1 --json databaseId -q '.[0].databaseId') && test -n "$id" && test "$(gh run view "$id" --json jobs -q '.jobs[]|select(.name|startswith("Windows")).conclusion')" = success` (salida 1)
- 2026-11-05 21:52 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/secondary/hostbind -run TestHost_SaltaShellsDeWindows` (salida 1)
      --- FAIL: TestHost_SaltaShellsDeWindows (0.00s)
          hostbind_test.go:81: el hook 300 debía enlazar con el host 100 saltando su shell: 200 true
          hostbind_test.go:81: el hook 320 debía enlazar con el host 100 saltando su shell: 210 true
          hostbind_test.go:81: el hook 330 debía enlazar con el host 100 saltando su shell: 220 true
      FAIL
      FAIL	mem/adapters/secondary/hostbind	0.660s
      FAIL
- 2026-11-05 21:52 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/hostbind -run TestHost_SaltaShellsDeWindows` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.146s
- 2026-11-05 21:53 -05:00 · tarea 3 · verde · caducada · `GOOS=windows GOARCH=amd64 go vet ./adapters/secondary/hostbind/ && go test ./adapters/secondary/hostbind -run TestPsInfo_ProcesoActual` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.174s
- 2026-11-05 21:53 -05:00 · tarea 4 · verde · caducada · `go test ./...` (salida 0)
      ok  	mem/adapters/secondary/generator	(cached)
      ok  	mem/adapters/secondary/gitdiff	(cached)
      ok  	mem/adapters/secondary/hcdfiles	(cached)
      ok  	mem/adapters/secondary/hostbind	0.130s
      ok  	mem/adapters/secondary/launcher	(cached)
      ok  	mem/adapters/secondary/persistence	(cached)
      ok  	mem/adapters/secondary/release	(cached)
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	(cached)
      ok  	mem/adapters/secondary/speckit	(cached)
      ok  	mem/adapters/secondary/tokens	(cached)
      ok  	mem/adapters/secondary/usage	(cached)
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	(cached)
      ?   	mem/assets	[no test files]
      ok  	mem/domain	(cached)
      ok  	mem/infrastructure	0.327s
      ok  	mem/tests/contract	99.812s
      ok  	mem/tests/integration	(cached)
      ?   	mem/version	[no test files]
- 2026-11-05 21:57 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/hostbind -run TestHost_SaltaShellsDeWindows` (salida 0)
      ok  	mem/adapters/secondary/hostbind	0.125s
- 2026-11-05 21:57 -05:00 · tarea 3 · verde · caducada · `GOOS=windows GOARCH=amd64 go vet ./adapters/secondary/hostbind/ && go test ./adapters/secondary/hostbind -run TestPsInfo_ProcesoActual` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
- 2026-11-05 21:57 -05:00 · tarea 4 · verde · caducada · `go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/generator	1.237s
      ok  	mem/adapters/secondary/gitdiff	0.691s
      ok  	mem/adapters/secondary/hcdfiles	0.631s
      ok  	mem/adapters/secondary/hostbind	0.131s
      ok  	mem/adapters/secondary/launcher	0.546s
      ok  	mem/adapters/secondary/persistence	33.075s
      ok  	mem/adapters/secondary/release	0.363s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.188s
      ok  	mem/adapters/secondary/speckit	0.125s
      ok  	mem/adapters/secondary/tokens	0.138s
      ok  	mem/adapters/secondary/usage	0.123s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	14.163s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.260s
      ok  	mem/infrastructure	0.452s
      ok  	mem/tests/contract	103.545s
      ok  	mem/tests/integration	55.634s
      ?   	mem/version	[no test files]
- 2026-11-05 21:59 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/hostbind -run TestHost_SaltaShellsDeWindows` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
- 2026-11-05 21:59 -05:00 · tarea 3 · verde · caducada · `GOOS=windows GOARCH=amd64 go vet ./adapters/secondary/hostbind/ && go test ./adapters/secondary/hostbind -run TestPsInfo_ProcesoActual` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
- 2026-11-05 21:59 -05:00 · tarea 4 · verde · caducada · `go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/generator	1.397s
      ok  	mem/adapters/secondary/gitdiff	0.669s
      ok  	mem/adapters/secondary/hcdfiles	0.691s
      ok  	mem/adapters/secondary/hostbind	0.193s
      ok  	mem/adapters/secondary/launcher	0.753s
      ok  	mem/adapters/secondary/persistence	32.756s
      ok  	mem/adapters/secondary/release	0.369s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.236s
      ok  	mem/adapters/secondary/speckit	0.173s
      ok  	mem/adapters/secondary/tokens	0.118s
      ok  	mem/adapters/secondary/usage	0.107s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	13.767s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.229s
      ok  	mem/infrastructure	0.479s
      ok  	mem/tests/contract	100.672s
      ok  	mem/tests/integration	51.058s
      ?   	mem/version	[no test files]
- 2026-11-05 22:10 -05:00 · tarea 1 · falló · caducada · `id=$(gh run list --workflow test --commit $(git rev-parse HEAD) --limit 1 --json databaseId -q '.[0].databaseId') && test -n "$id" && test "$(gh run view "$id" --json jobs -q '.jobs[]|select(.name|startswith("Windows")).conclusion')" = success` (salida 1)
- 2026-11-05 22:24 -05:00 · tarea 1 · verde · vigente · `id=$(gh run list --workflow test --commit $(git rev-parse HEAD) --limit 1 --json databaseId -q '.[0].databaseId') && test -n "$id" && test "$(gh run view "$id" --json jobs -q '.jobs[]|select(.name|startswith("Windows")).conclusion')" = success` (salida 0)
- 2026-11-05 22:24 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/secondary/hostbind -run TestHost_SaltaShellsDeWindows` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
- 2026-11-05 22:24 -05:00 · tarea 3 · verde · vigente · `GOOS=windows GOARCH=amd64 go vet ./adapters/secondary/hostbind/ && go test ./adapters/secondary/hostbind -run TestPsInfo_ProcesoActual` (salida 0)
      ok  	mem/adapters/secondary/hostbind	(cached)
- 2026-11-05 22:24 -05:00 · tarea 4 · verde · vigente · `go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/generator	1.190s
      ok  	mem/adapters/secondary/gitdiff	0.709s
      ok  	mem/adapters/secondary/hcdfiles	0.620s
      ok  	mem/adapters/secondary/hostbind	0.136s
      ok  	mem/adapters/secondary/launcher	0.587s
      ok  	mem/adapters/secondary/persistence	32.475s
      ok  	mem/adapters/secondary/release	0.387s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.215s
      ok  	mem/adapters/secondary/speckit	0.136s
      ok  	mem/adapters/secondary/tokens	0.125s
      ok  	mem/adapters/secondary/usage	0.127s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	12.510s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.255s
      ok  	mem/infrastructure	0.499s
      ok  	mem/tests/contract	100.150s
      ok  	mem/tests/integration	50.310s
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión 56119c31815b.
