package domain

// --- Telemetría de enrutamiento (feature 027) ---
//
// Todo lo que se mide aquí son CIFRAS y ENUMS. Ninguna estructura de este
// archivo tiene un campo de texto libre alimentado por contenido: ni resumen, ni
// evidencia, ni razonamiento. Esa restricción es lo que convierte INV-AAR-013 en
// una propiedad verificable del esquema, en vez de una promesa de quien escribe.

// Quality es la valoración que el runtime da al resultado.
type Quality string

const (
	QualityAccepted Quality = "accepted"
	QualityPartial  Quality = "partial"
	QualityRejected Quality = "rejected"
)

// ExecutionReport es lo que el runtime informa tras ejecutar una unidad.
type ExecutionReport struct {
	TaskID        string
	Route         Route
	Status        ResultStatus
	ContextTokens int
	OutputTokens  int
	DurationMS    int
	Quality       Quality
}

// ExecutionRecord es una fila del historial: la decisión y, si llegó, su reporte.
type ExecutionRecord struct {
	TaskID        string
	PlanID        string
	Class         TaskClass
	Route         Route
	Reason        Reason
	ContextBudget int
	OutputBudget  int
	EstimatedCost int
	DecidedAt     string
	// Reported distingue "todavía sin reporte" de "reportó cero tokens".
	Reported      bool
	Status        ResultStatus
	ContextTokens int
	OutputTokens  int
	DurationMS    int
	Quality       Quality
	ReportedAt    string
}

// RoutingStats son los agregados de telemetría de un proyecto.
type RoutingStats struct {
	PorRuta          map[Route]int
	TokensEstimados  int
	TokensReales     int
	Exitos           int
	Fallos           int
	ContextoInsuf    int
	AnchoParaleloMax int
	// Decisiones es el total, con y sin reporte.
	Decisiones int
	// ConReporte es cuántas decisiones tienen consumo real medido. Sin él, el
	// ahorro solo puede presentarse como estimación (FR-033).
	ConReporte int
}

// AhorroEstimado es la diferencia entre lo que se estimó gastar y lo que
// realmente se gastó, cuando hay reportes suficientes para compararlo.
//
// Devuelve (0, false) si no hay ningún reporte: sin medición no hay ahorro que
// declarar, y presentar la estimación como resultado sería exactamente lo que
// prohíbe FR-033.
func (s RoutingStats) AhorroEstimado() (int, bool) {
	if s.ConReporte == 0 {
		return 0, false
	}
	return s.TokensEstimados - s.TokensReales, true
}

// RatioDeExito es la proporción de delegaciones terminadas correctamente.
// Devuelve (0, false) si no hay ninguna con reporte.
func (s RoutingStats) RatioDeExito() (float64, bool) {
	total := s.Exitos + s.Fallos + s.ContextoInsuf
	if total == 0 {
		return 0, false
	}
	return float64(s.Exitos) / float64(total), true
}
