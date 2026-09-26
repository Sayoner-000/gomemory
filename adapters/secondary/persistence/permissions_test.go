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

// TestEnsureDir_HardensExistingGlobalProjectDir: un directorio de proyecto
// del store global creado antes del hardening (0755) queda en 0700 al
// abrirse (C-001 de acr_ad72cce1).
func TestEnsureDir_HardensExistingGlobalProjectDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modelo de permisos Unix no aplica en Windows")
	}

	root := t.TempDir()
	globalDir, err := GlobalProjectDir(ProjectKey(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	info, err := os.Stat(globalDir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("esperaba 0700 en el directorio global existente, got %o", perm)
	}
}

// TestHardenGlobalStore_FixesDormantProjects: los proyectos que no se vuelven
// a abrir también quedan en 0700/0600, incluidos -wal y -shm; los archivos
// ajenos a mem.db no se tocan (C-001 de acr_ad72cce1).
func TestHardenGlobalStore_FixesDormantProjects(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modelo de permisos Unix no aplica en Windows")
	}
	t.Setenv(dataHomeEnvOverride, t.TempDir())

	dir, err := GlobalProjectDir("dormido-0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbFiles := []string{DbName, DbName + "-wal", DbName + "-shm"}
	for _, name := range append(dbFiles, "otro.txt") {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	n, err := HardenGlobalStore()
	if err != nil {
		t.Fatalf("HardenGlobalStore: %v", err)
	}
	if n != 4 {
		t.Errorf("esperaba 4 entradas corregidas (dir + 3 mem.db*), got %d", n)
	}
	assertPerm := func(p string, want os.FileMode) {
		t.Helper()
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: esperaba %o, got %o", filepath.Base(p), want, got)
		}
	}
	assertPerm(dir, 0o700)
	for _, name := range dbFiles {
		assertPerm(filepath.Join(dir, name), 0o600)
	}
	assertPerm(filepath.Join(dir, "otro.txt"), 0o644)

	if n, err := HardenGlobalStore(); err != nil || n != 0 {
		t.Errorf("segunda pasada debe ser idempotente: n=%d err=%v", n, err)
	}
}

// TestHardenGlobalStore_NoStoreIsNotAnError: una máquina sin store global
// todavía (instalación nueva) no produce error.
func TestHardenGlobalStore_NoStoreIsNotAnError(t *testing.T) {
	t.Setenv(dataHomeEnvOverride, filepath.Join(t.TempDir(), "no-existe"))
	if n, err := HardenGlobalStore(); err != nil || n != 0 {
		t.Fatalf("esperaba (0, nil), got (%d, %v)", n, err)
	}
}
