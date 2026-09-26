package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// FR-006 / R6: `mem update` sustituye el global cuando se ejecuta desde una
// copia local y hay un global distinto; si no, el ejecutable en curso.
func TestResolveUpdateTarget(t *testing.T) {
	binDir := t.TempDir()
	global := filepath.Join(binDir, memBinaryName())
	writeExec(t, global)
	project := t.TempDir()
	local := filepath.Join(project, memBinaryName())
	writeExec(t, local)

	t.Run("desde el global", func(t *testing.T) {
		t.Setenv("PATH", binDir)
		target, fromLocal := resolveUpdateTarget(global, project)
		if !sameFile(target, global) || fromLocal {
			t.Fatalf("target=%q fromLocal=%v", target, fromLocal)
		}
	})
	t.Run("desde una copia local con global", func(t *testing.T) {
		t.Setenv("PATH", binDir)
		target, fromLocal := resolveUpdateTarget(local, project)
		if !sameFile(target, global) || !fromLocal {
			t.Fatalf("debe apuntar al global: target=%q fromLocal=%v", target, fromLocal)
		}
	})
	t.Run("desde una copia local sin global", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		target, fromLocal := resolveUpdateTarget(local, project)
		if target != local || fromLocal {
			t.Fatalf("sin global se actualiza el ejecutable en curso: target=%q fromLocal=%v", target, fromLocal)
		}
	})
}

// R6: antes de descargar se comprueba que el destino admite escritura.
func TestCheckReplaceable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, memBinaryName())
	writeExec(t, bin)
	if err := checkReplaceable(bin); err != nil {
		t.Fatalf("un directorio con escritura debe admitirse: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if err := checkReplaceable(bin); err == nil {
		t.Fatal("un directorio sin escritura debe rechazarse")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".mem-update-*")); len(left) != 0 {
		t.Errorf("la comprobación no debe dejar temporales: %v", left)
	}
}
