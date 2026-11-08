package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

func TestInstallLanguageIntegration(t *testing.T) {
	// 1. mem install no interactivo con --lang=en
	tmp1 := t.TempDir()
	deps1 := &Deps{
		ProjectRepo:  persistence.NewProjectRepository(),
		SettingsRepo: persistence.NewSettingsRepository(),
	}

	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	CmdInstall(deps1, []string{tmp1, "--yes", "--agents=none", "--lang=en"})

	w.Close()
	os.Stdout = origStdout
	_, _ = buf.ReadFrom(r)
	installOut := buf.String()

	// Verificar que stdout emitió en inglés
	if !strings.Contains(installOut, "Installing gomemory in") {
		t.Errorf("stdout de install debe estar en inglés ('Installing gomemory in'), se obtuvo:\n%s", installOut)
	}

	s1 := deps1.SettingsRepo.Read(tmp1)
	if s1.Language != "en" {
		t.Errorf("se esperaba Language 'en', se obtuvo %q", s1.Language)
	}

	// 2. mem install no interactivo por defecto (sin flag)
	tmp2 := t.TempDir()
	deps2 := &Deps{
		ProjectRepo:  persistence.NewProjectRepository(),
		SettingsRepo: persistence.NewSettingsRepository(),
	}

	buf.Reset()
	r, w, _ = os.Pipe()
	os.Stdout = w

	CmdInstall(deps2, []string{tmp2, "--yes", "--agents=none"})

	w.Close()
	os.Stdout = origStdout
	_, _ = buf.ReadFrom(r)
	installOutES := buf.String()

	if !strings.Contains(installOutES, "Instalando gomemory en") {
		t.Errorf("stdout de install por defecto debe estar en español ('Instalando gomemory en'), se obtuvo:\n%s", installOutES)
	}

	s2 := deps2.SettingsRepo.Read(tmp2)
	if s2.Language != "es" {
		t.Errorf("se esperaba Language 'es' por defecto, se obtuvo %q", s2.Language)
	}

	// 3. Reinstalación en tmp1 conservando el idioma 'en' previamente guardado
	CmdInstall(deps1, []string{tmp1, "--yes", "--agents=none"})
	s1Re := deps1.SettingsRepo.Read(tmp1)
	if s1Re.Language != "en" {
		t.Errorf("reinstalación debe conservar 'en' guardado, se obtuvo %q", s1Re.Language)
	}

	// 4. Comprobar que settings.json en disco contiene language "en"
	data, err := os.ReadFile(filepath.Join(tmp1, ".memory", "settings.json"))
	if err != nil {
		t.Fatalf("leer settings.json falló: %v", err)
	}
	if !strings.Contains(string(data), `"language": "en"`) {
		t.Errorf("archivo settings.json en disco debe contener '\"language\": \"en\"', contenido: %s", string(data))
	}

	// 5. Comprobar que CmdSettings con proyecto en inglés emite etiquetas en inglés
	buf.Reset()
	r, w, _ = os.Pipe()
	os.Stdout = w

	deps1.Project = "test"
	deps1.Root = tmp1
	deps1.ProjectRepo = &fakeProjectRepo{root: tmp1}
	CmdSettings(deps1, []string{"--show"})

	w.Close()
	os.Stdout = origStdout
	_, _ = buf.ReadFrom(r)
	settingsOut := buf.String()

	if !strings.Contains(settingsOut, "CONFIGURATION") || !strings.Contains(settingsOut, "Language: en") {
		t.Errorf("CmdSettings para proyecto en inglés debe mostrar 'CONFIGURATION' y 'Language: en', se obtuvo:\n%s", settingsOut)
	}
}
