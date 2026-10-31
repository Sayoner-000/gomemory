# uninstall e install no pierden texto propio

ID: HCD-010
Tipo: fix
Carril: full
Motivo: Riesgo alto sobre datos del usuario y contrato de uninstall, con decisiones interdependientes

## Intención
mem uninstall, mem install y mem update retiran solo bloques completos de gomemory, sin borrar ni truncar texto propio, con copia en install y update (C-001 y C-005 del ACR 70ebfee4)

## Reproducción
Con binario y `HOME` temporales, en un repositorio de prueba: `mem install . --yes` sobre un `AGENTS.md` propio **sin ningún marcador** responde `AGENTS.md retirado` y el archivo desaparece; solo queda en `.memory/backups/agent-files/`, que `.gitignore` excluye. `mem uninstall --yes .` sobre `antes`, un bloque `v10` completo y `después propio` deja solo `antes`: el texto posterior se pierde y no hay copia.

## Alcance
Incluido: `mem install`, `mem update`, `mem uninstall` y la retirada global de `mem uninstall` retiran solo bloques completos de gomemory usando `stripMarkedBlocks`; `install` y `update` copian antes con nombre único, y un fallo de retirada llega al resumen de `uninstall`; aviso cuando la migración global omite agentes no elegidos (C-003); aviso cuando el presupuesto deja el baseline fuera de `--keep-baseline` (C-004); el párrafo del CLI equivalente sube por encima del corte de Claude; `mem doctor` avisa si el plugin de OpenCode instalado está desactualizado.
Excluido: el borrado del binario global en `mem uninstall --all`, que es parte de esa orden; `.cursorrules` y `.windsurfrules`, que siguen sin tocarse; recuperar archivos ya retirados por versiones anteriores, que siguen en sus respaldos.

## ¿Qué pasa si no lo hacemos?
`mem install` y `mem update` siguen sacando del repositorio instrucciones propias que nunca fueron de gomemory, y `mem uninstall` sigue perdiendo sin copia el texto posterior a un bloque.

## Alternativa más barata
Dejar de limpiar en `install` y `update`, sin tocar `uninstall`: dos líneas, pero mantiene la pérdida sin copia y deja bloques heredados sin retirar. Reutilizar `stripMarkedBlocks` cuesta poco más y cierra ambos.

## Historias
### US1. Texto propio a salvo
Como persona con instrucciones propias, quiero que gomemory solo retire lo suyo, para no perder texto ni encontrar archivos desaparecidos.
- C-01: Dado un `AGENTS.md` o `CLAUDE.md` sin marcadores, cuando se ejecuta `mem install`, `mem update` o `mem uninstall`, entonces el archivo no cambia.
- C-02: Dado un archivo con un bloque completo y texto propio, cuando se ejecuta cualquiera de esos comandos, entonces se retira el bloque y el texto propio queda igual; en `install` y `update` existe además una copia que no reemplaza a las anteriores, y `uninstall` no hace copia.
- C-03: Dado un archivo con un bloque sin cierre, cuando se ejecuta cualquiera de esos comandos, entonces el archivo no cambia, se informa cómo retirarlo a mano y `uninstall` no lo da por correcto.

### US2. Avisos y entrega completos
Como persona que opera gomemory, quiero enterarme de lo omitido, para no creer que algo se aplicó cuando no fue así.
- C-04: Dado `--agents` acotado y bloques en agentes no elegidos, cuando se ejecuta `setup-mcp --scope global`, entonces la salida los nombra como pendientes.
- C-05: Dado un presupuesto que deja fuera el baseline, cuando OpenCode pide el contexto, entonces el canal registra la omisión.
- C-06: Dado el corte de las instrucciones MCP en Claude, cuando se recorta el bloque en ese punto, entonces conserva el párrafo del CLI equivalente.
- C-07: Dado un plugin de OpenCode distinto del embebido, cuando se ejecuta `mem doctor`, entonces lo informa como desactualizado con `mem setup-mcp --scope global` como remedio.

## Aclaraciones
- [x] Q1. ¿Dónde queda la copia previa en install, update y uninstall de proyecto? → En .memory/backups
- [x] Q2. ¿Qué pasa en mem install con un archivo generado solo con el bloque y sin texto propio? → Eliminarlo (Recomendado)
- [x] Q3. mem uninstall borra .memory después de crear la copia, así que C-02 no se cumple en esa orden. ¿Dónde queda la copia de uninstall de proyecto? → Sin copia en uninstall

## Slices
### VS-01. Install, update y uninstall solo retiran bloques completos
Tareas: 1, 2, 3, 8, 9, 10, 11, 13
Criterios: C-01, C-02, C-03

### VS-02. Lo omitido se informa y la entrega queda completa
Tareas: 4, 5, 6, 7, 12
Criterios: C-04, C-05, C-06, C-07

## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/primary/cli/ -run 'TestLimpiarArchivos_|TestRetiradaProyecto_' -count=1`
- [x] 2. Retirar con la misma regla estricta en la desinstalación global (dep: 1) → `go test ./adapters/primary/cli/ -run 'TestRemoveProtocolBlockFile_' -count=1`
- [x] 3. Comprobar install y uninstall con el binario real en entorno aislado (dep: 1) → guion `repro10` en HOME y PATH temporales: archivo propio intacto, texto posterior conservado
- [x] 4. Nombrar los agentes omitidos con bloques pendientes (dep: 1) → `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_InformaAgentesOmitidos' -count=1`
- [x] 5. Registrar la omisión del baseline por presupuesto en OpenCode (dep: 1) → `node --test infrastructure/plugin/opencode/gomemory.test.mjs && go test ./tests/contract/ -run 'TestOpenCodeV2_FabricaV1IdenticaALaReferencia' -count=1`
- [x] 6. Subir el párrafo del CLI equivalente por encima del corte (dep: 1) → `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado' -count=1`
- [x] 7. Informar en doctor un plugin de OpenCode desactualizado (dep: 1) → `go test ./adapters/primary/setup/ -run 'TestPluginOpenCode_Desactualizado' -count=1`
- [x] 8. Dar nombre único y creación exclusiva a la copia de proyecto, y omitirla en uninstall (dep: 1) → `go test ./adapters/primary/cli/ -run 'TestCopiaProyecto_' -count=1`
- [x] 9. Propagar el fallo de retirada al resumen de uninstall (dep: 1) → `go test ./adapters/primary/cli/ -run 'TestUninstallProyecto_FalloDeRetirada' -count=1`
- [x] 10. Eliminar un archivo solo si no queda ningún texto tras retirar el bloque (dep: 1) → `go test ./adapters/primary/cli/ -run 'TestRetirada_TituloEsTextoPropio' -count=1`
- [x] 11. Tratar como ausencia solo un archivo inexistente y propagar otros fallos de acceso (dep: 9) → `go test ./adapters/primary/cli/ -run 'TestRetirada_FalloDeLectura' -count=1`
- [x] 12. Inventariar el baseline sin protocolo en uninstall global y comparar también un plugin v1 compatible (dep: 7) → `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestGlobalRemover_BaselineSolo|TestPluginOpenCode_V1CompatibleDesactualizado' -count=1`
- [x] 13. Crear la copia de proyecto con permisos 0600 (dep: 8) → `go test ./adapters/primary/cli/ -run 'TestCopiaProyecto_' -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-31 21:51 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestLimpiarArchivos_|TestRetiradaProyecto_' -count=1` (salida 1)
      --- FAIL: TestLimpiarArchivos_SinMarcadoresNoToca (0.00s)
          project_block_retire_test.go:27: archivo alterado: "", open /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestLimpiarArchivos_SinMarcadoresNoToca1689280605/001/AGENTS.md: no such file or directory
      --- FAIL: TestLimpiarArchivos_RetiraBloqueConCopia (0.00s)
          project_block_retire_test.go:39: resultado: ""
      --- FAIL: TestLimpiarArchivos_SinCierreNoTocaEInforma (0.00s)
          project_block_retire_test.go:52: archivo alterado: ""
      --- FAIL: TestRetiradaProyecto_ConservaTextoPosterior (0.00s)
          project_block_retire_test.go:71: resultado: "antes\n"
      --- FAIL: TestRetiradaProyecto_SinCierreNoToca (0.00s)
          project_block_retire_test.go:80: archivo "antes\n", salida   ✅ AGENTS.md: bloque de protocolo de memoria removido
                ℹ️  CLAUDE.md: no encontrado
                ℹ️  CLAUDE.txt: no encontrado
                ℹ️  .cursorrules: no encontrado
                ℹ️  .windsurfrules: no encontrado
      FAIL
      FAIL	mem/adapters/primary/cli	0.271s
      FAIL
- 2026-10-31 21:51 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestRemoveProtocolBlockFile_' -count=1` (salida 1)
      --- FAIL: TestRemoveProtocolBlockFile_SinCierreNoToca (0.01s)
          project_block_retire_test.go:89: changed=true err=<nil>
      --- FAIL: TestRemoveProtocolBlockFile_EncabezadoSinMarcadorNoSeToca (0.01s)
          project_block_retire_test.go:100: changed=true err=<nil>
      FAIL
      FAIL	mem/adapters/primary/cli	0.282s
      FAIL
- 2026-10-31 22:03 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestLimpiarArchivos_|TestRetiradaProyecto_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.309s
- 2026-10-31 22:03 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestRemoveProtocolBlockFile' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.301s
- 2026-10-31 22:03 · tarea 3 · verde · caducada · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad/repro10c.sh /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad` (salida 0)
      install sin marcadores: intacto
      install con bloque: texto y copia
      uninstall: texto posterior conservado
      sin cierre: intacto
- 2026-10-31 22:03 · tarea 4 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_InformaAgentesOmitidos' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.290s
- 2026-10-31 22:03 · tarea 5 · verde · caducada · `node --test infrastructure/plugin/opencode/gomemory.test.mjs && go test ./tests/contract/ -run 'TestOpenCodeV2_FabricaV1IdenticaALaReferencia' -count=1` (salida 0)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.453833ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.182625ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.376208ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.254625ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.282458ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.346917ms)
      ✔ expand: stop sends the last assistant text (0.26025ms)
      ✔ expand: stop without messages sends an empty last message (0.160292ms)
      ✔ expand: post-tool observes bash only (16.201125ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.925042ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.709041ms)
      ℹ tests 63
      ℹ suites 0
      ℹ pass 63
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1846.272875
      ok  	mem/tests/contract	0.254s
- 2026-10-31 22:03 · tarea 6 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.281s
- 2026-10-31 22:03 · tarea 7 · verde · caducada · `go test ./adapters/primary/setup/ -run 'TestPluginOpenCode_Desactualizado' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.166s
- 2026-10-31 22:15 · tarea 8 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestCopiaProyecto_' -count=1` (salida 1)
      --- FAIL: TestCopiaProyecto_NoReemplazaLaAnterior (0.01s)
          project_block_retire_test.go:127: copias = [/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestCopiaProyecto_NoReemplazaLaAnterior1888341574/001/.memory/backups/agent-files/AGENTS.md], <nil>
      --- FAIL: TestCopiaProyecto_UninstallNoCopia (0.00s)
          project_block_retire_test.go:143: uninstall creó copia: <nil>
      FAIL
      FAIL	mem/adapters/primary/cli	0.388s
      FAIL
- 2026-10-31 22:15 · tarea 9 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestUninstallProyecto_FalloDeRetirada' -count=1` (salida 1)
      --- FAIL: TestUninstallProyecto_FalloDeRetirada (0.00s)
          project_block_retire_test.go:152: err = <nil>
      FAIL
      FAIL	mem/adapters/primary/cli	0.274s
      FAIL
- 2026-10-31 22:18 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestLimpiarArchivos_|TestRetiradaProyecto_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.385s
- 2026-10-31 22:18 · tarea 2 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestRemoveProtocolBlockFile' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.303s
- 2026-10-31 22:18 · tarea 3 · verde · caducada · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad/repro10c.sh /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad` (salida 0)
      install sin marcadores: intacto
      install con bloque: texto y copia
      uninstall: texto posterior conservado
      sin cierre: intacto
- 2026-10-31 22:18 · tarea 4 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_InformaAgentesOmitidos' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.276s
- 2026-10-31 22:18 · tarea 5 · verde · caducada · `node --test infrastructure/plugin/opencode/gomemory.test.mjs && go test ./tests/contract/ -run 'TestOpenCodeV2_FabricaV1IdenticaALaReferencia' -count=1` (salida 0)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.305417ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.1345ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.238459ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.156667ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.147ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.734584ms)
      ✔ expand: stop sends the last assistant text (0.23125ms)
      ✔ expand: stop without messages sends an empty last message (0.111125ms)
      ✔ expand: post-tool observes bash only (16.155083ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (0.68875ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.436583ms)
      ℹ tests 63
      ℹ suites 0
      ℹ pass 63
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1865.190875
      ok  	mem/tests/contract	0.243s
- 2026-10-31 22:18 · tarea 6 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.271s
- 2026-10-31 22:18 · tarea 7 · verde · caducada · `go test ./adapters/primary/setup/ -run 'TestPluginOpenCode_Desactualizado' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.157s
- 2026-10-31 22:18 · tarea 8 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestCopiaProyecto_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.286s
- 2026-10-31 22:18 · tarea 9 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestUninstallProyecto_FalloDeRetirada' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.284s
- 2026-10-31 22:21 · tarea 10 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestRetirada_TituloEsTextoPropio' -count=1` (salida 1)
      --- FAIL: TestRetirada_TituloEsTextoPropio (0.00s)
          project_block_retire_test.go:163: "# Instrucciones para agentes AI\n": "", open /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestRetirada_TituloEsTextoPropio3584045815/001/AGENTS.md: no such file or directory
      FAIL
      FAIL	mem/adapters/primary/cli	0.310s
      FAIL
- 2026-10-31 22:21 · tarea 11 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestRetirada_FalloDeLectura' -count=1` (salida 1)
      --- FAIL: TestRetirada_FalloDeLectura (0.00s)
          project_block_retire_test.go:176: err = <nil>
      FAIL
      FAIL	mem/adapters/primary/cli	0.279s
      FAIL
- 2026-10-31 22:21 · tarea 12 · rojo observado · caducada · `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestGlobalRemover_BaselineSolo|TestPluginOpenCode_V1CompatibleDesactualizado' -count=1` (salida 1)
      --- FAIL: TestGlobalRemover_BaselineSolo (0.00s)
          project_block_retire_test.go:194: el baseline solo debe entrar en el plan
      FAIL
      FAIL	mem/adapters/primary/cli	0.293s
      --- FAIL: TestPluginOpenCode_V1CompatibleDesactualizado (0.00s)
          opencode_plugin_stale_test.go:46: problema = ""
      FAIL
      FAIL	mem/adapters/primary/setup	0.168s
      FAIL
- 2026-10-31 22:28 · tarea 13 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestCopiaProyecto_' -count=1` (salida 1)
      --- FAIL: TestCopiaProyecto_PermisosPrivados (0.01s)
          project_block_retire_test.go:214: original 600: copia -rw-r--r--, <nil>
      FAIL
      FAIL	mem/adapters/primary/cli	0.438s
      FAIL
- 2026-10-31 22:31 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestLimpiarArchivos_|TestRetiradaProyecto_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.284s
- 2026-10-31 22:31 · tarea 2 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestRemoveProtocolBlockFile' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.278s
- 2026-10-31 22:31 · tarea 3 · verde · vigente · `bash /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad/repro10c.sh /private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/5dcf41f4-faf6-490a-b447-279f082e52b3/scratchpad` (salida 0)
      install sin marcadores: intacto
      install con bloque: texto y copia
      uninstall: texto posterior conservado
      sin cierre: intacto
- 2026-10-31 22:31 · tarea 4 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestMigracionGlobal_InformaAgentesOmitidos' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.446s
- 2026-10-31 22:31 · tarea 5 · verde · vigente · `node --test infrastructure/plugin/opencode/gomemory.test.mjs && go test ./tests/contract/ -run 'TestOpenCodeV2_FabricaV1IdenticaALaReferencia' -count=1` (salida 0)
      ✔ expand: tool.execute.before throws the Go block reason and sends tool, args and session (0.571042ms)
      ✔ expand: a failing mem never blocks a tool (fail-open) (0.400292ms)
      ✔ expand: pre-tool context is delivered once in the next system.transform of that session (0.340042ms)
      ✔ expand: a blocked prompt is replaced by the reason and is not recorded (0.287541ms)
      ✔ expand: prompt context is prepended as a synthetic part (0.220125ms)
      ✔ expand: session events send startup and compact sources; stop block becomes a pending notice (0.560042ms)
      ✔ expand: stop sends the last assistant text (0.354833ms)
      ✔ expand: stop without messages sends an empty last message (0.172708ms)
      ✔ expand: post-tool observes bash only (16.761667ms)
      ✔ v2 expand: execute.before propagates the block to OpenCode and passes the input (1.423708ms)
      ✔ v2 expand: execute.before allows when mem fails or the session is foreign (0.697791ms)
      ℹ tests 63
      ℹ suites 0
      ℹ pass 63
      ℹ fail 0
      ℹ cancelled 0
      ℹ skipped 0
      ℹ todo 0
      ℹ duration_ms 1934.594917
      ok  	mem/tests/contract	0.254s
- 2026-10-31 22:31 · tarea 6 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestIntegrationBlock_ReglasSobrevivenAlTruncado' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.268s
- 2026-10-31 22:31 · tarea 7 · verde · vigente · `go test ./adapters/primary/setup/ -run 'TestPluginOpenCode_' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.188s
- 2026-10-31 22:31 · tarea 8 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestCopiaProyecto_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.317s
- 2026-10-31 22:31 · tarea 9 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestUninstallProyecto_FalloDeRetirada' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.283s
- 2026-10-31 22:31 · tarea 10 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestRetirada_TituloEsTextoPropio' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.288s
- 2026-10-31 22:31 · tarea 11 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestRetirada_FalloDeLectura' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.276s
- 2026-10-31 22:31 · tarea 12 · verde · vigente · `go test ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestGlobalRemover_BaselineSolo|TestPluginOpenCode_V1CompatibleDesactualizado' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.279s
      ok  	mem/adapters/primary/setup	0.163s
- 2026-10-31 22:31 · tarea 13 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestCopiaProyecto_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.306s

## Siguiente
Cerrada con aprobación sobre la revisión d37b8bd31dad.
