package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/primary/console"
	"mem/adapters/secondary/persistence"
)

func TestCmdLangSwitch(t *testing.T) {
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

	// 1. Switch en caliente de es a en
	CmdLang(deps, []string{"en"})
	if settingsRepo.Read(tmp).Language != "en" {
		t.Errorf("Language debe ser 'en'")
	}

	// 2. Switch en caliente de en de vuelta a es
	CmdLang(deps, []string{"es"})
	if settingsRepo.Read(tmp).Language != "es" {
		t.Errorf("Language debe ser 'es'")
	}

	// 3. Código no soportado 'xx' debe fallar con código 2 y mensaje de disponibles
	var exitCode int
	origExit := exitProcess
	exitProcess = func(c int) {
		exitCode = c
		panic("exit")
	}
	defer func() { exitProcess = origExit }()

	var errBuf bytes.Buffer
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	func() {
		defer func() {
			_ = recover()
		}()
		CmdLang(deps, []string{"xx"})
	}()

	w.Close()
	os.Stderr = origStderr
	_, _ = errBuf.ReadFrom(r)

	if exitCode != 2 {
		t.Errorf("se esperaba exit code 2 para código de idioma inválido, se obtuvo %d", exitCode)
	}
	errMsg := errBuf.String()
	if !strings.Contains(errMsg, "xx") || !strings.Contains(errMsg, "es") {
		t.Errorf("mensaje de error debe mencionar 'xx' y disponibles: %s", errMsg)
	}

	// 4. Ayuda con --help
	var outBuf bytes.Buffer
	origStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	CmdLang(deps, []string{"--help"})

	wOut.Close()
	os.Stdout = origStdout
	_, _ = outBuf.ReadFrom(rOut)

	if !strings.Contains(outBuf.String(), "mem lang") {
		t.Errorf("ayuda debe mencionar 'mem lang', se obtuvo: %s", outBuf.String())
	}

	// 5. Selector interactivo sin argumentos (selecciona 'en')
	s.Language = "es"
	_ = settingsRepo.Write(tmp, s)

	fakeUI := &fakeLangUI{selectedLang: "en"}
	langUI = fakeUI
	defer func() { langUI = nil }()

	CmdLang(deps, []string{})
	if settingsRepo.Read(tmp).Language != "en" {
		t.Errorf("selector interactivo debe cambiar idioma a 'en'")
	}

	// 6. Selector interactivo cancelado
	fakeCancelUI := &fakeCancelLangUI{}
	langUI = fakeCancelUI

	outBuf.Reset()
	rOut, wOut, _ = os.Pipe()
	os.Stdout = wOut

	CmdLang(deps, []string{})

	wOut.Close()
	os.Stdout = origStdout
	_, _ = outBuf.ReadFrom(rOut)

	if !strings.Contains(outBuf.String(), "Cancelado") && !strings.Contains(outBuf.String(), "Canceled") {
		t.Errorf("cancelación interactiva debe emitir 'Cancelado' o 'Canceled', se obtuvo: %s", outBuf.String())
	}
	if settingsRepo.Read(tmp).Language != "en" {
		t.Errorf("cancelación interactiva no debe cambiar el idioma")
	}
}

type fakeCancelLangUI struct {
	console.UI
}

func (f *fakeCancelLangUI) Select(title string, opts []console.Option) (string, error) {
	return "", console.ErrCanceled
}
