# Reglas de trabajo por defecto: codegraph antes de editar y antes de buscar

ID: HCD-067
Tipo: evolution
Carril: fast
Motivo: Evolución acotada de una plantilla sembrada; el mecanismo de actualización ya existe y no hay decisiones abiertas

## Intención
Que las reglas de trabajo que gomemory siembra por defecto, en todos los proyectos y runtimes, exijan consultar codegraph antes de editar código (para conocer el proyecto, quién llama a lo que cambia y su impacto) y antes de cualquier búsqueda de texto, sin depender de una preferencia de cada persona; y que los proyectos con las reglas v3 sin editar las reciban con mem update


Contexto: la persona tuvo que reforzar dos veces en la sesión de HCD-065/066 que codegraph va antes de editar y de buscar (memoria 2230), y que antes de definir hay que recuperar decisiones previas (memoria 2239). Pidió que esto sea el comportamiento por defecto de gomemory y no una preferencia de cada persona. La plantilla vigente (workrules-v3, `infrastructure/templates/agent-preamble.md`) pide el grafo para «entender o localizar código», pero no antes de editar ni para medir el impacto, y «Recupera antes de rehacer» no dice qué hacer si el historial HCD sale vacío.

## Alcance
Incluido: reglas de trabajo v4 sembradas por defecto, su huella y la actualización de los proyectos con la v3 intacta (detalle):
- `infrastructure/templates/agent-preamble.md` pasa a `gomemory-workrules-v4`: una regla nueva «Antes de editar, el grafo» (codegraph sobre lo que cambia, aunque sea una línea: proyecto, llamadores e impacto), «Entender o localizar» con el grafo antes de cualquier búsqueda de texto, «Editar» extendido a los archivos de texto del proyecto, y «Recupera antes de rehacer» con `mem hcd history` más `search_memories` y `hcd/` cuando el historial sale vacío. Se conserva el cierre del glosario que exige `TestSeedsAparecenSinInstall`.
- `domain/seed.go`: `DefaultSHA256` de la v4 y la huella de la v3 en `PreviousDefaultSHA256`, para que `UpgradePristineSeeds` la lleve con `mem update` (y con el arranque del MCP) a todo proyecto que no editó sus reglas, en los cuatro runtimes.
- Un test de contrato que exige el contenido de las reglas nuevas en la plantilla embebida.
- Este proyecto: sus reglas fijadas pasan a la v4 si siguen intactas.
Excluido: el baseline universal, la constitución y el playbook HCD; cambiar las reglas de expand (ya cubiertas en HCD-066); reescribir las reglas que un equipo editó a mano (el mecanismo las respeta y las marca como personalizadas); retirar las preferencias 2230 y 2239, que siguen valiendo como registro de lo que pidió la persona.

## ¿Qué pasa si no lo hacemos?
Cada persona tiene que repetir la misma corrección en cada proyecto y en cada runtime, como pasó dos veces en esta sesión, y los agentes siguen editando sin mirar el impacto ni las decisiones previas (HCD-030 quedó sin citar en HCD-066 por eso).

## Alternativa más barata
Dejar las preferencias guardadas (2230, 2239). Solo llegan al proyecto donde se guardaron y dependen de que el agente las recupere; no cubren proyectos nuevos ni otros runtimes. Se descarta.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Exigir en un test de contrato que la plantilla embebida de reglas sea la v4 y contenga «Antes de editar», el grafo antes de buscar, la edición nativa de archivos de texto y la recuperación con search_memories y hcd/ → go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1
- [x] 2. Reescribir agent-preamble.md como workrules-v4 con esas reglas → go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1 && go test ./tests/integration/ -run '^TestSeedsAparecenSinInstall$' -count=1
- [x] 3. Mover la huella v3 a PreviousDefaultSHA256 y fijar la de la v4 → go test ./domain/ -run '^(TestPinnedDocs_HuellaActualCoincideConLaPlantilla|TestIsPreviousDefault)$' -count=1
- [x] 4. Comprobar que una semilla v3 intacta se actualiza a la v4 y una editada no → go test ./tests/contract/ -run '^TestHCD067_V3IntactaSeActualiza$' -count=1
- [x] 6. Comprobar en vivo que este proyecto, con las reglas v3 intactas, recibe la v4 al sembrar con el binario nuevo (sin rojo: la actualización ya se ejecutó una vez con `mem seed`) → sh -c 'go run ./infrastructure seed >/dev/null && go run ./infrastructure docs show rules | head -1 | grep -q "gomemory-workrules-v4"'
- [x] 5. Ejecutar la regresión de domain, seeds, CLI, contrato e integración → go vet ./... && go test -race ./domain/... ./application/... ./adapters/primary/cli/... ./tests/contract/ && go test -race ./tests/integration/

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-26 21:32 -05:00 · tarea 1 · rojo observado · caducada · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1` (salida 1)
      --- FAIL: TestHCD067_ReglasDeTrabajoV4 (0.00s)
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "<!-- gomemory-workrules-v4 -->"
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "**Antes de editar, el grafo**"
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "aunque sea una línea"
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "quién llama a lo que cambia"
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "antes de cualquier búsqueda de texto"
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "archivos de texto del proyecto"
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "`hcd/`"
      FAIL
      FAIL	mem/tests/contract	0.386s
      FAIL
- 2026-11-26 21:32 -05:00 · tarea 4 · rojo observado · caducada · `go test ./tests/contract/ -run '^TestHCD067_V3IntactaSeActualiza$' -count=1` (salida 1)
      --- FAIL: TestHCD067_V3IntactaSeActualiza (0.02s)
          hcd067_work_rules_test.go:57: la v3 intacta debe reconocerse como plantilla anterior para que mem update la actualice
          hcd067_work_rules_test.go:63: la huella vigente sigue siendo la de la v3
      FAIL
      FAIL	mem/tests/contract	0.244s
      FAIL
- 2026-11-26 21:32 -05:00 · tarea 2 · falló · caducada · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1 && go test ./tests/integration/ -run '^TestSeedsAparecenSinInstall$' -count=1` (salida 1)
      --- FAIL: TestHCD067_ReglasDeTrabajoV4 (0.00s)
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "quién llama a lo que cambia"
          hcd067_work_rules_test.go:36: la plantilla de reglas no contiene "antes de cualquier búsqueda de texto"
      FAIL
      FAIL	mem/tests/contract	0.230s
      FAIL
- 2026-11-26 21:32 -05:00 · tarea 2 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1 && go test ./tests/integration/ -run '^TestSeedsAparecenSinInstall$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.232s
      ok  	mem/tests/integration	1.328s
- 2026-11-26 21:33 -05:00 · tarea 3 · rojo observado · caducada · `go test ./domain/ -run '^(TestPinnedDocs_HuellaActualCoincideConLaPlantilla|TestIsPreviousDefault)$' -count=1` (salida 1)
      --- FAIL: TestPinnedDocs_HuellaActualCoincideConLaPlantilla (0.00s)
          seed_test.go:168: rules: la plantilla agent-preamble.md cambió (huella 0e8658c9591ae8cbc91d40609fc76aa19390f589bd5381639d9603a0e9511129) y el catálogo dice 1df6c4d17e30be814e41f4fdedfcf5f5a33f808c50f63aa3b8700c7e13991f59. Mueve la huella anterior a PreviousDefaultSHA256 y registra la nueva en DefaultSHA256
      FAIL
      FAIL	mem/domain	0.159s
      FAIL
- 2026-11-26 21:33 -05:00 · tarea 3 · verde · caducada · `go test ./domain/ -run '^(TestPinnedDocs_HuellaActualCoincideConLaPlantilla|TestIsPreviousDefault)$' -count=1` (salida 0)
      ok  	mem/domain	0.149s
- 2026-11-26 21:33 -05:00 · tarea 4 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD067_V3IntactaSeActualiza$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.289s
- 2026-11-26 21:33 -05:00 · tarea 5 · verde · caducada · `go vet ./... && go test -race ./domain/... ./application/... ./adapters/primary/cli/... ./tests/contract/ && go test -race ./tests/integration/` (salida 0)
      ok  	mem/domain	1.515s
      ok  	mem/domain/i18n	(cached)
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	92.873s
      ok  	mem/adapters/primary/cli	58.636s
      ok  	mem/adapters/primary/cli/clikit	(cached)
      ok  	mem/adapters/primary/cli/hcd	(cached)
      ok  	mem/adapters/primary/cli/review	(cached)
      ok  	mem/adapters/primary/cli/socratica	15.142s
      ok  	mem/tests/contract	108.547s
      ok  	mem/tests/integration	154.346s
- 2026-11-26 21:38 -05:00 · tarea 2 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1 && go test ./tests/integration/ -run '^TestSeedsAparecenSinInstall$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.260s
      ok  	mem/tests/integration	1.360s
- 2026-11-26 21:38 -05:00 · tarea 1 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.228s
- 2026-11-26 21:38 -05:00 · tarea 6 · verde · caducada · `sh -c 'go run ./infrastructure seed >/dev/null && go run ./infrastructure docs show rules | head -1 | grep -q "gomemory-workrules-v4"'` (salida 0)
- 2026-11-26 21:38 -05:00 · tarea 2 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1 && go test ./tests/integration/ -run '^TestSeedsAparecenSinInstall$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.226s
      ok  	mem/tests/integration	1.255s
- 2026-11-26 21:38 -05:00 · tarea 3 · verde · caducada · `go test ./domain/ -run '^(TestPinnedDocs_HuellaActualCoincideConLaPlantilla|TestIsPreviousDefault)$' -count=1` (salida 0)
      ok  	mem/domain	0.163s
- 2026-11-26 21:38 -05:00 · tarea 4 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD067_V3IntactaSeActualiza$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.262s
- 2026-11-26 21:38 -05:00 · tarea 5 · verde · caducada · `go vet ./... && go test -race ./domain/... ./application/... ./adapters/primary/cli/... ./tests/contract/ && go test -race ./tests/integration/` (salida 0)
      ok  	mem/domain	(cached)
      ok  	mem/domain/i18n	(cached)
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	(cached)
      ok  	mem/adapters/primary/cli	57.486s
      ok  	mem/adapters/primary/cli/clikit	(cached)
      ok  	mem/adapters/primary/cli/hcd	(cached)
      ok  	mem/adapters/primary/cli/review	(cached)
      ok  	mem/adapters/primary/cli/socratica	(cached)
      ok  	mem/tests/contract	110.417s
      ok  	mem/tests/integration	(cached)
- 2026-11-26 21:41 -05:00 · tarea 1 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.237s
- 2026-11-26 21:41 -05:00 · tarea 2 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD067_ReglasDeTrabajoV4$' -count=1 && go test ./tests/integration/ -run '^TestSeedsAparecenSinInstall$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.247s
      ok  	mem/tests/integration	1.322s
- 2026-11-26 21:41 -05:00 · tarea 3 · verde · vigente · `go test ./domain/ -run '^(TestPinnedDocs_HuellaActualCoincideConLaPlantilla|TestIsPreviousDefault)$' -count=1` (salida 0)
      ok  	mem/domain	0.128s
- 2026-11-26 21:41 -05:00 · tarea 4 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD067_V3IntactaSeActualiza$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.267s
- 2026-11-26 21:41 -05:00 · tarea 6 · verde · vigente · `sh -c 'go run ./infrastructure seed >/dev/null && go run ./infrastructure docs show rules | head -1 | grep -q "gomemory-workrules-v4"'` (salida 0)
- 2026-11-26 21:41 -05:00 · tarea 5 · falló · vigente · `go vet ./... && go test -race ./domain/... ./application/... ./adapters/primary/cli/... ./tests/contract/ && go test -race ./tests/integration/` (salida 1)
        ○ .mcp.json: no encontrado
        ○ opencode.json: no encontrado
        ○ .opencode.json: no encontrado
        ○ .cursor/mcp.json: no encontrado
        ○ .windsurf/mcp_config.json: no encontrado
        ○ .cline/mcp_settings.json: no encontrado
        ○ ~/.codex/config.toml conserva [mcp_servers.gomemory] y los hooks del ciclo de gomemory — son configuración global compartida por todos los proyectos.
        ○ .claude/plugins/gomemory: no encontrado
        ✓ .claude/settings.json: hooks de gomemory removidos
        ✓ .claude/settings.json: permisos de gomemory removidos
        ○ opencode.json: sin permisos de gomemory
        ○ envoltorios nativos: no encontrados
      Resumen:
        ✓ Integración de gomemory en /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestUninstallRemovesPermissionsAndStopHook3509398281/002
        ✓ /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestUninstallRemovesPermissionsAndStopHook3509398281/002/.memory
        ✓ Memoria del proyecto (/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/gomemory-test-integration-99473924/projects/002-96093f45982addcc)
      FAIL
      FAIL	mem/tests/integration	381.546s
      FAIL
- 2026-11-26 21:51 -05:00 · tarea 5 · verde · vigente · `go vet ./... && go test -race ./domain/... ./application/... ./adapters/primary/cli/... ./tests/contract/ && go test -race ./tests/integration/` (salida 0)
      ok  	mem/domain	(cached)
      ok  	mem/domain/i18n	(cached)
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	(cached)
      ok  	mem/adapters/primary/cli	121.128s
      ok  	mem/adapters/primary/cli/clikit	(cached)
      ok  	mem/adapters/primary/cli/hcd	(cached)
      ok  	mem/adapters/primary/cli/review	(cached)
      ok  	mem/adapters/primary/cli/socratica	(cached)
      ok  	mem/tests/contract	169.142s
      ok  	mem/tests/integration	246.819s

## Siguiente
Cerrada con aprobación sobre la revisión 71c77fe44fb6.
