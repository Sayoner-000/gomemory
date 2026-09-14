package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// acr_5836d32d, C-003/C-004: en un checkout fresco el directorio del marcador
// puede no existir; sin crearlo, el marcador de primera sesión no se escribía y
// el bootstrap se repetía en cada prompt. Directorio y archivo quedan privados.
func TestWriteHookMarker_CreaDirectorioConPermisosPrivados(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".memory")
	marker := filepath.Join(dir, ".session-tools-injected")

	if err := writeHookMarker(marker); err != nil {
		t.Fatalf("writeHookMarker: %v", err)
	}
	fi, err := os.Stat(marker)
	if err != nil {
		t.Fatalf("el marcador no quedó escrito: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("permisos del marcador = %o, want 600", perm)
	}
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("permisos del directorio = %o, want 700", perm)
	}
}
