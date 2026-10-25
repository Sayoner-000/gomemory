# Tasks: Evaluación de la base Go para vertical slices en arquitectura hexagonal

**Input**: Design documents from `specs/043-vertical-slices-assessment/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: la spec no pide pruebas automatizadas nuevas. Cada historia termina con una tarea de **validación** del informe, no con código de prueba.

**Organization**: tareas agrupadas por historia de usuario. La evaluación es de solo lectura: ninguna tarea crea, modifica ni borra archivos del repositorio.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede hacer en paralelo (archivos distintos, sin dependencias pendientes). Por las reglas de trabajo de la persona, la ejecución real es **secuencial**; `[P]` solo marca independencia lógica.
- **[Story]**: US1–US4, según spec.md.

## Path Conventions

- `$OUT`: directorio de salida **fuera del repositorio**. En esta sesión es `/private/tmp/claude-501/-Users-josegomezj-home-rcw-gomemory/28b636af-efe0-44a3-bf23-a78eb416e974/scratchpad/043`; en otra sesión, cualquier directorio que no esté dentro de `git rev-parse --show-toplevel`.
- `$OUT/informe-043.md`: el informe. Sus secciones numeradas 1–11 son las de [contracts/report-contract.md](contracts/report-contract.md).
- Las rutas del código evaluado son relativas a la raíz del repositorio y **solo se leen**.

---

## Phase 1: Setup

**Purpose**: preparar la salida fuera del repositorio y el esqueleto del informe.

- [X] T001 Crear `$OUT` y comprobar que no está dentro del repositorio: `case "$OUT" in "$(git rev-parse --show-toplevel)"*) echo DENTRO;; *) echo FUERA;; esac` debe imprimir `FUERA`
- [X] T002 Crear `$OUT/informe-043.md` con los encabezados de las 11 secciones de `specs/043-vertical-slices-assessment/contracts/report-contract.md`, en el mismo orden

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: fijar la línea base y reunir la evidencia ejecutada que usan todas las historias.

**⚠️ CRITICAL**: no se empieza ninguna historia sin T003.

- [X] T003 Registrar la línea base: `git rev-parse HEAD > $OUT/baseline-head.txt` y `git status --porcelain > $OUT/baseline-status.txt`; escribir `head`, `status` (puede no estar vacío) y la fecha en la sección 1 de `$OUT/informe-043.md`
- [X] T004 [P] Obtener el grafo de importaciones de producción: `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... > $OUT/imports.txt` (research R1)
- [X] T005 [P] Obtener las importaciones de prueba por separado: `go list -f '{{.ImportPath}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... > $OUT/imports-test.txt`
- [X] T006 [P] Ejecutar `go vet ./... > $OUT/vet.txt 2>&1` y `go test ./... -count=1 > $OUT/test.txt 2>&1`, anotando el código de salida de cada uno; si alguno falla por red o caché, registrar el motivo para marcar como no verificados los criterios afectados (FR-011)
- [X] T007 Escribir la sección 2 de `$OUT/informe-043.md`: referencia a `specs/043-vertical-slices-assessment/contracts/evaluation-criteria.md` y "sin cambios en los criterios", o el cambio con su motivo

**Checkpoint**: línea base registrada y evidencia en `$OUT`; el repositorio sigue sin cambios.

---

## Phase 3: User Story 1 - Mapa estructural y de flujos reales (Priority: P1) 🎯 MVP

**Goal**: un mapa verificable de paquetes por capa, de sus importaciones y de tres flujos de extremo a extremo (C1).

**Independent Test**: tomar cualquier paso citado en las secciones 3–4, abrir el `archivo:línea` en el `head` de la línea base y comprobar que coincide; reproducir la tabla de la sección 3 con el comando de T004.

- [X] T008 [US1] Clasificar los 30 paquetes de `$OUT/imports.txt` por capa (domain · ports · usecases · adapter-primary · adapter-secondary · infrastructure · otro, con nota para `version/`, `assets/`, `tests/integration` y `tests/contract`), listar sus importaciones de producción dentro de `mem/` y escribir la tabla PaqueteCapa en la sección 3 de `$OUT/informe-043.md`, junto con el comando que la reproduce
- [X] T009 [P] [US1] Recorrer el slice **Socratica** desde sus tres entradas, `adapters/primary/cli/cmd_socratica.go` (CLI), `adapters/primary/tui/socratica.go` (TUI) y `adapters/primary/cli/cmd_mcp_socratica_tool.go` (MCP), pasando por `application/usecases/socratica_flow.go` y `application/usecases/socratica_specify.go`, luego `application/ports/socratica_repository.go` y `application/ports/socratica_unit_of_work.go`, hasta `adapters/secondary/persistence/socratica.go` y `adapters/secondary/persistence/socratica_ephemeral.go`; anotar cada paso como `{capa, símbolo, archivo:línea}` en la sección 4
- [X] T010 [P] [US1] Recorrer el slice **Review consensuado** desde `adapters/primary/cli/cmd_mcp_review_tools.go` (MCP) y `adapters/primary/cli/cmd_review.go` (CLI), pasando por `application/usecases/start_review.go`, `submit_reviewer_result.go`, `build_consensus.go`, `rejudge_review.go`, `finalize_review.go` y `promote_review_memory.go`, luego `application/ports/review_repository.go` y `application/ports/consensus_repository.go`, hasta `adapters/secondary/persistence/review.go`; anotar cada paso en la sección 4
- [X] T011 [P] [US1] Recorrer el slice **Instincts** desde `adapters/primary/cli/cmd_hook.go` y `adapters/primary/cli/hook_expand.go` (hook), pasando por `application/usecases/instincts.go` y `application/ports/instinct_repository.go`, hasta `adapters/secondary/persistence/instincts.go` (incluida la base de datos global separada); anotar cada paso en la sección 4
- [X] T012 [US1] Localizar en `infrastructure/container.go` y `infrastructure/main.go` dónde se construyen los adaptadores de los tres slices y añadirlo como flujo transversal en la sección 4
- [X] T013 [US1] Validar la US1: comprobar que en la sección 4 aparecen los cuatro tipos de entrada (CLI, MCP, TUI y hook), que hay al menos 3 slices (SC-002) y revisar al menos 10 citas `archivo:línea` contra el código del `head` (SC-001)

**Checkpoint**: secciones 3 y 4 completas y comprobables por sí solas.

---

## Phase 4: User Story 2 - Hallazgos, preferencias y prácticas a conservar (Priority: P1)

**Goal**: aplicar los criterios L1–L4, M1–M4 y T1–T3 y clasificar cada observación como hallazgo, preferencia o práctica a conservar (C2, C3).

**Independent Test**: cada fila de las secciones 6–8 tiene la evidencia y los campos obligatorios de `data-model.md`; ninguna fila de la sección 7 (preferencias) cumple las tres condiciones de research R3.

- [X] T014 [US2] Aplicar L1, L2 y L3 sobre `$OUT/imports.txt` con los filtros de `specs/043-vertical-slices-assessment/quickstart.md` paso 2 (domain solo `mem/domain`; application sin `adapters`/`infrastructure`; adapters sin ciclos y sin cruces primario→secundario sin motivo) y anotar el resultado por slice en la matriz de la sección 5
- [X] T015 [P] [US2] Aplicar L4: revisar las firmas de `application/ports/*.go` en busca de `*sql.DB`, `*sql.Tx`, `sql.Row` o tipos de driver, y revisar los métodos `Get*`/`Find*` de `adapters/secondary/persistence/` usados por los tres slices para ver si devuelven `sql.ErrNoRows` sin traducir (patrón de calibración de research R4)
- [X] T016 [P] [US2] Aplicar M1: comparar las decisiones de negocio (validaciones, transiciones, reglas) que hay en `adapters/primary/cli/cmd_socratica.go`, `adapters/primary/tui/socratica.go`, `adapters/primary/cli/cmd_mcp_socratica_tool.go`, `adapters/primary/cli/cmd_mcp_review_tools.go`, `adapters/primary/cli/cmd_review.go` y `adapters/primary/cli/hook_expand.go` con las de `application/usecases/`, señalando reglas duplicadas entre entradas
- [X] T017 [P] [US2] Aplicar M2: para cada slice, simular un cambio típico (añadir una acción o un campo) y listar los archivos que habría que tocar fuera del slice (registro de herramientas MCP en `adapters/primary/cli/cmd_mcp.go` y `adapters/primary/cli/cmd_mcp_schemas.go`, dispatcher en `adapters/primary/cli/dispatcher.go`, composition root), distinguiendo los puntos de registro inevitables de los acoplamientos reales
- [X] T018 [P] [US2] Aplicar M3 y M4: buscar construcciones de adaptadores fuera de `infrastructure/container.go` (`New*` de `adapters/secondary` llamados desde `adapters/primary`) y medir tamaño y responsabilidades de los archivos de entrada de los tres slices
- [X] T019 [P] [US2] Aplicar T1 y T2: identificar si las pruebas de los casos de uso de cada slice en `application/usecases/*_test.go` usan dobles de los ports o infraestructura real, y ejecutarlas aisladas (p. ej. `go test ./application/usecases -run 'Socratica|Specify' -count=1`, `-run 'Review|Consensus|Rejudge'`, `-run 'Instinct'`), guardando la salida en `$OUT/t2-<slice>.txt`
- [X] T020 [P] [US2] Aplicar T3: revisar `.github/workflows/test.yml` y buscar en `tests/` y en los `*_test.go` comprobaciones que inspeccionen importaciones entre capas; registrar si existen y si fallarían ante una importación prohibida (research R5)
- [X] T021 [US2] Escribir la sección 6 de `$OUT/informe-043.md`: un Hallazgo `H-NN` por cada celda "no cumple" que cumpla las tres condiciones de research R3, con categoría de la lista cerrada, criterio, evidencia, consecuencia, `verificado`, riesgo, beneficio y `cierre` (propuesta sin implementar)
- [X] T022 [US2] Escribir la sección 7: una Preferencia `P-NN` por cada observación sin consecuencia demostrable, indicando qué condición de research R3 le falta
- [X] T023 [US2] Escribir la sección 8: una PrácticaConservar `B-NN` por cada criterio que se cumple de forma destacada, con `archivo:línea` y el motivo ligado a slices o a la separación hexagonal
- [X] T024 [US2] Validar la US2: cada `H-NN` tiene todos los campos de `data-model.md`, ninguna `P-NN` cumple las tres condiciones y cada `B-NN` tiene evidencia (SC-003, SC-004)

**Checkpoint**: secciones 6–8 completas; la matriz de la sección 5 tiene todas las celdas de capas, módulos y pruebas.

---

## Phase 5: User Story 3 - Conclusión y roadmap (Priority: P2)

**Goal**: elegir una opción con su justificación y, si procede, un roadmap priorizado (C4, C5).

**Independent Test**: la sección 9 elige exactamente una opción y cita al menos un `H-NN` o `B-NN` verificado; la sección 10 tiene un criterio de éxito ejecutable por paso o la frase "Sin roadmap: …".

- [X] T025 [US3] Completar la matriz criterio × slice de la sección 5 de `$OUT/informe-043.md`: no puede quedar ninguna celda vacía (cumple · no cumple (H-NN) · no aplica)
- [X] T026 [US3] Escribir la sección 9 (Conclusión) aplicando la regla "Uso en la conclusión" de `specs/043-vertical-slices-assessment/contracts/evaluation-criteria.md`: una sola opción, `sostenida_por` solo con elementos verificados y razonamiento de por qué se descartan las otras dos
- [X] T027 [US3] Escribir la sección 10: si la opción no es `mantener`, una tabla PasoRoadmap ordenada por riesgo ascendente y beneficio descendente, en la que cada paso tiene slice candidato, los `H-NN` que resuelve y un criterio de éxito ejecutable (comando) o medible; si es `mantener`, escribir "Sin roadmap: …" con el motivo
- [X] T028 [US3] Validar la US3 contra SC-005 y SC-006

**Checkpoint**: el informe ya permite decidir (SC-008).

---

## Phase 6: User Story 4 - Evaluación sin alterar el repositorio (Priority: P1)

**Goal**: demostrar que la evaluación no cambió nada (C6). Va al final porque compara contra la línea base de T003.

**Independent Test**: el paso 5 de `specs/043-vertical-slices-assessment/quickstart.md` imprime `solo lectura: OK`.

- [X] T029 [US4] Ejecutar `diff <(git rev-parse HEAD) $OUT/baseline-head.txt && diff <(git status --porcelain) $OUT/baseline-status.txt && echo "solo lectura: OK"` y escribir el resultado en la sección 11 de `$OUT/informe-043.md`
- [X] T030 [US4] Si T029 detecta diferencias, listar cada ruta afectada en la sección 11, eliminar solo los archivos que la evaluación haya creado dentro del repositorio (nunca los cambios que ya estaban en la línea base), repetir T029 e informar del incidente en vez de ocultarlo

**Checkpoint**: SC-007 cumplido o incidente documentado.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T031 Validar `$OUT/informe-043.md` completo contra las "Reglas de validez" de `specs/043-vertical-slices-assessment/contracts/report-contract.md` y la lista "Validación del informe" de `specs/043-vertical-slices-assessment/quickstart.md`
- [X] T032 Entregar el informe en la conversación junto con la ruta absoluta de `$OUT/informe-043.md`, y guardar en gomemory (`save_memory`) la conclusión y los hallazgos principales, sin copiar el informe completo

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias.
- **Foundational (Phase 2)**: depende de Setup; T003 bloquea todo lo demás.
- **US1 (Phase 3)**: depende de T004 (grafo) y de T003.
- **US2 (Phase 4)**: depende de US1 (necesita los flujos para M1/M2) y de T004–T006.
- **US3 (Phase 5)**: depende de US2 (la conclusión se basa en hallazgos y prácticas).
- **US4 (Phase 6)**: depende de que terminen todas las demás, porque compara el estado final con la línea base.
- **Polish (Phase 7)**: depende de US4.

### User Story Dependencies

- **US1 → US2 → US3** es una cadena de datos, no de código: cada historia consume el informe de la anterior.
- **US4** se prepara en T003 y se cierra en T029; es transversal a todas.

### Within Each User Story

- En US1, los recorridos T009–T011 son independientes entre sí; T013 los valida.
- En US2, la aplicación de criterios (T014–T020) precede a la clasificación (T021–T023).

### Parallel Opportunities

- T004, T005 y T006 (evidencia de base).
- T009, T010 y T011 (un slice cada una).
- T015–T020 (un grupo de criterios cada una).

Por las reglas de trabajo de la persona se ejecutan **en secuencia**; las marcas `[P]` solo indican que el orden entre ellas es libre.

## Parallel Example: User Story 1

```text
T009 Recorrer Socratica (CLI, TUI, MCP → persistence/socratica*.go)
T010 Recorrer Review (MCP, CLI → persistence/review.go)
T011 Recorrer Instincts (hook → persistence/instincts.go)
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Fases 1 y 2 (línea base y evidencia).
2. Fase 3: el mapa estructural y los tres flujos.
3. **Parar y validar** con T013: el mapa ya es útil por sí solo para razonar sobre slices.

### Incremental Delivery

1. MVP (US1) → mapa verificable.
2. US2 → diagnóstico clasificado.
3. US3 → decisión y, si procede, roadmap.
4. US4 → prueba de solo lectura y entrega (T031–T032).

## Notes

- Ninguna tarea escribe en el repositorio; toda salida va a `$OUT`.
- Una afirmación sin verificar se marca `verificado: no` y no sostiene la conclusión.
- No se proponen parches ni diffs: el `cierre` de cada hallazgo es una propuesta en texto.
