# Research: Gate de pre-escritura, evidencia de anclas y masa de memorias

**Feature**: `031-memory-gate-evidence-mass` · **Fecha**: 2026-09-13

Toda cifra de este documento se midió en solo lectura sobre el almacén real
`~/.local/share/gomemory/projects/gomemory-72eb21d7fd6b9e68/mem.db` (152
memorias, 110 relaciones, 420 archivos indexados), o se leyó del código en
`HEAD f4fcf29`.

## R0. Estado del almacén real (línea base)

| Dato | Valor | Consecuencia para el diseño |
|------|-------|-----------------------------|
| Memorias por tipo | checkpoint 74 · decision 24 · bugfix 17 · learning 16 · discovery 14 · pattern 4 · preference 2 · architecture 1 | El gate compara contra decenas de memorias por tipo: la comparación exacta sobra (no hace falta MinHash). |
| Relaciones | `related` 0.5 ×109 · `supersedes` 1.0 ×1 | Casi toda la señal son sinapsis automáticas: cuota ambigua, no aristas juzgadas. |
| Relaciones con un checkpoint en un extremo | 27 | Justifica excluir checkpoints del grafo y de la sección. |
| Relaciones huérfanas (un extremo ya no existe) | **65 de 110** | El grafo de masa DEBE ignorar aristas con extremos ausentes. Sin esto, la mayoría de la masa se iría a nodos fantasma. |
| Anclas de memorias no checkpoint | 51 relativas · 1 absoluta fuera del repo (148) | La 148 es el caso "no verificable". Las anclas de checkpoints son absolutas dentro del repo. |
| Memorias con `topic_key` | 25 | El gate debe saber sugerir la clave existente. |

## R1. Anomalía 207/209: prerrequisito, fuera de la feature

**Hechos verificados**:

- 207 y 209 tienen el mismo `project`, el mismo `type` (learning), la misma sesión (b2b16ddf), `topic_key` NULL y un título idéntico byte a byte (52 bytes). 209 se creó a las 21:00:57, cuatro minutos después que 207. `dedup_window_days` = 7.
- Con esos datos, `findDuplicateTx` (título exacto, mismo tipo, dentro de la ventana) DEBÍA devolver 207. No hubo cambios relevantes en esa función desde el 2026-08-25: b4a0502 solo quitó una cláusula redundante.
- Solo dos sentencias escriben `UPDATE memories`: el upsert del dedup (`memory.go:105`) y `UpdateMemoryContent` (`memory.go:551`). Esta última solo la llaman `ImportADRs` (no aplica: learning no se exporta como ADR) y `ConsolidateMemories` (solo agrupa por `topic_key` o checkpoints: no aplica). Por tanto, 209 ya tenía el título T al crearse. El `updated_at` de las 08:01:45 es un upsert por título que eligió 209 por `ORDER BY id DESC`, no un cambio de título.

**Hipótesis que quedan, por orden de probabilidad**:

1. **Fail-open silencioso en `findDuplicateTx`**: `QueryRow(...).Scan(&id); err == nil` trata CUALQUIER error (`SQLITE_BUSY`, conexión inválida, contexto) como "no hay duplicado" y sigue con el INSERT. Es el mismo defecto que el principio "el silencio no debe parecerse al fallo" pretende evitar.
2. **Inserción por una vía sin dedup**: `ImportMemory` (`memory.go:683`) no llama a `findDuplicateTx`.

**Cierre propuesto (bugfix directo, memoria 93, antes de calibrar el gate)**:

1. Test de reproducción en `adapters/secondary/persistence/memory_dedup_test.go`: dos `Insert` con mismo título, tipo y sesión devuelven el mismo id (la ruta feliz). Otro test inyecta un error distinto de `sql.ErrNoRows` en la consulta de dedup (p. ej., con la tabla bloqueada por otra conexión en `BEGIN IMMEDIATE`) y fija el comportamiento esperado.
2. Si se confirma la hipótesis 1: `findDuplicateTx` distingue `sql.ErrNoRows` (no hay duplicado) de otro error (propaga y aborta la transacción: fallar rápido, constitución V.5). Commit `fix(persistence)` propio.
3. Fusionar 207/209 (`forget_memory 207` o `209`) requiere aprobación explícita de la persona, porque es irreversible.

**Relación con el gate**: el gate NO depende de este cierre para funcionar, porque también detecta el título exacto. Pero sí para calibrarse con honestidad: si el dedup exacto falla de forma intermitente, los pares "título idéntico" inflarían la tabla de calibración.

## R2. Gate: dónde vive y qué lee

**Decision**: caso de uso `application/usecases/save_gate.go` con dos funciones:

- `CheckNearDuplicates(all []domain.Memory, saved domain.Memory, savedID int64) []NearDuplicate`. Es pura y testeable.
- `LoadGateCandidates(lister ports.MemoryFullLister, project string) ([]domain.Memory, error)`. Hace la carga.

El adaptador (MCP/CLI) carga la instantánea ANTES del `Insert`, inserta y evalúa DESPUÉS.

**Rationale**:

- `ports.MemoryLister.List` está limitado a 200 filas (`ListAll` existe precisamente por eso). Hoy hay 152, pero el gate empezaría a perder memorias antiguas en silencio al crecer el almacén. Se añade un puerto estrecho `MemoryFullLister { ListAll(project) }` en `application/ports/context_builder.go`. `ports.MemoryRepository` ya lo satisface sin cambios (memoria 167: puertos estrechos, sin ampliar interfaces con dobles de prueba existentes).
- La instantánea previa al `Insert` permite detectar la actualización sin tocar la persistencia: si el `id` devuelto ya estaba en la instantánea, fue un upsert (por `topic_key` o por título exacto) y NO se avisa (FR-007, US1-3). `findDuplicateTx`/`insertMemory` no se modifican.
- **`topic_key`**: si el guardado trae una clave que ya existe, es un upsert y la regla anterior lo silencia. Si trae una clave NUEVA, el gate SÍ evalúa: una clave recién inventada no resuelve la identidad frente a memorias sin ella. De hecho, `findDuplicateTx` salta el dedup por título cuando hay `topic_key`, así que una clave nueva es justo la vía por la que se cuela un duplicado. Los candidatos con `topic_key` sí se comparan, y el aviso sugiere reutilizar esa clave. Esto precisa la redacción de FR-002 en la spec.
- Error al cargar la instantánea → `Checked=false` y el aviso "comprobación no realizada" (FR-008). Nunca se bloquea el guardado.

**Alternatives considered**:

- Gate dentro de `insertMemory` (persistencia): rechazado. Mezcla política consultiva con la garantía transaccional y obliga a cambiar la firma de `Insert` (retorno extra), lo que rompe los dobles de prueba.
- Reutilizar `DetectDuplicateGroups`: rechazado. Agrupa por union-find con un umbral de recall (0.09) y es transitivo. El gate necesita pares directos con un umbral de precisión.

## R3. Gate: similitud, filtro de tamaño y umbrales

**Decision**:

- Dos similitudes Jaccard con `tokenize`/`jaccardSimilarity` existentes: `simTitle` (títulos) y `simBody` (título+contenido). Un candidato dispara si `simTitle ≥ T_title` **o** `simBody ≥ T_body`.
- Orden: `simBody` descendente, después `simTitle` descendente, después `id` ascendente. Se devuelven como máximo 3.
- **Filtro de tamaño** (poda exacta, sin pérdida de recall): como `J(A,B) ≤ min(|A|,|B|)/max(|A|,|B|)`, si ese cociente es menor que el umbral de la dimensión, el par no puede disparar en ella y se omite el cálculo de la intersección.
- Exclusiones: memoria guardada de tipo checkpoint (no se evalúa), candidatos checkpoint, candidatos de otro tipo, y el propio `savedID`.
- Umbrales como **constantes del caso de uso** con el comentario de su medición, igual que `duplicateSimilarityThreshold`. No son configuración de entorno (constitución IV): son parte del algoritmo y se recalibran con el mismo test.

**Calibración (tarea previa a fijar las constantes)**: test de integración `tests/integration/save_gate_calibration_test.go` con `//go:build calibration`. Lee la base indicada en `GOMEMORY_CALIBRATION_DB` con el driver SQLite en modo `?mode=ro` y un `SELECT` plano, SIN `persistence.Open`, porque `Open` migra y escribiría en el almacén real. Para cada memoria no checkpoint, en orden de id, calcula el máximo `simTitle`/`simBody` contra las anteriores del mismo tipo. Imprime la tabla `umbral × avisos (%) × recall de pares confirmados`. Los pares confirmados son 207/209 más los grupos de `DetectProjectDuplicates` que la persona confirme al ver la tabla. Se versiona detrás de la etiqueta de compilación para que la medición sea reproducible y no corra en `go test ./...`.

**Objetivo** (SC-002): avisos ≤ 10 % de los guardados históricos y recall = 100 % de los pares confirmados. Si chocan, la tabla se presenta a la persona y ella elige.

**Alternatives considered**: MinHash/LSH (descartado en el plan: con decenas de memorias por tipo el cálculo exacto cuesta microsegundos); reutilizar 0.09 (descartado: dispararía en casi cada guardado, que es lo que motivó la feature); una sola similitud (descartado: un título casi idéntico con contenidos reescritos es el caso 207/209, con `simBody` bajo por prosa distinta).

## R3.1 Resultado de la calibración (2026-09-13)

El calibrador de solo lectura recorrió 82 memorias no checkpoint. Se fijaron
`T_title = 0.70` y `T_body = 0.25`: 4 avisos (4.9 %) y el par 207/209 fue
detectado. Cumple el máximo de 10 % sin requerir una decisión adicional.

## R4. Evidencia de anclas

**Decision**:

- `domain/anchor_evidence.go`, pura: `GradeAnchor(path, root string, isFile bool, indexed []string) AnchorEvidence`, donde `indexed` son las rutas relativas del índice.
- Reglas en orden:
  1. Ruta vacía → no verificable.
  2. Absoluta fuera de `root` → no verificable.
  3. Absoluta dentro de `root` → se normaliza a relativa.
  4. `isFile` → vigente.
  5. Si falta: nombre base en exactamente 1 ruta del índice → movida (con esa ruta); en ≥2 rutas → movida ambigua (con el número de rutas); en ninguna → huérfana candidata.
- Una ruta que existe pero es un directorio → no verificable.
- `Build()` resuelve `isFile` con `os.Stat(filepath.Join(Root, rel))`. `build_context.go` ya importa `os`/`path/filepath` para `WriteFile`, así que no se añade una dependencia de E/S nueva a la capa. Los tests usan `t.TempDir()` como `Root` con archivos reales.
- Puerto estrecho `IndexedFilesQuerier { FileHashes(project) (map[string]string, error) }` y campo opcional `Builder.Files`, cableado en `infrastructure/container.go` con `codeGraphRepo`. Las claves son rutas relativas con `/` (`index_project.go`: `filepath.Rel` + `ToSlash`). Con `Files == nil`, error o índice vacío, solo se mira el disco: nunca se afirma "movida", y la sección lo dice (FR-013).
- Alcance: todas las memorias no checkpoint (`ListAll`), no solo las 100 recientes que listan las secciones por tipo. Corregido por C-002 de la ACR `acr_0106584b`: un ancla antigua rota es justo la que nadie revisa.
- Solo `fs.ErrNotExist` cuenta como ausencia. Un error de permisos o de E/S deja el ancla como no verificable (`anchorStatOf`), porque no prueba que el archivo falte (C-001 de la misma ACR).
- Sección `## 🧭 Anclas sin evidencia`, ubicada tras "Memoria conectada a código activo". Máximo 8 entradas, cada línea bajo `fits()`, en orden de id ascendente para que sea determinista. Lleva la leyenda fija de FR-011.

**Alternatives considered**: reutilizar `domain/liveness.go` (rechazado: modela la vitalidad de canales de inyección, no de anclas); una ruta "movida" por similitud de contenido (fuera de alcance: el nombre base es la evidencia más barata y declarable).

## R5. Masa: algoritmo

**Decision**: `domain/memory_mass.go`, solo stdlib.

- Entrada: `MassGraph{Nodes []int64; Edges []MassEdge{From, To int64; Weight float64}}` y semillas `map[int64]float64`.
- Nodos ordenados por id. Aristas agregadas por par dirigido (se suman los pesos) y ordenadas. Se descartan autolazos, pesos ≤ 0 y extremos fuera de `Nodes`.
- `p` = semillas normalizadas a 1, ignorando las que no están en `Nodes`. Semillas vacías → uniforme.
- Iteración: `rank' = d·Mᵀ·rank + [(1−d) + d·Σ rank(colgantes)]·p`, con `d = 0.85`. `M` normaliza por el peso saliente de cada nodo.
- Corte cuando `‖rank' − rank‖₁ < 1e-9` o tras 100 iteraciones. Las sumas se hacen en orden de id, lo que da determinismo bit a bit.
- Invariantes: Σ masa = 1 (±1e-9); la masa de los nodos colgantes vuelve a `p` (FR-017).

**Rationale**: es PageRank personalizado estándar. El reinicio a `p` en lugar de a la uniforme es lo que hace que la masa responda a "lo que importa ahora". Devolver la masa colgante a `p` evita que las memorias sin enlaces salientes (casi todas, con 110 aristas para ~78 nodos) la regalen al grafo entero.

## R6. Masa: grafo, semillas e integración

**Decision**:

- `application/usecases/memory_mass.go`:
  - `BuildMassGraph(mems, rels)`: nodos = memorias no checkpoint; aristas según FR-016, con relaciones huérfanas ignoradas (65 de 110 hoy).
  - `RankMass(mems, rels, seeds) []MassEntry`: ordena por masa descendente y después por id ascendente.
  - `DescribeSeeds(...) string`: el texto `{semillas}` de la leyenda.
  - Lo usan `Build()`, `BuildContextPack` y `mem mass`: una sola implementación.
- **`Build()`**:
  - Carga TODAS las relaciones y memorias por aserción de tipo a `ports.RelationFullLister`/`ports.MemoryFullLister`, el mismo patrón que `Topics`. Si no se satisfacen (dobles de prueba), cae a `List` como hoy.
  - Semillas: memorias de la sesión activa más memorias ancladas a hotspot (`ImpactFor`); si no hay ninguna, uniforme.
  - La sección Sinapsis mantiene los tipos que muestra hoy (`related`, `supersedes`), excluye las aristas con checkpoint o con extremos ausentes, y se ordena por `masa(a)+masa(b)` descendente (desempate por ids). El tope sigue siendo 12. Cada línea pasa por `fits()`. Termina con la leyenda de FR-019.
- **`BuildContextPack`**:
  - Campo opcional `ContextRequest.Relations ports.RelationFullLister`. Con nil no se ejecuta nada nuevo (FR-021).
  - Con valor: semillas = candidatos no checkpoint de `Search`, con peso `1/(rango+1)`. Nodos y tipos vía `memRepo.ListAll` (una consulta en lugar de N `Get`). Se añaden hasta 5 nodos de mayor masa que no estén entre los candidatos y tengan masa > 0, como `PriorityOptional`, AL FINAL de `items`. Así son lo primero que se descarta por presupuesto (US4-3). Cuentan en `ItemsRetrieved`.
  - Se cablea en `pack_build` (MCP) y en `mem pack build` con `deps.RelationRepo`. La TUI no se toca y queda en nil, con el comportamiento idéntico.
- **`mem mass [--task T] [--top N]`** (`adapters/primary/cli/cmd_mass.go`, registrado en `dispatcher.go`):
  - Con `--task`, semillas = `Search(project, T, 20)` con peso `1/(rango+1)`; sin él, las mismas que `Build()`.
  - `--top` vale 15 por omisión.

**Rationale de ListAll en Build (cierra un defecto latente)**: `ListRelations` convierte cualquier `limit > 50` en **20** (`relation.go:72`), y `Build()` pide 200. Hoy la sección de conflictos y la de sinapsis ven solo las 20 relaciones más recientes. Un `conflicts_with` antiguo desaparece del contexto, contra el contrato "los conflictos nunca se recortan". Como esta feature cambia justo esa lectura, el defecto se paga en el mismo cambio, con un test: un conflicto con más de 20 relaciones posteriores sigue apareciendo.

**Alternatives considered**: ordenar por confianza (rechazado: 109 de 110 valen 0.5, así que no ordena nada); ampliar `RelationLister` con `ListAll` (rechazado por la memoria 167: rompe dobles de prueba); corregir el recorte en `ListRelations` (rechazado como vía principal: cambia un contrato de persistencia usado por otros llamadores; la aserción a `ListAll` resuelve el caso del contexto sin tocarlo).

## R7. Rendimiento y determinismo

- Gate: una consulta `ListAll` (152 filas) más ≤ 24 Jaccard por guardado, con la poda por tamaño. Del orden de milisegundos (SC-010). Sin caché en proceso (constitución: no cachear valores que cambian en caliente).
- Masa: ~80 nodos × 100 iteraciones como máximo. Microsegundos.
- Determinismo: ninguna salida itera un `map` sin ordenar. Los tests comparan dos ejecuciones y una entrada permutada (SC-007).

## R8. Pruebas y entregas

- Tests junto al código (`*_test.go` en el mismo paquete): es la práctica vigente del repo, que ya se aparta del `tests/unit/` de la constitución. Los tests con BD real y el calibrador van en `tests/integration/`.
- Cuatro commits en orden: (P0) `fix(persistence)` de la anomalía, si se confirma; (1) `feat(memory): gate de pre-escritura`; (2) `feat(context): evidencia de anclas`; (3) `feat(context): masa de memorias`, incluido el cierre del recorte de relaciones. Cada uno se puede revertir por separado (FR-023).
