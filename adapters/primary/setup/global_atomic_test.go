package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGlobalWrapper_Atomico(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modos POSIX")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "atomic-decomposition.md")
	if err := os.WriteFile(dest, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := writeGlobalWrapper(dest, "---\n", "nuevo"); err == nil {
		t.Fatal("debía fallar sin temporal")
	}
	if got, _ := os.ReadFile(dest); string(got) != "original" {
		t.Fatalf("original alterado: %q", got)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeGlobalWrapper(dest, "---\n", "nuevo"); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(dest); info.Mode().Perm() != 0o640 {
		t.Fatalf("modo = %o", info.Mode().Perm())
	}
}
