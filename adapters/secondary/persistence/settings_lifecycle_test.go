package persistence

import (
	"strings"
	"testing"

	"mem/application/ports"
)

// Feature 034: Write reconstruye el Settings completo desde SettingsData; los
// ajustes nuevos deben sobrevivir a guardar cualquier otra preferencia.
func TestSettingsRepository_ConservaAjustesDelCicloDeVida(t *testing.T) {
	root := t.TempDir()
	repo := NewSettingsRepository()
	if err := repo.Write(root, ports.SettingsData{
		UpdateCheckDisabled: true,
		Agents:              []string{"claude", "codex"},
		AgentScope:          "global",
	}); err != nil {
		t.Fatal(err)
	}
	s := repo.Read(root)
	s.AutoApprove = true // otra preferencia cualquiera
	if err := repo.Write(root, s); err != nil {
		t.Fatal(err)
	}
	got := repo.Read(root)
	if !got.UpdateCheckDisabled || strings.Join(got.Agents, ",") != "claude,codex" || got.AgentScope != "global" {
		t.Fatalf("se perdieron ajustes al reescribir: %+v", got)
	}
	if !ReadSettings(root).UpdateCheckDisabled {
		t.Error("ReadSettings debe ver update_check_disabled")
	}
}
