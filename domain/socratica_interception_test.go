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
		// mode=specify (feature 042): alias, Codex form and a natural request.
		"/socratica-hu Quiero exportar el historial":                                         true,
		"  /socratica-hu\nmulti":                                                             true,
		"$socratica prepara para speckit-specify: Quiero exportar el historial":              true,
		"Usa Socrática para preparar esta idea como entrada de Spec Kit: exportar historial": true,
		"usa socratica para preparar esto como entrada de speckit-specify":                   true,
		"/socratica-hux algo":                                    false,
		"/socratica-hu2":                                         false,
		"usa Socrática para limpiar la caché":                    false,
		"Hoy usa Socrática para preparar la entrada de Spec Kit": false,
		"No uses Socrática para preparar la entrada de Spec Kit": false,
		"prepara la entrada de Spec Kit sin Socrática":           false,
		"/socraticas algo":                                       false,
		"/socratic-question":                                     false,
		"usa /socratica luego":                                   false,
		"hola":                                                   false,
		"":                                                       false,
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
		"alias specify":         with(func(i *InterceptionInput) { i.Text = "/socratica-hu Quiero exportar el historial autorizado" }),
		"Codex specify": with(func(i *InterceptionInput) {
			i.Text = "$socratica prepara para speckit-specify: Quiero exportar el historial autorizado"
		}),
		"petición natural": with(func(i *InterceptionInput) {
			i.Text = "Usa Socrática para preparar esta idea como entrada de Spec Kit: exportar el historial autorizado"
		}),
		"adjunto":          with(func(i *InterceptionInput) { i.Attachments = true }),
		"agente ocupado":   with(func(i *InterceptionInput) { i.Busy = true }),
		"poca información": with(func(i *InterceptionInput) { i.Text = "sí, la segunda" }),
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
