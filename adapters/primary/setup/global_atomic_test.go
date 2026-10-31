package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGlobalInstructions_Atomico(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modos POSIX")
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(cfg, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	compose := func(string) (string, bool) { return "nuevo", true }

	// Un directorio sin escritura impide el temporal: el original no se toca.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := writeGlobalInstructions(cfg, compose); err == nil {
		t.Fatal("debía fallar sin temporal")
	}
	if got, _ := os.ReadFile(cfg); string(got) != "original" {
		t.Fatalf("original alterado: %q", got)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Conserva el modo.
	if _, err := writeGlobalInstructions(cfg, compose); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(cfg); info.Mode().Perm() != 0o640 {
		t.Fatalf("modo = %o", info.Mode().Perm())
	}
}
