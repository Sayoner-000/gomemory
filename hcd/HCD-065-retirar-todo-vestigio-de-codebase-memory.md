# Retirar todo vestigio de codebase-memory-mcp; solo queda el conector de codegraph

ID: HCD-065
Tipo: technical
Carril: fast
Motivo: Cambio técnico de alcance claro: el runtime ya ignora codebase-memory-mcp; quedan restos en doctor, plantillas, skill, scripts, tests y comentarios

## Intención
Que gomemory solo conozca a codegraph como grafo de código externo: el doctor observa el hook real de codegraph (codegraph prompt-hook) en lugar de los cbm-*, ninguna plantilla, skill, script, test ni comentario vigente nombra codebase-memory-mcp, y los ajustes heredados que lo nombren se normalizan sin error


Contexto: el runtime ya solo crea proveedores codegraph (`adapters/secondary/codegraph/providers.go:17`, `NewProviders`) y la persona eligió codegraph como índice principal (memoria 2217, 2026-11-23). El 2026-11-26 se desinstaló codebase-memory-mcp del entorno de la máquina; esta unidad cubre el repo.

## Alcance
Incluido: doctor, textos que llegan al agente, ajustes, comentarios de producción, tests, script de activación y documentación vigente (detalle):
- Doctor: `inspectCodegraph` / `codegraphHookPattern` (`adapters/primary/setup/activation_inspect.go:522-545`) dejan de reconocer `cbm-*` y observan el hook real de CodeGraph (`codegraph prompt-hook` en `~/.claude/settings.json`), con su detalle; sus tests y `tests/contract/scope_dedup_test.go`.
- Texto que llega al agente: `infrastructure/templates/locales/{es,en}/get_context.md.tmpl:9` y la skill `defensive-qa-audit` (`infrastructure/templates/…/SKILL.md:76` y su copia `skills/…`) nombran solo codegraph.
- Ajustes: `persistence/settings.go`, `ports/settings_repository.go` y la ayuda de `mem settings`: comentarios y textos sin codebase-memory-mcp. `ReadSettings` descarta de `code_graph_providers`/`code_graph_command` lo que no sea codegraph, así un `settings.json` heredado (como el de este repo hasta hoy) no vuelve a mostrar el proveedor retirado.
- Código de producción con comentarios o cadenas residuales: `cmd_hook.go`, `cmd_install.go`, `hook_tool_output.go`, `proc_unix.go`, `tui/tui.go`, `codegraph/providers.go`, `domain/code_provider.go`, `domain/optimize.go`.
- Tests que usan codebase-memory-mcp como proveedor vigente o como nombre de ejemplo: `cmd_hook_bootstrap_test.go`, `cmd_index_external_test.go`, `cmd_doctor_opencode_test.go`, `codegraph_mcp_test.go`, `hook_tool_output_safety_test.go`, `console/{components,flow}_test.go`, `setup/{hcd015_agy_setup,opencode_global,opencode_plugin_inspect}_test.go`, `tui_test.go`, `providers_test.go`, `adr_sync_test.go`, `usecases/{build_context,detect_duplicates,provider_selection}_test.go`, `tests/integration/{code_graph_multi_provider,hook_marker_integration}_test.go`.
- Retirar `scripts/test-codebase-memory-activation.sh`, que exige tools `mcp__codebase-memory-mcp__*` que gomemory ya no emite.
- Documentación vigente: `INSTALLATION.md`, `docs/ARQUITECTURA.md`, `docs/architecture.md`, `docs/MANUAL.md` y las guías HTML de `docs/`.

Excluido: registro histórico, datos medidos o capturados, configuración fuera del repo, tests de compatibilidad con ajustes heredados y el bloqueo de graph-before-grep (detalle):
- Registro histórico: `specs/**`, `hcd/**`, `CHANGELOG*`, `docs/audits/**` (incluida su evidencia), `docs/lessons.md`, `tasks/todo.md`.
- Datos medidos o capturados: corpus y golden de compresión (`tests/testdata/compression_corpus/**`, `tests/integration/testdata/golden_structural/**`), salidas reales en `tests/contract/testdata/**` y la fixture de protocolo heredado `tests/fixtures/038/protocol-v8.md`. Cambiarlos falsea mediciones o migraciones.
- La configuración de la máquina fuera del repo (ya se limpió aparte, con copia de seguridad).
- Tests cuyo propósito es probar que un ajuste heredado con "codebase-memory-mcp" se ignora o normaliza (p. ej. TestNewProviders_SeleccionaSoloCodeGraphEIgnoraLegados): se conservan; el literal queda solo como entrada de compatibilidad (Q1).
- El bloqueo de graph-before-grep cuando el agente no tiene herramienta nativa de búsqueda: va en una unidad fix aparte, al cerrar esta (Q2).

## ¿Qué pasa si no lo hacemos?
El doctor sigue dando ✓ al brazo codegraph por la mera presencia de hooks `cbm-*` (falso positivo, y tras la desinstalación del 2026-11-26 la fila desaparece aunque codegraph sí esté enganchado: falso negativo). `get_context` y la skill ADM siguen sugiriendo al agente un servidor que ya no existe, lo que invita a llamadas fallidas. Un `settings.json` heredado sigue mostrando `codebase-memory-mcp` como candidato. El script de activación falla o miente, y el código conserva una dependencia conceptual que confunde a quien lo mantiene.

## Alternativa más barata
Cambiar solo los dos textos que llegan al agente (plantilla de contexto y skill ADM) y dejar el resto. Resuelve las llamadas fallidas, pero deja el doctor reportando un canal equivocado, la configuración heredada sin normalizar y el resto de restos, que es justo lo que pidió la persona («retira todo vestigio»). Se descarta.


## Aclaraciones
- [x] Q1. Los tests que pasan "codebase-memory-mcp" como ajuste heredado para probar que se ignora, ¿se conservan como prueba de compatibilidad o se retira también ese literal? → Conservar como legado: se quedan solo los tests cuyo propósito es probar que un ajuste heredado se ignora o normaliza; el literal aparece únicamente como entrada de compatibilidad.
- [x] Q2. La regla graph-before-grep bloquea la búsqueda literal en .go aunque el agente no tenga herramienta nativa de búsqueda (la regla 10 dice que en ese caso solo debe avisar). ¿Entra en esta unidad o va en una aparte? → Unidad aparte: es un fallo distinto (el hook no detecta que el harness carece de Grep); se abre otra unidad fix al cerrar HCD-065.
- [x] Q3. hook_tool_output.go:46 excluye de la compresión mcp__codebase-memory-mcp__get_code_snippet (código fuente sobre el que se edita). ¿Se sustituye por mcp__codegraph__codegraph_explore, que hoy sí se comprime, o solo se retira la exclusión? → Excluir codegraph_explore: se sustituye la exclusión por mcp__codegraph__codegraph_explore (y su forma en OpenCode), porque codegraph entrega fuente literal numerada apta para Edit; comprimirla rompe esa promesa.

## Checklist
- [x] 1. Cambiar inspectCodegraph para que observe `codegraph prompt-hook` y no `cbm-*` → go test ./adapters/primary/setup/ -run '^TestActivationInspect_Codegraph' -count=1
- [x] 2. Hacer que ReadSettings descarte de code_graph_providers y code_graph_command lo que no sea codegraph → go test ./adapters/secondary/persistence/ -run '^TestReadSettings_' -count=1
- [x] 3. Añadir la prueba de contrato TestHCD065_SinVestigiosDeCodebaseMemory con un subtest por área y una lista explícita de excepciones (histórico, datos medidos, tests de compatibilidad de Q1), cada una existente y aún con el término → go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^excepciones$' -count=1
- [x] 4. Limpiar los textos que llegan al agente (get_context.md.tmpl es/en y la skill defensive-qa-audit en sus dos copias) → go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^plantillas$' -count=1
- [x] 5. Limpiar comentarios y cadenas del código de producción → go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^produccion$' -count=1
- [x] 6. Retirar codebase-memory-mcp de los tests que lo usan como proveedor vigente o nombre de ejemplo → go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^tests$' -count=1
- [x] 7. Borrar scripts/test-codebase-memory-activation.sh → go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^scripts$' -count=1
- [x] 8. Actualizar la documentación vigente (INSTALLATION.md, docs/ARQUITECTURA.md, docs/architecture.md, docs/MANUAL.md y guías HTML de docs/) → go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^docs$' -count=1
- [x] 9. Comprobar en vivo que el doctor del binario compilado reporta el brazo codegraph por su hook real → go run ./infrastructure doctor | grep -Eq 'codegraph +claude +user +instructions +.*codegraph prompt-hook'
- [x] 10. Ejecutar la regresión completa (1/3: vet y paquetes fuera de tests/; partida en tres porque el motor corta a los 10 min y la suite con -race los supera) → go vet ./... && go test -race ./adapters/... ./application/... ./assets/... ./domain/... ./infrastructure/... ./version/...
- [x] 11. Excluir codegraph_explore (Claude y OpenCode) de la compresión de salidas en lugar de get_code_snippet (Q3) → go test ./adapters/primary/cli/ -run '^TestToolOutputExclusion_HerramientasDeConsultaExacta$' -count=1
- [x] 12. Ejecutar la regresión completa (2/3: pruebas de contrato) → go test -race ./tests/contract/
- [x] 13. Ejecutar la regresión completa (3/3: pruebas de integración) → go test -race ./tests/integration/

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-26 20:18 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/setup/ -run '^TestActivationInspect_Codegraph' -count=1` (salida 1)
      --- FAIL: TestActivationInspect_CodegraphPresenteObservaElHookReal (0.06s)
          activation_inspect_test.go:322: con el hook de CodeGraph instalado debe haber un canal codegraph, got []
      --- FAIL: TestActivationInspect_CodegraphIgnoraHooksRetirados (0.06s)
          activation_inspect_test.go:344: un hook cbm-* no debe producir canal codegraph: {Arm:codegraph Agent:claude Scope:user Kind:instructions State:ok Detail:hooks cbm-* presentes en ~/.claude/settings.json (solo lectura)}
      FAIL
      FAIL	mem/adapters/primary/setup	0.772s
      FAIL
- 2026-11-26 20:18 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/setup/ -run '^TestActivationInspect_Codegraph' -count=1` (salida 1)
      --- FAIL: TestActivationInspect_CodegraphPresenteObservaElHookReal (0.06s)
          activation_inspect_test.go:322: con el hook de CodeGraph instalado debe haber un canal codegraph, got []
      --- FAIL: TestActivationInspect_CodegraphIgnoraHooksRetirados (0.06s)
          activation_inspect_test.go:344: un hook cbm-* no debe producir canal codegraph: {Arm:codegraph Agent:claude Scope:user Kind:instructions State:ok Detail:hooks cbm-* presentes en ~/.claude/settings.json (solo lectura)}
      FAIL
      FAIL	mem/adapters/primary/setup	0.365s
      FAIL
- 2026-11-26 20:18 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/setup/ -run '^TestActivationInspect_Codegraph' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.643s
- 2026-11-26 20:19 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/secondary/persistence/ -run '^TestReadSettings_' -count=1` (salida 1)
      --- FAIL: TestReadSettings_DescartaProveedoresRetirados (0.00s)
          settings_test.go:82: CodeGraphProviders = [codebase-memory-mcp codegraph], se esperaba [codegraph]
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.223s
      FAIL
- 2026-11-26 20:19 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/persistence/ -run '^TestReadSettings_' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.236s
- 2026-11-26 20:20 -05:00 · tarea 3 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^excepciones$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.642s
- 2026-11-26 20:20 -05:00 · tarea 4 · rojo observado · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^plantillas$' -count=1` (salida 1)
      --- FAIL: TestHCD065_SinVestigiosDeCodebaseMemory (0.48s)
          --- FAIL: TestHCD065_SinVestigiosDeCodebaseMemory/plantillas (0.00s)
              hcd065_codebase_memory_test.go:99: infrastructure/templates/defensive-qa-audit/SKILL.md nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:99: infrastructure/templates/locales/en/get_context.md.tmpl nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:99: infrastructure/templates/locales/es/get_context.md.tmpl nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:99: skills/defensive-qa-audit/SKILL.md nombra al proveedor retirado codebase-memory-mcp
      FAIL
      FAIL	mem/tests/contract	0.690s
      FAIL
- 2026-11-26 20:20 -05:00 · tarea 4 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^plantillas$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.765s
- 2026-11-26 20:22 -05:00 · tarea 5 · rojo observado · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^produccion$' -count=1` (salida 1)
      --- FAIL: TestHCD065_SinVestigiosDeCodebaseMemory (0.53s)
          --- FAIL: TestHCD065_SinVestigiosDeCodebaseMemory/produccion (0.00s)
              hcd065_codebase_memory_test.go:102: adapters/primary/cli/cmd_hook.go:675 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: adapters/primary/cli/cmd_hook.go:847 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: adapters/primary/cli/cmd_install.go:361 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: adapters/primary/cli/hook_tool_output.go:46 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: adapters/primary/cli/proc_unix.go:12 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: adapters/primary/tui/tui.go:456 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: adapters/primary/tui/tui.go:2567 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: adapters/secondary/codegraph/providers.go:16 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: application/ports/settings_repository.go:21 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: domain/code_provider.go:86 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: domain/optimize.go:26 nombra al proveedor retirado codebase-memory-mcp
      FAIL
      FAIL	mem/tests/contract	0.754s
      FAIL
- 2026-11-26 20:26 -05:00 · tarea 11 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run '^TestToolOutputExclusion_HerramientasDeConsultaExacta$' -count=1` (salida 1)
      --- FAIL: TestToolOutputExclusion_HerramientasDeConsultaExacta (0.00s)
          hook_tool_output_safety_test.go:53: codegraph_explore devuelve código exacto: no se reescribe en Claude
          hook_tool_output_safety_test.go:57: codegraph_explore devuelve código exacto: no se reescribe en OpenCode
      FAIL
      FAIL	mem/adapters/primary/cli	0.430s
      FAIL
- 2026-11-26 20:26 -05:00 · tarea 11 · verde · caducada · `go test ./adapters/primary/cli/ -run '^TestToolOutputExclusion_HerramientasDeConsultaExacta$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.287s
- 2026-11-26 20:26 -05:00 · tarea 5 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^produccion$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.749s
- 2026-11-26 20:26 -05:00 · tarea 6 · rojo observado · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^tests$' -count=1` (salida 1)
      test.go:102: application/usecases/build_context_test.go:352 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: application/usecases/build_context_test.go:397 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: application/usecases/detect_duplicates_test.go:172 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: application/usecases/provider_selection_test.go:20 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: application/usecases/provider_selection_test.go:23 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/contract/scope_dedup_test.go:47 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/contract/scope_dedup_test.go:105 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/contract/scope_dedup_test.go:139 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/integration/code_graph_multi_provider_test.go:38 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/integration/code_graph_multi_provider_test.go:73 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/integration/code_graph_multi_provider_test.go:74 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/integration/hook_marker_integration_test.go:69 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/integration/hook_marker_integration_test.go:192 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:102: tests/integration/hook_marker_integration_test.go:290 nombra al proveedor retirado codebase-memory-mcp
      FAIL
      FAIL	mem/tests/contract	0.686s
      FAIL
- 2026-11-26 20:28 -05:00 · tarea 6 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^tests$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.833s
- 2026-11-26 20:28 -05:00 · tarea 7 · rojo observado · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^scripts$' -count=1` (salida 1)
      --- FAIL: TestHCD065_SinVestigiosDeCodebaseMemory (0.46s)
          --- FAIL: TestHCD065_SinVestigiosDeCodebaseMemory/scripts (0.00s)
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:2 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:6 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:11 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:98 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:103 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:133 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:154 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:170 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:174 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:178 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:288 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: scripts/test-codebase-memory-activation.sh:367 nombra al proveedor retirado codebase-memory-mcp
      FAIL
      FAIL	mem/tests/contract	0.683s
      FAIL
- 2026-11-26 20:28 -05:00 · tarea 7 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^scripts$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.676s
- 2026-11-26 20:28 -05:00 · tarea 8 · rojo observado · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^docs$' -count=1` (salida 1)
      y-mcp
              hcd065_codebase_memory_test.go:103: docs/Guia-SDD_HCD_GOMEMORY-v2.56.html:817 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Guia-SDD_HCD_GOMEMORY-v2.56.html:833 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Guia-SDD_HCD_GOMEMORY-v2.56.html:868 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Guia-SDD_HCD_GOMEMORY-v2.56.html:1130 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Guia-SDD_HCD_GOMEMORY-v2.56.html:1419 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Guia-Tecnica_GOMEMORY-v2.56.html:817 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Guia-Tecnica_GOMEMORY-v2.56.html:821 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/MANUAL.md:199 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/MANUAL.md:201 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/MANUAL.md:208 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Manual-Referencia-Developers-GOMEMORY.html:893 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Manual-Referencia-Developers-GOMEMORY.html:908 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/Manual-Referencia-Developers-GOMEMORY.html:952 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/architecture.md:324 nombra al proveedor retirado codebase-memory-mcp
              hcd065_codebase_memory_test.go:103: docs/architecture.md:332 nombra al proveedor retirado codebase-memory-mcp
      FAIL
      FAIL	mem/tests/contract	0.683s
      FAIL
- 2026-11-26 20:31 -05:00 · tarea 8 · verde · caducada · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^docs$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.936s
- 2026-11-26 20:31 -05:00 · tarea 9 · verde · caducada · `go run ./infrastructure doctor | grep -Eq 'codegraph +claude +user +instructions +.*codegraph prompt-hook'` (salida 0)
- 2026-11-26 20:32 -05:00 · tarea 10 · falló · caducada · `go vet ./... && go test -race ./...` (salida -1)
        ○ .cline/mcp_settings.json: no encontrado
        ○ ~/.codex/config.toml conserva [mcp_servers.gomemory] y los hooks del ciclo de gomemory — son configuración global compartida por todos los proyectos.
        ○ .claude/plugins/gomemory: no encontrado
        ○ .claude/settings.json: no encontrado
        ○ .claude/settings.json: sin permisos de gomemory
        ○ opencode.json: sin permisos de gomemory
        ○ envoltorios nativos: no encontrados
      Resumen:
        ✓ Integración de gomemory en /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition3786072312/003
        ✓ /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCmdUninstallAcceptsYesFlagInAnyPosition3786072312/003/mem (copia v2.59.0)
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ Memoria 1 eliminada
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) de el proyecto "001-47d610f083b146c2".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      ✓ 1 memoria(s) eliminada(s) por garbage collection de el proyecto "001-47d610f083b146c2".
      ⚠ 1 memoria(s) borrada(s), pero el texto sigue en disco: limpieza pendiente
      FAIL
      FAIL	mem/tests/contract	441.336s
      (tiempo límite agotado)
- 2026-11-26 20:44 -05:00 · tarea 10 · verde · vigente · `go vet ./... && go test -race ./adapters/... ./application/... ./assets/... ./domain/... ./infrastructure/... ./version/...` (salida 0)
      ok  	mem/adapters/secondary/generator	(cached)
      ok  	mem/adapters/secondary/gitdiff	(cached)
      ok  	mem/adapters/secondary/hcdfiles	(cached)
      ok  	mem/adapters/secondary/hostbind	(cached)
      ok  	mem/adapters/secondary/i18n	(cached)
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
      ok  	mem/domain/i18n	(cached)
      ok  	mem/infrastructure	(cached)
      ?   	mem/version	[no test files]
- 2026-11-26 20:45 -05:00 · tarea 12 · verde · vigente · `go test -race ./tests/contract/` (salida 0)
      ok  	mem/tests/contract	106.577s
- 2026-11-26 20:47 -05:00 · tarea 13 · verde · vigente · `go test -race ./tests/integration/` (salida 0)
      ok  	mem/tests/integration	201.003s
- 2026-11-26 20:51 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/setup/ -run '^TestActivationInspect_Codegraph' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.664s
- 2026-11-26 20:51 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/secondary/persistence/ -run '^TestReadSettings_' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	0.241s
- 2026-11-26 20:51 -05:00 · tarea 3 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^excepciones$' -count=1` (salida 0)
      ok  	mem/tests/contract	0.978s
- 2026-11-26 20:51 -05:00 · tarea 4 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^plantillas$' -count=1` (salida 0)
      ok  	mem/tests/contract	1.642s
- 2026-11-26 20:51 -05:00 · tarea 5 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^produccion$' -count=1` (salida 0)
      ok  	mem/tests/contract	1.117s
- 2026-11-26 20:51 -05:00 · tarea 6 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^tests$' -count=1` (salida 0)
      ok  	mem/tests/contract	1.002s
- 2026-11-26 20:51 -05:00 · tarea 7 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^scripts$' -count=1` (salida 0)
      ok  	mem/tests/contract	1.078s
- 2026-11-26 20:51 -05:00 · tarea 8 · verde · vigente · `go test ./tests/contract/ -run '^TestHCD065_SinVestigiosDeCodebaseMemory$/^docs$' -count=1` (salida 0)
      ok  	mem/tests/contract	1.350s
- 2026-11-26 20:51 -05:00 · tarea 9 · verde · vigente · `go run ./infrastructure doctor | grep -Eq 'codegraph +claude +user +instructions +.*codegraph prompt-hook'` (salida 0)
- 2026-11-26 20:51 -05:00 · tarea 11 · verde · vigente · `go test ./adapters/primary/cli/ -run '^TestToolOutputExclusion_HerramientasDeConsultaExacta$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.434s

## Siguiente
Cerrada con aprobación sobre la revisión b29203d93917.
