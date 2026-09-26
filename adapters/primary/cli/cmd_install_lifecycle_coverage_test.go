package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// El flujo completo de install también debe quedar cubierto en el paquete CLI:
// las pruebas de contrato lanzan otro binario y no cuentan en su cobertura.
func TestCmdInstall_Cursor_Reinstalacion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("la prueba usa un ejecutable simulado de shell")
	}
	base := t.TempDir()
	home := filepath.Join(base, "home")
	bin := filepath.Join(base, "bin")
	project := filepath.Join(base, "project")
	for _, dir := range []string{home, bin, project} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("GOMEMORY_DATA_HOME", filepath.Join(base, "data"))
	t.Setenv("PATH", strings.Join([]string{bin, "/usr/bin", "/bin"}, string(os.PathListSeparator)))
	// El global simulado permite ejercer init/seed sin afectar el HOME real.
	global := filepath.Join(bin, memBinaryName())
	if err := os.WriteFile(global, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	deps := &Deps{SettingsRepo: persistence.NewSettingsRepository(), ProjectRepo: persistence.NewProjectRepository()}
	for i := 0; i < 2; i++ {
		args := []string{"--yes", project}
		if i == 0 {
			args = append(args, "--agents", "cursor")
		}
		CmdInstall(deps, args)
		if _, err := os.Stat(filepath.Join(project, memBinaryName())); !os.IsNotExist(err) {
			t.Fatalf("pasada %d: se creó una copia local: %v", i+1, err)
		}
	}
	settings := deps.SettingsRepo.Read(project)
	if len(settings.Agents) != 1 || settings.Agents[0] != "cursor" || settings.AgentScope != "project" {
		t.Fatalf("selección guardada perdida: %+v", settings)
	}
	data, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil || !strings.Contains(string(data), ".memory/") {
		t.Fatalf("falta .gitignore de install: %v, %q", err, data)
	}
}
