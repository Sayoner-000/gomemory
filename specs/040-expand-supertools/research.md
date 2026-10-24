# Research: 040 mem expand

Cada decisión parte del código actual (verificado el 2026-10-21 con codegraph y lecturas focalizadas), no del plan de entrada. Cuando el código contradice ese plan, se indica en **Corrige**.

## R1. Un solo subcomando de hook con el evento como argumento

- **Decision**: `mem hook expand <event>`, con `<event>` ∈ {`session-start`, `prompt`, `pre-tool`, `post-tool`, `stop`} y `--emit=<dialecto>` opcional, como ya hace el resto de hooks.
- **Rationale**: `hookCommandIsGomemory` y `CodexHookPresente` reconocen por subcadena `hook <sub>`, y `gomemoryHookSubRe` (`hook ([a-z-]+)`) extrae `expand`. Un único subcomando es una sola entrada que reconocer, deduplicar y desinstalar. Además, `hookSubName` ya parte en el primer espacio (`"expand pre-tool"` → `"expand"`), igual que `"tool-output claude"`.
- **Alternatives**: un subcomando por supertool (`hook rules`, `hook shield`…). Se rechaza porque multiplica los procesos por evento y las entradas que tiene que reconocer el desinstalador.

## R2. Registro único de subcomandos de hook (cierre de la brecha FR-009)

- **Decision**: se crea `domain.GomemoryHookSubcommands` (lista ordenada que incluye `expand`). `hookCommandIsGomemory` pasa a recorrer esa lista en lugar de su cadena de `strings.Contains`. La prueba `TestHookCommandIsGomemoryRecognizesAllSubcommands` se amplía: además de `claudeHookEvents`, recorre `CodexGomemoryHooks()`, los subcomandos que invoca el plugin de OpenCode (extraídos de `gomemory.ts` por expresión regular en la prueba) y la propia lista del dominio. También comprueba que cada `case` de `CmdHook` que aparece en un instalador figura en el registro.
- **Rationale**: hoy la lista está escrita a mano en tres sitios y la prueba solo recorre los eventos de Claude. Un subcomando que solo usen Codex u OpenCode quedaría sin reconocer.
- **Alternatives**: añadir `|| strings.Contains(cmd, "hook expand")` y nada más. Se rechaza porque deja la brecha abierta para el siguiente subcomando.

## R3. Matchers y eventos por harness

| Harness | Evento nativo | Matcher | Argumento de `expand` | Dialecto |
|---|---|---|---|---|
| Claude Code | `SessionStart` | `startup\|resume\|clear\|compact` | `session-start` | claude (detectado) |
| Claude Code | `UserPromptSubmit` | `""` | `prompt` | claude |
| Claude Code | `PreToolUse` | `Bash\|Write\|Edit\|MultiEdit\|NotebookEdit\|Task\|Agent` | `pre-tool` | claude |
| Claude Code | `PostToolUse` | `Bash` | `post-tool` | claude |
| Claude Code | `Stop` | `""` | `stop` | claude |
| Codex | `SessionStart` | `startup\|resume\|clear\|compact` | `session-start` | json |
| Codex | `UserPromptSubmit` | — | `prompt` | json |
| Codex | `PreToolUse` | `Bash\|apply_patch\|collaborationspawn_agent` (verificado: `evidence/codex-probe.md`) | `pre-tool` | json |
| Codex | `PostToolUse` | `Bash` | `post-tool` | json |
| Codex | `Stop` | — | `stop` | json |
| OpenCode | `session.created` / `session.compacted` | — | `session-start` | text |
| OpenCode | `chat.message` | — | `prompt` | text |
| OpenCode | `tool.execute.before` | todas (filtro en Go) | `pre-tool` | text |
| OpenCode | `tool.execute.after` | todas (filtro en Go) | `post-tool` | text |
| OpenCode | `session.idle` | — | `stop` | text |

- **Rationale**: el matcher de `PreToolUse` limita los procesos a las herramientas sobre las que actúan las reglas (shell, escritura de archivos y delegación), y así Read/Grep/Glob no pagan el coste. `PostToolUse` solo hace falta para observar comandos (instincts). OpenCode no tiene matcher nativo, así que el filtro se hace en Go.
- **Codex `PreToolUse`** (decisión de la persona tras analyze F1/F2: ampliar y verificar en real). La tabla de `codex_setup.go` exige verificar cada hook en sesión interactiva, porque `codex exec` no ejecuta hooks. Antes de implementar se hace una **sonda real** (T004) con estas condiciones:
  - `CODEX_HOME` temporal, para no tocar la configuración de la persona.
  - Un hook de registro que solo hace `cat >> log`.
  - Matcher `Bash|apply_patch`.

  La sesión pide tres acciones: un comando de shell, una edición de archivo y una delegación. Se registran los `tool_name` reales y si una respuesta `permissionDecision: deny` bloquea. Esto fija el matcher definitivo de T027.
  - Si `apply_patch` no dispara o Codex ignora la denegación, se documenta con evidencia como excepción y el matcher conserva solo lo que funcione.
  - Si Codex no tiene herramienta de delegación, no hay nada que sugerir en `iterative-retrieval`. Eso no es una ruptura de paridad, porque el disparador no existe.
  - **Resultado de la sonda (2026-10-21, codex-cli 0.163.0, `evidence/codex-probe.md`)**:
    - `codex exec` no ejecuta hooks (reconfirmado).
    - En sesión interactiva, `PreToolUse` dispara con `tool_name` = `Bash`, `apply_patch` y `collaborationspawn_agent`.
    - La respuesta `hookSpecificOutput.permissionDecision: "deny"` **bloquea** tanto `Bash` como `apply_patch` («Command blocked by PreToolUse hook: …»).

    El matcher definitivo es `Bash|apply_patch|collaborationspawn_agent`, sin excepciones para Codex. El payload de Codex trae `hook_event_name` + `tool_name` (forma Claude). En `apply_patch`, el parche viaja en `tool_input.command`.
- **OpenCode, bloqueo**: en `tool.execute.before` se bloquea lanzando un `Error` con el motivo. El plugin hoy no implementa ese gancho, así que es código nuevo en `gomemory.ts`.
- **Corrige**: el plan instalaba el hook en 4 eventos y evaluaba rules también en `Stop`. Se añade `Stop` (spec, Clarifications).

## R4. Salida por dialecto y por evento

- **Decision**: `renderGuardDecision` está ligado a plan-guard (codifica `"hookEventName": "PreToolUse"` y solo conoce `deny`). Se añade `renderExpandOutput(d hookDialect, event string, out domain.ExpandOutcome)` en `hook_dialect.go`, que reutiliza la misma forma:
  - `block` en `pre-tool`: claude/json → `hookSpecificOutput{hookEventName:"PreToolUse", permissionDecision:"deny", permissionDecisionReason}`; text → motivo en stdout con prefijo `BLOCK:` (el plugin lanza `Error`); neutral → stderr y exit 1.
  - `block` en `prompt` o `stop`: claude/json → `{"decision":"block","reason":…}`; text y neutral, igual que arriba.
  - `warn` y sugerencias: contexto adicional mediante `promptHookJSON`, generalizado con `hookEventName` como parámetro. En `pre-tool` se emite solo `additionalContext`, **sin** `permissionDecision`, para no aprobar la herramienta en silencio (2.º analyze H1, R18).
  - `allow` sin texto: lo mismo que hoy, `{}` en claude/json y cadena vacía en text/neutral.
- **Rationale**: se reutiliza el contrato de cuatro dialectos ya probado, y `renderGuardDecision` no cambia (no hay regresión en plan-guard).
- **Alternatives**: parametrizar `renderGuardDecision`. Se rechaza porque tocaría un contrato probado (`contracts/hook-plan-guard.md` de 019) por un caso nuevo.

## R5. Presupuesto, fallo abierto y estado sin SQLite en el camino caliente

- **Decision**: cada `hook expand` se ejecuta dentro de `withBudget(domain.HookBudget, …)`, que ya vale 150 ms. En `pre-tool` y `prompt` no se abre SQLite: las reglas se leen de la caché compilada, la deduplicación por episodio va en un archivo marcador y el contador de herramientas en otro. Los registros de activación (`recordGuard` sobre `hook_guard_events`) solo se escriben cuando la acción emite algo (`warn`, `block` o una sugerencia), y en ese caso dentro del presupuesto.
- **Rationale**: el patrón `plan_episode.go` ya justifica no abrir SQLite en el camino del guard. `withBudget` recupera pánicos y devuelve `nil` al vencer, que es exactamente el fallo abierto que pide FR-003/FR-004.
- **Riesgo**: que el arranque del proceso (`Deps`) abra la base de todos modos. T072 lo mide; si el p95 supera 150 ms, se usa una ruta de `Deps` sin base de datos para `hook expand pre-tool|prompt`.
- **Registro de activaciones (analyze F3)**: incluso en la ruta ligera, `recordGuard` abre la base de forma **perezosa**, y solo cuando el resultado emite algo (`warn`, `block`, `suggest`) o hay un error. Así `mem expand status` muestra los bloqueos sin cargar la base en el caso común (allow silencioso), que es la inmensa mayoría de eventos.
- **Errores (analyze D4)**: un pánico, un payload inválido o un error de E/S de una acción registra `expand.error`, con el nombre de la acción y sin contenido, por la misma vía perezosa y sin salida visible.

## R6. Dónde vive cada archivo

- **Decision**: las reglas propias van en `<root>/.gomemory/rules/*.md`, un directorio nuevo y **versionable**. Todo el estado local va bajo `.memory/` (ignorado por git):
  - `.memory/.expand-rules-cache.json`: reglas compiladas, con mtime y tamaño por archivo.
  - `.memory/.expand-episode-<sid8>`: claves de sugerencia ya emitidas.
  - `.memory/.expand-tools-<sid8>`: contador de herramientas desde la última compactación.
  - `.memory/.expand-shield-hash` y `.memory/expand-shield-last.json`.
  - `.memory/expand-observations.ndjson`: cola de observaciones, con tope de 256 KiB y rotación.
- **Rationale**: `.memory/` está en `.gitignore` (línea 3), así que unas reglas guardadas ahí no se podrían compartir con el equipo. Una caché dentro de `.gomemory/` acabaría en git.
- **Corrige**: el plan proponía la caché «en .gomemory/».
- `<sid8>` son los 8 primeros caracteres hex del SHA-256 del `session_id` del payload, o `default` si no viene. Así sesiones simultáneas no comparten episodio (edge case de la spec). Los marcadores huérfanos de más de 7 días se purgan en `session-start`; la purga recibe `now` inyectado (analyze D3).
- El contador cuenta **herramientas interceptadas**: shell, escritura y delegación, que son las que pasan por el matcher de `pre-tool`. No se añade un hook sin matcher solo para contar, porque costaría un proceso en cada `Read`/`Grep` (analyze U1; FR-001 y FR-023 ya lo definen así).
- Los archivos que escriben los **casos de uso** (cola de observaciones y estado de shield) pasan por puertos (R16). Los marcadores del adaptador primario (episodio y contador) se quedan en `hook_expand_state.go`, porque son estado del adaptador y no lógica de aplicación, con el mismo patrón que `plan_episode.go`.

## R7. Formato y evaluación de reglas

- **Decision**: cada archivo `.md` tiene un frontmatter YAML mínimo, con las claves `name`, `event` (`bash|file|prompt|stop|any`), `pattern`, `match` (`contains|regex`, por defecto `contains`), `action` (`warn|block`), `enabled` (por defecto `true`) y `overrides` (opcional: nombre de una regla de serie que esta anula). El cuerpo Markdown es el mensaje que ve el agente. Se parsea a mano con el mismo parser de frontmatter que ya usan las skills, sin dependencias nuevas.
- **Topes**: 16 KiB por archivo, 64 reglas, 512 caracteres por patrón. Las expresiones regulares usan RE2 (`regexp` de Go), que tiene tiempo lineal y por tanto no sufre backtracking catastrófico; esto cierra el edge case «regex patológica». El texto pasa por `domain.StripTerminalControls` al leerse.
- **Precedencia**: `block` > `warn` > `allow`. Entre reglas con la misma acción gana la primera por orden de nombre de archivo; la regla de serie se evalúa primero.
- **Coincidencias**: `bash` se compara con `tool_input.command`; `file` con la ruta del archivo y el contenido nuevo (`file_path`, `content`/`new_string`, truncado a 64 KiB). En Codex (`apply_patch`), las rutas salen de las cabeceras `*** (Add|Update|Delete) File: <ruta>` de `tool_input.command` y el contenido es el cuerpo del parche, mediante la función pura `domain.ParseApplyPatch`; `prompt` con el texto del prompt; `stop` con `last_assistant_message` si viene.

## R8. Regla de serie destructive-shell

- **Decision**: es una regla Go embebida (`domain.BuiltinRules`), no un archivo. Se tokeniza el comando con un lexer de shell mínimo (comillas y separadores `;`, `&&`, `||` y `|`) y se evalúa cada subcomando:
  - `rm` con las opciones `-r`/`-R`/`--recursive` y `-f`/`--force` juntas o separadas (`-rf`, `-fr`, `-r -f`), sobre cualquier objetivo.
  - `git reset --hard`.
  - `git push` con `--force`, `-f` o `--force-with-lease`. Esta última se trata como `warn`, no `block`.
  - `git checkout -- .` y `git restore .` (equivalente moderno), solo con el destino `.` o `:/`.
  - `git clean` con `-f` y `-d`.
- **Rationale**: se usan tokens y no subcadenas para evitar falsos positivos como `echo "rm -rf"` o `grep -- -rf`.
- **Anulación**: con una regla propia `overrides: destructive-shell`, `enabled: false`, o con `mem expand off rules`.

## R9. Router: tabla de disparadores

- **Decision**: `domain.RouteSupertools(sig ExpandSignals, st ExpandSwitches) []SupertoolAction` es una función pura, sin E/S, reloj ni azar.

| Evento | Condición | Acciones |
|---|---|---|
| session-start | siempre | `instincts.fold` |
| session-start | `ConfigHash != LastShieldHash` | `shield.scan` |
| session-start | origen compact | `counter.reset`, `episode.reset` |
| prompt | siempre | `rules.eval(prompt)` |
| prompt | `domain.MatchesDependencyIntent(prompt)` | `suggest(search-first)` |
| prompt | siempre | `instincts.observe(prompt)` (solo patrones de corrección) |
| pre-tool | herramienta de shell | `rules.eval(bash)` |
| pre-tool | shell y `git commit`, `gh pr create` o `glab mr create` | `suggest(verify-loop)` |
| pre-tool | Write/Edit/MultiEdit/NotebookEdit | `rules.eval(file)` |
| pre-tool | Task/Agent | `suggest(iterative-retrieval)` |
| pre-tool | `ToolCount >= CompactThreshold` | `suggest(strategic-compact)` |
| post-tool | herramienta de shell | `instincts.observe(command)` |
| stop | siempre | `rules.eval(stop)`, `instincts.fold` |

- **Sin `instincts.inject` en el router (analyze C1)**: la inyección la hace `build_context.go` (R12), que ya alimenta el SessionStart con las preferencias. Así hay un único punto de inyección y no hay riesgo de duplicarla. Con instincts apagada, `build_context` omite la sección.
- Una supertool apagada elimina sus acciones; con `ExpandDisabled` el resultado es `nil`. Las sugerencias ya emitidas en el episodio las filtra el adaptador, no el dominio, porque es estado.
- `CompactThreshold` = 50 herramientas por defecto (supuesto de la spec), configurable con `expand_compact_threshold` en los ajustes.
- `MatchesDependencyIntent` es una lista cerrada de verbos y objetos en español e inglés («añade/instala/integra/agrega … dependencia/librería/paquete/SDK/integración/MCP», «add/install/integrate … dependency/library/package/SDK/integration»). Ante la duda devuelve `false`.

## R10. Shield

- **Decision**: es un caso de uso puro en `application/usecases/shield.go` que recibe un puerto `ports.AgentConfigReaderPort` (devuelve rutas y contenido con topes) y produce `[]domain.ShieldFinding`. Las reglas de detección van en `domain/shield.go`:
  - `secret`: reutiliza `secretPatterns` de `domain/redact.go`. Se expone una función `domain.FindSecrets(s) []SecretHit` que devuelve la categoría y la línea, nunca el valor. Severidad critical.
  - `remote-exec`: comandos de hook que contienen `curl|wget … | sh|bash|zsh|pwsh|iex`. Severidad high.
  - `hook-outside-home` (3.ª pasada de analyze, A1): el ejecutable o script del hook está en un directorio temporal o escribible por todos (`/tmp`, `/private/tmp`, `/var/tmp`, `$TMPDIR`, o cualquier directorio con el bit `o+w` sin sticky), o es una ruta absoluta fuera de `$HOME` que no está en ningún directorio del `PATH` de la persona. Los binarios del PATH (`/opt/homebrew/bin/mem`, `/usr/local/bin/…`) no se marcan, así que los hooks de gomemory nunca generan este hallazgo. Severidad medium.
  - `mcp-inline-credential`: un servidor MCP con `env` o `args` que contienen un valor que encaja con `secretPatterns` o una clave `*_TOKEN|*_KEY|*SECRET*|PASSWORD` con valor literal no vacío y que no es `${…}`. Severidad high.
  - `broad-permission`: en `permissions.allow`, entradas `Bash`, `Bash(*)`, `Bash(:*)` o `*`, o `mcp__*` sin servidor. En Codex, `sandbox_mode = "danger-full-access"` o `approval_policy = "never"`. Severidad medium.
- **Archivos**: `~/.claude/settings.json`, `<root>/.claude/settings.json`, `<root>/.claude/settings.local.json`, `<root>/.mcp.json`, `~/.claude.json` (solo la clave `mcpServers`), `~/.codex/config.toml`, `~/.config/opencode/opencode.json(c)`, y `AGENTS.md`/`CLAUDE.md` del proyecto y del usuario. Topes: 1 MiB por archivo y 32 archivos; los que se saltan se informan como `skipped`.
- **Huella**: SHA-256 de (ruta, tamaño, mtime) de esos archivos, sin leer el contenido. Así el cálculo en `session-start` es barato.
- **Código de salida**: `mem doctor --shield` devuelve 2 si hay algún hallazgo critical, 1 si hay high y 0 en el resto de casos. Sin `--shield`, `mem doctor` no escanea, para no cambiar el contrato actual.
- **Falsos positivos (7.6)**: los valores `${VAR}`, `$VAR`, `env:` o `<private>` no son secretos. Los permisos que el propio gomemory añade (`ClaudeAutoAllowTools`) se excluyen de `broad-permission`.

## R11. Flow-skills: ubicación, sincronía y contrato

- **Decision**: las fuentes canónicas van en `infrastructure/templates/expand/<skill>/SKILL.md` y `infrastructure/templates/expand/NOTICE`; las copias publicadas, en `skills/<skill>/SKILL.md`. Ya están embebidas por `go:embed all:templates`.
- **Corrige**: `TestSkillSourcesInSync` solo mira el primer nivel de `infrastructure/templates/` y salta en silencio las skills sin copia publicada (salvo socratica). Se amplía para recorrer también `expand/*` y exigir copia publicada en las skills de expand.
- **Prueba de contrato** (`tests/contract/expand_skills_contract_test.go`), que falla si:
  - `name` del frontmatter es distinto del directorio.
  - Se nombra una tool MCP con forma `snake_case` de gomemory que no está en `domain.AllMCPTools()`, que se crea si no existe agregando las listas de `domain/mcp_tools.go`.
  - Aparece `npm`, `npx`, `pnpm`, `yarn`, `~/.claude` o `/compact`, `/context`, `/status` fuera de la tabla `| Claude Code | Codex | OpenCode |`.
  - Falta la línea de atribución a ECC.
- **Instalación**: `setup.InstallExpandSkills(bodies map[string]string)` hace un bucle sobre `InstallAgentSkill` y se llama desde `runGlobalScopeSetup` junto a socratica.
- **Corrige**: las rutas reales de skills son `~/.claude/skills`, `~/.codex/skills` y `~/.config/opencode/skills` (`skillTargets`). *Revisado tras C-002 del ACR*: la versión inicial de esta nota daba `~/.opencode/skills`, que OpenCode 2.0.32 solo descubre por el walk-up de un proyecto bajo el home (`opencode debug config`); ver `evidence/us3-skills-opencode.md`.

## R12. Instincts: almacenamiento y heurística

- **Decision**:
  - **Observación** (camino caliente): se añade una línea NDJSON `{t, kind: cmd|correction, key, sample}` a `.memory/expand-observations.ndjson`. `key` es el n-grama normalizado: binario y subcomando (`go test`, `git commit`), o bigramas de comandos consecutivos guardados en el mismo archivo. `sample` pasa antes por `domain.RedactSecrets` y la limpieza de `<private>`, y se limita a 200 caracteres.
  - **Plegado**: en `stop` y en `session-start`, dentro del presupuesto, se lee la cola, se agrega y se actualiza `instinct_stats` en `mem.db` del proyecto. Es una migración aditiva en `migrate()`: `instinct_stats(project, slug PK, kind, description, observations, confidence, scope, status, first_seen, last_seen)`.
  - **Idempotencia por observación (2.º analyze H2, R18)**: tabla `instinct_folded` y lote acotado de 500. El orden de D2 se mantiene para el registro global.
  - **Idempotencia (analyze D2)**: el orden es fijo.
    1. `Fold` en `mem.db`, en una transacción.
    2. Si hace commit, **se trunca la cola de inmediato**.
    3. Después, `RecordProject` en el registro global, que es idempotente porque `projects` es un conjunto. Un fallo en este paso no deja nada que recontar: el siguiente plegado vuelve a llamar a `RecordProject` para todos los instincts del proyecto con confianza ≥ 0.5 (reconciliación por estado, no por lote).
    4. Si `Fold` falla, no se trunca nada y no se toca el registro global.
  - **Confianza**: `c = min(0.9, 0.3 + 0.1·⌊log2(observations)⌋)`; las correcciones explícitas suman +0.1. Es una fórmula pura y probada por tabla (`domain.InstinctConfidence`).
  - **Promoción global**: hay un registro global SQLite en `DataHome()/instincts.db`, con la tabla `instinct_global(slug PK, kind, description, projects JSON, confidence)`. Al plegar un instinct de confianza ≥ 0.5 se registra la `ProjectKey` del proyecto. Si `len(projects) >= 2`, `scope=global`.
  - **Memoria**: al confirmar (skill `instincts` → `mem expand instincts confirm <slug>`) se guarda `save_memory type=preference topic_key=instinct:<slug>`. Al rechazar, `status=rejected` y nunca se reinyecta.
  - **Inyección**: `build_context.go` añade la sección `## Instincts` después de `## Preferencias del Usuario`, con los instincts del proyecto y globales de confianza ≥ 0.7 y estado distinto de `rejected`. La sección se añade también a la lista de `session_delta.go`.
- **Corrige**: el plan guardaba los instincts como memorias `type=instinct` desde la observación. Eso llenaría el contexto con propuestas no confirmadas y obligaría a añadir un `MemoryType`. Solo los confirmados pasan a memoria, como `preference`, y `ValidMemoryType` no cambia.
- **Rationale de la cola**: no hay escritura SQLite por herramienta, el plegado va por lotes y no hace falta un agente en segundo plano.
- **Rationale del registro global aparte**: desde la feature 005 cada proyecto tiene su propio `mem.db`, aislado del resto. La promoción exige ver en qué proyectos aparece un instinct, y ningún `mem.db` puede responder eso sin abrir los de los demás. `instincts.db` es la única base compartida de `DataHome()` y solo guarda `slug`, el conjunto de `ProjectKey` y la confianza; las observaciones y el estado siguen en el `mem.db` de cada proyecto.
- **Alternatives**: una tabla en cada `mem.db` y la agregación al leer. Se rechaza porque cada plegado tendría que abrir N bases ajenas, y eso rompe el aislamiento de 005. También se descarta una base global única para todos los datos de instincts, porque mezclaría observaciones de proyectos distintos.
- **Proyectos efímeros**: un proyecto cuya raíz está bajo el directorio temporal del sistema no se registra en `instinct_global`. Así un `mktemp -d` no cuenta como segundo proyecto en la promoción, aunque sí puede leer los instincts globales.

## R13. Evolve (hoja 6.6 del plan de entrada)

- **Decision**: se reutiliza `ports.SocraticGenerator.Rewrite` sin métodos nuevos. Se construye una `domain.RefinementFlow` en modo refine sin preguntas, cuyo original es la plantilla embebida `expand/evolve-prompt.md` con el instinct incrustado. Se elige el harness con `SocraticHarnessCatalog` y la misma interfaz de selección que `mem socratica`. El resultado se valida con la misma prueba de contrato que las skills (en tiempo de ejecución, mediante `domain.ValidateSkillBody`), se muestra y solo se escribe tras confirmar en un TTY con `console.Confirm`. Se instala con `InstallAgentSkill("instinct-<slug>", body)`.
- **Rationale**: no añade puertos y reutiliza los generadores de solo lectura ya endurecidos.
- **Fuera de alcance**: sin TTY, `evolve` falla con código 2 y nunca escribe.

## R14. Activación desde la instalación y en `mem update`

- **Decision**: `mem update` ya ejecuta `mem install <root>` (`cmd_update.go`, «Refrescando integración del proyecto»), y eso reescribe los hooks con `writeClaudeHooksSkipping`, `codexMissingHooks` y la plantilla del plugin de OpenCode. Al meter `expand` en `claudeHookEvents` (fijo, no en `claudeHookEventsFor`) y en `codexGomemoryHooks`, `update` lo cablea sin código nuevo. Además se sube el protocolo a `v10` (`domain.ProtocolVersionMarker`) para añadir un párrafo breve sobre `expand` (rules activas y skills de flujo disponibles). Así lo exige la memoria 460 cuando cambia el texto instalado.
- **Ámbito global**: `runGlobalScopeSetup` y `WriteClaudeHooksGlobal` consumen el mismo mapa, así que la cobertura es la misma.

## R15. Interruptor

- **Decision**: en `ports.SettingsData`, `ExpandDisabled bool \`json:"expand_disabled,omitempty"\`` y `ExpandOff []string \`json:"expand_off,omitempty"\`` (nombres de supertool). `mem expand off` → `ExpandDisabled=true`; `mem expand off rules` → añade `rules` a `ExpandOff`; `on` hace lo inverso. Los nombres válidos son `rules`, `shield`, `flow` e `instincts`.
- **Rationale**: es el mismo patrón que `PlanGuardDisabled` (lectura en `cmd_hook.go:1402`). Con `omitempty`, los `settings.json` existentes no cambian.
- **TUI**: fuera de alcance en 040, porque la spec no lo pide. `mem expand status` cubre la visibilidad.
- **Interruptor por supertool en el hook (analyze C2)**: `hook_expand.go` construye `ExpandSwitches{Disabled: s.ExpandDisabled, Off: set(s.ExpandOff)}` y se lo pasa al router. Con `off rules`, `rm -rf` se permite y la sugerencia de `verify-loop` se sigue emitiendo.

## R16. Puertos para el estado que escriben los casos de uso (analyze D1, §3)

- **Decision**: dos puertos nuevos en `application/ports/expand_state.go`:
  - `ObservationQueueRepository`: `Append(ctx, Observation) error`, `ReadBatch(ctx, max int) ([]Observation, error)`, `Remove(ctx, ids []string) error`, `Sanitize(ctx, redact func(string) string) (int, error)`. La cola NDJSON tiene un tope de 256 KiB con rotación, y cada línea lleva un `ID` UUIDv7 (R18).
  - `ShieldStateRepository`: `LastHash(ctx) string`, `Save(ctx, report ShieldReport) error`, `Last(ctx) (ShieldReport, bool)`.
  - `InstinctGlobalRepository` añade `Sanitize(ctx, redact func(string) string) (int, error)` sobre `instinct_global.description`. `mem sanitize` invoca los dos `Sanitize` además de su lector por columna de `mem.db` (3.ª pasada, K1, §20). Ningún otro almacén de 040 guarda texto de la persona: `hook_guard_events` solo guarda nombres de regla o skill, y el estado de shield ya está redactado.

  Ambos tienen un adaptador de archivo en `adapters/secondary/expandstate/`, con permisos `0600` y best-effort en lectura, y se construyen en `infrastructure/container.go`. En las pruebas se usan fakes en memoria.
- **Rationale**: `usecases/shield.go` y `usecases/instincts.go` quedan sin E/S directa, como exige §3. El adaptador queda probado aparte (tope, rotación, cola corrupta → vacía).
- **Alternatives**: hacer la E/S en el adaptador primario y pasar los datos ya leídos. Se rechaza porque el orden de truncado de D2 es lógica de aplicación y tiene que vivir en el caso de uso.

## R18. Cierres del 2.º /speckit-analyze (K1–K3, H1, H2, U1)

- **K3, nombres según §3**: los puertos nuevos siguen la regla sin tocar los existentes. Son repositorios `ObservationQueueRepository`, `ShieldStateRepository`, `InstinctRepository` e `InstinctGlobalRepository`, y servicios `RuleSourcePort` y `AgentConfigReaderPort`. Los puertos previos (`Compressor`, `SocraticGenerator`, …) quedan como deuda heredada fuera de alcance.
- **K2, `context.Context` (§1.2)**: toda la E/S nueva recibe `ctx` como primer parámetro. Esto incluye los puertos anteriores, `rulesfs.Loader.Load(ctx, root)`, `ShieldScan(ctx, …)`, `ExpandHook.Run(ctx, …)`, `InstinctsUseCase.Observe/FoldPending(ctx, …)` y `InstinctRepository.*(ctx, …)`. `hook_expand.go` crea `ctx, cancel := context.WithTimeout(context.Background(), domain.HookBudget)`, y todas las acciones lo respetan: SQLite con `QueryContext`/`ExecContext` y archivos comprobando `ctx.Err()` entre pasos. `withBudget` se mantiene como red de seguridad ante pánicos y llamadas que no respetan `ctx`.
- **K1, sin disparadores en producción (§11, §20)**: el pánico y el timeout se prueban a nivel unitario. `runHookExpand(ctx, deps hookExpandDeps, event, stdin, emit) hookRenderedOutput` es la función pura del adaptador, y `hookExpand` solo hace la E/S de proceso; sus dependencias inyectadas pueden entrar en pánico o bloquearse en las pruebas. No hay variables de entorno de prueba ni etiquetas de compilación en el binario.
- **H1, nunca aprobar en silencio**: en `pre-tool`, «allow + contexto» emite `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"…"}}` **sin** `permissionDecision`. En Claude Code, `permissionDecision:"allow"` se salta la confirmación de permisos, de modo que un aviso o una sugerencia aprobarían el comando. La única decisión explícita que emite el módulo es `deny`. Una prueba comprueba que la cadena `"allow"` no aparece nunca en la salida de `pre-tool` de ningún dialecto.
- **H2, plegado idempotente y acotado**:
  - Cada `Observation` lleva un `ID` UUIDv7 (`github.com/google/uuid`, ya presente).
  - `FoldPending` lee como mucho 500 observaciones (`ReadBatch`). Dentro de **una** transacción hace `INSERT OR IGNORE INTO instinct_folded(project, obs_id, folded_at)` y solo agrega las observaciones cuya inserción afectó una fila. Así, una observación ya plegada no vuelve a contar, aunque el proceso muera entre el commit y `Remove`.
  - Tras el commit, `Remove(ids)` es best-effort.
  - `instinct_folded` se purga a los 7 días (con `now` inyectado).
  - Si vence el `ctx`, la transacción se revierte entera, sin estado intermedio, y el siguiente evento continúa. Como el lote está acotado, el plegado avanza aunque haya mucha cola.
  - La latencia de `stop` y `session-start` se mide en T074 (C2).
- **U1, `rules --check`**: `mem expand rules --check [--event bash|file|prompt|stop]` lee la entrada por stdin (el diff, en el caso de `verify-loop`), la evalúa con las mismas reglas que el hook y devuelve el veredicto en texto o `--json`. Códigos de salida: 0 allow, 1 warn, 2 block. Lo usa `verify-loop`. Sin `--event`, el diff se evalúa como `file` (rutas de las cabeceras `+++`/`---` y el cuerpo como contenido).

## R17. Paquetes y puertos añadidos (analyze F5)

`adapters/secondary/rulesfs/` (cargador de reglas, puerto `ports.RuleSourcePort`), `adapters/secondary/agentconfig/` (puerto `ports.AgentConfigReaderPort`) y `adapters/secondary/expandstate/` (puertos de R16). Los tres son adaptadores secundarios de archivo, separados para poder probar en aislamiento las garantías de cada uno: topes, solo lectura y permisos.
