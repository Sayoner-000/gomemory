# Memoria del Proyecto

## Reglas de trabajo (memoria fijada)

<!-- gomemory-workrules-v2 -->
## Reglas operativas del proyecto

Estas reglas complementan el baseline universal. Prevalecen solo cuando aportan
evidencia operativa específica de este proyecto.

1. **Primero reproduce la realidad.** Ante un bug o una paridad con un sistema
   en ejecución, comprueba logs, `curl`, navegador o el servicio real antes de
   cambiar código o iniciar un flujo de especificación.
2. **Tests verdes no bastan.** Valida el artefacto realmente servido cuando sea
   pertinente; un fixture que no representa al upstream no demuestra el caso
   real.
3. **Si el cambio no se ve, comprueba el despliegue.** Revisa binario/bundle,
   URL y caché antes de atribuir el problema a la implementación.
4. **Usa constitución y especificaciones como guía, no como ritual.** No fuerces
   un proceso que contradiga la evidencia o el flujo real de la persona.
5. **Cierra los hallazgos.** Todo riesgo o defecto descubierto debe incluir una
   propuesta de cierre y una validación proporcional antes de declararlo listo.
6. **Delega de forma intencional.** Si Octopus está disponible, consulta su ruta
   antes de crear subagentes; en cualquier caso, el beneficio debe justificar
   el coste de coordinación.

## 🔗 Sinapsis (memorias enlazadas)

- [13] "BuildContextPack prioriza por relevancia, no excluye: no basta para aislar contexto delegado" ↔ [12] "mem mcp sale al cerrar stdin: un printf con pipe nunca obtiene respuesta"
- [10] "Verificar contra el binario real: mem no tiene --list-tools ni doctor --db-path, y la BD vive en el store global" ↔ [7] "Octopus AAR: registro condicional de tools MCP y huella cero con el módulo apagado"
- [12] "mem mcp sale al cerrar stdin: un printf con pipe nunca obtiene respuesta" ↔ [10] "Verificar contra el binario real: mem no tiene --list-tools ni doctor --db-path, y la BD vive en el store global"
- [33] "ACR 027 corregida: InlineCostTokens domina Total() si se deriva de sus propios componentes" ↔ [30] "review_submit exige status=\"success\"|\"failure\", no hay fuente local del feature ACR"
- [14] "Aislar el store global en pruebas: GOMEMORY_DATA_HOME, nunca \"la base más reciente\"" ↔ [13] "BuildContextPack prioriza por relevancia, no excluye: no basta para aislar contexto delegado"
- [7] "Octopus AAR: registro condicional de tools MCP y huella cero con el módulo apagado" ↔ [5] "Octopus AAR nace como módulo opt-in (apagado por defecto)"
_Orden: masa = centralidad en el grafo de memorias sembrado en todas las memorias (uniforme); no mide importancia ni corrección._

## Preferencias del Usuario


## Decisiones de Arquitectura

- **Octopus AAR: registro condicional de tools MCP y huella cero con el módulo apagado**: Decisión de diseño de la fase de planificación de 027-octopus-aar.

Las 4 tools MCP de Octopus (octopus_route_task, octopus_route_plan, octopus_report, octopus_status) se registran SOLO con el módulo… → `get_memory 7`
- **La consolidación de hooks de Codex pertenece al adaptador global**: La migración distribuible de hooks se ejecuta desde setupCodexGlobal, compartido por mem install y setup-mcp global. → `get_memory 3`
  → `adapters/primary/cli/codex_hooks_migration.go`
- **Constitución del proyecto (spec-kit)**: # Constitución Técnica

**Versión:** 2.0.0
**Fecha de corte:** 2026-09-24

> Esta constitución aplica a todo proyecto nuevo. → `get_memory 2`

## Decisiones Técnicas

- **Excepciones §23 registradas en la constitución del proyecto (E-001 migraciones, E-002 puertos sin ctx)**: 2026-09-25, a petición de la persona («resuelve antes de implementar»), al cerrar el /speckit-analyze de la 033: la constitución fijada pasó de «por defecto» a «personalizado» (mem docs import… → `get_memory 87`
- **Spec 033: motor NATIVO de compresión con prácticas de Headroom, sin Headroom como actor**: Decisión de la persona (2026-09-25): el propio motor de gomemory comprime aplicando las prácticas de Headroom, sin Headroom como proceso ("agnóstico, no actor"). → `get_memory 79`
- **Semillas intactas se actualizan solas por huella; las editadas nunca**: Para que `mem update` entregue una plantilla nueva (constitución 2.0.0, 2026-09-24) sin pisar lo del equipo: domain.PinnedDoc lleva DefaultSHA256 + PreviousDefaultSHA256 (sha256 del contenido con… → `get_memory 74`
- **Plugin OpenCode dual: v2 es un adaptador sobre la fábrica v1 intacta**: Decisión tomada al implementar la feature 032, en lugar del GomemoryCore del plan: la rama v2 (createV2Setup) construye los ganchos con GomemoryPlugin({ $: bunShellShim(execFileExec), directory… → `get_memory 62`
- **Octopus AAR: consulta obligatoria antes de delegar en todos los runtimes**: Implementado: con OctopusEnabled=true, el generador compartido octopusDelegationPolicy se añade al bootstrap de hooks de Codex/Claude y se expone por `mem hook octopus-delegation-policy opencode`… → `get_memory 42`
- **Octopus integrado y publicado como v2.18.0**: La feature Octopus AAR se rebaseó sobre origin/main preservando el ledger ACR y los cambios de Octopus. → `get_memory 40`
- **Octopus AAR nace como módulo opt-in (apagado por defecto)**: La especificación 027-octopus-aar define el enrutador adaptativo de agentes como un módulo con interruptor propio en la pantalla de configuración de la TUI, APAGADO por defecto.

Esto se aparta… → `get_memory 5`
- **Los releases de GoMemory no usan un ciclo CI separado**: Retirar el workflow CI independiente y su badge del README. → `get_memory 4`

## Patrones y Convenciones

- **Aislar el store global en pruebas: GOMEMORY_DATA_HOME, nunca "la base más reciente"**: Patrón establecido al escribir la prueba de huella cero de Octopus AAR (tests/integration/octopus_module_off_test.go).

Para verificar en una prueba QUÉ escribió gomemory en su base, aísla el store… → `get_memory 14`

## Bugfixes

- **OpenCode 2.x reparte eventos Y ganchos de sesión a todas las instancias del plugin**: C-001 de la ACR acr_1f27e836 (feature 032), reproducido con OpenCode 2.0.16 real y 3 ubicaciones en un servicio: cada ubicación tiene su instancia del plugin, pero el servicio entrega a TODAS los… → `get_memory 71`
  → `infrastructure/plugin/opencode/gomemory.ts`
- **ACR 032 (solo informe): ESCALATED — 2 CONFIRMED HIGH, 1 SUSPECT**: ACR acr_1f27e836-c64c-4b60-9fed-3884395481f5 (target_type=worktree, revisión de solo informe con fix_authorized=false) sobre digest d9215357b29958f29814599857d0754c2156494e305298aac5ff80d7b0958fe5… → `get_memory 70`
- **ACR 032: 3 sospechas reales en mem doctor/OpenCode corregidas**: ACR APPROVED sobre la feature 032 dejó 3 SUSPECT; verificadas contra el código, las 3 eran reales:
1) Compatible() consideraba compatible un plugin solo v1 con la versión sin detectar (Major 0 < 2). → `get_memory 68`
  → `adapters/primary/cli/cmd_doctor.go`
- **OpenCode 2.0.16 real: sin session.idle, tools shell/subagent (bugs cazados solo contra el binario)**: Feature 032. → `get_memory 64`
  → `infrastructure/plugin/opencode/gomemory.ts`
- **ACR nativa de Octopus (cmd_hook.go): C-001 RESOLVED, revisión de verificación C-002 APPROVED**: Cierre del ciclo de ACR nativa (review_start/submit/consensus/fix_record/rejudge/finalize) sobre el enganche de política de delegación de Octopus AAR en cmd_hook.go, corroborando id=45.

Revisión… → `get_memory 52`
- **Corregidos A-001 y A-002 de la ACR post-rebase de Octopus (autorizados por el usuario)**: Sobre id=43 (ACR post-rebase de Octopus AAR v2.18.0): el usuario autorizó explícitamente corregir los dos hallazgos SUSPECT más accionables (single-reviewer, Reviewer A), aunque el protocolo ACR por… → `get_memory 45`
- **ACR post-rebase de Octopus AAR (v2.18.0): 1 CONFIRMED corregido, 13 SUSPECT sin acción automática**: ACR con dos revisores independientes (Reviewer A en Opus, Reviewer B en Sonnet, independencia FULL por diversidad de modelo) sobre el estado de Octopus AAR en main tras el rebase de publicación de… → `get_memory 43`
- **Revalidación independiente: RouteTask respeta las métricas explícitas de WorkUnit**: Dos re-revisores independientes comprobaron el fix del hallazgo C-001 sobre digest d5050fe5518ccf2fa844a337eeb2ffb872b31ae8533553141ae305da07accbfb. → `get_memory 39`
- **Bugfix real: buildInput descartaba ContractTokens/InlineCostTokens ya puestos en WorkUnit**: Detectado por una ACR adicional que el usuario corrió por su cuenta (fuera de esta sesión) sobre el estado ya "corregido" de C-001, veredicto ESCALATED. → `get_memory 37`
- **AAR 027: cierre de rutas inertes y señales ignoradas**: Se corrigieron los pendientes del ACR: Reader.Read devuelve el grafo completo de Spec Kit cuando task está vacío (arregla `octopus plan` sin --file); contexto no medido ya no equivale a contexto… → `get_memory 25`
  → `application/usecases/octopus_route_plan.go`
- **Corrección parcial AAR 027: rutas reales, presupuesto y telemetría**: Se corrigieron defectos reproducidos del AAR: los handlers MCP ahora aplican los ajustes del proyecto y miden objetivo+archivos; el contrato delegado recibe MemoryRepo/compresor/SpecKit reales; el… → `get_memory 24`
  → `adapters/primary/cli/cmd_mcp_octopus_tools.go`
- **Revisión de Octopus AAR (027): 11 defectos pendientes de corregir**: VEREDICTO: ESCALATED. → `get_memory 22`
- **Revisión de Octopus AAR (027): 11 defectos pendientes de corregir**: Estado: NINGUNO corregido. → `get_memory 21`

## Aprendizajes Recientes

- **Spec 033 analyze: el contexto de arranque y el post-compact no están cubiertos, y comprimir rompe la supresión por hash**: /speckit-analyze sobre 033 (2026-09-25), informe de solo lectura:
1) La supresión de plan-context compara HashDeContenido(raw ContextBuilder.Build()) con el hash registrado por cmd_context.go… → `get_memory 85` (`specs/033-native-context-compression/tasks.md`)
- **Línea base de la compresión estructural: ahorro casi nulo en JSON y en contexto**: Medido con mem 2.25.0 `mem pack compress <archivo>` (2026-09-25): go list -json ./... → `get_memory 82` (`adapters/primary/cli/cmd_pack.go`)
- **Reescritura de salida de tools por runtime (verificado en binarios reales 2026-09-25)**: Verificado con strings de los binarios instalados:
- Claude Code 2.1.282: PostToolUse acepta hookSpecificOutput.updatedToolOutput (para TODAS las tools: "Replaces the tool output before it is sent to… → `get_memory 81`
- **ACR del worktree OpenCode v2: APPROVED con tres observaciones no corroboradas**: ACR acr_8956249c-1ae0-439e-8614-6841b3309578 congeló HEAD 62746f64f4d3e83895283c4deca3951437065815 con digest b57b24688fd76efcdc6f5cb4486d38fd86495e6c12b880b24c9d515bf421da3e. → `get_memory 67` (`infrastructure/plugin/opencode/gomemory.ts`)
- **OpenCode 2.0 rompe el plugin v1 de gomemory: contrato nuevo y vía dual**: Verificado contra binarios reales (spike en HOME aislado, 2026-09-24). → `get_memory 58` (`infrastructure/plugin/opencode/gomemory.ts`)
- **git fetch falla por SSH: usar remote HTTPS anónimo para traer main**: En este entorno el remote SSH (git@github.com) deniega publickey y `git fetch/pull` fallan. → `get_memory 57`
- **ACR del HEAD actual queda INCOMPLETE por presupuesto de delegación**: La ACR acr_3ebbf738-08bf-40a7-9cea-5411c7219d07 congeló HEAD 7fafb5ac35cddbf10fc76aa7fe09bcb4a6984aad (árbol c10600b3bf7a8debb14c4a4dd01be5c69a02509d, sin cambios no confirmados). → `get_memory 56`
- **Verificar que un test de regresión falla sin el fix, sin tocar el árbol de trabajo (go -overlay + GOFLAGS)**: En revisiones adversariales hay que demostrar que un test de regresión REALMENTE falla si se revierte el fix, pero el revisor no puede modificar archivos. → `get_memory 51` (`tests/integration/hook_marker_integration_test.go`)
- **ACR: Codex no garantiza inyección en subagentes**: La ACR acr_acd14d0a-55ad-4820-8a19-b602d089922e confirmó con dos revisores un defecto HIGH: la tabla de hooks de Codex solo registra SessionStart, Stop y UserPromptSubmit; no existe SubagentStart ni… → `get_memory 47`
- **ACR extra Octopus: C-001 sigue abierto y el worktree no es integrable aún con main**: ACR acr_cebbd044-037a-4a21-815a-5419bd8fcdd1 congelada en digest 4b14c76fbf2712c97dbfb19211c8d78ee86cd00e1d63005e55da3ea6cc986b66 produjo ESCALATED: dos revisores independientes confirmaron HIGH… → `get_memory 36`
- **ACR 027 corregida: InlineCostTokens domina Total() si se deriva de sus propios componentes**: Al cerrar C-001 de la ACR reintento (acr_83df0e6d.../acr_3ee3a662..., ambas APPROVED tras re-juicio), se verificó empíricamente que poblar `RouteInput.InlineCostTokens` con cualquier subconjunto de… → `get_memory 33`
- **review_submit exige status="success"|"failure", no hay fuente local del feature ACR**: El tool MCP `review_submit` (usado por el protocolo adversarial-consensus-review) exige que el campo `status` sea exactamente "success" o "failure" (domain.ReviewerResultSuccess/ReviewerResultFailure… → `get_memory 30`
- **ACR posterior a correcciones AAR 027: APPROVED sin defectos confirmados**: ACR acr_de63761f-138f-4ead-900c-6f667379dfaf sobre digest 2b5229a85f3e94560648f3888f6a633371767a09d57c657b2a7baed4160048dd y base 3e43daa. → `get_memory 26` (`adapters/primary/cli/cmd_mcp_octopus_tools.go`)
- **Revisión feature 027 (Octopus AAR): 6 defectos reales encontrados**: Revisión de código del working tree de la feature 027 (Octopus AAR). → `get_memory 20` (`application/usecases/octopus_route_plan.go`)
- **BuildContextPack prioriza por relevancia, no excluye: no basta para aislar contexto delegado**: Hallazgo al implementar el paquete de contexto delegado de Octopus AAR (feature 027, AC-006). → `get_memory 13`

## Actividad Reciente (auto)

- Comandos: cd /repo; S=/tmp/scratch; ./mem docs export constitution -o $S/constitution.md &&… → `get_memory 88`
- Comandos: cd /repo; sed -n 686,703p .specify/memory/constitution.md; sed -n 20,40p adapters/primary/cli/cmd_context.go; sed -n 135,150p adapters/primary/cli/cmd_hook.go… → `get_memory 86`
- Editó: /repo/specs/033-native-context-compression/tasks.md. → `get_memory 84`
- Editó: /repo/specs/033-native-context-compression/spec.md, /repo/specs/033-native-context-compression/data-model.md… → `get_memory 83`
- Editó: /repo/specs/033-native-context-compression/spec.md. → `get_memory 80`

## Código indexado

331 archivos, 2549 símbolos, 6097 relaciones. Paquetes principales: cli, persistence, domain, main, usecases. Usa search_code/get_symbol/list_dependencies para consultarlo.

## Grafo de código externo (codebase-memory-mcp)

Proyecto indexado: `repo` — pásalo tal cual en el parámetro `project` de sus tools.

Grafo estructural indexado: 8867 nodos, 28557 relaciones. Lenguajes: Go (462), YAML (5), Bash (5), TypeScript (2), Python (1).

Módulos de facto (clusters):
- **application** — 333 símbolos, cohesión 0.78 · Close, Open, RecordFix, Scan, SubmitReviewerResult
- **application** — 234 símbolos, cohesión 0.69 · Build, Init, NewMemoryRepository, NewSessionRepository, New
- **domain** — 218 símbolos, cohesión 0.88 · RouteTask, registerOctopusTools, Route, unidadDelegable, capacidadesPlenas
- **adapters** — 176 símbolos, cohesión 0.65 · fail, Run, FindRoot, CmdUninstall, CmdInstall
- **adapters** — 171 símbolos, cohesión 0.80 · captureStdout, CmdUsage, Record, CmdDoctor, CaptureLearnings
- **adapters** — 168 símbolos, cohesión 0.84 · FindRoot, CmdHook, Read, hookUserPromptSubmit, Key

Hotspots (más referenciados): Close (fan-in 196), Init (fan-in 79), Open (fan-in 78), NewMemoryRepository (fan-in 69), InsertMemory (fan-in 64), fail (fan-in 55).

> Para consultas estructurales profundas (quién llama a qué, trazas de llamadas, impacto de un diff) usa las tools del proveedor externo: search_graph, trace_path, query_graph, get_architecture, detect_changes. gomemory guarda el PORQUÉ (decisiones, sinapsis); el grafo externo responde el QUÉ/CÓMO del código.

## 🧭 Anclas sin evidencia

> Hipótesis, no orden de borrado: verifica y usa judge_memories/forget_memory.

- [4] «Los releases de GoMemory no usan un ciclo CI separado» — `.github/workflows/ci.yml` → huérfana candidata (no está en disco ni en el índice)

## Sesión Activa

- Iniciada: 2026-09-25 15:35:59

## Sesiones Recientes

- 2026-09-25 14:11:17 → 2026-09-25 14:11:24: Objetivo: /speckit-plan de la feature 033 (motor nativo de compresión con prácticas de Headroom).
Hallazgos: línea base (estructural: 3,9 % en JSON, 0,2 % en contexto, 15,8 % en logs); runtimes…
- 2026-09-25 11:54:06 → 2026-09-25 13:50:39: Objetivo: especificar cómo llevar la optimización de Headroom a gomemory (/speckit-specify).
Hallazgos: el puerto application/ports/compressor.go ya reserva niveles Semantic/Aggressive; hoy solo…
- 2026-09-25 08:48:17 → 2026-09-25 09:18:35: (sin resumen)
- 2026-09-24 11:16:48 → 2026-09-24 11:56:37: Objetivo: ejecutar una ACR (solo informe, sin correcciones) sobre el working tree sin commitear de la feature 032-opencode-v2-plugin-compat.

Hallazgos: target congelado digest d9215357 (base…

