package cli

import "testing"

// T054 (feature 035, FR-020): el recordatorio de guardado mide desde el inicio
// de la conversación (o desde el último guardado si es posterior), no desde la
// sesión de memoria, que puede llevar horas abierta.
func TestSaveNudgeDue(t *testing.T) {
	cases := []struct {
		name       string
		age, since int64
		want       bool
	}{
		{"conversación de 2 min en una sesión vieja", 120, -1, false},
		{"conversación de 20 min sin guardar desde que empezó", 1200, 5 * 3600, true},
		{"conversación de 20 min con un guardado hace 5 min", 1200, 300, false},
		{"sin guardados y conversación de 10 min", 600, -1, false},
		{"sin guardados y conversación de 16 min", 960, -1, true},
	}
	for _, c := range cases {
		if got := saveNudgeDue(c.age, c.since); got != c.want {
			t.Errorf("%s: saveNudgeDue(%d, %d) = %v, se esperaba %v", c.name, c.age, c.since, got, c.want)
		}
	}
}

func TestConversationAge_SinConversacionUsaLaSesion(t *testing.T) {
	root := t.TempDir()
	if age, ok := conversationAge(root); ok || age != 0 {
		t.Errorf("sin .conversation no hay edad propia: %d %v", age, ok)
	}
}
