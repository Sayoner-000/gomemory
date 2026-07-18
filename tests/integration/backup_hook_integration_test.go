package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// TestHookSessionEndCreatesBackupSnapshot cubre la Historia de Usuario 1
// (specs/009-mitigacion-riesgos): cerrar sesión debe generar automáticamente
// un snapshot exportable, sin que el usuario invoque `mem export` a mano.
func TestHookSessionEndCreatesBackupSnapshot(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	project := persistence.ProjectKey(target)
	if _, err := persistence.InsertMemory(db, &domain.Memory{
		Project: project, Type: domain.Decision, Title: "backup e2e", Content: "contenido de prueba",
	}); err != nil {
		db.Close()
		t.Fatalf("insert memory: %v", err)
	}
	db.Close()

	runHook(t, bin, target, "session-start")
	runHook(t, bin, target, "session-end")

	backupDir, err := persistence.BackupDir(project)
	if err != nil {
		t.Fatalf("backup dir: %v", err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("leer directorio de backups (esperaba que existiera): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("esperaba 1 snapshot tras cerrar sesión, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if !bytes.Contains(data, []byte("backup e2e")) {
		t.Fatalf("snapshot no contiene la memoria esperada: %s", data)
	}
}

// TestHookSessionEndWithoutActiveSessionSkipsBackup confirma que sin sesión
// activa (session-end sin session-start previo) no se genera ruido: el hook
// sale temprano antes de llegar al punto donde se dispara el snapshot.
func TestHookSessionEndWithoutActiveSessionSkipsBackup(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	project := persistence.ProjectKey(target)
	runHook(t, bin, target, "session-end") // sin session-start previo

	backupDir, err := persistence.BackupDir(project)
	if err != nil {
		t.Fatalf("backup dir: %v", err)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatalf("no debía crearse directorio de backups sin sesión activa (err=%v)", err)
	}
}
