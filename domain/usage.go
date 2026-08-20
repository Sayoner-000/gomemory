package domain

// Nombres de operación: identifican QUÉ hizo gomemory, no el nombre que le da
// un canal concreto (tool MCP, subcomando CLI, acción de la TUI). Un canal que
// exponga la misma operación bajo otro nombre traduce en su adaptador; el
// dominio no conoce el vocabulario de ningún canal (feature 020, FR-003).
const (
	OpBuildContext   = "build_context"
	OpSearchMemories = "search_memories"
	OpListMemories   = "list_memories"
	OpGetMemory      = "get_memory"
	OpBuildPack      = "build_pack"
	OpCompressPack   = "compress_pack"
	OpPlanContext    = "plan_context"
	OpSaveMemory     = "save_memory"
	// OpOther es el destino de las emisiones que el middleware de canal
	// registra como respaldo (FR-005) y no sabe clasificar en una operación
	// más específica. Es un valor legítimo, no un error.
	OpOther = "other"
)

// UsageRecord es una emisión de contexto medida: la unidad mínima que se
// persiste (feature 020). BaselineTokens es lo que la emisión habría costado
// sin optimizar; EmittedTokens es lo que efectivamente se emitió. Channel es
// una etiqueta descriptiva y abierta (mcp/cli/tui/…), nunca una lista cerrada
// de valores permitidos (FR-004).
type UsageRecord struct {
	ID             int64
	Project        string
	SessionID      string
	Operation      string
	Channel        string
	BaselineTokens int
	EmittedTokens  int
	CreatedAt      string
}

// Saved es el ahorro absoluto de este registro. Método, no columna: un valor
// derivado almacenado abriría la puerta a que la fila se contradiga a sí
// misma.
func (r UsageRecord) Saved() int {
	return r.BaselineTokens - r.EmittedTokens
}

// UsageBucket es el desglose agregado de un UsageReport por un eje (operación
// o canal).
type UsageBucket struct {
	Key            string
	Calls          int
	BaselineTokens int
	EmittedTokens  int
}

// UsageReport es la agregación de UsageRecord para una sesión o para todas las
// de un proyecto. Se calcula al vuelo y nunca se persiste, igual que
// ContextPack (feature 020, FR-036).
type UsageReport struct {
	Project        string
	SessionID      string
	Calls          int
	BaselineTokens int
	EmittedTokens  int
	// WindowTokens es la ventana de referencia provista por el usuario. 0 =
	// sin ventana: WindowRatio no es válido (FR-014, FR-015).
	WindowTokens int
	// SchemaTokens es el costo medido de los descriptores de operación que
	// gomemory publica. 0 si no se midió.
	SchemaTokens     int
	SchemaOperations int
	ByOperation      []UsageBucket
	ByChannel        []UsageBucket
}

// Saved es el ahorro absoluto del reporte.
func (r UsageReport) Saved() int {
	return r.BaselineTokens - r.EmittedTokens
}

// ReductionRatio es el ahorro como fracción de la línea base. 0 cuando
// BaselineTokens es 0: nunca división por cero, nunca NaN.
func (r UsageReport) ReductionRatio() float64 {
	if r.BaselineTokens == 0 {
		return 0
	}
	return float64(r.Saved()) / float64(r.BaselineTokens)
}

// WindowRatio es el ahorro como fracción de la ventana de referencia — el
// único dato ESTIMADO del modelo. ok es false cuando WindowTokens es 0 (sin
// ventana configurada): quien formatea debe omitir la línea entera en ese
// caso, no mostrar un cero (FR-015). El ratio puede superar 1.0 si el ahorro
// excede la ventana declarada; se devuelve tal cual, sin recortarlo.
func (r UsageReport) WindowRatio() (ratio float64, ok bool) {
	if r.WindowTokens <= 0 {
		return 0, false
	}
	return float64(r.Saved()) / float64(r.WindowTokens), true
}
