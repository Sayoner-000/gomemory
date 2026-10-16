//go:build !windows

package persistence

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestPermisos_BaseYAjustesPrivados cierra H-05 (auditoría 2026-10-16): con
// umask 022, mem.db, su WAL/SHM y settings.json nacen solo para su dueño.
func TestPermisos_BaseYAjustesPrivados(t *testing.T) {
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	root := t.TempDir()
	path := filepath.Join(root, "mem.db")
	db, err := openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err = db.Exec(`INSERT INTO sessions(id, project) VALUES ('s','p')`); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s: permisos %o, se esperaba 600", filepath.Base(f), perm)
		}
	}
	if err := WriteSettings(root, Settings{}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(SettingsPath(root)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("settings.json: %v %v", info.Mode().Perm(), err)
	}
	// Un archivo previo con permisos amplios se endurece al reescribirlo.
	if err := os.Chmod(SettingsPath(root), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteSettings(root, Settings{}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(SettingsPath(root)); info.Mode().Perm() != 0o600 {
		t.Fatalf("settings.json previo: %o", info.Mode().Perm())
	}
}
