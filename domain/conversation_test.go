package domain

import (
	"regexp"
	"testing"
)

func TestIsNewConversation(t *testing.T) {
	stored := &Conversation{ID: "A", StartedAt: 100}
	cases := []struct {
		name     string
		stored   *Conversation
		incoming string
		want     bool
	}{
		{"sin conversación registrada", nil, "A", true},
		{"mismo id: reanudación", stored, "A", false},
		{"id distinto: conversación nueva", stored, "B", true},
		// C-004 (acr_715249c3): sin id del host se continúa la conversación
		// registrada; solo se crea una si no hay ninguna.
		{"sin id del host y con conversación: continúa", stored, "", false},
		{"sin id del host y sin conversación: nueva", nil, "", true},
	}
	for _, c := range cases {
		if got := IsNewConversation(c.stored, c.incoming); got != c.want {
			t.Errorf("%s: IsNewConversation = %v, se esperaba %v", c.name, got, c.want)
		}
	}
}

func TestShouldRotate_LimiteDeInactividad(t *testing.T) {
	cases := []struct {
		idle int64
		want bool
	}{
		{StaleSessionSecs - 1, false},
		{StaleSessionSecs, false},
		{StaleSessionSecs + 1, true},
		{0, false},
		{-30, false}, // relojes con desfase: nunca rota
	}
	for _, c := range cases {
		if got := ShouldRotate(c.idle); got != c.want {
			t.Errorf("ShouldRotate(%d) = %v, se esperaba %v", c.idle, got, c.want)
		}
	}
	if StaleSessionSecs != 4*3600 {
		t.Errorf("el umbral acordado es 4 h, got %d s", StaleSessionSecs)
	}
}

func TestNewLocalConversationID_Formato(t *testing.T) {
	id := NewLocalConversationID(1791119471, "a1b2c3")
	if !regexp.MustCompile(`^local-1791119471-a1b2c3$`).MatchString(id) {
		t.Errorf("formato inesperado: %q", id)
	}
}
