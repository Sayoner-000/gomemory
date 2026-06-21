---

description: "Task list for plugin-memory-context feature"

---

# Tasks: Plugin de Memoria con Contexto Automático

**Input**: Design documents from `specs/001-plugin-memory-context/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: TDD obligatorio por constitución. Cada fase incluye tests.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: repository root — Go project with `plugin/`, `internal/`, `store/`, etc.
- Paths follow the structure defined in `plan.md`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure for plugins

- [X] T001 Create `plugin/` directory structure with subdirectories for opencode and claude-code
- [X] T002 [P] Create `docs/PLUGINS.md` documentation for the plugin system
- [X] T003 [P] Create `docs/MEMORY-PROTOCOL.md` with Memory Protocol reference

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Implement HTTP server package in `internal/server/server.go` with:
  - `POST /session/start` — create session
  - `POST /session/end` — close session with summary
  - `GET /context` — get context payload for injection
  - `GET /health` — health check endpoint
  - Configurable port (default 9735), bind to 127.0.0.1 only
- [X] T005 [P] Create `cmd_serve.go` CLI command for `mem serve [port]` using `internal/server`
- [X] T006 [P] Create `cmd_setup.go` CLI command for `mem setup <agent>` dispatcher
- [X] T007 Implement `internal/setup/setup.go` with go:embed for plugin directories:
  - EmbeddedFS for `plugin/opencode/` and `plugin/claude-code/`
  - Idempotent file copy (skip existing)
  - Template placeholder replacement (project root, binary path, port)
- [X] T008 [P] Write unit tests for HTTP server in `internal/server/server_test.go`
- [X] T009 [P] Write unit tests for setup installer in `internal/setup/setup_test.go`

**Checkpoint**: Foundation ready — `mem serve` starts HTTP API, `mem setup` dispatches to agents

---

## Phase 3: User Story 1 — Plugin OpenCode (Priority: P1) 🎯 MVP

**Goal**: Plugin TypeScript para OpenCode que inyecta Memory Protocol, gestiona sesiones y proporciona contexto automático

**Independent Test**: Instalar plugin via `mem setup opencode`, iniciar OpenCode, verificar que el servidor HTTP se auto-inicia y el agente recibe contexto de sesiones previas

### Tests for User Story 1 ⚠️

- [X] T010 [P] [US1] Write contract test for plugin TypeScript API surface in `tests/contract/memory_protocol_test.go`
- [X] T011 [P] [US1] Write integration test for HTTP server session lifecycle in `tests/integration/plugin_integration_test.go`

### Implementation for User Story 1

- [X] T012 [P] [US1] Create `internal/setup/opencode_setup.go` — instala plugin.ts en `~/.config/opencode/plugins/` y configura MCP en `.opencode.json`
- [X] T013 [US1] Create `plugin/opencode/plugin.ts` with plugin activation entry point
- [X] T014 [US1] Implement auto-server start in plugin.ts — detect if `mem serve` runs on port 9735, start if not
- [X] T015 [US1] Implement session lifecycle in plugin.ts — `ensureSession()` via HTTP API, resilient to restarts
- [X] T016 [US1] Implement Memory Protocol injection in plugin.ts — `chat.system.transform` hook, concatenate protocol to existing system message
- [X] T017 [US1] Implement context injection in plugin.ts — fetch context from HTTP API on session start, inject recent sessions + memories
- [X] T018 [US1] Implement compaction recovery in plugin.ts — inject saved session context + recovery instructions on `compact.after` hook
- [X] T019 [US1] Implement context enrichment in plugin.ts — inject ToolSearch instruction on first prompt, passthrough on subsequent prompts

**Checkpoint**: OpenCode auto-recibe contexto de memoria, guarda eventos significativos, sobrevive compactaciones

---

## Phase 4: User Story 2 — Plugin Claude Code (Priority: P2)

**Goal**: Plugin con hooks nativos de Claude Code que gestiona sesiones, importa memorias git-sync, y proporciona skill de Memory Protocol

**Independent Test**: Instalar plugin via `mem setup claude-code`, iniciar Claude Code, verificar hooks se ejecutan y sesión se crea automáticamente

### Tests for User Story 2 ⚠️

- [ ] T020 [P] [US2] Write unit test for hook scripts execution — moved to `internal/setup/setup_test.go` (pending: test hook script syntax/execution)
- [X] T021 [P] [US2] Write integration test for Claude Code plugin file structure in `tests/integration/plugin_integration_test.go`

### Implementation for User Story 2

- [X] T022 [P] [US2] Create `internal/setup/claude_code_setup.go` — instala `.mcp.json`, hooks, scripts y skill
- [X] T023 [US2] Create `plugin/claude-code/.mcp.json` — MCP server config pointing to `mem mcp --root <project>`
- [X] T024 [US2] Create `plugin/claude-code/.claude-plugin/plugin.json` — plugin manifest
- [X] T025 [US2] Create `plugin/claude-code/hooks/hooks.json` — hook registrations for startup, compact, submit, shutdown
- [X] T026 [P] [US2] Create `plugin/claude-code/scripts/session-start.sh` — verifica servidor HTTP, crea sesión, importa git-sync, inyecta contexto
- [X] T027 [P] [US2] Create `plugin/claude-code/scripts/session-stop.sh` — cierra sesión activa via HTTP API
- [X] T028 [P] [US2] Create `plugin/claude-code/scripts/user-prompt-submit.sh` — inyecta ToolSearch en primer prompt, recordatorio opcional después
- [X] T029 [P] [US2] Create `plugin/claude-code/scripts/post-compaction.sh` — inyecta resumen de sesión previa + instrucción de recuperación
- [X] T030 [US2] Create `plugin/claude-code/skills/memory/SKILL.md` — Memory Protocol skill con reglas de guardado, búsqueda, cierre y recuperación

**Checkpoint**: Claude Code tiene sesiones automáticas, hooks funcionales y skill de memoria siempre disponible

---

## Phase 5: User Story 3 — Memory Protocol con Consumo Eficiente (Priority: P3)

**Goal**: Protocolo de memoria con reglas explícitas y revelación progresiva de 3 capas para minimizar tokens

**Independent Test**: Inyectar protocolo en cualquier agente MCP, el agente guarda solo eventos significativos y busca con revelación progresiva

### Tests for User Story 3 ⚠️

- [X] T031 [P] [US3] Write contract test for progressive disclosure pattern in `tests/contract/memory_protocol_test.go`

### Implementation for User Story 3

- [ ] T032 [US3] Implement progressive disclosure tools in MCP server `cmd_mcp.go`:
  - Layer 1: resúmenes compactos via `search_memories` (ID, título, tipo, score, ~100 tokens)
  - Layer 2: línea de tiempo via `timeline` tool (eventos antes/después de una memoria)
  - Layer 3: contenido completo via `get_memory` (solo cuando es necesario)
- [X] T033 [US3] Write Memory Protocol text content in `docs/MEMORY-PROTOCOL.md`: save triggers, search triggers, session close protocol, compaction recovery, progressive disclosure rules
- [X] T034 [US3] Implement context payload generator in `internal/server/server.go` — genera contexto de sesiones previas optimizado (<200 tokens)
- [ ] T035 [P] [US3] Write tests for context payload token budget — moved to `internal/server/server_test.go` (pending)
- [ ] T036 [P] [US3] Write tests for progressive disclosure flow — moved to `tests/integration/plugin_integration_test.go` (pending)

**Checkpoint**: Agentes usan memoria con consumo eficiente de tokens — solo buscan lo necesario, nada automático

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T037 [P] Update `cmd_mcp_setup.go` (`setup-mcp` command) to reference new plugin installers
- [ ] T038 [P] Update `README.md` with plugin setup instructions for OpenCode and Claude Code
- [X] T039 [P] Add `docs/PLUGINS.md` architecture documentation for the plugin system
- [ ] T040 [P] Update AGENTS.md with new plugin setup commands for gomemory protocol
- [ ] T041 Update CLAUDE.md with Memory Protocol instructions for Claude Code users
- [X] T042 Run integration tests against real HTTP server to verify session lifecycle (passed)
- [X] T043 Run full test suite and fix any regressions (all tests pass)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational completion — US1 is MVP
- **User Story 2 (Phase 4)**: Depends on Foundational completion — can run parallel to US1
- **User Story 3 (Phase 5)**: Depends on Foundational completion — can run parallel to US1 and US2
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) — Independent of US1
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) — May integrate with US1/US2 protocols

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Core infrastructure before agent-specific code
- Scripts/plugins before documentation

### Parallel Opportunities

- Setup Phase: T002 and T003 can run in parallel
- Foundational Phase: T005, T006, T008, T009 can run in parallel after T004
- User Story 1: T010-T012 + T014 can run in parallel; T014 is parallel; T015-T019 are sequential within plugin.ts
- User Story 2: T022 + T026-T029 can run in parallel
- User Story 3: T031 + T035 can run in parallel
- Polish Phase: T037-T041 can run in parallel

---

## Parallel Example: User Story 1

```bash
# All test tasks together:
Task: "Write contract test in tests/contract/memory_protocol_test.go"
Task: "Write integration test in tests/integration/plugin_integration_test.go"

# Setup installer + plugin.ts core:
Task: "Create opencode_setup.go in internal/setup/"
Task: "Create plugin.ts with activation entry point"
```

## Parallel Example: User Story 2

```bash
# Hook scripts (all independent, different files):
Task: "Create session-start.sh"
Task: "Create session-stop.sh"
Task: "Create user-prompt-submit.sh"
Task: "Create post-compaction.sh"

# Setup installer + config files:
Task: "Create claude_code_setup.go in internal/setup/"
Task: "Create .mcp.json, plugin.json, hooks.json"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (OpenCode plugin)
4. **STOP and VALIDATE**: `mem setup opencode` funciona, plugin inyecta contexto, sesiones persisten
5. Deploy/demo: OpenCode ya tiene memoria automática

### Incremental Delivery

1. Complete Setup + Foundational → `mem serve` + `mem setup` ready
2. Add User Story 1 (OpenCode plugin) → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 (Claude Code plugin) → Test independently → Deploy
4. Add User Story 3 (Memory Protocol) → Test independently → Deploy
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (OpenCode plugin)
   - Developer B: User Story 2 (Claude Code plugin)
   - Developer C: User Story 3 (Memory Protocol)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing (TDD per constitution)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence

---

## Progress Summary (2026-06-21)

| Phase | Total | Done | Pending | Progress |
|-------|-------|------|---------|----------|
| Phase 1: Setup | 3 | 3 | 0 | ✅ 100% |
| Phase 2: Foundational | 6 | 6 | 0 | ✅ 100% |
| Phase 3: US1 — OpenCode Plugin | 10 | 10 | 0 | ✅ 100% |
| Phase 4: US2 — Claude Code Plugin | 11 | 10 | 1 (T020) | 🔶 91% |
| Phase 5: US3 — Memory Protocol | 6 | 3 | 3 (T032, T035, T036) | 🔶 50% |
| Phase 6: Polish | 7 | 3 | 4 (T037, T038, T040, T041) | 🔶 43% |
| **Total** | **43** | **35** | **8** | **✅ 81%** |

### Pending Tasks

- **T020** — Write unit test for hook scripts (syntax/execution validation)
- **T032** — Progressive disclosure tools in `cmd_mcp.go` (Layer 2: timeline tool)
- **T035** — Tests for context payload token budget in `internal/server/server_test.go`
- **T036** — Tests for progressive disclosure flow in `tests/integration/plugin_integration_test.go`
- **T037** — Update `cmd_mcp_setup.go` to reference new plugin installers
- **T038** — Update `README.md` with plugin setup instructions
- **T040** — Update AGENTS.md with new plugin setup commands
- **T041** — Update CLAUDE.md with Memory Protocol instructions
