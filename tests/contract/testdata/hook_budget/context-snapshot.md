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

- [331] "Guarda de reentrada de hooks: duplicado solo si el dueño del lock sigue vivo (no \"mismo hash en 5 s\")" ↔ [328] "Plan 035: decisiones de diseño (identidad por session_id, FitHookOutput, ToolOutput mode, Grep no excluido por T057, test Octopus requiere autorización)"
- [328] "Plan 035: decisiones de diseño (identidad por session_id, FitHookOutput, ToolOutput mode, Grep no excluido por T057, test Octopus requiere autorización)" ↔ [326] "Spec 035: decisiones de clarify (limpieza de duplicados en update/install, recordatorio de plan diferenciado por señal, observabilidad en el registro de canales)"
- [332] "Tasks 035 generadas: 87 tareas, MVP = F1+F2+US1; el plugin de OpenCode vive en infrastructure/plugin/opencode/gomemory.ts" ↔ [331] "Guarda de reentrada de hooks: duplicado solo si el dueño del lock sigue vivo (no \"mismo hash en 5 s\")"
- [326] "Spec 035: decisiones de clarify (limpieza de duplicados en update/install, recordatorio de plan diferenciado por señal, observabilidad en el registro de canales)" ↔ [321] "Spec 035: hooks de Claude degradados por doble registro y tope de 10k, avisos que cruzan de conversación, compresión de salidas destructiva"
- [222] "Preferencia: conservar ceremonia y orden de tasks durante speckit-implement" ↔ [221] (memoria previa)
- [305] "Cobertura agregada Feature 034 y brecha constitucional heredada" ↔ [304] "Feature 034: cierre FR-024 y cobertura de tres paquetes"
- [304] "Feature 034: cierre FR-024 y cobertura de tres paquetes" ↔ [303] "Feature 034: selección vacía y scope global de install corregidos"
- [302] "ACR worktree Feature 034: selección vacía y alcance global (suspect)" ↔ [300] "Feature 034 US4: aviso de versión y checksum implementados (hallazgos)"
- [300] "Feature 034 US4: aviso de versión y checksum implementados (hallazgos)" ↔ [299] "Feature 034 US2: desinstalación sin rastros implementada (hallazgos)"
- [299] "Feature 034 US2: desinstalación sin rastros implementada (hallazgos)" ↔ [297] "Feature 034 US1: hallazgos de implementación (dialecto de session-start, PATH real en pruebas de update)"
- [297] "Feature 034 US1: hallazgos de implementación (dialecto de session-start, PATH real en pruebas de update)" ↔ [293] "Plan 034: decisiones de diseño y defectos latentes descubiertos (export 0644, .env.example ausente)"
- [293] "Plan 034: decisiones de diseño y defectos latentes descubiertos (export 0644, .env.example ausente)" ↔ [287] "Feature 034 console-lifecycle: binario global único, solo aviso de versión y desinstalación sin rastros"
_Orden: masa = centralidad en el grafo de memorias sembrado en sesión activa (5) + anclas a hotspot (3); no mide importancia ni corrección._

## Preferencias del Usuario

- **Preferencia: conservar ceremonia y orden de tasks durante speckit-implement**: La persona exige seguir estrictamente el orden y la ceremonia de tasks.md durante la implementación: TDD, dependencias, marcado de checkboxes, validaciones por fase y pedir aprobación solo en tareas… → `get_memory 222`

## Decisiones de Arquitectura

- **Plan 035: decisiones de diseño (identidad por session_id, FitHookOutput, ToolOutput mode, Grep no excluido por T057, test Octopus requiere autorización)**: specs/035-hook-integrity-compression/plan.md + research.md (2026-10-04):
- Conversación nueva = session_id del payload de session-start distinto del de .memory/.conversation. → `get_memory 328`
  → `specs/035-hook-integrity-compression/plan.md`
- **Plan 034: decisiones de diseño y defectos latentes descubiertos (export 0644, .env.example ausente)**: specs/034-console-lifecycle/plan.md (2026-09-26). → `get_memory 293`
  → `specs/034-console-lifecycle/plan.md`

## Decisiones Técnicas

- **Tasks 035 generadas: 87 tareas, MVP = F1+F2+US1; el plugin de OpenCode vive en infrastructure/plugin/opencode/gomemory.ts**: specs/035-hook-integrity-compression/tasks.md (2026-10-04): 87 tareas (US1 20, US2 11, US3 14, US4 9, US5 7, más setup, foundational y polish), con TDD estricto. → `get_memory 332`
- **Guarda de reentrada de hooks: duplicado solo si el dueño del lock sigue vivo (no "mismo hash en 5 s")**: Corrección del diseño R4 de la 035 (2026-10-04). → `get_memory 331`
- **Spec 035: decisiones de clarify (limpieza de duplicados en update/install, recordatorio de plan diferenciado por señal, observabilidad en el registro de canales)**: Decisiones tomadas con la persona en /speckit-clarify (2026-10-04), specs/035-hook-integrity-compression:
1) Hooks duplicados (usuario + proyecto): la limpieza automática ocurre en `mem update` y en… → `get_memory 326`
- **Release v2.27.0 publicado (feature 034 console-lifecycle)**: v2.27.0 se publicó el 2026-09-26 en GitHub como Latest, con 5 binarios (darwin amd64/arm64, linux amd64/arm64, windows amd64) y checksums.txt. → `get_memory 316`
- **Cobertura global Go con binario instrumentado y perfiles unidos**: Para medir cobertura real de CLI ejecutada por tests/contract, scripts/coverage.sh ejecuta toda la suite con -coverpkg=./..., repite contrato con binario go build -cover -coverpkg=./... → `get_memory 306`
- **Feature 034 console-lifecycle: binario global único, solo aviso de versión y desinstalación sin rastros**: Spec en specs/034-console-lifecycle/spec.md (4 historias: P1 binario único, P2 desinstalación sin rastros, P3 consola guiada al estilo skills.sh, P4 aviso de versión al estilo Codex). → `get_memory 287`
- **Release v2.26.3 publicado (C-004 permisos .memory + C-001 snapshot atómico)**: v2.26.3 está publicado en GitHub (latest), con 5 binarios y checksums.txt, y la rama por defecto es main. → `get_memory 273`
- **Release v2.26.1 usa el skip de GitHub porque origin no es GitLab**: El worktree de v2.26.1 está en main y origin apunta a https://github.com/Sayoner-000/gomemory.git. → `get_memory 268`

## Patrones y Convenciones

- **Release: los recuentos de tools MCP de la documentación se actualizan en cada release que añade tools**: En el release de v2.26.2 solo se actualizó el badge de versión, y quedó desfasado el badge MCP ("28_core_tools"), que no cambiaba desde v2.26.0, cuando entraron pack_retrieve y pack_savings. → `get_memory 275`

## Bugfixes

- **Feature 034: FR-024 en update y FR-004a en install cerrados (T084/T085)**: Causa raíz: CmdUpdate usaba fail()/os.Exit en cada paso y nunca acumulaba StepResult, así que no tenía resumen final. → `get_memory 313`
  → `adapters/primary/cli/cmd_update.go`
- **Feature 034 T082 cerrado: nota Codex contradictoria en uninstall --all y copias de otra arquitectura**: Validación de quickstart 034 en sandbox (2026-09-26). → `get_memory 307`
  → `adapters/primary/cli/uninstall_plan.go`
- **Feature 034: cierre FR-024 y cobertura de tres paquetes**: Se añadió resumen final por pasos a CmdInstall con ✓/⚠ y acción manual para fallos. → `get_memory 304`
  → `adapters/primary/cli/cmd_install.go`
- **Feature 034: selección vacía y scope global de install corregidos**: C-001 de acr_c52c4fa4 reproducido: resolveInstallSelection interpretaba len(Agents)==0 como ausencia incluso con AgentScope guardado, reactivando agentes detectados tras elegir ninguno. → `get_memory 303`
  → `adapters/primary/cli/cmd_install.go`
- **Feature 034 US2: desinstalación sin rastros implementada (hallazgos)**: US2 (T026–T048) completa y en verde. → `get_memory 299`
  → `adapters/primary/cli/uninstall_plan.go`
- **acr_ad72cce1: C-001 cerrado — store global endurecido completo (v2.26.4)**: Causa raíz: igual que el C-004 anterior. → `get_memory 285`
  → `adapters/secondary/persistence/globalstore.go`
- **acr_65a3773c: C-004 y C-001 cerrados (.memory 0700 impuesto y snapshot del grafo atómico)**: C-004: la causa raíz era que persistence.EnsureDir usaba MkdirAll(.memory, 0700), y MkdirAll no corrige un directorio que ya existe. → `get_memory 280`
  → `adapters/secondary/persistence/db.go`
- **acr_0814de3a: contador de tokens único, .memory 0700 y fin del flaky de plan-entered (cierra memoria 140)**: - C-002 (MEDIUM). → `get_memory 270`
- **Marcas de gomemory: una sola regla (domain.IsMarkerLine) y tope de hooks en caracteres**: Hallazgos de acr_6793454b, todos confirmados y corregidos:
- C-001: el tope del canal de hooks de Claude Code son 10 000 CARACTERES (specs/019, research.md), no bytes. → `get_memory 266`
- **C-002/C-001/C-005/C-006 de acr_961a1676 implementados (entrega real ≠ enviada a la sesión)**: C-002. → `get_memory 260`
- **Compresión nativa: C-004 y C-003 de acr_961a1676 corregidos (marca en duplicadas y omisiones por bloque)**: C-004. → `get_memory 257`
- **acr_5836d32d: roundtrip export/import conserva topic_key y source_review_id (bundle v2)**: Causa raíz de C-001 (HIGH): ExportMemory no tenía campos para topic_key ni source_review_id, y ImportMemory no los insertaba. → `get_memory 249`
  → `application/usecases/portability.go`
- **ACR 031 (acr_0106584b): anclas falsas por errores de stat y anclas antiguas sin clasificar, corregidos**: C-001 (HIGH). → `get_memory 225`
  → `application/usecases/build_context.go`

## Aprendizajes Recientes

- **Spec 035: hooks de Claude degradados por doble registro y tope de 10k, avisos que cruzan de conversación, compresión de salidas destructiva**: Evidencia medida el 2026-10-04 sobre v2.27.1 (base de specs/035-hook-integrity-compression/spec.md):
1) Hooks de gomemory registrados a la vez en ~/.claude/settings.json y .claude/settings.json: cada… → `get_memory 321` (`specs/035-hook-integrity-compression/spec.md`)
- **Compresión observable por inferencia: pack explícito y hook de salida opt-in**: La compresión de gomemory tiene dos rutas distintas. → `get_memory 320` (`adapters/primary/cli/hook_tool_output.go`)
- **Badge GitHub Release del README es dinámico: si no se ve la versión nueva es caché, no hay que editarlo**: En el README, la línea 9 contiene img.shields.io/github/v/release/Sayoner-000/gomemory, que lee la API de GitHub; solo el badge "Version" (línea 10) se edita en cada release. → `get_memory 318`
- **Converge 034: FR-024 en update y FR-004a en resumen de install siguen parciales (corrige memoria 304)**: La ejecución de /speckit-converge del 2026-09-26 añadió la Phase 8, con T084 y T085. → `get_memory 311` (`specs/034-console-lifecycle/tasks.md`)
- **ACR worktree Feature 034 acr_57dcc554: APPROVED sin hallazgos**: ACR de solo lectura del worktree completo congelado en HEAD 351215e7e5ef… con digest sha256:9eeb34f272a7d438a45343b2dd0d3ff5cd3b3766e0ebccd05ad410fac053963b. → `get_memory 310`
- **Cobertura agregada Feature 034 y brecha constitucional heredada**: Medición comparable con go test ./... → `get_memory 305` (`specs/034-console-lifecycle/plan.md`)
- **ACR worktree Feature 034: selección vacía y alcance global (suspect)**: ACR solo lectura acr_c52c4fa4 sobre pending-changes de HEAD 351215e7e5ef, digest sha256:c149745fe0fb1003f94af718323bc02dfbfdb376d20b5f2955c4ca3f76f931ab. → `get_memory 302` (`adapters/primary/cli/install_selection.go`)
- **Feature 034 US4: aviso de versión y checksum implementados (hallazgos)**: US4 (T049–T065) completa. → `get_memory 300` (`adapters/primary/cli/cmd_update_check.go`)
- **Feature 034 US1: hallazgos de implementación (dialecto de session-start, PATH real en pruebas de update)**: 1) El payload de SessionStart trae hook_event_name pero NO tool_name, así que detectDialect devuelve neutral también en Claude Code. → `get_memory 297` (`adapters/primary/cli/local_copy.go`)
- **acr_65a3773c: C-002 y C-003 son obsoletos en bc26aea (ya corregidos por 5f732f9)**: El informe ACR acr_65a3773c (sobre bc26aea) partía de una premisa falsa: afirmaba que 07ae0683..bc26aea solo contenía docs y test. → `get_memory 278` (`adapters/secondary/compression/native/engine.go`)
- **[skip ci] omitió el workflow de GoReleaser en el tag v2.26.1**: El workflow .github/workflows/release.yml se activa en push de tags v*. → `get_memory 269`
- **ACR worktree v2.26.0+ (acr_6793454b): APPROVED, 2 SUSPECT LOW, sin correcciones**: ACR solo lectura sobre el worktree sin commit de gomemory (base HEAD 7fac11e8 = v2.26.0; 29 archivos modificados + 3 tests sin trackear; digest… → `get_memory 265`
- **ACR v2.26.0 (acr_961a1676): ESCALATED con 2 CONFIRMED en compresión nativa**: ACR solo lectura sobre v2.25.0..7fac11e (compresión nativa). → `get_memory 255`
- **ACR acr_2c2f9a2b: release v2.25.0 sin hallazgos**: ACR del snapshot HEAD 1bbf25005fc7187af50b795310e6fddd15a1d741 (worktree limpio, tag v2.25.0), con scope [.]: revisó commit de release compuesto por entrada CHANGELOG.md y bump version.Version de… → `get_memory 253` (`version/version.go`)
- **Release v2.23.3 publicada con binarios y changelog**: Release v2.23.3 publicada (2026-09-14): fix(portability) conserva topic_key y source_review_id en export/import (bundle v2 con migración v1 y rechazo de versiones futuras), todas las lecturas… → `get_memory 252`

<!-- mem:volatile -->

## Actividad Reciente (auto)

- Editó: /home/user/home/rcw/gomemory/specs/035-hook-integrity-compression/tasks.md. → `get_memory 334`
- Editó: /home/user/home/rcw/gomemory/specs/035-hook-integrity-compression/tasks.md. → `get_memory 333`
- Editó: /home/user/home/rcw/gomemory/specs/035-hook-integrity-compression/research.md, /home/user/home/rcw/gomemory/specs/035-hook-integrity-compression/data-model.md… → `get_memory 330`
- Editó: /home/user/home/rcw/gomemory/specs/035-hook-integrity-compression/research.md, /home/user/home/rcw/gomemory/specs/035-hook-integrity-compression/data-model.md… → `get_memory 329`
- Comandos: python3 - <<'EOF'
p='spec.md'; s=open(p).read()
anchor="cuando cambia su activación.\n"
i=s.index(anchor)+len(anchor)
s=s[:i]+"- Q: ¿Las protecciones (duplicados descartados, recortes por… → `get_memory 327`

## Código indexado

536 archivos, 3995 símbolos, 8832 relaciones. Paquetes principales: cli, domain, persistence, usecases, main. Usa search_code/get_symbol/list_dependencies para consultarlo.

## Grafo de código externo (codebase-memory-mcp)

Proyecto indexado: `Users-josegomezj-home-rcw-gomemory` — pásalo tal cual en el parámetro `project` de sus tools.

Grafo estructural indexado: 11137 nodos, 36848 relaciones. Lenguajes: Go (598), Bash (6), YAML (5), Python (3), TypeScript (2).

Módulos de facto (clusters):
- **adapters** — 340 símbolos, cohesión 0.69 · fail, FindRoot, registerTools, Key, CmdHook
- **adapters** — 320 símbolos, cohesión 0.70 · WriteFile, Setenv, captureStdout, Mode, CmdUninstall
- **adapters** — 313 símbolos, cohesión 0.71 · openTestDB, InsertMemory, Scan, Scan, Query
- **application** — 250 símbolos, cohesión 0.81 · GetReview, RecordFix, NewTarget, SubmitReviewerResult, NewReviewRepository
- **adapters** — 246 símbolos, cohesión 0.69 · String, keyMsg, New, updateList, Render
- **domain** — 222 símbolos, cohesión 0.81 · RouteTask, registerOctopusTools, unidadDelegable, capacidadesPlenas, RoutePlan

Hotspots (más referenciados): Close (fan-in 270), String (fan-in 143), WriteFile (fan-in 132), Run (fan-in 98), Open (fan-in 95), Init (fan-in 84).

> Para consultas estructurales profundas (quién llama a qué, trazas de llamadas, impacto de un diff) usa las tools del proveedor externo: search_graph, trace_path, query_graph, get_architecture, detect_changes. gomemory guarda el PORQUÉ (decisiones, sinapsis); el grafo externo responde el QUÉ/CÓMO del código.

## 🔥 Memoria conectada a código activo

- **ACR 031 (acr_0106584b): anclas falsas por errores de stat y anclas antiguas sin clasificar, corregidos** — `application/usecases/build_context.go` (fan-in 132, hotspot vigente)
- **acr_65a3773c: C-004 y C-001 cerrados (.memory 0700 impuesto y snapshot del grafo atómico)** — `adapters/secondary/persistence/db.go` (fan-in 95, hotspot vigente)

## 🧭 Anclas sin evidencia

> Hipótesis, no orden de borrado: verifica y usa judge_memories/forget_memory.

- [82] «Spec 029: integridad del veredicto ACR, nacida de la revisión que destapó las regresiones de v2.16.3» — `specs/029-fix-acr-verdict-integrity/spec.md` → huérfana candidata (no está en disco ni en el índice)

## Sesión Activa

- Iniciada: 2026-10-04 08:09:09

## Sesiones Recientes

- 2026-09-27 08:00:30 → 2026-09-27 08:03:00: Objetivo: explicar cómo funciona el compresor y cómo observarlo en inferencias de Codex, Claude Code y OpenCode.

Hallazgos: el proyecto separa ContextPack (`pack_build`) de compresión de salidas de…
- 2026-09-26 16:22:14 → 2026-09-26 16:22:20: Objetivo: publicar v2.27.0 (feature 034).
Logrado: README (badge 2.27.0, árbol de la CLI con install/uninstall/doctor/update, ajuste update_check_disabled), CHANGELOG [v2.27.0] con la entrada de…
- 2026-09-26 15:57:47 → 2026-09-26 16:02:03: Objetivo: ejecutar ACR del worktree sin correcciones y entregar solo el informe.

Hallazgos: no hubo findings confirmados ni suspect tras dos revisiones independientes.
- 2026-09-26 15:44:34 → 2026-09-26 15:56:29: Objetivo: cerrar T082 de la spec 034 (Phase 7).
Hallazgos: F1, bug corregido: uninstall --all anunciaba que conservaba la configuración de Codex que después retiraba.

## Estilo de respuesta

Responde de forma concisa: ve al grano y evita repetir lo que ya está en el contexto, sin omitir información necesaria para la tarea.
