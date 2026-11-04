package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"github.com/charmbracelet/x/ansi"

	"mem/domain"
)

// HCD-022 C-04: la TUI marca los errores con el ✗ de console y los tipos de
// memoria con símbolos del juego, sin ✕ ni ♥.
func TestTUI_ErrorUsaMarcaDeConsole(t *testing.T) {
	m := model{
		screen:           screenUsage,
		project:          "proj",
		memRepo:          &fakeMemRepo{},
		usageTaskInput:   textinput.New(),
		usageBudgetInput: textinput.New(),
		width:            100,
		height:           40,
		ready:            true,
		usageSnapshotErr: "la tarea no puede estar vacía",
	}
	view := ansi.Strip(m.usageView())
	if !strings.Contains(view, "✗ la tarea no puede estar vacía") {
		t.Errorf("el error debe marcarse con ✗:\n%s", view)
	}
	if strings.Contains(view, "✕") {
		t.Errorf("la TUI no debe usar ✕:\n%s", view)
	}
	if got := typeIcon(string(domain.Preference)); got != "⚑" {
		t.Errorf("preferencia: %q, se esperaba ⚑", got)
	}
	if got := typeIcon(string(domain.Bugfix)); got != "✗" {
		t.Errorf("bugfix: %q, se esperaba ✗", got)
	}
}
