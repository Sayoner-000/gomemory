# Data Model: Gate de pre-escritura, evidencia de anclas y masa de memorias

**Feature**: `031-memory-gate-evidence-mass`

Ninguna entidad de esta feature se persiste. No hay migraciones ni tablas
nuevas. Todo se deriva en lectura de `memories`, `memory_relations` y
`code_files`.

## Gate de pre-escritura (US1)

### NearDuplicate (`application/usecases/save_gate.go`)

| Campo | Tipo | Regla |
|-------|------|-------|
| `ID` | int64 | Id de la memoria parecida. Nunca es el id recién guardado. |
| `Title` | string | Título mostrado (`displayTitle`). |
| `TopicKey` | string | Clave de tema del candidato, si tiene. El aviso sugiere reutilizarla. |
| `TitleSim` | float64 | Jaccard de tokens de título, en [0,1]. |
| `BodySim` | float64 | Jaccard de tokens de título+contenido, en [0,1]. |

Un candidato existe solo si es del mismo `Type` que la memoria guardada, no es
checkpoint y cumple `TitleSim ≥ T_title` o `BodySim ≥ T_body`. Se devuelven
como máximo 3, ordenados por `BodySim`↓, `TitleSim`↓ e `ID`↑.

### GateResult

| Campo | Tipo | Regla |
|-------|------|-------|
| `Checked` | bool | false si no se pudo cargar la instantánea (FR-008). |
| `Reason` | string | Motivo breve cuando `Checked=false`. |
| `Updated` | bool | true si el id devuelto por `Insert` ya existía (upsert). En ese caso no hay candidatos (FR-007). |
| `Candidates` | []NearDuplicate | Vacío cuando no hay casi-duplicados. |

Estados posibles: `Checked=false` → aviso "comprobación no realizada".
`Checked ∧ Updated` → sin aviso. `Checked ∧ ¬Updated ∧ len>0` → aviso de
posibles duplicados. `Checked ∧ ¬Updated ∧ len=0` → sin aviso.

### Constantes

`gateTitleThreshold` (T_title) y `gateBodyThreshold` (T_body): se fijan con la
calibración (research.md R3). `gateMaxCandidates = 3`.

## Evidencia de anclas (US2)

### AnchorGrade (`domain/anchor_evidence.go`)

| Valor | Condición | ¿Se lista? |
|-------|-----------|------------|
| `AnchorLive` (vigente) | La ruta relativa existe y es un archivo. | No |
| `AnchorMoved` (movida) | Falta; su nombre base aparece en exactamente 1 ruta del índice. | Sí, con la ruta candidata |
| `AnchorMovedAmbiguous` (movida ambigua) | Falta; el nombre base aparece en ≥ 2 rutas del índice. | Sí, con el número de rutas y sin afirmar ninguna |
| `AnchorOrphanCandidate` (huérfana candidata) | Falta y no aparece en el índice. | Sí |
| `AnchorUnverifiable` (no verificable) | Ruta vacía, absoluta fuera de `root`, o ruta que existe pero no es un archivo. | No |

### AnchorEvidence

| Campo | Tipo | Regla |
|-------|------|-------|
| `Grade` | AnchorGrade | Ver la tabla anterior. |
| `Path` | string | Ruta normalizada: relativa con `/` si estaba dentro de `root`. |
| `Candidate` | string | Solo con `AnchorMoved`. |
| `Matches` | int | Número de rutas del índice con el mismo nombre base. |

Sin índice (nil, error o vacío), `Moved` y `MovedAmbiguous` son inalcanzables:
lo que falta en disco se clasifica como huérfana candidata, y la sección
declara "sin índice de código: solo se comprobó el disco".

## Masa (US3, US4)

### MassEdge / MassGraph (`domain/memory_mass.go`)

| Entidad | Campos | Reglas |
|---------|--------|--------|
| `MassEdge` | `From`, `To` int64; `Weight` float64 | Dirigida. Sin autolazos. `Weight > 0`. |
| `MassGraph` | `Nodes` []int64; `Edges` []MassEdge | `Nodes` ordenado por id y sin repetidos. Las aristas con extremos fuera de `Nodes` se descartan. Las aristas repetidas por par dirigido suman su peso. |

### Derivación desde relaciones (`BuildMassGraph`, FR-016)

| `domain.RelationType` | Aristas |
|-----------------------|---------|
| `related`, `compatible`, `scoped` | A→B y B→A, con peso = `Confidence`. |
| `supersedes` (A sustituye a B) | Solo B→A, con peso = `Confidence`. |
| `conflicts_with`, `not_conflict` | Ninguna. |
| Cualquiera con un extremo checkpoint o inexistente | Ninguna. |

`Confidence ≤ 0` → arista descartada.

### Semillas

`map[int64]float64` más una descripción legible para la leyenda:

| Contexto | Semillas | Peso |
|----------|----------|------|
| `Build()` y `mem mass` sin `--task` | Memorias de la sesión activa ∪ memorias ancladas a un hotspot | Uniforme entre ellas |
| Ninguna de las anteriores | Todos los nodos | Uniforme |
| `BuildContextPack` y `mem mass --task` | Resultados de la búsqueda (no checkpoint), por rango r desde 0 | `1/(r+1)` |

### MassEntry

| Campo | Tipo | Regla |
|-------|------|-------|
| `ID` | int64 | Nodo del grafo. |
| `Mass` | float64 | En [0,1]. Σ de todas = 1 ± 1e-9. |

El ranking se ordena por `Mass`↓ y después por `ID`↑. Para las mismas entradas,
el resultado es idéntico en cualquier orden de entrada.

## Puertos nuevos (`application/ports/context_builder.go`)

| Puerto | Método | Lo satisface hoy |
|--------|--------|------------------|
| `MemoryFullLister` | `ListAll(project string) ([]domain.Memory, error)` | `ports.MemoryRepository` (persistence) |
| `RelationFullLister` | `ListAll(project string) ([]domain.Relation, error)` | `ports.RelationRepository` (persistence) |
| `IndexedFilesQuerier` | `FileHashes(project string) (map[string]string, error)` | `ports.CodeGraphRepository` (persistence) |

Son puertos estrechos: no se amplía ninguna interfaz existente (memoria 167).
