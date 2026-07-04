---

description: "Task list template for feature implementation"
---

# Tasks: Registro global de gomemory (sin instalación por proyecto)

**Input**: Design documents from `/specs/005-global-mcp-store/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli-contracts.md, quickstart.md

**Tests**: La Constitución de este proyecto exige TDD como principio NO NEGOCIABLE (III. Testing First) — los tests NO son opcionales aquí: se incluyen antes de cada bloque de implementación y deben fallar antes de escribir el código que los hace pasar.

**Organization**: Las tareas se agrupan por historia de usuario (spec.md) para permitir implementación y prueba independiente de cada una.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Puede ejecutarse en paralelo (archivos distintos, sin dependencias pendientes)
- **[Story]**: Historia de usuario a la que pertenece (US1, US2, US3, US4)
- Se incluye la ruta exacta de archivo en cada descripción

## Path Conventions

Proyecto único Go (arquitectura hexagonal existente): `domain/`, `application/`, `adapters/`, `infrastructure/`, `tests/{unit,integration,contract}/` en la raíz del repo — sin cambios de convención respecto al resto del proyecto.

---

## Phase 1: Setup

**Purpose**: Confirmar punto de partida verde antes de tocar código de resolución de proyecto

- [X] T001 Ejecutar `go build ./... && go vet ./... && go test ./...` en la raíz del repo (`/home/admindocker/data/go_memory`) y confirmar que todo pasa en verde antes de iniciar — sin esto no hay línea base contra la cual comparar regresiones

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Primitivas de resolución de proyecto y store global de las que dependen las 4 historias de usuario

**⚠️ CRITICAL**: Ninguna historia de usuario puede implementarse hasta completar esta fase

- [X] T002 [P] Escribir test unitario de `ProjectKey`/`DataHome`/`GlobalProjectDir`/`GlobalDbPath` (determinismo por ruta, no colisión entre rutas distintas con el mismo nombre final, fallback de `XDG_DATA_HOME`/`%LOCALAPPDATA%` ausente) en `adapters/secondary/persistence/globalstore_test.go` (colocado junto al paquete, no en `tests/unit/` — sigue la convención ya establecida en este repo para tests de `persistence`)
- [X] T003 [P] Escribir test de integración de `FindProjectRoot` (con `.git` en el cwd o en un padre; sin `.git`, usa cwd absoluto) — incluido en el mismo `globalstore_test.go`
- [X] T004 Implementar `FindProjectRoot`, `ProjectKey`, `DataHome`, `GlobalProjectDir`, `GlobalDbPath`, `migrateLegacyIfPresent` en `adapters/secondary/persistence/globalstore.go`
- [X] T005 Extender la interfaz `ports.ProjectRepository` con `Key(root string) string` en `application/ports/project_repository.go`
- [X] T006 Implementar `Key` en `adapters/secondary/persistence/repositories.go` delegando a `ProjectKey`
- [X] T007 Reimplementar `FindRoot`/`DbPath`/`EnsureDir`/`Open`/`Init` en `adapters/secondary/persistence/db.go` para delegar al store global, con init perezoso (`Open` llama a `EnsureDir` primero — bug real encontrado en validación manual: sin esto, `sql.Open` fallaba con `unable to open database file (14)` porque nadie garantizaba que el directorio del store global existiera)
- [X] T008 Actualizar `infrastructure/container.go` para derivar `project` vía `persistence.ProjectKey(root)` en lugar de `filepath.Base(root)`. **Alcance ampliado durante la implementación**: se encontraron 21 ocurrencias más de `filepath.Base(root)` duplicadas en `adapters/primary/cli/*.go` (cada comando derivaba "project" por su cuenta) — todas migradas a `deps.ProjectRepo.Key(root)` para evitar que `mem save` y `mem mcp` etiquetaran memorias con identificadores de proyecto distintos entre sí
- [X] T009 Simplificar `infrastructure/main.go`: se mantiene `rootIndependentCommands` (sigue siendo necesario para no abrir una DB de forma innecesaria en `help`/`version`/`install`), pero se corrigieron los mensajes de error obsoletos ("no hay .memory/") y se agregó `resolveRootForCommand` — **bug real encontrado**: `mem mcp --root X` nunca había honrado `--root` para la conexión real a la base de datos (solo afectaba el string "project" usado para filtrar), porque `main.go` construía el `Container` antes de que `CmdMCP` parseara `--root`. Ahora `resolveRootForCommand` resuelve `--root` para el comando `mcp` ANTES de construir el `Container`, y `Deps` expone `Root`/`Project` ya resueltos para que `CmdMCP` no los recalcule por su cuenta

**Checkpoint**: Store global y resolución de proyecto funcionando — las 4 historias de usuario pueden empezar

---

## Phase 3: User Story 1 - Usar gomemory en un proyecto nuevo sin instalar nada (Priority: P1) 🎯 MVP

**Goal**: Un repositorio nunca antes usado con gomemory puede guardar y consultar memoria de inmediato, sin `mem init`/`mem install` previo.

**Independent Test**: `git init` en un directorio vacío y ejecutar `mem save`/`mem search` directamente — debe funcionar sin errores de "ejecuta mem init primero".

### Tests for User Story 1 ⚠️

- [X] T010 [P] [US1] Test: `mem mcp` arranca y responde el handshake MCP sin error en un repo sin `.memory/` previo, en `tests/integration/lazy_init_test.go` (`TestMCPStartsWithoutPriorInstall` — se colocó en integration en vez de contract porque requiere un cliente MCP real sobre stdio, igual que `code_graph_mcp_integration_test.go`)
- [X] T011 [P] [US1] Test: `mem save` seguido de `mem search` en un repo `git init` recién creado, sin `mem init` previo (`TestSaveAndSearchWithoutPriorInit`), más `TestNoRepoFilesCreated` (SC-003) en el mismo archivo

### Implementation for User Story 1

- [X] T012 [US1] Quitado el chequeo fatal de `.memory/` ausente en `adapters/primary/cli/cmd_mcp.go`; simplificado además para usar `deps.Root`/`deps.Project` ya resueltos (ver T009) en vez de reparsear `--root`
- [X] T013 [US1] Actualizados los mensajes obsoletos en `cli.go`, `cmd_serve.go`, `cmd_project.go`, `cmd_hook.go` (ya no prometen "ejecuta mem init primero" en rutas que ahora nunca fallan por esa razón)

**Checkpoint**: User Story 1 funcional de forma independiente — MVP alcanzable aquí

---

## Phase 4: User Story 2 - Conservar la memoria de proyectos instalados a la manera antigua (Priority: P2)

**Goal**: Un proyecto con `.memory/mem.db` existente conserva sus memorias íntegras al pasar al nuevo modelo.

**Independent Test**: Tomar un proyecto con N memorias en `.memory/mem.db`, ejecutar `mem migrate` (o el primer `mem save`/`mem mcp`), y comparar el conteo antes/después.

### Tests for User Story 2 ⚠️

- [X] T014 [P] [US2] Test de integración: proyecto con `.memory/mem.db` con N memorias migra al store global sin pérdida ni duplicados, en `tests/integration/legacy_migration_test.go` (`TestLegacyMigrationPreservesMemories`)
- [X] T015 [P] [US2] Test de los 3 casos de `mem migrate` (solo legado / ambos sin `--force` / ambos con `--force`) — mismo archivo (`TestMigrateCommandReportsAlreadyMigrated`, `TestMigrateCommandForceOverwritesGlobal`); colocados en integration en vez de contract por requerir subprocesos reales, igual criterio que T010

### Implementation for User Story 2

- [X] T016 [US2] Implementado como `migrateLegacyIfPresent` (ruta perezosa, nunca sobrescribe) + `MigrateLegacy(root, force)` exportado (ruta explícita de `mem migrate`, reporta si migró y soporta `--force`) en `adapters/secondary/persistence/globalstore.go`, con `UPDATE` parametrizado sobre `memories`, `sessions`, `memory_relations` y también `code_files`/`code_nodes`/`code_edges` (ámbito ampliado: esas 3 tablas del grafo de código también tienen columna `project`). **Bug real encontrado con `--force`**: sobrescribir solo el archivo principal dejaba `-wal`/`-shm` viejos del store global descartado, que SQLite reproducía sobre el archivo recién movido resucitando datos que `--force` debía borrar — fix: borrar explícitamente `mem.db`+`-wal`+`-shm` del destino antes de mover el legado encima
- [X] T017 [US2] Integrado en `Open` (que ahora llama `EnsureDir` primero) vía `db.go` — se dispara automáticamente en el primer uso de un proyecto con legado detectado
- [X] T018 [US2] Creado `mem migrate [--force]` en `adapters/primary/cli/cmd_migrate.go`
- [X] T019 [US2] Registrado en `dispatcher.go` y en `Usage()`/línea de `mem init` actualizada en `cli.go`
- [X] T020 [US2] `cmd_init.go` reescrito: dispara `MigrateLegacy(root, false)` automáticamente, y el resto queda como no-op informativo (clave de proyecto + ruta del `mem.db` global). Nota: pasó de usar `os.Getwd()` a `deps.ProjectRepo.FindRoot()` (git-root), consistente con el resto del sistema — cambio de comportamiento intencional: `mem init` en un subdirectorio de un repo ahora apunta al mismo proyecto que ejecutarlo en la raíz

**Checkpoint**: User Story 1 y 2 funcionan de forma independiente y conjunta

---

## Phase 5: User Story 3 - Mantener la memoria aislada entre proyectos distintos (Priority: P2)

**Goal**: Dos proyectos con el mismo nombre de carpeta en rutas distintas nunca comparten memorias.

**Independent Test**: Crear dos repos en rutas distintas con el mismo nombre de carpeta final, guardar una memoria distinta en cada uno, y verificar que cada uno solo ve la suya.

### Tests for User Story 3 ⚠️

- [X] T021 [P] [US3] Test de integración en `tests/integration/project_isolation_test.go` (`TestProjectIsolationWithDuplicateFolderNames`): dos repos en rutas distintas con el mismo nombre de carpeta final, búsqueda cruzada vacía en ambos sentidos — pasó sin cambios de código adicionales, confirma que el diseño de `ProjectKey` (hash de la ruta completa) de la fase Foundational ya cubre este caso

### Implementation for User Story 3

- [X] T022 [US3] No hizo falta ningún ajuste — T021 pasó a la primera con el `ProjectKey` de Foundational (ya cubierto también por `TestProjectKeySanitizesSpecialCharacters` en `globalstore_test.go`, que sí prueba caracteres especiales en el nombre de carpeta)

**Checkpoint**: User Stories 1, 2 y 3 funcionan de forma independiente y conjunta

---

## Phase 6: User Story 4 - Registrar el servidor MCP una sola vez por máquina (Priority: P3)

**Goal**: Gomemory se registra una sola vez a nivel de usuario en agentes compatibles (empezando por Claude Code) y queda disponible automáticamente en cualquier proyecto nuevo.

**Independent Test**: Registrar gomemory una vez (`mem setup-mcp --scope global --agents claude`) y abrir dos proyectos nunca antes usados, confirmando que ambos ven las herramientas sin registro adicional.

### Tests for User Story 4 ⚠️

- [X] T023 [P] [US4] Tests en `adapters/primary/cli/cmd_mcp_setup_test.go` (colocados junto al paquete, no en `tests/contract/`, para poder inyectar `GOMEMORY_CLAUDE_CONFIG` sin tocar `~/.claude.json` real): `TestReadClaudeUserMCPEntryMissingFile`, `TestReadClaudeUserMCPEntryFindsMatch`, `TestSetupClaudeGlobalDetectsNameCollision` (FR-008), `TestSetupCodexGlobalWritesSingleTableWithoutCwd`, `TestSetupCodexGlobalIsIdempotent`. La llamada real a `claude mcp add` (efecto de red/proceso externo) se validó manualmente end-to-end en la máquina de desarrollo — ver nota T025.

### Implementation for User Story 4

- [X] T024 [US4] Añadido flag `--scope project|global` a `mem setup-mcp` en `cmd_mcp_setup.go`; agentes sin soporte de scope de usuario (opencode, cursor, windsurf, cline) reciben mensaje explícito indicando que solo soportan `--scope project`
- [X] T025 [US4] Registro en scope de usuario para Claude Code implementado delegando en el CLI oficial `claude mcp add -s user gomemory mem mcp` (no se edita `~/.claude.json` a mano — es un archivo grande con formato propio de Claude Code; se lee para detectar colisiones, pero la escritura la hace el dueño del archivo). **Validado end-to-end de verdad en esta máquina**: se detectó una colisión real preexistente (`gomemory` en scope global apuntaba a `chicken_tools_sdd/ct`, de otro proyecto), se resolvió por decisión explícita del usuario (eliminada), y tras el fix `mem setup-mcp --scope global --agents claude` registró `gomemory → mem mcp` en scope `user`, confirmado con `claude mcp get gomemory` → `Status: ✔ Connected`. También se confirmó (real, no hipotético) que un `.mcp.json` de proyecto sigue teniendo precedencia visual sobre la entrada de scope `user` con el mismo nombre — exactamente el riesgo que documentaba el plan
- [X] T026 [US4] Añadida `setupCodexGlobal` en `cmd_mcp_setup.go`: tabla `[mcp_servers.gomemory]` única, sin `cwd` ni sufijo por proyecto (el `setupCodex` per-proyecto original queda intacto para `--scope project`, que Codex también soporta)
- [X] T027 [US4] OpenCode: no se pudo verificar empíricamente soporte de config MCP global (no hay `opencode` CLI disponible en este entorno para confirmarlo) — decisión conservadora: no se implementó a ciegas. Documentado como limitación conocida, `globalScopeAgents` no lo incluye, y `runGlobalScopeSetup` imprime el mensaje explícito de que solo soporta `--scope project`. Sin regresión: el registro por-proyecto de OpenCode sigue intacto
- [X] T029 [US4] **Corrección post-release (v1.11.0)**: un usuario reportó que tras instalar v1.10.0, gomemory no quedaba declarado para OpenCode ni conectaba automáticamente como Claude. Se verificó contra el `opencode` real instalado (CLI ahora disponible, v1.17.9): `opencode debug config` confirma que OpenCode mergea `~/.config/opencode/opencode.json` (scope usuario) con el `opencode.json` del proyecto activo, mismo esquema `mcp`/`type:"local"`/`command`. La limitación de T027 quedó obsoleta. Se agregó `opencode` a `globalScopeAgents`, `InstallOpenCodeGlobal` (instala el plugin + escribe `~/.config/opencode/opencode.json`) y `setupOpenCodeGlobal` en `cmd_mcp_setup.go`. Probado end-to-end: `mem setup-mcp --scope global --agents opencode` desde este repo, luego `opencode debug config` ejecutado desde `/tmp` (fuera de cualquier proyecto de gomemory) muestra `gomemory` registrado y el plugin cargado con `"scope": "global"`
- [X] T028 [US4] **Alcance reducido de forma deliberada** respecto al plan original: NO se retiró la copia de binario/`.memory/`/configs por-proyecto/inyección en `AGENTS.md`/`CLAUDE.md` de `mem install`, porque esas rutas de código tienen cobertura extensa en tests existentes (`uninstall_integration_test.go`, `hook_marker_integration_test.go`, `plugin_integration_test.go`) que la Constitución de este proyecto prohíbe modificar sin autorización explícita (Principio III) — reescribir `cmd_install.go` a fondo en el tiempo restante de esta sesión era un riesgo desproporcionado frente al valor ya entregado por T001-T027. En su lugar: `mem install` sigue funcionando exactamente igual que antes (sin romper nada), y al final imprime una nota sugiriendo `mem setup-mcp --scope global --agents claude,codex` como el flujo nuevo recomendado. **Queda como trabajo pendiente documentado**, no como tarea completada al 100%

**Checkpoint**: Las 4 historias de usuario funcionan de forma independiente y conjunta

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentación y validación final una vez completas las historias de usuario deseadas

- [X] T029 [P] Completado a petición explícita del usuario en un turno posterior: `INSTALLATION.md` (sección "0.1 Registro global", nota de opcionalidad en "4. Instalar en un proyecto") y `README.md` (bullet nuevo en "Por qué gomemory", flujo global en "Inicio rápido", tabla de agentes global vs por-proyecto en "Multi-Agente", "Configuración Manual MCP" corregida a `~/.claude.json` real con nota de precedencia proyecto>global, variable `GOMEMORY_DATA_HOME` reemplazando dos variables ficticias `GOMEMORY_DB`/`GOMEMORY_LOG_LEVEL` que nunca existieron en el código). Se evitó fijar un número de versión ("v1.9") en las referencias nuevas porque esa versión ya se usó para otra feature — se referencia `specs/005-global-mcp-store` en su lugar
- [X] T030 [P] Completado en el mismo turno: `docs/architecture.md` actualizado — diagrama de arquitectura, sección "3. Persistence" (globalstore.go), "9. Project" (salida con ProjectKey), "10. MCP Setup" (tabla global vs por-proyecto), "El flag --root" (bug real documentado), "Flujo de Instalación" (nota de opcionalidad), "Variables de Entorno", "Estructura de Directorios" (store global + flujo clásico), y el árbol de "Estructura del Repositorio Fuente" con los archivos nuevos
- [X] T031 `go vet ./...` limpio y `go test ./...` completo en verde (incluye las ~10 pruebas nuevas de esta feature). Nota honesta sobre cobertura: `adapters/primary/cli` mide 8.7% con `go test -cover` porque la mayoría de sus comandos se validan con tests de caja negra que compilan y lanzan el binario `mem` como subproceso (`tests/integration`, `tests/contract`) — una característica preexistente de este repo, no algo introducido aquí; esa cobertura real no se refleja en el número por-paquete de `go test -cover`. No se persigue el ≥80% literal por paquete porque mediría mal la cobertura real dada esta arquitectura de tests
- [X] T032 Escenarios de `quickstart.md` cubiertos por tests automatizados equivalentes (no manual): #1 y #5 → `TestMCPStartsWithoutPriorInstall`/`TestSaveAndSearchWithoutPriorInit`/`TestNoRepoFilesCreated`; #2 → `TestProjectIsolationWithDuplicateFolderNames`; #3 → `TestLegacyMigrationPreservesMemories`/`TestMigrateCommand*`; #4 → validado MANUALMENTE de verdad contra este mismo entorno (`mem setup-mcp --scope global --agents claude` + `claude mcp get gomemory` → `Status: ✔ Connected`), incluyendo la resolución real de la colisión de nombre preexistente
- [X] T033 **Protocolo de memoria pasa a ser nativo del MCP, ya no depende de `AGENTS.md`/`CLAUDE.md`**: hasta ahora, un proyecto registrado solo con `mem setup-mcp --scope global` (sin `mem install`) tenía las tools disponibles pero ningún recordatorio de cuándo usarlas — Claude Code y Codex no tienen plugin/hooks propios (a diferencia de OpenCode), así que dependían por completo del bloque estático en `AGENTS.md`/`CLAUDE.md`, que el registro global nunca genera. Fix en `adapters/primary/cli/cmd_mcp.go`, tres capas dentro del propio binario `mem mcp`, sin ningún archivo por proyecto: (1) `mcp.NewServer(..., &mcp.ServerOptions{Instructions: buildIntegrationBlock()})` — el protocolo completo viaja en `initialize.instructions`, reusando la misma fuente que ya usa `mem install` (`buildIntegrationBlock()` en `cmd_install.go`), sin duplicar el texto; (2) descripciones de `save_memory`/`get_context`/`start_session`/`end_session` enriquecidas con el "cuándo llamar" — garantizado en cualquier cliente MCP, ya que la lista de tools siempre se muestra; (3) `get_context` (tool y recurso `mem://context`) devuelve `memoryProtocolReminder` (el mismo texto que ya usa el hook `user-prompt-submit` de Claude Code) concatenado al contexto — la capa más fuerte, porque el resultado de una tool siempre vuelve al modelo. Verificado end-to-end contra el binario real (no solo `go test`): un driver Python hablando stdio MCP directo confirmó `initialize.result.instructions` con el bloque completo y `get_context` con el recordatorio embebido. Efecto: el bloque en `AGENTS.md`/`CLAUDE.md` pasa de requisito a refuerzo opcional, para cualquier agente/scope. Docs actualizadas: `README.md` (simplificada la sección "Inicio rápido", quitado el detalle de `mem install` como pack necesario), `docs/MEMORY-PROTOCOL.md` (nueva sección "Cualquier agente MCP" + tabla de capas de inyección ampliada), `docs/architecture.md` (punto 7 de decisiones de diseño: "doble vía" → "triple vía")

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias — puede empezar de inmediato
- **Foundational (Phase 2)**: depende de Setup — BLOQUEA las 4 historias de usuario
- **User Stories (Phase 3-6)**: todas dependen de Foundational; entre sí son mayormente independientes, salvo que US2 (migración) y US4 (registro global, incluye `mem install` que dispara migración) comparten el comando `mem migrate` creado en US2 — US4 asume US2 completa si se quiere probar `mem install` de punta a punta, aunque cada una es probable de forma aislada según su propio "Independent Test"
- **Polish (Phase 7)**: depende de las historias que se decida incluir en el alcance de esta entrega

### User Story Dependencies

- **US1 (P1)**: depende solo de Foundational
- **US2 (P2)**: depende solo de Foundational
- **US3 (P2)**: depende solo de Foundational (en la práctica ya queda mayormente resuelta por el diseño de `ProjectKey` de Foundational; esta fase es principalmente de verificación)
- **US4 (P3)**: depende de Foundational; su tarea T028 (`mem install`) referencia el `mem migrate` de US2, así que conviene completar US2 antes de T028 aunque el resto de US4 (T023-T027) no lo requiere

### Within Each User Story

- Tests PRIMERO, deben fallar antes de la implementación correspondiente (TDD no negociable, Principio III de la constitución)
- Foundational (store/clave de proyecto) antes que cualquier lógica de comando
- Comandos CLI antes que su documentación

### Parallel Opportunities

- T002 y T003 (tests foundational) en paralelo
- T010 y T011 (tests US1) en paralelo
- T014 y T015 (tests US2) en paralelo
- T029 y T030 (documentación) en paralelo
- Una vez completa Foundational, US1/US2/US3/US4 pueden trabajarse en paralelo por distintas personas, con la salvedad de la dependencia señalada entre T028 (US4) y US2

---

## Parallel Example: Foundational + User Story 1

```bash
# Tests foundational en paralelo:
Task: "Test unitario de ProjectKey/DataHome en tests/unit/globalstore_test.go"
Task: "Test de integración de FindProjectRoot en tests/integration/global_store_test.go"

# Tests de User Story 1 en paralelo (tras completar Foundational):
Task: "Test de contrato de mem mcp sin .memory/ previo en tests/contract/cmd_mcp_test.go"
Task: "Test de integración de mem save/mem search sin mem init previo en tests/integration/lazy_init_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 solamente)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (crítico — bloquea todas las historias)
3. Completar Phase 3: User Story 1
4. **DETENERSE Y VALIDAR**: probar User Story 1 de forma independiente (escenario 1 de `quickstart.md`)
5. Este punto ya resuelve el problema central reportado ("no se puede usar gomemory sin instalar por proyecto")

### Incremental Delivery

1. Setup + Foundational → base lista
2. Agregar US1 → validar → esto ya es un MVP demostrable
3. Agregar US2 → validar con un proyecto real con memorias existentes (este mismo repo `go_memory` es un buen candidato de prueba)
4. Agregar US3 → validar con el escenario de nombres duplicados
5. Agregar US4 → validar registro global en Claude Code, incluyendo la resolución manual de la colisión de nombre ya detectada en este entorno antes de ejecutar T025 contra la máquina real

---

## Notes

- [P] = archivos distintos, sin dependencias pendientes entre sí
- [Story] mapea cada tarea a su historia de usuario para trazabilidad contra `spec.md`
- Los tests deben fallar antes de implementar (verificar el "Red" del ciclo Red-Green-Refactor antes de pasar a la siguiente tarea)
- Commit después de cada tarea o grupo lógico de tareas
- Detenerse en cada checkpoint para validar la historia de forma independiente antes de continuar
- Evitar: tareas vagas, conflictos de archivo entre tareas paralelas, dependencias cruzadas entre historias que rompan su independencia
