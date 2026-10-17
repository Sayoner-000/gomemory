package tui

import (
	"context"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// TestSocraticaTUI_PintaSinSecuencias cierra H-01 (auditoría 2026-10-17,
// ronda 3) en la TUI: original, preguntas y opciones se pintan sin secuencias
// de control, y elegir una opción sigue enviando sus bytes exactos.
func TestSocraticaTUI_PintaSinSecuencias(t *testing.T) {
	const option = "Equipo \x1b[2J"
	s := newSocTUI(t)
	s.gen.questions = []domain.Question{{Text: "¿Para \x1b]0;PWNED\x07 quién?", Options: []string{option, "Cliente"}}}
	r, err := s.m.soc.deps.Flow.Start(context.Background(), "p", "claude", domain.ModeRefine, "envía \x1b]52;c;aGFjaw==\x07 el informe")
	if err != nil {
		t.Fatal(err)
	}
	if r, err = s.m.soc.deps.Flow.Apply(context.Background(), r.Flow.ID, domain.Event{Kind: "resume"}, usecases.OriginLocal); err != nil || len(r.Flow.Questions) != 1 {
		t.Fatal(r.Flow.Phase, err)
	}
	painted := func(where, visible string) {
		t.Helper()
		v := s.m.renderView()
		if strings.Contains(v, "\x1b]") || strings.Contains(v, "\x07") || strings.Contains(v, "\x1b[2J") {
			t.Fatalf("%s pinta secuencias de control:\n%q", where, v)
		}
		if !strings.Contains(v, visible) {
			t.Fatalf("%s perdió el texto visible %q:\n%q", where, visible, v)
		}
	}
	s.m.screen = screenSocratica
	s.m.soc.pending = []usecases.SocraticaResult{r}
	painted("la lista", "]52;c;aGFjaw==")
	s.m.soc.current = r
	s.m.screen = screenSocraticaQuestions
	painted("las preguntas", "]0;PWNED")
	s.m.screen = screenSocraticaDetail
	painted("el detalle", "]52;c;aGFjaw==")

	s.m.screen = screenSocraticaQuestions
	s.press("1")
	f, err := s.store.GetFlow(context.Background(), r.Flow.ID)
	if err != nil || len(f.Answers) != 1 || f.Answers[0].Choice != option {
		t.Fatalf("la respuesta perdió bytes: %+v %v", f.Answers, err)
	}
}
