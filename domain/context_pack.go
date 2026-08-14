package domain

// Priority clasifica qué tan prescindible es un ContextItem al llenar un
// presupuesto de tokens. Critical nunca se acorta ni se descarta; Relevant
// puede acortarse; Optional puede acortarse o descartarse por completo
// (feature 015, spec.md FR-005).
type Priority int

const (
	PriorityCritical Priority = iota
	PriorityRelevant
	PriorityOptional
)

// ContextItem es una unidad de información dentro de un ContextPack: el
// contenido (posiblemente comprimido) de una memoria, artefacto de Spec Kit
// u otra fuente, junto con las señales que decidieron su inclusión.
type ContextItem struct {
	// ID identifica el origen exacto, p. ej. "memory:42" o
	// "speckit:015-context-optimization/requirements".
	ID         string
	Content    string
	Source     string
	Priority   Priority
	Relevance  float32
	Importance float32
	Confidence float32
	RawTokens  int
	Tokens     int
	Compressed bool
}

// ContextPack es el resultado de BuildContextPack: el conjunto de items que
// entra en el presupuesto solicitado, listo para entregarse a un agente.
// Nunca se persiste — se construye y se devuelve en la misma llamada.
type ContextPack struct {
	Items           []ContextItem
	Budget          int
	RawTokenCount   int
	TokenCount      int
	CompressionRate float64
	Stats           ContextStats
}

// SpecKitFeatureContext es la vista de solo lectura, acotada a UNA feature de
// Spec Kit, que alimenta a BuildContextPack cuando ContextRequest.
// IncludeSpecKit está activo (FR-015). Nunca mezcla contenido de otra
// feature — el llamador la construye a partir de .specify/feature.json.
type SpecKitFeatureContext struct {
	Feature          string
	Constraints      []string
	Requirements     []string
	Decisions        []string
	TaskDependencies []string
}

// ContextStats es el reporte de reducción adjunto a un ContextPack.
type ContextStats struct {
	RawTokens        int
	FinalTokens      int
	SavedTokens      int
	CompressionRatio float64
	ItemsRetrieved   int
	ItemsDuplicate   int
	ItemsCritical    int
	ItemsRelevant    int
	ItemsOptional    int
	ItemsDiscarded   int
}
