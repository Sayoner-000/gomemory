package cli

import (
	"os"
	"path/filepath"
	"testing"

	"mem/application/ports"
)

// memSettingsRepo guarda los ajustes en memoria para la prueba.
type memSettingsRepo struct{ s ports.SettingsData }

func (m *memSettingsRepo) Read(string) ports.SettingsData              { return m.s }
func (m *memSettingsRepo) Write(_ string, s ports.SettingsData) error  { m.s = s; return nil }
func (m *memSettingsRepo) ApplyAutoApprove(string, ports.SettingsData) {}

// T046 — instalación nueva → max; existente → sin tocar.
func TestInstallCompressionDefault(t *testing.T) {
	nueva := &memSettingsRepo{}
	if err := applyInstallCompressionDefault(&Deps{SettingsRepo: nueva}, t.TempDir(), true); err != nil {
		t.Fatal(err)
	}
	if nueva.s.ContextCompressionLevel != "max" {
		t.Errorf("instalación nueva debe quedar en max, quedó %q", nueva.s.ContextCompressionLevel)
	}

	existente := &memSettingsRepo{}
	_ = applyInstallCompressionDefault(&Deps{SettingsRepo: existente}, t.TempDir(), false)
	if existente.s.ContextCompressionLevel != "" {
		t.Error("una instalación existente no debe cambiar de nivel")
	}

	fijado := &memSettingsRepo{s: ports.SettingsData{ContextCompressionLevel: "none"}}
	_ = applyInstallCompressionDefault(&Deps{SettingsRepo: fijado}, t.TempDir(), true)
	if fijado.s.ContextCompressionLevel != "none" {
		t.Error("un nivel ya fijado no se pisa")
	}
}

func TestSettingsFileExists(t *testing.T) {
	dir := t.TempDir()
	if settingsFileExists(dir) {
		t.Fatal("sin archivo no debe existir")
	}
	_ = os.MkdirAll(filepath.Join(dir, ".memory"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".memory", "settings.json"), []byte("{}"), 0o644)
	if !settingsFileExists(dir) {
		t.Error("con archivo debe existir")
	}
}
