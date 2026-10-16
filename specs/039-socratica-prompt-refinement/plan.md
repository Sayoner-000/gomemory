# Implementation Plan: Socratica — refinamiento de prompts y escrutinio socrático

**Branch**: `main` (sin rama nueva) | **Date**: 2026-10-15 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/039-socratica-prompt-refinement/spec.md`

## Summary

Habilidad canónica `socratica`, con los modos `refine` (`/socratica`) y `scrutiny` (`/socratic-q`), distribuida a Claude Code, Codex y OpenCode por el canal existente `InstallAgentSkill`. El núcleo de dominio es puro: un reductor de estados, la decisión de intercepción y el parser de generación. Se apoya en dos tablas nuevas en `mem.db` y en una única herramienta MCP, `socratica_flow`, que hace cumplir la compuerta de aprobación y el despacho sin duplicados. Ofrece CLI `mem socratica` y una pantalla en la TUI de `mem`. Fuera de un agente, la generación se delega en Claude Code (sin herramientas) o Codex (sandbox de solo lectura); OpenCode no se usa como generador hasta verificar un modo de solo lectura. La intercepción automática es opt-in por agente y se hace inyectando contexto en los hooks ya instalados; solo se habilita tras verificarla con evidencia. No hay paneles nativos del anfitrión en esta iteración (research R10).

## Technical Context

**Language/Version**: Go 1.27.x (toolchain go1.27.2); TypeScript del plugin OpenCode existente (`infrastructure/plugin/opencode/gomemory.ts`), solo un cambio en `chat.message`.

**Primary Dependencies**: las existentes. `charm.land/bubbletea/v2` y `lipgloss/v2` (TUI), `modernc.org/sqlite`, `github.com/google/uuid` (UUIDv7). **Sin dependencias nuevas.**

**Storage**: `mem.db` (SQLite) por proyecto. Se crean las tablas `socratica_flows` y `socratica_history` mediante la migración aditiva de `migrate()`. Los ajustes van en `.memory/settings.json`.

**Testing**: `go test`. Las unitarias van junto al código; integración y contrato en `tests/`. El repositorio se prueba contra SQLite real. Generadores y hooks se sustituyen por fakes, sin llamadas de pago.

**Target Platform**: macOS, Linux y Windows, las mismas plataformas que el binario `mem`.

**Project Type**: CLI + servidor MCP + TUI + plugin de agente (monolito Go hexagonal existente).

**Performance Goals**: las operaciones de estado (`socratica_flow`, `history`) tardan menos de 50 ms en local. El hook de intercepción añade menos de 20 ms al turno y no consulta la base cuando la intercepción está desactivada.

**Constraints**: cero ejecución de la tarea antes de aprobar; cero pérdida del original; nada de `<private>` persistido; sin credenciales nuevas; render puro; migración solo aditiva.

**Scale/Scope**: 3 agentes, 1 habilidad, 3 envoltorios, 1 herramienta MCP, 7 subcomandos de CLI, 3 vistas de TUI (lista, detalle/revisión, preguntas) y 2 tablas.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Regla | Estado | Evidencia |
|---|---|---|
| §3 Hexagonal: dominio sin infraestructura; sin lógica en handlers | PASS | `domain/socratica*.go` puro; puertos `SocraticaRepository`, `SocraticaUnitOfWork`, `SocraticGenerator` y `AgentLauncher` en `application/ports`; Ejecutar lo orquesta `SocraticaFlow.Execute`, y la CLI y la TUI solo lo invocan (corrección C2 del 2.º análisis) |
| §4 Composition root único | PASS (nombre heredado) | el composition root del repositorio es `infrastructure/container.go`, no `composition.go`; es una convención previa a esta feature y no se relaja nada nuevo. |
| §4 (detalle) | PASS | `infrastructure/container.go` construye la lista ordenada de generadores concretos según `socratica_generators` y la inyecta en el adaptador compuesto `generator.FirstAvailable`, que no conoce los adaptadores concretos (corrección C1 de /speckit-analyze) |
| §5 SQL parametrizado, sin `SELECT *`, transacciones en caso de uso | PASS | `SocraticaUnitOfWork.WithTransaction` engloba cada evento (transición + historial/poda o borrado) en una sola transacción con rollback completo (corrección C1 del 2.º análisis); `approve` atómico con `UPDATE … WHERE phase IN ('review','failed')` (`failed` solo por `send_original`); reclamo exclusivo `ClaimDispatch` antes de lanzar, para que dos Execute concurrentes produzcan un solo arranque; **ninguna transacción abarca un proceso externo ni el generador**; toda consulta con `context.WithTimeout` según `socratica_query_timeout_ms`: Execute = approve (tx) → ClaimDispatch → Launch bloqueante con ack (tx) dentro de `onStarted`, solo si arrancó |
| §6 Migraciones | PASS (convención del runner vigente) | `migrate()` aditivo (`db.go:161`); no se mezclan archivos `V00N__` |
| §9 Configuración tipada, validada al inicio, falla rápida | PASS | ajustes `socratica_*` tipados en `SettingsData`; `ErrInvalidSocraticaSettings` con la clave y el valor en CLI/TUI/MCP; los hooks no bloquean el turno y desactivan la intercepción |
| §10 Pruebas deterministas; reloj y UUID inyectados; repositorio contra motor real; cobertura ≥ 80 % | PASS | `Reduce(…, now, newID)`; pruebas de persistencia con SQLite real; `scripts/coverage.sh` (umbral 80 %, ya en CI) dentro de T070 |
| §11 Fakes para puertos externos | PASS | fakes de `SocraticGenerator`, `AgentLauncher` y `SocraticaRepository` en archivos `*_fakes_test.go` del paquete que los usa (convención vigente del repositorio, permitida por §11 como «módulo de soporte de pruebas»); sin mocks en producción |
| §15/§20 Sin dependencias nuevas | PASS | ninguna |
| §18 Documentación en español, identificadores en inglés; docs obligatorios | PASS | T067/T068 actualizan `docs/socratica.md`, `docs/DATABASE.md` (2 tablas) y `docs/ARQUITECTURA.md` (ports y flujo) |
| §20 Registrar datos personales sin control | PASS | `ProtectText` al escribir y al leer; modo privado (borra el flujo al terminar); `mem sanitize` con lector propio `readSocraticaCandidates` y una categoría por columna (corrección U3) |

Sin violaciones; *Complexity Tracking* vacío.

**Re-check post-diseño**: PASS. El diseño de Phase 1 no introduce capas ni dependencias nuevas. La única superficie MCP es una herramienta con `action`, como exige FR-031.

## Project Structure

### Documentation (this feature)

```text
specs/039-socratica-prompt-refinement/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli.md
│   ├── mcp-tool.md
│   ├── skill.md
│   ├── generation.schema.json
│   ├── compatibility-matrix.md
│   └── tui.md
├── checklists/requirements.md
└── tasks.md            # /speckit-tasks
```

### Source Code (repository root)

```text
domain/
├── socratica.go                    # tipos, fases, Reduce, Effect, errores
├── socratica_interception.go       # DecideInterception (pura)
├── socratica_generation.go         # ParseGeneration + reparación, WordCount
├── socratica_generation.schema.json # esquema embebido (copia de contracts/generation.schema.json)
├── socratica_capabilities.go       # tabla SocraticaCapabilities por agente
└── *_test.go
application/
├── ports/socratica_repository.go
├── ports/socratic_generator.go
├── ports/agent_launcher.go
├── ports/socratica_unit_of_work.go
└── usecases/socratica_flow.go      # orquesta Reduce + repo + efectos
adapters/
├── secondary/persistence/socratica.go          # repo SQLite + ProtectText ida/vuelta
├── secondary/persistence/db.go                 # +2 tablas e índices (aditivo)
├── secondary/persistence/socratica_sanitize.go # readSocraticaCandidates + Apply (no ledgerColumns)
├── secondary/persistence/privacy_maintenance.go# invoca readSocraticaCandidates en Candidates/Apply
├── secondary/launcher/{exec,print}.go          # AgentLauncher
├── secondary/persistence/settings.go           # +claves socratica_*
├── secondary/generator/{claude,codex}.go       # SocraticGenerator por subproceso
├── secondary/generator/first_available.go     # compuesto; recibe la lista del composition root
├── primary/cli/cmd_socratica.go                # mem socratica …
├── primary/cli/cmd_mcp_socratica_tool.go       # socratica_flow
├── primary/cli/cmd_mcp_schemas.go              # esquema de socratica_flow
├── primary/cli/cmd_mcp_setup.go                # llama a InstallSocraticaSkill
├── primary/cli/cmd_hook.go / hook_dialect.go   # rama de intercepción en user-prompt-submit
├── primary/cli/dispatcher.go                   # registro del subcomando
├── primary/setup/agent_skills_setup.go         # InstallSocraticaSkill + envoltorios
├── primary/setup/global_artifacts.go           # socratica + envoltorios en desinstalación
├── primary/tui/socratica.go                    # screenSocratica/Detail/Preguntas, estilos en styles.go
└── primary/tui/{tui.go,tui_docs.go}            # menú, navegación y ayuda
application/ports/settings_repository.go        # +campos SettingsData
infrastructure/
├── templates/socratica/SKILL.md                # FUENTE CANÓNICA
├── templates/socratica/commands/*.md           # envoltorios finos
├── plugin/opencode/gomemory.ts                 # chat.message: intercepción
└── container.go (composition root: wiring del caso de uso y lista ordenada de generadores)
skills/socratica/{SKILL.md,README.md}           # copia publicada (igual byte a byte)
tests/contract/skill_sources_sync_test.go       # skills/* == templates/* (las 3 habilidades)
docs/socratica.md + README.md                   # on/off/history/aprobación/límites/privacidad/tokens
docs/DATABASE.md, docs/ARQUITECTURA.md          # tablas, ports y flujo
```

**Structure Decision**: se sigue la estructura hexagonal ya vigente del repositorio (`domain`, `application`, `adapters`, `infrastructure`) y los patrones de la revisión adversarial (herramienta MCP con ledger propio) y de `defensive-qa-audit` (distribución de habilidad).

## Árbol de tareas atómicas

🎯 Socratica disponible y verificada en Claude Code, Codex y OpenCode, en la CLI y en la TUI; `go test ./...` en verde y la matriz de compatibilidad respaldada por evidencia.

```text
├─ [1] Fuente única de habilidades
│  ├─ [1.1] ✓ Escribir prueba de contrato skills/*/SKILL.md == templates/*/SKILL.md → falla con socratica ausente
│  ├─ [1.2] ✓ Redactar infrastructure/templates/socratica/SKILL.md según contracts/skill.md → archivo con 12 secciones   (dep: 1.1)
│  ├─ [1.3] ✓ Copiar a skills/socratica/SKILL.md + README.md → prueba 1.1 en verde   (dep: 1.2)
│  └─ [1.4] ✓ Escribir prueba de contenido (frontmatter name=socratica, secciones 1–12 presentes) → verde   (dep: 1.2)
├─ [2] Dominio puro   (∥ con 1)
│  ├─ [2.1] ✓ Escribir tabla de pruebas de Reduce según data-model (todas las transiciones + inválidas) → rojo
│  ├─ [2.2] ✓ Implementar Reduce/Effect en domain/socratica.go → 2.1 verde   (dep: 2.1)
│  ├─ [2.3] ✓ Escribir tabla de pruebas de DecideInterception (9 casos de quickstart §1) → rojo
│  ├─ [2.4] ✓ Implementar DecideInterception → 2.3 verde   (dep: 2.3)
│  ├─ [2.5] ✓ Escribir pruebas de ParseGeneration (0/1–3/>3, opciones, JSON roto) → rojo
│  ├─ [2.6] ✓ Implementar ParseGeneration y WordCount → 2.5 verde   (dep: 2.5)
│  └─ [2.7] ✓ Declarar SocraticaCapabilities con todo false salvo lo ya verificado → prueba que on falla si false
├─ [3] Persistencia y ajustes   (dep: 2.2)
│  ├─ [3.1] ✓ Añadir tablas e índices a migrate() → prueba abre base previa y nueva sin error
│  ├─ [3.2] ✓ Escribir pruebas de repo (tope 20, approve concurrente, private ida/vuelta) → rojo   (dep: 3.1)
│  ├─ [3.3] ✓ Implementar SocraticaRepository SQLite → 3.2 verde   (dep: 3.2)
│  ├─ [3.4] ✓ Implementar readSocraticaCandidates/Apply con categorías por columna → prueba sanitize depura flujo e historial   (dep: 3.1)
│  └─ [3.5] ✓ Añadir claves socratica_* a SettingsData y Settings → prueba round-trip no pierde campos ajenos
├─ [4] Caso de uso y MCP   (dep: 3.3, 2.6)
│  ├─ [4.1] ✓ Implementar usecases.SocraticaFlow (Reduce + repo + historial + poda pendientes) → pruebas con repo falso verdes
│  ├─ [4.2] ✓ Añadir esquema y handler socratica_flow → prueba approve duplicado devuelve duplicate=true   (dep: 4.1)
│  ├─ [4.3] ✓ Probar get sobre approved sin ack → review recovered=true   (dep: 4.2)
│  └─ [4.4] ✓ Conectar PurgeStale a mem gc → prueba purga no terminales > 7 días   (dep: 4.1)
├─ [5] Distribución   (dep: 1.3)
│  ├─ [5.1] ✓ Implementar InstallSocraticaSkill + envoltorios y llamarlo en runGlobalScopeSetup → segunda ejecución 0 escrituras
│  ├─ [5.2] ✓ Añadir socratica y envoltorios a globalSkillNames/GlobalGeneratedArtifacts → TestUninstall_SistemaSinRastros verde   (dep: 5.1)
│  └─ [5.3] ✓ Verificar que envoltorios no contienen el cuerpo de la habilidad → prueba verde   (dep: 5.1)
├─ [6] Generadores no interactivos   (dep: 2.6)
│  ├─ [6.1] ✓ Implementar adaptador claude (-p --json-schema --tools "") → prueba con ejecutable falso en PATH aislado
│  ├─ [6.2] ✓ Implementar adaptador codex (exec --ephemeral --sandbox read-only --output-schema) → ídem
│  ├─ [6.3] ✓ Implementar FirstAvailable y construir la lista en container.go → prueba sin agentes = ErrNoGenerator (código 2)
│  └─ [6.4] ✓ Implementar AgentLauncher (Exec/Print) y SocraticaFlow.Execute → prueba: sin ack si Launch falla
├─ [7] CLI   (dep: 4.1, 6.4)
│  ├─ [7.1] ✓ Implementar mem socratica refine (TTY y no-TTY, Ejecutar/--print, Copiar OSC52) → e2e con generador falso
│  ├─ [7.2] ✓ Implementar history/status/pending/private → e2e tope 20 y cancel repite original
│  └─ [7.3] ✓ Implementar on/off por agente con guarda de capacidad → e2e no altera otro agente
├─ [8] Intercepción   (dep: 2.4, 3.5)
│  ├─ [8.1] ✓ Añadir rama Socratica en user-prompt-submit (sin leer DB si todo off) → prueba dialectos Claude/Codex
│  ├─ [8.2] ✓ Añadir intercepción en chat.message del plugin OpenCode (strip '>' en la parte) → prueba del plugin
│  └─ [8.3] ✓ Reconocer GOMEMORY_SOCRATICA_DISPATCH como origen aprobado → prueba sin recaptura tras Ejecutar desde CLI
├─ [9] TUI   (dep: 4.1, 6.4)
│  ├─ [9.1] ✓ Añadir tokens soc* a styles.go → prueba de identidad visual
│  ├─ [9.2] ✓ Implementar screenSocratica/Detail/Preguntas → pruebas render 59/80/120 columnas   (dep: 9.1)
│  └─ [9.3] ✓ Probar pureza de View() con repo falso → verde   (dep: 9.2)
├─ [10] Verificación por agente   (dep: 5, 7, 8)
│  ├─ [10.1] ✓ Ejecutar quickstart §6 en Claude Code → evidence/claude.md
│  ├─ [10.2] ✓ Ejecutar quickstart §6 en Codex → evidence/codex.md
│  ├─ [10.3] ✓ Ejecutar quickstart §6 en OpenCode → evidence/opencode.md
│  └─ [10.4] ✓ Actualizar SocraticaCapabilities y la matriz solo con celdas evidenciadas   (dep: 10.1–10.3)
│  └─ [10.5] ✓ Verificar si OpenCode admite un agente de solo lectura (--agent) → evidencia; habilitar adaptador solo si existe
├─ [11] Documentación   (dep: 10.4)
│  ├─ [11.1] ✓ Redactar docs/socratica.md (on/off/history/aprobación/límites/privacidad/tokens/duplicados) → archivo
│  └─ [11.2] ✓ Enlazar desde README.md → enlace presente
└─ [12] Cierre
   ├─ [12.1] ✓ Ejecutar go test ./... -count=1 → verde
   └─ [12.2] ⚠ no atómica → skills-ref validate: binario no instalado; instalarlo es decisión de la persona (registrar "no ejecutado" si sigue ausente)
```

45 hojas, por encima del umbral de 25 del método. Priorización propuesta: el MVP es 1 + 2 + 3 + 4 + 5 (habilidad con compuerta en los tres agentes, US1–US3). Después vienen 6–7 (CLI, US4), 9 (TUI, US5) y 8 (intercepción, US6). 10–12 cierran.

## Riesgos y cierre

| Riesgo | Cierre |
|---|---|
| Un modelo ignora la compuerta y ejecuta antes de aprobar | La habilidad lo prohíbe, `socratica_flow` es la única vía para obtener `dispatch_id` y la aceptación §6.1 comprueba `git status` limpio. Si un agente lo incumple, la matriz lo registra |
| La intercepción no puede detectar adjuntos o imágenes en Claude/Codex | En caso de duda, deja pasar; la capacidad no se habilita sin evidencia (10.4) |
| Deriva entre `skills/` y `templates/` (ya latente) | Prueba 1.1, que también cubre las habilidades existentes |
| Recaptura del prompt aprobado al lanzarlo desde CLI/TUI | `GOMEMORY_SOCRATICA_DISPATCH` en el entorno del agente lanzado (8.3) |
| Generador que ejecuta la tarea | solo Claude sin herramientas y Codex en solo lectura; OpenCode excluido sin evidencia (10.5 / T066) |
| Pérdida de ajustes al escribir | Campos en `SettingsData` y en `Settings`, con prueba de round-trip (3.5) |
| Prompts de Socratica adjuntos como origen de memorias generales (hook existente `SetLastPrompt`) | `ClassifyPrompt.SkipProvenance` antes de `SetLastPrompt` en `user-prompt-submit` y `mem hook prompt` (T024/T028) |
| Retención indefinida de flujos terminales | poda a `history_limit` por (project, client) y `PurgeTerminal` de más de 24 h en `mem gc` |
| Fuga de `<private>` en lecturas | `ProtectText` al leer y al escribir, más cobertura de sanitize (3.3, 3.4) |

## Complexity Tracking

Sin violaciones constitucionales que justificar.
