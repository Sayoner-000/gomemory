package setup

import (
	"embed"
	"io/fs"
	"testing"
)

//go:embed testdata
var testFS embed.FS

func TestInstallPluginIdempotent(t *testing.T) {
	root := t.TempDir()
	binPath := "/usr/local/bin/mem"
	port := 9735

	fsys, err := fs.Sub(testFS, "testdata")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}

	count, err := InstallPlugin(fsys, ".", root, &PluginContext{
		ProjectRoot: root,
		BinPath:     binPath,
		Port:        port,
	})
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	if count == 0 {
		t.Error("expected files to be copied on first install")
	}

	count2, err := InstallPlugin(fsys, ".", root, &PluginContext{
		ProjectRoot: root,
		BinPath:     binPath,
		Port:        port,
	})
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected idempotent install (0 files), got %d", count2)
	}
}
