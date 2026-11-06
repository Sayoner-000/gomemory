package cli

import (
	"testing"

	"mem/adapters/secondary/persistence"
)

// conversacionDeHook simula el resolver de un proceso: adopta el id del hook.
type conversacionDeHook struct{ id string }

func (c *conversacionDeHook) Current() string  { return c.id }
func (c *conversacionDeHook) Adopt(id string)  { c.id = id }
func (c *conversacionDeHook) adoptado() string { return c.id }

type enlaceAnotado struct{ publicados []string }

func (e *enlaceAnotado) Publish(id string) error { e.publicados = append(e.publicados, id); return nil }
func (e *enlaceAnotado) Lookup() (string, bool)  { return "", false }

// HCD-036 tarea 5: cada hook adopta el session_id de su payload; SessionStart
// además publica el enlace con su host; el cierre de un agente cierra solo su
// sesión.
func TestSesionPropia(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	agente := func() (*Deps, *conversacionDeHook, *enlaceAnotado) {
		conv, enlace := &conversacionDeHook{}, &enlaceAnotado{}
		repo := persistence.NewSessionRepository(db).(*persistence.SessionRepository).WithConversation(conv)
		return &Deps{SessionRepo: repo, Conversation: conv, HostBinding: enlace}, conv, enlace
	}
	depsA, convA, enlaceA := agente()
	depsB, _, _ := agente()

	adoptHookConversation(depsA, map[string]any{"session_id": "agente-A"}, true)
	if convA.adoptado() != "agente-A" || len(enlaceA.publicados) != 1 || enlaceA.publicados[0] != "agente-A" {
		t.Fatalf("SessionStart adopta y publica: conv=%q publicados=%v", convA.adoptado(), enlaceA.publicados)
	}
	sa, err := depsA.SessionRepo.Start("p")
	if err != nil {
		t.Fatal(err)
	}
	adoptHookConversation(depsB, map[string]any{"session_id": "agente-B"}, false)
	if _, err := depsB.SessionRepo.Start("p"); err != nil {
		t.Fatal(err)
	}
	// Un hook sin session_id no cambia la conversación adoptada.
	adoptHookConversation(depsA, map[string]any{}, false)
	if convA.adoptado() != "agente-A" {
		t.Fatalf("un payload sin id no borra la conversación: %q", convA.adoptado())
	}

	endOwnSession(depsB, "p", map[string]any{"session_id": "agente-B", "summary": "fin de B"})
	if got, _ := depsA.SessionRepo.Active("p"); got == nil || got.ID != sa.ID {
		t.Fatalf("C-04: el SessionEnd de B no cierra la sesión de A: %+v", got)
	}
	if got, _ := depsB.SessionRepo.Active("p"); got != nil {
		t.Fatalf("la sesión de B queda cerrada: %+v", got)
	}
}
