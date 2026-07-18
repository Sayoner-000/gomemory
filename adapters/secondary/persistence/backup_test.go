package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBackupDir_ResolvesUnderDataHomeBackups cubre la Historia de Usuario 1
// (specs/009-mitigacion-riesgos): el directorio de snapshots automáticos vive
// bajo <DataHome>/backups/<project-key>/, reutilizando DataHome() tal cual lo
// usa GlobalProjectDir para mem.db, no un directorio inventado aparte.
func TestBackupDir_ResolvesUnderDataHomeBackups(t *testing.T) {
	home, err := DataHome()
	if err != nil {
		t.Fatalf("data home: %v", err)
	}

	dir, err := BackupDir("proj-abc123")
	if err != nil {
		t.Fatalf("backup dir: %v", err)
	}

	want := filepath.Join(home, "backups", "proj-abc123")
	if dir != want {
		t.Fatalf("esperaba %q, got %q", want, dir)
	}
}

// TestBackupDir_DistinctFromProjectDataDir confirma que backups/ no colisiona
// con el directorio de datos activo del proyecto (projects/<key>/), para que
// nunca se confundan el mem.db en uso y sus snapshots.
func TestBackupDir_DistinctFromProjectDataDir(t *testing.T) {
	key := "proj-xyz789"

	projectDir, err := GlobalProjectDir(key)
	if err != nil {
		t.Fatalf("project dir: %v", err)
	}
	backupDir, err := BackupDir(key)
	if err != nil {
		t.Fatalf("backup dir: %v", err)
	}

	if backupDir == projectDir {
		t.Fatalf("backup dir no debe coincidir con el directorio de datos activo: %q", backupDir)
	}

	// Sanity check adicional: crear ambos directorios no debe pisarse.
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatalf("project dir debía seguir existiendo: %v", err)
	}
}
