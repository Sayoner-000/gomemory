package persistence

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestOpen_HardensFilePermissions cubre la Historia de Usuario 2
// (specs/009-mitigacion-riesgos): el directorio del proyecto y el archivo
// mem.db recién creados no deben ser legibles/escribibles por otros usuarios
// del sistema operativo. Solo aplica en plataformas con modelo de permisos
// Unix — en Windows los bits 0700/0600 no tienen el mismo significado y el
// hardening se omite sin romper nada (ver spec.md, Edge Cases).
func TestOpen_HardensFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modelo de permisos Unix no aplica en Windows")
	}

	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	dbPath := DbPath(root)
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat mem.db: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("esperaba permisos 0600 en mem.db, got %o", perm)
	}

	dirInfo, err := os.Stat(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("stat directorio del proyecto: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("esperaba permisos 0700 en el directorio del proyecto, got %o", perm)
	}
}
