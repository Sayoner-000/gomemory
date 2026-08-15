---

description: "Lista de tareas — Reindexado dual de grafos de código + edición de huella de contexto en TUI"
---

# Tasks: Reindexado dual de grafos de código + edición de huella de contexto en TUI

**Input**: Documentos de diseño de `/specs/016-external-graph-reindex/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/code-graph-indexer.md](./contracts/code-graph-indexer.md), [quickstart.md](./quickstart.md)

**Tests**: OBLIGATORIOS, no opcionales — Constitución Principio III ("Testing First, NO
NEGOCIABLE"): los tests se escriben PRIMERO, deben fallar, y solo entonces se implementa
(Red-Green-Refactor). Cada tarea de implementación depende de su tarea de test
correspondiente, ya listada antes en el mismo bloque. Los tests existentes
(`adapters/primary/tui/tui_test.go:538,549`, que referencian `configRowAtomicPlan`/
`configOptions` por nombre) son intocables — las filas nuevas se agregan al final del menú,
nunca insertadas en medio.

**Organización**: agrupadas por historia de usuario (spec.md) para poder implementar y
probar cada una de forma independiente.

## Formato: `[ID] [P?] [Story] Descripción`

- **[P]**: puede correr en paralelo (archivo distinto, sin dependencia de una tarea sin terminar)
- **[Story]**: US1–US3, mapea a las historias de spec.md
- Rutas de archivo exactas en cada descripción, todas relativas a la raíz del repo

## Convenciones de ruta

Módulo Go único ya existente (`mem`), arquitectura hexagonal — sin paquetes nuevos:
`application/ports/` (puerto nuevo), `adapters/secondary/codegraph/codebasememory/`
(adaptador existente), `adapters/primary/{cli,tui}/` (adaptadores primarios existentes).
Ver plan.md "Project Structure" para el árbol completo.

---

## Phase 1: Setup

**Purpose**: línea base antes de tocar código.

- [X] T001 Ejecutar `go build ./...` y `go test ./...` en la raíz del repo y confirmar que
  el estado actual (sin esta feature) está verde — línea base para comparar contra T020 al
  final (plan.md: "compilación limpia, baseline ya confirmado limpio antes de tocar nada")

**Checkpoint**: línea base confirmada, ningún cambio de código todavía.

---

## Phase 2: Foundational (Parte 0 — pieza compartida, bloqueante para US1 y US2)

**Purpose**: el puerto `ports.CodeGraphIndexer` y su única implementación real
(`codebasememory.Provider.IndexRepository`), de los que dependen tanto el CLI (US1) como la
TUI (US2). US3 (editar huella de contexto) no depende de esta fase y podría desarrollarse en
paralelo, pero reutiliza el esquema de numeración de filas fijado en la Fase 4 (ver nota en T013).

**⚠️ CRÍTICO**: ninguna tarea de US1 o US2 puede empezar hasta que esta fase esté completa.

- [X] T002 [P] Crear `application/ports/code_graph_indexer.go`: interfaz `CodeGraphIndexer`
  (`Name() string`; `IndexRepository(ctx context.Context, mode string) (nodes, edges int, err error)`)
  y sentinel `ErrIndexerNotInstalled` (contracts/code-graph-indexer.md)
- [X] T003 [P] Crear fixture `adapters/secondary/codegraph/codebasememory/testdata/index_repository.json`
  con la respuesta real verificada en vivo de `cli index_repository`
  (`{"project":"ejemplo","status":"indexed","excluded":{"dirs":["node_modules",".git"],"count":2},"nodes":6529,"edges":15713,"adr_present":false}`)
- [X] T004 [P] Escribir tests que deben fallar en
  `adapters/secondary/codegraph/codebasememory/provider_test.go`:
  `TestParseIndexRepositoryResponse_Fixture` (parsea el fixture de T003, `nodes`/`edges` no
  vacíos), `TestParseIndexRepositoryResponse_Garbage` (JSON inválido → `ok=false`),
  `TestIndexRepository_SinBinario` (`New(root, tmpDir, "/ruta/inexistente")` →
  `IndexRepository` devuelve `ports.ErrIndexerNotInstalled`, verificable con `errors.Is`)
  (depende de: T002, T003)
- [X] T005 Implementar en `adapters/secondary/codegraph/codebasememory/provider.go`:
  aserción `_ ports.CodeGraphIndexer = (*Provider)(nil)`, constante
  `indexTimeout = 10 * time.Minute` junto a `probeTimeout`/`snapshotTTL`, refactor de
  `runCLI` delegando en `runCLIWithTimeout(ctx, timeout, tool, argsJSON)`, método público
  `IndexRepository` (no llama a `resolveProject` primero — research.md §2), función
  `parseIndexRepositoryResponse`, hasta que T004 pase (depende de: T002, T004)

**Checkpoint**: `ports.CodeGraphIndexer` implementado y probado sin binario real — US1 y US2
pueden empezar.

---

## Phase 3: User Story 1 - Un solo comando refresca ambos grafos de código (Priority: P1) 🎯 MVP

**Goal**: `mem index` actualiza el grafo propio (como hoy) y, a continuación, el grafo
externo (cuando el proveedor está instalado), con una opción explícita para omitirlo.

**Independent Test**: ejecutar el comando de indexado en un proyecto con el proveedor
externo instalado y verificar que, al terminar, ambos grafos reflejan el estado actual del
código, sin ejecutar ningún otro comando (quickstart.md Escenarios 1-4).

### Implementación para User Story 1

- [X] T006 [US1] Agregar flag `--skip-graph` (default `false`) y función
  `indexExternalGraph(deps *Deps)` en `adapters/primary/cli/cmd_index.go`: tras el indexado
  nativo Go (sin cambios), si no se pidió `--skip-graph`, obtiene
  `deps.CodeProviders[0].(ports.CodeGraphIndexer)` (sin filtrar por `FirstAvailable` —
  research.md §5) e invoca `IndexRepository(ctx, "full")`; reporta nodos/aristas en éxito;
  si `errors.Is(err, ports.ErrIndexerNotInstalled)` imprime línea informativa; cualquier otro
  error se reporta como advertencia (`⚠️`) — nunca usa `fail()`, el exit code permanece `0`
  porque el indexado nativo ya tuvo éxito (depende de: T005)

**Checkpoint**: US1 completamente funcional y verificable de forma independiente
(quickstart.md Escenarios 1-4).

---

## Phase 4: User Story 2 - Refrescar el grafo externo desde la interfaz interactiva (Priority: P2)

**Goal**: la pantalla de configuración de la TUI ofrece una acción equivalente al refresco
de US1, asíncrona (no bloquea la interfaz) y con guardia contra disparos concurrentes.

**Independent Test**: abrir la TUI, disparar la acción "reindexar grafo externo" desde
Configuración, y verificar que el resultado (éxito con conteos, "no disponible", o fallo) se
refleja en la interfaz sin haber usado la línea de comandos (quickstart.md Escenarios 5-7).

### Tests para User Story 2 (escribir primero, deben fallar)

- [X] T007 [P] [US2] Agregar dobles de prueba en `adapters/primary/tui/tui_test.go`:
  `fakeCodeIndexer` (implementa `ports.CodeGraphProvider` + `ports.CodeGraphIndexer`) y
  `fakeCodeProviderNoIndexer` (solo `ports.CodeGraphProvider`)
- [X] T008 [US2] Escribir tests que deben fallar en `adapters/primary/tui/tui_test.go`:
  `TestConfigView_ReindexLabel_SegunSoporteDeInterfaz`,
  `TestUpdateConfig_ReindexRow_SinSoporte_NoDisparaCmd`,
  `TestUpdateConfig_ReindexRow_ConSoporte_DisparaCmd`,
  `TestUpdateConfig_ReindexRow_YaEnCurso_NoDisparaSegundoCmd` (guardia de concurrencia,
  FR-011), `TestReindexExternalGraphCmd_PropagaResultado`,
  `TestUpdate_ExternalReindexDoneMsg_Exito`, `TestUpdate_ExternalReindexDoneMsg_NoInstalado`
  (depende de: T007)

### Implementación para User Story 2

- [X] T009 [US2] Agregar en `adapters/primary/tui/tui.go`: constante
  `configRowReindexGraph = configRowAtomicPlan + 1` (=7), tipo
  `externalReindexDoneMsg{nodes, edges int; err error}`, campo `reindexInProgress bool` en
  `model` (guardia de FR-011 — data-model.md "Estados del reindexado externo"), método
  `(m model) reindexExternalGraphCmd() tea.Cmd` que asertúa `m.codeProvider.(ports.CodeGraphIndexer)`
  y devuelve `externalReindexDoneMsg{err: ports.ErrIndexerNotInstalled}` si no hay soporte
- [X] T010 [US2] Wire en `adapters/primary/tui/tui.go`: caso `configRowReindexGraph` en
  `updateConfig` (sin soporte de interfaz → `statusMsg` "no disponible", sin disparar
  `tea.Cmd`; `reindexInProgress == true` → `statusMsg` "ya en curso", sin disparar un
  segundo `tea.Cmd`; en otro caso → fija `reindexInProgress = true`, `statusMsg`
  "🔗 Indexando grafo externo... (puede tardar, no bloquea la TUI)" y devuelve
  `reindexExternalGraphCmd()`), y caso `externalReindexDoneMsg` en el `Update()` de nivel
  superior (limpia `reindexInProgress`, fija `statusMsg` según éxito / `ErrIndexerNotInstalled`
  / error genérico) — hace pasar T008 (depende de: T009, T008)
- [X] T011 [US2] Agregar la fila "Reindexar grafo externo" al final de `rows` en
  `configView()` en `adapters/primary/tui/tui.go`, con label condicionado a si
  `m.codeProvider.(ports.CodeGraphIndexer)` tiene soporte ("Reindexar grafo externo
  (codebase-memory-mcp)" vs "Reindexar grafo externo: no disponible") (depende de: T009)

**Checkpoint**: US1 y US2 funcionan de forma independiente (quickstart.md Escenarios 1-7).

---

## Phase 5: User Story 3 - Editar la huella de contexto sin salir de la interfaz interactiva (Priority: P3)

**Goal**: la pantalla de configuración permite editar Budget/CompactThreshold/DedupWindowDays
sin salir de la TUI, vía una pantalla reutilizable de un solo campo.

**Independent Test**: entrar a Configuración, editar cada uno de los tres valores
(incluyendo cero y negativo), y verificar que el nuevo valor se guarda, se refleja de
inmediato en el resumen, y persiste en `.memory/settings.json` (quickstart.md Escenarios 8-10).

### Tests para User Story 3 (escribir primero, deben fallar)

- [X] T012 [US3] Escribir tests que deben fallar en `adapters/primary/tui/tui_test.go`:
  `TestUpdateConfig_EditBudgetRow_PrecargaValorActual`, `TestUpdateEditSetting_GuardaValorValido`,
  `TestUpdateEditSetting_RechazaNoNumerico` (incluye vacío y decimal, ej. `3.5`),
  `TestUpdateEditSetting_PermiteCeroYNegativo`, `TestUpdateEditSetting_Esc_CancelaSinGuardar`

### Implementación para User Story 3

- [X] T013 [US3] Agregar en `adapters/primary/tui/tui.go`: `screenEditSetting` al enum
  `screen`; tipo `editSettingField` (`editFieldBudget`, `editFieldCompactThreshold`,
  `editFieldDedupDays`); campos `editSettingField editSettingField`,
  `editSettingInput textinput.Model`, `editSettingErr string` en `model`; constantes
  `configRowEditBudget = configRowReindexGraph + 1` (=8),
  `configRowEditCompactThreshold = configRowEditBudget + 1` (=9),
  `configRowEditDedupDays = configRowEditCompactThreshold + 1` (=10),
  `configOptions = configRowEditDedupDays + 1` (=11) — depende del offset de
  `configRowReindexGraph` fijado en T009, por el esquema de numeración secuencial de
  plan.md que protege `tui_test.go:538,549` (depende de: T009)
- [X] T014 [P] [US3] Inicializar `editSettingInput` en `initialModel()` (placeholder
  "valor entero", mismo molde que los demás `textinput.New()` del archivo) en
  `adapters/primary/tui/tui.go` (depende de: T013)
- [X] T015 [US3] Agregar en `updateConfig()` los 3 casos (`configRowEditBudget`,
  `configRowEditCompactThreshold`, `configRowEditDedupDays`): leen el ajuste actual,
  precargan `editSettingInput` con `strconv.Itoa(valor)`, llaman `Focus()`, pasan a
  `screenEditSetting` — hace pasar `TestUpdateConfig_EditBudgetRow_PrecargaValorActual`
  (depende de: T013, T012)
- [X] T016 [US3] Implementar `updateEditSetting(msg tea.KeyMsg)` en
  `adapters/primary/tui/tui.go`: Esc vuelve a `screenConfig` sin guardar; Enter parsea con
  `strconv.Atoi` (vacío, no numérico o decimal → fija `editSettingErr` "Debe ser un número
  entero", permanece en la pantalla); valor entero válido (positivo, cero o negativo,
  semántica ya existente en settings.go) → `m.settingsRepo.Write(m.root, s)`, fija
  `statusMsg` de confirmación, vuelve a `screenConfig` — hace pasar el resto de T012
  (depende de: T013, T012)
- [X] T017 [P] [US3] Implementar `editSettingView()` (molde de `importView()`: título +
  ayuda según `editSettingField`, el `textinput`, error si `editSettingErr` no está vacío,
  ayuda de teclas) y agregar caso `screenEditSetting` en `View()` en
  `adapters/primary/tui/tui.go` (depende de: T013)
- [X] T018 [US3] Agregar las 3 filas de edición al final de `rows` en `configView()` y
  actualizar el texto final de las 3 líneas de solo-lectura de "Huella de contexto" (antes
  "editar en .memory/settings.json", ahora reflejando que también es editable desde el menú
  de abajo) en `adapters/primary/tui/tui.go` (depende de: T015)

**Checkpoint**: las tres historias funcionan de forma independiente (quickstart.md
Escenarios 1-10 completos).

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T019 [P] Ejecutar `go vet ./...` sobre `adapters/secondary/codegraph/codebasememory`,
  `adapters/primary/cli`, `adapters/primary/tui`
- [X] T020 Ejecutar `go test ./...` completo desde la raíz del repo y confirmar cero
  regresiones, incluyendo que el test existente que referencia `configRowAtomicPlan`/
  `configOptions` por nombre (`adapters/primary/tui/tui_test.go:538,549`) sigue pasando sin
  modificación — prueba de que agregar filas al final no rompió nada (depende de: T001–T018)
- [X] T021 Ejecutar manualmente los 10 escenarios de
  `specs/016-external-graph-reindex/quickstart.md`: CLI con/sin proveedor instalado,
  `--skip-graph`, fallo simulado del proceso externo; TUI reindex con/sin proveedor, disparo
  repetido mientras uno está en curso; TUI editar los 3 ajustes (incl. caso `0` y caso
  negativo), validación de entrada inválida, cancelar con Esc (depende de: T020)
- [X] T022 Confirmar con `git diff --stat` que el diff final toca exactamente los archivos
  listados en plan.md "Project Structure"
  (`application/ports/code_graph_indexer.go`,
  `adapters/secondary/codegraph/codebasememory/{provider.go,provider_test.go,testdata/index_repository.json}`,
  `adapters/primary/cli/cmd_index.go`, `adapters/primary/tui/{tui.go,tui_test.go}`) y
  ningún otro (depende de: T021)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias — puede empezar de inmediato
- **Foundational (Phase 2)**: depende de Setup — BLOQUEA a US1 (Phase 3) y US2 (Phase 4)
- **US1 (Phase 3)**: depende solo de Foundational
- **US2 (Phase 4)**: depende de Foundational; independiente de US1 en tiempo de ejecución,
  pero comparte el mismo puerto `ports.CodeGraphIndexer` creado en Foundational
- **US3 (Phase 5)**: no depende de Foundational ni de US1 en funcionalidad, pero T013
  depende de la constante `configRowReindexGraph` fijada en T009 (US2), por el esquema de
  numeración secuencial de filas de menú de plan.md — en la práctica, US3 se implementa
  después de que T009 (US2) haya fijado esa constante
- **Polish (Phase 6)**: depende de que todas las historias deseadas estén completas

### Within Each User Story

- Tests (obligatorios, Constitución Principio III) se escriben y deben FALLAR antes de la implementación
- La implementación de cada tarea hace pasar los tests de su bloque correspondiente
- Historia completa antes de pasar a la siguiente prioridad (o en paralelo si se coordina el
  offset de constantes de US3 con US2, ver nota arriba)

### Parallel Opportunities

- T002 y T003 (Foundational) en paralelo — archivos distintos
- T004 depende de ambos, pero es la única tarea de test de esa fase
- T007 (dobles de prueba de US2) puede empezar en paralelo con T006 (US1) una vez completa
  Foundational
- T014 y T017 (US3) marcadas [P] — tocan porciones independientes de `tui.go` una vez que
  T013 existe
- T019 (Polish) en paralelo con nada más, pero no depende de T020-T022

---

## Parallel Example: Foundational

```bash
# Lanzar juntas las dos tareas de preparación de Foundational:
Task: "Crear application/ports/code_graph_indexer.go"
Task: "Crear adapters/secondary/codegraph/codebasememory/testdata/index_repository.json"
```

---

## Implementation Strategy

### MVP First (User Story 1 solamente)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (CRÍTICO — bloquea a US1 y US2)
3. Completar Phase 3: User Story 1
4. **DETENERSE y VALIDAR**: probar `mem index` de forma independiente
   (quickstart.md Escenarios 1-4)
5. Entregar/demostrar si está listo — es el hueco de flujo de trabajo más costoso hoy

### Incremental Delivery

1. Setup + Foundational → base lista
2. Agregar US1 → probar de forma independiente → entregar (MVP)
3. Agregar US2 → probar de forma independiente → entregar
4. Agregar US3 → probar de forma independiente → entregar
5. Cada historia agrega valor sin romper las anteriores

---

## Notes

- [P] = archivos distintos, sin dependencias pendientes
- [Story] mapea la tarea a su historia de usuario para trazabilidad
- Verificar que los tests fallan antes de implementar (Red-Green-Refactor)
- Detenerse en cada checkpoint para validar la historia de forma independiente
- Evitar: tareas vagas, conflictos de mismo archivo sin coordinar, dependencias
  entre historias que rompan su independencia (más allá del offset de constantes ya
  documentado para US3)
