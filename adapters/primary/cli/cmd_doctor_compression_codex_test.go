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

// T007 (feature 036, US2, FR-004): el estado de OpenCode describe emisión
// best-effort, no "no soportado" ni "sin verificar".
func TestToolOutputHookStates_OpencodeBestEffort(t *testing.T) {
	got := toolOutputHookStates(t.TempDir(), true)["opencode"]
	if !strings.Contains(got, "best-effort") {
		t.Errorf("estado de OpenCode debe ser best-effort: %q", got)
	}
	for _, prohibido := range []string{"no soportado", "sin verificar"} {
		if strings.Contains(got, prohibido) {
			t.Errorf("estado de OpenCode no debe decir %q: %q", prohibido, got)
		}
	}
}
