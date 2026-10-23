package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Con una copia en el directorio actual, el global sigue siendo el del PATH:
// en Windows exec.LookPath encontraba la copia y devolvía exec.ErrDot.
func TestLookPathGlobal_IgnoraElDirectorioActual(t *testing.T) {
	global, proyecto := t.TempDir(), t.TempDir()
	for _, dir := range []string{global, proyecto} {
		if err := os.WriteFile(filepath.Join(dir, memBinaryName()), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", global)
	t.Chdir(proyecto)

	got, err := lookPathGlobal()
	if err != nil {
		t.Fatalf("lookPathGlobal: %v", err)
	}
	if want := filepath.Join(global, memBinaryName()); !sameFilePath(got, want) {
		t.Errorf("lookPathGlobal = %q, want %q", got, want)
	}
}
