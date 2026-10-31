package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanBinaries_SoloDentroDeHome(t *testing.T) {
	home, fuera := t.TempDir(), t.TempDir()
	dentro := filepath.Join(home, ".local", "bin", "mem")
	if err := os.MkdirAll(filepath.Dir(dentro), 0o755); err != nil {
		t.Fatal(err)
	}
	if !binaryInsideHome(home, dentro) {
		t.Fatal("un binario dentro de HOME debe retirarse")
	}
	if binaryInsideHome(home, filepath.Join(fuera, "mem")) {
		t.Fatal("un binario fuera de HOME debe conservarse")
	}
	if binaryInsideHome(home, home+"-otro/mem") {
		t.Fatal("un prefijo de ruta no es estar dentro")
	}
	enlace := filepath.Join(home, "enlace")
	if err := os.Symlink(fuera, enlace); err != nil {
		t.Skip("sin symlinks")
	}
	if binaryInsideHome(home, filepath.Join(enlace, "mem")) {
		t.Fatal("un enlace que sale de HOME no cuenta como dentro")
	}
}
