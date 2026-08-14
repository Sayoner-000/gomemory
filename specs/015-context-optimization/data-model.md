# Fase 1 — Modelo de Datos: Context Optimization & Budgeting

**Feature**: [spec.md](./spec.md) · **Investigación**: [research.md](./research.md)

Todo lo de este documento es dominio puro nuevo (`domain/context_pack.go`): sin I/O,
sin imports de infraestructura, coherente con el Principio I de la Constitución.
`ContextPack` nunca se persiste (ver research.md §6) — vive solo en memoria durante una
llamada.

## Priority

```go
type Priority int

const (
    PriorityCritical Priority = iota
    PriorityRelevant
    PriorityOptional
)
```

- **Critical**: debe preservarse exacto — nunca se acorta, nunca se descarta.
- **Relevant**: puede acortarse (compresión estructural), no se descarta salvo que
  el presupuesto se agote después de servir todo lo crítico.
- **Optional**: puede acortarse o descartarse por completo si el presupuesto no
  alcanza.

**Regla de clasificación (FR-005)**: un `ContextItem` es `Critical` si su
`domain.MemoryType` de origen es `Decision`, `Architecture` o `Bugfix`, o si el
`ContextRequest` lo marca explícitamente como obligatorio (p. ej. un requisito de Spec
Kit). Es `Relevant` si el tipo es `Pattern`, `Discovery` o `Learning`. Es `Optional` si
el tipo es `Preference` o si su score de relevancia (ver research.md §2) cae por debajo
de `MinRelevance`. `Checkpoint` nunca es candidato (excluido en retrieval, mismo
criterio que `DetectDuplicateGroups`).

## ContextRequest

| Campo | Tipo | Obligatorio | Notas |
|---|---|---|---|
| `Task` | `string` | sí | Descripción de la tarea; alimenta `MemoryRepository.Search` |
| `Project` | `string` | sí | Scope de retrieval — nunca cruza proyectos |
| `Namespace` | `string` | no | Sub-scope opcional dentro del proyecto (FR-016) |
| `MaxTokens` | `int` | sí | Presupuesto total; debe ser > 0 |
| `MinRelevance` | `float32` | no | Default desde `SettingsData`; descarta candidatos por debajo |
| `MaxItems` | `int` | no | Tope de candidatos a considerar antes de rankear |
| `IncludeSpecKit` | `bool` | no | Default `true` salvo `SpeckitContextDisabled` (settings existentes) |
| `IncludeMemory` | `bool` | no | Default `true` |
| `Compression` | `CompressionLevel` | no | `None` \| `Structural` (default) — `Semantic`/`Aggressive` reservados a adaptadores futuros |

**Validación**: `Task` y `Project` no vacíos; `MaxTokens > 0` (si no, error de
validación en el borde, Principio Operativo #5 "fallar rápido" — nunca se llega a
construir un `ContextPack` con presupuesto inválido).

## ContextItem

| Campo | Tipo | Notas |
|---|---|---|
| `ID` | `string` | `"memory:<id>"` o `"speckit:<feature>/<sección>"` — identifica el origen exacto |
| `Content` | `string` | Contenido final (posiblemente comprimido) que entra al paquete |
| `Source` | `string` | Ruta o referencia legible (p. ej. `filepath` de la memoria, o ruta del artefacto Spec Kit) |
| `Priority` | `Priority` | Ver arriba |
| `Relevance` | `float32` | 0–1, derivado del orden de `Search` (research.md §2) |
| `Importance` | `float32` | 0–1, derivado de `MemoryType` |
| `Confidence` | `float32` | 1.0 por defecto (no hay señal de incertidumbre almacenada hoy) |
| `RawTokens` | `int` | Tokens del contenido original, antes de comprimir |
| `Tokens` | `int` | Tokens del contenido final, después de comprimir (== `RawTokens` si no se comprimió) |
| `Compressed` | `bool` | `true` si `Tokens < RawTokens` |

**Invariante**: `Tokens <= RawTokens` siempre (la compresión nunca puede alargar
contenido — si un compresor lo hiciera, el caso de uso lo trata como fallo y usa el
original, FR-011).

## ContextPack

| Campo | Tipo | Notas |
|---|---|---|
| `Items` | `[]ContextItem` | Orden: `Critical` → `Relevant` → `Optional`, por relevancia descendente dentro de cada nivel |
| `Budget` | `int` | Eco de `ContextRequest.MaxTokens` |
| `RawTokenCount` | `int` | Suma de `RawTokens` de todos los candidatos retenidos tras dedup (antes de comprimir/recortar por budget) |
| `TokenCount` | `int` | Suma de `Tokens` de los items finalmente incluidos — **nunca > `Budget`** |
| `CompressionRate` | `float64` | `1 - TokenCount/RawTokenCount` (0 si `RawTokenCount == 0`) |
| `Stats` | `ContextStats` | Ver abajo |

**Invariante dura (FR-007/FR-008)**: `TokenCount <= Budget`. Si los items `Critical`
por sí solos no caben en `Budget`, `BuildContextPack` no devuelve un `ContextPack`
parcial — devuelve `ErrCriticalContextOverflow` y ningún paquete.

## ContextStats

| Campo | Tipo | Notas |
|---|---|---|
| `RawTokens` | `int` | = `ContextPack.RawTokenCount` |
| `FinalTokens` | `int` | = `ContextPack.TokenCount` |
| `SavedTokens` | `int` | `RawTokens - FinalTokens` |
| `CompressionRatio` | `float64` | = `ContextPack.CompressionRate` |
| `ItemsRetrieved` | `int` | Candidatos antes de dedup/budget |
| `ItemsDuplicate` | `int` | Descartados por `DetectDuplicateGroups` |
| `ItemsCritical` | `int` | Incluidos con `Priority == Critical` |
| `ItemsRelevant` | `int` | Incluidos con `Priority == Relevant` |
| `ItemsOptional` | `int` | Incluidos con `Priority == Optional` |
| `ItemsDiscarded` | `int` | Candidatos que no entraron por presupuesto (solo pueden ser `Relevant`/`Optional`, nunca `Critical`) |

**Invariante**: `ItemsRetrieved == ItemsDuplicate + ItemsCritical + ItemsRelevant + ItemsOptional + ItemsDiscarded`.

## SpecKitFeatureContext (solo si `IncludeSpecKit`)

| Campo | Tipo | Notas |
|---|---|---|
| `Feature` | `string` | Nombre de carpeta bajo `specs/`, p. ej. `015-context-optimization` |
| `Constraints` | `[]string` | Extraído de `.specify/memory/constitution.md`, filtrado por relevancia a `Task` |
| `Requirements` | `[]string` | Extraído de `spec.md` (Functional Requirements) de la feature activa |
| `Decisions` | `[]string` | Extraído de `plan.md`/`research.md` de la feature activa (secciones "Decisión") |
| `TaskDependencies` | `[]string` | Extraído de `tasks.md` de la feature activa, si existe |

No es un tipo persistido: es una vista de solo lectura sobre archivos ya en disco
(`specs/<feature>/*.md`), acotada a la feature de la tarea actual (FR-015) — nunca
mezcla artefactos de otras features del mismo proyecto.

## Relaciones

```
ContextRequest ──(1 llamada)──> BuildContextPack (caso de uso)
                                       │
                                       ├─ retrieve  → []domain.Memory   (MemoryRepository.Search)
                                       ├─ dedup     → DetectDuplicateGroups (reuso, ver research.md §3)
                                       ├─ classify  → Priority por item
                                       ├─ compress  → Compressor (puerto)
                                       ├─ count     → TokenCounter (puerto)
                                       └─ budget    → asigna Critical → Relevant → Optional
                                                          │
                                                          ▼
                                                    ContextPack { Items, Stats }
```

## Errores de dominio

- `ErrCriticalContextOverflow`: el conjunto de items `Critical` excede `MaxTokens`.
  Ningún `ContextPack` se devuelve; el llamador (CLI/API/MCP) debe mostrar el error tal
  cual, nunca degradarlo a un paquete parcial silencioso (FR-008, edge case en spec.md).
- `ErrInvalidContextRequest`: `Task`/`Project` vacíos o `MaxTokens <= 0` — validado en
  el borde antes de tocar cualquier puerto (Principio Operativo #5).
