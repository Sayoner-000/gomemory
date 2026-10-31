//go:build !windows

package cli

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCopiaProyecto_PermisosPrivados(t *testing.T) {
	previa := syscall.Umask(0)
	defer syscall.Umask(previa)
	for _, modo := range []os.FileMode{0o600, 0o644} {
		dir, path := proyectoCon(t, "AGENTS.md", "antes\n\n"+bloqueCompleto)
		if err := os.Chmod(path, modo); err != nil {
			t.Fatal(err)
		}
		captureStdout(t, func() { cleanupLegacyArtifacts(dir, ".memory") })
		info, err := os.Stat(filepath.Join(dir, ".memory", "backups", "agent-files", "AGENTS.md"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("original %o: copia %v, %v", modo, info.Mode().Perm(), err)
		}
	}
}
