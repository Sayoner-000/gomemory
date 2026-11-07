package domain

import (
	"strings"
	"testing"
)

// HCD-051: con splash instalado en ~/.codex/agents, el aviso recomienda
// lanzarlo por su tipo (agent_type), que carga sus instrucciones de revisor y
// su sandbox de solo lectura, y sin heredar la conversación.
func TestSplashCodexForkWarning_RecomiendaAgentType(t *testing.T) {
	for _, want := range []string{`agent_type: "splash"`, `fork_turns: "none"`} {
		if !strings.Contains(SplashCodexForkWarning, want) {
			t.Errorf("el aviso no recomienda %s: %s", want, SplashCodexForkWarning)
		}
	}
	if !SplashSpawnInheritsContext("spawn_agent", map[string]any{"agent_type": "splash"}) {
		t.Error("un splash por agent_type sin fork_turns hereda la conversación")
	}
	if SplashSpawnInheritsContext("spawn_agent", map[string]any{"agent_type": "splash", "fork_turns": "none"}) {
		t.Error("con fork_turns none no hereda nada")
	}
}
