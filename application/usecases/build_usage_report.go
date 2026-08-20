package usecases

import (
	"sort"

	"mem/application/ports"
	"mem/domain"
)

// BuildUsageReport agrega los registros de uso de una sesión (o de todas las
// del proyecto, si sessionID es "") en un domain.UsageReport (feature 020).
// windowTokens es la ventana de referencia que provee el usuario; 0 = sin
// ventana (FR-014) — se propaga sin normalizar, WindowRatio() decide si es
// válida. Sin ports.TokenCounter: los registros ya vienen en tokens (quien
// emitió hizo la conversión una sola vez); agregar no requiere volver a
// contar nada.
func BuildUsageReport(repo ports.UsageRepository, project, sessionID string, windowTokens int) (domain.UsageReport, error) {
	var recs []domain.UsageRecord
	var err error
	if sessionID == "" {
		recs, err = repo.Totals(project)
	} else {
		recs, err = repo.BySession(project, sessionID)
	}
	if err != nil {
		return domain.UsageReport{}, err
	}

	report := domain.UsageReport{
		Project:      project,
		SessionID:    sessionID,
		WindowTokens: windowTokens,
	}

	byOp := map[string]*domain.UsageBucket{}
	byCh := map[string]*domain.UsageBucket{}
	for _, r := range recs {
		report.Calls++
		report.BaselineTokens += r.BaselineTokens
		report.EmittedTokens += r.EmittedTokens

		addToBucket(byOp, r.Operation, r)
		addToBucket(byCh, r.Channel, r)
	}

	report.ByOperation = sortedBuckets(byOp)
	report.ByChannel = sortedBuckets(byCh)

	return report, nil
}

func addToBucket(m map[string]*domain.UsageBucket, key string, r domain.UsageRecord) {
	b, ok := m[key]
	if !ok {
		b = &domain.UsageBucket{Key: key}
		m[key] = b
	}
	b.Calls++
	b.BaselineTokens += r.BaselineTokens
	b.EmittedTokens += r.EmittedTokens
}

// sortedBuckets devuelve los buckets ordenados descendente por
// BaselineTokens (G6 de contracts/usage-report.md), en orden estable para que
// el JSON producido sea determinista entre llamadas con los mismos datos.
func sortedBuckets(m map[string]*domain.UsageBucket) []domain.UsageBucket {
	out := make([]domain.UsageBucket, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].BaselineTokens != out[j].BaselineTokens {
			return out[i].BaselineTokens > out[j].BaselineTokens
		}
		return out[i].Key < out[j].Key
	})
	return out
}
