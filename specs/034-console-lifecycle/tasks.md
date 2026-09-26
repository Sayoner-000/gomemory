---

description: "Tareas de implementación de la feature 034: ciclo de vida de gomemory en consola"
---

# Tasks: Ciclo de vida de gomemory en consola — instalar, actualizar y desinstalar sin rastros

**Input**: Documentos de diseño en `specs/034-console-lifecycle/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: **Obligatorias y primero.** La constitución vigente (§10, TDD) y la preferencia de la persona (memoria 222: ceremonia estricta, TDD y dependencias) lo exigen. Cada prueba DEBE escribirse, ejecutarse y **fallar** antes de la tarea que la hace pasar.

**Organization**: tareas agrupadas por historia de usuario. El orden de fases sigue el **orden de entrega** del plan (US1 → US2 → US4 → US3). Las etiquetas `[USn]` conservan la numeración de la spec.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[Story]**: historia de usuario a la que pertenece (US1…US4)
- **⚠ AUTORIZACIÓN**: la tarea modifica una prueba existente. Requiere la aprobación explícita de la persona antes de ejecutarse (§10: solo por un cambio legítimo de comportamiento, en el mismo cambio que la spec)

## Path Conventions

Estructura hexagonal existente en la raíz del repositorio: `domain/`, `application/`, `adapters/`, `infrastructure/`. Pruebas unitarias junto al código (`*_test.go`); pruebas de contrato e integración en `tests/`.

---

## Phase 1: Setup (infraestructura compartida)

**Purpose**: preparar el terreno sin cambiar comportamiento.

- [X] T001 Pasar `github.com/charmbracelet/x/term` de indirecta a directa en `go.mod` con `go mod tidy` y comprobar con `go list -m all | wc -l` que el número de módulos no cambia (R7)
- [X] T002 [P] Crear `.env.example` en la raíz con `GOMEMORY_DATA_HOME`, `GOMEMORY_RELEASE_DOWNLOAD_BASE`, `GOMEMORY_NO_UPDATE_CHECK` y `GOMEMORY_BIN_DIR`, cada una con propósito, ejemplo seguro, obligatoriedad y unidad (R13, constitución §9)
- [X] T003 [P] Crear la ayuda de pruebas `tests/contract/lifecycle_sandbox_test.go`, con un `HOME` y un `GOMEMORY_DATA_HOME` temporales, el binario compilado en `$HOME/.local/bin`, `PATH` acotado y `~/.claude.json` y `~/.codex/config.toml` sembrados con una entrada ajena `otro` (quickstart, preparación común)

---

## Phase 2: Foundational (prerrequisitos que bloquean las historias)

**Purpose**: piezas que usan varias historias. **Ninguna historia empieza antes de terminar esta fase.**

### Pruebas primero

- [X] T004 [P] Prueba de `ParseVersion` y `Newer` (con y sin `v`, prerreleases, entradas inválidas) en `domain/version_test.go`
- [X] T005 [P] Prueba de la resolución única del binario global (sin global, global distinto, global que es la misma copia por `os.SameFile`, `mem.exe` en Windows) en `adapters/primary/cli/binref_test.go`
- [X] T006 [P] Pruebas de la consola: detección de modo (TTY, `--yes`, `NO_COLOR`, `TERM=dumb`, ancho < 60) y renderizador de texto plano de selección, confirmación, confirmación escrita, pasos y resumen ✓/⚠, con entradas simuladas (`tea.WithInput`) en `adapters/primary/console/console_test.go`

### Implementación

- [X] T007 [P] Implementar `Version`, `ParseVersion`, `Newer` y la constante `UpdateCheckTTL = 24h` en `domain/version.go` (hace pasar T004)
- [X] T008 Extraer la resolución del global a `resolveGlobalBinary(projectRoot) (path string, ok bool)` y hacer que `binRefFor` la use en `adapters/primary/cli/binref.go`. `binRefFor` mantiene exactamente su comportamiento (hace pasar T005; dep: T005)
- [X] T009 Implementar el paquete `adapters/primary/console/`: `Mode`, `MultiSelect`, `Select`, `Confirm`, `TypedConfirm`, `Steps`/`Summary` sobre bubbletea/bubbles/lipgloss v2, más un renderizador de texto plano equivalente (hace pasar T006; dep: T001, T006)

**Checkpoint**: `go test ./domain/... ./adapters/primary/console/... ./adapters/primary/cli/ -run 'Version|Binref|Console'` en verde. `go vet` limpio.

---

## Phase 3: User Story 1 — Un solo binario, siempre al día (Priority: P1) 🎯 MVP

**Goal**: con un global, ningún proyecto conserva ni recibe una copia del binario. `./mem update` actualiza el global.

**Independent Test**: quickstart, escenarios 1 y 2. En el sandbox, `install --yes` no crea `p1/mem`; una copia v2.8.0 real en `p2` desaparece en `session-start` con un aviso en `systemMessage`; un `p2/mem` que es un script propio no se toca.

### Pruebas primero

- [X] T010 [P] [US1] Pruebas de identificación y retirada de copias locales: binario de gomemory real (compilado en la prueba) → retirable; script propio `mem`, enlace simbólico, directorio y el mismo archivo que el global → no retirable; tiempo límite de 2 s en `version` → no retirable, en `adapters/primary/cli/local_copy_test.go`
- [X] T011 [P] [US1] Prueba de `renderSessionStart`: sin aviso, salida byte a byte igual a la actual en los dialectos `claude`, `json`, `neutral` y `text`; con aviso, JSON `systemMessage`+`additionalContext` en claude, `systemMessage` en json y `Informa a la persona: …` en neutral/text, en `adapters/primary/cli/hook_dialect_session_start_test.go`
- [X] T012 [P] [US1] Prueba de contrato de `install` con global: `--yes` no crea `<proyecto>/mem` e informa de que usa el global; sin global, mantiene la copia e indica cómo pasar a global (FR-001, FR-002), en `tests/contract/lifecycle_install_test.go`
- [X] T013 [P] [US1] Prueba de contrato de `session-start` con una copia real antigua: se retira, el aviso aparece una vez y el hook termina con código 0 aunque la retirada falle (archivo sin permisos) (FR-003, FR-004, FR-004a), en `tests/contract/hook_session_start_notice_test.go`
- [X] T014 [P] [US1] Prueba de `update` con destino global: ejecutado desde una copia local con un global presente, sustituye el global (servidor `httptest`) y retira la copia; con el global sin permiso de escritura, no descarga nada, imprime el comando y devuelve 1 (FR-006, R6), en `adapters/primary/cli/cmd_update_target_test.go`
- [X] T015 [P] [US1] Prueba de que el protocolo escrito por `install` usa `mem …` y no `./mem …` cuando hay global (FR-005), en `adapters/primary/cli/cmd_install_protocol_global_test.go`

### Implementación

- [X] T016 [US1] Implementar `identifyLocalCopy(root)` y `retireLocalCopy(root, global)` con las reglas de R2 (archivo regular, `version` con tiempo límite de 2 s, prefijo `gomemory `, `!SameFile`) en `adapters/primary/cli/local_copy.go` (hace pasar T010; dep: T008)
- [X] T017 [US1] Implementar `renderSessionStart(d, contexto, avisos)` en `adapters/primary/cli/hook_dialect.go` siguiendo el patrón de `renderTurnEnd` (hace pasar T011)
- [X] T018 [US1] Integrar en `hookSessionStart` la retirada de la copia local (best-effort) y la emisión de su aviso con `renderSessionStart`, en `adapters/primary/cli/cmd_hook.go` (hace pasar T013; dep: T016, T017)
- [X] T019 [US1] Modificar el paso 1 de `CmdInstall` para que no copie el binario si `resolveGlobalBinary` da un global, retire la copia existente e informe; sin global, mantener la copia actual y añadir la indicación para pasar a global, en `adapters/primary/cli/cmd_install.go` (hace pasar T012; dep: T016)
- [X] T020 [US1] Hacer que los subprocesos `init` y `seed` de `CmdInstall` usen el binario global cuando existe (hoy usan `destBin`), en `adapters/primary/cli/cmd_install.go` (dep: T019)
- [X] T021 [US1] Sustituir `./mem` por `mem` en el texto del protocolo cuando hay global (`cmd_install.go`, cerca de la línea 510, y el bloque del protocolo versionado), en `adapters/primary/cli/cmd_install.go` (hace pasar T015)
- [X] T021a [US1] ⚠ AUTORIZACIÓN (concedida 2026-09-26) — Subir `domain.ProtocolVersionMarker` a `<!-- gomemory-protocol-v8 -->` en `domain/protocol.go` (el texto del protocolo cambia en T021; sin subirlo, los proyectos instalados nunca reciben el texto nuevo) y actualizar la aserción literal de `TestProtocolVersionMarker_SubioAV7` en `adapters/primary/cli/cmd_install_protocol_test.go` a v8, conservando el motivo en su comentario
- [X] T022 [P] [US1] Hacer que el script del brazo extensor prefiera `mem` del PATH y use `./mem` solo como alternativa, en `.specify/extensions/gomemory-context/scripts/bash/update-gomemory-context.sh` y `.../powershell/update-gomemory-context.ps1`, y en su copia embebida que distribuye `setup/speckit_extension.go` (FR-005)
- [X] T023 [US1] Hacer que `CmdUpdate` resuelva el destino (el global si es distinto del ejecutable en curso), compruebe que admite escritura antes de descargar y retire la copia local tras sustituir, en `adapters/primary/cli/cmd_update.go` (hace pasar T014; dep: T016)
- [X] T024 [US1] ⚠ AUTORIZACIÓN — Actualizar `tests/integration/update_integration_test.go` (líneas 73 y 117), que usan `<proyecto>/mem` como destino, para reflejar FR-006 (destino global cuando existe), sin debilitar sus aserciones (dep: T023)
- [X] T025 [US1] Actualizar `CHANGELOG.md` (entrada `Unreleased`: binario único y retirada de copias) y `docs/MANUAL.md` (sección de instalación: ya no hay `./mem` en los proyectos)

**Checkpoint US1**: `go test ./...` en verde. Quickstart, escenarios 1 y 2, en el sandbox. **En la máquina real** (solo lectura, escenario 8): tras instalar el binario nuevo y abrir cada proyecto, ya no quedan las 6 copias. Release propio.

---

## Phase 4: User Story 2 — Desinstalar sin dejar rastros (Priority: P2)

**Goal**: `mem uninstall`, con alcance de proyecto o de sistema, deja 0 rastros de gomemory, exporta de forma privada y no toca las entradas ajenas.

**Independent Test**: quickstart, escenarios 6 y 7. `uninstall --all --yes --export` en el sandbox con dos proyectos → `grep -rl gomemory $HOME` vacío, solo quedan las entradas `otro` y la exportación queda con 0700/0600 y se puede reimportar.

### Pruebas primero

- [X] T026 [P] [US2] Pruebas de `UninstallPlan`: orden de `Items` según FR-014, transiciones `pending → ok|warn` y que un `warn` no detiene el recorrido, en `domain/lifecycle_test.go`
- [X] T027 [P] [US2] Pruebas del registro y del escaneo: `Register` idempotente, escritura de `root` con 0600, escaneo con profundidad 6, sin seguir enlaces, omitiendo `.git`/`node_modules`/`vendor`/cachés, reconociendo solo directorios con `.memory/settings.json` y acumulando las rutas sin permiso, en `adapters/secondary/persistence/registry_test.go`
- [X] T028 [P] [US2] Prueba de `OpenByKey`: abre la base de un proyecto por clave sin conocer su ruta y devuelve `nil` si no existe, en `adapters/secondary/persistence/db_test.go` (prueba nueva en el archivo; no modifica las existentes)
- [X] T029 [P] [US2] Prueba de que `mem export` crea el archivo con 0600 (R12), en `adapters/primary/cli/cmd_export_perm_test.go`
- [X] T030 [P] [US2] Pruebas de retirada **parcial** en archivos globales compartidos: `~/.claude.json`, `~/.claude/settings.json`, `~/.config/opencode/*`, `~/.codex/config.toml` y los envoltorios de `atomic_plan_global.go` pierden solo las entradas de gomemory; un archivo que no se puede interpretar se deja como está y produce `warn`, en `adapters/primary/cli/uninstall_global_test.go`
- [X] T031 [P] [US2] Prueba de contrato de alcance de proyecto: con memoria borrada desaparece `DataHome/projects/<key>` (FR-010); con `--keep-memory` se conserva; la configuración global de Codex no se toca, en `tests/contract/lifecycle_uninstall_test.go`
- [X] T032 [P] [US2] Prueba de contrato de alcance de sistema: `--all --yes --export` en el sandbox con dos proyectos deja 0 coincidencias de `gomemory` en `$HOME` y 0 archivos en `DATA_HOME`, conserva las entradas `otro`, crea la exportación con 0700/0600 y un `index.json`, y `mem import` recupera el mismo número de memorias (SC-003, SC-011), en `tests/contract/lifecycle_uninstall_test.go`
- [X] T033 [P] [US2] Prueba de contrato de las garantías: `--dry-run` no modifica ningún archivo (SC-004); sin TTY y sin `--yes`, código 1 y nada borrado (FR-018); `--keep-memory` junto con `--export` da código 2; una exportación fallida (destino de solo lectura) impide borrar la memoria de ese proyecto (FR-009); escaneo por defecto frente a `--scan`/`--no-scan` y aviso de profundidad (aclaración 2026-09-26), en `tests/contract/lifecycle_uninstall_test.go`

### Implementación

- [X] T034 [P] [US2] Implementar `UninstallPlan`, `UninstallItem`, sus categorías y estados, y la lista de directorios omitidos del escaneo en `domain/lifecycle.go` (hace pasar T026)
- [X] T035 [P] [US2] Declarar `ProjectRegistryRepository` en `application/ports/project_registry_repository.go` según data-model
- [X] T036 [US2] Implementar el registro (`projects/<key>/root`, atómico, 0600) y `Scan(ctx, root, maxDepth)` en `adapters/secondary/persistence/registry.go` (hace pasar T027; dep: T034, T035)
- [X] T037 [US2] Implementar `OpenByKey(key)` junto a `Open` en `adapters/secondary/persistence/db.go` (hace pasar T028)
- [X] T038 [US2] Cambiar `os.Create` por `os.OpenFile(path, O_CREATE|O_TRUNC|O_WRONLY, 0o600)` en `adapters/primary/cli/cmd_export.go` (hace pasar T029)
- [X] T039 [US2] Registrar la ruta del proyecto en cada `install` llamando al registro, en `adapters/primary/cli/cmd_install.go` (dep: T036)
- [X] T040 [US2] Implementar el caso de uso de exportación por lotes (directorio 0700, un bundle 0600 por clave reutilizando `ExportProject` de `portability.go`, e `index.json`) en `application/usecases/lifecycle_export.go` (dep: T037)
- [X] T041 [US2] Implementar el caso de uso `PlanUninstall(scope, memoria, scan)`, que construye el inventario con `ActivationInspector` más el binario (R1), el almacén con tamaños y los proyectos registrados ∪ escaneados, en `application/usecases/lifecycle_uninstall.go` (dep: T034, T036)
- [X] T042 [US2] Extender las funciones `remove*` de `cmd_uninstall.go` para que acepten la ruta base (proyecto o `$HOME`) y editen solo las entradas de gomemory en los archivos globales, más la retirada de los envoltorios globales, en `adapters/primary/cli/uninstall_global.go` (hace pasar T030)
- [X] T043 [US2] Reescribir `CmdUninstall` sobre `PlanUninstall`: flags (`--all`, `--yes`, `--keep-memory`, `--export`, `--scan`, `--no-scan`, `--dry-run`), orden de FR-014, borrado de `DataHome/projects/<key>`, continuidad ante `warn`, resumen ✓/⚠ con comando manual y códigos de salida 0/1/2/3, en `adapters/primary/cli/cmd_uninstall.go` (hace pasar T031–T033; dep: T040–T042)
- [X] T044 [US2] Implementar el autoborrado del binario: en Unix `os.Remove` del ejecutable en curso al final; en Windows borrado diferido con `detach`, en `adapters/primary/cli/uninstall_self.go` (dep: T043)
- [X] T045 [US2] Añadir el flujo interactivo de `uninstall` con `console`: alcance → inventario agrupado con tamaños → memoria (recomendada: exportar) → confirmación (`gomemory` en el alcance de sistema) → pasos → resumen, en `adapters/primary/cli/cmd_uninstall.go` (dep: T009, T043)
- [X] T046 [US2] Corregir el texto de confirmación para que enumere exactamente las categorías que se borrarán (FR-019), en `adapters/primary/cli/cmd_uninstall.go` (dep: T043)
- [X] T047 [US2] ⚠ AUTORIZACIÓN — Actualizar `tests/integration/uninstall_integration_test.go` (el `mem` falso de la línea 38 y la aserción de la línea 139) y `tests/contract/maintenance_cli_test.go` (líneas 201-216), para usar un binario de gomemory real y el nuevo contrato de flags, sin debilitar lo que verifican (dep: T043)
- [X] T048 [US2] Documentar la desinstalación (alcances, flags, exportación y reimportación) en `docs/MANUAL.md` y en la sección de seguridad del `README.md`; añadir la entrada al `CHANGELOG.md` (Security: la memoria del almacén global ya no se queda en el disco; la exportación, con 0600)

**Checkpoint US2**: `go test ./...` en verde. Quickstart, escenarios 6 y 7, en el sandbox. **Nunca** en el `$HOME` real. Release propio.

---

## Phase 5: User Story 4 — Enterarse de que hay una versión nueva (Priority: P4)

**Goal**: aviso visible, una vez por versión y sesión, sin red en el camino crítico; `mem update` verifica el checksum; `doctor` informa.

**Independent Test**: quickstart, escenarios 3 y 4. Con una caché `v99.0.0`, un `session-start` muestra el aviso y el siguiente de la misma sesión no; con `GOMEMORY_NO_UPDATE_CHECK=1` no hay proceso ni escritura; un checksum alterado aborta la actualización.

### Pruebas primero

- [X] T049 [P] [US4] Pruebas del adaptador de releases contra `httptest`: `Latest` con ETag (200 y 304), tiempo límite de 5 s, `Checksum` cuando falta el archivo, falta la línea o coincide, en `adapters/secondary/release/github_test.go`
- [X] T050 [P] [US4] Pruebas de la caché: lectura de un archivo inexistente o corrupto (vencida), escritura atómica con 0600, y que un fallo conserva `latest` y `etag` y actualiza `checked_at` y `last_error`, en `adapters/secondary/persistence/update_check_test.go`
- [X] T051 [P] [US4] Pruebas de la decisión de aviso con un reloj inyectado: vencida o vigente, `Newer`, desactivado por ajuste o por variable, dedupe por versión y sesión con `.update-notice`, en `application/usecases/lifecycle_notice_test.go`
- [X] T052 [P] [US4] Prueba de contrato del hook con aviso de versión: aviso una vez; misma sesión, sin aviso; con `GOMEMORY_NO_UPDATE_CHECK=1` no se lanza ningún proceso ni se escribe la caché; una caché corrupta no produce aviso; la duración con la API inalcanzable no aumenta (SC-006, SC-008), en `tests/contract/hook_session_start_notice_test.go`
- [X] T053 [P] [US4] Prueba de `mem update` con un checksum alterado o ausente: código 1 y el binario instalado sin cambios (SC-009), en `adapters/primary/cli/cmd_update_checksum_test.go`
- [X] T054 [P] [US4] Prueba de la sección `Versión y binario` de `mem doctor`, en texto y en `--json` (FR-033), en `adapters/primary/cli/cmd_doctor_version_test.go`

### Implementación

- [X] T055 [P] [US4] Declarar `ReleasePort` y `UpdateCheckRepository` en `application/ports/release.go` y `application/ports/update_check_repository.go`
- [X] T056 [US4] Implementar el adaptador `adapters/secondary/release/github.go` (API de releases con ETag, tiempo límite de 5 s y `checksums.txt`), moviendo allí el HTTP que hoy vive en `cmd_update.go` (`latestReleaseTag`) (hace pasar T049; dep: T055)
- [X] T057 [US4] Implementar la caché en `adapters/secondary/persistence/update_check.go` (hace pasar T050; dep: T055)
- [X] T058 [US4] Añadir `update_check_disabled` a `persistence.Settings` en `adapters/secondary/persistence/settings.go`
- [X] T059 [US4] Implementar el caso de uso de decisión de aviso en `application/usecases/lifecycle_notice.go` (hace pasar T051; dep: T007, T057, T058)
- [X] T060 [US4] Implementar el subcomando fire-and-forget `mem update-check` (sin contenedor, sin salida, código 0) y registrarlo en el despachador, en `adapters/primary/cli/cmd_update_check.go` y `adapters/primary/cli/dispatcher.go` (dep: T056, T057)
- [X] T061 [US4] Integrar en `hookSessionStart` el lanzamiento desacoplado de `update-check` si la caché está vencida (patrón `MaybeRefresh`/`detach`), la emisión del aviso de versión y la escritura de `.memory/.update-notice`, en `adapters/primary/cli/cmd_hook.go` (hace pasar T052; dep: T018, T059, T060)
- [X] T062 [US4] Añadir la verificación SHA-256 antes de `extractBinary` en `CmdUpdate`, y hacer que `--check` renueve la caché, en `adapters/primary/cli/cmd_update.go` (hace pasar T053; dep: T056)
- [X] T063 [US4] Añadir la sección `Versión y binario` (texto y JSON) a `mem doctor` en `adapters/primary/cli/cmd_doctor.go` (hace pasar T054; dep: T008, T016, T057)
- [X] T064 [US4] Hacer el wiring de `ReleasePort` y `UpdateCheckRepository` en el composition root, `infrastructure/container.go`, más la raíz ligera para los comandos sin contenedor (dep: T056, T057)
- [X] T065 [US4] Documentar el aviso, cómo desactivarlo y la verificación del checksum en `docs/MANUAL.md` y en `CHANGELOG.md`, y completar `GOMEMORY_NO_UPDATE_CHECK` en `.env.example` (dep: T002)

**Checkpoint US4**: `go test ./...` en verde. Quickstart, escenarios 3 y 4. Release propio.

---

## Phase 6: User Story 3 — Instalar y actualizar con una consola guiada (Priority: P3)

**Goal**: `install` y `update` guiados al estilo skills.sh, con flags equivalentes y la selección guardada; los instaladores de consola ofrecen la instalación guiada.

**Independent Test**: quickstart, escenario 5. En una terminal real, la secuencia agentes → alcance → resumen → confirmación. `install </dev/null` termina sin esperar; `jq .agents` muestra la selección; `NO_COLOR=1` da la misma información en texto plano.

### Pruebas primero

- [X] T066 [P] [US3] Pruebas de detección de agentes a partir de `domain.KnownAgents`, con un `PATH` y un `HOME` simulados (ejecutable o directorio de configuración; Windsurf y Cline sin marcar), en `adapters/primary/cli/agent_detect_test.go`
- [X] T067 [P] [US3] Prueba de contrato de `install` no interactivo: `--agents claude` solo escribe la configuración de Claude (FR-021); un agente desconocido da código 2 sin escribir nada; `</dev/null` no espera; `agents` y `agent_scope` quedan guardados, y una reinstalación sin flags los reutiliza (FR-022, FR-023), en `tests/contract/lifecycle_install_test.go`
- [X] T068 [P] [US3] Prueba del flujo interactivo de `install` con entradas simuladas: orden de las preguntas, agentes detectados ya marcados, pregunta del binario solo sin global, cancelación sin cambios, en `adapters/primary/cli/cmd_install_interactive_test.go`
- [X] T069 [P] [US3] Prueba del flujo interactivo de `update`: `Actual → Disponible`, confirmación, pasos y resumen; con `--yes`, sin preguntas (FR-025), en `adapters/primary/cli/cmd_update_interactive_test.go`

### Implementación

- [X] T070 [US3] Implementar `detectAgents(home, path)` sobre `domain.KnownAgents` en `adapters/primary/cli/agent_detect.go` (hace pasar T066)
- [X] T071 [US3] Añadir `agents` y `agent_scope` a `persistence.Settings` en `adapters/secondary/persistence/settings.go`
- [X] T072 [US3] Hacer que el paso 5 de `CmdInstall` configure solo los agentes seleccionados (`setup.InstallOpenCode`, `InstallClaudeCode`, `setupCursor`, `setupCodex`); para `--scope global`, validar Claude/Codex/OpenCode antes de escribir y configurar solo sus integraciones globales, sin hooks ni configuración de agente en el proyecto, en `adapters/primary/cli/cmd_install.go` (FR-022a; dep: T070, T071)
- [X] T073 [US3] Añadir los flags `--yes/-y`, `--agents` y `--scope`, la validación de agentes (código 2), el modo no interactivo y la persistencia y reutilización incluso de una selección vacía en `adapters/primary/cli/cmd_install.go` (hace pasar T067; dep: T072)
- [X] T074 [US3] Añadir el flujo interactivo de `install` con `console` (agentes → binario → alcance → resumen/confirmación → pasos → resumen ✓/⚠) en `adapters/primary/cli/cmd_install.go` (hace pasar T068; dep: T009, T073)
- [X] T075 [US3] Añadir el flujo interactivo de `update` con `console` y el flag `--yes`, y hacer que el refresco del proyecto use la selección guardada, en `adapters/primary/cli/cmd_update.go` (hace pasar T069; dep: T009, T062, T073)
- [X] T076 [P] [US3] Hacer que `scripts/install.sh` ofrezca `mem install .` leyendo de `/dev/tty` cuando hay TTY y el directorio es un repositorio git; sin TTY, mantener el mensaje actual (R14)
- [X] T077 [P] [US3] Hacer que `scripts/install.ps1` ofrezca `mem install .` con `Read-Host` cuando la sesión es interactiva (R14)
- [X] T078 [US3] Documentar la instalación guiada, los flags y la selección guardada en `docs/MANUAL.md`, `README.md` (inicio rápido) y `CHANGELOG.md`

**Checkpoint US3**: `go test ./...` en verde. Quickstart, escenario 5, en una terminal real (la persona lo valida). Release propio.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T079 [P] Actualizar `docs/architecture.md`: paquete `console`, puertos nuevos, caché de versiones, registro de proyectos y el nuevo contrato de `session-start`
- [X] T080 [P] Revisar la idempotencia (FR-035): ejecutar dos veces seguidas `install --yes`, la retirada de copias y `uninstall --dry-run` en el sandbox, sin diferencias en el segundo paso (prueba en `tests/contract/lifecycle_idempotency_test.go`)
- [X] T081 Ejecutar `go vet ./...`, `gofmt -l`, `golangci-lint run` y `govulncheck ./...`, y comprobar que la cobertura global no baja de la línea base; `govulncheck` queda además como requisito previo del job de release en CI
- [X] T082 Validar la guía `quickstart.md` completa en el sandbox y el escenario 8 (solo lectura) en la máquina real; registrar los resultados en la sección de revisión de `tasks/todo.md`
- [X] T083 Ejecutar `/speckit-analyze` sobre spec, plan y tasks y cerrar las inconsistencias que encuentre (análisis cruzado y correcciones realizados; cobertura global combinada 80,8 % mediante `scripts/coverage.sh`)

---

## Dependencies & Execution Order

### Dependencias entre fases

- **Setup (F1)** → **Foundational (F2)** → historias.
- **US1 (F3)** es el MVP y **no depende** de otras historias.
- **US2 (F4)** depende de F2 (consola para el flujo interactivo, T045) y reutiliza `resolveGlobalBinary` y la identificación de copias de US1 (T016) para el inventario del binario. Su parte no interactiva (T026–T044) solo necesita F2.
- **US4 (F5)** depende de T018 (US1) para integrar el aviso en `session-start`. Todo lo demás es independiente.
- **US3 (F6)** depende de F2 (consola) y de T062 (checksum de US4) para el flujo de `update`. La instalación guiada (T070–T074) solo necesita F2.
- **Polish (F7)** al final.

### Dentro de cada historia

Pruebas (y ver que fallan) → dominio y puertos → adaptadores secundarios → casos de uso → comandos y hook → documentación. Las tareas ⚠ AUTORIZACIÓN van después de la implementación que cambia el comportamiento y **antes** de declarar el checkpoint.

## Parallel Execution Examples

- **F2**: T004, T005 y T006 en paralelo; después T007 ∥ T008 ∥ T009.
- **US1**: T010–T015 (seis pruebas en archivos distintos) en paralelo; T022 en paralelo con T016–T021.
- **US2**: T026–T033 en paralelo; T034 ∥ T035 ∥ T037 ∥ T038 ∥ T042.
- **US4**: T049–T054 en paralelo; T055 y después T056 ∥ T057 ∥ T058.
- **US3**: T066–T069 en paralelo; T076 ∥ T077 en cualquier momento.

## Implementation Strategy

1. **MVP = US1** (F1 + F2 + F3): cierra la causa raíz que se reportó (copias en 2.8.0–2.16.9 con el global en 2.26.4). Se publica como v2.27.0, y la persona valida en su máquina que las 6 copias desaparecen.
2. **US2**: corrige el defecto de privacidad de `uninstall` y el de `export`. Release.
3. **US4**: aviso de versión y checksum. A partir de aquí, cada release llega sola a la persona. Release.
4. **US3**: la capa guiada al estilo skills.sh sobre flujos que ya funcionan sin ella. Release.

En cada release: validación en el sistema real (reglas de trabajo 1–3), CHANGELOG, badge de versión y recuento de tools MCP si cambia (memoria 275).

## Phase 8: Convergence

- [X] T084 Hacer que `mem update` termine con un resumen de ✓/⚠ por paso (versión, descarga, checksum, sustitución, retirada de la copia local, refresco de la integración), incluidas las salidas por fallo, con `console.NewReporter` como `install`; prueba de contrato roja→verde en `tests/contract/lifecycle_update_test.go` y cambio en `adapters/primary/cli/cmd_update.go` per FR-024 (partial)
- [X] T085 Hacer que `installBinary` devuelva el resultado de la retirada de la copia local y que el paso "Binario" del resumen de `mem install` incluya la ruta, la versión retirada y la global (✓), o ⚠ con la acción manual si la retirada falla; prueba roja→verde y cambio en `adapters/primary/cli/cmd_install.go` per FR-004a (partial)
