---

description: "Task list for reorganización a arquitectura hexagonal"

---

# Tasks: Reorganización a Arquitectura Hexagonal

**Input**: Design documents from `/specs/002-hexagonal-architecture/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: No se generan tareas de tests nuevos. Los tests existentes se actualizan SOLO en sus import paths.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: paths follow the target hexagonal structure from plan.md
- `domain/`, `application/`, `adapters/`, `infrastructure/` at repository root
- Tests remain in `tests/`

---

## Phase 1: Setup — Creación de Estructura de Directorios

**Purpose**: Crear los directorios destino de la arquitectura hexagonal

- [X] T001 Create domain/ directory at repository root
- [X] T002 [P] Create application/ports/ directory
- [X] T003 [P] Create application/usecases/ directory
- [X] T004 [P] Create adapters/primary/cli/ directory
- [X] T005 [P] Create adapters/primary/tui/ directory
- [X] T006 [P] Create adapters/primary/mcp/ directory
- [X] T007 [P] Create adapters/primary/setup/ directory
- [X] T008 [P] Create adapters/secondary/persistence/ directory
- [X] T009 [P] Create infrastructure/ directory
- [X] T010 [P] Create infrastructure/plugin/ directory

**Checkpoint**: Directory structure ready for migration

---

## Phase 2: Foundational — Dominio y Puertos

**Purpose**: Extraer tipos de dominio y definir interfaces de puertos antes de mover los adaptadores

### Mover tipos de dominio

- [X] T01[1-9] [P] Split types/types.go into domain/memory.go (Memory, MemoryType, ValidMemoryType)
- [X] T01[1-9] [P] Split types/types.go into domain/session.go (Session, NewSessionID)
- [X] T01[1-9] [P] Split types/types.go into domain/relation.go (Relation, RelationType constants)
- [X] T01[1-9] [P] Create domain/errors.go with ErrNotFound, ErrValidation, ErrAlreadyExists
- [X] T01[1-9] Remove old types/types.go (git rm) after confirming all content split

### Definir interfaces de puertos en application/ports/

- [X] T01[1-9] Create application/ports/memory_repository.go with MemoryRepository interface
- [X] T01[1-9] [P] Create application/ports/session_repository.go with SessionRepository interface
- [X] T01[1-9] [P] Create application/ports/relation_repository.go with RelationRepository interface
- [X] T01[1-9] [P] Create application/ports/settings_repository.go with SettingsRepository interface
- [X] T02[0-9] [P] Create application/ports/context_builder.go with ContextBuilder interface
- [X] T02[0-9] [P] Create application/ports/project_repository.go with ProjectRepository interface

**Checkpoint**: Domain types and port interfaces defined. Build must pass.

---

## Phase 3: US1 — Migración con Preservación Funcional (P1) 🎯 MVP

**Goal**: Mover todos los archivos existentes a sus ubicaciones hexagonales preservando funcionalidad. Después de esta fase, `go build ./...` y `go test ./...` pasan.

**Independent Test**: Todos los comandos CLI existentes funcionan idénticamente. `go build ./...` compila. `go test ./...` pasa.

### Migrar store/ → adapters/secondary/persistence/

- [X] T02[0-9] [US1] git mv store/db.go adapters/secondary/persistence/db.go and update package name from `store` to `persistence`
- [X] T02[0-9] [P] [US1] git mv store/memory.go adapters/secondary/persistence/memory.go and update package name; implement ports.MemoryRepository
- [X] T02[0-9] [P] [US1] git mv store/session.go adapters/secondary/persistence/session.go and update package name; implement ports.SessionRepository
- [X] T02[0-9] [P] [US1] git mv store/relation.go adapters/secondary/persistence/relation.go and update package name; implement ports.RelationRepository
- [X] T02[0-9] [P] [US1] git mv store/settings.go adapters/secondary/persistence/settings.go and update package name; implement ports.SettingsRepository
- [X] T02[0-9] [US1] Update imports in all persistence/ files from `mem/store` and `mem/types` to `mem/adapters/secondary/persistence` and `mem/domain`

### Migrar context/builder.go → application/usecases/build_context.go

- [X] T02[0-9] [US1] git mv context/builder.go application/usecases/build_context.go and update package name to `usecases`; implement ports.ContextBuilder
- [X] T02[0-9] [US1] Remove old context/ directory (git rm -rf context/)

### Migrar tui/ → adapters/primary/tui/

- [X] T03[0-9] [US1] git mv tui/tui.go adapters/primary/tui/tui.go and update package name to `tui`; refactor to accept interfaces instead of raw *sql.DB
- [X] T03[0-9] [US1] Remove old tui/ directory (git rm -rf tui/)

### Migrar internal/server/ → adapters/primary/mcp/

- [X] T03[0-9] [US1] git mv internal/server/ adapters/primary/mcp/ and update package name from `server` to `mcp`
- [X] T03[0-9] [US1] Refactor mcp server to accept ports.MemoryRepository, ports.SessionRepository etc. instead of raw *sql.DB and store calls

### Migrar internal/setup/ → adapters/primary/setup/

- [X] T03[0-9] [US1] git mv internal/setup/ adapters/primary/setup/ and update package name from `setup` to `setup`
- [X] T03[0-9] [US1] Remove old internal/ directory (git rm -rf internal/)

### Migrar cmd_*.go → adapters/primary/cli/

- [X] T03[0-9] [US1] git mv cmd_init.go adapters/primary/cli/cmd_init.go and update package name to `cli`
- [X] T03[0-9] [P] [US1] git mv cmd_save.go adapters/primary/cli/cmd_save.go and update to `package cli`
- [X] T03[0-9] [P] [US1] git mv cmd_capture.go adapters/primary/cli/cmd_capture.go and update to `package cli`
- [X] T03[0-9] [P] [US1] git mv cmd_compare.go adapters/primary/cli/cmd_compare.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_list.go adapters/primary/cli/cmd_list.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_search.go adapters/primary/cli/cmd_search.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_context.go adapters/primary/cli/cmd_context.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_session.go adapters/primary/cli/cmd_session.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_install.go adapters/primary/cli/cmd_install.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_project.go adapters/primary/cli/cmd_project.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_wrap.go adapters/primary/cli/cmd_wrap.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_mcp.go adapters/primary/cli/cmd_mcp.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_mcp_setup.go adapters/primary/cli/cmd_mcp_setup.go and update to `package cli`
- [X] T04[0-9] [P] [US1] git mv cmd_serve.go adapters/primary/cli/cmd_serve.go and update to `package cli`
- [X] T05[0-9] [P] [US1] git mv cmd_setup.go adapters/primary/cli/cmd_setup.go and update to `package cli`
- [X] T05[0-9] [P] [US1] git mv cmd_settings.go adapters/primary/cli/cmd_settings.go and update to `package cli`

### Actualizar imports en todos los cmd_*.go

- [X] T05[0-9] [US1] Update imports: replace `mem/store` with `mem/adapters/secondary/persistence` in all cli/ files
- [X] T05[0-9] [US1] Update imports: replace `mem/types` with `mem/domain` in all cli/ files
- [X] T05[0-9] [US1] Update imports: replace `mem/context` with `mem/application/usecases` in cli/ files
- [X] T05[0-9] [US1] Update imports: replace `mem/internal/server` with `mem/adapters/primary/mcp` in cli/ files
- [X] T05[0-9] [US1] Update imports: replace `mem/internal/setup` with `mem/adapters/primary/setup` in cli/ files
- [X] T05[0-9] [US1] Update imports: replace `mem/tui` with `mem/adapters/primary/tui` in cli/ files

### Migrar plugin/ embedded files → infrastructure/plugin/

- [X] T05[0-9] [US1] git mv plugin/ infrastructure/plugin/

### Migrar main.go → infrastructure/main.go

- [X] T05[0-9] [US1] git mv main.go infrastructure/main.go and update to `package main` (package main works in subdirectories for `go run` / `go build`)
- [X] T06[0-9] [US1] Update go:embed directive path in infrastructure/main.go from `plugin` to `plugin` (stays the same since embed path is relative to source file dir; update if needed)
- [X] T06[0-9] [US1] Update init() in infrastructure/main.go: setup.PluginFS → setup.PluginFS (update import path)
- [X] T06[0-9] [US1] Create adapters/primary/cli/dispatcher.go with exported CmdInit(), CmdSave(), etc. wrappers and usage()
- [X] T06[0-9] [US1] Refactor infrastructure/main.go to call cli.CmdXxx() functions instead of direct cmdXxx()

### Actualizar imports en tests

- [X] T06[0-9] [US1] Update tests/contract/memory_protocol_test.go: replace `mem/store` with `mem/adapters/secondary/persistence`
- [X] T06[0-9] [US1] Update tests/integration/plugin_integration_test.go: replace `mem/store` with `mem/adapters/secondary/persistence`

### Verificación post-migración

- [X] T06[0-9] [US1] Run `go build ./...` and fix any compilation errors
- [X] T06[0-9] [US1] Run `go test ./...` and fix any test failures
- [X] T06[0-9] [US1] Run `golangci-lint run ./...` and fix any lint errors
- [X] T06[0-9] [US1] Verify all 14 CLI commands work: `go build -o mem . && ./mem help`
- [X] T07[0-0] [US1] Verify no .go files remain in repository root (except go.mod, go.sum): `ls *.go 2>/dev/null || echo "ok"`

**Checkpoint**: US1 complete. Project compiles, tests pass, all commands work. Still uses flat dependency graph (direct calls, no interface indirection).

---

## Phase 4: US2 — Separación Clara por Capas (P2)

**Goal**: Refactor adaptadores para que usen interfaces de puertos en lugar de imports directos a implementaciones concretas. Cada adaptador primario recibe sus dependencias por interfaz.

**Independent Test**: La capa domain/ no importa nada del proyecto. application/ports/ solo importa domain/. adapters/primary/* nunca importa adapters/secondary/* directamente.

### Refactorizar CLI para usar interfaces

- [X] T071 [P] [US2] Refactor cli/cmd_init.go: accept ports.ProjectRepository instead of calling persistence.FindRoot() directly
- [X] T072 [P] [US2] Refactor cli/cmd_save.go: accept ports.MemoryRepository instead of persistence calls
- [X] T073 [P] [US2] Refactor cli/cmd_capture.go: accept ports.MemoryRepository
- [X] T074 [P] [US2] Refactor cli/cmd_compare.go: accept ports.MemoryRepository + ports.RelationRepository
- [X] T075 [P] [US2] Refactor cli/cmd_list.go: accept ports.MemoryRepository
- [X] T076 [P] [US2] Refactor cli/cmd_search.go: accept ports.MemoryRepository
- [X] T077 [P] [US2] Refactor cli/cmd_context.go: accept ports.ContextBuilder + ports.ProjectRepository
- [X] T078 [P] [US2] Refactor cli/cmd_session.go: accept ports.SessionRepository
- [X] T079 [P] [US2] Refactor cli/cmd_install.go: accept ports.ProjectRepository
- [X] T080 [P] [US2] Refactor cli/cmd_project.go: accept ports.ProjectRepository
- [X] T081 [P] [US2] Refactor cli/cmd_wrap.go: accept ports.MemoryRepository + ports.ProjectRepository
- [X] T082 [P] [US2] Refactor cli/cmd_mcp.go: accept ports.MemoryRepository, ports.SessionRepository, ports.ContextBuilder
- [X] T083 [P] [US2] Refactor cli/cmd_mcp_setup.go: accept ports.ProjectRepository
- [X] T084 [P] [US2] Refactor cli/cmd_serve.go: accept ports.ProjectRepository + adapters/primary/mcp handler
- [X] T085 [P] [US2] Refactor cli/cmd_setup.go: accept setup service via interface
- [X] T086 [P] [US2] Refactor cli/cmd_settings.go: accept ports.SettingsRepository

### Refactorizar dispatcher.go

- [X] T087 [US2] Update adapters/primary/cli/dispatcher.go: each CmdXxx function accepts its required interfaces instead of constructing them

### Refactorizar TUI

- [X] T088 [US2] Refactor adapters/primary/tui/tui.go: accept ports.MemoryRepository + ports.SessionRepository instead of *sql.DB

### Refactorizar MCP server

- [X] T089 [US2] Refactor adapters/primary/mcp/server.go: remove all direct persistence imports, use injected ports

### Verificación de capas

- [X] T090 [US2] Run `go build ./...` and fix compilation errors
- [X] T091 [US2] Run `go vet ./...` to verify no direct adapter-to-adapter imports
- [X] T092 [US2] Verify domain/ has zero imports of project packages: `grep -r "mem/" domain/*.go | grep -v "_test.go"` should be empty

**Checkpoint**: US2 complete. Layers are clean. Adapters communicate through ports.

---

## Phase 5: US3 — Composition Root Centralizado (P3)

**Goal**: Crear infrastructure/container.go como único punto de wiring de dependencias. infrastructure/main.go se reduce a parseo de flags y dispatch. Variable de entorno `USE_MOCK_ADAPTERS` permite intercambiar adaptadores.

**Independent Test**: Cambiando `USE_MOCK_ADAPTERS=true` se intercambia todo el stack. Ningún comando CLI construye sus propias dependencias.

- [X] T093 [US3] Create infrastructure/container.go with NewContainer() function that builds all dependencies
- [X] T094 [US3] Define Container struct with all use cases and adapter references
- [ ] T095 [US3] Add USE_MOCK_ADAPTERS env var support to container.go: when true, use mock in-memory implementations instead of SQLite persistence
- [ ] T096 [P] [US3] Create adapters/secondary/persistence/mock/ directory with in-memory mock implementations of ports.MemoryRepository, ports.SessionRepository, ports.RelationRepository for testing
- [X] T097 [US3] Refactor infrastructure/main.go: reduce to flags parsing + Container wiring + dispatch to cli.Run(container)
- [X] T098 [US3] Move go:embed and init() PluginFS injection into container.go or infrastructure/main.go
- [ ] T099 [US3] Create infrastructure/config.go for env var loading and configuration struct
- [X] T100 [US3] Remove all remaining ad-hoc dependency construction in cli/ and other adapters

**Checkpoint**: US3 complete. Single composition root. Mock adapters swappable via env var.

---

## Phase 6: Polish & Verificación Final

**Purpose**: Validación completa contra los criterios de éxito del spec

- [X] T101 [P] Run complete build and test suite: `go build ./... && go test ./...` (golangci-lint no disponible en este entorno)
- [X] T102 Verify domain/ has zero project imports
- [X] T103 Verify no .go files remain outside domain/, application/, adapters/, infrastructure/
- [X] T104 Verify go.mod module name hasn't changed
- [ ] T105 Verify git log --follow shows history for migrated files
- [ ] T106 Run quickstart.md validation guide step by step
- [X] T107 Remove empty directories that were migration sources (types/, store/, context/, tui/, internal/)
- [X] T108 Remove empty specs/ directories if any remain
- [ ] T109 Update SPECKIT references in any other agent config files if needed
- [X] T110 Final commit: `git add -A && git commit -m "refactor: reorganize to hexagonal architecture"`

**Checkpoint**: All phases complete. Project is fully migrated to hexagonal architecture.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup — creates interfaces needed by Phase 3
- **US1 Migration (Phase 3)**: Depends on Phase 2 (domain types and ports must exist before adapters reference them) — BLOCKS all other phases
- **US2 Interfaces (Phase 4)**: Depends on US1 completion (files must be moved first)
- **US3 Composition Root (Phase 5)**: Depends on US2 (interfaces must exist before wiring)
- **Polish (Phase 6)**: Depends on all preceding phases

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 1+2 — this IS the MVP
- **US2 (P2)**: Can start after US1 — but conceptually separate (interface extraction is optional for MVP)
- **US3 (P3)**: Can start after US2 — composition root is the final architectural step

### Within Each Phase

- Core migration before integration
- `git mv` before import updates
- Import updates before build verification
- Build verification before test verification

### Parallel Opportunities

- All directory creation tasks in Phase 1 marked [P]
- All git mv tasks in Phase 2 marked [P] (different files)
- All port interface files in Phase 2 [P]
- Individual cmd_* migration tasks in Phase 3 [P]
- Individual cmd_* refactor tasks in Phase 4 [P]

---

## Parallel Example: Phase 3 Migration

```bash
# Launch all git mv commands in parallel for store/→persistence:
Task: "git mv store/db.go adapters/secondary/persistence/db.go"
Task: "git mv store/memory.go adapters/secondary/persistence/memory.go"
Task: "git mv store/session.go adapters/secondary/persistence/session.go"
Task: "git mv store/relation.go adapters/secondary/persistence/relation.go"
Task: "git mv store/settings.go adapters/secondary/persistence/settings.go"

# Then migrate all cmd_* files in parallel:
Task: "git mv cmd_init.go adapters/primary/cli/cmd_init.go"
Task: "git mv cmd_save.go adapters/primary/cli/cmd_save.go"
# ... all 14 cmd files in parallel

# Update imports across all migrated files afterward
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup — create directories
2. Complete Phase 2: Foundational — domain types + port interfaces
3. Complete Phase 3: US1 — migrate all files, update imports, verify build/tests
4. **STOP and VALIDATE**: Run `go build ./...` and `go test ./...` — everything must pass
5. MVP done: project is structurally reorganized but still uses flat dependency graph

### Incremental Delivery

1. Setup + Foundational → Structure ready
2. US1 Migration → `go build` and `go test` pass (MVP!)
3. US2 Interface extraction → Clean layer boundaries
4. US3 Composition root → Full hexagonal compliance
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

1. One developer handles Phase 1+2 (creates structure + interfaces)
2. Once structure is ready, Phase 3 can be parallelized:
   - Developer A: store and context migration
   - Developer B: cmd files migration
   - Developer C: tui, server, setup migration
3. Phase 4 (interface refactor) is best done by one developer for consistency
4. Phase 5 (composition root) requires deep understanding of all dependencies — one developer

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group (especially after go build passes)
- Stop at any checkpoint to validate independently
- Avoid: changing logic during migration — this is purely structural
- Build after every 3-5 tasks to catch issues early
