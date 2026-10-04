---

description: "Tareas de implementación de la feature 035: integridad de los hooks, avisos por conversación y compresión que no destruye"
---

# Tasks: Integridad de los hooks, avisos por conversación y compresión que no destruye

**Input**: Documentos de diseño en `specs/035-hook-integrity-compression/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/hooks.md](contracts/hooks.md), [quickstart.md](quickstart.md)

**Tests**: **Obligatorios y primero.** Lo exigen la constitución III (TDD, no negociable) y la preferencia de la persona (memoria 222: ceremonia estricta, TDD, dependencias, checkboxes y validación por fase). Cada prueba DEBE escribirse, ejecutarse y **fallar** antes de la tarea que la hace pasar.

**Verificación real**: cada historia cierra con su bloque de [quickstart.md](quickstart.md) ejecutado sobre el binario instalado (regla 1 del proyecto: «verde en tests no es funciona»).

**Organization**: tareas agrupadas por historia de usuario, en el orden de entrega del plan: US1 → US2 → US3 → US4 → US5.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[Story]**: historia de usuario de la spec (US1…US5)
- **⚠ AUTORIZACIÓN**: la tarea modifica una prueba existente; requiere la aprobación explícita de la persona (constitución III)

## Path Conventions

Estructura hexagonal existente en la raíz: `domain/`, `application/`, `adapters/`, `infrastructure/`. Las pruebas unitarias van junto al código (`*_test.go`) y las de contrato e integración, en `tests/`. Los puertos nuevos se prueban con *fakes* declarados en los propios tests, el patrón vigente del repositorio.

**Lectura de código durante la implementación**: mientras la US2 no esté hecha, se lee el código con la herramienta Read y nunca con `cat`/`sed` por Bash; el hook de salidas actual destruye esas salidas (memoria 328).

---

## Phase 1: Setup (fixtures reales)

**Purpose**: capturar la evidencia real como fixtures, sin cambiar comportamiento.

- [X] T001 [P] Crear `tests/contract/testdata/tool_output/claude-bash-sed-go.json`: payload PostToolUse de Claude con `tool_name:"Bash"`, `tool_input.command:"sed -n 1,80p adapters/secondary/persistence/compression_stats.go"` y en `tool_response.stdout` el contenido literal de esas 80 líneas (generado con `sed`, sin pasar por el agente)
- [X] T002 [P] Crear `tests/contract/testdata/tool_output/claude-bash-awk-go.json`: Bash con `tool_input.command:"awk 'NR<=200' adapters/primary/cli/cmd_hook.go"` (no excluido) y esas 200 líneas reales de Go con indentación por tabuladores en `stdout`
- [X] T003 [P] Crear `tests/contract/testdata/tool_output/claude-mcp-search30.json`: `tool_name:"mcp__codebase-memory-mcp__search_code"` con un `tool_response` de bloque de texto cuyo JSON tenga `results` de 30 elementos (forma real de `search_code` en modo compact)
- [X] T004 [P] Crear `tests/contract/testdata/tool_output/claude-bash-doctor.json`: Bash con `tool_input.command:"mem doctor"` y la salida real de `mem doctor` en `stdout`
- [X] T005 [P] Crear `tests/contract/testdata/tool_output/claude-grep-listing.json` (Grep en modo content con 400 líneas `ruta:línea:` de archivos distintos, generadas con `grep -rn "func " --include='*.go' .`) y `tests/contract/testdata/tool_output/listing-find.txt` (salida real de `find . -name '*.go'`, ≥ 300 líneas)
- [X] T006 Documentar los fixtures nuevos y su origen en `tests/contract/testdata/tool_output/README.md`
- [X] T007 [P] Crear `tests/contract/testdata/hook_budget/context-snapshot.md` con la salida real de `mem context` de este repositorio (≥ 15 000 caracteres) y `tests/contract/testdata/hook_budget/plan-doc-snapshot.md` con la de `mem plan-context`

---

## Phase 2: Foundational (prerrequisitos que bloquean las historias)

**Purpose**: piezas puras de dominio, puertos, persistencia y utilidades que usan varias historias. **Ninguna historia empieza antes de terminar esta fase.**

### Pruebas primero

- [X] T008 [P] Escribir `domain/hook_budget_test.go`: tabla para `FitHookOutput`. Casos: cabe entero sin cambios; recorta primero la prioridad menor; termina con `HookTrimNotice`; nunca corta una sección `Trimmable:false`; resultado ≤ `HookInlineContextMaxChars` medido en runas (con texto multibyte); orden de salida = prioridad; devuelve recortes con tamaños original y emitido
- [X] T009 [P] Escribir `domain/conversation_test.go`: `IsNewConversation(stored, incoming)` (sin archivo → nueva; mismo id → continúa; id distinto → nueva; id vacío → nueva con id local); `ShouldRotate(lastActivity, now)` con el límite de `StaleSessionSecs` (14 400 s, exacto, menos 1 y más 1); formato `local-<epoch>-<rand6>`
- [X] T010 [P] Escribir `adapters/primary/cli/conversation_test.go`: `writeFileAtomic` con 20 goroutines escribiendo enteros (con `-race`, el archivo siempre parsea); `readConversation` con un archivo corrupto devuelve ausente y lo borra; ida y vuelta de `writeConversation`/`readConversation` con permisos 0600
- [X] T011 [P] Escribir `adapters/primary/cli/hook_guard_test.go`:
  - `acquireHookLock(evento, payload)` adquiere cuando no hay bloqueo;
  - devuelve «duplicado» si existe uno con < 5 s y PID vivo (el PID del propio test);
  - NO es duplicado si el PID del bloqueo no existe (PID inventado muy alto) o si tiene > 5 s;
  - purga los bloqueos con > 60 s;
  - payloads distintos generan claves distintas.
- [X] T012 [P] Escribir `adapters/secondary/persistence/hook_guard_events_test.go`: la migración es idempotente (abrir la BD dos veces); `RecordGuardEvent` suma por (proyecto, agente, tipo, día); `GuardEventsSince(7 días)` agrega y excluye los días viejos; `last_detail` guarda solo el detalle pasado (sin contenido)
- [X] T013 [P] Escribir `adapters/secondary/persistence/session_activity_test.go`: `LastActivity(sessionID)` devuelve el `MAX(created_at)` de las memorias de la sesión, **incluidos los checkpoints**, y cae en `sessions.created_at` si no hay memorias; una sesión inexistente devuelve `false` sin error

### Implementación

- [X] T014 [P] Implementar `domain/hook_budget.go` (`HookSection`, `HookTrim`, `FitHookOutput`, `HookTrimNotice`, constantes de prioridad) hasta pasar T008
- [X] T015 [P] Implementar `domain/conversation.go` y `domain/conversation_policy.go` (`Conversation`, `IsNewConversation`, `ShouldRotate`, `NewLocalConversationID`, `StaleSessionSecs = 14400`, `HookReentryWindowSecs = 5`, `HookLockPurgeSecs = 60`) hasta pasar T009
- [X] T016 [P] Declarar los puertos `application/ports/session_activity.go` (`SessionActivityReader.LastActivity(sessionID string) (string, bool, error)`) y `application/ports/hook_guard.go` (`HookGuardRecorder.Record(agent, kind, detail string)` y `Since(days int) ([]domain.GuardCount, error)`), con los tipos `domain.GuardKind`/`GuardCount` en `domain/hook_budget.go`
- [X] T017 Implementar en `adapters/secondary/persistence/db.go` el `CREATE TABLE IF NOT EXISTS hook_guard_events` de data-model.md, y en `adapters/secondary/persistence/channel_activity.go` `RecordGuardEvent`/`GuardEventsSince`, más la implementación de `HookGuardRecorder` sobre `ChannelActivityRepository`, hasta pasar T012
- [X] T018 Implementar `LastActivity` en `adapters/secondary/persistence/session.go` (consulta con parámetros bind) hasta pasar T013
- [X] T019 [P] Implementar `adapters/primary/cli/conversation.go` (`writeFileAtomic`, `readConversation`, `writeConversation`, `conversationPath`) hasta pasar T010
- [X] T020 [P] Implementar `adapters/primary/cli/hook_guard.go` (`acquireHookLock`, con el PID y la hora escritos en el bloqueo y la comprobación de PID vivo portable; `recordGuard(deps, agent, kind, detail)` fire-and-forget) hasta pasar T011
- [X] T021 Cablear `SessionActivityReader` y `HookGuardRecorder` (resuelto sin tocar el wiring: igual que `SessionSummaryUpdater`, los implementan `SessionRepository` y `ChannelActivityRepository`, y quien los usa hace una aserción de tipo sobre `deps.SessionRepo`/`deps.ChannelActivity`; se reutiliza `writeFileAtomic` de `cmd_mcp_setup.go`) y comprobar `go build ./... && go test ./domain/... ./adapters/secondary/persistence/... ./adapters/primary/cli/ -run 'Conversation|HookGuard|HookBudget' -race`

**Checkpoint**: la base está lista y las historias pueden empezar.

---

## Phase 3: User Story 1 — El agente recibe completas y una sola vez las instrucciones de gomemory en Claude Code (Priority: P1) 🎯 MVP

**Goal**: cada hook corre una sola vez, ninguna salida supera el tope, y los recordatorios no se repiten sin motivo.

**Independent Test**: quickstart §1 y §2. Una conversación nueva en modo plan sobre este repositorio, sin «Output too large» ni duplicados, y el agente llama a las tools de memoria en el primer turno.

### Pruebas primero

- [X] T022 [P] [US1] Escribir `tests/contract/hook_budget_test.go`: con el binario real y un proyecto temporal cuya memoria se siembra desde `tests/contract/testdata/hook_budget/context-snapshot.md` (vía `mem import` o inserción directa):
  - `session-start` → `additionalContext` ≤ 10 000 runas;
  - el primer `user-prompt-submit` con `{"permission_mode":"plan"}` → ≤ 10 000 runas, con el bootstrap `select:` y el protocolo completos y al principio, y el `HookTrimNotice` si recortó;
  - `mem doctor` muestra `budget_trimmed` ≥ 1.
- [X] T023 [P] [US1] Escribir `tests/integration/hook_reentry_integration_test.go`:
  - dos `user-prompt-submit` **concurrentes** con el mismo stdin → exactamente una salida con contenido y la otra `{}`, y `duplicate_dropped` = 1;
  - dos **secuenciales** idénticos → ambos procesados;
  - dos `turn-end` concurrentes con el mismo transcript → un solo checkpoint en la BD.
- [X] T024 [P] [US1] Escribir `tests/contract/scope_dedup_test.go` reutilizando el sandbox de `tests/contract/lifecycle_sandbox_test.go`:
  - con hooks globales de gomemory en `~/.claude/settings.json` (sin `tool-output`) y hooks ajenos `cbm-*`/`herdr` en ambos archivos, `mem install --yes --agents claude --scope project` con `tool_output_compression=true` deja en el proyecto **solo** `tool-output` de gomemory (cada subcomando una vez entre los dos ámbitos);
  - el global queda intacto, también los hooks ajenos;
  - sin hooks globales, el proyecto registra el juego completo (comportamiento vigente);
  - `mem update --version v<actual>` («nada que hacer») también retira del proyecto los duplicados.
- [X] T025 [P] [US1] Escribir en `tests/integration/hook_reentry_integration_test.go` el caso de `session-start` con duplicación: la salida trae un `systemMessage` con «hooks duplicados» y «mem update», y el hash de ambos `settings.json` no cambia tras el hook
- [X] T026 [P] [US1] Escribir `adapters/primary/cli/cmd_doctor_guard_test.go`: con duplicación sembrada, la salida contiene «⚠» y «mem update»; con eventos sembrados en `hook_guard_events`, aparece «protecciones (7 días)» con los tres conteos y «⚠» si `duplicate_dropped` o `budget_trimmed` > 0
- [X] T027 [P] [US1] Escribir `tests/integration/plan_reminder_gating_integration_test.go`:
  - dialecto claude: turno 1 con el recordatorio de plan; turno 2 sin él; turno 3 con `permission_mode:"plan"` por primera vez → presente; turno 4 en plan otra vez → ausente;
  - `--emit=json` (Codex): presente en los turnos 1–3;
  - tras `post-compact`, el primer turno lo vuelve a incluir en claude.
- [X] T028 [US1] ⚠ AUTORIZACIÓN (concedida el 2026-10-04) Reescribir `TestHookOctopusEncendido_SiguePresenteEnTurnosPosteriores` como `TestHookOctopus_ActivarAMitadDeSesionLlegaEnElTurnoSiguiente` en `tests/integration/hook_marker_integration_test.go` (en rojo solo en el turno 3, verificado)
- [X] T029 [P] [US1] Escribir en `tests/integration/plan_reminder_gating_integration_test.go` el caso «plan-doc sin memoria repetida»: tras un `session-start` que entregó el contexto, el primer `user-prompt-submit` con `permission_mode:"plan"` NO contiene la cabecera `# Memoria del Proyecto`; sin `session-start` previo, sí la contiene (dentro del tope)

### Implementación

- [X] T030 [US1] Reescribir `entregaContextoDeArranque` en `adapters/primary/cli/cmd_hook.go`: arma secciones (memoria Trimmable), aplica `domain.FitHookOutput`, emite **siempre** lo ajustado, anota como entregado solo lo emitido y llama a `recordGuard(…, "budget_trimmed", "orig=N,emit=M")` si recortó (T022)
- [X] T031 [US1] Reescribir la rama del primer prompt de `hookUserPromptSubmit` en `adapters/primary/cli/cmd_hook.go` con secciones bootstrap(100) > protocolo(90) > Octopus(80) > plan-doc(70) ajustadas por `FitHookOutput`. El plan-doc se arma sin «Memoria del Proyecto» si `DeliveryLog` registra la entrega de esta conversación (T022, T029)
- [X] T032 [US1] Aplicar `FitHookOutput` en `hookPostCompact`/`printRecoveryAndContext` (`adapters/primary/cli/cmd_hook.go`). `hookSubagentStart` no lo necesita: emite solo texto fijo y acotado (bootstrap, protocolo y aviso de subagente), sin memoria
- [X] T033 [US1] Integrar la guarda en `CmdHook` (`adapters/primary/cli/cmd_hook.go`) para `session-start`, `user-prompt-submit`, `turn-end`, `subagent-start` y `subagent-stop`: leer stdin una vez y guardarlo en una variable de paquete que `readHookStdin` consume si está puesta; `acquireHookLock`; si es duplicado → salida neutra del dialecto + `recordGuard(…, "duplicate_dropped", evento)` + `os.Exit(0)` (T023)
- [X] T033a [US1] Cerrar el hallazgo surgido en T023: dos procesos que abren a la vez un almacén **nuevo** (hooks duplicados en el primer arranque) fallaban con `migrate: database is locked (SQLITE_BUSY)` y código 1. Test `adapters/secondary/persistence/db_concurrent_open_test.go` (rojo y luego verde 10/10); corrección en `adapters/secondary/persistence/db.go`: `busy_timeout` antes que `journal_mode` en el DSN y `migrateWithRetry` (reintento acotado ≈ 5 s; la migración es idempotente)
- [X] T033b [US1] ⚠ AUTORIZACIÓN (concedida el 2026-10-04) Ajustar la detección de truncado de `TestPlanContextSuppression_Max` en `tests/integration/plan_context_suppression_max_test.go`: el hook ahora recorta él mismo y lo declara con «contexto recortado», así que medir la longitud ya no basta
- [X] T034 [US1] Implementar en `adapters/primary/setup/claude_code_setup.go` `GomemoryHookSubs(settingsPath string) map[string]bool` (subcomandos de gomemory registrados) y `DuplicateClaudeHookSubs(home, root string) []string` (intersección global ∩ proyecto), ambos de solo lectura
- [X] T035 [US1] Añadir en `hookSessionStart` (`adapters/primary/cli/cmd_hook.go`) el aviso de duplicación a `avisos` (sale por `systemMessage` mediante `renderSessionStart`), sin escribir ningún archivo del host (T025)
- [X] T036 [US1] Hacer que `InstallClaudeCode` (`adapters/primary/setup/claude_code_setup.go`) escriba en el proyecto solo los subcomandos que no cubren los hooks globales de `~/.claude/settings.json` (parámetro `skip` en `writeClaudeHooks`; el global no se toca nunca desde un proyecto), y añadir `DedupClaudeProjectHooks(home, root string) (removed []string, err error)`, que retira del proyecto los duplicados sin tocar entradas ajenas. Codex y OpenCode no necesitan cambios: tienen una sola ubicación
- [X] T037 [US1] Informar en el resumen de `adapters/primary/cli/cmd_install.go` el paso «hooks globales activos: el proyecto solo añade <subcomandos>», y llamar a `DedupClaudeProjectHooks` desde `adapters/primary/cli/cmd_update.go` también en el camino «Ya estás actualizado» (T024)
- [X] T038 [US1] Añadir en `adapters/primary/cli/cmd_doctor.go` el ⚠ de duplicación (con `DetectDuplicateGomemoryHooks`) y la sección «protecciones (7 días)» (con `HookGuardRecorder.Since(7)`) (T026)
- [X] T039 [US1] Controlar los recordatorios por turno en `adapters/primary/cli/cmd_hook.go`:
  - el recordatorio de plan en el dialecto claude, solo si no existe `.memory/.plan-reminder-emitted` o si `permission_mode=plan` llega y el marcador no lo registra;
  - en json, en cada turno;
  - Octopus, solo en `hookUserPromptSubmit` (canal acumulativo de Claude y Codex), si `octopus_enabled` difiere de `.memory/.octopus-emitted`. OpenCode (`system.transform`, no acumulativo) lo sigue recibiendo en cada turno (FR-008a precisado);
  - `computePlanModeReminder` no se toca (T027, T028).
- [X] T040 [US1] Borrar `.plan-reminder-emitted` y `.octopus-emitted` en `hookPostCompact` (`adapters/primary/cli/cmd_hook.go`) y ejecutar `go test ./... -count=1`: T022–T029 en verde y sin regresiones
- [X] T041 [US1] Validar contra el binario real: `go build -o ~/.local/bin/mem ./infrastructure`; quickstart §1 y §2; abrir una sesión nueva de Claude Code en modo plan y comprobar que no aparece «persisted-output» y que el recordatorio sale una sola vez; anotar la evidencia en `specs/035-hook-integrity-compression/quickstart.md` (sección «Resultados»)

**Checkpoint**: la US1 funciona sola. Es el MVP: corrige la pérdida de tools y las malas decisiones en Claude.

---

## Phase 4: User Story 2 — La compresión de salidas nunca destruye lo que el agente pidió (Priority: P1)

**Goal**: código, resultados y diagnósticos llegan intactos; solo se resume el volumen redundante.

**Independent Test**: quickstart §3. Las salidas de `sed` sobre Go, la búsqueda de 30 resultados y `mem doctor` son idénticas con el hook y sin él.

### Pruebas primero

- [X] T042 [P] [US2] Escribir `adapters/secondary/compression/native/router_go_test.go`: Go sin `package` (fixture T002) → `ContentCode/LangGo`; un bloque indentado con llaves sin lenguaje claro → `ContentCode`; un párrafo de prosa con sangría de lista → `ContentProse`
- [X] T043 [P] [US2] Escribir `adapters/secondary/compression/native/engine_tooloutput_test.go`. Con `ToolOutput:true`:
  - la prosa sale idéntica;
  - el código conserva los espacios iniciales de toda línea conservada;
  - un JSON con un array de 30 objetos sale completo;
  - uno de 60 se recorta con marca.
  - Con `ToolOutput:false`, la salida es byte a byte la de antes en los fixtures existentes (sin regresión).
- [X] T044 [P] [US2] Escribir `adapters/primary/cli/hook_tool_output_safety_test.go`:
  - Bash con `command` que empieza por `sed `, `cat `, `head `, `tail `, `git diff`, `git show` o `mem doctor` → salida vacía y `tool_output_excluded`;
  - `mcp__codebase-memory-mcp__get_code_snippet` → vacía;
  - Grep → no excluido;
  - texto < 2 000 tokens → vacío;
  - OpenCode con `{"tool","output","command"}` aplica las mismas exclusiones.
- [X] T045 [P] [US2] Escribir `tests/contract/hook_tool_output_safety_test.go` con el binario real y los fixtures T001–T004:
  - `claude-bash-sed-go` y `claude-bash-doctor` → salida vacía;
  - `claude-bash-awk-go` → cada línea de la salida que no sea marca existe idéntica en el original, en el mismo orden;
  - `claude-mcp-search30` → los 30 resultados presentes;
  - además, `go test ./tests/contract -run TestHookToolOutput` (T057 existente) sigue en verde.
- [X] T046 [P] [US2] Escribir en `adapters/primary/cli/hook_tool_output_safety_test.go` el caso de la nota de marcas: la primera salida comprimida de la conversación incluye `RetrieveHint`; la segunda, no; tras una conversación nueva, vuelve a incluirla

### Implementación

- [X] T047 [P] [US2] Añadir `ToolOutput bool` a `ports.CompressionOptions` en `application/ports/compressor.go`, y `ToolOutputMinTokens = 2000` y `ToolOutputJSONMinItems = 51` en `domain/compression_policy.go`
- [X] T048 [US2] Añadir en `adapters/secondary/compression/native/router.go` los patrones de Go en `codePatterns` (con desempate antes que TS/Java) y la heurística de indentación de research R5 en `classify` (T042)
- [X] T049 [US2] Implementar en `adapters/secondary/compression/native/engine.go`, con `ToolOutput`: la prosa pasa sin comprimir, sin limpieza estructural y con `th.JSONMinItems = max(th.JSONMinItems, ToolOutputJSONMinItems)`. En todos los modos, `compressBlock` omite la limpieza estructural si hay indentación significativa (T043)
- [X] T050 [US2] Implementar en `adapters/primary/cli/hook_tool_output.go`:
  - lectura de `tool_input.command` (claude) y `command` (opencode);
  - lista de exclusión por prefijo de comando y por `get_code_snippet`;
  - `recordGuard(…, "tool_output_excluded", herramienta)`;
  - `ToolOutput:true` en `Compress`;
  - el umbral pasa a `domain.ToolOutputMinTokens` (T044, T045).
- [X] T051 [US2] Emitir `RetrieveHint` solo si no existe `.memory/.markers-note-emitted` (y crearlo), en `adapters/primary/cli/hook_tool_output.go` (T046)
- [X] T052 [US2] Ejecutar `go test ./... -count=1` (T042–T046 en verde, T057 en verde) y validar quickstart §3 en una sesión real de Claude Code; anotar la evidencia en quickstart.md

**Checkpoint**: US1 y US2 funcionan; Claude Code ya no pierde tools ni recibe salidas falsas.

---

## Phase 5: User Story 3 — Ningún aviso cruza de una conversación a otra (Priority: P2)

**Goal**: cada conversación empieza limpia, el aviso pendiente solo llega a su conversación y las sesiones inactivas rotan.

**Independent Test**: quickstart §4. Aviso de A → conversación B → 3 prompts sin aviso.

### Pruebas primero

- [X] T053 [P] [US3] Escribir `tests/integration/conversation_hook_integration_test.go`:
  - **checkpoint pegado**: `session-start {"session_id":"A"}` → prompt → huella sobre el umbral → `turn-end --emit=json` → `session-start {"session_id":"B"}` → 3 prompts sin «AVISO DE MEMORIA»;
  - el aviso de la conversación en curso se entrega en el primer prompt tras el arranque y una sola vez;
  - `session-start` con el mismo id no borra `.footprint`;
  - un `.pending-agent-notice` en texto plano sin `.conversation` se sigue entregando (compatibilidad con `TestHookUserPromptSubmit_ConsumeElAvisoPendienteUnaSolaVez`).
- [X] T054 [P] [US3] Escribir `adapters/primary/cli/nudge_conversation_test.go`: con la sesión de BD creada hace 3 h y `.conversation.started_at` hace 2 min, `computeSaveNudge` no recuerda; con `started_at` hace 20 min y sin guardados posteriores, sí recuerda; sin `.conversation`, usa `session.CreatedAt` (comportamiento vigente)
- [X] T055 [P] [US3] Escribir en `tests/integration/conversation_hook_integration_test.go` la rotación: con una sesión cuya última memoria tiene > 4 h, `session-start` con un id nuevo deja la sesión vieja con `ended_at` y el resumen «cerrada automáticamente por inactividad», un snapshot en el directorio de backups y una sesión activa nueva; con una memoria de hace 10 min, no rota
- [X] T056 [P] [US3] Escribir `adapters/primary/cli/footprint_atomic_test.go`: 20 goroutines con `footprintAdd` (`-race`) dejan un entero válido; un `.footprint` con `5078217904191461789312457` se lee como 0 y el archivo se borra
- [X] T057 [P] [US3] Escribir `tests/integration/conversation_entrypoints_integration_test.go`: `mem session start --conversation=X` crea `.conversation` con el id X y reinicia el estado por turno; arrancar `mem mcp` sin `.conversation` lo crea con un id `local-…`; con `.conversation` existente, no lo toca

### Implementación

- [X] T058 [US3] Usar `writeFileAtomic` en `footprintAdd`, en los debounces (`computeCompactNudge`, `computePreferenceReinforcement` en `adapters/primary/cli/footprint.go`; `computeSaveNudge` en `adapters/primary/cli/nudge.go`), y hacer que `footprintRead` se autorrepare (T056)
- [X] T059 [US3] Implementar `beginConversation(deps, root, convID string)` en `adapters/primary/cli/conversation.go`:
  - si `domain.IsNewConversation`: borra la lista de estado por turno de data-model.md y escribe `.conversation`;
  - valora la rotación con `SessionActivityReader` y `domain.ShouldRotate` (`End` + `backupSessionSnapshot` + `Start`).
- [X] T060 [US3] Llamar a `beginConversation` desde `hookSessionStart` con el `session_id` del payload, sustituyendo los `os.Remove` sueltos de los marcadores (`adapters/primary/cli/cmd_hook.go`) (T053, T055)
- [X] T061 [US3] Añadir `--conversation=<id>` a `cmdSessionStart` en `adapters/primary/cli/cmd_session.go`, que llama a `beginConversation`; en `CmdMCP` (`adapters/primary/cli/cmd_mcp.go`), sustituir los resets de las líneas 33-34 por `beginConversation` solo si no hay `.conversation` (T057)
- [X] T062 [US3] Cambiar `writePendingAgentNotice`/`consumePendingAgentNotice` en `adapters/primary/cli/footprint.go` al formato `{conv_id, text}`: se descarta y borra si `conv_id` ≠ la conversación actual; el texto plano heredado se entrega solo si no hay `.conversation` (T053)
- [X] T063 [US3] Consumir el aviso pendiente también en la rama del primer prompt de `hookUserPromptSubmit`, como sección `notice` (prioridad 40) (`adapters/primary/cli/cmd_hook.go`) (T053)
- [X] T064 [US3] Hacer que `computeSaveNudge` (`adapters/primary/cli/nudge.go`) mida desde `.conversation.started_at`, con el último guardado como `max(lastSave, started_at)` y fallback a `session.CreatedAt` (T054)
- [X] T065 [US3] Enviar `--conversation=${event.properties.info.id}` en `session.created` en el plugin embebido de OpenCode `infrastructure/plugin/opencode/gomemory.ts`
- [X] T065a [US3] Hacer que la guarda de reentrada solo actúe si el evento está registrado en ambos ámbitos de Claude (`hookRegisteredTwice`), y que el dueño del bloqueo viva al menos 250 ms (`holdHookLock`): bajo carga, un primer proceso rápido terminaba antes de que arrancara el duplicado. Sin duplicación, coste cero (`TestHookReentry_SinDuplicacionNoHayGuarda`)
- [X] T065b [US3] Actualizar la referencia de la fábrica v1 en `specs/032-opencode-v2-plugin-compat/baseline/gomemory.v1.ts` con el cambio deliberado de T065 (el propio contrato `TestOpenCodeV2_FabricaV1IdenticaALaReferencia` lo prescribe)
- [X] T065c [US3] ⚠ AUTORIZACIÓN (concedida el 2026-10-04) Atrasar también el inicio de la conversación en `TestHookUserPromptSubmit_NudgeDeGuardadoVaEnAdditionalContext` (`tests/integration/hook_marker_integration_test.go`): el reloj del recordatorio es la conversación (FR-020)
- [X] T065d [US3] ⚠ AUTORIZACIÓN (concedida el 2026-10-04) Cerrar el test intermitente `TestBuild_RespetaPresupuestoYConservaConflictos`. Causa raíz: `ORDER BY created_at DESC` (resolución de segundo) decidía qué memorias entraban en la ventana, y los títulos de conflictos y sinapsis solo salían de esa ventana. Corrección: desempate determinista por `id`/`rowid` en `memory.go`, `relation.go` y `session.go` (test `memory_order_test.go`), y títulos desde `allMems` en `build_context.go`; `build_context_test.go:123` pasa a esperar el título en lugar de «(memoria previa)». Con `-race`: 20/20
- [X] T066 [US3] Ejecutar `go test ./... -race -count=1` (T053–T057 en verde, `TestHookUserPromptSubmit_ConsumeElAvisoPendienteUnaSolaVez` y el resto de agent_notice en verde) y validar quickstart §4, más la prueba en vivo en Codex; anotar la evidencia en quickstart.md

**Checkpoint**: US1–US3 funcionan; el «checkpoint pegado» desaparece en los tres agentes.

---

## Phase 6: User Story 4 — La compresión ahorra de verdad en listados y la métrica dice la verdad (Priority: P3)

**Goal**: los listados largos ahorran ≥ 40 % conservando errores y extremos, y `pack savings` separa «sin ganancia» de «degradado».

**Independent Test**: quickstart §5.

### Pruebas primero

- [X] T067 [P] [US4] Escribir `adapters/secondary/compression/native/listing_test.go` (fixtures T005):
  - detección (≥ 30 líneas, ≥ 70 %; 29 líneas → no es listado);
  - 15 primeras + 5 últimas literales;
  - las líneas con `error`/`FAIL`/`panic`/`fatal` del medio se conservan;
  - el conteo por archivo (top 5 + «+N otros») va en la marca con la ref;
  - pasa `checkLiteral`;
  - ahorro ≥ 40 %;
  - benchmark en `bench_test.go` < `domain.HookBudget`.
- [X] T068 [P] [US4] Escribir `adapters/secondary/persistence/compression_stats_nogain_test.go`: un resultado con `FallbackReason:"no_gain"` incrementa `no_gains` y NO `fallbacks`; uno con `literal_guard` incrementa `fallbacks`; la migración `addColumnIfMissing` es idempotente
- [X] T069 [P] [US4] Escribir `application/usecases/compression_savings_nogain_test.go` (allí vive `SavingsReport.Format`): la tabla de `mem pack savings` tiene las columnas «sin ganancia» y «degradaciones» con los valores sembrados

### Implementación

- [X] T068a [US4] ⚠ AUTORIZACIÓN (concedida el 2026-10-04) Ajustar `TestCompressionStats` (`adapters/secondary/persistence/compression_stats_test.go`): `no_gain` cuenta en `NoGains`, no en `Fallbacks` (FR-024)
- [X] T070 [P] [US4] Añadir `ContentListing` en `domain/compression.go` y las constantes de listado (`ListingMinLines=30`, `ListingRatio=0.7`, `ListingKeepHead=15`, `ListingKeepTail=5`) en `domain/compression_policy.go`
- [X] T071 [US4] Detectar listados en `classify` de `adapters/secondary/compression/native/router.go`, antes de log y de prosa (T067)
- [X] T072 [US4] Implementar `adapters/secondary/compression/native/listing.go` (`compressListing`) y registrarlo en `compressorFor` de `engine.go`, también admitido en modo `ToolOutput` (T067)
- [X] T073 [US4] Añadir la columna `no_gains` (`addColumnIfMissing` en `adapters/secondary/persistence/db.go`) y su suma en `Record` de `adapters/secondary/persistence/compression_stats.go` (T068)
- [X] T074 [US4] Mostrar las columnas «sin ganancia» y «degradaciones» en `adapters/primary/cli/cmd_pack_compression.go` (T069)
- [X] T075 [US4] Ejecutar `go test ./... -count=1` y validar quickstart §5 (`mem pack compress`, `mem pack retrieve`, `mem pack savings`); anotar la evidencia

---

## Phase 7: User Story 5 — Codex y OpenCode con expectativas claras (Priority: P3)

**Goal**: Codex informa con honestidad dónde comprime y orienta al agente; OpenCode aplica las mismas exclusiones.

**Independent Test**: quickstart §6.

### Pruebas primero

- [X] T076 [P] [US5] Escribir `adapters/primary/cli/protocol_codex_test.go`: el bloque de protocolo contiene la orientación a `search_code`/`get_symbol` de gomemory. Hay un solo bloque para todos los agentes (instrucciones MCP y archivos de usuario), así que la orientación es neutral y nombra a Codex como ejemplo
- [X] T077 [P] [US5] Escribir `adapters/primary/cli/cmd_doctor_compression_codex_test.go`: la línea de Codex dice «solo en origen (Codex no permite reescribir salidas)» y conserva el sufijo «hook antiguo registrado…» cuando corresponde
- [X] T078 [P] [US5] Escribir `tests/contract/opencode_plugin_command_test.go` (lee `infrastructure/plugin/opencode/gomemory.ts`, siguiendo el patrón de `tests/contract/opencode_v2_shape_test.go`): el plugin envía `command` en el payload de `tool-output` y `--conversation` en `session.created`

### Implementación

- [X] T079 [US5] Añadir la línea de orientación de Codex en el renderer del bloque de integración por agente (el que usa `buildIntegrationBlock`) (T076)
- [X] T080 [US5] Cambiar el estado de Codex en `adapters/primary/cli/cmd_doctor_compression.go` (T077)
- [X] T081 [US5] Añadir `command: input.args?.command ?? ""` al payload de `tool-output` en `infrastructure/plugin/opencode/gomemory.ts` (T078, junto a T065)
- [X] T081a [US5] Actualizar otra vez la referencia v1 de la 032 (`specs/032-opencode-v2-plugin-compat/baseline/gomemory.v1.ts`) con el cambio deliberado de T081; el umbral del plugin sube de 1 600 a 8 000 caracteres, alineado con `ToolOutputMinTokens`
- [X] T082 [US5] Ejecutar `go test ./... -count=1` y validar quickstart §6 (doctor, protocolo de Codex y prueba en OpenCode); anotar la evidencia

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T083 Ejecutar `go test ./... -race -count=1` y `scripts/coverage.sh`: todo en verde y cobertura agregada ≥ la de v2.27.1 (memoria 305); registrar las cifras en quickstart.md
- [X] T084 [P] Ejecutar `golangci-lint run` y corregir los hallazgos de los archivos tocados (no está instalado y la regla del entorno prohíbe instalar en el host: se usaron `go vet ./...`, limpio, y `gofmt -l`, dos archivos corregidos)
- [X] T085 [P] Actualizar `docs/MANUAL.md` (secciones de hooks, compresión de salidas, `mem doctor` y estado por conversación) y añadir la entrada `[Unreleased]` en `CHANGELOG.md`
- [X] T086 Ejecutar quickstart.md completo (§0–§7), incluida la observabilidad §7, sobre el binario instalado, en Claude Code, Codex y OpenCode; dejar la evidencia final en quickstart.md
- [X] T087 Cambiar `**Status**` de `spec.md` a `Implemented` y guardar en gomemory las decisiones y bugfixes con su causa raíz (`save_memory`)

---

## Phase 9: Correcciones del ACR acr_715249c3 (2026-10-04)

- [X] T088 Cerrar C-001 (TOCTOU en `acquireHookLock`): un bloqueo reciente sin sello es un dueño a medio escribir y cuenta como duplicado. Test `TestAcquireHookLock_BloqueoRecienteVacioEsDuplicado` (rojo y luego verde) en `adapters/primary/cli/hook_guard_test.go`; corrección en `adapters/primary/cli/hook_guard.go`
- [X] T089 Cerrar C-002 (bloqueo que sobrevive al proceso y reciclaje de PID): el dueño libera su bloqueo tras la espera de 250 ms y nunca toca uno ajeno (`releaseHookLock`). Test `TestReleaseHookLock_ElDuenoLiberaSuBloqueo`; llamada en `dropDuplicateHook` (`adapters/primary/cli/cmd_hook.go`)
- [X] T090 Cerrar C-004 (id vacío = conversación nueva): sin id del host se continúa la conversación registrada y solo se abre una si no hay ninguna. `domain/conversation.go` + `domain/conversation_test.go` (casos de esta feature); spec (casos borde) y data-model actualizados
- [X] T091 Cerrar S-001 (dedup por entrada completa): `withoutDuplicateCommands` retira solo los comandos duplicados de una entrada agrupada. Test `adapters/primary/setup/claude_dedup_grouped_test.go`
- [X] T092 S-002 (referencia 032): se conserva la actualización, porque el contrato de la 032 la prescribe cuando la fábrica cambia a conciencia; queda trazada en T065b/T081a. S-003: informativo, sin acción

## Dependencies & Execution Order

### Dependencias entre fases

- **Setup (F1)**: sin dependencias.
- **Foundational (F2)**: depende de F1 (T007 para T022). Bloquea todas las historias.
- **US1 (F3)**: depende de F2. Es el MVP.
- **US2 (F4)**: depende de F2. Es independiente de US1 en código, salvo `recordGuard` (F2). T051 usa `.conversation` solo para el reinicio, así que se completa del todo tras US3 (T059).
- **US3 (F5)**: depende de F2. T063 toca la rama del primer prompt que reescribe T031, por eso se hace después de US1.
- **US4 (F6)**: depende de US2 (T047–T049: el modo `ToolOutput` y el router).
- **US5 (F7)**: T081 depende de T065 (mismo archivo: `infrastructure/plugin/opencode/gomemory.ts`).
- **Polish (F8)**: depende de todas.

### Dentro de cada historia

Pruebas (rojo) → dominio/puertos → adaptadores → integración en los hooks → `go test` → validación sobre el binario real. Las tareas que comparten archivo (`cmd_hook.go`: T030–T033, T035, T039, T040, T060, T063) son secuenciales.

## Parallel Execution Examples

- **F1**: T001–T005 y T007 en paralelo.
- **F2**: pruebas T008–T013 en paralelo; luego T014, T015, T016, T019 y T020 en paralelo; T017→T018→T021 en secuencia.
- **US1**: pruebas T022–T027 y T029 en paralelo; implementación T034 y T036 en paralelo con T030–T032 (archivos distintos).
- **US2**: T042–T046 en paralelo; T047 en paralelo con las pruebas.
- **US3**: T053–T057 en paralelo.
- **US4**: T067–T069 en paralelo; T070 en paralelo con las pruebas.
- **US5**: T076–T078 en paralelo.

Nota: la persona prefiere ejecución secuencial y auditable (CLAUDE.md, «Ejecución secuencial»). Las marcas [P] indican independencia, no una obligación de paralelizar.

## Implementation Strategy

1. **MVP = F1 + F2 + US1**: corrige la pérdida de tools y las decisiones equivocadas en Claude (duplicados, truncado y ruido). Se valida en vivo y se propone un commit (con la lista de archivos para tu aprobación).
2. **+ US2**: elimina las salidas destructivas. Se valida y se propone un commit.
3. **+ US3**: desaparece el «checkpoint pegado». Se valida en Codex y se propone un commit.
4. **+ US4 y US5**: ahorro real y expectativas claras. Se valida y se propone un commit.
5. **Polish**: cobertura, lint, documentación y release (versión a decidir con la persona).

Cada incremento se detiene en su checkpoint para la validación y la aprobación de la persona antes de seguir.
