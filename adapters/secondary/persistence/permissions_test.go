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
	defer func() { _ = db.Close() }()

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

// TestEnsureDir_HardensExistingMemDir: un .memory previo en 0755 (instalación
// anterior al hardening, o creado por un hook antes que Init) queda en 0700
// tras EnsureDir; MkdirAll por sí solo no lo corregía.
func TestEnsureDir_HardensExistingMemDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modelo de permisos Unix no aplica en Windows")
	}

	root := t.TempDir()
	memDir := filepath.Join(root, MemDir)
	if err := os.Mkdir(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(memDir, 0o755); err != nil { // anula la umask
		t.Fatalf("chmod: %v", err)
	}

	if err := EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	info, err := os.Stat(memDir)
	if err != nil {
		t.Fatalf("stat .memory: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("esperaba 0700 en .memory existente, got %o", perm)
	}
}
