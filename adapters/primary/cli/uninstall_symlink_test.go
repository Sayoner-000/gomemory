package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mem/adapters/primary/setup"
)

// ADM 2026-10-23, H-01: con .claude/plugins enlazado, uninstall no borra el
// plugin del destino externo, y la limpieza de settings.json sigue.
func TestRemoveClaudePlugin_EnlaceNoSeSigue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	root, external := t.TempDir(), t.TempDir()
	plugin := filepath.Join(external, "gomemory", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(plugin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, ".claude", "plugins")); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, ".claude", "settings.json")
	if err := os.WriteFile(settings, []byte(`{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"mem hook turn-end"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	removeClaudePlugin(root)

	if _, err := os.Stat(plugin); err != nil {
		t.Errorf("el plugin del destino externo debe conservarse: %v", err)
	}
	if data, _ := os.ReadFile(settings); strings.Contains(string(data), "mem hook turn-end") {
		t.Errorf("la limpieza de settings.json debe seguir aunque el plugin se omita:\n%s", data)
	}
}

// Los envoltorios de un directorio enlazado no se borran.
func TestRemoveNativeWrappers_EnlaceNoSeSigue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear symlinks en Windows exige privilegios")
	}
	paths := setup.GeneratedWrapperPaths()
	if len(paths) == 0 {
		t.Skip("sin envoltorios generados")
	}
	seg := paths[0]
	root, external := t.TempDir(), t.TempDir()
	// Enlaza el primer tramo (.claude o .opencode) a un directorio externo que
	// contiene el envoltorio.
	ajeno := filepath.Join(append([]string{external}, seg[1:]...)...)
	if err := os.MkdirAll(filepath.Dir(ajeno), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ajeno, []byte("ajeno"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, seg[0])); err != nil {
		t.Fatal(err)
	}

	removeNativeWrappers(root)

	if _, err := os.Stat(ajeno); err != nil {
		t.Errorf("el envoltorio del destino externo debe conservarse: %v", err)
	}
}
