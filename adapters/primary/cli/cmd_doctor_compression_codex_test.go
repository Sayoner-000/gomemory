package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T077 (feature 035, FR-025): doctor dice con honestidad dónde comprime Codex.
func TestToolOutputHookStates_CodexSoloEnOrigen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got := toolOutputHookStates(t.TempDir(), true)["codex"]
	if !strings.Contains(got, "solo en origen (Codex no permite reescribir salidas)") {
		t.Errorf("estado de Codex: %q", got)
	}
	_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("command = 'mem hook tool-output codex'\n"), 0o600)
	if got := toolOutputHookStates(t.TempDir(), true)["codex"]; !strings.Contains(got, "hook antiguo registrado") {
		t.Errorf("debe conservar el aviso del hook antiguo: %q", got)
	}
}
