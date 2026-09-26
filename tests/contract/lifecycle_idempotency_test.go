package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// configSnapshot resume la configuración escrita (proyecto fuera de .memory y
// directorio personal). La base de datos y los marcadores de sesión cambian
// legítimamente en cada ejecución y quedan fuera.
func configSnapshot(t *testing.T, s *lifecycleSandbox, p string) map[string]string {
	t.Helper()
	all := snapshot(t, p, s.Home)
	out := map[string]string{}
	for k, v := range all {
		if strings.Contains(k, string(filepath.Separator)+".memory") || strings.HasPrefix(k, s.BinDir) {
			continue
		}
		out[k] = v
	}
	return out
}

// FR-035: repetir install, la retirada de copias y la simulación de
// desinstalación sobre el mismo estado no produce cambios ni errores.
func TestCicloDeVida_Idempotente(t *testing.T) {
	s := newLifecycleSandbox(t)
	s.globalSetup()
	p := s.Project("p1")
	copyExecutable(t, buildLifecycleBinary(t), filepath.Join(p, exeName("mem")))

	if r := s.Run("", p, "install", "--yes", p); r.ExitCode != 0 {
		t.Fatalf("install 1: %s", r.Stdout+r.Stderr)
	}
	antes := configSnapshot(t, s, p)

	r := s.Run("", p, "install", "--yes", p)
	if r.ExitCode != 0 {
		t.Fatalf("install 2: %s", r.Stdout+r.Stderr)
	}
	if strings.Contains(r.Stdout, "Se retiró") {
		t.Error("la segunda instalación no tiene copia que retirar")
	}
	despues := configSnapshot(t, s, p)
	for k, v := range antes {
		if despues[k] != v {
			t.Errorf("la segunda instalación cambió %s", k)
		}
	}
	if len(despues) != len(antes) {
		t.Errorf("la segunda instalación creó o borró archivos: %d → %d", len(antes), len(despues))
	}

	d1 := s.Run("", p, "uninstall", "--all", "--dry-run", "--no-scan")
	d2 := s.Run("", p, "uninstall", "--all", "--dry-run", "--no-scan")
	if d1.ExitCode != 0 || d1.Stdout != d2.Stdout {
		t.Errorf("la simulación repetida debe dar lo mismo:\n%s\n---\n%s", d1.Stdout, d2.Stdout)
	}
}
