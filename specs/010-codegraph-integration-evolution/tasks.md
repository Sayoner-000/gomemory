# Tasks: Evolución de la Integración con Grafo de Código Externo

**Input**: Documentos de diseño en `/specs/010-codegraph-integration-evolution/`
**Prerequisitos**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/cli-and-settings.md`, `quickstart.md`

**Tests**: la Constitución del proyecto (§III, "Testing First — NO NEGOCIABLE")
exige TDD estricto: cada tarea de test se escribe y falla ANTES que su
implementación correspondiente. No son opcionales en este proyecto.

**Organización**: tareas agrupadas por historia de usuario (spec.md), en
orden de prioridad P1 → P2 → P3, para que cada una se pueda implementar,
probar y entregar de forma independiente.

## Formato: `[ID] [P?] [Story] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[Story]**: historia de usuario a la que pertenece (US1/US2/US3)
- Cada tarea incluye la ruta de archivo exacta

## Nota de diseño (post-`/speckit-plan`, antes de generar estas tareas)

Durante la planificación se corrigieron dos supuestos del primer borrador
tras inspeccionar el código actual (ver `research.md` para el detalle):

1. **Historia 3 no necesita un adaptador nuevo**: `build_context.go` ya
   itera `[]ports.CodeGraphProvider` y salta en silencio los no disponibles.
   Solo hace falta alimentar esa lista con más de un candidato.
2. **Historias 1 y 2 no necesitan una capa de "usecase" para el camino de
   guardado**: el choke point real de este proyecto para lógica transversal
   al guardar (provenance, sinapsis) es `InsertMemory` en
   `adapters/secondary/persistence/memory.go` — ahí es donde se engancha la
   anotación de impacto y el export de ADR, siguiendo el patrón ya
   establecido. Solo la IMPORTACIÓN de ADR (proveedor → gomemory) amerita un
   usecase propio, porque coordina tres puertos distintos.

---

## Phase 1: Setup

- [X] T001 Verificar baseline verde en la raíz del repo antes de tocar código: `go build ./... && go vet ./... && go test ./...` — sin esto limpio, ninguna tarea siguiente es confiable

---

## Phase 2: Foundational (bloqueante para las 3 historias)

**Propósito**: extender la única struct de configuración (`persistence.Settings`)
con los 3 campos que cada historia lee de forma independiente. Es un solo
archivo compartido — no dividir por historia evita tres ediciones
conflictivas del mismo struct.

- [X] T002 Test de compatibilidad de settings: `code_graph_providers` vacío + `code_graph_command` legado ⇒ se normaliza a 1 elemento; defaults `adr_sync_enabled=false`, `code_impact_annotation_disabled=false` en `adapters/secondary/persistence/settings_test.go` (archivo nuevo — escribir PRIMERO, debe fallar)
- [X] T003 Extender `Settings` en `adapters/secondary/persistence/settings.go`: campos `CodeGraphProviders []string`, `AdrSyncEnabled bool`, `CodeImpactAnnotationDisabled bool` (nombre invertido a propósito — un bool JSON no distingue "ausente" de "false", mismo patrón que `CodeGraphDisabled` ya usado en este archivo, así la anotación queda ON por defecto sin opt-in), defaults en `DefaultSettings()` y normalización retrocompatible (hace pasar T002)

**Checkpoint**: fundación lista — las 3 historias pueden arrancar (en orden de prioridad o en paralelo si hay más de una persona).

---

## Phase 3: User Story 1 - Anotación de impacto al guardar (Priority: P1) 🎯 MVP

**Goal**: al guardar una memoria con `filepath` que casa con un hotspot
conocido del proveedor externo, el contenido persistido incluye una nota de
impacto — leyendo únicamente el snapshot ya cacheado, sin latencia extra.

**Independent Test**: guardar una memoria con `filepath` de un símbolo que
el snapshot reporta como hotspot (fixture) → verificar la anotación en el
contenido persistido. Repetir sin proveedor disponible → guardado idéntico
al actual, sin anotación ni demora (quickstart.md, sección Historia 1).

### Tests for User Story 1 ⚠️

> Escribir estos tests PRIMERO, confirmar que fallan, y solo entonces implementar.

- [X] T004 [P] [US1] ~~Test: `parseArchitecture` lee "file" de hotspots~~ → CORREGIDO en implementación: `get_architecture` real NO expone `file` (verificado en vivo contra el CLI); se agregó en cambio `TestHotspotQualifiedNames_MatchesRealFixture` (nuevo helper `hotspotQualifiedNames`, resuelto vía `search_code` aparte) en `provider_test.go`
- [X] T005 [P] [US1] Test: `Provider.ImpactFor(filepath)` casa por `Hotspot.File` contra el snapshot cacheado y devuelve `(_, false)` sin match/sin snapshot, en el mismo `provider_test.go`
- [X] T006 [US1] Test: `InsertMemory` anota impacto en el `content` cuando el `codeProvider` inyectado reporta hotspot para el `filepath`, y guarda igual (sin anotación, sin demora) sin proveedor/sin match, en `adapters/secondary/persistence/memory_test.go` (nuevo caso)

### Implementation for User Story 1

- [X] T007 [US1] Agregar `domain.CodeImpactAnnotation` (Hotspot bool, Symbol string, FanIn int) y extender `CodeHotspot` con `File string \`json:"file,omitempty"\`` en `domain/code_provider.go`
- [X] T008 [US1] CORREGIDO: en vez de leer "file" de `get_architecture` (no existe), se agregó `hotspotQualifiedNames()` + `resolveHotspotFiles()` en `provider.go`, que resuelven el archivo vía `search_code` por `qualified_name`, dentro de `Refresh()` (detached, fuera del hot path)
- [X] T009 [US1] Agregar `ImpactFor(filepath string) (domain.CodeImpactAnnotation, bool)` a la interfaz `ports.CodeGraphProvider` en `application/ports/code_graph_provider.go`
- [X] T010 [US1] Implementar `Provider.ImpactFor` en `adapters/secondary/codegraph/codebasememory/provider.go`: matchea contra `Snapshot()` ya cacheado, sin I/O nuevo (hace pasar T005; depende de T008, T009)
- [X] T011 [US1] Actualizar `fakeCodeProvider` en `application/usecases/build_context_test.go` para implementar `ImpactFor` (stub que devuelve `false`) — necesario para que el paquete siga compilando tras T009
- [X] T012 [US1] CORREGIDO: no se infiltró en `MemoryRepository` (struct) sino con el mismo patrón singleton que `dedupWindowDays`/`SetDedupWindowDays` ya usa este archivo — `codeImpactProvider` + `SetCodeImpactProvider()` en `adapters/secondary/persistence/memory.go`
- [X] T013 [US1] En `InsertMemory`: `annotateImpact()` anexa la nota de impacto al `content` cuando hay `codeImpactProvider` y `filepath` no vacío — best-effort, nunca bloquea ni falla el guardado (hace pasar T006)
- [X] T014 [US1] Wiring en `infrastructure/container.go`: `persistence.SetCodeImpactProvider(codeProviders[0])` (nil si la lista está vacía o `CodeImpactAnnotationDisabled`)
- [X] T015 [US1] `mem settings --code-impact-annotation=true|false` en `adapters/primary/cli/cmd_settings.go` (flag + caso en `fs.Visit` + línea en `printSettings`); extendido también `ports.SettingsData`/`SettingsRepository` (repositories.go) con los 3 campos nuevos de una vez, mismo criterio que Foundational

**Checkpoint**: User Story 1 funcional e independientemente verificable (quickstart.md, Historia 1) sin requerir US2 ni US3.

---

## Phase 4: User Story 2 - Sincronización bidireccional de ADR (Priority: P2)

**Goal**: las memorias `architecture`/`decision` se reflejan como ADR en el
proveedor externo (gomemory→proveedor), y los ADR del proveedor que no se
originaron en gomemory aparecen como memoria consultable (proveedor→gomemory),
sin bucles de resincronización.

**Independent Test**: activar `adr_sync_enabled`, guardar una memoria
`architecture`, verificar el ADR en el proveedor (`mem adr-sync status`).
Crear un ADR directo en el proveedor, correr el refresco, verificar que
aparece como memoria y no se duplica en refrescos siguientes (quickstart.md,
Historia 2). No requiere US1 ni US3.

### Tests for User Story 2 ⚠️

- [X] T016 [P] [US2] AMPLIADO: además de `SyncOrigin`/`SyncStatus`/`ADRSyncRecord`, se descubrió en vivo que `manage_adr` NO es CRUD multi-ADR sino un documento único de 6 secciones fijas (ver research.md §2, decisión consultada al usuario: bloques marcados `<!-- gomemory:id=N -->` dentro de secciones, no espejo de documento completo) — se agregaron tests de `ParseADRDocument`/`Render`/`UpsertBlock` en `domain/adr_document_test.go`
- [X] T017 [P] [US2] Test: migración crea `adr_sync_records` con índices únicos `(project, memory_id)` y `(project, provider, block_key)` (renombrado de `external_adr_id`, ver research.md §2), rechazando duplicados, en `adapters/secondary/persistence/adr_sync_test.go` (nuevo)
- [X] T018 [P] [US2] Test: CRUD (`InsertADRSyncRecord`/`GetADRSyncByMemory`/`GetADRSyncByBlockKey`/`UpdateADRSyncStatus`) en el mismo `adr_sync_test.go`
- [X] T019 [P] [US2] CORREGIDO: no hay paquete `adrsync/` aparte — `GetDocument`/`UpdateDocument` se agregaron al `Provider` ya existente de `codegraph/codebasememory` (mismo binario/proyecto que Historia 1). Tests con fixture real capturada en vivo (`testdata/manage_adr_get_no_adr.json`) en el mismo `provider_test.go`
- [X] T020 [US2] Test: `InsertMemory` exporta memorias `architecture`/`decision` a ADR (mapeadas a secciones ARCHITECTURE/TRADEOFFS) cuando `settingsAdrSyncEnabled=true` — best-effort, actualiza en vez de duplicar, pending si no se puede leer el doc — en `adapters/secondary/persistence/memory_test.go`
- [X] T021 [P] [US2] Test: `ImportADRs` trae bloques nuevos SIN marcador del documento del proveedor, los inserta como memoria `architecture` (vía `ImportMemory`, sin disparar exportToADR), y NO reimporta un bloque marcado `gomemory:id=N`, en `application/usecases/import_adrs_test.go` (nuevo, con fakes de los 3 puertos)
- [X] T022 [P] [US2] CORREGIDO: sin timestamp real por bloque (la API no lo expone), el conflicto se detecta comparando el hash del contenido local actual contra el último `ContentHash` sincronizado — si la copia local cambió Y el proveedor también, se conserva la local y queda `status=conflict_resolved` sin descartar ninguna versión; mismo archivo que T021

### Implementation for User Story 2

- [X] T023 [US2] Crear `domain/adr_sync.go` (tipos) + `domain/adr_document.go` (`ADRDocument`/`ADRSection`/`ADRBlock`, `ParseADRDocument`/`Render`/`UpsertBlock`, dominio puro) (hace pasar T016)
- [X] T024 [US2] Migración aditiva `CREATE TABLE IF NOT EXISTS adr_sync_records` + índices únicos en `adapters/secondary/persistence/db.go` (`migrate()`) (hace pasar T017)
- [X] T025 [US2] CRUD SQL directo en `adapters/secondary/persistence/adr_sync.go` (nuevo) (hace pasar T018; depende de T024)
- [X] T026 [P] [US2] Puerto `ADRSyncProvider` (`GetDocument(ctx) (string, error)`, `UpdateDocument(ctx, content) error` — documento completo, el merge por bloque es del dominio) en `application/ports/adr_sync_provider.go` (nuevo)
- [X] T027 [P] [US2] Puerto `ADRSyncRepository` (mismas operaciones que T025, como interfaz) en `application/ports/adr_sync_repository.go` (nuevo)
- [X] T028 [US2] `GetDocument`/`UpdateDocument` + `parseGetADRResponse` en el `Provider` ya existente (`codegraph/codebasememory/provider.go`) (hace pasar T019; depende de T026); agregadas aserciones `var _ ports.CodeGraphProvider/_ ports.ADRSyncProvider = (*Provider)(nil)`
- [X] T029 [US2] CORREGIDO: mismo patrón singleton que T012 (US1), no struct — `SetADRSync(provider, repo)` + `SetAdrSyncEnabled(bool)` en `memory.go`; `ADRSyncRepository` concreta (envuelve T025) en `repositories.go`
- [X] T030 [US2] `exportToADR()` en `memory.go`: parsea el doc (`domain.ParseADRDocument`), `UpsertBlock`, `Render`, `UpdateDocument`; nunca sobrescribe el doc si `GetDocument` falló (evita pisar contenido real con uno "vacío"); registra `ok`/`pending`/`failed` vía `recordADRSyncAttempt` (hace pasar T020; depende de T025, T026, T027, T028, T029). Corre síncrono con timeout de 4s (no goroutine) — decisión documentada en el código: la capacidad es opt-in (default off) así que el costo solo se paga cuando está activa, y mantiene `adr_sync_records` observable inmediatamente después de `InsertMemory`
- [X] T031 [US2] `ImportADRs(ctx, provider, adrRepo, memRepo, project)` en `application/usecases/import_adrs.go` — dedup por `block_key` (hash de sección+heading), conflicto por comparación de hash local vs. último sincronizado (hace pasar T021, T022). También se agregaron `ports.MemoryRepository.Get`/`UpdateContent` (con tests) — necesarios para leer/actualizar una memoria importada sin pasar por el choke point `InsertMemory` (evitaría reexportarla en bucle)
- [X] T032 [US2] `ImportADRs` enganchado a `hookTurnEnd` en `adapters/primary/cli/cmd_hook.go` (mismo bloque que `MaybeRefresh`), gateado por `deps.ADRSyncProvider != nil`, timeout 4s, error ignorado en silencio
- [X] T033 [US2] Wiring en `infrastructure/container.go`: `codeProviders[0]` casteado a `ports.ADRSyncProvider` (type assertion — el mismo `Provider` implementa ambas interfaces), `persistence.SetADRSync`/`SetAdrSyncEnabled`, y expuesto en `Deps.ADRSyncProvider/ADRSyncRepo` (nuevo, `deps.go`) para el hook
- [X] T034 [US2] `mem settings --adr-sync=true|false` en `adapters/primary/cli/cmd_settings.go`
- [X] T035 [US2] `mem adr-sync status` en `adapters/primary/cli/cmd_adr_sync.go` (nuevo, solo lectura, no expuesto vía MCP) — agregado `ListByProject` al puerto/repo (con test) para poder listar

**Checkpoint**: User Story 2 funcional e independientemente verificable (quickstart.md, Historia 2), sin requerir US1 ni US3.

---

## Phase 5: User Story 3 - Múltiples proveedores con fallback automático (Priority: P3)

**Goal**: declarar más de un proveedor candidato y que gomemory use
automáticamente el primero disponible, sin reconfigurar al cambiar de
entorno.

**Independent Test**: configurar dos candidatos (uno inexistente en PATH,
uno presente) y verificar que `get_context` se enriquece igual sin
intervención manual (quickstart.md, Historia 3). No requiere US2; refina
(no bloquea) el wiring que dejó US1.

### Tests for User Story 3 ⚠️

- [X] T036 [P] [US3] Test: `FirstAvailable(providers)` devuelve el primero con `Snapshot().Available=true`, `nil` si ninguno/lista vacía, salta entradas nil, en `application/usecases/provider_selection_test.go` (nuevo, con fakes)
- [X] T037 [P] [US3] Test de integración real (`tests/integration/code_graph_multi_provider_test.go`): binario `mem` compilado, un proveedor inexistente + un script fake que responde `list_projects`/`get_architecture`, `mem code-refresh` + `mem context` confirman que el disponible enriquece el contexto (dato distintivo del fake, no del ausente)

### Implementation for User Story 3

- [X] T038 [US3] `FirstAvailable` en `application/usecases/provider_selection.go` (hace pasar T036)
- [X] T039 [US3] `buildCodeProviders()` en `infrastructure/container.go` (itera `settings.CodeGraphProviders`, ya normalizada con el legado) + mismo fix en el fast-path `code-refresh` de `infrastructure/main.go` (antes solo refrescaba `CodeGraphCommand` singular) + flag `mem settings --code-graph-providers=cmd1,cmd2` en `cmd_settings.go` (faltaba en el desglose original, agregado acá). **Bug real encontrado y corregido antes de cablear**: el archivo de snapshot cacheado era uno solo por `.memory/`, compartido entre proveedores — con 2 candidatos, el segundo en refrescar pisaba los datos buenos del primero. Fix: `snapshotPath()` deriva el nombre de archivo de la identidad del proveedor (hash de `binOverride`), preservando el nombre legado para el caso de un solo proveedor sin configurar (sin invalidar caches existentes)
- [X] T040 [US3] `codeProviders[0]` (T014/T033) reemplazado por `usecases.FirstAvailable(codeProviders)` en `infrastructure/container.go`, reusado también por `tuiProvider()`

**Checkpoint**: las tres historias funcionan juntas; `get_context` sigue mostrando una sección por proveedor disponible (sin regresión) y puede haber más de un candidato configurado.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T041 [P] `docs/architecture.md`: nueva subsección "Evolución del brazo extensor (feature 010)" con las 3 historias, incluyendo el bug de snapshot compartido encontrado y corregido
- [X] T042 [P] `README.md`: 3 bullets nuevos en "Uso y Características Principales" + fila `mem adr-sync status` en la tabla de CLI
- [X] T043 `go build ./... && go vet ./... && go test ./...` limpio en todo el repo (verificado repetidamente en cada checkpoint de esta implementación). Cobertura por paquete: `domain` 88.4%, `usecases` 71.6%, `persistence` 53.5%, `codebasememory` 51.9% — el código NUEVO de esta feature tiene test directo en cada función/método; los paquetes con cobertura <80% arrastran código preexistente sin tests (p.ej. la mayoría de `adapters/primary/cli` ya estaba sin cobertura antes de esta feature) — subir eso es fuera de alcance de esta feature
- [X] T044 Recorrido manual contra el binario `mem` real compilado (no solo unit tests): Historia 1 confirmada end-to-end (`[impacto: HotFunc es un hotspot con 9 llamadores directos]` en `mem context`/`mem search` real); Historia 2 confirmada end-to-end (`mem adr-sync status` muestra el registro `ok` tras guardar una memoria `architecture` con `--adr-sync=true`); Historia 3 ya cubierta por el test de integración real T037 (`mem code-refresh` + `mem context` con 2 candidatos, uno ausente)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias
- **Foundational (Phase 2)**: depende de Setup — bloquea las 3 historias
- **User Stories (Phase 3-5)**: dependen de Foundational; entre sí, ver abajo
- **Polish (Phase 6)**: depende de las historias que se decida entregar

### User Story Dependencies

- **US1 (P1)**: sin dependencia de US2/US3. Introduce el wiring "proveedor activo" (T014) con la aproximación mínima `codeProviders[0]`.
- **US2 (P2)**: sin dependencia de US1/US3 — usa sus propios puertos (`ADRSyncProvider`/`ADRSyncRepository`), completamente ajenos a `CodeGraphProvider`. **Acoplamiento real, no de lógica sino de archivo**: T015 (US1), T034 (US2) y T039 (US3) tocan el mismo `cmd_settings.go` — implementarlas en orden de prioridad evita conflictos de diff, aunque ninguna depende funcionalmente de la anterior.
- **US3 (P3)**: sin dependencia de US2. Con US1: T040 específicamente reemplaza el `codeProviders[0]` que T014 introdujo — si se implementa US3 antes que US1, T040 no tiene nada que refinar todavía (se omite sin afectar el resto de US3). También: `CodeGraphProvider` ganó el método `ImpactFor` en T009 (US1) — cualquier test-double nuevo que implemente la interfaz completa (como el de T037) debe incluirlo, sin importar el orden de implementación.

### Dentro de cada historia

- Tests (obligatorios por Constitución) se escriben y fallan ANTES que su implementación
- Dominio → persistencia/puertos → adaptadores → wiring (`container.go`) → CLI
- Historia completa y verificada (checkpoint) antes de pasar a la siguiente prioridad

### Parallel Opportunities

- T002 (test) es la única tarea de Foundational — sin paralelismo real ahí
- Dentro de US1: T004/T005 en paralelo (mismo archivo pero casos de test independientes); T007 no paralelo con nada (bloquea T008)
- Dentro de US2: T016, T017, T018, T019, T021, T022, T026, T027 son test/diseño en archivos distintos — paralelizables entre sí
- US1 y US2 se pueden implementar en paralelo por personas distintas (sin dependencia funcional cruzada) — solo coordinar el orden de edición de `cmd_settings.go`
- Una vez Foundational completo, US3 puede empezar en paralelo con US1/US2; solo T040 espera a que exista T014

---

## Parallel Example: User Story 1

```bash
# Tests de US1 en paralelo:
Task: "parseArchitecture lee 'file' de hotspots — provider_test.go"
Task: "ImpactFor casa por filepath contra el snapshot — provider_test.go"

# Tras el checkpoint de dominio (T007), implementación:
Task: "Extender parseArchitecture para leer file (T008)"
Task: "Agregar ImpactFor a la interfaz CodeGraphProvider (T009)"
```

## Parallel Example: User Story 2

```bash
# Tests de US2 en paralelo (archivos distintos):
Task: "Validación de SyncOrigin/SyncStatus — domain/adr_sync_test.go"
Task: "Migración e índices de adr_sync_records — persistence/adr_sync_test.go"
Task: "Adaptador CLI de manage_adr — adrsync/codebasememory/provider_test.go"
Task: "ImportADRs sin bucles — usecases/import_adrs_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 solamente)

1. Phase 1 (Setup) → Phase 2 (Foundational) → Phase 3 (US1)
2. **DETENER Y VALIDAR**: correr la sección Historia 1 de `quickstart.md`
3. Esto ya es una mejora entregable por sí sola: memorias con `filepath` de código riesgoso quedan anotadas, sin tocar nada de ADR ni multi-proveedor

### Entrega incremental

1. Setup + Foundational → base lista
2. + US1 → validar independientemente → **MVP**
3. + US2 → validar independientemente (sin romper US1)
4. + US3 → validar independientemente (refina el wiring de US1 vía T040, sin romper ninguna de las dos anteriores)
5. Cada historia suma valor sin romper las anteriores — igual que exige la Constitución (§V.1, simplicidad; §V.7, idempotencia)

### Estrategia con más de una persona

1. Completar Setup + Foundational en conjunto (bloqueante, es chico: 2 tareas)
2. Con la fundación lista: Persona A → US1, Persona B → US2, en paralelo real (sin dependencia funcional)
3. US3 puede sumarse por una tercera persona en cuanto Foundational esté listo; solo coordina con A el momento de aplicar T040
4. Coordinar entre A/B/C únicamente el orden de aplicar los flags en `cmd_settings.go` (T015 → T034 → T039, mismo archivo)

---

## Notes

- `[P]` = archivos distintos, sin dependencias pendientes
- La etiqueta de historia (`[US1]`/`[US2]`/`[US3]`) es para trazabilidad — Setup/Foundational/Polish no llevan etiqueta
- Verificar que cada test falla antes de implementar (Constitución §III, no negociable en este proyecto)
- Cometer (`git commit`) después de cada tarea o grupo lógico
- Detenerse en cada checkpoint a validar la historia de forma independiente antes de seguir
- Evitar: tareas vagas, dos tareas [P] tocando el mismo archivo, dependencias cruzadas entre historias que rompan su independencia real (la única excepción documentada es T040, y está señalada explícitamente arriba)
