package ports

import (
	"context"
	"time"

	"mem/domain"
)

// CompressorStats es el acumulado de un compresor para un tipo de contenido en
// un proyecto (FR-026). Solo cifras: nunca contenido.
type CompressorStats struct {
	Compressor         string
	ContentType        string
	Uses               int
	RawTokens          int
	StructuralTokens   int
	FinalTokens        int
	Omissions          int
	Retrievals         int
	Fallbacks          int
	LatencyMicrosTotal int64
}

// CompressionStatsRepository acumula el uso del motor nativo por proyecto.
type CompressionStatsRepository interface {
	Record(ctx context.Context, project string, r CompressionResult) error
	RecordRetrieval(ctx context.Context, project, compressor, contentType string) error
	Summary(ctx context.Context, project string) ([]CompressorStats, error)
}

// TuningEntry es la agresividad vigente de un tipo de contenido y su motivo.
type TuningEntry struct {
	ContentType    string
	Aggressiveness domain.Aggressiveness
	Reason         string
	UpdatedAt      time.Time
}

// CompressionTuningRepository guarda el ajuste adaptativo (FR-027). Sin fila,
// la agresividad es domain.AggressivenessMax.
type CompressionTuningRepository interface {
	Get(ctx context.Context, project, contentType string) (domain.Aggressiveness, error)
	Lower(ctx context.Context, project, contentType, reason string) error
	// Reset vuelve a la agresividad máxima un tipo, o todos si contentType="".
	Reset(ctx context.Context, project, contentType string) error
	List(ctx context.Context, project string) ([]TuningEntry, error)
	// SinceLastAdjustment devuelve omisiones y recuperaciones de un tipo desde
	// su último ajuste: la tasa se mide desde ahí para no bajar dos veces la
	// agresividad por las mismas recuperaciones.
	SinceLastAdjustment(ctx context.Context, project, contentType string) (omissions, retrievals int, err error)
}
