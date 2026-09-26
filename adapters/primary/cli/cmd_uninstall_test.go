package cli

import (
	"strings"
	"testing"
)

const codexSharedNote = "~/.codex/config.toml conserva [mcp_servers.gomemory]"

func TestRemoveProjectIntegration_ConservaElRegistroGlobalDeCodex(t *testing.T) {
	salida := captureStdout(t, func() { _ = removeProjectIntegration(t.TempDir(), true) })
	if !strings.Contains(salida, codexSharedNote) {
		t.Errorf("el uninstall de proyecto debe explicar que el registro Codex es compartido, salida:\n%s", salida)
	}
}

// En el alcance de sistema el registro global de Codex se retira después, así
// que anunciar que se conserva contradice el propio resumen.
func TestRemoveProjectIntegration_SistemaNoAnunciaQueConservaCodex(t *testing.T) {
	salida := captureStdout(t, func() { _ = removeProjectIntegration(t.TempDir(), false) })
	if strings.Contains(salida, codexSharedNote) {
		t.Errorf("uninstall --all no debe decir que conserva el registro Codex, salida:\n%s", salida)
	}
}
