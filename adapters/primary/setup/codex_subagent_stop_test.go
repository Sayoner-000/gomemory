package setup

import "testing"

// TestCodexGomemoryHooks_SubagentStopAlFinalDeLaTabla cubre US3 (feature 030,
// capacidad C3): SubagentStop se añade AL FINAL de codexGomemoryHooks para no
// desplazar el índice [2] (Stop), que codex_gomemory_hooks_test.go usa
// directamente — así ningún test existente se toca.
//
// ⚠ Esta entrada NO se verificó en sesión interactiva (a diferencia del
// estándar que exige el comentario de codexGomemoryHooks): se basa en
// documentación oficial de Codex + inspección de `strings` sobre el binario
// instalado (0.154.0, que contiene "SubagentStop" y "last_assistant_message").
// Debe confirmarse en vivo antes de considerarla al mismo nivel que el resto
// de la tabla (quickstart.md Q0 de la feature 030).
func TestCodexGomemoryHooks_SubagentStopAlFinalDeLaTabla(t *testing.T) {
	hooks := CodexGomemoryHooks()
	if len(hooks) == 0 {
		t.Fatal("codexGomemoryHooks está vacía")
	}
	last := hooks[len(hooks)-1]
	if last.Event != "SubagentStop" || last.Sub != "subagent-stop" || last.Emit != "json" {
		t.Errorf("el último elemento debía ser SubagentStop→subagent-stop (Emit=json), got %+v", last)
	}
	if hooks[2].Event != "Stop" {
		t.Fatalf("el índice [2] debía seguir siendo Stop (codex_gomemory_hooks_test.go depende de esto), got %+v", hooks[2])
	}
}
