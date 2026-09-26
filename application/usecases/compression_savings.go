package usecases

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"mem/application/ports"
	"mem/domain"
)

// SavingsReport es el informe de `mem pack savings` / pack_savings (FR-026).
type SavingsReport struct {
	Rows           []ports.CompressorStats `json:"compressors"`
	Tuning         []ports.TuningEntry     `json:"tuning"`
	OriginalsBytes int64                   `json:"originals_bytes"`
	OriginalsCount int                     `json:"originals_count"`
	OriginalsMax   int64                   `json:"originals_max_bytes"`
	TTLDays        int                     `json:"originals_ttl_days"`
}

// BuildSavingsReport reúne estadísticas, ajustes y uso del almacén. Cualquier
// dependencia puede ser nil: esa parte del informe sale vacía.
func BuildSavingsReport(ctx context.Context, stats ports.CompressionStatsRepository, tuning ports.CompressionTuningRepository,
	store ports.OriginalStoreRepository, project string, maxBytes int64, ttlDays int) (SavingsReport, error) {
	rep := SavingsReport{OriginalsMax: maxBytes, TTLDays: ttlDays}
	if rep.OriginalsMax <= 0 {
		rep.OriginalsMax = domain.CompressionOriginalsMaxBytes
	}
	if rep.TTLDays <= 0 {
		rep.TTLDays = domain.CompressionOriginalsTTLDays
	}
	var err error
	if stats != nil {
		if rep.Rows, err = stats.Summary(ctx, project); err != nil {
			return rep, err
		}
	}
	if tuning != nil {
		if rep.Tuning, err = tuning.List(ctx, project); err != nil {
			return rep, err
		}
	}
	if store != nil {
		if rep.OriginalsBytes, rep.OriginalsCount, err = store.Usage(ctx); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// Format devuelve el informe en texto (contracts/cli-and-mcp.md).
func (r SavingsReport) Format() string {
	var b strings.Builder
	if len(r.Rows) == 0 {
		b.WriteString("Sin compresiones registradas todavía. Activa el nivel max con `mem settings --compression-level=max`.\n")
	} else {
		w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "compresor\ttipo\tusos\ttokens antes → después\tahorro\trecuperación\tdegradaciones\tlatencia media")
		for _, s := range r.Rows {
			// Una fila sin usos solo acumula omisiones o recuperaciones de los
			// bloques de un documento mixto: no tiene tokens propios que mostrar.
			tokens, ahorro, recup, lat := "—", "—", "—", "—"
			if s.Uses > 0 {
				tokens = fmt.Sprintf("%d → %d", s.RawTokens, s.FinalTokens)
			}
			if s.RawTokens > 0 {
				ahorro = fmt.Sprintf("%.1f%%", 100*(1-float64(s.FinalTokens)/float64(s.RawTokens)))
			}
			if s.Omissions > 0 {
				recup = fmt.Sprintf("%.1f%%", 100*float64(s.Retrievals)/float64(s.Omissions))
			}
			if s.Uses > 0 {
				lat = fmt.Sprintf("%.1f ms", float64(s.LatencyMicrosTotal)/float64(s.Uses)/1000)
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%d\t%s\n", s.Compressor, s.ContentType, s.Uses, tokens, ahorro, recup, s.Fallbacks, lat)
		}
		_ = w.Flush()
	}
	if len(r.Tuning) > 0 {
		b.WriteString("\nAjustes adaptativos:\n")
		for _, e := range r.Tuning {
			fmt.Fprintf(&b, "- %s: agresividad %d (%s, %s)\n", e.ContentType, e.Aggressiveness, e.Reason, e.UpdatedAt.Format("2006-01-02"))
		}
	}
	fmt.Fprintf(&b, "\nOriginales: %.1f MB de %.0f MB · %d refs · caducan a los %d días sin uso\n",
		float64(r.OriginalsBytes)/(1<<20), float64(r.OriginalsMax)/(1<<20), r.OriginalsCount, r.TTLDays)
	return b.String()
}
