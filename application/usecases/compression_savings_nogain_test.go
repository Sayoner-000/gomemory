package usecases_test

import (
	"strings"
	"testing"

	"mem/application/ports"
	"mem/application/usecases"
)

// T069 (feature 035, FR-024): el informe separa «sin ganancia» de «degradaciones».
func TestSavingsReport_SeparaSinGananciaDeDegradaciones(t *testing.T) {
	rep := usecases.SavingsReport{Rows: []ports.CompressorStats{
		{Compressor: "structural", ContentType: "unknown", Uses: 276, RawTokens: 469164, FinalTokens: 467793, Fallbacks: 3, NoGains: 273},
	}}
	out := rep.Format()
	if !strings.Contains(out, "sin ganancia") || !strings.Contains(out, "degradaciones") {
		t.Fatalf("faltan las columnas:\n%s", out)
	}
	if !strings.Contains(out, "273") || !strings.Contains(out, " 3 ") {
		t.Errorf("cada causa en su columna:\n%s", out)
	}
}
