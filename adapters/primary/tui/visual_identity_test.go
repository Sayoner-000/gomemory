package tui

import (
	"strings"
	"testing"

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
