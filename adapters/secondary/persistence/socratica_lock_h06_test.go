//go:build !windows

package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSocraticaLock_NoSigueEnlaces cierra el subpunto TOCTOU de H-06: aunque
// el enlace aparezca después de cualquier comprobación previa, la apertura no
// lo sigue.
func TestSocraticaLock_NoSigueEnlaces(t *testing.T) {
	root := t.TempDir()
	lock := NewSocraticaDispatchLock(root)
	path, _ := lock.path("flow")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "victima")
	if err := os.WriteFile(target, []byte("intacto"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := lock.TryAcquire("flow"); err == nil || ok {
		t.Fatal("siguió un enlace simbólico", ok, err)
	}
	if f, err := openSocraticaLockFile(path); err == nil {
		_ = f.Close()
		t.Fatal("la apertura de bajo nivel sigue el enlace")
	}
}

// TestSocraticaLock_RemoveNoBorraUnLockRetenido: purgar no puede quitar el lock
// que otro proceso retiene (H-06).
func TestSocraticaLock_RemoveNoBorraUnLockRetenido(t *testing.T) {
	lock := NewSocraticaDispatchLock(t.TempDir())
	release, ok, err := lock.TryAcquire("flow")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := lock.Remove("flow"); err != nil {
		t.Fatal(err)
	}
	path, _ := lock.path("flow")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("borró un lock retenido", err)
	}
	release()
	if err := lock.Remove("flow"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("no borró un lock libre", err)
	}
}
