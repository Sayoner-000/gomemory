package cli

import (
	"strings"
	"testing"
	"time"

	"mem/adapters/primary/console"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// TestFormatTokensLine fija la línea de resumen de `mem pack compress`.
func TestFormatTokensLine(t *testing.T) {
	cases := []struct {
		in   ports.CompressionResult
		want string
	}{
		{ports.CompressionResult{RawTokens: 1000, Tokens: 250, Compressor: "json"}, "tokens: 1000 → 250 (json, -75.0%)"},
		{ports.CompressionResult{RawTokens: 0, Tokens: 0}, "tokens: 0 → 0 (structural, -0.0%)"},
		{ports.CompressionResult{RawTokens: 400, Tokens: 400, Compressor: "none", FallbackReason: "private"}, "tokens: 400 → 400 (none, -0.0%) · fallback private"},
	}
	for _, c := range cases {
		if got := FormatTokensLine(c.in); got != c.want {
			t.Errorf("FormatTokensLine(%+v) = %q, quiero %q", c.in, got, c.want)
		}
	}
}

// TestRenderSavingsHuman: sin registros explica cómo activarlos; con
// registros muestra por compresor ahorro, recuperación y latencia, el total y
// los ajustes adaptativos, con las mismas cifras que el informe.
func TestRenderSavingsHuman(t *testing.T) {
	l := console.NewLayout(console.Env{Width: 140, Getenv: func(string) string { return "" }})

	empty := renderSavingsHuman(usecases.SavingsReport{OriginalsMax: 64 << 20, TTLDays: 7}, l)
	for _, want := range []string{"Sin compresiones registradas", "--compression-level=max", "caducan a los 7 días"} {
		if !strings.Contains(empty, want) {
			t.Fatalf("informe vacío sin %q:\n%s", want, empty)
		}
	}

	rep := usecases.SavingsReport{
		Rows: []ports.CompressorStats{
			{Compressor: "listing", Uses: 4, RawTokens: 10000, FinalTokens: 2500, Omissions: 4, Retrievals: 1, LatencyMicrosTotal: 8000},
			{Compressor: "json", Uses: 0},
		},
		Tuning:         []ports.TuningEntry{{ContentType: "log", Aggressiveness: domain.Aggressiveness(2), Reason: "recuperaciones frecuentes", UpdatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}},
		OriginalsBytes: 2 << 20, OriginalsCount: 3, OriginalsMax: 64 << 20, TTLDays: 7,
	}
	got := renderSavingsHuman(rep, l)
	for _, want := range []string{"listing", "75.0%", "25.0%", "2.0 ms", "Ahorro total", "7 500", "Ajustes adaptativos", "agresividad 2", "2026-10-01", "3 refs"} {
		if !strings.Contains(got, want) {
			t.Fatalf("informe sin %q:\n%s", want, got)
		}
	}
}
