package setup

import "testing"

// TestHookCommandIsGomemoryRecognizesPlanTriggerSubcommands cubre los dos
// subcomandos nuevos de la feature 019 (plan-guard, plan-entered). Sin este
// reconocimiento, cada reinstalación duplicaría la entrada de hook: el mismo
// tropiezo que la feature 007 tuvo que corregir con plan-approved (T004 de
// specs/007-memoria-sinaptica-neurocognitiva/tasks.md).
func TestHookCommandIsGomemoryRecognizesPlanTriggerSubcommands(t *testing.T) {
	for _, sub := range []string{"plan-guard", "plan-entered"} {
		cmd := "mem hook " + sub
		if !hookCommandIsGomemory(cmd) {
			t.Errorf("hookCommandIsGomemory no reconoce %q", cmd)
		}
	}
}
