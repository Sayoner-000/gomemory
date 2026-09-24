package setup

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
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

// TestInstallPlugin_OmiteArchivosDePrueba: el directorio embebido del plugin de
// OpenCode incluye su test (gomemory.test.mjs). Copiarlo a la carpeta de
// plugins de la persona usuaria instalaba un artefacto de desarrollo que
// OpenCode podría intentar cargar (feature 032, research R-9).
func TestInstallPlugin_OmiteArchivosDePrueba(t *testing.T) {
	fsys := fstest.MapFS{
		"plugin/opencode/gomemory.ts":       {Data: []byte("export const GomemoryPlugin = {};\n")},
		"plugin/opencode/gomemory.test.mjs": {Data: []byte("import { test } from 'node:test';\n")},
	}
	dst := t.TempDir()
	if _, err := InstallPlugin(fsys, "plugin/opencode", dst, nil); err != nil {
		t.Fatalf("InstallPlugin: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "gomemory.ts")); err != nil {
		t.Errorf("gomemory.ts debía instalarse: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "gomemory.test.mjs")); !os.IsNotExist(err) {
		t.Errorf("gomemory.test.mjs no debía instalarse; stat devolvió %v", err)
	}
}
