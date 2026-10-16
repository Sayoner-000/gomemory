package tui

import (
	"fmt"
	"strings"
	"testing"

	"mem/domain"

	"github.com/charmbracelet/x/ansi"
)

func TestEmptyStateExplainsNextActionAndUsesSharedBrand(t *testing.T) {
	m := newTestModel(nil, 24)
	m.width = 80
	view := ansi.Strip(m.listView())
	if !strings.Contains(view, "goMemory ›") || !strings.Contains(view, "s guardar tu primer aprendizaje") {
		t.Fatalf("sin identidad o siguiente acción: %q", view)
	}
	m.filterInput.SetValue("inexistente")
	view = ansi.Strip(m.listView())
	if !strings.Contains(view, "esc limpiar filtro") {
		t.Fatalf("búsqueda vacía sin remedio: %q", view)
	}
}

func TestVisualIdentityFitsAllThemes(t *testing.T) {
	defer applyTheme("dark")
	for _, theme := range []string{"dark", "light", "matrix"} {
		applyTheme(theme)
		for _, width := range []int{40, 80, 120} {
			m := newTestModel(nil, 24)
			m.width, m.theme = width, theme
			out := m.renderView()
			for _, line := range strings.Split(out, "\n") {
				if ansi.StringWidth(line) > width {
					t.Errorf("%s ancho %d: %q", theme, width, line)
				}
			}
			if len(strings.Split(out, "\n")) > 24 {
				t.Errorf("%s desborda altura", theme)
			}
		}
	}
}

// TestSocraticaTokens (T050, contracts/tui.md): valores exactos y sin emojis.
func TestSocraticaTokens(t *testing.T) {
	hex := func(c interface{ RGBA() (r, g, b, a uint32) }) string {
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
	}
	for name, tc := range map[string]struct {
		got  interface{ RGBA() (r, g, b, a uint32) }
		want string
	}{
		"socDone": {socDone, "#3DD68C"}, "socCurrent": {socCurrent, "#F2C46D"}, "socFg": {socFg, "#E6EDF3"},
		"socLabel": {socLabel, "#7D8590"}, "socControl": {socControl, "#262B33"}, "socWarn": {socWarn, "#E3B341"},
	} {
		if got := hex(tc.got); got != tc.want {
			t.Errorf("%s = %s, se esperaba %s", name, got, tc.want)
		}
	}
	defer applyTheme("dark")
	for _, theme := range []string{"dark", "light"} {
		applyTheme(theme)
		if hex(socUpcoming()) != hex(faint) {
			t.Errorf("%s: socUpcoming no sigue el atenuado del tema", theme)
		}
	}

	s := newSocTUI(t)
	s.gen.questions = []domain.Question{{Text: "¿Para quién?", Options: []string{"Equipo", "Cliente"}}}
	s.refine("envía el informe")
	for _, sc := range []screen{screenSocratica, screenSocraticaQuestions, screenSocraticaDetail} {
		s.m.screen = sc
		for _, r := range ansi.Strip(s.m.renderView()) {
			if r >= 0x1F300 || (r >= 0x2600 && r <= 0x27BF && r != '✓') {
				t.Errorf("pantalla %d: emoji %q en el render de Socratica", sc, r)
			}
		}
	}
}
