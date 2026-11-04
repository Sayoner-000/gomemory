package console

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// HCD-022 C-01/C-02: cada estado tiene un único símbolo; en terminal
// estilizada va pintado con su color de la paleta y fuera de ella va limpio.
func TestMark(t *testing.T) {
	want := map[StepState]string{
		StatePending: "○", StateRunning: "●", StateDone: "✓", StateWarn: "⚠",
		StateFail: "✗", StateSkip: "−", StateSelected: "▸", StateFlagged: "⚑",
	}
	styled := Env{StdoutTTY: true, Width: 80, Getenv: func(string) string { return "" }}
	plainEnvs := map[string]Env{
		"sin TTY":  {Width: 80, Getenv: func(string) string { return "" }},
		"NO_COLOR": {StdoutTTY: true, Width: 80, Getenv: func(k string) string { return map[string]string{"NO_COLOR": "1"}[k] }},
		"dumb":     {StdoutTTY: true, Width: 80, Getenv: func(k string) string { return map[string]string{"TERM": "dumb"}[k] }},
	}
	for state, glyph := range want {
		got := NewLayout(styled).Mark(state)
		if ansi.Strip(got) != glyph {
			t.Errorf("estado %d: símbolo %q, se esperaba %q", state, ansi.Strip(got), glyph)
		}
		if !strings.Contains(got, "\x1b[") {
			t.Errorf("estado %d: en terminal estilizada debe ir pintado: %q", state, got)
		}
		for name, e := range plainEnvs {
			if got := NewLayout(e).Mark(state); got != glyph {
				t.Errorf("%s, estado %d: %q, se esperaba %q sin ANSI", name, state, got, glyph)
			}
		}
	}
	if NewLayout(styled).Mark(StateDone) == NewLayout(styled).Mark(StateFail) {
		t.Error("completado y error deben pintarse distinto")
	}
}

// HCD-022 C-05: Layout pinta con Mark cualquier línea que empiece por un
// glifo del juego, también ○ ● ▸ ⚑, para que humanf coloree igual que Mark.
func TestLayout_ColoreaTodosLosEstados(t *testing.T) {
	l := NewLayout(Env{StdoutTTY: true, Width: 80, Getenv: func(string) string { return "" }})
	for _, s := range []StepState{StatePending, StateRunning, StateDone, StateWarn, StateFail, StateSkip, StateSelected, StateFlagged} {
		out := l.Document("  " + stateGlyph(s) + " agy: texto de prueba\n")
		if !strings.Contains(out, l.Mark(s)) {
			t.Errorf("estado %d (%s): Layout no lo pinta con Mark: %q", s, stateGlyph(s), out)
		}
	}
}
