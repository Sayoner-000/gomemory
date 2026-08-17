# Tasks: codebase-memory-mcp activation regression tests

**Input**: Design documents from `/specs/014-codebase-memory-activation-tests/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: No additional unit/integration tests — the script IS the test. Tasks focus on formalizing the existing implementation against the spec.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US6)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Verify existing script matches spec requirements

- [ ] T001 Verify `scripts/test-codebase-memory-activation.sh` has executable permission (`chmod +x`)
- [ ] T002 Verify script header comment matches spec description (line 2-6 of script)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Ensure helpers and constants match data-model.md definitions

- [ ] T003 Verify TOOLS array in script matches `CodebaseMemoryMCPDiscoveryTools` from `domain/mcp_tools.go` (6 tools)
- [ ] T004 Verify ADMIN_TOOLS array in script matches exclusion list from `contracts/hook-schemas.md` (4 tools)
- [ ] T005 Verify gomemory_tools array in `test_bootstrap_completeness` matches `domain.MCPAllTools()` count (15 tools)

**Checkpoint**: Constants verified — user story checks can proceed

---

## Phase 3: User Story 1 - Ejecutar regresión completa (Priority: P1) 🎯 MVP

**Goal**: Un solo comando verifica los 4 canales y reporta pasaron/fallaron

**Independent Test**: Ejecutar `./scripts/test-codebase-memory-activation.sh` y verificar exit code 0 con 50 checks

### Implementation for User Story 1

- [ ] T006 [P] [US1] Verify `build_binary()` function compiles successfully and creates temp binary in `scripts/test-codebase-memory-activation.sh:55-62`
- [ ] T007 [P] [US1] Verify `main()` function calls all 7 test sections in order in `scripts/test-codebase-memory-activation.sh:234-275`
- [ ] T008 [US1] Verify exit code logic: exit 1 when FAIL > 0, exit 0 when FAIL == 0 in `scripts/test-codebase-memory-activation.sh:261-267`
- [ ] T009 [US1] Run full script end-to-end and verify 50 checks pass with exit code 0

**Checkpoint**: Script compila, ejecuta, y reporta correctamente — MVP funcional

---

## Phase 4: User Story 2 - Validar canal Claude Code hook (Priority: P1)

**Goal**: Hook `user-prompt-submit` entrega tools en `additionalContext`, NO en `systemMessage`

**Independent Test**: Ejecutar sección 2 del script y verificar checks de additionalContext y systemMessage

### Implementation for User Story 2

- [ ] T010 [P] [US2] Verify `test_claude_code_hook()` creates temp dir with `.memory/` and runs `session-start` in `scripts/test-codebase-memory-activation.sh:66-105`
- [ ] T011 [P] [US2] Verify `assert_not_contains` check for `systemMessage` exists (regression v2.3.3) in `scripts/test-codebase-memory-activation.sh:86`
- [ ] T012 [P] [US2] Verify loop over TOOLS array checks all 6 `mcp__codebase-memory-mcp__` tools in `scripts/test-codebase-memory-activation.sh:92-94`
- [ ] T013 [P] [US2] Verify loop over ADMIN_TOOLS checks absence of 4 admin tools in `scripts/test-codebase-memory-activation.sh:97-99`
- [ ] T014 [US2] Verify `get_plan_context` instruction presence check in `scripts/test-codebase-memory-activation.sh:102`

**Checkpoint**: Canal Claude Code verificado — 14 checks cubiertos

---

## Phase 5: User Story 3 - Validar canal subagentes (Priority: P2)

**Goal**: Hook `subagent-start` entrega bootstrap con codebase-memory-mcp en additionalContext

**Independent Test**: Ejecutar sección 3 del script

### Implementation for User Story 3

- [ ] T015 [P] [US3] Verify `test_claude_code_subagent()` runs `subagent-start` hook without requiring prior `session-start` in `scripts/test-codebase-memory-activation.sh:109-130`
- [ ] T016 [P] [US3] Verify `assert_not_contains` for `systemMessage` in subagent hook in `scripts/test-codebase-memory-activation.sh:124`
- [ ] T017 [US3] Verify loop checks all 6 `mcp__codebase-memory-mcp__` tools in subagent output in `scripts/test-codebase-memory-activation.sh:127-129`

**Checkpoint**: Canal subagentes verificado — 8 checks cubiertos

---

## Phase 6: User Story 4 - Validar canal OpenCode (Priority: P2)

**Goal**: Plugin `gomemory.ts` contiene `EXTERNAL CODE GRAPH` con las 6 tools

**Independent Test**: Ejecutar sección 4 del script

### Implementation for User Story 4

- [ ] T018 [P] [US4] Verify `test_opencode_plugin()` reads `infrastructure/plugin/opencode/gomemory.ts` in `scripts/test-codebase-memory-activation.sh:134-155`
- [ ] T019 [P] [US4] Verify check for `EXTERNAL CODE GRAPH` section exists in `scripts/test-codebase-memory-activation.sh:146`
- [ ] T020 [P] [US4] Verify loop checks all 6 `codebase-memory-mcp_<tool>` prefixed names in `scripts/test-codebase-memory-activation.sh:148-150`
- [ ] T021 [US4] Verify `PLAN MODE` and `get_plan_context` checks in `scripts/test-codebase-memory-activation.sh:153-154`

**Checkpoint**: Canal OpenCode verificado — 9 checks cubiertos

---

## Phase 7: User Story 5 - Validar canal integración AGENTS.md (Priority: P3)

**Goal**: `cmd_install.go` referencia codebase-memory-mcp

**Independent Test**: Ejecutar sección 5 del script

### Implementation for User Story 5

- [ ] T022 [P] [US5] Verify `test_integration_block()` reads `adapters/primary/cli/cmd_install.go` in `scripts/test-codebase-memory-activation.sh:159-174`
- [ ] T023 [P] [US5] Verify check for `codebase-memory-mcp` reference in `scripts/test-codebase-memory-activation.sh:169`
- [ ] T024 [US5] Verify check for `exploración de código` instruction in `scripts/test-codebase-memory-activation.sh:170`
- [ ] T025 [US5] Verify check for `CodebaseMemoryMCPDiscoveryTools` variable reference in `scripts/test-codebase-memory-activation.sh:173`

**Checkpoint**: Canal integración verificado — 3 checks cubiertos

---

## Phase 8: User Story 6 - Validar contrato de tools gomemory (Priority: P3)

**Goal**: Las 15 tools de gomemory aparecen en el bootstrap

**Independent Test**: Ejecutar sección 7 del script

### Implementation for User Story 6

- [ ] T026 [P] [US6] Verify `test_bootstrap_completeness()` runs `session-start` + `user-prompt-submit` in `scripts/test-codebase-memory-activation.sh:195-229`
- [ ] T027 [P] [US6] Verify gomemory_tools array has 15 entries matching `domain.MCPAllTools()` in `scripts/test-codebase-memory-activation.sh:207-223`
- [ ] T028 [US6] Verify loop checks all 15 `mcp__gomemory__<tool>` names in `scripts/test-codebase-memory-activation.sh:225-227`

**Checkpoint**: Contrato verificado — 15 checks cubiertos

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and CI integration

- [ ] T029 [P] Verify `scripts/test-codebase-memory-activation.sh` appears in project README or CONTRIBUTING docs (if applicable)
- [ ] T030 [P] Verify script cleanup logic removes temp binary and dirs in `scripts/test-codebase-memory-activation.sh:271-272`
- [ ] T031 Run quickstart.md validation scenarios end-to-end
- [ ] T032 Verify total check count is 50 by counting all `pass`/`fail` calls across all test functions

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories
- **User Stories (Phase 3-8)**: All depend on Foundational phase completion
  - US1 and US2 (P1) can run in parallel
  - US3 and US4 (P2) can run in parallel, after US1/US2
  - US5 and US6 (P3) can run in parallel, after US3/US4
- **Polish (Phase 9)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Script compilation + main runner — independent
- **US2 (P1)**: Claude Code hook validation — independent
- **US3 (P2)**: Subagent hook validation — independent (shares pattern with US2)
- **US4 (P2)**: OpenCode plugin validation — independent
- **US5 (P3)**: AGENTS.md integration validation — independent
- **US6 (P3)**: gomemory tools contract — independent

### Within Each User Story

- Verification tasks marked [P] can run in parallel (different aspects of same function)
- Sequential tasks verify logical flow within the function

### Parallel Opportunities

- T003, T004, T005: All constant verification tasks (different files)
- T010-T014: All US2 checks (same function, different assertions)
- T015-T017: All US3 checks
- T018-T021: All US4 checks
- T022-T025: All US5 checks
- T026-T028: All US6 checks
- T029, T030: Polish tasks (different files)

---

## Parallel Example: User Story 2

```bash
# All US2 verification tasks can run in parallel (different assertions, same file):
Task: "Verify test_claude_code_hook() creates temp dir (T010)"
Task: "Verify assert_not_contains for systemMessage (T011)"
Task: "Verify loop over TOOLS array (T012)"
Task: "Verify loop over ADMIN_TOOLS (T013)"
Task: "Verify get_plan_context check (T014)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (verify permissions)
2. Complete Phase 2: Foundational (verify constants)
3. Complete Phase 3: User Story 1 (verify compilation + runner)
4. **STOP and VALIDATE**: Run `./scripts/test-codebase-memory-activation.sh` — exit code 0 with 50 checks
5. Script is functional as regression tool

### Incremental Delivery

1. Setup + Foundational → Constants verified
2. US1 → Script compiles and runs → MVP regression tool
3. US2 → Claude Code hook fully verified
4. US3 → Subagent hook fully verified
5. US4 → OpenCode plugin fully verified
6. US5 → AGENTS.md integration fully verified
7. US6 → gomemory tools contract fully verified
8. Polish → Documentation + CI ready

### Notes

- This feature is a VERIFICATION of an existing script, not implementation from scratch
- The script already passes all 50 checks — tasks are about formalizing against the spec
- Each task is specific enough to be completed by an LLM without additional context
- File paths reference exact line numbers for precision
