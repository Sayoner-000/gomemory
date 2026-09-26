package console

import (
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
