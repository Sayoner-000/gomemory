package cli

import (
	"strings"
	"testing"
)

func TestRemoveMCPEntries_ConservaElRegistroGlobalDeCodex(t *testing.T) {
	salida := captureStdout(t, func() { removeMCPEntries(t.TempDir()) })
	if !strings.Contains(salida, "~/.codex/config.toml conserva [mcp_servers.gomemory]") {
		t.Errorf("el uninstall debe explicar que el registro Codex es compartido, salida:\n%s", salida)
	}
}
