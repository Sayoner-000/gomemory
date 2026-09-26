package usecases_test

import (
	"strings"
	"testing"

	"mem/application/ports"
	"mem/application/usecases"
)

// B-R01 (acr_e366819b) — una fila que solo acumula omisiones de bloques no
// aparenta un uso con 0 tokens: muestra sus tokens como no aplicables.
func TestSavingsReportOmissionOnlyRow(t *testing.T) {
	rep := usecases.SavingsReport{Rows: []ports.CompressorStats{
		{Compressor: "json", ContentType: "mixed", Uses: 1, RawTokens: 1000, FinalTokens: 300},
		{Compressor: "json", ContentType: "json", Omissions: 40, Retrievals: 10},
	}}
	out := rep.Format()
	if !strings.Contains(out, "1000 → 300") {
		t.Errorf("la fila con usos debe mostrar sus tokens:\n%s", out)
	}
	if strings.Contains(out, "0 → 0") {
		t.Errorf("la fila sin usos no debe mostrar 0 → 0:\n%s", out)
	}
	if !strings.Contains(out, "25.0%") {
		t.Errorf("la fila sin usos conserva su tasa de recuperación:\n%s", out)
	}
}
