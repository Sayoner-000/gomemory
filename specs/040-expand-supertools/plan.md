# Implementation Plan: mem expand — supertools inspiradas en ECC

**Branch**: `main` (sin rama nueva) | **Date**: 2026-10-21 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/040-expand-supertools/spec.md`

## Summary

Un único subcomando de hook, `mem hook expand <event>`, queda cableado desde la instalación en Claude Code, Codex y OpenCode en los eventos session-start, prompt, pre-tool, post-tool y stop. En cada invocación llama una sola vez a un router de dominio puro (`domain.RouteSupertools`), que decide qué supertools actúan:

- **rules**: reglas `.gomemory/rules/*.md` más la regla de serie `destructive-shell`, evaluadas en Go.
- **shield**: escaneo de solo lectura de la configuración de los agentes; se lanza al iniciar sesión si cambia la huella y también con `mem doctor --shield`.
- **flow**: sugerencias de skill, una por episodio, más 5 skills reinterpretadas sobre gomemory.
- **instincts**: observación a una cola NDJSON, plegado por lotes en `instinct_stats` y en un registro global, e inyección en el contexto de los instincts de confianza ≥ 0.7.

Todo corre dentro del presupuesto existente `domain.HookBudget` (150 ms) y con fallo abierto. El estado del camino caliente vive en archivos de `.memory/` y no abre SQLite. La CLI `mem expand` informa y ofrece un interruptor global o por supertool. Además, el reconocimiento de hooks de gomemory pasa a un registro único (`domain.GomemoryHookSubcommands`), lo que cierra la brecha de hooks huérfanos que este cambio abriría.

## Technical Context

**Language/Version**: Go 1.27.x (toolchain go1.27.2). TypeScript del plugin OpenCode existente (`infrastructure/plugin/opencode/gomemory.ts`): se añade `tool.execute.before` y llamadas a `mem hook expand` en los ganchos existentes.

**Primary Dependencies**: las existentes (`modernc.org/sqlite`, stdlib `regexp` RE2). **Sin dependencias nuevas**, sin Node ni Python en tiempo de ejecución (memorias 612 y 617).

**Storage**:
- `mem.db` por proyecto: tabla aditiva `instinct_stats`.
- `DataHome()/instincts.db`: registro global `instinct_global`.
- Archivos de estado en `.memory/` (ignorado por git).
- Reglas en `.gomemory/rules/`, versionables.
- Ajustes en `.memory/settings.json`.

**Testing**: `go test -race`. Pruebas de tabla en `domain/`, contrato en `tests/contract/` (sincronía y contenido de las skills, registro de subcomandos), integración en `tests/integration/` (de `hook expand` a extremo con dialectos). Fixtures de shield anonimizados. Verificación real en los 3 harnesses (quickstart §3).

**Target Platform**: macOS, Linux y Windows.

**Project Type**: CLI + hooks de agente + servidor MCP (monolito Go hexagonal existente).

**Performance Goals**: `hook expand pre-tool` con p95 < 150 ms en 100 invocaciones con 64 reglas; `post-tool` < 20 ms (solo añade una línea a la cola).

**Constraints**: fallo abierto; sin escritura en configuraciones ajenas fuera de install/uninstall; secretos nunca mostrados ni persistidos; migraciones solo aditivas; neutralidad de harness (memoria 579); skills sin comandos exclusivos fuera de la tabla por harness (memoria 621).

**Scale/Scope**: 4 supertools, 1 subcomando de hook (5 eventos × 3 harnesses), 1 comando CLI con 11 subacciones (contracts/expand-cli.md), 1 sección de `doctor`, 5 skills, 3 tablas, 1 regla de serie.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Regla | Estado | Evidencia |
|---|---|---|
| §3 Hexagonal: dominio sin infraestructura | PASS | `domain/expand.go`, `domain/rules.go`, `domain/shield.go` y `domain/instinct.go` son puros. Los casos de uso `usecases/expand_hook.go`, `shield.go` e `instincts.go` dependen de puertos (`AgentConfigReaderPort`, `RuleSourcePort`, `ObservationQueueRepository`, `ShieldStateRepository`, `InstinctRepository`, `InstinctGlobalRepository`, `SettingsRepository`, `HookGuardRecorder`). No hacen E/S directa (analyze D1, research R16). La CLI y los hooks solo adaptan E/S; los marcadores de episodio y contador son estado del adaptador primario (patrón `plan_episode.go`). |
| §4 Composition root único | PASS | Los nuevos repositorios se construyen en `infrastructure/container.go` y se exponen en `Deps`. |
| §5 SQL parametrizado, sin `SELECT *`, transacciones en caso de uso | PASS | El plegado de instincts es una transacción por lote con `INSERT … ON CONFLICT` parametrizado. La cola se trunca tras el commit y el registro global se reconcilia por estado, de forma idempotente (analyze D2, R12). No hay transacciones que abarquen E/S externa. |
| §12 Observabilidad | PASS | `expand.block`, `warn`, `suggest`, `shield`, `timeout` y `error` en `hook_guard_events`, sin contenido del usuario, visibles en `mem expand status`. |
| §6 Migraciones aditivas | PASS | `CREATE TABLE IF NOT EXISTS instinct_stats` en `migrate()`; `instincts.db` tiene su propia apertura idempotente. |
| §9 Configuración tipada y validada | PASS | `ExpandDisabled`, `ExpandOff` y `ExpandCompactThreshold` en `SettingsData`. Un umbral fuera de rango usa el valor por defecto; un nombre de supertool inválido devuelve el código 2 en la CLI. |
| §1.2 Go: `context.Context` en toda E/S | PASS | Todos los puertos y casos de uso nuevos reciben `ctx`; el hook lo crea con `context.WithTimeout(domain.HookBudget)` (2.º analyze K2, R18). |
| §3 Nombres de puertos (`{Name}Repository` / `{Name}Port`) | PASS | Puertos nuevos: `ObservationQueueRepository`, `ShieldStateRepository`, `InstinctRepository`, `InstinctGlobalRepository`, `RuleSourcePort`, `AgentConfigReaderPort` (K3). Los puertos previos que no siguen la regla son deuda heredada fuera de alcance. |
| §11/§20 Sin hooks de depuración en producción | PASS | Pánico y timeout se prueban con dependencias inyectadas en `runHookExpand`; nada activable por entorno en el binario (K1). |
| §10 Pruebas deterministas; reloj inyectado; cobertura ≥ 80 % | PASS | Router y reglas puros. `InstinctConfidence`, el plegado, la purga de marcadores y la ventana de activaciones reciben `now` (analyze D3). `scripts/coverage.sh` en la tarea de cierre. |
| §11 Fakes para puertos | PASS | Fakes de `AgentConfigReaderPort`, `InstinctRepository` y `SocraticGenerator` en `*_fakes_test.go`. |
| §15 Seguridad de aplicación | PASS | Las reglas son RE2 (sin ReDoS) con topes. Shield es de solo lectura y redacta. La cola de instincts aplica `RedactSecrets` y la limpieza de `<private>`. Los archivos de estado tienen permisos `0600`. |
| §15/§20 Sin dependencias nuevas | PASS | Ninguna. |
| §18 Documentación | PASS | `docs/expand.md` (nuevo), `docs/DATABASE.md` (3 tablas: `instinct_stats`, `instinct_folded`, `instinct_global`), `docs/ARQUITECTURA.md` (router y puertos), README y CHANGELOG. Documentación en español; identificadores en inglés. |
| §20 Datos personales sin control | PASS | `mem sanitize` cubre `instinct_stats.description` (lector por columna, memorias 510 y 599), `instinct_global.description` en `DataHome()/instincts.db` mediante `InstinctGlobalRepository.Sanitize(ctx, …)` y el campo `sample` de la cola de observaciones mediante `ObservationQueueRepository.Sanitize(ctx, …)`. La cola es además transitoria: cada observación se borra al plegarse (3.ª pasada de analyze, K1). |
| Licencias de terceros | PASS | Las ideas de ECC (MIT) se reinterpretan sin copiar texto. `NOTICE` con la atribución. |

Sin violaciones; *Complexity Tracking* vacío.

**Re-check post-diseño**: PASS. El diseño no añade capas ni dependencias, y el único puerto nuevo con efectos fuera de la base de datos (`AgentConfigReaderPort`) es de solo lectura.

## Project Structure

### Documentation (this feature)

```text
specs/040-expand-supertools/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── router.md
│   ├── hook-expand.md
│   ├── expand-cli.md
│   ├── rules-format.md
│   ├── shield.md
│   └── skills.md
├── checklists/requirements.md
└── tasks.md            # /speckit-tasks
```

### Source Code (repository root)

```text
domain/
├── expand.go                  # Supertools, ExpandEvent, ExpandSignals, SupertoolAction, RouteSupertools, ClassifyTool, IsCommitCommand, MatchesDependencyIntent
├── rules.go                   # Rule, RuleVerdict, EvaluateRules, ParseRule, BuiltinRules (destructive-shell, lexer de shell)
├── shield.go                  # ShieldFinding, ShieldReport, reglas de detección
├── shell_lexer.go             # SplitShellCommands (compartido por IsCommitCommand y destructive-shell)
├── redact.go                  # + FindSecrets (reutiliza secretPatterns sin devolver el valor)
├── instinct.go                # Instinct, InstinctConfidence, ObservationKey, correcciones
├── hook_subcommands.go        # GomemoryHookSubcommands (registro único, R2)
├── skill_contract.go          # ValidateSkillBody (usado por evolve y por la prueba de contrato)
├── mcp_tools.go               # + AllMCPTools()
├── protocol.go                # ProtocolVersionMarker → v10
└── *_test.go
application/
├── ports/agent_config_reader.go
├── ports/rule_source.go
├── ports/expand_state.go             # ObservationQueueRepository + ShieldStateRepository (R16)
├── ports/instinct_repository.go      # InstinctRepository + InstinctGlobalRepository
├── ports/settings_repository.go      # + ExpandDisabled, ExpandOff, ExpandCompactThreshold
└── usecases/
    ├── expand_hook.go                # ejecuta las acciones del router → ExpandOutcome
    ├── shield.go
    └── instincts.go                  # observe (cola), fold, list, confirm, reject, evolve
adapters/
├── primary/cli/
│   ├── hook_expand.go                # mem hook expand; estado en archivos (.memory/), withBudget
│   ├── hook_dialect.go               # + renderExpandOutput (no toca renderGuardDecision)
│   ├── hook_expand_state.go          # marcadores de episodio y contador por sesión (estado del adaptador)
│   ├── cmd_expand.go                 # mem expand … (E/S inyectable)
│   ├── cmd_doctor.go                 # + --shield
│   ├── cmd_hook.go                   # + case "expand"
│   ├── cmd_mcp_setup.go              # + InstallExpandSkills en runGlobalScopeSetup
│   └── cli.go                        # helpIndex + ayuda detallada
├── primary/setup/
│   ├── claude_code_setup.go          # claudeHookEvents + expand; hookCommandIsGomemory sobre el registro
│   ├── codex_setup.go                # codexGomemoryHooks + expand (PreToolUse `Bash|apply_patch|collaborationspawn_agent`, verificado en la sonda T004)
│   └── agent_skills_setup.go         # InstallExpandSkills
└── secondary/
    ├── persistence/db.go             # migrate(): instinct_stats
    ├── persistence/instincts.go      # InstinctRepository + registro global instincts.db
    ├── persistence/privacy_maintenance.go  # sanitize: instinct_stats.description
    ├── agentconfig/reader.go         # AgentConfigReaderPort (solo lectura, topes)
    ├── rulesfs/loader.go             # RuleSourcePort (topes, saneamiento, caché por mtime)
    └── expandstate/{queue,shield}.go # ObservationQueueRepository + ShieldStateRepository (archivos 0600)
application/usecases/build_context.go  # + sección «## Instincts»
application/usecases/session_delta.go  # + encabezado de sección
infrastructure/
├── container.go
├── plugin/opencode/gomemory.ts      # tool.execute.before/after, chat.message, session.* → mem hook expand
└── templates/expand/{NOTICE,evolve-prompt.md,<5 skills>/SKILL.md}
skills/{verify-loop,search-first,iterative-retrieval,strategic-compact,instincts}/SKILL.md
tests/
├── contract/expand_skills_contract_test.go
├── contract/skill_sources_sync_test.go           # ampliado a expand/*
├── contract/hook_subcommands_registry_test.go
└── integration/hook_expand_test.go
docs/expand.md, docs/DATABASE.md, docs/ARQUITECTURA.md, README.md, CHANGELOG.md
```

**Structure Decision**: se usa el monolito hexagonal existente. Se añaden tres adaptadores secundarios de archivo (research R17):
- `agentconfig`: único lector de configuración de terceros; aislado para probar S1 (solo lectura).
- `rulesfs`: topes y caché de reglas.
- `expandstate`: cola y estado de shield, detrás de los puertos de R16.

No hay más paquetes nuevos.

## Riesgos y cierre

| Riesgo | Mitigación | Verificación |
|---|---|---|
| El arranque de `Deps` abre SQLite y rompe el p95 | ruta ligera para `hook expand pre-tool\|prompt` sin base de datos (R5) | quickstart §5 |
| Codex no respeta la denegación en `PreToolUse`, o `apply_patch` no dispara | sonda real previa (T004) con `CODEX_HOME` aislado que fija el matcher definitivo; si algo no funciona, se declara una excepción con evidencia | `evidence/codex-probe.md` y quickstart §3 |
| Falsos positivos de `destructive-shell` | evaluación por tokens, tabla allow/block en el contrato, anulación por regla propia | `rules-format.md` como prueba de tabla |
| Hooks huérfanos o duplicados al desinstalar o actualizar | registro único y prueba que recorre los 3 instaladores | quickstart §7 |
| Fuga de secretos por shield o instincts | `FindSecrets` sin valor, `RedactSecrets` en la cola, prueba de subcadena | `contracts/shield.md` S2 |
| Ruido de contexto | sugerencias una por episodio, tope de 600 caracteres, silencio ante la duda | prueba G4 |
| Instincts de baja calidad inyectados | solo los de confianza ≥ 0.7 y no rechazados; rechazo definitivo | quickstart §10 |

## Fases y MVP

- **MVP (P1)**: router + `hook expand` + registro de subcomandos + cableado en los 3 harnesses + protocolo v10 + `mem expand status|off|on` + rules con `destructive-shell`. Corresponde a US1 y US2.
- **P2**: shield (US4) y flow-skills con sugerencias (US3). Son independientes entre sí.
- **P3**: instincts (US5).
- **P4**: evolve (US6).
- **Cierre**: `go test -race ./...`, CI, verificación real en los 3 harnesses, latencia, interruptor, `doctor --shield` en la máquina de referencia y documentación.

## Complexity Tracking

Sin violaciones de la constitución.
