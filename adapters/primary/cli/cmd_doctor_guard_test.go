package cli

import (
	"strings"
	"testing"

	"mem/domain"
)

// T026 (feature 035, FR-003/FR-029): doctor advierte los hooks duplicados con
// su remedio y muestra los conteos de las protecciones de los últimos 7 días.
func TestPrintDoctorHookGuard(t *testing.T) {
	t.Run("duplicados", func(t *testing.T) {
		out := captureStdout(t, func() { printDoctorHookGuard([]string{"session-start", "user-prompt-submit"}, nil) })
		for _, want := range []string{"⚠", "hooks de gomemory duplicados", "session-start", "mem update"} {
			if !strings.Contains(out, want) {
				t.Errorf("falta %q en:\n%s", want, out)
			}
		}
	})
	t.Run("protecciones", func(t *testing.T) {
		counts := []domain.GuardCount{
			{Agent: "claude", Kind: domain.GuardBudgetTrimmed, Count: 3, LastDetail: "orig=17777,emit=9990"},
			{Agent: "claude", Kind: domain.GuardDuplicateDropped, Count: 2},
			{Agent: "opencode", Kind: domain.GuardToolOutputExcluded, Count: 5, LastDetail: "bash"},
		}
		out := captureStdout(t, func() { printDoctorHookGuard(nil, counts) })
		for _, want := range []string{"protecciones (7 días)", "budget_trimmed", "3", "duplicate_dropped", "2", "tool_output_excluded", "5", "⚠"} {
			if !strings.Contains(out, want) {
				t.Errorf("falta %q en:\n%s", want, out)
			}
		}
	})
	t.Run("solo exclusiones no es un problema", func(t *testing.T) {
		out := captureStdout(t, func() {
			printDoctorHookGuard(nil, []domain.GuardCount{{Agent: "claude", Kind: domain.GuardToolOutputExcluded, Count: 4}})
		})
		if strings.Contains(out, "⚠") {
			t.Errorf("las exclusiones son protección esperada, no un problema:\n%s", out)
		}
	})
	t.Run("sin nada que contar", func(t *testing.T) {
		if out := captureStdout(t, func() { printDoctorHookGuard(nil, nil) }); strings.TrimSpace(out) != "" {
			t.Errorf("sin duplicados ni eventos no se imprime la sección:\n%s", out)
		}
	})
}
