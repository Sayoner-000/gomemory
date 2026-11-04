package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HCD-023: una segunda ejecución de setup-mcp sin cambios informa agy como ya
// registrado y no deja vacía la sección de habilidades.
func TestSetupMCP_InformaSinCambios(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PATH", t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, ".gemini", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func() string {
		return captureStdout(t, func() { runGlobalScopeSetup([]string{"codex", "agy"}) })
	}
	_ = run()
	out := run()
	if !strings.Contains(out, "✓ agy: ya registrado en ~/.gemini/config (sin cambios)") {
		t.Errorf("sin cambios, agy debe informarse como ya registrado:\n%s", out)
	}
	if !strings.Contains(out, "✓ todas al día (sin cambios)") {
		t.Errorf("sin cambios, la sección de habilidades no debe quedar vacía:\n%s", out)
	}
}
