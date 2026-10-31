package cli

import (
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPlanTitle_CortaEnRuna(t *testing.T) {
	got := planTitle(strings.Repeat("a", 79) + "ñ final")
	if !utf8.ValidString(got) {
		t.Fatalf("título inválido: %q", got)
	}
}

func TestExportMode_FalloDeChmod(t *testing.T) {
	boom := errors.New("sin permiso")
	err := ensureExportMode(func(os.FileMode) error { return boom }, "bundle.json")
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "bundle.json") {
		t.Fatalf("err = %v", err)
	}
	var mode os.FileMode
	if err = ensureExportMode(func(m os.FileMode) error { mode = m; return nil }, "b"); err != nil || mode != 0o600 {
		t.Fatalf("modo = %o, err = %v", mode, err)
	}
}
