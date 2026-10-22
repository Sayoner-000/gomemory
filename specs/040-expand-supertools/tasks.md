# Tasks: mem expand — supertools inspiradas en ECC

**Input**: Design documents from `specs/040-expand-supertools/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: SÍ. La constitución (§10) exige pruebas y el proyecto trabaja con TDD: cada tarea de prueba se escribe primero y se comprueba que falla (rojo) antes de implementar.

**Organization**: tareas agrupadas por historia de usuario de `spec.md`. El cableado en los harnesses va en US1, porque su prueba independiente («instalación limpia bloquea `rm -rf`») lo necesita. US2 cubre migración, idempotencia, desinstalación, CLI y ayuda. Revisión 2 (2026-10-21): incorpora las correcciones del 1.er `/speckit-analyze` (D1, D2, D3, D4, F1–F9, C1–C3, U1, B1): puertos de estado, plegado idempotente, sonda real de Codex previa, registro perezoso de activaciones, interruptor por supertool en el hook y reloj inyectado. Revisión 3 (2026-10-21): incorpora el 2.º `/speckit-analyze` (K1–K3, H1, H2, U1, C1–C3, I1–I4, A1, U2): `ctx` en toda E/S nueva, nombres de puertos según §3, pruebas de pánico y timeout sin disparadores en el binario, `pre-tool` nunca emite `allow`, plegado idempotente por observación, `rules --check` y latencia de los 5 eventos. Revisión 4 (2026-10-21): incorpora la 3.ª pasada (K1 sanitize de los tres almacenes, I1 eventos de Codex verificados por la tabla existente, I2 cláusula de excepciones de paridad, U1 `source:"compact"` en OpenCode, A1 detector `hook-outside-home` sin falsos positivos del PATH, C1 `skipped` visible, I3, D1, F1, F3, U2). A partir de aquí, las referencias a hallazgos llevan el número de pasada («2.º analyze H1», «3.ª pasada, U1») porque los IDs se reutilizan entre pasadas (F4).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizable (archivos distintos, sin dependencias pendientes)
- **[Story]**: US1–US6 según spec.md

## Path Conventions

Monolito Go hexagonal en la raíz: `domain/`, `application/`, `adapters/`, `infrastructure/`, `tests/`. Unitarias junto al código (`*_test.go`). Composition root: `infrastructure/container.go`. Las referencias R*, I*, G* y S* remiten a `research.md` y `contracts/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: guardias de fuente única y carpeta de evidencias.

- [X] T001 Ampliar `TestSkillSourcesInSync` en `tests/contract/skill_sources_sync_test.go` para que recorra también `infrastructure/templates/expand/<skill>/SKILL.md` y exija la copia publicada `skills/<skill>/SKILL.md` (sin salto silencioso) para las 5 skills de expand (R11). Resultado esperado: rojo, porque aún no existen.
- [X] T002 [P] Crear la prueba `tests/contract/hook_subcommands_registry_test.go` (R2), que comprueba:
  - Cada subcomando de `claudeHookEvents` (via `setup.ClaudeHookSubs()`, exportado en T007), de `setup.CodexGomemoryHooks()` y de las llamadas `["hook", "<sub>"` extraídas con regex de `infrastructure/plugin/opencode/gomemory.ts` figura en `domain.GomemoryHookSubcommands`.
  - `setup.IsGomemoryHookEntry` reconoce `mem hook <sub>` para cada entrada del registro, incluida `expand pre-tool`.
  - Un comando de terceros no se reconoce.

  Resultado esperado: rojo.
- [X] T003 [P] Crear `specs/040-expand-supertools/evidence/README.md` con el formato de registro por harness (versión del harness, binario `mem version`, orden, salida literal, veredicto) y la lista de verificaciones reales del quickstart §2–§11.
- [X] T004 Hacer la sonda real de Codex (research R3; decisión de la persona tras analyze F1/F2) y registrarla en `specs/040-expand-supertools/evidence/codex-probe.md`.
  - **Preparación**: `CODEX_HOME` temporal en el scratchpad, con una copia de `config.toml` de la persona sin `[hooks]` previos. La credencial se referencia por enlace, nunca se copia a evidencia.
  - **Hooks de prueba**:
    - `PreToolUse` con matcher `Bash|apply_patch|.*`: un script que añade el stdin a un log y, si el comando contiene `PROBE_DENY`, responde `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"probe"}}`.
    - `PostToolUse` sin matcher: un script que solo registra.
  - **Sesión interactiva**: pedir (a) `echo PROBE_DENY`, (b) `ls`, (c) editar un archivo del directorio temporal y (d) delegar una subtarea si Codex lo admite.
  - **Registro**: versión de Codex, `tool_name` observados por acción, si (a) se bloqueó y si (c) disparó `PreToolUse`.
  - **Resultado**: fija el matcher definitivo de T029. Lo que no funcione se declara como excepción con evidencia.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: registro de subcomandos, ajustes, router puro, renderizado y punto de entrada `mem hook expand`. Ninguna historia puede empezar sin esta fase.

- [X] T005 [P] Crear `domain/hook_subcommands.go` con `GomemoryHookSubcommands` (lista ordenada: `session-start`, `session-end`, `pre-compact`, `post-compact`, `compaction-context`, `compact-summary`, `user-prompt-submit`, `turn-end`, `subagent-start`, `subagent-stop`, `plan-approved`, `plan-guard`, `plan-entered`, `tool-output`, `prompt`, `socratica-decide`, `nudge`, `agent-notice`, `octopus-delegation-policy`, `channel-fired`, `channel-error`, `expand`), más `IsGomemoryHookSubcommand(sub string) bool`. Añadir una prueba de tabla en `domain/hook_subcommands_test.go`.
- [X] T006 Refactorizar `hookCommandIsGomemory` en `adapters/primary/setup/claude_code_setup.go:498` para que extraiga el subcomando con `gomemoryHookSubRe` y consulte `domain.IsGomemoryHookSubcommand`, conservando el reconocimiento por ruta `plugins/gomemory`. Ampliar `TestHookCommandIsGomemoryRecognizesAllSubcommands` en `adapters/primary/setup/claude_code_hooks_test.go` para que recorra `domain.GomemoryHookSubcommands` (dep: T005).
- [X] T007 Exportar `ClaudeHookSubs() []string` (subcomandos de `claudeHookEvents` con `hookSubName`) en `adapters/primary/setup/claude_code_setup.go`, para que T002 pase en lo que toca a los registros existentes (dep: T006).
- [X] T008 [P] Añadir `ExpandDisabled bool`, `ExpandOff []string` y `ExpandCompactThreshold int` (json `expand_disabled`, `expand_off`, `expand_compact_threshold`, todos `omitempty`) a `ports.SettingsData` en `application/ports/settings_repository.go`. Normalizar el umbral (fuera de 10–1000 → 50) en `adapters/secondary/persistence/settings.go` con un `applyExpandDefaults`. Probarlo en `adapters/secondary/persistence/settings_test.go`: lectura de JSON antiguo sin cambios y valores fuera de rango.
- [X] T009 [P] Escribir pruebas de tabla en `domain/expand_test.go` (rojo) para:
  - `ParseExpandEvent` (5 válidos y 1 inválido).
  - `ClassifyTool`: nombres de Claude (`Bash`, `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `Task`, `Agent`, `Read`), de Codex verificados en `evidence/codex-probe.md` (`Bash` → shell, `apply_patch` → file, `collaborationspawn_agent` → delegate, `collaborationwait_agent` → other) y de OpenCode (`bash`, `write`, `edit`, `patch`, `task`, `read`).
  - `IsCommitCommand`: `git commit -m x`, `ls && git commit`, `gh pr create`, `glab mr create`, `echo "git commit"` → false, `git commit-tree` → false.
  - `MatchesDependencyIntent`: ≥ 10 positivos y ≥ 10 negativos, en español e inglés (R9).
- [X] T010 Implementar en `domain/expand.go` los tipos `SupertoolName`, `Supertools`, `ExpandEvent`, `ToolKind`, `ExpandSignals`, `ExpandSwitches`, `ActionKind` y `SupertoolAction`, y las funciones `ParseExpandEvent`, `ClassifyTool`, `IsCommitCommand` (con el lexer de shell compartido de T011) y `MatchesDependencyIntent`, hasta poner T009 en verde (data-model §1).
- [X] T011 [P] Crear `domain/shell_lexer.go` con `SplitShellCommands(cmd string) [][]string`, que tokeniza con comillas simples y dobles y escapes, y separa por `;`, `&&`, `||`, `|` y salto de línea. Probarlo en `domain/shell_lexer_test.go` con comillas anidadas, `echo "rm -rf"` y cadenas vacías.
- [X] T012 Escribir `domain/expand_router_test.go` con la tabla de verdad completa de `contracts/router.md` (filas 1–18) y las invariantes I1 (dos llamadas → resultado igual), I2 e I4. Resultado esperado: rojo (dep: T010).
- [X] T013 Implementar `RouteSupertools(sig ExpandSignals, sw ExpandSwitches) []SupertoolAction` en `domain/expand.go` según la tabla R9 y el orden I4, hasta poner T012 en verde.
- [X] T014 [P] Escribir `adapters/primary/cli/hook_expand_dialect_test.go` (rojo) con `renderExpandOutput` para cada fila de la tabla «Salida por dialecto» de `contracts/hook-expand.md` × los 4 dialectos × los eventos `pre-tool`, `prompt`, `stop` y `session-start`. Comprobar que el contexto lleva el prefijo `[gomemory expand]`, el tope de 600 caracteres y que en allow no hay contexto (`{}` en claude/json, vacío en text/neutral). G6 (2.º analyze H1): en `pre-tool` y en todos los dialectos, la salida nunca contiene la cadena `"allow"`; «allow + contexto» lleva solo `additionalContext`, sin `permissionDecision`.
- [X] T015 Implementar en `adapters/primary/cli/hook_dialect.go` el tipo `domain.ExpandOutcome` (en `domain/expand.go`) y `renderExpandOutput(d hookDialect, event domain.ExpandEvent, out domain.ExpandOutcome) hookRenderedOutput`, generalizando `promptHookJSON` con `hookEventName` como parámetro y sin modificar `renderGuardDecision` (R4). Debe poner T014 en verde y dejar intactas las pruebas existentes de `hook_dialect_test.go`.
- [X] T016 [P] Crear `adapters/primary/cli/hook_expand_state.go` con el estado en archivos de R6:
  - `expandSessionKey(payload)`: `sha256(session_id)[:8]` o `default`.
  - `expandEpisodeEmitted`, `expandEpisodeMark` y `expandEpisodeReset`: un archivo ilegible equivale a «ya emitido».
  - `expandToolCountIncr` y `expandToolCountReset`.
  - `purgeExpandMarkers(root, now)` (> 7 días, con `now` inyectado; analyze D3).

  Todo con permisos `0600` y best-effort. Probarlo en `hook_expand_state_test.go` con reloj fijo: dos sesiones no comparten episodio, el marcador corrupto silencia y la purga borra solo los antiguos.
- [X] T017 [P] Crear `application/ports/expand_state.go` con `ObservationQueueRepository` (`Append(ctx, obs)`, `ReadBatch(ctx, max)`, `Remove(ctx, ids)`, `Sanitize(ctx, redact)`) y `ShieldStateRepository` (`LastHash(ctx)`, `Save(ctx, report)`, `Last(ctx)`), todos con `ctx` primero (2.º analyze K2; nombres según §3, K3); `Observation` lleva `ID` UUIDv7 (H2), el tipo `domain.Observation` en `domain/instinct.go` (solo el struct) y el adaptador `adapters/secondary/expandstate/{queue,shield}.go` (archivos `.memory/expand-observations.ndjson`, `.memory/.expand-shield-hash` y `.memory/expand-shield-last.json`, permisos 0600, tope de 256 KiB con rotación que descarta la mitad más antigua, línea ilegible ignorada). Construirlos en `infrastructure/container.go`. Probarlos en `adapters/secondary/expandstate/*_test.go`: tope, rotación, cola corrupta, permisos e ida y vuelta del informe (research R16; analyze D1).
- [X] T018 Crear el caso de uso `application/usecases/expand_hook.go` con `ExpandHook.Run(ctx context.Context, sig, actions, deps ExpandHookDeps) domain.ExpandOutcome` (recibe las acciones ya decididas por el router, que el adaptador invoca una sola vez y que gestiona los resets de su estado; I5) (cada evaluador recibe `ctx` y debe respetar `ctx.Done()`). Ejecuta en orden las acciones del router delegando en interfaces inyectadas (`RulesEvaluator`, `ShieldRunner`, `InstinctObserver`, `SuggestionText`), cada una opcional (nil = no-op). Así cada historia enchufa la suya sin tocar esta. Probarlo en `expand_hook_test.go` con fakes: acciones nil, varias sugerencias y un block que prevalece sobre un warn.
- [X] T019 Crear `adapters/primary/cli/hook_expand.go` con dos piezas (2.º analyze K1 y K2, R18):
  - `runHookExpand(ctx context.Context, d hookExpandDeps, event, emit string, stdin []byte) hookRenderedOutput`: función sin E/S de proceso, con dependencias inyectadas (ajustes, router, `ExpandHook`, estado de episodio, registrador perezoso y reloj).
  - `hookExpand(deps *Deps, args []string)`: crea `ctx, cancel := context.WithTimeout(context.Background(), domain.HookBudget)`, lee stdin, llama a `runHookExpand` dentro de `withBudget` (red de seguridad) y escribe y sale.

  Pasos de `runHookExpand`:
  1. Parsear el evento y `--emit`.
  2. Leer stdin con tope de 256 KiB y parsear el payload (normalizar el formato OpenCode `{tool, args, prompt, session_id}`).
  3. Leer los ajustes; con `ExpandDisabled`, salida de allow. Construir `ExpandSwitches{Off: set(ExpandOff)}` (analyze C2).
  4. Construir `ExpandSignals` (contador y huella desde archivos) y llamar a `RouteSupertools` una sola vez.
  5. Llamar a `ExpandHook.Run(ctx, …)`. Si `ctx` vence (o `withBudget` corta), registrar `expand.timeout` y salida de allow. Ante un pánico o un error, registrar `expand.error` (analyze D4).
  6. Filtrar las sugerencias por episodio y marcarlas.
  7. `recordGuard` solo si algo se emite, abriendo la base de forma **perezosa** también en la ruta ligera (analyze F3, R5).
  8. `renderExpandOutput`, escritura y `os.Exit` con el código renderizado.

  Añadir `case "expand"` en `CmdHook` (`adapters/primary/cli/cmd_hook.go`). No abrir SQLite en `pre-tool` ni en `prompt`: si `Deps` abre la base al construirse, añadir una ruta ligera en `main.go` o en el constructor de `Deps` para `hook expand` (R5) (deps: T008, T013, T015, T016, T017, T018; 3.ª pasada, F1).
- [X] T020 Escribir `tests/integration/hook_expand_test.go` (binario construido en `TestMain`, igual que las integraciones existentes de hooks) que cubra G1–G4:
  - Evento inválido → exit 0 sin salida.
  - Payload no JSON → allow.
  - `post-tool` sin stdout ni stderr.
  - Sugerencia emitida una vez y silencio en la segunda llamada con el mismo `session_id`, pero emitida de nuevo con otro `session_id`.
  - `ExpandDisabled` → allow en todos los eventos.
  - `expand_off: ["flow"]` → sin sugerencias.
  - Una sugerencia emitida queda registrada como `expand.suggest` aunque el camino no haya abierto la base antes (apertura perezosa).

  Además, en `adapters/primary/cli/hook_expand_test.go` (unitaria sobre `runHookExpand`, **sin** variables de entorno ni etiquetas en el binario; 2.º analyze K1 y C1):
  - Una dependencia inyectada que entra en pánico → allow y `expand.error` registrado.
  - Un evaluador inyectado que bloquea más que el plazo → allow, sin contexto y `expand.timeout` registrado; el evaluador recibe `ctx.Done()`.
  - Registrador o base no disponibles → allow y la observación sigue en la cola falsa (edge case de la spec; 2.º analyze U2).

  (dep: T019)

**Checkpoint**: `mem hook expand <event>` existe, decide con el router y responde allow o sugerencias, aunque todavía no hay reglas, shield ni instincts enchufados.

---

## Phase 3: User Story 1 - Bloqueo automático de comandos destructivos sin configurar nada (Priority: P1) 🎯 MVP

**Goal**: rules (de serie y propias) evaluadas en pre-tool, prompt y stop, con el módulo cableado en los 3 harnesses desde `mem install`.

**Independent Test**: instalación limpia → `rm -rf /tmp/x` denegado con motivo en Claude Code, Codex y OpenCode; una regla propia `warn` sobre un prompt añade contexto sin bloquear (quickstart §2–§4).

### Tests for User Story 1 ⚠️

- [X] T021 [P] [US1] Escribir `domain/rules_builtin_test.go` (rojo) con la tabla completa allow/warn/block de `contracts/rules-format.md` («Regla de serie destructive-shell»), incluidas `sudo rm -rf /`, `ls && rm -rf build`, `echo "rm -rf /"` y `git push --force-with-lease` → warn.
- [X] T022 [P] [US1] Escribir `domain/rules_test.go` (rojo) para `ParseRule` y `EvaluateRules`:
  - Frontmatter válido; claves desconocidas ignoradas.
  - `event` inválido, `pattern` vacío, `pattern` > 512 caracteres y regex que no compila → `RuleLoadError`.
  - `match: contains` y `match: regex`.
  - Eventos `bash`, `file` (ruta y contenido), `prompt`, `stop` y `any`.
  - Precedencia `block > warn > allow`; empate → primera por nombre.
  - `overrides: destructive-shell` con `enabled: false` desactiva la regla de serie.
  - Mensaje saneado de controles de terminal y recortado a 2 KiB.
- [X] T023 [P] [US1] Escribir `adapters/secondary/rulesfs/loader_test.go` (rojo) con:
  - Directorio ausente → solo la regla de serie.
  - 65 archivos → se cargan 64.
  - Archivo de 17 KiB → error de carga.
  - Enlace simbólico ignorado.
  - Archivo con ESC/CSI saneado.
  - La caché se reutiliza si no cambian mtime ni tamaño, se recompila al tocar un archivo y una caché corrupta se recompila sin error.

### Implementation for User Story 1

- [X] T024 [US1] Implementar `domain/rules.go` (`Rule`, `RuleEvent`, `MatchKind`, `RuleAction`, `RuleVerdict`, `RuleLoadError`, `ParseRule(name string, raw []byte) (Rule, error)` con el parser de frontmatter existente y `EvaluateRules(target RuleEvent, input RuleInput, rules []Rule) RuleVerdict`) y `BuiltinRules` con el evaluador `destructive-shell` sobre `SplitShellCommands`, más `ParseApplyPatch(patch string) (paths []string, body string)`, que extrae rutas de las cabeceras `*** (Add|Update|Delete) File:` del parche de Codex. Probar `ParseApplyPatch` en `domain/rules_test.go` con los parches reales de `evidence/codex-probe.md` (rutas anonimizadas). Todo hasta poner T021 y T022 en verde (R7, R8).
- [X] T025 [US1] Implementar el adaptador `adapters/secondary/rulesfs/loader.go` (`Loader.Load(ctx, root) ([]domain.Rule, []domain.RuleLoadError)`, que implementa `ports.RuleSourcePort`) con los topes de `contracts/rules-format.md` y la caché `.memory/.expand-rules-cache.json` (clave nombre+tamaño+mtime), hasta poner T023 en verde. Definir el puerto `ports.RuleSourcePort` en `application/ports/rule_source.go`.
- [X] T026 [US1] Enchufar `RulesEvaluator` en `ExpandHook` (`application/usecases/expand_hook.go`) para que construya `RuleInput` desde las señales (command, file_path+content/new_string truncado a 64 KiB, prompt, last_assistant_message). Construir `rulesfs.Loader` en `infrastructure/container.go` y exponerlo en `Deps`. Formatear el mensaje de bloqueo según `contracts/rules-format.md`.
- [X] T027 [US1] Ampliar `tests/integration/hook_expand_test.go` con los casos de extremo a extremo de rules: `rm -rf /tmp/x` en los dialectos claude, json, text (`BLOCK:`) y neutral (exit 1); `ls` → `{}`; regla propia `warn` en prompt → `additionalContext`; regla propia `block` en prompt → `{"decision":"block"}`; regla `stop` → block en stop; con `expand_off: ["rules"]`, `rm -rf /tmp/x && git commit -m x` se permite y la sugerencia de `verify-loop` sigue saliendo (analyze C2); un bloqueo deja una fila `expand.block`.
- [X] T028 [US1] Cablear Claude Code: añadir a `buildClaudeHookEvents` en `adapters/primary/setup/claude_code_setup.go` las entradas de R3 (`SessionStart` con matcher `startup|resume|clear|compact` → `expand session-start`; `UserPromptSubmit` → `expand prompt`; `PreToolUse` con matcher `Bash|Write|Edit|MultiEdit|NotebookEdit|Task|Agent` → `expand pre-tool`; `PostToolUse` con matcher `Bash` → `expand post-tool`; `Stop` → `expand stop`), en el conjunto fijo y no en `claudeHookEventsFor`. Comprobar que `plan-guard` y `ExitPlanMode` siguen presentes. Pruebas en `adapters/primary/setup/claude_code_hooks_test.go`: presencia, idempotencia (dos escrituras = una entrada) y conservación de los hooks de terceros.
- [X] T029 [US1] Cablear Codex: añadir a `codexGomemoryHooks` en `adapters/primary/setup/codex_setup.go`, AL FINAL (sin desplazar los índices que usan las pruebas), las entradas `SessionStart`/`expand session-start`, `UserPromptSubmit`/`expand prompt` (Emit json), `PreToolUse` con Matcher `Bash|apply_patch|collaborationspawn_agent` (fijado por T004)/`expand pre-tool` (Emit json; en `pre-tool` el dialecto json emite el sobre `hookSpecificOutput.permissionDecision`, el mismo que Claude, que es el que Codex respetó en la sonda), `PostToolUse` con Matcher `Bash`/`expand post-tool` (Emit json) y `Stop`/`expand stop` (Emit json). Las entradas `PreToolUse` y `PostToolUse` citan en un comentario `evidence/codex-probe.md` (fecha y versión de Codex). `SessionStart`, `UserPromptSubmit` y `Stop` ya los verifica en sesión interactiva la tabla `codexGomemoryHooks` existente con los hooks actuales de gomemory, así que se instalan; su capacidad de **bloquear** (`{"decision":"block"}` en prompt y stop) se verifica en T032, y si no se cumple se declara como excepción de paridad con evidencia (spec, Assumptions; 3.ª pasada, I1). `ClassifyTool` ya incluye los nombres de Codex desde T009 y T010; aquí solo se comprueba (3.ª pasada, D1) (dep: T004). Pruebas en `adapters/primary/setup/codex_gomemory_hooks_test.go`: `codexMissingHooks` las detecta ausentes y presentes.
- [X] T030 [US1] Cablear OpenCode en `infrastructure/plugin/opencode/gomemory.ts`:
  - Nuevo `"tool.execute.before"`, que envía `{tool, args, session_id}` a `mem hook expand pre-tool --emit=text` y, si la salida empieza por `BLOCK:`, lanza `new Error(motivo)`. Si devuelve otro texto, lo guarda para el siguiente `chat.message`.
  - En `"tool.execute.after"` (solo bash), `mem hook expand post-tool --emit=text`, fire-and-forget.
  - En `chat.message`, `mem hook expand prompt --emit=text`: con `BLOCK:` detiene el envío igual que socratica-decide; si devuelve contexto, lo antepone.
  - En `session.created`, `mem hook expand session-start` con `{source:"startup", session_id}`; en `session.compacted`, con `{source:"compact", session_id}`, para que se reinicien el contador y el episodio (3.ª pasada, U1). Caso en `gomemory.test.mjs`: `session.compacted` envía `source:"compact"`.
  - En `session.idle`, `mem hook expand stop`. OpenCode no tiene forma de impedir el fin de turno: un `BLOCK:` en stop se muestra como aviso en el siguiente `chat.message`. Esto se declara como excepción de paridad en la matriz de `docs/expand.md` (2.º analyze I1). El aviso de `pre-tool` en OpenCode llega en el siguiente mensaje, no en la misma herramienta; también se declara.

  Ampliar `infrastructure/plugin/opencode/gomemory.test.mjs` con un `mem` simulado que devuelve `BLOCK:`, para comprobar el `throw` y que un fallo del proceso no bloquea.
- [X] T031 [US1] Poner T002 en verde: comprobar que el plugin TS, la tabla Codex y `claudeHookEvents` solo usan subcomandos del registro, y que `mem uninstall` reconoce `hook expand` en los 3 harnesses (`tests/contract/hook_subcommands_registry_test.go`).
- [X] T032 [US1] Verificación real con un binario recompilado y una instalación limpia en un directorio temporal (quickstart §2 y §3) en Claude Code, en Codex (sesión interactiva) y en OpenCode. En los 3 harnesses se verifica también quickstart §4 (2.º analyze C3): una regla propia `warn` en pre-tool (contexto visible sin bloquear ni aprobar en silencio, comprobando que el harness sigue pidiendo permiso si correspondía), una `block` en prompt y una `block` en stop. Registrar la salida literal y el veredicto en `specs/040-expand-supertools/evidence/us1-destructive.md`. Lo que un harness no respete (por ejemplo, `{"decision":"block"}` en el UserPromptSubmit o el Stop de Codex, que la sonda T004 no cubrió) se declara con evidencia en la matriz de `docs/expand.md`.

**Checkpoint**: MVP de protección. `rm -rf` se bloquea sin configurar nada y las reglas propias funcionan.

---

## Phase 4: User Story 2 - Módulo siempre activo, visible y con interruptor de emergencia (Priority: P1)

**Goal**: actualización, idempotencia y desinstalación limpias; `mem expand status|off|on [<supertool>]`; ayuda.

**Independent Test**: `mem expand status` muestra ✓ en los 3 harnesses; `off` deja de bloquear; `on` restablece; `mem uninstall` deja 0 entradas `hook expand` (quickstart §6–§7).

### Tests for User Story 2 ⚠️

- [X] T033 [P] [US2] Escribir `adapters/primary/cli/cmd_expand_test.go` (rojo) contra `runExpand(deps, args, expandIO)` y `contracts/expand-cli.md`:
  - `status` en texto y `--json` (cableado leído de archivos fixture en un HOME temporal).
  - `off`, `on`, `off rules`, `on rules`, `off bogus` → 2.
  - `mem expand xyz` → 2.
  - `rules --json` lista la regla de serie y los errores de carga.
  - `rules --check` (2.º analyze U1): un diff con `rm -rf` en una línea añadida → 2; un diff inocuo → 0; con una regla `warn` sobre la ruta → 1; stdin ilegible → 3; `--event bash` evalúa el texto como comando.
  - Persistencia en `settings.json` y nada escrito fuera de `.memory/`.
  - Con un `HookGuardRecorder` falso que contiene un `expand.block` de hace 2 días y otro de hace 9 (reloj fijo), `status` muestra solo el primero (analyze F3 y D3).
- [X] T034 [P] [US2] Escribir `adapters/primary/cli/cmd_install_expand_test.go` (rojo):
  - Sobre un `settings.json` y un `config.toml` de una instalación v2.35.0 sin expand, `mem install` (vía la función de setup) añade `hook expand` en los 5 eventos.
  - Dos ejecuciones no duplican.
  - `uninstall` deja 0 entradas `hook expand` y conserva las de terceros.
  - El bloque de instrucciones v9 se migra a v10.

### Implementation for User Story 2

- [X] T035 [US2] Subir `domain.ProtocolVersionMarker` a `<!-- gomemory-protocol-v10 -->` en `domain/protocol.go` y añadir al texto del protocolo instalado (plantilla del bloque de protocolo usada por `cmd_install.go`) un párrafo «Supertools expand» de como mucho 6 líneas. Debe decir que las rules están activas (`.gomemory/rules`), nombrar las 5 skills de flujo y dar el interruptor `mem expand off`. Actualizar las pruebas que fijan v9 (`grep -rn 'protocol-v9'`).
- [X] T036 [US2] Implementar `adapters/primary/cli/cmd_expand.go` con `CmdExpand` y `runExpand(deps, args, expandIO{in,out,err,tty}) int`:
  - `status`: cableado por harness (lee `.claude/settings.json` del proyecto y global con `setup.GomemoryHookSubs`, `setup.CodexHookInstalado` para cada hook expand y la presencia de `"expand"` en el plugin de OpenCode instalado), estado de cada supertool, reglas y errores, activaciones de 7 días por `HookGuardRecorder.GuardSince(7)` filtradas por `expand.*` (el reloj llega por `expandIO.now`) y resumen de `.memory/expand-shield-last.json` si existe.
  - `off|on [<supertool>]`.
  - `rules` y `rules --check [--event …] [--json]` (lee stdin con tope de 1 MiB, usa el mismo `RuleSourcePort` y `domain.EvaluateRules` que el hook; para un diff, rutas de `+++`/`---` y solo las líneas añadidas como contenido; códigos de `contracts/expand-cli.md`; 2.º analyze U1).

  Registrar el comando en el despachador de `cli.go`. Debe poner T033 en verde.
- [X] T037 [US2] Añadir `expand` a `helpIndex` (`adapters/primary/cli/cli.go:51`) y su ayuda detallada (todas las subacciones de `contracts/expand-cli.md`), y comprobar que la prueba de índice bidireccional existente (memoria 572) sigue en verde.
- [X] T038 [US2] Hacer que el uninstall (Claude, Codex y la plantilla del plugin de OpenCode) y `DedupClaudeProjectHooks` cubran `hook expand` gracias a T006, y ajustar lo que falte hasta poner T034 en verde. Archivos: `adapters/primary/cli/cmd_uninstall.go` y `adapters/primary/setup/codex_setup.go`, si su limpieza enumera subcomandos a mano.
- [X] T039 [US2] Verificación real (quickstart §6 y §7): `mem expand off` → `rm -rf` permitido; `on` → bloqueado; `off rules` → status lo refleja; `mem update` sobre una instalación previa → `hook expand` presente y protocolo v10; `mem uninstall` → 0 entradas. Registrar en `specs/040-expand-supertools/evidence/us2-lifecycle.md`.

**Checkpoint**: P1 completo. El módulo está activo, es visible, se puede apagar y se desinstala limpio.

---

## Phase 5: User Story 3 - Sugerencias de flujo en el momento justo (Priority: P2)

**Goal**: 5 skills reinterpretadas, instaladas siempre en los 3 harnesses, y sugerencias una por episodio.

**Independent Test**: un `git commit` produce una sugerencia de `verify-loop` una sola vez; las 5 skills existen en `~/.claude/skills`, `~/.codex/skills` y `~/.config/opencode/skills` (quickstart §4 y §9; corregido por C-002 del ACR).

### Tests for User Story 3 ⚠️

- [X] T040 [P] [US3] Escribir `tests/contract/expand_skills_contract_test.go` (rojo) según `contracts/skills.md`:
  - `name` = directorio.
  - Las tools mencionadas entre backticks existen en `domain.AllMCPTools()`.
  - No aparecen `npm`, `npx`, `pnpm`, `yarn`, `~/.claude`, `~/.codex` ni `~/.config/opencode`.
  - `/compact`, `/context` y `/status` solo aparecen dentro de la tabla «Por harness».
  - Está la línea de atribución y existe `infrastructure/templates/expand/NOTICE`.
  - Cada skill nombra sus tools mínimas.
- [X] T041 [P] [US3] Escribir `domain/skill_contract_test.go` (rojo) para `ValidateSkillBody(name, body string, knownTools []string) []string` con casos válidos e inválidos por cada regla anterior.

### Implementation for User Story 3

- [X] T042 [P] [US3] Añadir `AllMCPTools() []string` en `domain/mcp_tools.go`, que agrega todas las listas del archivo sin duplicados. Probarlo en `domain/mcp_tools_test.go`.
- [X] T043 [US3] Implementar `domain/skill_contract.go` (`ValidateSkillBody`) hasta poner T041 en verde, y hacer que T040 lo use (deps: T041, T042).
- [X] T044 [P] [US3] Redactar `infrastructure/templates/expand/verify-loop/SKILL.md` siguiendo `contracts/skills.md`:
  - Detección del stack (go.mod, package.json/tsconfig, *.ps1).
  - Pasos: build, `go vet`, `go test -race`, lint disponible y `git diff --stat`.
  - `search_memories` con `type=bugfix` sobre los archivos tocados.
  - Aplicar rules al diff: `git diff | mem expand rules --check` (CLI como vía principal, porque no hay capacidad MCP equivalente; contracts/skills.md).
  - Veredicto con `save_memory`.
  - `review_start` si el riesgo es alto.
  - Tabla «Por harness».
  - Fallback `mem …` y procedimiento manual.
  - Atribución.
- [X] T045 [P] [US3] Redactar `infrastructure/templates/expand/search-first/SKILL.md`:
  - Orden: memoria (`search_memories`, decisiones o rechazos previos), grafo (`search_code`, `get_symbol`, proveedor externo si existe), skills instaladas y, al final, externo (dependencias o MCP).
  - Matriz Adopt/Extend/Build.
  - `save_memory type=decision`.
  - Atribución.
- [X] T046 [P] [US3] Redactar `infrastructure/templates/expand/iterative-retrieval/SKILL.md`:
  - Bucle dispatch→evaluate→refine (como mucho 3 ciclos) con `pack_build`, `pack_retrieve` y el grafo.
  - Criterio de suficiencia.
  - Entrega del paquete al subagente vía `octopus_route_task` cuando Octopus está disponible.
  - Atribución.
- [X] T047 [P] [US3] Redactar `infrastructure/templates/expand/strategic-compact/SKILL.md`:
  - Cuándo compactar (tras investigar, tras un hito, tras un enfoque fallido) y cuándo no (a mitad de implementación).
  - `save_session_summary` antes de compactar.
  - Tabla «Por harness» con el medidor y la orden de compactación: Claude `/context` y `/compact`; Codex `/status` y `/compact`; OpenCode, el equivalente verificado en su ayuda, o «no disponible: compactación automática» si no existe.
  - Atribución.
- [X] T048 [P] [US3] Redactar `infrastructure/templates/expand/instincts/SKILL.md`:
  - Revisar con `mem expand instincts list`.
  - Confirmar con `mem expand instincts confirm <slug>`, que guarda la preferencia, o `save_memory type=preference topic_key=instinct:<slug>` si no hay CLI.
  - Rechazar con `mem expand instincts reject <slug>`.
  - Criterios para confirmar.
  - Atribución.
- [X] T049 [P] [US3] Crear `infrastructure/templates/expand/NOTICE` con el texto de licencia MIT de ECC (affaan-m/ECC v2.2.3, copyright del autor original) y una nota que diga que las skills son reinterpretaciones y no copias.
- [X] T050 [US3] Publicar las copias `skills/{verify-loop,search-first,iterative-retrieval,strategic-compact,instincts}/SKILL.md`, idénticas byte a byte, hasta poner T001 y T040 en verde (deps: T043–T049).
- [X] T051 [US3] Implementar `setup.InstallExpandSkills(bodies map[string]string) ([]string, error)` en `adapters/primary/setup/agent_skills_setup.go`, en bucle sobre `InstallAgentSkill` sin filtro de agente, y llamarlo en `runGlobalScopeSetup` (`adapters/primary/cli/cmd_mcp_setup.go`, después de socratica) con `embeddedTemplate("expand/<skill>/SKILL.md")`. Probarlo en `adapters/primary/setup/agent_skills_setup_test.go` con un HOME temporal: escribe solo donde existe `~/.claude`, `~/.codex` u `~/.opencode` y es idempotente.
- [X] T052 [US3] Enchufar `SuggestionText` en `ExpandHook` (`application/usecases/expand_hook.go`) con el texto fijo por skill de `contracts/hook-expand.md` («Formato de la sugerencia»). Incrementar el contador de herramientas en cada `pre-tool` y reiniciarlo en `counter.reset` (`hook_expand.go`). Ampliar `tests/integration/hook_expand_test.go`:
  - `git commit` → `verify-loop` una sola vez.
  - Prompt de dependencia → `search-first`.
  - Task → `iterative-retrieval`.
  - 50 herramientas → `strategic-compact`.
  - `session-start` con origen compact → se vuelve a emitir.
  - `mem expand off flow` → ninguna sugerencia.
- [ ] T053 [US3] Verificación real (quickstart §4 y §9) en los 3 harnesses: sugerencia de `verify-loop` visible una sola vez y 5 skills presentes. En OpenCode, comprobar la detección por `~/.opencode` frente a `~/.config/opencode` (en la máquina de referencia existen los dos; 3.ª pasada, U2): si una instalación solo tiene `~/.config/opencode`, documentar el criterio en `docs/expand.md` y, si la binaria de OpenCode lee `~/.config/opencode/skills`, ampliar `skillTargets` con prueba. Registrar en `specs/040-expand-supertools/evidence/us3-flow.md`.

**Checkpoint**: las sugerencias de flujo funcionan sin coste permanente de contexto.

---

## Phase 6: User Story 4 - Escaneo de seguridad de la configuración del agente (Priority: P2)

**Goal**: shield de solo lectura en `mem doctor --shield` y al iniciar sesión cuando cambia la huella.

**Independent Test**: con los fixtures de `contracts/shield.md` aparecen las 4 categorías sin valores de secreto; dos sesiones sin cambios → un único escaneo (quickstart §8).

### Tests for User Story 4 ⚠️

- [X] T054 [P] [US4] Crear los fixtures anonimizados en `tests/testdata/shield/` (uno por fila de la matriz de `contracts/shield.md`, con tokens sintéticos que cumplan `secretPatterns`) y escribir `domain/shield_test.go` (rojo):
  - Cada fixture → la categoría y severidad esperadas.
  - `${VAR}`, `$VAR`, `env:` y `<private>` → ningún hallazgo.
  - Las entradas de `ClaudeAutoAllowTools` no se marcan.
  - S2: la salida (texto y JSON) no contiene el token.
  - Fixtures negativos (3.ª pasada, A1): los hooks de gomemory con `mem` en el PATH (`/opt/homebrew/bin`, `/usr/local/bin`) y un hook en `/usr/local/bin/tool` → ningún hallazgo; un hook en `/tmp/x/run.sh` o en un directorio `o+w` → `hook-outside-home`.
- [X] T055 [P] [US4] Escribir `adapters/secondary/agentconfig/reader_test.go` (rojo) con un HOME y un root temporales:
  - Lista de archivos conocidos de R10.
  - Tope de 32 archivos y de 1 MiB → `skipped`.
  - S1: mtime y contenido iguales tras leer.
  - `Fingerprint` cambia al tocar un archivo y es estable sin cambios.
  - `~/.claude.json` solo expone `mcpServers`.

### Implementation for User Story 4

- [X] T056 [US4] Añadir `domain.FindSecrets(s string) []SecretHit{Category, Line}` en `domain/redact.go`, reutilizando `secretPatterns` sin devolver el valor. Implementar `domain/shield.go` con `ShieldFinding`, `ShieldReport` y los detectores `remote-exec`, `hook-outside-home`, `mcp-inline-credential` (JSON y TOML), `broad-permission` (Claude `permissions.allow`; Codex `sandbox_mode`/`approval_policy`) y `secret`, hasta poner T054 en verde.
- [X] T057 [US4] Crear el puerto `application/ports/agent_config_reader.go` y el adaptador `adapters/secondary/agentconfig/reader.go` (solo `os.Open`, topes y `Fingerprint` por ruta, tamaño y mtime), hasta poner T055 en verde. Construirlo en `infrastructure/container.go`.
- [X] T058 [US4] Implementar `application/usecases/shield.go` (`ShieldScan(ctx, reader ports.AgentConfigReaderPort, state ports.ShieldStateRepository) domain.ShieldReport`, sin E/S directa) y `ShieldRunner` para `ExpandHook`. En `session-start`, solo si `Fingerprint() != state.LastHash()`, se escanea, se guarda con `state.Save(report)` (el adaptador de T017 escribe la huella y el informe), y se emite contexto solo si hay hallazgos (con el recuento por severidad y la remisión a `mem doctor --shield`). Probarlo en `application/usecases/shield_test.go` con un fake: S4, una ejecución por huella.
- [X] T059 [US4] Añadir `--shield` a `mem doctor` en `adapters/primary/cli/cmd_doctor.go`: sección «Shield» en texto, clave aditiva `"shield"` en `--json` (incluida `shield.skipped`; la lista de lo no analizado aparece también en el texto, 3.ª pasada, C1) y código de salida 2 si hay critical o 1 si hay high (si no, el código actual). Probarlo en `adapters/primary/cli/cmd_doctor_shield_test.go`: sin `--shield` la salida actual no cambia, y se comprueban las claves JSON y los códigos.
- [X] T060 [US4] Verificación real (quickstart §8): `mem doctor --shield --json` en la máquina de referencia, sin falsos positivos critical. Revisar cada hallazgo, ajustar los detectores si alguno es falso y añadirlo como caso negativo en T054. Registrar en `specs/040-expand-supertools/evidence/us4-shield.md`.

**Checkpoint**: la configuración de los agentes se audita sin escribir nada y sin filtrar secretos.

---

## Phase 7: User Story 5 - Aprendizaje continuo por instincts (Priority: P3)

**Goal**: observación silenciosa, plegado por lotes, confianza, promoción global e inyección de los de confianza ≥ 0.7, con list, confirm y reject.

**Independent Test**: el mismo comando repetido en 2 proyectos → instinct global con confianza creciente, y `## Instincts` en el contexto cuando llega a ≥ 0.7 (quickstart §10).

### Tests for User Story 5 ⚠️

- [X] T061 [P] [US5] Escribir `domain/instinct_test.go` (rojo):
  - `InstinctConfidence(obs, corrections)` con la tabla 1→0.3, 2→0.4, 4→0.5, 8→0.6, 16→0.7, 64→0.9, 1024→0.9 y correcciones +0.1 con tope 0.9.
  - `ObservationKey(cmd)`: `go test -race ./...` → `cmd:go-test`, `git commit -m "x"` → `cmd:git-commit`, comando vacío → sin clave.
  - `CorrectionKey(prompt)`: patrones en español e inglés («no, usa X en vez de Y», «siempre …», «nunca …», «don't …», «always …»), con ≥ 8 positivos y ≥ 8 negativos.
  - Transiciones `proposed → confirmed|rejected` finales.
- [X] T062 [P] [US5] Escribir `adapters/secondary/persistence/instincts_test.go` (rojo) contra SQLite real:
  - La migración crea `instinct_stats` en una base existente sin tocar datos.
  - El upsert por (project, slug) suma observaciones.
  - Plegar dos veces el mismo lote (mismos `obs_id`) no cambia `observations` (`instinct_folded`; 2.º analyze H2).
  - `rejected` no vuelve a `proposed`.
  - El registro global `instincts.db` (con `GOMEMORY_DATA_HOME` temporal) acumula `projects` sin duplicar y fija `scope=global` con 2 proyectos.
  - Permisos 0600.

### Implementation for User Story 5

- [X] T063 [US5] Implementar `domain/instinct.go` (`Instinct`, `InstinctConfidence`, `ObservationKey`, `CorrectionKey`, reglas de transición) hasta poner T061 en verde.
- [X] T064 [US5] Añadir `CREATE TABLE IF NOT EXISTS instinct_stats (…)` e `instinct_folded (…)` (data-model §2; 2.º analyze H2) en `migrate()` de `adapters/secondary/persistence/db.go`. Implementar `adapters/secondary/persistence/instincts.go` (`InstinctRepository`: `Fold(ctx, []Observation, now)`, idempotente por `obs_id` mediante `INSERT OR IGNORE` en `instinct_folded` en la **misma** transacción que el `INSERT … ON CONFLICT` sobre `instinct_stats` (3.ª pasada, F3), `List`, `SetStatus`, `ForContext(minConf)`) y el registro global (`OpenInstinctGlobal(DataHome)`, `RecordProject`, `GlobalFor(minConf)`). Definir los puertos en `application/ports/instinct_repository.go`. Todo debe poner T062 en verde.
- [X] T065 [US5] Ampliar `mem sanitize` (3.ª pasada, K1, §20) para que cubra los tres almacenes de texto de 040:
  - `instinct_stats.description` en `adapters/secondary/persistence/privacy_maintenance.go`, con una categoría propia por columna (memorias 510 y 599).
  - `instinct_global.description` en `DataHome()/instincts.db`, mediante `InstinctGlobalRepository.Sanitize(ctx, redact)`.
  - El campo `sample` de la cola, mediante `ObservationQueueRepository.Sanitize(ctx, redact)`.

  `mem sanitize` informa una categoría por almacén. Pruebas: un secreto sembrado en cada almacén desaparece tras `sanitize` y el recuento por categoría es correcto; la cola es transitoria (una observación plegada ya no está en el archivo).
- [X] T066 [US5] Implementar en `application/usecases/instincts.go`, sin E/S directa (puertos `ObservationQueueRepository`, `InstinctRepository` e `InstinctGlobalRepository`; analyze D1):
  - `Observe`: `queue.Append` con `RedactSecrets`, limpieza de `<private>` y `sample` ≤ 200 caracteres.
  - `FoldPending(ctx, now)`, en un orden fijo (analyze D2; 2.º analyze H2, R18):
    1. `queue.ReadBatch(ctx, 500)`.
    2. `repo.Fold(ctx, batch, now)` en **una** transacción: `INSERT OR IGNORE INTO instinct_folded` por `obs_id` y agregar solo las observaciones nuevas (filas afectadas = 1).
    3. Tras el commit, `queue.Remove(ctx, ids)` (best-effort: una observación ya plegada nunca vuelve a contar).
    4. Purgar `instinct_folded` de más de 7 días con `now`.
    5. `global.RecordProject` para **todos** los instincts del proyecto con confianza ≥ 0.5 (reconciliación por estado). Un fallo de este paso se registra, pero no reintroduce observaciones.
    6. Si `Fold` falla, no se trunca nada ni se toca el registro global.
  - Enchufar `InstinctObserver` en `ExpandHook`: `observe` en `post-tool` y `prompt`; `fold` en `stop` y `session-start`.

  Probarlo en `application/usecases/instincts_test.go` con fakes y un reloj inyectado:
  - Un secreto en el comando no llega a la cola.
  - Nada se escribe en stdout.
  - Si `RecordProject` falla, `observations` no se duplica en el siguiente plegado y el proyecto queda registrado al reintentar.
  - Si `Fold` falla, la cola sigue intacta.
  - **Muerte tras el commit** (2.º analyze H2): `Fold` hace commit y `Remove` no llega a ejecutarse; el siguiente `FoldPending` con la misma cola no cambia `observations`.
  - Con 1200 observaciones en cola y un `ctx` corto, cada plegado avanza un lote de como mucho 500 sin estado intermedio.
- [X] T067 [US5] Añadir la sección `## Instincts` (después de `## Preferencias del Usuario`; los de confianza ≥ 0.7 del proyecto y globales, sin `rejected`, como mucho 8 líneas) en `application/usecases/build_context.go`, y añadir el encabezado a la lista de `application/usecases/session_delta.go`. Es el **único** punto de inyección, porque el router no tiene acción de inyección (analyze C1). Con `ExpandDisabled` o `instincts` en `ExpandOff`, la sección se omite. Probarlo en `build_context_test.go`: umbral, rechazados, global y sección omitida con el interruptor.
- [X] T068 [US5] Añadir `mem expand instincts list [--json] [--all]`, `confirm <slug>` (`SetStatus(confirmed)` + `save_memory type=preference topic_key=instinct:<slug>` por el caso de uso de memorias existente) y `reject <slug>` a `adapters/primary/cli/cmd_expand.go`. Ampliar `cmd_expand_test.go` y la ayuda detallada.
- [X] T069 [US5] Ampliar `tests/integration/hook_expand_test.go`:
  - 16 llamadas `post-tool` con `go test ./...` más una `stop` → `instincts list` muestra `cmd:go-test` con confianza 0.7.
  - El mismo flujo en un segundo proyecto → `scope=global`.
  - `post-tool` nunca escribe en stdout ni en stderr.
- [ ] T070 [US5] Verificación real (quickstart §10) en dos repositorios. Registrar en `specs/040-expand-supertools/evidence/us5-instincts.md`.

**Checkpoint**: instincts aprende sin ruido y solo inyecta lo consolidado.

---

## Phase 8: User Story 6 - Evolucionar un instinct a skill (Priority: P4)

**Goal**: `mem expand instincts evolve <slug>` con un LLM elegido por harness y aprobación obligatoria.

**Independent Test**: con TTY, la propuesta se muestra y no se escribe hasta confirmar; sin TTY, código 2 (quickstart §11).

- [X] T071 [P] [US6] Escribir la prueba en `adapters/primary/cli/cmd_expand_evolve_test.go` (rojo), con un fake de `SocraticGenerator` y un catálogo de harness:
  - Sin TTY → 2 sin escribir.
  - Sin harness disponible → 1.
  - Fallo del generador → 1 sin archivos.
  - Propuesta inválida según `ValidateSkillBody` → 1 con el motivo.
  - Cancelar → 3 sin escribir.
  - Confirmar → `InstallAgentSkill("instinct-<slug>")` con el cuerpo devuelto.
- [X] T072 [US6] Crear la plantilla `infrastructure/templates/expand/evolve-prompt.md`, que pide un SKILL.md en el formato de `contracts/skills.md` a partir del instinct incrustado. Implementar `Evolve` en `application/usecases/instincts.go`: construye una `domain.RefinementFlow` en modo refine sin preguntas, llama a `SocraticGenerator.Rewrite`, valida con `ValidateSkillBody` y devuelve la propuesta sin escribir. Añadir `evolve` en `cmd_expand.go` con la misma selección de harness que `mem socratica`, más `console.Confirm` e instalación (R13). Debe poner T071 en verde.
- [X] T073 [US6] Verificación real (quickstart §11) con un harness disponible. Registrar en `specs/040-expand-supertools/evidence/us6-evolve.md`.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T074 Medir la latencia (quickstart §5) de los **5 eventos** (2.º analyze C2): con 64 reglas, p95 < 150 ms en `pre-tool`, `prompt`, `stop` (con cola de 500) y `session-start` (con escaneo shield de 32 archivos y plegado); `post-tool` < 20 ms. Si `session-start` no cabe, el escaneo guarda la huella aunque sea parcial, y lo `skipped` se reintenta solo cuando cambia la huella. Si no se cumple, aplicar la ruta ligera de `Deps` de R5 y volver a medir. Registrar en `specs/040-expand-supertools/evidence/latency.md`.
- [X] T075 [P] Crear `docs/expand.md`: qué hace cada supertool, formato de las reglas, regla de serie y cómo anularla, interruptor, shield, instincts, skills y matriz de compatibilidad por harness con el resultado de T032, T053 y T060.
- [X] T076 [P] Actualizar `docs/DATABASE.md` (`instinct_stats`, `instinct_folded` e `instinct_global`; 3.ª pasada, I3) y `docs/ARQUITECTURA.md` (router, puertos `AgentConfigReaderPort`, `RuleSourcePort`, `InstinctRepository` y flujo de `hook expand`).
- [X] T077 [P] Actualizar `README.md` (sección expand y nota de que `.gomemory/rules/` se versiona) y `CHANGELOG.md` (entrada de feature, protocolo v10 y atribución a ECC).
- [ ] T078 Ejecutar `go vet ./...`, `go test -race ./...`, `scripts/coverage.sh` (umbral 80 %) y las pruebas TS y PS del workflow `test.yml`. Todo en verde.
- [ ] T079 Revisión final, registrada en `specs/040-expand-supertools/evidence/final.md`:
  - `mem expand status` y `mem doctor --shield` en la máquina de referencia.
  - FR-036: `git diff master -- go.mod go.sum` sin dependencias nuevas, `grep -rn 'exec.Command("node\|python' adapters application domain` vacío y sin duplicar capacidades existentes (analyze C3).
  - Repaso de `git status` sin archivos de IA, certificados ni `.env` (regla del usuario).
  - Registro de las decisiones finales en gomemory con `save_memory`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias.
- **Foundational (Phase 2)**: depende de Setup y bloquea todas las historias.
- **US1 (Phase 3)**: depende de Foundational. Es el MVP.
- **US2 (Phase 4)**: depende de US1, porque el cableado está en US1 y `status`/uninstall lo inspeccionan.
- **US3 (Phase 5) y US4 (Phase 6)**: dependen de Foundational y del cableado de US1 (T028–T030); son independientes entre sí.
- **US5 (Phase 7)**: depende de Foundational y del cableado de US1. Comparte `cmd_expand.go` con US2, así que va después de T036.
- **US6 (Phase 8)**: depende de US5 (instincts) y de T043 (`ValidateSkillBody`).
- **Polish (Phase 9)**: después de las historias que se vayan a entregar.

### Within Each User Story

Pruebas (rojo), luego dominio, adaptadores, caso de uso, CLI o hook, integración y verificación real.

### Parallel Opportunities

- Setup: T002 ∥ T003.
- Foundational: T005 ∥ T008 ∥ T009 ∥ T011 ∥ T014 ∥ T016.
- US1: T021 ∥ T022 ∥ T023. T028, T029 y T030 tocan archivos distintos y pueden ir en paralelo después de T026.
- US3: T044–T049 en paralelo (archivos distintos); T040 ∥ T041 ∥ T042.
- US4: T054 ∥ T055.
- US5: T061 ∥ T062.
- US3 ∥ US4 completas tras US1.

---

## Parallel Example: User Story 3

```text
Task: "T044 Redactar verify-loop/SKILL.md"
Task: "T045 Redactar search-first/SKILL.md"
Task: "T046 Redactar iterative-retrieval/SKILL.md"
Task: "T047 Redactar strategic-compact/SKILL.md"
Task: "T048 Redactar instincts/SKILL.md"
Task: "T049 Crear NOTICE"
```

---

## Implementation Strategy

### MVP First

1. Phase 1 + Phase 2.
2. Phase 3 (US1): bloqueo destructivo en los 3 harnesses, con verificación real T032.
3. Phase 4 (US2): ciclo de vida e interruptor. Con esto, P1 está completo y se puede publicar.

### Incremental Delivery

4. US3 y US4 (P2), en cualquier orden o en paralelo.
5. US5 (P3).
6. US6 (P4), opcional en la primera entrega.
7. Polish con la latencia medida y la documentación.

### Notes

- Toda E/S nueva (puertos, adaptadores y casos de uso) recibe `context.Context` como primer parámetro (constitución §1.2; 2.º analyze K2), aunque una tarea no lo repita.
- El proceso es secuencial por defecto (regla del usuario). [P] solo indica independencia, no obliga a paralelizar.
- «Verde en tests» no basta: cada historia cierra con una tarea de verificación real sobre el binario recompilado y los harnesses (T032, T039, T053, T060, T070, T073).
- No hacer commit sin mostrar antes la lista de archivos y esperar aprobación.
