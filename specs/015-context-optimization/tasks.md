---

description: "Lista de tareas para implementar Context Optimization & Budgeting"
---

# Tasks: Context Optimization & Budgeting

**Input**: Documentos de diseño de `/specs/015-context-optimization/`
**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: OBLIGATORIOS, no opcionales — Constitución Principio III ("Testing First,
NO NEGOCIABLE"): tests primero, deben fallar, solo entonces se implementa
(Red-Green-Refactor). Cada tarea de implementación depende de su tarea de test
correspondiente, ya listada antes en el mismo bloque.

**Organización**: agrupadas por historia de usuario (spec.md) para poder implementar y
probar cada una de forma independiente.

## Formato: `[ID] [P?] [Story] Descripción`

- **[P]**: puede correr en paralelo (archivo distinto, sin dependencia de una tarea sin terminar)
- **[Story]**: US1–US5, mapea a las historias de spec.md
- Rutas de archivo exactas en cada descripción, todas relativas a la raíz del repo

## Convenciones de ruta

Módulo Go único ya existente (`mem`), arquitectura hexagonal — sin `src/` genérico:
`domain/`, `application/{ports,usecases}/`, `adapters/{primary,secondary}/`,
`infrastructure/`, `tests/{unit,integration,contract}/`. Ver plan.md "Estructura del
Proyecto" para el árbol completo.

---

## Fase 1: Setup

**Propósito**: línea base antes de tocar código.

- [X] T001 Ejecutar `go build ./...` y `go test ./...` en la raíz del repo y confirmar que el estado actual (sin esta feature) está verde — línea base para comparar contra T035 al final (FR-018/SC-005, no-regresión)

**Checkpoint**: línea base confirmada, ningún cambio de código todavía.

---

## Fase 2: Foundational (bloqueante para todas las historias)

**Propósito**: dominio, puertos y adaptadores que reusan TODAS las historias.

**⚠️ CRÍTICO**: ninguna historia de usuario empieza hasta que esta fase esté completa.

- [X] T002 [P] Crear `domain/context_pack.go`: `Priority` (`PriorityCritical`/`PriorityRelevant`/`PriorityOptional`), `ContextItem`, `ContextPack`, `ContextStats`, `ErrCriticalContextOverflow`, `ErrInvalidContextRequest` — campos y comentarios exactos según data-model.md (errores agregados a `domain/errors.go`, consistente con el archivo existente)
- [X] T003 [P] Escribir tests de tabla (deben fallar) en `adapters/secondary/tokens/approximate_test.go`: mismo input → mismo output dos veces seguidas (determinismo, SC-006), conteo proporcional a longitud, string vacío → 0 tokens
- [X] T004 [US-foundational] Implementar `application/ports/token_counter.go` (interfaz `TokenCounter`) y `adapters/secondary/tokens/approximate.go` (`ApproximateTokenCounter`) hasta que T003 pase (depende de: T003)
- [X] T005 [P] Escribir tests de tabla (deben fallar) en `adapters/secondary/compression/compression_test.go`: `NoopCompressor` no toca el contenido (`Tokens == RawTokens`, `Compressed == false`); `StructuralCompressor` colapsa whitespace repetido y párrafos/encabezados duplicados idénticos; preserva intacto bloques ```` ``` ````, URLs, rutas de archivo, valores numéricos, números de versión y líneas de mensaje de error (caso de ejemplo: un bloque con `POST /v1/auth/refresh`, `30`, `15`, `auth:refresh` debe conservar los cuatro tras comprimir); mismo input comprimido dos veces produce el mismo output (SC-006)
- [X] T006 Implementar `application/ports/compressor.go` (`Compressor`, `CompressionLevel`, `CompressionOptions`, `CompressionResult`) y `adapters/secondary/compression/{noop.go,structural.go}` hasta que T005 pase (depende de: T005)
- [X] T007 [P] Agregar a `SettingsData` en `application/ports/settings_repository.go` los campos de configuración de FR-019 (presupuesto por defecto, relevancia mínima, máximo de candidatos, dedup on/off, compresión on/off) — inclusión de Spec Kit reusa `SpeckitContextDisabled` ya existente (research.md §7), mismo patrón opt-out `...Disabled`/`0→default,negativo→opt-out` que el resto de `SettingsData` ya usa. También reflejado en `adapters/secondary/persistence/settings.go` (struct + defaults + `applyContextDefaults`) y en el mapeo de `adapters/secondary/persistence/repositories.go`
- [X] T008 Extender `adapters/secondary/persistence/settings_test.go` para verificar defaults, opt-out negativo, y que los campos nuevos de T007 sobreviven un ciclo `Write`→`Read` sobre `.memory/settings.json` (depende de: T007)

**Checkpoint**: dominio + puertos de compresión/tokens + configuración listos — las historias de usuario pueden empezar.

---

## Fase 3: Historia de Usuario 1 - Paquete de contexto acotado a una tarea y a un presupuesto (Prioridad: P1) 🎯 MVP

**Goal**: dado tarea + presupuesto de tokens, producir un `ContextPack` que solo
incluya lo relevante y nunca exceda el presupuesto, fallando explícito si lo crítico
no cabe.

**Independent Test**: con memorias variadas en un proyecto, `mem pack build --task ... --max-tokens N` devuelve solo lo relevante, con `TokenCount <= N`; con un presupuesto absurdamente bajo, falla con `ErrCriticalContextOverflow` en vez de devolver un paquete parcial.

### Tests para la Historia 1 (escribir primero, deben fallar)

- [X] T009 [P] [US1] Test de integración en `tests/integration/build_context_pack_test.go`: sembrar SQLite con memorias relevantes e irrelevantes a una tarea fija, llamar `BuildContextPack`, verificar que las relevantes entran, las irrelevantes no, `TokenCount <= MaxTokens`, y cada `ContextItem.ID` traza de vuelta a la memoria de origen (spec.md US1, escenarios 1 y 3)
- [X] T010 [P] [US1] Test unitario en `application/usecases/build_context_pack_test.go`: `Task`/`Project` vacíos o `MaxTokens <= 0` → `ErrInvalidContextRequest` sin tocar ningún puerto; un conjunto de items `Critical` cuya suma de tokens excede `MaxTokens` → `ErrCriticalContextOverflow`, sin `ContextPack` parcial devuelto (spec.md US1 escenario 2, FR-008)
- [X] T011 [P] [US1] Test de contrato en `tests/contract/pack_build_cli_test.go`: `ParsePackBuildFlags` exige `--task`/`--max-tokens`, default de `--project` al proyecto actual, `--no-speckit`/`--json` — se testea la función de parseo pura (sin `os.Exit`), mismo patrón que `ParsePurgeFlags`; el camino de salida real (código 0/1) queda cubierto manualmente en quickstart.md (T034), igual que el resto de comandos del CLI
- [X] T012 [US1] Implementar `application/usecases/build_context_pack.go`: tipo `ContextRequest`, función `BuildContextPack(memRepo ports.MemoryRepository, compressor ports.Compressor, counter ports.TokenCounter, req ContextRequest) (domain.ContextPack, error)` — valida en el borde, recupera candidatos vía `MemoryRepository.Search`, deriva relevancia del orden de resultados + `MemoryType` (research.md §2 — recencia quedó fuera de la fórmula v1 por simplicidad, ver comentario en el código), clasifica `Priority` (data-model.md, con la aclaración de que Critical por tipo nunca se degrada por relevancia baja), reserva presupuesto crítico primero (falla cerrado con `ErrCriticalContextOverflow` si no cabe), comprime `Relevant`/`Optional` vía `Compressor`, llena el resto del presupuesto por relevancia descendente, arma `ContextPack` + `ContextStats` (depende de: T009, T010, T011, T002, T004, T006)
- [X] T013 [P] [US1] Agregar `ToolPackBuild = "pack_build"` (y de una vez `ToolPackShow`/`ToolPackCompress`/`ToolPackStats`, usadas recién en T023) + el grupo `MCPContextPackTools` a `domain/mcp_tools.go`, incluidas en `MCPAllTools()` y `MCPAutoApprovableTools()` (ninguna tool de esta feature es destructiva)
- [X] T014 [US1] Implementar `adapters/primary/cli/cmd_pack.go`: `CmdPack` (despacha por subcomando, mismo patrón que `CmdSession`) + `cmdPackBuild` con flags `--task`, `--max-tokens`, `--project`, `--min-relevance`, `--max-items`, `--no-compress`, `--no-speckit`, `--json` (contracts/cli.md); registrar `case "pack":` en `adapters/primary/cli/dispatcher.go` (depende de: T012, T011)
- [X] T015 [US1] Registrar el handler MCP `pack_build` con `mcp.AddTool` dentro de `registerTools` en `adapters/primary/cli/cmd_mcp.go`, esquema de entrada/salida según contracts/mcp-tools.md (depende de: T012, T013)
- [X] T016 [P] [US1] Construir `TokenCounter`/`Compressor` concretos en el composition root `infrastructure/container.go` y agregar los campos correspondientes a `Deps` en `adapters/primary/cli/deps.go` (depende de: T004, T006)

**Nota de implementación**: `tests/contract/mcp_tool_sync_test.go` (ya existente, no anticipado en el plan) exige que TODA tool en `domain.MCPAllTools()` esté además: (a) realmente registrada en el servidor MCP, y (b) declarada con el prefijo `gomemory_` en `infrastructure/plugin/opencode/gomemory.ts`. Por eso T013 declaró las 4 constantes juntas — declarar solo `pack_build` y dejar las otras 3 sin registrar habría dejado ese test en rojo hasta T023. El registro real de los handlers `pack_show`/`pack_compress`/`pack_stats` en el servidor MCP y `gomemory.ts` se hizo en T023 (Historia 3), como estaba planeado.

**Checkpoint**: `mem pack build` funciona de punta a punta — MVP entregable.

---

## Fase 4: Historia de Usuario 2 - Deduplicación (Prioridad: P2)

**Goal**: colapsar memorias casi idénticas antes de gastar presupuesto en ellas.

**Independent Test**: dos memorias casi idénticas sobre el mismo tema → el paquete incluye solo una y `ContextStats.ItemsDuplicate` refleja la descartada.

### Tests para la Historia 2 (escribir primero, deben fallar)

- [X] T017 [P] [US2] Test de integración en `tests/integration/build_context_pack_dedup_test.go`: sembrar dos memorias casi idénticas (mismo `MemoryType`, contenido parafraseado) sobre el tema de la tarea, llamar `BuildContextPack`, verificar que el paquete resultante contiene solo una y `ContextStats.ItemsDuplicate == 1` (spec.md US2, ambos escenarios)

### Implementación para la Historia 2

- [X] T018 [US2] Integrar la función pura ya existente `DetectDuplicateGroups` (`application/usecases/detect_duplicates.go`, mismo paquete `usecases` — sin exportar nada nuevo) dentro de `BuildContextPack` (`application/usecases/build_context_pack.go`): corre sobre los candidatos recuperados antes de clasificar prioridad, conserva `SuggestedKeepID` por grupo, cuenta el resto en `ContextStats.ItemsDuplicate` (depende de: T012, T017; research.md §3)

**Checkpoint**: dedup verificada de punta a punta vía `mem pack build`.

---

## Fase 5: Historia de Usuario 3 - Observabilidad de la reducción (Prioridad: P3)

**Goal**: exponer cuánto se redujo el contexto y por qué, vía CLI/API/MCP, incluyendo compresión de texto suelto.

**Independent Test**: tras construir un paquete, `mem pack stats`/`--json` muestra raw/final/ahorrados/reducción e items por prioridad, consistentes entre sí; `mem pack compress` comprime texto arbitrario y reporta tokens antes/después.

### Tests para la Historia 3 (escribir primero, deben fallar)

- [X] T019 [P] [US3] Test unitario en `application/usecases/build_context_pack_test.go`: invariantes de `ContextStats` — `ItemsRetrieved == ItemsDuplicate + ItemsCritical + ItemsRelevant + ItemsOptional + ItemsDiscarded` y `RawTokens - SavedTokens == FinalTokens` (data-model.md, spec.md US3 escenario 1)
- [X] T020 [P] [US3] Test de contrato en `tests/contract/pack_stats_cli_test.go`: `ParseContextPackInput`/`FormatContextStats` sobre una fixture JSON de un `ContextPack` conocido imprimen raw/final/ahorrados/reducción y el desglose por prioridad exacto esperado (contracts/cli.md) — se testean las funciones puras, no el binario, mismo criterio que `ParsePurgeFlags`
- [X] T021 [P] [US3] Test de contrato en `tests/contract/pack_compress_cli_test.go`: `CompressText` sobre un texto con párrafos duplicados colapsa el contenido y reporta tokens antes/después (contracts/cli.md, spec.md US3 escenario 3)

### Implementación para la Historia 3

- [X] T022 [US3] Implementar `cmdPackShow`, `cmdPackStats`, `cmdPackCompress` + `ParseContextPackInput`/`FormatContextStats`/`CompressText` (exportadas y testeables) en `adapters/primary/cli/cmd_pack.go` — `show`/`stats` leen un `ContextPack` en JSON desde archivo o stdin (`-`), sin estado entre invocaciones (contracts/cli.md); `compress` corre `Compressor` directo sobre archivo/stdin sin retrieval ni budget (depende de: T019, T020, T021, T014)
- [X] T023 [US3] Agregar `ToolPackShow`, `ToolPackCompress`, `ToolPackStats` a `domain/mcp_tools.go` (grupo `MCPContextPackTools`, ya declaradas junto con `ToolPackBuild` en T013) y registrar sus handlers con `mcp.AddTool` en `registerTools` en `adapters/primary/cli/cmd_mcp.go`; también se actualizó `infrastructure/plugin/opencode/gomemory.ts` (las 4 tools `gomemory_pack_*` + mención en `MEMORY_PROTOCOL`), requerido por `tests/contract/mcp_tool_sync_test.go` que no estaba anticipado en el research/plan original (depende de: T022, T013)

**Checkpoint**: superficie completa de inspección (CLI + MCP) disponible.

---

## Fase 6: Historia de Usuario 4 - Ingesta de Spec Kit acotada a la feature activa (Prioridad: P4)

**Goal**: incluir automáticamente requisitos/decisiones/restricciones/dependencias de tareas de la feature Spec Kit activa, nunca de otras.

**Independent Test**: con dos features Spec Kit en el mismo proyecto, un paquete pedido para una tarea de una de ellas incluye solo el contenido de esa feature; `--no-speckit` no incluye nada de `specs/`.

### Tests para la Historia 4 (escribir primero, deben fallar)

- [X] T024 [P] [US4] Test unitario en `adapters/secondary/speckit/reader_test.go`: `ActiveFeature(root)` lee `.specify/feature.json` y devuelve el nombre de carpeta, o `""` sin error si no existe; `Read(root, feature, task)` extrae requisitos (de `spec.md`) recortados por relevancia léxica a `task`, sin mezclar una segunda feature fixture
- [X] T025 [P] [US4] Test de integración en `tests/integration/build_context_pack_speckit_test.go`: dos features Spec Kit fixture en disco, `BuildContextPack` con `IncludeSpecKit=true` incluye solo el contenido de la feature activa (vía `.specify/feature.json`) y ninguno de la otra; con `IncludeSpecKit=false` no incluye ningún item con ID `speckit:*` (spec.md US4, ambos escenarios)

### Implementación para la Historia 4

- [X] T026 [US4] Implementar `application/ports/speckit_reader.go` (interfaz `SpecKitReader`) y `adapters/secondary/speckit/reader.go` (`Reader`) hasta que T024 pase (depende de: T024). Se agregó también `domain.SpecKitFeatureContext` (no estaba en el `context_pack.go` de T002, se completó aquí)
- [X] T027 [US4] Integrar `SpecKitReader` en el pipeline de `BuildContextPack` (`application/usecases/build_context_pack.go`): nuevo parámetro `specKit ports.SpecKitReader` y campo `ContextRequest.Root` (necesario para que `SpecKitReader.ActiveFeature`/`Read` ubiquen `.specify/` y `specs/` en disco — no estaba en el data-model.md original, señal real encontrada al implementar: el caso de uso solo conocía `Project`, una clave de BD, no una ruta de filesystem); Requirements/Constraints entran como `Priority=Critical`, Decisions/TaskDependencies como `Priority=Relevant` (FR-005). `--no-speckit` ya estaba parseado desde T014; acá se conecta de verdad. Cambiar la firma de `BuildContextPack` rompió los call-sites de T009/T010/T017 (tests) y de `cmd_pack.go`/`cmd_mcp.go` — se actualizaron todos en el mismo paso (depende de: T012, T026, T025, T014)
- [X] T028 [US4] Construir `SpecKitReader` concreto (`speckit.Reader{}`) en `infrastructure/container.go` y agregar el campo a `Deps` en `adapters/primary/cli/deps.go`; `cmdPackBuild`/el handler MCP `pack_build` ahora pasan `deps.Root` en `ContextRequest.Root` (depende de: T026)

**Checkpoint**: alcance por feature de Spec Kit verificado de punta a punta.

---

## Fase 7: Historia de Usuario 5 - Optimización de descripciones de tools MCP (Prioridad: P5)

**Goal**: acortar la descripción de una tool sin tocar su nombre/parámetros/schema.

**Independent Test**: una `ToolDescriptor` con descripción verbosa optimizada mantiene `Name`/`Schema` idénticos byte a byte y `Description` más corta.

### Tests para la Historia 5 (escribir primero, deben fallar)

- [X] T029 [P] [US5] Test unitario en `application/usecases/optimize_tool_test.go`: `OptimizeToolDescription` sobre una `ToolDescriptor` con descripción verbosa (párrafo duplicado — lo único que `StructuralCompressor` v1 realmente acorta, no resumen semántico) devuelve `Name`/`Schema` idénticos byte a byte al input y `Description` más corta; con `Description` vacía no falla ni cambia nada (spec.md US5, ambos escenarios; contracts/go-api.md)

### Implementación para la Historia 5

- [X] T030 [US5] Implementar `application/usecases/optimize_tool.go` (`ToolDescriptor{Name, Description, Schema []byte}`, `OptimizeToolDescription(t ToolDescriptor, compressor ports.Compressor) (ToolDescriptor, error)`), usando el `Compressor` ya existente con `CompressionStructural` — sin lógica específica de MCP aquí; fallo de compresión devuelve el original sin error (FR-011) (depende de: T029, T006)

**Checkpoint**: las 5 historias de usuario están completas e independientemente verificables.

---

## Fase Final: Pulido y Cross-Cutting

**Propósito**: mejoras que afectan a varias historias a la vez.

- [X] T031 [P] `tests/contract/mcp_tool_sync_test.go` (ya existente, sin cambios de código) valida automáticamente las 4 tools nuevas contra el `tools/list` real del servidor porque compara contra `domain.MCPAllTools()` dinámicamente — no requirió tocar el test, solo mantenerlo verde a medida que se registraban los handlers (T015, T023) (depende de: T013, T015, T023)
- [X] T032 [P] `golangci-lint`/`gofumpt` no están instalados en este entorno; se usó `gofmt -l` (0 archivos de esta feature con diffs de formato) + `go vet ./...` (limpio) como verificación equivalente disponible. Los 8 archivos que sí reporta `gofmt -l .` en el repo son preexistentes, no tocados por esta feature — fuera de alcance corregirlos aquí (Constitución: 120 columnas, `gofumpt`)
- [X] T033 Cobertura verificada con `-coverpkg` cruzado (unit+integration+contract): `BuildContextPack` 95.9%, `specKitCandidates` 100%, `newContextCandidate` 94.4%, `OptimizeToolDescription` 83.3%, `adapters/secondary/compression` 89.2%, `adapters/secondary/tokens` 85.7%, `adapters/secondary/speckit` 80.0%. Los wrappers CLI (`cmdPackBuild`/`cmdPackShow`/`cmdPackStats`/`cmdPackCompress`, `CmdPack`) quedan sin cobertura automatizada porque llaman `fail()` (`os.Exit`) — mismo patrón ya existente en el proyecto (p. ej. `CmdPurge` tampoco se testea directo, solo su `ParsePurgeFlags`); se validan manualmente en T034 (Constitución Principio III)
- [X] T034 Ejecutado manualmente contra el binario compilado (`go build -o mem ./infrastructure`), Historias 1–5 + No-regresión, con una corrección real encontrada y aplicada: la tarea de ejemplo de Historia 1 en quickstart.md ("Implementar rotación **de** refresh tokens") incluía el stopword "de", que `tokenizeFTS` (existente, sin filtrado de stopwords) trata como término de búsqueda real — coincidía por casualidad con "Preferencia **de** estilo" y la incluía en el paquete pese a ser irrelevante. Se corrigió la frase de ejemplo en quickstart.md (sin tocar `tokenizeFTS`, que es infraestructura compartida fuera de alcance de esta feature) y se revalidó: la memoria irrelevante queda excluida. Resto de historias (dedup, stats/compress, Spec Kit con `--no-speckit`, no-regresión) verificadas sin sorpresas
- [X] T035 `mem context`/`mem search "Redis"` corridos manualmente tras toda la implementación: salida idéntica en formato y comportamiento al de antes de esta feature (FR-018/SC-005) — ningún camino de esos comandos pasa por `BuildContextPack`

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Setup (Fase 1)**: sin dependencias, arranca de inmediato
- **Foundational (Fase 2)**: depende de Setup — BLOQUEA todas las historias
- **Historias de usuario (Fase 3+)**: todas dependen de que Foundational esté completa
  - Orden recomendado por prioridad: US1 → US2 → US3 → US4 → US5
  - US2/US4 modifican el pipeline que crea US1 (`build_context_pack.go`), así que en la práctica van secuenciales después de US1, no en paralelo con ella
  - US3 y US5 son más independientes entre sí (superficies distintas) y podrían intercalarse después de US1
- **Pulido (Fase Final)**: depende de que todas las historias que se vayan a entregar estén completas

### Dependencias entre historias

- **US1 (P1)**: sin dependencia de otras historias — MVP autocontenido
- **US2 (P2)**: extiende el pipeline creado en US1 (T012); no puede empezar antes de T012
- **US3 (P3)**: extiende el CLI/MCP creados en US1 (T014, T013); no toca el pipeline de US1/US2
- **US4 (P4)**: extiende el pipeline creado en US1 (T012) y el CLI creado en US1 (T014); independiente de US2/US3
- **US5 (P5)**: solo depende de Foundational (T006, el `Compressor`) — puede implementarse en cualquier momento después de la Fase 2, incluso en paralelo con US1

### Dentro de cada historia

- Tests primero, deben fallar antes de implementar (Constitución, TDD no negociable)
- Dominio/puertos antes que casos de uso
- Casos de uso antes que CLI/MCP
- Historia completa y con checkpoint validado antes de pasar a la siguiente prioridad

### Oportunidades de paralelismo

- Todas las tareas Foundational marcadas [P] (T002, T003, T005, T007) corren en paralelo — archivos distintos
- Todos los tests de una misma historia marcados [P] corren en paralelo entre sí
- T016 (wiring del composition root) puede correr en paralelo con T013/T014/T015 de US1 — solo depende de Foundational
- US5 puede desarrollarse en paralelo con US1–US4 por un segundo desarrollador — solo depende de T006

---

## Ejemplo de Paralelismo: Fase 2 (Foundational)

```bash
# Lanzar juntos los tests + implementaciones sin dependencia cruzada de Foundational:
Task: "Escribir tests de tabla en adapters/secondary/tokens/approximate_test.go"
Task: "Escribir tests de tabla en adapters/secondary/compression/compression_test.go"
Task: "Crear domain/context_pack.go"
Task: "Agregar campos de configuración a SettingsData en application/ports/settings_repository.go"
```

## Ejemplo de Paralelismo: Historia de Usuario 1

```bash
# Lanzar juntos los tres tests de US1 (archivos distintos, sin dependencias entre sí):
Task: "Test de integración en tests/integration/build_context_pack_test.go"
Task: "Test unitario en application/usecases/build_context_pack_test.go"
Task: "Test de contrato en tests/contract/pack_build_cli_test.go"
```

---

## Estrategia de Implementación

### MVP primero (solo Historia 1)

1. Completar Fase 1: Setup
2. Completar Fase 2: Foundational (CRÍTICO — bloquea todas las historias)
3. Completar Fase 3: Historia de Usuario 1
4. **DETENERSE Y VALIDAR**: correr las secciones "Historia 1" y "No-regresión" de quickstart.md
5. Ese es el MVP entregable — `mem pack build` funcionando de punta a punta con presupuesto respetado y overflow crítico explícito

### Entrega incremental

1. Setup + Foundational → base lista
2. + Historia 1 → validar independientemente → MVP
3. + Historia 2 (dedup) → validar independientemente
4. + Historia 3 (observabilidad) → validar independientemente
5. + Historia 4 (Spec Kit) → validar independientemente
6. + Historia 5 (tool descriptions) → validar independientemente
7. Fase Final de pulido

### Estrategia de equipo en paralelo

Con varias personas disponibles, después de completar Foundational:

- Persona A: Historia 1 (bloquea a B en T012, pero no a C)
- Persona B: espera T012, luego toma Historia 2 y/o Historia 4 (ambas extienden el mismo pipeline, mejor secuenciales entre sí)
- Persona C: Historia 5, en paralelo desde el inicio (solo depende de Foundational)
- Historia 3 encaja bien como siguiente tarea de quien termine primero, apenas T013/T014 de US1 estén listos

---

## Notas

- [P] = archivo distinto, sin dependencia de una tarea sin terminar
- [Story] mapea cada tarea a su historia de usuario para trazabilidad
- TDD es obligatorio en este proyecto (Constitución, no negociable): verificar que cada test falla antes de escribir la implementación que lo hace pasar
- Commit por tarea o por grupo lógico pequeño, sin trailer de coautoría IA (preferencia fija del usuario)
- Detenerse en cada checkpoint de historia y validar contra quickstart.md antes de seguir
- Evitar: tareas vagas, dos tareas tocando el mismo archivo marcadas [P], dependencias cruzadas entre historias que rompan su independencia
