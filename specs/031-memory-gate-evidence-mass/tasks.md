---

description: "Tareas de implementación de la feature 031: gate de pre-escritura, evidencia de anclas y masa de memorias"
---

# Tasks: Gate de pre-escritura, evidencia de anclas y masa de memorias

**Input**: Documentos de diseño en `specs/031-memory-gate-evidence-mass/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: OBLIGATORIOS. La constitución (principio III, no negociable) exige TDD: cada tarea de test se escribe primero y DEBE fallar (rojo o sin compilar) antes de su implementación. **Ningún test existente se modifica.** Los tests nuevos van en archivos nuevos, o como funciones nuevas al final de un archivo existente.

**Organization**: una fase por historia (US1 a US4), en orden de prioridad. Cada historia se entrega y se revierte por separado (FR-023).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede hacer en paralelo (otro archivo, sin depender de tareas incompletas)
- **[Story]**: historia de spec.md (US1…US4)

## Convenciones del repo (aplican a todas las tareas)

- Módulo Go `mem`. Identificadores en inglés, comentarios en español latino y solo sobre el "por qué" (memoria 132).
- Tests junto al código (`*_test.go`). Los de `application/usecases` están en el paquete externo `usecases_test` y montan la persistencia real con `persistence.Init(t.TempDir())`, `persistence.NewMemoryRepository(db)`, `NewSessionRepository` y `NewRelationRepository` (ver `application/usecases/build_context_test.go`).
- `Insert` con `SessionID` forma sinapsis automáticas `related` 0.5 con memorias de la misma sesión. Para aristas controladas, inserta sin sesión y crea las relaciones con `relRepo.Insert(&domain.Relation{...})` o `usecases.RecordVerdict`.
- Dos `Insert` con el mismo tipo y título dentro de la ventana de dedup devuelven el MISMO id (upsert). Para ejercitar el gate, usa títulos que no sean idénticos.
- Puertos nuevos: son estrechos y nunca amplían `ports.MemoryRepository` ni `ports.RelationLister` (memoria 167).
- Comprobación de cada tarea: `go test ./<paquete>/... -run <Test>`. Al cerrar cada fase: `gofmt -l .`, `go vet ./...` y `go test ./...`.

---

## Phase 1: Setup (línea base)

**Purpose**: saber qué estaba verde antes de tocar nada. Así un fallo posterior se atribuye a la tarea que lo causó.

- [X] T001 Ejecutar `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...` (v2.13.2, memoria 172) y `go test ./...` en la raíz, y anotar el resultado (verde, o la lista de fallos preexistentes; la memoria 140 cita uno conocido con `-race ./...`) en la sección "Línea base" al final de specs/031-memory-gate-evidence-mass/tasks.md

---

## Phase 2: Foundational (prerrequisitos que bloquean)

**Purpose**: cerrar el defecto que condiciona la calibración del gate (P0, bugfix directo, memoria 93) y declarar los puertos estrechos que usan las historias. Los puertos van en un solo archivo compartido, así que se declaran aquí y no dentro de cada historia, para evitar conflictos.

### P0 — Anomalía 207/209: el dedup exacto no debe fallar en silencio

- [X] T002 Añadir en adapters/secondary/persistence/memory_dedup_test.go (archivo nuevo, `package persistence`) el test `TestInsert_MismoTituloTipoSesion_DevuelveMismoID`: `db := Init(t.TempDir())`; dos `InsertMemory` con `Project:"p"`, `Type: domain.Learning`, el mismo `Title`, el mismo `SessionID:"s1"` y contenidos distintos; assert de que ambos devuelven el mismo id y de que `ListAllMemories` (o `ListAll` vía `NewMemoryRepository`) tiene 1 fila. Esperado: verde. Documenta la ruta feliz que 207/209 contradice.
- [X] T003 Añadir en adapters/secondary/persistence/memory_dedup_test.go el test `TestFindDuplicateTx_ErrorDeConsulta_NoSeTrataComoSinDuplicado`:
  - Abrir `tx, _ := db.Begin()` y ejecutar dentro `ALTER TABLE memories RENAME TO memories_x`, de modo que la consulta falle con un error distinto de `sql.ErrNoRows`.
  - Llamar `findDuplicateTx(tx, &domain.Memory{Project:"p", Type: domain.Learning, Title:"t"}, "t", "c")`.
  - Assert de que devuelve un error no nil.
  - Hacer `tx.Rollback()`.

  DEBE fallar: hoy la firma es `(int64, bool)` y el error se descarta.
- [X] T004 Cambiar `findDuplicateTx` en adapters/secondary/persistence/memory.go a la firma `(int64, bool, error)`. `sql.ErrNoRows` → `(0, false, nil)`; cualquier otro error de `Scan` → `(0, false, fmt.Errorf("dedup lookup: %w", err))`, en las tres consultas (`topic_key`, checkpoint y título). En `insertMemory`, si hay error, devolver `0, err` antes del INSERT (fallar rápido; el `defer Rollback` ya existe). Comprobar que T002, T003 y `go test ./adapters/secondary/persistence/...` pasan.
- [X] T005 Guardar con `save_memory` (tipo bugfix, `filepath` adapters/secondary/persistence/memory.go) la causa de la anomalía 207/209: el fail-open en `findDuplicateTx`, corregido. Si T003 hubiera pasado sin T004, registrar en su lugar que la causa restante es `ImportMemory` sin dedup.
- [X] T006 ⚠ no atómica: fusionar 207/209 (la persona aprobó olvidar la 207 el 2026-09-13; ejecutado) (`forget_memory` de una de las dos) requiere la aprobación explícita de la persona, porque es irreversible. Pedirla y NO ejecutar sin respuesta. No bloquea el resto de fases.
- [ ] T007 Proponer a la persona el commit `fix(persistence): el dedup por identidad no trata errores de consulta como ausencia de duplicado`, con adapters/secondary/persistence/memory.go y adapters/secondary/persistence/memory_dedup_test.go. Mostrar la lista de archivos y esperar su aprobación antes de `git commit` (reglas de commit de la persona).

### Puertos estrechos

- [X] T008 Añadir en application/ports/context_builder.go tres interfaces, cada una con un comentario de una línea que explique por qué es estrecha:
  - `MemoryFullLister { ListAll(project string) ([]domain.Memory, error) }`
  - `RelationFullLister { ListAll(project string) ([]domain.Relation, error) }`
  - `IndexedFilesQuerier { FileHashes(project string) (map[string]string, error) }`

  Añadir en adapters/secondary/persistence/repositories.go las aserciones de compilación `var _ ports.MemoryFullLister = (*MemoryRepository)(nil)`, `var _ ports.RelationFullLister = (*RelationRepository)(nil)` y `var _ ports.IndexedFilesQuerier = (*CodeGraphRepository)(nil)`, con los nombres de tipo reales del archivo. Verificar con `go build ./...`.

**Checkpoint**: P0 cerrado y puertos declarados. Las historias pueden empezar.

---

## Phase 3: User Story 1 - Aviso de posible duplicado al guardar (Priority: P1) 🎯 MVP

**Goal**: `save_memory` y `mem save` avisan de hasta 3 casi-duplicados del mismo tipo, siempre guardan, callan en el upsert y declaran cuando no pudieron comprobar (contracts/save-gate.md).

**Independent Test**: dos guardados casi iguales por MCP en memoria → el segundo contiene `Posible duplicado de #`; un tema nuevo no avisa.

### Similitud y calibración

- [X] T009 [P] [US1] Crear application/usecases/save_gate_test.go (`package usecases_test`) con:
  - `TestGateSimilarity_TituloIdentico`: títulos iguales → `title == 1`.
  - `TestGateSimilarity_SinTokensComunes`: → `(0, 0)`.
  - `TestGateSizeCompatible`: `GateSizeCompatible(10, 2, 0.3) == false`, `GateSizeCompatible(10, 5, 0.3) == true`, y `false` si algún tamaño es 0.

  DEBE no compilar todavía.
- [X] T010 [US1] Crear application/usecases/save_gate.go con:
  - `GateSimilarity(a, b domain.Memory) (title, body float64)`: reutiliza `tokenize`/`jaccardSimilarity` de detect_duplicates.go; `title` compara `tokenize(Title)` y `body` compara `tokenize(Title+" "+Content)`.
  - `GateSizeCompatible(na, nb int, t float64) bool`: `min/max >= t`, false si alguno ≤ 0. Un comentario explica que `J ≤ min/max`, así que la poda no pierde recall.

  Hacer pasar T009.
- [X] T011 [US1] Crear tests/integration/save_gate_calibration_test.go con `//go:build calibration` y `TestSaveGateCalibration`:
  - Si `GOMEMORY_CALIBRATION_DB` está vacía → `t.Skip`.
  - Abrir `sql.Open("sqlite", "file:"+path+"?mode=ro")` con el driver `modernc.org/sqlite`. NO usar `persistence.Open`/`Init`, porque migran y escribirían.
  - `SELECT id, type, title, content, COALESCE(topic_key,'') FROM memories WHERE type <> 'checkpoint' ORDER BY id`.
  - Para cada memoria, calcular `max title` y `max body` con `usecases.GateSimilarity` contra las anteriores del mismo tipo, recordando el par que da el máximo.
  - Imprimir con `t.Logf` una tabla para `T_title ∈ {0.5, 0.6, 0.7, 0.8, 0.9, 1.0}` × `T_body ∈ {0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.5}`: nº de memorias que habrían avisado, % sobre el total, y si el par (207, 209) dispara.
  - Imprimir también los grupos de `usecases.DetectDuplicateGroups(mems, 0.09)`, para que la persona confirme cuáles son duplicados reales.

  Verificar que `go test ./tests/integration/...` sin la etiqueta no lo compila ni lo ejecuta.
- [X] T012 [US1] Ejecutar `GOMEMORY_CALIBRATION_DB=~/.local/share/gomemory/projects/gomemory-72eb21d7fd6b9e68/mem.db go test -tags calibration -run TestSaveGateCalibration -v ./tests/integration/`. Comprobar con `stat -f %m` antes y después que el mtime de `mem.db` no cambia. Pegar la tabla en research.md, en una sección nueva "R3.1 Resultado de la calibración (2026-09-…)".
- [X] T013 [US1] Elegir `T_title` y `T_body`: el par con menos avisos que cumpla avisos ≤ 10 % y recall = 100 % sobre 207/209 más los grupos que la persona confirme de T011. ⚠ Si ningún par cumple ambos, presentar la tabla a la persona y usar su elección. Declarar en application/usecases/save_gate.go las constantes `gateTitleThreshold`, `gateBodyThreshold` y `gateMaxCandidates = 3`, con un comentario que cite la medición (fecha, nº de memorias, % de avisos). Guardar con `save_memory` (tipo decision, `topic_key` "gate-umbrales") los valores y la tabla resumida.

### Gate

- [X] T014 [US1] Añadir en application/usecases/save_gate_test.go tests de `CheckNearDuplicates(snapshot []domain.Memory, saved domain.Memory, savedID int64) usecases.GateResult`, con memorias construidas en memoria (sin BD):
  - (a) Par tipo 207/209: mismo tipo, títulos iguales, contenido parecido, `savedID` nuevo → 1 candidato con su id, `TitleSim`, `BodySim` y `Checked=true`.
  - (b) `savedID` presente en el snapshot → `Updated=true` y 0 candidatos.
  - (c) `saved.Type == domain.Checkpoint` → 0 candidatos.
  - (d) Candidato de otro tipo → 0.
  - (e) Candidato checkpoint → 0.
  - (f) Cinco candidatos que disparan → como máximo 3, ordenados por `BodySim`↓, `TitleSim`↓ e `ID`↑.
  - (g) Candidato con `TopicKey` → se propaga en `NearDuplicate.TopicKey`.
  - (h) La misma entrada permutada → el mismo resultado.

  DEBE no compilar.
- [X] T015 [US1] Añadir en application/usecases/save_gate_test.go tests de `SaveWithGate(store, m *domain.Memory) (int64, usecases.GateResult, error)` con un doble local `gateStoreFake{insertID int64; listErr error; mems []domain.Memory}` que implementa `Insert` y `ListAll`:
  - (a) `listErr != nil` → `Insert` se llama, se devuelve el id, `Checked=false` y `Reason` no vacío.
  - (b) `Insert` falla → se devuelve el error.
  - (c) Camino normal → los candidatos equivalen a `CheckNearDuplicates(mems, *m, id)`.

  DEBE no compilar.
- [X] T016 [US1] Implementar en application/usecases/save_gate.go:
  - Los tipos `NearDuplicate{ID int64; Title, TopicKey string; TitleSim, BodySim float64}` y `GateResult{Checked, Updated bool; Reason string; Candidates []NearDuplicate}` (data-model.md).
  - `CheckNearDuplicates`, con la poda `GateSizeCompatible` por dimensión antes de calcular la intersección.
  - `SaveWithGate`: el `store` es una interfaz local `interface{ Insert(*domain.Memory) (int64, error); ports.MemoryFullLister }`. Hace `ListAll(m.Project)`, luego `Insert` y luego `CheckNearDuplicates`. Si `ListAll` falla → `Checked=false`, `Reason = "no se pudieron leer las memorias: " + err.Error()`, y se inserta igual.

  Hacer pasar T014 y T015.
- [X] T017 [P] [US1] Crear adapters/primary/cli/save_gate_notice_test.go con tests de `formatGateNotice(res usecases.GateResult) string` que comparan cadenas exactas de contracts/save-gate.md:
  - Sin candidatos → `""`.
  - `Updated` → `""`.
  - 1 candidato sin `topic_key` → la línea `⚠ Posible duplicado de #…`, más la línea `  (similitud léxica…)`.
  - Candidato con `topic_key` → sugiere `topic_key="<clave>"`.
  - `Checked=false` → la línea `ℹ Comprobación de duplicados no realizada (<motivo>): la memoria se guardó igual`.

  Similitudes con 2 decimales. DEBE no compilar.
- [X] T018 [US1] Crear adapters/primary/cli/save_gate_notice.go con `formatGateNotice` (títulos con `displayTitle`, o el título crudo si esa función no se exporta) y hacer pasar T017.
- [X] T019 [US1] Crear adapters/primary/cli/cmd_mcp_gate_test.go siguiendo el patrón de `TestMCPServer_SearchAndList_RecordUsage` en adapters/primary/cli/cmd_mcp_usage_test.go (servidor real, `mcp.NewInMemoryTransports()`, `Deps` con `MemoryRepo` y `SessionRepo`):
  - Llamar `save_memory` con `{type:"learning", title:"ACR proyecto completo v2.22.3: APPROVED sin defectos", content:<texto A>}`.
  - Llamar de nuevo con un título casi igual (`"ACR proyecto completo v2.22.3: APPROVED, sin defectos"`) y un contenido parecido.
  - Assert de que el texto de la segunda respuesta contiene `✓ Memoria guardada` y `Posible duplicado de #<id1>`.
  - Llamar una tercera vez con un tema nuevo y assert de que no contiene `Posible duplicado`.

  DEBE fallar.
- [X] T020 [US1] En adapters/primary/cli/cmd_mcp.go, en el manejador de `save_memory`: reemplazar `deps.MemoryRepo.Insert(&mem)` por `usecases.SaveWithGate(deps.MemoryRepo, &mem)` y anexar a `✓ Memoria guardada (id=%d)` el resultado de `formatGateNotice` (precedido de `\n` si no está vacío). Hacer pasar T019 sin romper `go test ./adapters/primary/cli/...`.
- [X] T021 [US1] Crear adapters/primary/cli/cmd_save_gate_test.go: redirigir `os.Stderr` a un pipe, ejecutar `CmdSave` dos veces con títulos casi iguales sobre un `Deps` montado como en adapters/primary/cli/cmd_save_test.go, y comprobar que stderr contiene `Posible duplicado de #` y que stdout mantiene `✓ Memoria guardada`. DEBE fallar.
- [X] T022 [US1] En adapters/primary/cli/cmd_save.go: usar `usecases.SaveWithGate(deps.MemoryRepo, &mem)` y escribir `formatGateNotice(res)` en `os.Stderr` si no está vacío. stdout y el código de salida no cambian. Hacer pasar T021 y los tests existentes de cmd_save_test.go.
- [ ] T023 [US1] Cierre de US1: `gofmt -l .`, `go vet ./...` y `go test ./...` en verde. Proponer el commit `feat(memory): gate de pre-escritura con umbrales medidos` con la lista de archivos (save_gate*.go, save_gate_notice*.go, cmd_mcp_gate_test.go, cmd_save_gate_test.go, cmd_mcp.go, cmd_save.go, tests/integration/save_gate_calibration_test.go, application/ports/context_builder.go y repositories.go si no entraron en T007, research.md). Esperar la aprobación.

**Checkpoint**: US1 funciona y se prueba sola (MVP).

---

## Phase 4: User Story 2 - Anclas sin evidencia en el contexto (Priority: P2)

**Goal**: sección `## 🧭 Anclas sin evidencia` con las anclas movidas, movidas ambiguas y huérfanas candidatas (contracts/context-sections.md).

**Independent Test**: almacén con una memoria anclada a un archivo borrado y otra a uno existente → solo la primera aparece.

- [X] T024 [P] [US2] Crear domain/anchor_evidence_test.go (`package domain`) con dos tablas:
  - `NormalizeAnchor(path, root string) (rel string, ok bool)`:
    - `""` → `ok=false`
    - `"/otro/sitio/x.go"` con `root="/r"` → `ok=false`
    - `"/r/a/b.go"` → `"a/b.go", true`
    - `"a/b.go"` → `"a/b.go", true`
    - `"./a/b.go"` → `"a/b.go", true`
    - `"../x.go"` → `ok=false`
  - `GradeAnchor(rel string, stat domain.AnchorStat, indexed []string) domain.AnchorEvidence`:
    - `StatFile` → `AnchorLive`
    - `StatOther` → `AnchorUnverifiable`
    - `StatMissing` con `indexed=["z/b.go"]` → `AnchorMoved` con `Candidate "z/b.go"` y `Matches 1`
    - `StatMissing` con `["z/b.go","y/b.go"]` → `AnchorMovedAmbiguous` con `Matches 2`
    - `StatMissing` con `indexed=nil` → `AnchorOrphanCandidate`
    - `StatMissing` con índice sin el nombre base → `AnchorOrphanCandidate`
    - la ruta faltante presente en `indexed` con la misma ruta NO cuenta como "movida"

  DEBE no compilar.
- [X] T025 [US2] Crear domain/anchor_evidence.go (solo stdlib: `path`, `path/filepath`, `strings`) con:
  - los tipos `AnchorGrade` (`AnchorLive`, `AnchorMoved`, `AnchorMovedAmbiguous`, `AnchorOrphanCandidate`, `AnchorUnverifiable`), `AnchorStat` (`StatMissing`, `StatFile`, `StatOther`) y `AnchorEvidence{Grade AnchorGrade; Path, Candidate string; Matches int}`;
  - `NormalizeAnchor`, que devuelve rutas con `/`;
  - `GradeAnchor`, que compara nombres base con `path.Base` e ignora la propia ruta.

  Un comentario de cabecera dice que el grado es una hipótesis, no una orden de borrado. Hacer pasar T024.
- [X] T026 [US2] Crear application/usecases/build_context_anchors_test.go (`package usecases_test`, persistencia real en `root := t.TempDir()`):
  - (a) Memoria decision con `Filepath "borrado.go"` (no existe en `root`) → la salida contiene `## 🧭 Anclas sin evidencia`, `[<id>]` y `huérfana candidata`.
  - (b) Memoria con `Filepath "vivo.go"` y el archivo creado con `os.WriteFile(filepath.Join(root, "vivo.go"), …)` → su id no aparece en la sección.
  - (c) `Filepath "/fuera/del/repo.ts"` → no aparece.
  - (d) Con `builder.Files` = doble local `filesFake{m map[string]string}` con `{"nuevo/borrado.go": "h"}` → `movida a \`nuevo/borrado.go\``.
  - (e) `builder.Files = nil` → la leyenda incluye `Sin índice de código: solo se comprobó el disco` y nunca `movida`.
  - (f) Checkpoint con ancla faltante → no aparece.
  - (g) Nueve huérfanas → como máximo 8 líneas de entrada.
  - (h) `builder.Budget` pequeño (p. ej. 300) → la salida no supera `Budget`.

  DEBE fallar.
- [X] T027 [US2] En application/usecases/build_context.go:
  - Añadir el campo `Files ports.IndexedFilesQuerier` (opcional, nil-safe, con comentario) a `Builder`.
  - Añadir `func (b *Builder) writeAnchorEvidence(sb *strings.Builder, mems []domain.Memory)`, llamada justo después del bloque `## 🔥 Memoria conectada a código activo`. Hace lo siguiente:
    - Recorre las memorias no checkpoint con `Filepath`, aplica `domain.NormalizeAnchor(m.Filepath, b.Root)` y resuelve el `AnchorStat` con `os.Stat(filepath.Join(b.Root, filepath.FromSlash(rel)))`.
    - Obtiene `indexed` de `b.Files.FileHashes(b.Project)`; si es nil, hay error o está vacío → `indexed = nil` y el flag "sin índice".
    - Filtra Moved, MovedAmbiguous y OrphanCandidate; ordena por id y corta a 8.
    - Escribe el encabezado y la leyenda de contracts/context-sections.md solo si hay entradas y `b.fits(sb, 120)`; cada línea con `fits()`.

  Hacer pasar T026 y todos los tests existentes de build_context*_test.go.
- [X] T028 [US2] En infrastructure/container.go, tras `contextBuilder.Graph = codeGraphRepo`, añadir `contextBuilder.Files = codeGraphRepo` con un comentario de una línea. Verificar con `go build ./...` y `go test ./infrastructure/...`.
- [ ] T029 [US2] Cierre de US2: `gofmt`, `vet` y `go test ./...` en verde. Proponer el commit `feat(context): evidencia de anclas sin archivo` con la lista de archivos. Esperar la aprobación.

**Checkpoint**: US1 y US2 funcionan cada una por su lado.

---

## Phase 5: User Story 3 - Enlaces relevantes y ranking de masa (Priority: P3)

**Goal**: masa PPR; Sinapsis ordenada por masa, sin checkpoints y sobre todas las relaciones (cierra el recorte a 20 de `ListRelations`); comando `mem mass` (contracts/context-sections.md, contracts/mass.md).

**Independent Test**: almacén con enlaces entre memorias normales y checkpoints → la sección sin checkpoints, en orden de masa, con salidas idénticas en dos ejecuciones; `mem mass` lista sin checkpoints y con la línea "qué NO afirma".

- [X] T030 [P] [US3] Crear domain/memory_mass_test.go (`package domain`) para `ComputeMass(g MassGraph, seeds map[int64]float64) map[int64]float64`:
  - (a) Σ masas = 1 ± 1e-9 en un grafo de 5 nodos.
  - (b) Con semilla en el nodo 1 de una cadena 1–2–3 simétrica: `masa(1) > masa(2) > masa(3)`.
  - (c) Nodo colgante (4→1 sin salida de 4 y semilla en 1): la suma sigue siendo 1 y la masa vuelve a la semilla.
  - (d) Arista solo B→A (supersedes, A sustituye a B), semilla en B: `masa(A) > 0`; con semilla en A: `masa(B) == (1−d)·p(B) = 0`.
  - (e) Permutar `Nodes` y `Edges` → un mapa idéntico, con comparación exacta de `float64`.
  - (f) Semillas vacías → equivale a semillas uniformes.
  - (g) Aristas con extremo fuera de `Nodes`, autolazo o peso ≤ 0 → se ignoran, sin panic.
  - (h) Grafo vacío → mapa vacío.

  DEBE no compilar.
- [X] T031 [US3] Crear domain/memory_mass.go (solo stdlib) con:
  - los tipos `MassEdge{From, To int64; Weight float64}` y `MassGraph{Nodes []int64; Edges []MassEdge}`;
  - las constantes `massDamping = 0.85`, `massTolerance = 1e-9` y `massMaxIter = 100`;
  - `ComputeMass`, que:
    - ordena y deduplica los nodos;
    - agrega las aristas por par dirigido, sumando peso;
    - normaliza por el peso saliente;
    - calcula `p` a partir de las semillas normalizadas (uniforme si no hay);
    - itera `rank' = d·Mᵀ·rank + [(1−d) + d·Σcolgantes]·p` recorriendo nodos y aristas en orden de id.

  Un comentario explica por qué la masa colgante vuelve a `p`. Hacer pasar T030.
- [X] T032 [P] [US3] Crear application/usecases/memory_mass_test.go (`package usecases_test`, sin BD):
  - `BuildMassGraph(mems []domain.Memory, rels []domain.Relation) domain.MassGraph`:
    - `related`/`compatible`/`scoped` → dos aristas con peso = `Confidence`;
    - `supersedes` A→B → solo la arista B→A;
    - `conflicts_with`/`not_conflict` → ninguna;
    - relación con un extremo checkpoint → ninguna;
    - relación con un extremo inexistente → ninguna;
    - `Confidence` 0 → ninguna;
    - `Nodes` = ids no checkpoint.
  - `RankMass(mems, rels, seeds)` → `[]MassEntry` ordenado por masa↓ e id↑, sin checkpoints.
  - `DescribeSeeds(sessionN, hotspotN int, uniform bool) string` → los cuatro textos de contracts/context-sections.md.

  DEBE no compilar.
- [X] T033 [US3] Crear application/usecases/memory_mass.go con `BuildMassGraph`, `MassEntry{ID int64; Mass float64}`, `RankMass`, `DescribeSeeds` y la constante `MassDisclaimer = "masa = centralidad en el grafo de memorias sembrado en %s; no mide importancia ni corrección"`. Hacer pasar T032.
- [X] T034 [US3] Crear application/usecases/build_context_mass_test.go (`package usecases_test`, persistencia real):
  - (a) Un `conflicts_with` entre A y B registrado con `RecordVerdict`, seguido de 25 relaciones `related` más nuevas entre otras memorias → `Conflictos sin resolver` sigue listando `[A]`. Cierra el recorte a 20 de `ListRelations`.
  - (b) Relaciones con un checkpoint en un extremo → ninguna línea de la sección Sinapsis contiene su id.
  - (c) Con sesión activa (`sessRepo` con una sesión y memorias con ese `SessionID` enlazadas) y otra relación más reciente entre memorias ajenas → las líneas que tocan memorias de la sesión aparecen antes.
  - (d) La sección termina con `_Orden: masa = centralidad…_` y el texto de semillas correcto.
  - (e) Dos `Build()` seguidos → salidas idénticas.
  - (f) Relación a una memoria borrada (`Delete`) → no aparece.
  - (g) Más de 12 relaciones válidas → como máximo 12 líneas.

  DEBE fallar.
- [X] T035 [US3] En application/usecases/build_context.go:
  - Si `b.Relations` satisface `ports.RelationFullLister`, usar `ListAll(b.Project)`; si no, `List(b.Project, 200)` como hoy.
  - Si `b.Lister` satisface `ports.MemoryFullLister`, cargar `allMems` con `ListAll` para el grafo y los títulos (`titleByID`); si no, usar `mems`.
  - Extraer el cálculo de ids en hotspot del bloque `## 🔥` a un helper `hotspotMemoryIDs(mems, providers) map[int64]int`, reutilizado por esa sección y por las semillas.
  - Semillas = memorias de `b.Session.Active` ∪ hotspot, con peso uniforme; si no hay ninguna, `nil` (uniforme).
  - Sinapsis: filtrar `related`/`supersedes` con los dos extremos no checkpoint y presentes; ordenar por `masa(a)+masa(b)`↓, `a`↑, `b`↑; tope 12; cada línea con `fits()`; leyenda final con `fmt.Sprintf(usecases.MassDisclaimer, DescribeSeeds(...))` en cursiva, con el formato del contrato.

  Hacer pasar T034 y todos los tests existentes de build_context*_test.go.
- [X] T036 [P] [US3] Crear adapters/primary/cli/cmd_mass_test.go:
  - Tests de `ParseMassFlags(args) (task string, top int, err error)`: el default de `--top` es 15; `--top 0` da error.
  - Test de `runMass(deps, task, top, w io.Writer) error` con persistencia real en `t.TempDir()`:
    - la salida empieza por `Masa de memorias — semillas:`;
    - no contiene ids de checkpoints;
    - termina con la línea `masa = centralidad…`;
    - dos ejecuciones dan salidas idénticas;
    - con `--task` sin resultados → `Sin memorias que coincidan`.

  DEBE no compilar.
- [X] T037 [US3] Crear adapters/primary/cli/cmd_mass.go con `CmdMass(deps *Deps, args []string)`, `ParseMassFlags` y `runMass`:
  - Carga `deps.MemoryRepo.ListAll(deps.Project)` y `deps.RelationRepo.ListAll(deps.Project)`.
  - Semillas: con `task`, `deps.MemoryRepo.Search(deps.Project, task, 20)` sin checkpoints, con peso `1/(rango+1)`; sin `task`, sesión activa ∪ hotspot vía `deps.CodeProviders`, o uniforme.
  - Llama a `usecases.RankMass` e imprime según contracts/mass.md (4 decimales).

  Registrar `case "mass": CmdMass(deps, args)` en adapters/primary/cli/dispatcher.go, junto a `case "pack"`. Añadir la línea `mem mass` al texto de ayuda del CLI donde se lista `mem pack` (buscarlo con `grep -rn "mem pack" adapters/primary/cli/*.go`). Hacer pasar T036.
- [ ] T038 [US3] Cierre de US3: `gofmt`, `vet`, `go test ./...` y `go test -race ./application/...` en verde. Proponer el commit `feat(context): masa de memorias y sinapsis por centralidad` con la lista de archivos. En el cuerpo, mencionar el cierre del recorte a 20 relaciones. Esperar la aprobación.

**Checkpoint**: US1, US2 y US3 funcionan cada una por su lado.

---

## Phase 6: User Story 4 - El paquete de contexto incluye vecinos con masa (Priority: P4)

**Goal**: `pack_build` y `mem pack build` añaden hasta 5 vecinos opcionales de mayor masa. Con `Relations == nil`, el paquete es idéntico (contracts/mass.md).

**Independent Test**: una tarea cuya memoria encontrada tiene una vecina enlazada que la búsqueda no devuelve → la vecina aparece como opcional; sin relaciones, el paquete es idéntico.

- [X] T039 [US4] Crear application/usecases/build_context_pack_mass_test.go (`package usecases_test`; reutilizar `newContextPackTestDeps` si es accesible desde el mismo paquete de test; si no, montar `persistence.Init(t.TempDir())`):
  - (a) Mismo `ContextRequest` con `Relations: nil` dos veces → `reflect.DeepEqual` de los dos packs, y del pack contra el obtenido sin el campo.
  - (b) Memoria A que coincide con la tarea "redis cache" y memoria B sin esas palabras, enlazada con `related` 1.0 → con `Relations: relRepo`, el pack contiene `memory:<B>` con `Priority == domain.PriorityOptional` después de todos los demás ítems.
  - (c) Siete vecinas enlazadas → como máximo 5 añadidas.
  - (d) `MaxTokens` justo para los candidatos de la búsqueda → las vecinas cuentan en `ItemsDiscarded` y se mantiene la invariante `ItemsRetrieved == Σ categorías`.
  - (e) Una vecina checkpoint nunca se añade.

  DEBE fallar.
- [X] T040 [US4] En application/usecases/build_context_pack.go:
  - Añadir `Relations ports.RelationFullLister` a `ContextRequest`, con un comentario (opcional; nil = comportamiento previo).
  - Tras armar `items` de memorias y antes de Spec Kit, si `req.Relations != nil`:
    - `all, err := memRepo.ListAll(req.Project)` y `rels, err := req.Relations.ListAll(req.Project)`; ante cualquier error, degradar en silencio al comportamiento previo.
    - Semillas = candidatos no checkpoint por rango `r` con peso `1/(r+1)`.
    - `RankMass` y tomar hasta 5 ids con masa > 0 que no sean candidatos ni checkpoint.
    - Construirlos con `newContextCandidate(m, 0, 1, req.MinRelevance)`, forzar `priority = domain.PriorityOptional` y añadirlos AL FINAL de `items`, con `retrieved++` por cada uno.

  Hacer pasar T039 y todos los tests existentes de build_context_pack_test.go.
- [X] T041 [US4] Cablear `Relations: deps.RelationRepo` en adapters/primary/cli/cmd_mcp.go (`pack_build`, dentro del `usecases.ContextRequest{…}`) y en adapters/primary/cli/cmd_pack.go (`cmdPackBuild`: `req.Relations = deps.RelationRepo`). `adapters/primary/tui/tui_usage.go` NO se toca. Verificar con `go test ./adapters/primary/cli/...`.
- [ ] T042 [US4] Cierre de US4: `gofmt`, `vet` y `go test ./...` en verde. Proponer el commit `feat(pack): vecinos por masa en el paquete de contexto`, o incluirlo en el de T038 si la persona lo prefiere. Esperar la aprobación.

**Checkpoint**: las cuatro historias funcionan cada una por su lado.

---

## Phase 7: Polish & validación contra el sistema real

**Purpose**: documentación con "qué NO afirma", calidad completa y validación en el sistema en ejecución (regla de trabajo 2: verde no es "funciona").

- [X] T043 [P] Documentar en README.md (sección `## Key Features` y sección `## CLI`, con `mem mass [--task] [--top]`) las tres capacidades. Cada una lleva su bloque "Qué NO afirma":
  - similitud léxica ≠ mismo tema;
  - ancla sin evidencia = hipótesis, no orden de borrado;
  - masa = centralidad sembrada, no importancia ni corrección.
- [X] T044 [P] Documentar en docs/MANUAL.md el aviso del gate (formatos de contracts/save-gate.md), la sección de anclas y `mem mass`. En docs/architecture.md, los puertos `MemoryFullLister`, `RelationFullLister` e `IndexedFilesQuerier`, las reglas de aristas de FR-016 y los umbrales medidos, con fecha. Todo en español.
- [X] T045 Ejecutar la validación V1 de specs/031-memory-gate-evidence-mass/quickstart.md (`gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, `go test ./...`, `go test -race ./adapters/secondary/persistence/... ./application/...`) y comparar con la línea base de T001. Todo fallo nuevo se corrige antes de seguir.
- [ ] T046 Ejecutar V2 de quickstart.md: compilar e instalar el binario, pedir a la persona que reinicie el cliente MCP, y confirmar que `mem version` (y el hash del binario que usa el MCP) es el nuevo ANTES de V4–V6 (regla de trabajo 3).
- [ ] T047 Ejecutar V4 de quickstart.md (gate) contra el MCP real y pegar la evidencia (las respuestas) en la sección "Evidencia" de este archivo. ⚠ El borrado de las memorias de prueba con `forget_memory` requiere la aprobación de la persona.
- [X] T048 Ejecutar V5 de quickstart.md (anclas) y contrastar a mano cada ruta listada con `ls`. Pegar la evidencia. Esperado: 0 falsos positivos y la 148 ausente.
- [X] T049 Ejecutar V6 de quickstart.md (masa y pack): `diff` de dos ejecuciones de `mem mass`, `mem mass --task "sinapsis cache" --top 10` (197, 200 y 202 en el top 10), la sección Sinapsis sin `[178..180] ↔ 172`, y `mem pack build … --json` con vecinos opcionales. Pegar la evidencia. Si SC-006 no se cumple, investigar la causa (semillas o aristas) antes de dar la historia por cerrada.
- [ ] T050 Guardar con `save_memory` las decisiones finales (reglas de aristas de FR-016 y comportamiento de la sección Sinapsis; los umbrales ya en T013) y llamar a `end_session` con el resumen: Objetivo, Hallazgos, Logrado, Próximos pasos y Archivos.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: sin dependencias.
- **Foundational (T002–T008)**: depende de T001. P0 (T002–T007) bloquea la calibración de US1 (T011–T013). T008 bloquea US1 (T016), US2 (T027) y US3 (T035), porque declara sus puertos.
- **US1 (T009–T023)**: depende de P0 y T008.
- **US2 (T024–T029)**: depende de T008. Es independiente de US1.
- **US3 (T030–T038)**: depende de T008. Es independiente de US1 y US2.
- **US4 (T039–T042)**: depende de T033 (`RankMass`). Es independiente de US1 y US2.
- **Polish (T043–T050)**: depende de las historias que se entreguen.

### Conflictos de archivo (serializar aunque las historias sean independientes)

- `application/usecases/build_context.go`: T027 (US2) y T035 (US3). Hacer una después de la otra.
- `adapters/primary/cli/cmd_mcp.go`: T020 (US1) y T041 (US4).
- `application/usecases/save_gate_test.go`: T009, T014 y T015 (misma historia, en orden).

### Within Each User Story

- Test en rojo → implementación → test en verde → cierre de fase con commit propuesto.
- Dominio puro antes que casos de uso; casos de uso antes que adaptadores.

### Parallel Opportunities

- T024 (US2, domain) ∥ T030 (US3, domain) ∥ T009 (US1, usecases): archivos distintos y sin dependencias entre sí.
- T032 ∥ T036 dentro de US3 (archivos distintos; T036 no compila hasta T037, pero se puede escribir en paralelo).
- T017 ∥ T014 dentro de US1.
- T043 ∥ T044 en Polish.

---

## Parallel Example: arranque tras la fase Foundational

```text
Tarea: "T009 [US1] tests de GateSimilarity en application/usecases/save_gate_test.go"
Tarea: "T024 [US2] tabla de NormalizeAnchor/GradeAnchor en domain/anchor_evidence_test.go"
Tarea: "T030 [US3] invariantes de ComputeMass en domain/memory_mass_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. T001 (línea base) → T002–T008 (P0 y puertos).
2. T009–T023 (US1).
3. **PARAR Y VALIDAR**: T045–T047 solo para el gate (V1, V2, V4).

### Incremental Delivery

1. Foundational → US1 (commit) → US2 (commit) → US3 (commit) → US4 (commit o incluido en el de US3).
2. Cada commit se revierte por separado (FR-023). Si hay que recortar, el orden de prioridad es US1 → US2 → US3 → US4.

### Reglas de la persona que aplican durante la ejecución

- Antes de cada `git commit`: mostrar los archivos y el mensaje y esperar la aprobación. Nunca incluir CLAUDE.md, `.env*`, certificados ni archivos de IA.
- Antes de `git push`: confirmar la URL remota.
- `forget_memory` es irreversible: siempre con aprobación.

---

## Línea base

Ejecutada el 2026-09-13: `gofmt -l .` y `go vet ./...` sin salida. `golangci-lint` no está instalado en el entorno. `go test ./...` alcanzó las suites: fallaron únicamente `adapters/primary/cli.TestLatestReleaseTag` y `tests/integration.TestUpdateIntegration` porque el sandbox impide abrir el listener IPv6 de `httptest`; no son fallos funcionales del árbol inicial.

## Revisión de implementación (2026-09-13)

Contraste del árbol de trabajo con spec, contratos y tareas, hecho antes de continuar. Encontró trabajo de US2 y US3 sin marcar y sin sus tests, y defectos que las casillas no reflejaban:

| Hallazgo | Requisito | Cierre |
|----------|-----------|--------|
| `ComputeMass` sumaba floats en orden de `map` y de entrada: la salida cambiaba al permutar (`0.36718667097344593` frente a `…604`) | FR-015, SC-007 | Nodos en slice ordenado, aristas agregadas por par con pesos ordenados, sumas en orden de id. `domain/memory_mass_invariants_test.go` (8 casos) |
| Sinapsis incluía aristas con checkpoints y huérfanas | FR-018, SC-005 | Filtro por extremos presentes y no checkpoint. `TestBuild_SinapsisExcluyeCheckpointsYHuerfanas` |
| Sinapsis siempre sembrada uniforme | FR-018 | `usecases.ContextSeeds` (sesión activa + hotspots), compartido con `mem mass`. `TestBuild_SinapsisPriorizaLaSesionActiva` |
| Sinapsis sin `fits()`, y se omitía si `List` fallaba aunque `ListAll` funcionara | SC-008 | `loadRelations`, leyenda reservada junto al encabezado, cada línea bajo presupuesto |
| `mem mass` fuera de contrato: sin tipo ni título, sin semillas de sesión, mensaje distinto, línea final en cursiva | contracts/mass.md | Reescrito. `cmd_mass_test.go`, más la ayuda del CLI |
| `itoa` hecho a mano | Simplicidad | `strconv.Itoa` |
| La cabecera de anclas reservaba 120 bytes para unos 170 | SC-008 | Se reserva la longitud real |
| US2 sin test de integración (T026) | TDD | `build_context_anchors_test.go` (4 casos, incluido el presupuesto) |
| US4 sin empezar | FR-020, FR-021 | `massNeighborCandidates` y cableado MCP/CLI. `build_context_pack_mass_test.go` (4 casos) |
| 2 avisos `errcheck` en `cmd_mass.go` | memoria 141 | Errores de escritura propagados |
| C-001 (ACR `acr_0106584b`, HIGH): cualquier error de `os.Stat` se tomaba como ausencia, y un error de permisos o de E/S acababa como ancla movida o huérfana | FR-010, SC-004 | `anchorStatOf`: solo `fs.ErrNotExist` es ausencia; el resto queda no verificable. `TestAnchorStatOf`, `TestBuild_AnclasSinEvidencia_ErrorDePermisoNoEsHuerfana` |
| C-002 (ACR, MEDIUM): solo se clasificaban anclas de las 100 memorias recientes | FR-010, FR-011 | `writeAnchorEvidence` recibe todas las memorias. `TestBuild_AnclasSinEvidencia_IncluyeMemoriasFueraDeLas100` |
| S-001 (ACR, LOW, un solo revisor): `titleByID` se arma solo con las 100 recientes | — | Validado: no es un defecto. El test existente `TestBuild_ConflictoConMemoriaFueraDeLaVentana` fija el marcador `(memoria previa)` para extremos fuera de la ventana. Mostrar el título exigiría autorizar la modificación de ese test. |
| SC-006 inalcanzable con los datos reales (197/200/202 sin relaciones) | SC-006 | Enmendado por decisión de la persona. `TestRankMass_TareaSubeEnlazadasYAisladaConservaSuCuota` |
| S-001 (revisión, MEDIUM): el gate solo callaba ante la misma `topic_key` porque el upsert la fusionaba antes | FR-002, US1-3 | Defecto latente corregido: se omiten los candidatos con la misma `topic_key`. `TestCheckNearDuplicates_MismaTopicKeyNoAvisa` |
| S-002 (revisión, MEDIUM): el dedup falla cerrado y el gate falla abierto | FR-005, FR-008, constitución V.5 | Por diseño, sin cambio. El dedup es una garantía de integridad del guardado (fallar rápido, P0); el gate es consultivo y declara que no pudo comprobar. Los errores transitorios ya los absorbe el `busy_timeout` de 5 s de SQLite. |
| S-003 (revisión, LOW): el gate comparaba el texto sin redactar contra una instantánea redactada | FR-001 | Corregido: se aplica la misma redacción que `insertMemory` antes de comparar. `TestCheckNearDuplicates_ComparaElTextoRedactado` |
| S-004 (revisión, LOW): las semillas de `mem mass --task` se toman de 20 resultados fijos | contracts/mass.md | Por diseño (el mismo tope que `pack_build`); ahora documentado en el contrato. La cabecera ya muestra N. |
| S-005 (revisión, LOW): `massNeighborCandidates` devuelve nil en silencio ante errores | FR-020, FR-021 | Por diseño: sigue la convención de los extras del pack (Spec Kit, grafo de código), que degradan en silencio sin fallar el pack. Señalarlo exigiría ampliar `ContextStats`, fuera del alcance de la 031. |
| S-006 (revisión, LOW): la sección de anclas cortaba en 8 sin decir cuántas quedaban | SC-008, coherencia de secciones | Corregido: línea `- (+N anclas más; usa search_memories/get_memory)`. `TestBuild_AnclasSinEvidencia_IndicaCuantasQuedanFuera` |
| S-003 (2ª revisión, LOW): la instantánea lleva la nota `[impacto: … hotspot …]` que la memoria nueva no tiene, y diluía el Jaccard en cuerpos cortos | FR-001 | Corregido: el formato pasa a `domain.ImpactAnnotation` (la persistencia produce la misma cadena de antes) y el gate descuenta la nota con `domain.StripImpactAnnotation`. `TestStripImpactAnnotation`, `TestCheckNearDuplicates_IgnoraLaAnotacionDeImpacto` |
| S-004 (2ª revisión, LOW; en realidad un defecto de persistencia): `nullableTopic` guardaba la `topic_key` sin recortar mientras el dedup la buscaba recortada, así que tres guardados con la misma clave creaban 3 filas | FR-002; dedup por `topic_key` (feature 008) | Corregido en la causa: la clave se guarda recortada. El almacén real no tenía claves con espacios ni repetidas. `TestInsert_TopicKeyConEspaciosNoDuplica` |

Residuales de la 3ª revisión, sobre el delta anterior (sin reabrir sus veredictos):

| Hallazgo | Cierre |
|----------|--------|
| S-001 (LOW): `StripImpactAnnotation` cortaba desde el último prefijo si el texto acababa en el sufijo, aunque en medio hubiera texto real | Solo quita la nota con su forma exacta al final (expresión regular de la nota completa), manteniendo el formato de escritura en `domain`. `TestStripImpactAnnotation_SoloQuitaLaNotaExacta` |
| S-002 (LOW): la nota solo se descontaba en el candidato | Se descuenta también en la memoria nueva. `TestCheckNearDuplicates_DescuentaLaNotaEnAmbosLados` |
| S-003 (LOW): el hook `plan-entered` seguía ignorando el fallo al escribir el marcador | `claimPlanEntryMarker`, compartido por las dos entradas a plan. Sin registro, `plan-entered` entrega el recordatorio en lugar del documento. `TestClaimPlanEntryMarker` (el hook termina con `os.Exit`, así que el test cubre el helper) |

Residuales de la 4ª revisión, sobre el delta anterior (sin reabrir sus veredictos ni duplicar los INFO ya registrados):

| Hallazgo | Cierre |
|----------|--------|
| S-001 (LOW): el patrón excluía `]` del símbolo, así que la nota de un símbolo como `Cache[T]` no se descontaba | El símbolo sale de `h.Name` del grafo de código, un identificador sin espacios (hotspots reales: Close, String, Open, Scan, Init, NewMemoryRepository). Se reconoce como `\S+`: acepta corchetes y sigue rechazando un texto citado, que tiene espacios. `TestStripImpactAnnotation_SimboloConCorchetes`; `TestStripImpactAnnotation_SoloQuitaLaNotaExacta` sigue en verde |
| S-002 (LOW): la ruta por prompt reiniciaba el episodio de plan-guard antes de saber si registraba la entrada; sin marcador, cada prompt en modo plan lo reiniciaba | El reinicio pasa a después de un `claimPlanEntryMarker` exitoso. `TestPlanEntryFromPrompt_SinMarcadorNoReiniciaElEpisodio`. En el hook `plan-entered` se mantiene antes, por diseño: cada llamada a `EnterPlanMode` es una entrada explícita y siempre abre episodio |

Los hallazgos S-001/S-002 de esa misma revisión afectan al hook de entrada a plan por prompt (el arreglo de `plan_entry`, fuera del alcance de la 031) y al hook existente `plan-entered`. Se corrigieron con `planEntryDocument`, que declara cuando falta el historial, y con la degradación a un recordatorio si el marcador no se puede escribir: `TestPlanEntryFromPrompt_SinHistorialLoDeclara` y `TestPlanEntryFromPrompt_SinMarcadorDegradaAlRecordatorio`.

Nota: se intentó que los títulos de Sinapsis salieran de todas las memorias, pero rompía el test existente `TestBuild_ConflictoConMemoriaFueraDeLaVentana`, que no se modifica. Se revirtió: los extremos fuera de las 100 recientes conservan su marcador. P0 usa `memory_dedup_internal_test.go` y `memory_dedup_session_test.go` en lugar de un único `memory_dedup_test.go`.

## Evidencia

Validación del 2026-09-13 contra el almacén real, con el binario compilado en el scratchpad (`gomemory 2.22.3` + cambios sin commit). No es el binario instalado, así que el MCP real sigue sin reiniciar: T046 y T047 siguen pendientes.

**V5 — anclas (T048).** `mem context` lista una sola entrada: `[157] «ACR del worktree commit 981ae2f…» — \`HEAD\` → huérfana candidata`. `ls HEAD` falla, así que es correcta: 0 falsos positivos. La 148 (ruta absoluta fuera del repo) no aparece. Observación: `HEAD` es una referencia de git guardada como `filepath`; la leyenda de hipótesis cubre ese caso.

**Revalidación tras corregir C-001/C-002 y enmendar SC-006 (2026-09-13).**
- V5: con todas las memorias, la sección lista 2 entradas: `[82]` `specs/029-fix-acr-verdict-integrity/spec.md` (spec retirada, memoria 93) y `[157]` `HEAD`. `ls` confirma que ninguna existe: 0 falsos positivos.
- SC-006 enmendado: con `--task "sinapsis cache"`, la 155 (enlazada a la 154, que es un resultado) queda en el puesto 6 con masa 0.0553; sin tarea, su masa es 0.0000. 197 (0.0066) y 200 (0.0072), sin relaciones, quedan por debajo de su cuota de semilla. Cumple.
- Calidad: `gofmt`/`vet` limpios, `go test ./...` con 13 paquetes ok, `golangci-lint` v2.13.2 con 0 avisos, `-race` ok en `persistence` y `application`.

**V6 — sinapsis y masa (T049).**
- Sección Sinapsis: 0 checkpoints; ya no aparece `[178..180] ↔ 172`; leyenda `…sembrado en sesión activa (2) + anclas a hotspot (2)…`.
- `mem mass --top 15` dos veces → `diff` vacío (IDÉNTICAS). Sin checkpoints en el ranking.
- `mem mass --task "sinapsis cache" --top 10` → **SC-006 no se cumple**: solo 202 entra (#8); 197 y 200 quedan fuera. Causa verificada en solo lectura: 197, 200 y 202 tienen **0 relaciones** en `memory_relations`. Son semillas aisladas en los puestos 8 y 9 (base 0) de 11 resultados, con masa ≈ su cuota de semilla (~0.04), por debajo del décimo (0.0473). La masa no puede subir un nodo sin enlaces. La premisa del plan ("trabajo de sinapsis enlazado") no se da en los datos. ⚠ Enmendar SC-006 es decisión de la persona.
- Con semillas concentradas, casi todas las aristas empatan a masa 0 y el desempate por id mostraba primero las más viejas. Corregido: el empate va por relación más reciente (contrato actualizado y `TestBuild_SinapsisEmpateDeMasaPrefiereLaMasReciente`).

---

## Notes

- 50 tareas. Tres hojas marcadas ⚠ requieren a la persona: T006 (fusión 207/209), T013 (solo si la calibración no cumple ambos objetivos) y el borrado de las memorias de prueba en T047.
- `[P]` = archivos distintos y sin dependencias pendientes.
- Verificar que cada test falla antes de implementar.
