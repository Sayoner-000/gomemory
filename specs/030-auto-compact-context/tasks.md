---

description: "Tareas de implementación de la feature 030: compactación de contexto sin pérdida de memoria"
---

# Tasks: Compactación de contexto sin pérdida de memoria

**Input**: documentos de diseño en `specs/030-auto-compact-context/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/hooks.md, contracts/texts.md, quickstart.md

**Tests**: obligatorios. La constitución (principio III) exige TDD: cada test se
escribe primero y **debe fallar** antes de implementar. Ningún test existente se
modifica; todos los tests de esta feature van en **archivos nuevos**.

**Organization**: tareas agrupadas por historia, para implementar y validar cada
una por separado.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede hacer en paralelo (archivo distinto, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1, US2, US3, US4)

## Convenciones de este repositorio

- Tests unitarios junto al paquete (`<paquete>/<nombre>_test.go`); contratos en
  `tests/contract/`; integración con BD real en `tests/integration/` o en el
  paquete `persistence`.
- Identificadores en inglés; comentarios y documentación en español latino
  (memoria 132).
- Hooks best-effort: siempre terminan con código 0.
- Compilar: `go build -o /tmp/mem-030 ./infrastructure/`. Probar: `go test ./...`.
  Lint: `golangci-lint run`.

---

## Phase 1: Setup (confirmar capacidades contra los clientes en ejecución)

**Purpose**: regla de trabajo 1, primero reproducir la realidad. La tabla R1 de
research.md sale de documentación y tipos; aquí se confirma en vivo antes de
programar las integraciones.

- [X] T001 ⚠ NO EJECUTADA EN VIVO — registrar un `PostCompact` temporal en `~/.claude/settings.json` habría modificado la configuración de hooks que gobierna esta misma sesión, y "compactar a mano" para probarlo habría compactado esta conversación de trabajo. Se decidió no asumir ese costo sin pedirlo explícitamente. Capacidad C6 de claude queda confirmada solo por evidencia documental (research.md R1: documentación oficial de hooks, ejemplo JSON con `compact_summary`). Confirmación en vivo diferida a quickstart Q0, a ejecutar por el usuario.
- [X] T002 [P] ⚠ NO EJECUTADA EN VIVO — requiere una sesión interactiva de Codex con delegación real a subagente; no disponible de forma segura en este entorno no interactivo. Capacidad C3 de codex confirmada solo por documentación oficial + `strings` sobre el binario instalado (0.154.0: contiene `SubagentStop`, `last_assistant_message`). Diferida a quickstart Q0.
- [X] T003 [P] ⚠ NO EJECUTADA EN VIVO — requiere una sesión interactiva de OpenCode con delegación real y una compactación real. Capacidades C3/C6 de opencode confirmadas solo por los tipos instalados (`@opencode-ai/plugin`, `@opencode-ai/sdk`: `tool.execute.after`, `EventSessionCompacted`, `AssistantMessage.summary`). Diferida a quickstart Q0.
- [X] T004 Ninguna celda cambió de estado (no hubo confirmación en vivo que las contradijera); la tabla R1 se mantiene con la evidencia documental ya registrada. Se implementan T035/T038 (C6) y T045/T049 (C3) sobre esa base, con el entendido de que el usuario debe correr quickstart Q0 antes de considerar la feature verificada end-to-end.

**Checkpoint**: la tabla de capacidades refleja los clientes reales.

---

## Phase 2: Foundational (bloquea todas las historias)

**Purpose**: vocabulario de capacidades, textos neutrales y cierre del defecto
latente de la recuperación (research.md R4), que se paga antes que cualquier
historia porque todas lo activarían.

**⚠️ CRITICAL**: ninguna historia empieza antes de terminar esta fase.

### Capacidades del cliente (FR-020, FR-021)

- [X] T005 [P] Escribir el test de contrato INV-C1 en tests/contract/compaction_capabilities_test.go: para cada agente de `domain.KnownAgents` y cada una de las 6 capacidades, exactamente una de dos (`Compaction[c] == true` o `CompactionUnavailable[c] != ""`); además, verificar los valores de la tabla R1 (claude sin C1 y con motivo; codex sin C1 ni C6). Debe fallar.
- [X] T006 Crear domain/compaction_capabilities.go con el tipo `CompactionCapability` y las 6 constantes (`CompactionPreCompactInput`, `CompactionPostCompactChannel`, `CompactionSubagentFinalText`, `CompactionAgentTurnChannel`, `CompactionHumanChannel`, `CompactionSummaryInput`) y una función `AllCompactionCapabilities()` que las devuelva en orden C1–C6
- [X] T007 Añadir los campos `Compaction map[CompactionCapability]bool` y `CompactionUnavailable map[CompactionCapability]string` a `AgentCapability`, y los valores de claude, codex y opencode según la tabla R1 y la decisión R12, en domain/agents.go (dep T006). T005 debe pasar.

### Textos neutrales (FR-018, SC-007)

- [X] T008 [P] Escribir tests/contract/compaction_texts_test.go: recorre `domain.CompactionTexts()` y falla si algún texto contiene (sin distinguir mayúsculas) `claude`, `codex`, `opencode`, `cursor` o un comando de cliente con la forma `/palabra`; exige además que `RecoverySteps` contenga `save_session_summary` y NO contenga `end_session`. Debe fallar.
- [X] T009 Crear domain/compaction_texts.go con las constantes `RecoverySteps`, `CompactorPersistOrder`, `CompactionContextHeader`, `CompactionEmptySessionNote`, `CompactionOmittedNoteFormat` y `AgentPrepareNotice`, con el texto literal de specs/030-auto-compact-context/contracts/texts.md, y la función `CompactionTexts() []string`. T008 debe pasar.

### Resumen de sesión sin cerrar la sesión (FR-023, R4, R5)

- [X] T010 [P] Escribir adapters/secondary/persistence/session_summary_test.go con BD real: `UpdateSessionSummary` cambia `summary` y deja `ended_at` en NULL; repetirlo con el mismo texto no falla ni cambia nada más; sobre una sesión cerrada o inexistente devuelve un error. Debe fallar.
- [X] T011 [P] Declarar el puerto `SessionSummaryUpdater { UpdateSummary(id, summary string) error }` en application/ports/session_summary.go
- [X] T012 Implementar `UpdateSessionSummary(db, id, summary)` (`UPDATE sessions SET summary = ? WHERE id = ? AND ended_at IS NULL`, con parámetros bind y error si no afecta filas) en adapters/secondary/persistence/session.go, y el método `(*SessionRepository).UpdateSummary` con `var _ ports.SessionSummaryUpdater` en adapters/secondary/persistence/repositories.go (dep T011). T010 debe pasar.
- [X] T013 Añadir el campo `SessionSummaries ports.SessionSummaryUpdater` a `Deps` en adapters/primary/cli/deps.go y cablearlo con el `SessionRepository` concreto en infrastructure/container.go (dep T012)

### Herramienta MCP `save_session_summary`

- [X] T014 [P] Escribir adapters/primary/cli/cmd_mcp_session_summary_test.go contra el servidor MCP real: la herramienta está registrada; con sesión activa actualiza `summary` sin cerrarla; sin sesión activa abre una y guarda el resumen; con `summary` vacío responde un error de validación; la descripción no pasa de 250 caracteres. Debe fallar.
- [X] T015 Añadir `ToolSaveSessionSummary = "save_session_summary"` a domain/mcp_tools.go, dentro de `MCPMemoryTools` (así queda auto-aprobable por `MCPAutoApprovableTools`)
- [X] T016 Registrar la herramienta `save_session_summary` en adapters/primary/cli/cmd_mcp.go junto a `end_session`, según contracts/texts.md, usando `deps.SessionRepo.Active`/`Start` y `deps.SessionSummaries.UpdateSummary` (dep T013, T015). T014 debe pasar.
- [X] T017 Añadir `"gomemory_save_session_summary"` a la lista de herramientas de memoria en infrastructure/plugin/opencode/gomemory.ts, y a cualquier otra lista que exija `go test ./tests/contract/ -run 'ToolSync|DomainRefleja'` hasta que pase

### Recuperación tras compactar (R4)

- [X] T018 [P] Escribir adapters/primary/cli/cmd_hook_post_compact_test.go: sin sesión activa, `mem hook post-compact` abre una; la salida empieza por `domain.RecoverySteps` y no contiene `end_session`. Debe fallar.
- [X] T019 En adapters/primary/cli/cmd_hook.go, reemplazar la constante `compactionRecoveryInstructions` por `domain.RecoverySteps` y hacer que `hookPostCompact` abra una sesión si `deps.SessionRepo.Active` no devuelve ninguna, antes de imprimir (dep T009). T018 debe pasar.

**Checkpoint**: tras compactar ya no se cierra la sesión. `go test ./...` en verde.

---

## Phase 3: User Story 1 - La memoria de la sesión sobrevive a la compactación (Priority: P1) 🎯 MVP

**Goal**: el compresor (C1) y el agente que reanuda (C2) reciben la memoria de la
sesión, breve y acotada; el contexto de proyecto posterior va en modo índice,
dentro del presupuesto de arranque.

**Independent Test**: quickstart Q1 y Q2.

### Tests for User Story 1 ⚠️

- [X] T020 [P] [US1] Escribir adapters/secondary/persistence/memory_session_test.go con BD real: `ListBySession` devuelve solo las memorias de esa sesión y proyecto, excluye las borradas, ordena de la más reciente a la más antigua, respeta `limit`, y con `sessionID` vacío devuelve una lista vacía y `nil`. Debe fallar.
- [X] T021 [P] [US1] Escribir application/usecases/build_compaction_context_test.go con dobles locales: orden (decisiones y bugfixes primero, luego por recencia); extracto de 300, resumen previo de 600 y prompt de 200 caracteres; tope del 40 % de `Budget`; nota de omitidas con el recuento correcto; nota de sesión vacía; sin tope con `Budget <= 0`; sin sesión activa devuelve una cadena vacía. Debe fallar.

### Implementation for User Story 1

- [X] T022 [P] [US1] Declarar el puerto `SessionMemoryLister { ListBySession(project, sessionID string, limit int) ([]domain.Memory, error) }` en application/ports/session_memory.go
- [X] T023 [US1] Implementar `ListMemoriesBySession` con SQL bind en adapters/secondary/persistence/memory.go, y `(*MemoryRepository).ListBySession` con `var _ ports.SessionMemoryLister` en adapters/secondary/persistence/repositories.go (dep T022). T020 debe pasar.
- [X] T024 [US1] Implementar `BuildCompactionContext(sessions ports.SessionQuerier, mems ports.SessionMemoryLister, project string, budget int) (string, error)` en application/usecases/build_compaction_context.go, con los textos de domain/compaction_texts.go (dep T022). T021 debe pasar.
- [X] T025 [US1] Añadir a `Deps` los campos `SessionMemories ports.SessionMemoryLister` y `CompactContextBuilder ports.ContextBuilder` en adapters/primary/cli/deps.go, y cablearlos en infrastructure/container.go: el segundo es una copia del `usecases.Builder` vigente con `IndexMode = true` (dep T023)
- [X] T026 [US1] Ampliar adapters/primary/cli/cmd_hook_post_compact_test.go con un caso nuevo (función nueva en el archivo creado en T018): la salida tiene el orden `RecoverySteps` → `## Memoria de esta sesión` → contexto en modo índice; el total no supera `Settings.Budget`; los conflictos sin resolver aparecen íntegros. Debe fallar.
- [X] T027 [US1] En adapters/primary/cli/cmd_hook.go, hacer que `printRecoveryAndContext` componga `RecoverySteps` + `BuildCompactionContext` + `CompactContextBuilder` con `Budget` = presupuesto − lo ya emitido (dep T024, T025). T026 debe pasar.
- [X] T028 [P] [US1] Escribir adapters/primary/cli/cmd_hook_compaction_context_test.go: `mem hook compaction-context` imprime el contexto de compactación seguido de `CompactorPersistOrder`; sin sesión activa imprime solo la orden. Debe fallar.
- [X] T029 [US1] Añadir el subcomando `compaction-context` al `switch` de `CmdHook` en adapters/primary/cli/cmd_hook.go (dep T024). T028 debe pasar.
- [X] T030 [P] [US1] Escribir tests/contract/opencode_compaction_test.go: `gomemory.ts` llama a `["hook", "compaction-context"]` dentro de `experimental.session.compacting`; ya no llama a `post-compact` ahí; maneja el evento `session.compacted` llamando a `["hook", "post-compact"]`; y en `experimental.chat.system.transform` inyecta una sola vez la recuperación pendiente. Debe fallar.
- [X] T031 [US1] Actualizar infrastructure/plugin/opencode/gomemory.ts: `session.compacting` empuja a `output.context` la salida de `mem hook compaction-context`; el evento `session.compacted` guarda en un `Map` por `sessionID` la salida de `mem hook post-compact`; `system.transform` la inyecta y la borra del `Map` (dep T029). T030 debe pasar.
- [ ] T032 [US1] Compilar `/tmp/mem-030` y ejecutar quickstart Q1 y Q2; anotar en specs/030-auto-compact-context/quickstart.md, sección «Evidencia», los tamaños antes y después (SC-002: ≤ 50 %)

**Checkpoint**: US1 funciona sola en los tres clientes.

---

## Phase 4: User Story 2 - El resumen compactado se guarda como memoria (Priority: P2)

**Goal**: cada compactación deja el resumen persistido en la sesión, una vez, sin
cerrarla. La integración lo hace sola donde recibe el resumen (C6); en los demás
clientes, lo hace el agente por la orden de `RecoverySteps` (ya cubierta en la
fase 2).

**Independent Test**: quickstart Q3.

### Tests for User Story 2 ⚠️

- [X] T033 [P] [US2] Escribir adapters/primary/cli/cmd_hook_compact_summary_test.go: `mem hook compact-summary` acepta `{"compact_summary": ...}` y `{"summary": ...}`; persiste en la sesión activa; es idempotente; sin sesión activa abre una; con texto vacío no hace nada; con `--emit=json` imprime `{}`; sin `--emit` no imprime nada. Debe fallar.
- [X] T034 [P] [US2] Escribir adapters/primary/setup/claude_code_postcompact_test.go: la instalación registra `PostCompact` → `mem hook compact-summary`; `hookCommandIsGomemory("mem hook compact-summary")` es verdadero, así que la desinstalación lo retira; `PreCompact` sigue sin registrarse. Debe fallar.

### Implementation for User Story 2

- [X] T035 [US2] Añadir el subcomando `compact-summary` al `switch` de `CmdHook` en adapters/primary/cli/cmd_hook.go, usando `readHookStdin`, `deps.SessionRepo` y `deps.SessionSummaries` (dep T013). T033 debe pasar.
- [X] T036 [US2] Añadir `"PostCompact": {{matcher: "", sub: "compact-summary"}}` a `buildClaudeHookEvents`, y `strings.Contains(cmd, "hook compact-summary")` a `hookCommandIsGomemory`, en adapters/primary/setup/claude_code_setup.go. T034 debe pasar.
- [X] T037 [US2] Ampliar tests/contract/opencode_compaction_test.go con un caso nuevo (función nueva): tras `session.compacted`, el plugin busca el último mensaje con `summary: true` en `client.session.messages` y envía su texto a `["hook", "compact-summary"]` por stdin. Debe fallar.
- [X] T038 [US2] Implementar en infrastructure/plugin/opencode/gomemory.ts la búsqueda del mensaje de resumen y el envío por `memWithStdin(["hook", "compact-summary"], JSON.stringify({ summary }))`, best-effort con registro `channel-error` si falla (dep T035). T037 debe pasar. Omitir si T004 declaró C6 no disponible en opencode.
- [ ] T039 [US2] Instalar con scripts/install.sh, refrescar la integración y ejecutar quickstart Q3: CLI, claude en vivo (resumen guardado sin acción del agente) y codex en vivo (el agente llama a `save_session_summary`). Anotar la evidencia en specs/030-auto-compact-context/quickstart.md

**Checkpoint**: US1 y US2 funcionan cada una por su lado.

---

## Phase 5: User Story 3 - Captura pasiva de aprendizajes de subagentes (Priority: P3)

**Goal**: los ítems de la sección de aprendizajes del mensaje final de un
subagente se guardan solos, sin duplicados y sin acciones del agente.

**Independent Test**: quickstart Q4.

### Tests for User Story 3 ⚠️

- [X] T040 [P] [US3] Escribir domain/learnings_test.go (tabla): encabezados `##`/`###` «Aprendizajes», «Aprendizajes clave», «Key learnings», «Learnings», con o sin dos puntos y sin distinguir mayúsculas; toma la ÚLTIMA sección; corta en el siguiente encabezado; ítems numerados y viñetas; limpia `**`, `` ` `` y `*`; descarta los de menos de 20 caracteres o 4 palabras; sin sección devuelve nil. Debe fallar.
- [X] T041 [P] [US3] Escribir application/usecases/capture_learnings_test.go con dobles locales: N ítems válidos → N inserciones `learning` con `TopicKey` = `passive:` + 16 hex de sha256 del ítem normalizado; mismo ítem con otro formato → mismo tópico; tope de 10; el contenido termina en `Procedencia: subagente`; se asigna la sesión activa. Debe fallar.

### Implementation for User Story 3

- [X] T042 [P] [US3] Implementar `ExtractLearnings(text string) []string` (pura, sin I/O) en domain/learnings.go. T040 debe pasar.
- [X] T043 [US3] Implementar `CaptureLearnings(mems ports.MemoryRepository, sessions ports.SessionQuerier, project, text string) (int, error)` en application/usecases/capture_learnings.go (dep T042). T041 debe pasar.
- [X] T044 [P] [US3] Escribir adapters/primary/cli/cmd_hook_subagent_capture_test.go: `mem hook subagent-stop` con `last_assistant_message` crea las memorias extraídas y **además** sigue registrando el «Checkpoint de subagente» con el mismo payload; repetirlo no crea memorias nuevas; con `--emit=json` imprime `{}`. Debe fallar.
- [X] T045 [US3] En adapters/primary/cli/cmd_hook.go, hacer que `hookSubagentStop` lea el payload una sola vez, llame a `CaptureLearnings` y le pase el mismo payload al checkpoint (añadiendo una variante de `recordActivityCheckpoint` que reciba el payload ya leído), e imprima `{}` con `--emit=json` (dep T043). T044 debe pasar.
- [X] T046 [P] [US3] Escribir adapters/primary/setup/codex_subagent_stop_test.go: el ÚLTIMO elemento de `CodexGomemoryHooks()` es `{Event: "SubagentStop", Sub: "subagent-stop", Emit: "json"}` y `CodexGomemoryHooks()[2]` sigue siendo `Stop`. Debe fallar.
- [X] T047 [US3] Añadir `{Event: "SubagentStop", Sub: "subagent-stop", Emit: "json"}` AL FINAL de `codexGomemoryHooks` en adapters/primary/setup/codex_setup.go. T046 y adapters/primary/cli/codex_gomemory_hooks_test.go deben pasar sin tocarlos.
- [X] T048 [US3] Ampliar tests/contract/opencode_compaction_test.go con un caso nuevo (función nueva): `tool.execute.after` con `input.tool === "task"` envía `{ last_assistant_message: output.output }` a `["hook", "subagent-stop"]`. Debe fallar.
- [X] T049 [US3] Implementar ese envío en `tool.execute.after` de infrastructure/plugin/opencode/gomemory.ts, best-effort con `channel-error` (dep T045). T048 debe pasar. Omitir si T004 declaró C3 no disponible en opencode.
- [ ] T050 [US3] Ejecutar quickstart Q4 (CLI) y una delegación real en cada cliente; anotar la evidencia en specs/030-auto-compact-context/quickstart.md

**Checkpoint**: US3 funciona sola.

---

## Phase 6: User Story 4 - Aviso de preparación al agente (Priority: P4)

**Goal**: con la opción activada, al superar el umbral el agente recibe por C4 un
aviso para guardar lo pendiente, sin bloquear ni prolongar el turno. Con la
opción apagada, nada cambia.

**Independent Test**: quickstart Q5.

### Tests for User Story 4 ⚠️

- [X] T051 [P] [US4] Escribir adapters/secondary/persistence/settings_compact_notice_test.go: `DefaultSettings().CompactAgentNotice == false`; ida y vuelta por JSON con la clave `compact_agent_notice`. Debe fallar.
- [X] T052 [P] [US4] Escribir adapters/primary/cli/cmd_hook_agent_notice_test.go: (a) con la opción apagada, la salida de `turn-end` es byte a byte igual en los dialectos `claude`, `json` y `text` a la que produce la rama vigente con la misma huella; (b) con la opción encendida, `claude` emite un único objeto con `systemMessage` y `hookSpecificOutput.additionalContext` = `AgentPrepareNotice`, sin `decision` ni `continue`; `json` y `text` emiten solo el aviso a la persona y crean `.memory/.pending-agent-notice`; (c) `user-prompt-submit` antepone el aviso y borra el archivo, y una segunda llamada ya no lo emite; (d) `mem hook agent-notice` lo emite una sola vez; (e) `post-compact` borra el archivo. Debe fallar.
- [X] T053 [P] [US4] Escribir adapters/primary/tui/tui_compact_notice_test.go: la pantalla Configuración muestra la fila «Aviso de compactación al agente» junto al umbral; alternarla persiste `compact_agent_notice`. Debe fallar.

### Implementation for User Story 4

- [X] T054 [US4] Añadir `CompactAgentNotice bool` con la etiqueta json `compact_agent_notice,omitempty` a `Settings` en adapters/secondary/persistence/settings.go (el default `false` ya es el valor cero). T051 debe pasar.
- [X] T055 [US4] En adapters/primary/cli/cmd_hook.go (`hookTurnEnd`) y adapters/primary/cli/hook_dialect.go, emitir el aviso según contracts/hooks.md cuando `computeCompactNudge` dispare y `CompactAgentNotice` esté activo: sobre combinado en `claude` y archivo pendiente en `json` y `text`. Con la opción apagada, la rama actual queda intacta (dep T009, T054).
- [X] T056 [US4] En adapters/primary/cli/cmd_hook.go: consumir `.pending-agent-notice` en `hookUserPromptSubmit`, añadir el subcomando `agent-notice` y borrar el archivo en `hookPostCompact` (dep T055). T052 debe pasar.
- [X] T057 [US4] Añadir la fila de alternancia junto a `configRowEditCompactThreshold` en adapters/primary/tui/tui.go, siguiendo el patrón de las filas booleanas existentes (dep T054). T053 debe pasar.
- [X] T058 [US4] Ampliar tests/contract/opencode_compaction_test.go con un caso nuevo (función nueva): `experimental.chat.system.transform` empuja la salida no vacía de `["hook", "agent-notice"]`. Debe fallar.
- [X] T059 [US4] Implementarlo en infrastructure/plugin/opencode/gomemory.ts, junto a la llamada vigente a `["hook", "nudge"]` (dep T056). T058 debe pasar.
- [ ] T060 [US4] Ejecutar quickstart Q5 y anotar la evidencia en specs/030-auto-compact-context/quickstart.md

**Checkpoint**: las cuatro historias funcionan por separado.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T061 [P] Escribir adapters/primary/cli/cmd_doctor_compaction_test.go: `mem doctor` lista, por agente instalado, C1–C6 con «sí» o el motivo declarado. Debe fallar.
- [X] T062 Implementar esa sección en adapters/primary/cli/cmd_doctor.go leyendo `domain.KnownAgents` y `domain.AllCompactionCapabilities()` (dep T007). T061 debe pasar.
- [X] T063 [P] Documentar en docs/MANUAL.md la compactación sin pérdida (qué hace gomemory antes y después de compactar, la herramienta `save_session_summary`, la captura pasiva con el formato «## Aprendizajes clave», la opción de aviso y la tabla de capacidades), y añadir la entrada en CHANGELOG.md
- [X] T064 [P] Añadir en specs/008-reduce-context-footprint/spec.md, bajo FR-008, una referencia cruzada de una línea a la feature 030 (sin cambiar el requisito)
- [X] T065 gofmt limpio; go build y go test ./... en verde; go test -race ./... (repo completo, no solo los tres paquetes previstos) en verde; golangci-lint v2.13.2 (v1.64.8 es incompatible con go1.27.1 — ver memoria 172): 0 issues tras corregir 1 QF1001 (De Morgan) en un test nuevo. Sin supresiones.
- [X] T066 Parte automatizada de Q6/Q7 (tests de contrato de agnosticismo/capacidades, go test ./..., golangci-lint) verificada en T065. ⚠ Falta la parte en vivo: instalar con scripts/install.sh y confirmar con `mem doctor` los hooks nuevos en los tres clientes reales — mismo motivo que T001-T004.
- [ ] T067 Ejecutar la revisión adversarial por consenso (habilidad `adversarial-consensus-review`) sobre el cambio completo y cerrar los hallazgos confirmados hasta obtener APPROVED

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (fase 1)**: sin dependencias. Condiciona T038 y T049 a través de T004.
- **Foundational (fase 2)**: depende de la fase 1 y bloquea todas las historias.
- **US1 (fase 3)**: depende de la fase 2.
- **US2 (fase 4)**: depende de la fase 2 (T013, T016, T019). No depende de US1.
- **US3 (fase 5)**: depende solo de la fase 2 (textos y capacidades). No depende de US1 ni de US2.
- **US4 (fase 6)**: depende de la fase 2. No depende de otras historias.
- **Polish (fase 7)**: depende de las historias que se vayan a entregar.

### Archivos compartidos entre historias (orden obligatorio)

Estos archivos los tocan varias historias; sus tareas NO se hacen en paralelo:

- adapters/primary/cli/cmd_hook.go: T019 → T027 → T029 → T035 → T045 → T055 → T056
- infrastructure/plugin/opencode/gomemory.ts: T017 → T031 → T038 → T049 → T059
- tests/contract/opencode_compaction_test.go: T030 → T037 → T048 → T058
- adapters/primary/cli/deps.go e infrastructure/container.go: T013 → T025

### Within Each User Story

- El test se escribe primero y falla; después se implementa; el test pasa.
- Puerto → persistencia → caso de uso → hook → integración del cliente → validación en vivo.

### Parallel Opportunities

- Fase 1: T002 y T003 en paralelo tras T001.
- Fase 2: T005, T008, T010, T011, T014 y T018 (tests y puerto, en archivos distintos).
- US1: T020, T021, T022 y T028 en paralelo; T030 también.
- US2: T033 y T034 en paralelo.
- US3: T040, T041, T044 y T046 en paralelo; T042 en paralelo con T041.
- US4: T051, T052 y T053 en paralelo.
- Fase 7: T061, T063 y T064 en paralelo.

---

## Parallel Example: User Story 1

```bash
# Tests de US1 (archivos distintos, todos deben fallar primero):
Task: "Escribir adapters/secondary/persistence/memory_session_test.go (T020)"
Task: "Escribir application/usecases/build_compaction_context_test.go (T021)"
Task: "Escribir adapters/primary/cli/cmd_hook_compaction_context_test.go (T028)"
Task: "Escribir tests/contract/opencode_compaction_test.go (T030)"

# Puerto, en paralelo con los tests:
Task: "Declarar SessionMemoryLister en application/ports/session_memory.go (T022)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Fase 1: confirmar capacidades en vivo.
2. Fase 2: base y cierre del defecto de `end_session` (ya aporta valor: la
   sesión deja de cerrarse al compactar).
3. Fase 3: US1.
4. **Parar y validar**: quickstart Q1 y Q2 en los tres clientes.

### Incremental Delivery

1. Setup + Foundational → la recuperación ya no cierra la sesión.
2. + US1 → la memoria de la sesión sobrevive a la compactación (MVP).
3. + US2 → el resumen compactado queda guardado.
4. + US3 → los aprendizajes de los subagentes se guardan solos.
5. + US4 → aviso opcional al agente.
6. Cada incremento pasa `go test ./...` y su escenario del quickstart antes de
   seguir.

---

## Notes

- [P] = archivo distinto y sin dependencias pendientes.
- Ningún test existente se modifica. Si una tarea parece exigirlo, detenerse y
  pedir autorización (constitución, principio III).
- Commit al cerrar cada fase, con la lista de archivos confirmada antes
  (instrucciones del usuario).
- No hacer commit de CLAUDE.md, certificados ni archivos `.env`.
