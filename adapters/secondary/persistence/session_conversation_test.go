package persistence

import "testing"

type conversacionFija string

func (c conversacionFija) Current() string { return string(c) }
func (conversacionFija) Adopt(string)      {}

// HCD-036: cada conversación de agente tiene su propia sesión. Abrir, leer o
// cerrar la sesión de una no toca la de otra; sin conversación, todo sigue como
// antes (la última sesión abierta del proyecto).
func TestSesionPorConversacion(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	a := NewSessionRepository(db).(*SessionRepository).WithConversation(conversacionFija("agente-A"))
	b := NewSessionRepository(db).(*SessionRepository).WithConversation(conversacionFija("agente-B"))
	legado := NewSessionRepository(db)

	sa, err := a.Start("p")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Active("p"); got != nil {
		t.Fatalf("B no ve la sesión de A como suya: %+v", got)
	}
	sb, err := b.Start("p")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Active("p"); got == nil || got.ID != sa.ID {
		t.Fatalf("A conserva su sesión: %+v", got)
	}
	if err := a.SetLastPrompt("p", "prompt de A"); err != nil {
		t.Fatal(err)
	}
	if err := b.End(sb.ID, "cierre de B"); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Active("p"); got == nil || got.ID != sa.ID {
		t.Fatalf("C-04: el cierre de B no cierra la sesión de A: %+v", got)
	}
	var promptA, promptB string
	_ = db.QueryRow(`SELECT COALESCE(last_prompt,'') FROM sessions WHERE id = ?`, sa.ID).Scan(&promptA)
	_ = db.QueryRow(`SELECT COALESCE(last_prompt,'') FROM sessions WHERE id = ?`, sb.ID).Scan(&promptB)
	if promptA != "prompt de A" || promptB != "" {
		t.Fatalf("el prompt va a la sesión de su conversación: A=%q B=%q", promptA, promptB)
	}
	// Sin conversación, la sesión de proyecto SIN conversación, como antes; nunca
	// la de un agente identificado.
	if got, _ := legado.Active("p"); got != nil {
		t.Fatalf("un proceso sin conversación no toma la sesión de A: %+v", got)
	}
	sl, err := legado.Start("p")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := legado.Active("p"); got == nil || got.ID != sl.ID {
		t.Fatalf("C-05: sin conversación, su propia sesión de proyecto: %+v", got)
	}
	if got, _ := a.Active("p"); got == nil || got.ID != sa.ID {
		t.Fatalf("la sesión sin conversación no tapa la de A: %+v", got)
	}
}
