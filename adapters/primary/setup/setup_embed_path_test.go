package setup

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// Un fs.FS solo acepta rutas con "/": con filepath.Join, en Windows ningún
// archivo de un subdirectorio embebido se encontraba.
func TestInstallPlugin_SubdirectorioEmbebido(t *testing.T) {
	fsys := fstest.MapFS{
		"plugin/a.txt":     {Data: []byte("raíz")},
		"plugin/sub/b.txt": {Data: []byte("anidado")},
	}
	dst := t.TempDir()
	n, err := InstallPlugin(fsys, "plugin", dst, nil)
	if err != nil {
		t.Fatalf("InstallPlugin: %v", err)
	}
	if n != 2 {
		t.Errorf("archivos copiados = %d, want 2", n)
	}
	data, err := os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil || string(data) != "anidado" {
		t.Errorf("sub/b.txt = %q, %v", data, err)
	}
}
