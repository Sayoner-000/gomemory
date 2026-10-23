package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// H-01 (ADM 2026-10-22, 2.ª ronda): setup-mcp de proyecto no escribe a través
// de un .mcp.json enlazado; el destino externo queda intacto.
func TestSetupMCP_EnlaceNoSeSigue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	for name, run := range map[string]struct {
		rel   []string
		setup func(string) bool
	}{
		"claude": {[]string{".mcp.json"}, setupClaude},
		"cursor": {[]string{".cursor", "mcp.json"}, setupCursor},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			external := filepath.Join(t.TempDir(), "externo.json")
			before := []byte(`{"mcpServers":{"otro":{"command":"otro"}}}`)
			if err := os.WriteFile(external, before, 0o644); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(append([]string{root}, run.rel...)...)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(external, link); err != nil {
				t.Fatal(err)
			}
			if run.setup(root) {
				t.Error("debe rechazar el enlace")
			}
			if got, _ := os.ReadFile(external); string(got) != string(before) {
				t.Errorf("el destino externo cambió:\n%s", got)
			}
		})
	}
}
