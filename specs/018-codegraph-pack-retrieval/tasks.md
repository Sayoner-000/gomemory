---

description: "Lista de tareas — Señal de grafo de código en Retrieval de ContextPack"
---

# Tasks: Señal de grafo de código en Retrieval de ContextPack

**Input**: Documentos de diseño de `/specs/018-codegraph-pack-retrieval/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: OBLIGATORIOS, no opcionales — Constitución Principio III ("Testing First, NO
NEGOCIABLE"): los tests se escriben PRIMERO, deben fallar, y solo entonces se implementa
(Red-Green-Refactor). Cada tarea de implementación depende de su tarea de test
correspondiente, ya listada antes en el mismo bloque. Los tests existentes
(`TestBuild_HotCodeSection_MatchAparece`/`AusenteSinMatchNiProveedor` en
`build_context_test.go`, y todo `build_context_pack_test.go`/`pack_build_cli_test.go` ya
existentes) son intocables — solo se agregan tests nuevos.

**Organización**: agrupadas por historia de usuario (spec.md) para poder implementar y
probar cada una de forma independiente.

## Formato: `[ID] [P?] [Story] Descripción`

- **[P]**: puede correr en paralelo (archivo distinto, sin dependencia de una tarea sin terminar)
- **[Story]**: US1–US3, mapea a las historias de spec.md
- Rutas de archivo exactas en cada descripción, todas relativas a la raíz del repo

## Convenciones de ruta

Módulo Go único ya existente (`mem`), arquitectura hexagonal — sin paquetes nuevos. Todo
el cambio de lógica vive en `application/usecases/` (ya existente); los dos call sites a
tocar (`adapters/primary/cli/cmd_pack.go`, `adapters/primary/cli/cmd_mcp.go`) también ya
existen. Ver plan.md "Project Structure" para el árbol completo.

---

## Phase 1: Setup

**Purpose**: línea base antes de tocar código.

- [X] T001 Ejecutar `go build ./...` y `go test ./...` en la raíz del repo y confirmar que
  el estado actual (sin esta feature) está verde — línea base para comparar contra T019 al
  final

**Checkpoint**: línea base confirmada, ningún cambio de código todavía.

---

## Phase 2: Foundational (bloqueante para US1 y US2)

**Purpose**: los dos campos nuevos de `ContextRequest` y el wiring mínimo (siempre
activado, sin la válvula de escape todavía — esa llega en US3) que hacen que
`deps.CodeProviders` llegue a `BuildContextPack` desde los dos call sites. Sin esto,
ninguna de las dos historias tiene de dónde leer un `CodeGraphProvider`.

**⚠️ CRÍTICO**: ninguna tarea de US1 o US2 puede empezar hasta que esta fase esté completa.

- [X] T002 [P] Agregar campos `IncludeCodeGraph bool` y
  `CodeProviders []ports.CodeGraphProvider` a `ContextRequest` en
  `application/usecases/build_context_pack.go` (líneas 19-32, junto a `IncludeSpecKit`) —
  data-model.md "ContextRequest (campos nuevos)"
- [X] T003 [P] En `cmdPackBuild`, `adapters/primary/cli/cmd_pack.go` (líneas 85-98): tras
  `ParsePackBuildFlags`, fijar `req.CodeProviders = deps.CodeProviders` y
  `req.IncludeCodeGraph = true` (hardcodeado por ahora — el flag `--no-code-graph` llega en
  US3, T014) antes de llamar a `usecases.BuildContextPack` (depende de: T002)
- [X] T004 [P] En el handler de la tool `pack_build`, `adapters/primary/cli/cmd_mcp.go`
  (líneas 357-384): agregar `IncludeCodeGraph: true, CodeProviders: deps.CodeProviders` al
  literal `usecases.ContextRequest{...}` (hardcodeado por ahora — el parámetro
  `no_code_graph` llega en US3, T015) (depende de: T002)

**Checkpoint**: los campos existen y llegan a `BuildContextPack` desde CLI y MCP, pero el
pipeline todavía los ignora — T001 repetido aquí debe seguir en verde, cero cambio de
comportamiento observable todavía.

---

## Phase 3: User Story 1 - Memorias ligadas a código activo se priorizan (Priority: P1) 🎯 MVP

**Goal**: una memoria candidata cuyo `Filepath` es un hotspot vigente del grafo de código
sube de prioridad Optional a Relevant antes de repartir presupuesto.

**Independent Test**: con un `CodeGraphProvider` que marca un archivo como hotspot y una
memoria `Preference` (Optional por tipo) con ese mismo `Filepath`, un presupuesto que
alcanza para Relevant pero no para Optional debe incluirla (quickstart.md Historia 1).

### Tests para User Story 1 (escribir primero, deben fallar)

- [X] T005 [US1] Escribir tests que deben fallar en
  `application/usecases/build_context_pack_test.go` (reusa `fakeCodeProvider`, ya definido
  en `build_context_test.go`, mismo paquete `usecases_test`):
  `TestBuildContextPack_CodeGraphHotspotBoostsPriority` (memoria `domain.Preference` con
  `Filepath: "hot.go"`, `CodeProviders: []ports.CodeGraphProvider{&fakeCodeProvider{impactByFile: map[string]domain.CodeImpactAnnotation{"hot.go": {Hotspot: true, FanIn: 10}}}}`,
  `IncludeCodeGraph: true` → el `ContextItem` resultante tiene `Priority == domain.PriorityRelevant`),
  `TestBuildContextPack_CodeGraphNeverTouchesCriticalPriority` (memoria `domain.Decision`
  con el mismo `Filepath` hotspot → `Priority` permanece `domain.PriorityCritical`, sin
  cambio — acceptance scenario 2 de la Historia 1),
  `TestBuildContextPack_CodeGraphDisabled_NoBoost` (mismo setup pero
  `IncludeCodeGraph: false` → la memoria `Preference` permanece `PriorityOptional`),
  `TestBuildContextPack_NoCodeProviders_NoBoost` (`CodeProviders: nil`,
  `IncludeCodeGraph: true` → sin cambio, cero impacto sin proveedor — acceptance scenario 3)
  (depende de: T002)

### Implementación para User Story 1

- [X] T006 [US1] Implementar `boostHotspotCandidates(items []contextCandidate, providers []ports.CodeGraphProvider)`
  en `application/usecases/build_context_pack.go`: para cada item con `source` no vacío,
  consulta `ImpactFor(item.source)` contra CADA proveedor de `providers` (research.md §2 —
  no solo `FirstAvailable`); si hay match de hotspot en cualquiera y
  `item.priority == domain.PriorityOptional`, lo sube a `domain.PriorityRelevant`; nunca
  toca `domain.PriorityCritical` (research.md §6) — hace pasar T005 (depende de: T005)
- [X] T007 [US1] En `BuildContextPack` (`build_context_pack.go`), llamar
  `boostHotspotCandidates(items, req.CodeProviders)` inmediatamente después de ensamblar
  `items` (memorias + Spec Kit) y antes del cálculo de `criticalSum`, guardado por
  `if req.IncludeCodeGraph` (data-model.md "Relaciones") (depende de: T006)

**Checkpoint**: US1 completamente funcional y verificable de forma independiente
(quickstart.md Historia 1).

---

## Phase 4: User Story 2 - Orientación arquitectónica compacta (Priority: P2)

**Goal**: cuando hay snapshot de grafo de código disponible y el presupuesto alcanza, el
`ContextPack` incluye un ítem con el mismo resumen que ya muestra `mem context`.

**Independent Test**: con un `CodeGraphProvider` con snapshot disponible y presupuesto
amplio, `pack.Items` incluye un ítem `codegraph:architecture`; con presupuesto muy
ajustado, ese ítem queda descartado sin afectar el resto (quickstart.md Historia 2).

### Tests para User Story 2 (escribir primero, deben fallar)

- [X] T008 [US2] Escribir tests que deben fallar en
  `application/usecases/build_context_pack_test.go`:
  `TestBuildContextPack_CodeGraphArchitectureCandidate_WhenAvailable` (`fakeCodeProvider`
  con `snap.Available: true` y `snap.Architecture` no nulo, presupuesto amplio → aparece un
  `ContextItem` con `ID == "codegraph:architecture"` y `Priority == domain.PriorityOptional`),
  `TestBuildContextPack_CodeGraphArchitectureAbsent_WhenUnavailable`
  (`fakeCodeProvider{snap: domain.CodeProviderSnapshot{Available: false}}` → ningún ítem
  `codegraph:*`), `TestBuildContextPack_CodeGraphArchitectureDiscardedWhenBudgetTight`
  (snapshot disponible pero presupuesto que solo alcanza para un item Critical ya presente
  → el ítem de arquitectura no aparece en `pack.Items` y `pack.Stats.ItemsDiscarded`
  aumenta en 1) (depende de: T002; mismo archivo que T005 — no ejecutar en paralelo con
  T005, coordinar orden de escritura)

### Implementación para User Story 2

- [X] T009 [P] [US2] Extraer el cuerpo de `writeCodeProviderSection`
  (`application/usecases/build_context.go`, líneas 29-64) a una función pura nueva
  `formatCodeArchitecture(snap domain.CodeProviderSnapshot) string`; `writeCodeProviderSection`
  pasa a ser `sb.WriteString(formatCodeArchitecture(snap))` — mismo output exacto, sin
  cambio de comportamiento (research.md §3); los tests existentes
  `TestBuild_HotCodeSection_MatchAparece`/`AusenteSinMatchNiProveedor` deben seguir
  pasando sin modificarlos (depende de: T002; archivo distinto de T006/T007, paralelizable)
- [X] T010 [US2] Implementar `codeGraphArchitectureCandidate(providers []ports.CodeGraphProvider) (contextCandidate, bool)`
  en `application/usecases/build_context_pack.go`: usa `FirstAvailable(providers)` (mismo
  paquete `usecases`, sin import — research.md §2); si es `nil` o `!snap.Available` →
  `(contextCandidate{}, false)`; si no, arma
  `contextCandidate{id: "codegraph:architecture", source: snap.Provider, content: formatCodeArchitecture(snap), priority: domain.PriorityOptional, importance: 0.4, relevance: 1, confidence: 1}`
  — hace pasar T008 (depende de: T009, T008)
- [X] T011 [US2] En `BuildContextPack`, tras la llamada a `boostHotspotCandidates` (T007) y
  todavía dentro del mismo bloque guardado por `req.IncludeCodeGraph`, llamar
  `codeGraphArchitectureCandidate(req.CodeProviders)` y, si el segundo valor es `true`,
  agregar el candidato a `items` (data-model.md "Relaciones") (depende de: T010, T007)

**Checkpoint**: US1 y US2 funcionan de forma independiente (quickstart.md Historias 1-2).

---

## Phase 5: User Story 3 - Desactivar la señal de grafo de código por invocación (Priority: P3)

**Goal**: `--no-code-graph` (CLI) y `no_code_graph` (MCP) desactivan `IncludeCodeGraph`
para una llamada puntual, sin afectar el default activado (FR-007).

**Independent Test**: con un proveedor disponible, `mem pack build ... --no-code-graph`
produce un `ContextPack` idéntico al que se obtendría sin proveedor configurado
(quickstart.md Historia 3).

### Tests para User Story 3 (escribir primero, deben fallar)

- [X] T012 [US3] Escribir test que debe fallar en `tests/contract/pack_build_cli_test.go`:
  `TestParsePackBuildFlags_NoCodeGraphDisablesInclusion` (mismo patrón que
  `TestParsePackBuildFlags_NoSpeckitDisablesInclusion` ya existente) — `--no-code-graph`
  produce `req.IncludeCodeGraph == false`; ausencia del flag produce `true`
- [X] T013 [P] [US3] Escribir test de integración que debe fallar en
  `tests/integration/build_context_pack_codegraph_test.go` (archivo nuevo):
  `TestBuildContextPack_NoCodeGraph_ExcludesArchitectureAndBoost` — punta a punta con
  `MemoryRepository` real (mismo patrón que `build_context_pack_speckit_test.go`), un
  `fakeCodeProvider` con hotspot y snapshot disponible, `IncludeCodeGraph: false` →
  confirma cero ítem `codegraph:*` en `pack.Items` y cero boost de prioridad (depende de:
  T002; archivo nuevo, paralelizable con T012)

### Implementación para User Story 3

- [X] T014 [US3] Agregar flag `--no-code-graph` a `ParsePackBuildFlags` en
  `adapters/primary/cli/cmd_pack.go` (mismo patrón que `--no-speckit`); reemplazar el
  hardcode `req.IncludeCodeGraph = true` de T003 por
  `req.IncludeCodeGraph = !*noCodeGraph` — hace pasar T012 (depende de: T003, T012)
- [X] T015 [P] [US3] Agregar campo `NoCodeGraph bool json:"no_code_graph,omitempty"` al
  struct de entrada de la tool `pack_build` en `adapters/primary/cli/cmd_mcp.go`; reemplazar
  el hardcode `IncludeCodeGraph: true` de T004 por `IncludeCodeGraph: !in.NoCodeGraph` —
  nombre en negativo, no `include_code_graph` (research.md §5, para no heredar el default
  apagado por zero-value que ya tiene `include_speckit`) (depende de: T004)

**Checkpoint**: las tres historias funcionan de forma independiente (quickstart.md
Historias 1-3 + No-regresión).

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T016 [P] Agregar una nota breve en `docs/MANUAL.md`, sección de `mem pack`: ahora
  también consulta el grafo de código externo (opcional, solo snapshot cacheado, mismo
  criterio de degradación silenciosa que `mem context`; flags `--no-code-graph` /
  `no_code_graph`)
- [X] T017 [P] Agregar una nota breve en `docs/architecture.md`: `CodeGraphProvider` ahora
  tiene un segundo consumidor (`BuildContextPack`, además de `build_context.go`)
- [X] T018 [P] Ejecutar `go vet ./...` sobre `application/usecases`,
  `adapters/primary/cli`, `tests/integration`, `tests/contract`
- [X] T019 Ejecutar `go test ./...` completo desde la raíz del repo y confirmar cero
  regresiones, incluyendo que `TestBuild_HotCodeSection_MatchAparece`/
  `AusenteSinMatchNiProveedor` y todos los tests ya existentes de
  `build_context_pack_test.go`/`pack_build_cli_test.go` siguen pasando sin modificación
  (depende de: T001–T017)
- [X] T020 Ejecutar manualmente los escenarios de
  `specs/018-codegraph-pack-retrieval/quickstart.md` (Historias 1-3 + No-regresión) contra
  un binario compilado con esta feature y, si hay uno disponible, un proveedor real de
  grafo de código indexado sobre este mismo repo (depende de: T019)
- [X] T021 Confirmar con `git diff --stat` que el diff final toca exactamente los archivos
  listados en plan.md "Project Structure"
  (`application/usecases/{build_context.go,build_context_pack.go,build_context_pack_test.go}`,
  `adapters/primary/cli/{cmd_pack.go,cmd_mcp.go}`,
  `tests/integration/build_context_pack_codegraph_test.go`,
  `tests/contract/pack_build_cli_test.go`, `docs/MANUAL.md`, `docs/architecture.md`) y
  ningún otro (depende de: T020)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias — puede empezar de inmediato
- **Foundational (Phase 2)**: depende de Setup — BLOQUEA a US1 (Phase 3) y US2 (Phase 4)
- **US1 (Phase 3)**: depende solo de Foundational
- **US2 (Phase 4)**: depende de Foundational; T011 además depende de T007 (US1) porque
  ambas llamadas viven en el mismo bloque guardado de `BuildContextPack` — en la práctica
  se implementa después de US1, aunque su valor de negocio es independiente
- **US3 (Phase 5)**: depende de Foundational Y de que T003/T004 (Foundational) ya existan
  como hardcode a reemplazar — en la práctica se implementa después de US1/US2, aunque su
  test (T012/T013) puede escribirse antes
- **Polish (Phase 6)**: depende de que todas las historias deseadas estén completas

### Within Each User Story

- Tests (obligatorios, Constitución Principio III) se escriben y deben FALLAR antes de la implementación
- La implementación de cada tarea hace pasar los tests de su bloque correspondiente
- Historia completa antes de pasar a la siguiente prioridad

### Parallel Opportunities

- T002, T003, T004 (Foundational): T003 y T004 en paralelo una vez completo T002 (archivos
  distintos: `cmd_pack.go` vs `cmd_mcp.go`)
- T009 (extracción en `build_context.go`) puede correr en paralelo con T006/T007 (US1, en
  `build_context_pack.go`) — archivos distintos
- T005 y T008 tocan el mismo archivo (`build_context_pack_test.go`) — coordinar orden, no
  paralelizar entre sí
- T013 (US3, archivo nuevo de integración) en paralelo con T012 (US3, contrato CLI)
- T015 (US3, MCP) en paralelo con T014 (US3, CLI) — archivos distintos
- T016, T017, T018 (Polish) en paralelo entre sí

---

## Parallel Example: Foundational

```bash
# Lanzar juntas las dos tareas de wiring una vez completo T002:
Task: "Wire req.CodeProviders/IncludeCodeGraph en cmdPackBuild (cmd_pack.go)"
Task: "Wire IncludeCodeGraph/CodeProviders en el handler MCP pack_build (cmd_mcp.go)"
```

---

## Implementation Strategy

### MVP First (User Story 1 solamente)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (CRÍTICO — bloquea a US1 y US2)
3. Completar Phase 3: User Story 1
4. **DETENERSE y VALIDAR**: probar el boost de prioridad de forma independiente
   (quickstart.md Historia 1)
5. Entregar/demostrar si está listo — es el valor central de la feature (research.md §1)

### Incremental Delivery

1. Setup + Foundational → base lista
2. Agregar US1 → probar de forma independiente → entregar (MVP)
3. Agregar US2 → probar de forma independiente → entregar
4. Agregar US3 → probar de forma independiente → entregar
5. Cada historia agrega valor sin romper las anteriores (FR-009, no-regresión)

---

## Notes

- [P] = archivos distintos, sin dependencias pendientes
- [Story] mapea la tarea a su historia de usuario para trazabilidad
- Verificar que los tests fallan antes de implementar (Red-Green-Refactor)
- Detenerse en cada checkpoint para validar la historia de forma independiente
- Evitar: tareas vagas, conflictos de mismo archivo sin coordinar (T005/T008), dependencias
  entre historias que rompan su independencia de valor de negocio (aunque compartan punto
  de integración en `BuildContextPack`, como T007/T011)
