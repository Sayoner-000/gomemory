---
description: "Lista de tareas de la feature 020 — benchmark de tokens por sesión"
---

# Tasks: Benchmark de tokens por sesión (`mem usage`) y tres optimizaciones validadas con esa medición

**Input**: documentos de diseño en `/specs/020-token-usage-benchmark/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: **OBLIGATORIOS**. El Principio III de la constitución declara TDD «NO NEGOCIABLE»: el
test se escribe primero, falla, y solo entonces se implementa. Cada tarea de test lleva la marca ⚠
y debe quedar en rojo antes de pasar a la siguiente.

**Organización**: por historia de usuario, para que cada una se implemente y se pruebe por separado.

## Formato: `[ID] [P?] [Story] Descripción`

- **[P]**: paralelizable (archivo distinto, sin dependencias pendientes)
- **[Story]**: a qué historia pertenece (US1…US5). Setup, Foundational y Polish no llevan etiqueta
- Cada tarea indica la ruta exacta del archivo

## Convención de rutas de este repositorio

Arquitectura hexagonal, sin `src/`. Los tests viven junto a su paquete con sufijo `_test.go`:

```
domain/ · application/{ports,usecases}/ · adapters/{primary,secondary}/ · infrastructure/
```

---

## Phase 1: Setup

**Propósito**: dejar constancia del punto de partida antes de tocar nada.

- [X] T001 Ejecutar `go build ./... && go vet ./... && go test ./...` y anotar el resultado en verde como línea base del trabajo, en la nota de la sesión
- [X] T002 [P] Crear el directorio `adapters/secondary/usage/` para el adaptador de grabación
- [X] T003 [P] Compilar el binario con `go build -o mem .` (el repositorio no lo versiona) para poder hacer las validaciones en vivo de `quickstart.md`

---

## Phase 2: Foundational (Prerrequisitos bloqueantes)

**Propósito**: dominio, puertos, persistencia y cableado del registro de uso.

**⚠️ CRÍTICO**: ninguna historia puede empezar hasta que esta fase esté completa. US1, US2, US3 y
US4 dependen toda de ella.

### Tests primero (Principio III)

- [X] T004 [P] ⚠ Escribir `domain/usage_test.go` en rojo: `Saved()` cumple `baseline − saved == emitted`, `ReductionRatio()` devuelve 0 cuando `BaselineTokens == 0`, y `WindowRatio()` solo es válido con `WindowTokens > 0`
- [X] T005 [P] ⚠ Escribir `adapters/secondary/persistence/usage_test.go` en rojo reusando `openTestDB`: cobertura de las cuatro operaciones del repositorio y del caso «sin filas devuelve slice vacío, no error»
- [X] T006 [P] ⚠ Escribir `adapters/secondary/usage/recorder_test.go` en rojo: la etiqueta de canal viaja desde la construcción, un fallo de persistencia no propaga error, y un grabador en nulo no rompe a quien emite

### Dominio y puertos

- [X] T007 Definir `domain.UsageRecord` y las constantes de operación (`build_context`, `search_memories`, `list_memories`, `get_memory`, `build_pack`, `compress_pack`, `plan_context`, `save_memory`, `other`) en `domain/usage.go`, sin imports de infraestructura
- [X] T008 Definir `domain.UsageReport`, `domain.UsageBucket` y los métodos `Saved()`, `ReductionRatio()` y `WindowRatio()` en `domain/usage.go` (depende de T007)
- [X] T009 [P] Declarar `ports.UsageRepository` con `Record` / `BySession` / `Sessions` / `Totals` en `application/ports/usage_repository.go` (depende de T008)
- [X] T010 [P] Declarar `ports.UsageRecorder` con `Record(operation string, baselineTokens, emittedTokens int)` —sin canal en la firma y sin devolver error— en `application/ports/usage_recorder.go` (depende de T008)

### Persistencia

- [X] T011 Añadir a `migrate()` en `adapters/secondary/persistence/db.go` la tabla `usage_records` con `CREATE TABLE IF NOT EXISTS`, `created_at` con la constante `Now`, `channel TEXT NOT NULL` **sin `CHECK`** y sin clave foránea a `sessions` (depende de T009)
- [X] T012 Añadir en el mismo `migrate()` los índices `idx_usage_project_session` e `idx_usage_created` con `CREATE INDEX IF NOT EXISTS` (depende de T011)
- [X] T013 Implementar las cuatro operaciones del puerto con parámetros bind en `adapters/secondary/persistence/usage.go` (depende de T012)
- [X] T014 Añadir a `adapters/secondary/persistence/db_test.go` un test que ejecute `migrate()` dos veces sobre la misma base y verifique que termina sin error y sin alterar las tablas previas (SC-011) (depende de T013)

### Adaptador de grabación y cableado

- [X] T015 Implementar `NewRecorder(repo, counter, project, channel, session)` en `adapters/secondary/usage/recorder.go`, que traga cualquier error de persistencia (FR-006) y resuelve el identificador de sesión al momento de registrar (depende de T013)
- [X] T016 [P] Añadir el campo `UsageWindowTokens int` con etiqueta `usage_window_tokens,omitempty` y valor por defecto 0 a `ports.SettingsData` en `application/ports/settings_repository.go`, siguiendo el patrón declarativo de los ajustes existentes
- [X] T017 [P] Añadir a `adapters/secondary/persistence/settings_test.go` la comprobación de que `UsageWindowTokens` ausente se lee como 0 (depende de T016)
- [X] T018 Añadir los campos `UsageRepo ports.UsageRepository` y `UsageRecorder ports.UsageRecorder` —ambos opcionales, admiten nulo— a `Container` en `infrastructure/container.go` y a `Deps` en `adapters/primary/cli/deps.go` (depende de T015)
- [X] T019 Cablear en `infrastructure/container.go` el repositorio y el grabador, fijando la etiqueta de canal según el comando en ejecución (`mcp`, `cli`, `tui`) — **único lugar donde se nombra un canal** (depende de T018)

**Checkpoint**: el registro de uso existe, se persiste y está cableado. Las historias pueden empezar.

---

## Phase 3: User Story 1 - Saber cuánto ahorró gomemory en esta sesión (Priority: P1) 🎯 MVP

**Goal**: `mem usage` imprime, para la sesión activa, línea base, emitido, ahorro, porcentaje y
desglose por operación y por canal, con la cabecera que declara el método de conteo.

**Independent Test**: ejecutar una secuencia conocida de emisiones y comprobar que el reporte las
muestra, con línea base mayor que lo emitido en las que optimizan y con `baseline − saved == emitted`.

### Tests primero (Principio III) ⚠

- [X] T020 [P] [US1] ⚠ Escribir en `application/usecases/build_context_test.go` un test en rojo: con un `Budget` bajo, los contadores dan línea base mayor que lo emitido, **sin construir el texto dos veces**
- [X] T021 [P] [US1] ⚠ Escribir `application/usecases/build_usage_report_test.go` en rojo: agregación por operación y por canal, y las garantías G1 a G4 de `contracts/usage-report.md`
- [X] T022 [P] [US1] ⚠ Escribir `adapters/primary/cli/cmd_usage_test.go` en rojo reusando `captureStdout`: la cabecera declara el conteo aproximado; con ventana 0 no aparece la línea de porcentaje; con ventana mayor que 0 aparece rotulada «(estimado)»
- [X] T023 [P] [US1] ⚠ Escribir `adapters/primary/cli/cmd_mcp_schemas_test.go` en rojo: levantar el servidor real con `mcp.NewInMemoryTransports()`, pedir la lista de operaciones y comprobar que devuelve 19 descriptores con un costo en tokens mayor que cero
- [X] T024 [P] [US1] ⚠ Escribir en `application/usecases/build_context_test.go` un test en rojo que invoque el caso de uso **sin pasar por MCP** y verifique que deja registro (corrección V1 de `research.md §1`)

### Instrumentación: capturar la línea base

- [X] T025 [US1] Añadir los contadores `rawChars` y `finalChars` al struct `Builder` de `application/usecases/build_context.go:87` e incrementarlos en `acota()` (línea 122, línea base = `len(m.Content)` íntegro) y en `fits()` (línea 135, la línea base suma aunque no quepa)
- [X] T026 [US1] Sumar a la línea base también los registros de actividad descartados por el tope `i >= 5` de `application/usecases/build_context.go:302` — es el mayor de los tres puntos de descarte y el brief no lo contemplaba (depende de T025)
- [X] T027 [US1] Añadir el campo opcional `Recorder ports.UsageRecorder` al `Builder` y llamar a `Record(domain.OpBuildContext, raw, final)` al final de `Build()`, convirtiendo caracteres a tokens con `deps.TokenCounter` — **sin tocar `ports.ContextBuilder`**, que solo expone `Build()`/`WriteFile()` (depende de T026)
- [X] T028 [P] [US1] Instrumentar la emisión de búsqueda y listado en `adapters/primary/cli/cmd_mcp_render.go` (`renderSearchResults`, `renderMemoryList`), reportando el contenido íntegro como línea base y lo entregado como emitido (depende de T019)
- [X] T029 [P] [US1] Añadir el campo opcional `Recorder ports.UsageRecorder` a `ContextRequest` y reportar `pack.Stats.RawTokens` / `pack.Stats.FinalTokens` desde `application/usecases/build_context_pack.go:206-209`, sin cambiar la firma de `BuildContextPack` (depende de T019)
- [X] T030 [P] [US1] Reportar el resultado de compresión desde el camino de `pack_compress` en `adapters/primary/cli/cmd_pack.go` (depende de T019)

### Costo de los descriptores publicados

- [X] T031 [US1] Implementar en `adapters/primary/cli/cmd_mcp_schemas.go` la medición del costo: levantar el servidor real sobre un transporte en memoria, pedir la lista de operaciones, serializar cada descriptor a JSON y contarlo con `ports.TokenCounter`, devolviendo total y desglose por operación (depende de T023)

### Reporte y salidas

- [X] T032 [US1] Implementar `usecases.BuildUsageReport(repo, counter, project, sessionID, window)` en `application/usecases/build_usage_report.go`, devolviendo `domain.UsageReport` ordenado descendente por línea base (depende de T021)
- [X] T033 [US1] Añadir en el middleware de `adapters/primary/cli/cmd_mcp.go:46-56` la etiqueta de canal y el registro de respaldo para las operaciones que no optimizan, con línea base igual a lo emitido (FR-005), consultando la marca de «ya reportado» en el contexto de la petición para no contar dos veces (depende de T027)
- [X] T034 [US1] Implementar `FormatUsageReport(report)` en `adapters/primary/cli/cmd_usage.go` con la cabecera que declara el método de conteo, y la línea de ventana rotulada «(estimado)» solo si el ajuste es mayor que 0 (depende de T032)
- [X] T035 [US1] Implementar `CmdUsage(deps, args)` con `--session`, `--all` y `--json` en `adapters/primary/cli/cmd_usage.go`, respetando el esquema de `contracts/usage-report.md` — **no renombrar la función `Usage()` de `cli.go`**, que es el texto de ayuda (depende de T034)
- [X] T036 [US1] Registrar el caso `"usage"` en `adapters/primary/cli/dispatcher.go` y añadir su entrada —más la de `doctor`, que falta— al texto de `Usage()` en `adapters/primary/cli/cli.go` (depende de T035)
- [X] T037 [US1] Ejecutar la validación V7 de `quickstart.md`: emitir por el canal de línea de comandos con el servidor MCP apagado y comprobar con `git diff --stat` que los cinco archivos del criterio de agnosticismo quedaron intactos (SC-005) (depende de T036)

**Checkpoint**: `mem usage` funciona por sí solo y entrega la línea base que justifican US3 y US4.

---

## Phase 4: User Story 2 - La pantalla de uso en la interfaz interactiva (Priority: P1)

**Goal**: una pantalla propia con dos secciones: el reporte de la sesión activa y un snapshot
puntual efímero. Absorbe íntegra la spec 017.

**Independent Test**: entrar con `u`, comprobar que la sección [1] coincide con `mem usage`, y que
la sección [2] calcula un snapshot que no reaparece al salir y volver a entrar.

### Tests primero (Principio III) ⚠

- [X] T038 [P] [US2] ⚠ Escribir en `adapters/primary/tui/tui_test.go` un test en rojo siguiendo `newConfigTestModel`: la tecla `u` abre `screenUsage` y la sección [1] renderiza las mismas cifras que `FormatUsageReport`
- [X] T039 [P] [US2] ⚠ Escribir un test en rojo en `adapters/primary/tui/tui_test.go` que compruebe que la pantalla cabe en la altura del terminal, reusando `bodyBudget` y `windowLines`
- [X] T040 [P] [US2] ⚠ Escribir un test en rojo que verifique que un snapshot calculado no sobrevive a salir y volver a entrar a la pantalla (FR-023, SC-007)

### Implementación

- [X] T041 [US2] Añadir `screenUsage` **al final** del `enum screen` de `adapters/primary/tui/tui.go:24-38`, tras `screenEditSetting`, más los campos con prefijo `usage*` en el modelo (depende de T034)
- [X] T042 [US2] Añadir la tecla `u` en `updateList` de `adapters/primary/tui/tui.go` con guarda de disponibilidad, el `if` correspondiente en `Update` y el `case` en `View` — `u` está libre; ocupadas están `q / j k enter s a m c o` (depende de T041)
- [X] T043 [US2] Implementar `usageView()` sección [1] en `adapters/primary/tui/tui.go`, reusando `FormatUsageReport` y los estilos existentes (`titleStyle`, `backHint()`) (depende de T042)
- [X] T044 [US2] Implementar la sección [2] en `adapters/primary/tui/tui.go`: campos de tarea y presupuesto, disparo de `usecases.BuildContextPack` y render con `FormatContextStats` de `adapters/primary/cli/cmd_pack.go:219`, sin duplicar ni el motor ni el formato (depende de T043)
- [X] T045 [US2] Validar la entrada del snapshot en `adapters/primary/tui/tui.go`: tarea no vacía y presupuesto entero positivo, con mensaje claro (FR-021) (depende de T044)
- [X] T046 [US2] Emitir un mensaje comprensible —no un error crudo— cuando el contenido imprescindible exceda el presupuesto indicado (FR-022) (depende de T045)
- [X] T047 [US2] Limpiar el estado del snapshot al salir de la pantalla, para que no reaparezca ni se acumule (FR-023) (depende de T046)

**Checkpoint**: la fase A está completa. Los dos artefactos pedidos existen y producen datos.

---

## Phase 5: User Story 5 - Una interfaz interactiva sobre una base vigente (Priority: P3)

**Goal**: subir la librería de la interfaz interactiva a la línea v1 vigente sin cambiar el
comportamiento observable de ninguna pantalla.

**Nota de secuencia**: es P3 por prioridad, pero se ejecuta aquí, dentro de la fase A, porque toca
la misma superficie que US2 y verificar la interfaz una sola vez cuesta menos que verificarla dos.
No depende de US3 ni de US4.

**Independent Test**: actualizar, correr la suite completa sin modificar ninguna prueba, y recorrer
a mano las pantallas previas.

- [X] T048 [US5] Actualizar en `go.mod` `charmbracelet/bubbletea` a v1.3.10, `charmbracelet/bubbles` a v1.0.0 y `charmbracelet/lipgloss` a v1.1.0 con `go get`, y correr `go mod tidy` — **no migrar a `bubbletea/v2`**: cambia la ruta del módulo y rompe la interfaz de programación (depende de T047)
- [X] T049 [US5] Ejecutar `go build ./... && go vet ./... && go test ./...` y confirmar que pasa **sin haber modificado ninguna prueba existente** (SC-012, Principio III) (depende de T048)
- [X] T050 [US5] Recorrer a mano con `./mem tui` las pantallas previas —lista, detalle, guardar, mantenimiento, configuración, importar, optimizar— y confirmar que ninguna cambió de aspecto ni de comportamiento (FR-039) (depende de T049)

**Checkpoint**: fase A entregable y cerrada. **Corte natural si el trabajo debe pausarse aquí.**

---

## Phase 6: User Story 3 - Dejar de pagar N veces por el mismo tópico (Priority: P2)

**Goal**: consolidar grupos de memorias redundantes, con previsualización obligatoria, y demostrar
la reducción con el benchmark de la fase A.

**Independent Test**: partir de varias memorias del mismo grupo, consolidar, comprobar que queda
una sin pérdida de contenido, y que la línea base de la emisión de contexto baja de forma medible.

- [X] T051 [US3] ⚠ **PUERTA DE DECISIÓN — no atómica**: confirmar con el usuario la ampliación del criterio de agrupación documentada en Complexity Tracking de `plan.md`. Medido sobre la base real hay **0 grupos de `topic_key` con más de una fila**, así que consolidar solo por tópico da Δ cero y FR-030/SC-008 quedan incumplibles. El Δ observable está en los registros automáticos de actividad, con 55 % de duplicados literales. **No continuar con T052 sin esa confirmación**
- [X] T052 [P] [US3] ⚠ Escribir `application/usecases/consolidate_memories_test.go` en rojo: N memorias del mismo grupo quedan reducidas a 1 sin perder contenido, y las memorias sin criterio de agrupación no se tocan (FR-029) (depende de T051)
- [X] T053 [US3] Implementar `usecases.ConsolidateMemories(repo, project, criterio, dryRun)` en `application/usecases/consolidate_memories.go`, fundiendo cada grupo en su fila más reciente y eliminando el resto, con `dryRun` como valor por defecto (FR-027) (depende de T052)
- [X] T054 [US3] Implementar el criterio de agrupación por clave de tópico (`project + topic_key`, FR-026) en `application/usecases/consolidate_memories.go` (depende de T053)
- [X] T055 [US3] Implementar el criterio de agrupación por contenido idéntico para registros automáticos de actividad (`project + type + hash(content)`) en `application/usecases/consolidate_memories.go` — es el que hace medible el Δ de FR-030 (depende de T054)
- [X] T056 [US3] Exponer la operación como subcomando de `mem gc` en `adapters/primary/cli/cmd_gc.go`, con previsualización por defecto y `--apply` para confirmar (depende de T055)
- [X] T057 [US3] Añadir la acción equivalente como fila de la pantalla de mantenimiento en `adapters/primary/tui/tui.go`, con la misma confirmación previa (FR-028) (depende de T056)
- [X] T058 [US3] Ejecutar la validación V9 de `quickstart.md`: medir la línea base de la emisión de contexto antes y después de aplicar, y comprobar que baja de forma verificable (SC-008) (depende de T057)

**Checkpoint**: la fase B queda demostrada con un Δ medido, no argumentado.

---

## Phase 7: User Story 4 - Recibir un índice y pedir el detalle solo cuando hace falta (Priority: P3)

**Goal**: emisión de contexto en modo índice —protocolo íntegro más una línea por memoria—, con
detalle bajo demanda y vuelta atrás sin residuos.

**Independent Test**: activar el modo, comprobar que la salida contiene todos los identificadores y
ningún cuerpo, que el protocolo va completo, y medir la diferencia contra el modo completo.

### Tests primero (Principio III) ⚠

- [X] T059 [P] [US4] ⚠ Escribir en `application/usecases/build_context_test.go` un test en rojo: en modo índice la salida contiene todos los identificadores y **ningún cuerpo de memoria** (SC-009)
- [X] T060 [P] [US4] ⚠ Escribir un test de regresión en rojo en `application/usecases/build_context_test.go`: el protocolo de trabajo **nunca** se recorta en modo índice (FR-032)
- [X] T061 [P] [US4] ⚠ Escribir un test en rojo de reversibilidad: activar el modo índice y desactivarlo devuelve una emisión idéntica a la de partida (SC-010)

### Implementación

- [X] T062 [US4] Añadir al `Builder` de `application/usecases/build_context.go` el modo índice: protocolo íntegro más una línea por memoria con identificador, tipo y título, sin contenido (depende de T059)
- [X] T063 [US4] Añadir el campo `ContextIndexMode bool` con etiqueta `context_index_mode,omitempty` y valor por defecto `false` —comportamiento actual— a `ports.SettingsData` en `application/ports/settings_repository.go`, y cablearlo en `infrastructure/container.go` (depende de T062)
- [X] T064 [US4] Emitir un índice vacío explícito, y no una sección ausente, cuando no haya memorias que listar (caso borde de la spec) (depende de T062)
- [X] T065 [US4] Confirmar que el detalle se recupera con la lectura por identificador ya existente en los tres canales, sin introducir ninguna capacidad nueva, y dejarlo asentado en `contracts/usage-recorder.md` si hiciera falta precisarlo (FR-033) (depende de T062)
- [X] T066 [US4] Ejecutar la validación V10 de `quickstart.md`: medir con `mem usage` la diferencia de la **primera emisión de contexto de la sesión** entre ambos modos, y dejarla registrada como medición, no como estimación (FR-035) (depende de T063)

**Checkpoint**: las tres fases entregadas, cada una con su Δ medido.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T067 Ejecutar `go test -cover` sobre `domain/`, `application/`, `adapters/secondary/persistence/` y `adapters/secondary/usage/` y confirmar cobertura ≥ 80 % (Principio III)
- [X] T068 Recorrer las validaciones V1 a V11 de `quickstart.md` contra el binario y la base reales, no solo contra los tests (regla de campo 2 del proyecto)
- [X] T069 [P] Actualizar `README.md` y `docs/MANUAL.md` con `mem usage` y la pantalla de uso, en lenguaje neutral y sin ejemplos anclados a un agente concreto
- [X] T070 [P] Actualizar `AGENTS.md` e `INSTALLATION.md` con lo mismo — quedaron fuera de la sincronización de documentación anterior, así que conviene revisarlos con atención
- [X] T071 [P] Publicar una copia de `contracts/usage-report.md` fuera de `specs/`, en `docs/`, para que cualquier consumidor lea el contrato sin leer el código
- [X] T072 Marcar en `specs/017-context-snapshot-tui/spec.md` la trazabilidad definitiva: sus FR-001 a FR-008 quedaron cubiertos por FR-018 a FR-025 de esta feature
- [X] T073 ⚠ **No atómica**: publicar el release v2.8.0 en el remoto `github` (no `origin`). La versión, el momento y el remoto los decide el usuario y exigen su confirmación explícita. Sin trailer de coautoría en el commit

---

## Dependencies & Execution Order

### Dependencias entre fases

- **Setup (Phase 1)**: sin dependencias
- **Foundational (Phase 2)**: depende del Setup. **BLOQUEA todas las historias**
- **US1 (Phase 3)**: depende de Foundational. Es el MVP
- **US2 (Phase 4)**: depende de Foundational y de T034 (`FormatUsageReport`), que reutiliza
- **US5 (Phase 5)**: depende de US2 por conveniencia de verificación, no por técnica
- **US3 (Phase 6)**: depende de US1, que le aporta el instrumento para demostrar su Δ (FR-030)
- **US4 (Phase 7)**: depende de US1 por la misma razón (FR-035)
- **Polish (Phase 8)**: depende de las historias que se decidan entregar

### Dependencias entre historias

| Historia | Puede empezar tras | Motivo de la dependencia |
|---|---|---|
| US1 (P1) | Phase 2 | Ninguna otra |
| US2 (P1) | Phase 2 + T034 | Reutiliza el formateador; no duplica su lógica |
| US5 (P3) | US2 | Comparte superficie: verificar la interfaz una vez, no dos |
| US3 (P2) | US1 | FR-030 exige un Δ **medido**, y el instrumento lo entrega US1 |
| US4 (P3) | US1 | FR-035, misma razón |

Ni US3 ni US4 dependen entre sí: si se decide entregar solo una, la otra no queda bloqueada.

### Dentro de cada historia

- Los tests marcados ⚠ se escriben primero y deben **fallar** antes de implementar
- Dominio antes que puertos, puertos antes que adaptadores, adaptadores antes que cableado
- El cableado ocurre en un solo lugar: `infrastructure/container.go`

### Oportunidades de paralelización

- **Phase 1**: T002 y T003 en paralelo
- **Phase 2**: los tres tests (T004, T005, T006) en paralelo; luego T009 y T010; luego T016 y T017
- **US1**: los cinco tests (T020–T024) en paralelo; en implementación, T028, T029 y T030 en paralelo
- **US2**: los tres tests (T038, T039, T040) en paralelo
- **US4**: los tres tests (T059, T060, T061) en paralelo
- **Polish**: T069, T070 y T071 en paralelo

---

## Parallel Example: User Story 1

```bash
# Los cinco tests de US1, todos en rojo, a la vez (archivos distintos):
Task: "Contadores de línea base en application/usecases/build_context_test.go"
Task: "Agregación y garantías G1–G4 en application/usecases/build_usage_report_test.go"
Task: "Cabecera y línea de ventana en adapters/primary/cli/cmd_usage_test.go"
Task: "19 descriptores por transporte en memoria en adapters/primary/cli/cmd_mcp_schemas_test.go"
Task: "Registro sin pasar por MCP en application/usecases/build_context_test.go"

# La instrumentación de los emisores independientes, a la vez:
Task: "Búsqueda y listado en adapters/primary/cli/cmd_mcp_render.go"
Task: "Estadísticas de paquete en application/usecases/build_context_pack.go"
Task: "Resultado de compresión en adapters/primary/cli/cmd_pack.go"
```

---

## Implementation Strategy

### MVP primero (solo US1)

1. Phase 1 — Setup
2. Phase 2 — Foundational (**crítico**: bloquea todo)
3. Phase 3 — US1
4. **PARAR Y VALIDAR**: V1, V2, V3, V4, V6 y V7 de `quickstart.md`
5. `mem usage` ya responde la pregunta original con datos reales

### Entrega incremental

1. Setup + Foundational → cimientos listos
2. US1 → validar → **MVP**
3. US2 → validar → los dos artefactos pedidos existen
4. US5 → validar → **fin de la fase A, corte natural para pausar**
5. US3 → validar → fase B con Δ medido *(requiere la puerta T051)*
6. US4 → validar → fase C con Δ medido

### Corte recomendado

La fase A (Phases 1 a 5, tareas T001–T050) es **autónoma y entregable**: cubre US1, US2 y US5,
entrega `mem usage` y la pantalla de uso, y produce exactamente los datos que justifican si vale la
pena hacer B y C. Si el trabajo se hace largo, ese es el punto donde conviene parar y mirar los
números antes de seguir.

---

## Notes

- Las tareas marcadas ⚠ son de test —van primero y deben fallar— salvo T051 y T073, que son las dos
  no atómicas y llevan su motivo declarado
- Las tareas marcadas [P] tocan archivos distintos y no dependen entre sí
- Los tests existentes son intocables sin autorización explícita (Principio III)
- Comprometer después de cada tarea o grupo lógico, **sin trailer de coautoría**
- Se puede parar en cualquier checkpoint para validar la historia por separado

## Resumen

| Fase | Tareas | Historia | Fase de entrega |
|---|---|---|---|
| 1 · Setup | T001–T003 | — | A |
| 2 · Foundational | T004–T019 | — | A |
| 3 · US1 | T020–T037 | US1 (P1) | A · **MVP** |
| 4 · US2 | T038–T047 | US2 (P1) | A |
| 5 · US5 | T048–T050 | US5 (P3) | A |
| 6 · US3 | T051–T058 | US3 (P2) | B |
| 7 · US4 | T059–T066 | US4 (P3) | C |
| 8 · Polish | T067–T073 | — | — |

**73 tareas** · 71 ejecutables · 2 no atómicas declaradas (T051 puerta de decisión, T073 release) ·
27 paralelizables · 17 de test, de las cuales 15 llevan la marca ⚠ de «escribir en rojo primero».
