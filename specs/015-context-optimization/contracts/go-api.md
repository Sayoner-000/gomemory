# Contrato API Go: motor de Context Optimization

**Feature**: [../spec.md](../spec.md) · Modelo: [../data-model.md](../data-model.md)

Superficie pública que consumen `adapters/primary/cli/cmd_pack.go` y el registro de
tools MCP (`adapters/primary/cli/cmd_mcp*.go`). Nada aquí importa de `adapters/` —
cumple el Principio I (dominio/aplicación no dependen de infraestructura).

## Puertos nuevos (`application/ports/`)

```go
// compressor.go
type CompressionLevel int

const (
    // CompressionStructural es el valor cero: el zero value de un
    // ContextRequest sin Compression fijado debe comprimir (FR-009), no
    // dejar de comprimir. CompressionNone exige opt-out explícito.
    CompressionStructural CompressionLevel = iota
    CompressionNone
)

type CompressionOptions struct {
    Level          CompressionLevel
    PreserveCode   bool // default true, no configurable a false en v1
    PreserveURLs   bool // default true
    PreservePaths  bool // default true
    PreserveErrors bool // default true
}

type CompressionResult struct {
    Content    string
    RawTokens  int
    Tokens     int
    Compressed bool
}

type Compressor interface {
    Compress(input string, opts CompressionOptions) (CompressionResult, error)
}
```

```go
// token_counter.go
type TokenCounter interface {
    Count(text string) int
}
```

Ambos puertos son síncronos y puros desde la perspectiva del caso de uso (sin
`context.Context` de cancelación en v1 — ni el conteo por aproximación ni la
compresión estructural hacen I/O; se añade si un adaptador futuro con LLM lo necesita,
sin cambiar la firma del caso de uso que los consume).

## Caso de uso (`application/usecases/build_context_pack.go`)

```go
type ContextRequest struct {
    Task           string
    Project        string
    Namespace      string
    MaxTokens      int
    MinRelevance   float32
    MaxItems       int
    IncludeSpecKit bool
    IncludeMemory  bool
    Compression    ports.CompressionLevel
}

var (
    ErrCriticalContextOverflow = errors.New("critical context exceeds configured token budget")
    ErrInvalidContextRequest   = errors.New("invalid context request")
)

func BuildContextPack(
    memRepo ports.MemoryRepository,
    compressor ports.Compressor,
    counter ports.TokenCounter,
    specKit ports.SpecKitReader, // nil-able: si nil o IncludeSpecKit=false, se omite ese paso
    req ContextRequest,
) (domain.ContextPack, error)
```

**Contrato de comportamiento** (deriva 1:1 de los FR del spec):

- `req.Task == "" || req.Project == "" || req.MaxTokens <= 0` → devuelve
  `ErrInvalidContextRequest` sin tocar ningún puerto (FR-001, validación en el borde).
- Retrieval vía `memRepo.Search(req.Project, req.Task, cap)` (FR-002), `cap` derivado
  de `req.MaxItems` o un default de fábrica.
- Dedup vía `DetectDuplicateGroups` sobre los candidatos retrieved, no sobre todo el
  proyecto (FR-004, research.md §3).
- Clasificación de prioridad determinista por tipo (FR-005, data-model.md).
- Reserva de presupuesto: crítico primero; si `sum(RawTokens de Critical) > req.MaxTokens`
  → `ErrCriticalContextOverflow`, ningún `ContextPack` parcial (FR-008).
- Compresión de `Relevant`/`Optional` vía `compressor.Compress` (FR-009); si
  `compressor.Compress` devuelve error, se seguirov con el contenido original de ese
  item, nunca se aborta la construcción completa (FR-011).
- Relleno del presupuesto restante en orden `Relevant` → `Optional` por relevancia
  descendente hasta agotar `req.MaxTokens` (FR-007); lo que no entra se cuenta en
  `ContextStats.ItemsDiscarded`, nunca se descarta en silencio sin quedar reflejado.
- `req.IncludeSpecKit` y `specKit != nil` → agrega `SpecKitFeatureContext` de la
  feature activa (detectada vía `.specify/feature.json`, mismo mecanismo que ya usa el
  extension hook `speckit.gomemory-context.update`), acotado a esa feature (FR-015).

## Puerto opcional (`application/ports/speckit_reader.go`)

```go
type SpecKitReader interface {
    // ActiveFeature lee .specify/feature.json y devuelve el nombre de carpeta bajo
    // specs/, o "" si no existe (proyecto sin Spec Kit inicializado — no es error).
    ActiveFeature(root string) (string, error)
    // Read carga spec.md/plan.md/tasks.md/constitution.md de una feature y los
    // recorta a lo relevante para `task` (mismo texto de búsqueda que Search).
    Read(root, feature, task string) (domain.SpecKitFeatureContext, error)
}
```

Adaptador v1: `adapters/secondary/speckit/reader.go`, lectura directa de archivos
(`os.ReadFile`), sin caché — coherente con que `specs/*.md` cambia poco y este llamado
solo ocurre dentro de `BuildContextPack`, no en cada arranque.

## Herramientas de tool-optimización (`application/usecases/optimize_tool.go`)

```go
type ToolDescriptor struct {
    Name        string
    Description string
    Schema      json.RawMessage // opaco: nunca se parsea ni se muta
}

func OptimizeToolDescription(t ToolDescriptor, compressor ports.Compressor) (ToolDescriptor, error)
```

**Contrato**: `Name` y `Schema` se devuelven byte-por-byte idénticos al input siempre
(FR-017); solo `Description` puede cambiar, y solo pasa por `compressor.Compress` con
`CompressionOptions{Level: CompressionStructural}` — nunca se le aplica lógica
específica de MCP aquí, es el mismo `Compressor` que usa `BuildContextPack`.
