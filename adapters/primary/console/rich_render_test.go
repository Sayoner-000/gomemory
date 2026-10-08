package console

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbletea/v2"
)

func TestRichModels_RenderizanAntesYDespuesDeConfirmar(t *testing.T) {
	opts := []Option{{Label: "Claude", Value: "claude", Checked: true}, {Label: "Codex", Value: "codex", Recommended: true, Hint: "Disponible"}}
	for _, tc := range []struct {
		name  string
		model tea.Model
	}{
		{"selección múltiple", newMultiSelectModel("Agentes", opts)},
		{"selección única", newSelectModel("Alcance", opts)},
		{"confirmación", confirmModel{title: "Continuar", answer: true}},
		{"confirmación escrita", typedModel{title: "Eliminar", word: "si", input: textinput.New()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = tc.model.Init()
			_ = tc.model.View()
			next, _ := tc.model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			_ = next.View()
		})
	}
}

// Composición del selector de referencia ("17 agents"): instrucción, cursor ▸,
// casillas [x]/[ ], contador en el título y atajos al pie separados por •.
func TestMultiSelect_ComposicionDeReferencia(t *testing.T) {
	m := newMultiSelectModel("¿Qué agentes quieres configurar?", []Option{
		{Value: "claude", Label: "Claude Code", Hint: "detectado", Checked: true},
		{Value: "codex", Label: "Codex", Checked: true},
		{Value: "opencode", Label: "OpenCode"},
	})
	text := ansi.Strip(m.View().Content)
	for _, want := range []string{"2 de 3 seleccionados", "▸ [x] Claude Code", "  [x] Codex", "  [ ] OpenCode", "detectado", "espacio marcar • enter confirmar • esc cancelar"} {
		if !strings.Contains(text, want) {
			t.Fatalf("falta %q en:\n%s", want, text)
		}
	}
	for _, old := range []string{"◻", "◼", "› "} {
		if strings.Contains(text, old) {
			t.Fatalf("glifo anterior %q:\n%s", old, text)
		}
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	next, _ = next.(multiSelectModel).Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if text := ansi.Strip(next.(multiSelectModel).View().Content); !strings.Contains(text, "1 de 3 seleccionados") || !strings.Contains(text, "▸ [ ] Codex") {
		t.Fatalf("el contador y el cursor deben seguir la interacción:\n%s", text)
	}
}

func TestSelect_CursorDeReferenciaYPaletaPropia(t *testing.T) {
	m := newSelectModel("¿Dónde?", []Option{{Value: "g", Label: "Global", Recommended: true}, {Value: "p", Label: "Proyecto"}})
	view := m.View().Content
	text := ansi.Strip(view)
	if !strings.Contains(text, "▸ Global (recomendado)") || !strings.Contains(text, "  Proyecto") || !strings.Contains(text, "enter elegir • esc cancelar") {
		t.Fatalf("selector único:\n%s", text)
	}
	if strings.Contains(view, "\x1b[32m") {
		t.Fatalf("color ANSI fijo fuera de la paleta gomemory: %q", view)
	}
}

func TestMultiSelect_PistasAlineadasEnColumna(t *testing.T) {
	m := newMultiSelectModel("Agentes", []Option{{Value: "c", Label: "Claude Code", Hint: "detectado"}, {Value: "x", Label: "Codex", Hint: "detectado"}})
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	cols := []int{}
	for _, l := range lines {
		if i := strings.Index(l, "detectado"); i >= 0 {
			cols = append(cols, ansi.StringWidth(l[:i]))
		}
	}
	if len(cols) != 2 || cols[0] != cols[1] {
		t.Fatalf("pistas desalineadas %v:\n%s", cols, strings.Join(lines, "\n"))
	}
}
