package domain

import (
	"testing"
	"time"
)

func TestIsSocraticaInvocation(t *testing.T) {
	for text, want := range map[string]bool{
		"/socratica":                  true,
		"/socratica envía el informe": true,
		"  /socratic-q migrar a X":    true,
		"$socratica refina esto":      true,
		"/socratica\nmulti":           true,
		"/socraticas algo":            false,
		"/socratic-question":          false,
		"usa /socratica luego":        false,
		"hola":                        false,
		"":                            false,
	} {
		if got := IsSocraticaInvocation(text); got != want {
			t.Errorf("%q: got %v want %v", text, got, want)
		}
	}
}

func TestDecideInterception(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	base := InterceptionInput{Text: "envía el informe trimestral al cliente", Enabled: true, Now: now}
	with := func(f func(*InterceptionInput)) InterceptionInput {
		in := base
		f(&in)
		return in
	}
	if d := DecideInterception(base); !d.Capture || d.Directive != SocraticaDirective {
		t.Fatalf("debe capturar: %+v", d)
	}
	for name, in := range map[string]InterceptionInput{
		"comando con barra":     with(func(i *InterceptionInput) { i.Text = "/plan revisa esto ahora mismo" }),
		"invocación $socratica": with(func(i *InterceptionInput) { i.Text = "$socratica refina esto por favor" }),
		"adjunto":               with(func(i *InterceptionInput) { i.Attachments = true }),
		"agente ocupado":        with(func(i *InterceptionInput) { i.Busy = true }),
		"poca información":      with(func(i *InterceptionInput) { i.Text = "sí, la segunda" }),
		"pendiente en preguntas": with(func(i *InterceptionInput) {
			i.PendingPhase, i.PendingUpdatedAt = PhaseQuestions, now.Add(-time.Minute)
		}),
		"pendiente en revisión": with(func(i *InterceptionInput) {
			i.PendingPhase, i.PendingUpdatedAt = PhaseReview, now.Add(-ContinuationWindow)
		}),
		"origen aprobado": with(func(i *InterceptionInput) { i.Origin = OriginSocraticaApproved }),
		"deshabilitada":   with(func(i *InterceptionInput) { i.Enabled = false }),
	} {
		if d := DecideInterception(in); d.Capture {
			t.Errorf("%s: no debía capturar", name)
		}
	}
	// Un pendiente viejo ya no cuenta como continuación (M1).
	old := with(func(i *InterceptionInput) {
		i.PendingPhase, i.PendingUpdatedAt = PhaseReview, now.Add(-ContinuationWindow-time.Second)
	})
	if d := DecideInterception(old); !d.Capture {
		t.Fatal("un pendiente fuera de la ventana no debe impedir la captura")
	}
	for text, want := range map[string]string{">hola": "hola", ">>hola": ">hola", "  >envía": "envía"} {
		d := DecideInterception(with(func(i *InterceptionInput) { i.Text = text }))
		if d.Capture || !d.Forwarded || d.Forward != want || d.Directive != SocraticaForwardDirective {
			t.Errorf("%q: %+v", text, d)
		}
	}
	if d := DecideInterception(with(func(i *InterceptionInput) { i.Text, i.Enabled = ">hola", false })); d.Forwarded || d.Capture {
		t.Fatal("deshabilitada nunca toca el prompt", d)
	}
}
