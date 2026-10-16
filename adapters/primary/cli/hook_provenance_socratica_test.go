package cli

import (
	"context"
	"testing"
	"time"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

type provenanceSessions struct {
	ports.SessionRepository
	last  string
	calls int
}

func (s *provenanceSessions) SetLastPrompt(_, prompt string) error {
	s.calls++
	s.last = prompt
	return nil
}

// TestUserPromptSubmit_ProcedenciaSocratica (T024, G1, FR-026): con la
// intercepción apagada, los prompts de Socratica y las respuestas a un flujo
// pendiente vigente del mismo agente no quedan como origen de memorias. Lo
// comparten user-prompt-submit (Claude, Codex) y `mem hook prompt` (OpenCode).
func TestUserPromptSubmit_ProcedenciaSocratica(t *testing.T) {
	ctx := context.Background()
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	repo := persistence.NewSocraticaStore(db, time.Second)
	sessions := &provenanceSessions{}
	deps := &Deps{SessionRepo: sessions, Socratica: &usecases.SocraticaFlow{Repo: repo, UOW: repo, Clock: clock.SystemClock{}}}

	check := func(client, prompt, want string) {
		t.Helper()
		sessions.last = "previo"
		recordPromptProvenance(deps, "p", prompt, classifySocraticaPrompt(deps, "p", client, usecases.PromptSignals{Text: prompt}))
		if sessions.last != want {
			t.Errorf("%s %q: origen %q, se esperaba %q", client, prompt, sessions.last, want)
		}
	}
	for _, invocation := range []string{"/socratica envía el informe", "/socratic-q migrar a X", "$socratica refina"} {
		check("claude", invocation, "")
	}
	check("claude", "arregla el test", "arregla el test")

	now := time.Now().UTC()
	pending := domain.RefinementFlow{ID: "f", SchemaVersion: 1, Project: "p", Client: "codex", Mode: domain.ModeRefine, OriginalPrompt: "o", Phase: domain.PhaseQuestions, LastStablePhase: domain.PhaseQuestions, CreatedAt: now, UpdatedAt: now}
	if err = repo.CreateFlow(ctx, pending); err != nil {
		t.Fatal(err)
	}
	check("codex", "la opción 2", "")
	check("claude", "la opción 2", "la opción 2")
	check("opencode", "la opción 2", "la opción 2")

	stale := pending
	stale.UpdatedAt = now.Add(-domain.ContinuationWindow - time.Minute)
	if err = repo.SaveTransition(ctx, pending, stale); err != nil {
		t.Fatal(err)
	}
	check("codex", "otra cosa", "otra cosa")

	// Sin Socratica cableado, la invocación explícita se reconoce igual.
	deps.Socratica = nil
	check("claude", "/socratica algo", "")
}

// TestProcedenciaSocratica_MemoriaDelTurnoSinOrigen comprueba el efecto final
// con la sesión y el guardado reales: lo guardado en un turno de Socratica no
// hereda ni su prompt ni el del turno anterior.
func TestProcedenciaSocratica_MemoriaDelTurnoSinOrigen(t *testing.T) {
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	sessions := persistence.NewSessionRepository(db)
	if _, err = sessions.Start("p"); err != nil {
		t.Fatal(err)
	}
	deps := &Deps{SessionRepo: sessions}
	memories := persistence.NewMemoryRepository(db)

	record := func(prompt string) {
		recordPromptProvenance(deps, "p", prompt, classifySocraticaPrompt(deps, "p", "claude", usecases.PromptSignals{Text: prompt}))
	}
	record("arregla el login")
	control, err := memories.Insert(&domain.Memory{Project: "p", Title: "control", Type: "learning", Content: "turno normal"})
	if err != nil {
		t.Fatal(err)
	}
	if m, err := memories.Get("p", control); err != nil || m == nil || m.OriginPrompt != "arregla el login" {
		t.Fatalf("control: un turno normal debe llevar su prompt como origen: %+v %v", m, err)
	}
	record("/socratica envía el informe al cliente")
	id, err := memories.Insert(&domain.Memory{Project: "p", Title: "decisión", Type: "decision", Content: "algo"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := memories.Get("p", id)
	if err != nil || m == nil {
		t.Fatal(m, err)
	}
	if m.OriginPrompt != "" {
		t.Fatalf("la memoria lleva origen %q en un turno de Socratica", m.OriginPrompt)
	}
}
