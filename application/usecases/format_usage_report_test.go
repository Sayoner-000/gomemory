package usecases_test

import (
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// El reporte dice qué mide: sin esa línea, su porcentaje se leía como el
// ahorro de toda la sesión.
func TestFormatUsageReport_DeclaraSuAlcance(t *testing.T) {
	for _, report := range []domain.UsageReport{{}, {Calls: 1, BaselineTokens: 100, EmittedTokens: 20}} {
		if out := usecases.FormatUsageReport(report, "session"); !strings.Contains(out, usecases.UsageReportScope) {
			t.Errorf("falta el alcance en:\n%s", out)
		}
	}
}
