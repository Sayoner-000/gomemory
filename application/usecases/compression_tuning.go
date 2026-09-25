package usecases

import (
	"context"
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// RecordRetrievalAndTune suma una recuperación al compresor y tipo que
// produjeron el original y, si desde el último ajuste el agente recuperó más
// de threshold de lo omitido (con al menos domain.CompressionAdaptiveMinOmissions
// omisiones), baja un escalón la agresividad de ese tipo en el proyecto
// (FR-027, práctica `headroom learn`). Nunca la sube: eso lo hace la persona
// con `mem pack tune --reset`. Devuelve si hubo ajuste.
//
// threshold <= 0 usa domain.CompressionAdaptiveThreshold.
func RecordRetrievalAndTune(ctx context.Context, stats ports.CompressionStatsRepository, tuning ports.CompressionTuningRepository,
	project string, meta ports.OriginalMeta, threshold float64) (bool, error) {
	if stats == nil {
		return false, nil
	}
	if err := stats.RecordRetrieval(ctx, project, meta.Compressor, meta.ContentType); err != nil {
		return false, err
	}
	if tuning == nil || meta.ContentType == "" {
		return false, nil
	}
	if threshold <= 0 {
		threshold = domain.CompressionAdaptiveThreshold
	}
	omissions, retrievals, err := tuning.SinceLastAdjustment(ctx, project, meta.ContentType)
	if err != nil || omissions < domain.CompressionAdaptiveMinOmissions {
		return false, err
	}
	rate := float64(retrievals) / float64(omissions)
	if rate <= threshold {
		return false, nil
	}
	current, err := tuning.Get(ctx, project, meta.ContentType)
	if err != nil || current <= domain.AggressivenessOff {
		return false, err
	}
	reason := fmt.Sprintf("recuperación %.0f%% sobre %d omisiones", 100*rate, omissions)
	if err := tuning.Lower(ctx, project, meta.ContentType, reason); err != nil {
		return false, err
	}
	return true, nil
}
