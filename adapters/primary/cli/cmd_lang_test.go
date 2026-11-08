package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

func TestCmdLang(t *testing.T) {
	tmp := t.TempDir()
	memDir := filepath.Join(tmp, persistence.MemDir)
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsRepo := persistence.NewSettingsRepository()
	s := settingsRepo.Read(tmp)
	s.Language = "es"
	_ = settingsRepo.Write(tmp, s)

	deps := &Deps{
		ProjectRepo:  &fakeProjectRepo{root: tmp},
		SettingsRepo: settingsRepo,
	}

	// 1. mem lang sin argumentos muestra el idioma actual
	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	CmdLang(deps, []string{})

	w.Close()
	os.Stdout = origStdout
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "es") {
		t.Errorf("se esperaba mención de 'es', se obtuvo: %s", out)
	}

	// 2. mem lang en cambia en caliente a inglés
	buf.Reset()
	r, w, _ = os.Pipe()
	os.Stdout = w

	CmdLang(deps, []string{"en"})

	w.Close()
	os.Stdout = origStdout
	_, _ = buf.ReadFrom(r)
	out = buf.String()
	if !strings.Contains(out, "English") && !strings.Contains(out, "en") {
		t.Errorf("se esperaba confirmación de cambio a English, se obtuvo: %s", out)
	}

	// Verificar persistencia inmediata
	updated := settingsRepo.Read(tmp)
	if updated.Language != "en" {
		t.Errorf("Language persistido debe ser 'en', se obtuvo %s", updated.Language)
	}

	// 3. Probar flag --lang en CmdSettings
	buf.Reset()
	r, w, _ = os.Pipe()
	os.Stdout = w

	CmdSettings(deps, []string{"--lang=es"})

	w.Close()
	os.Stdout = origStdout
	_, _ = buf.ReadFrom(r)

	updated = settingsRepo.Read(tmp)
	if updated.Language != "es" {
		t.Errorf("CmdSettings con --lang=es debe persistir 'es', se obtuvo %s", updated.Language)
	}
}
