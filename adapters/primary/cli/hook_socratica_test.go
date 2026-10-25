package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// spySocraticaRepo cuenta las lecturas y consumos que hace el hook.
type spySocraticaRepo struct {
	ports.SocraticaRepository
	active, consume int
}

func (s *spySocraticaRepo) ActiveFlow(ctx context.Context, p, c string) (domain.RefinementFlow, bool, error) {
	s.active++
	return s.SocraticaRepository.ActiveFlow(ctx, p, c)
}
func (s *spySocraticaRepo) ConsumeDispatch(ctx context.Context, id, text string) (bool, error) {
	s.consume++
	return s.SocraticaRepository.ConsumeDispatch(ctx, id, text)
}

type hookSocEnv struct {
	store *persistence.SocraticaStore
	spy   *spySocraticaRepo
	deps  *Deps
}

func newHookSocEnv(tb testing.TB, settings domain.SocraticaSettings) *hookSocEnv {
	tb.Helper()
	tb.Setenv("GOMEMORY_DATA_HOME", tb.TempDir())
	db, err := persistence.Init(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = db.Close() })
	store := persistence.NewSocraticaStore(db, time.Second)
	spy := &spySocraticaRepo{SocraticaRepository: store}
	u := &usecases.SocraticaFlow{Repo: spy, UOW: store, Clock: clock.SystemClock{}, Settings: settings, NewID: func() string { return "id" }}
	return &hookSocEnv{store: store, spy: spy, deps: &Deps{Socratica: u}}
}

const capturable = "envía el informe trimestral al cliente antes del cierre"

func unverified(client string) domain.SocraticaSettings {
	return domain.SocraticaSettings{Interception: map[string]bool{client: true}, InterceptionUnverified: map[string]bool{client: true}}
}

// The specify entries (alias, Codex form, natural request) are explicit
// invocations: they keep the prompt out of general provenance and are never
// captured by interception, which an agent cannot switch on by itself.
func TestUserPromptSubmit_SocraticaSpecifyEntries(t *testing.T) {
	for _, text := range []string{
		"/socratica-hu Quiero exportar el historial autorizado de cada persona",
		"$socratica prepara para speckit-specify: Quiero exportar el historial autorizado de cada persona",
		"Usa Socrática para preparar esta idea como entrada de Spec Kit: exportar el historial autorizado",
	} {
		e := newHookSocEnv(t, unverified("codex"))
		c := classifySocraticaPrompt(e.deps, "p", "codex", usecases.PromptSignals{Text: text})
		if !c.SkipProvenance || c.Capture || c.Directive != "" || e.spy.active != 0 {
			t.Errorf("%q: %+v (active reads %d)", text, c, e.spy.active)
		}
	}
	// Without interception enabled nothing changes either, and plain text is not an invocation.
	off := newHookSocEnv(t, domain.SocraticaSettings{})
	if c := classifySocraticaPrompt(off.deps, "p", "claude", usecases.PromptSignals{Text: "Usa Socrática para limpiar la caché del servidor"}); c.SkipProvenance || c.Capture {
		t.Errorf("an unrelated sentence was treated as an invocation: %+v", c)
	}
}

// TestUserPromptSubmit_Socratica (T055).
func TestUserPromptSubmit_Socratica(t *testing.T) {
	t.Run("capture emits the directive in each dialect", func(t *testing.T) {
		e := newHookSocEnv(t, unverified("claude"))
		c := classifySocraticaPrompt(e.deps, "p", "claude", usecases.PromptSignals{Text: capturable})
		if !c.Capture || !c.SkipProvenance || c.Directive != domain.SocraticaDirective {
			t.Fatal(c)
		}
		text := fitPromptSectionsText(appendSocraticaSection(nil, c))
		for _, d := range []hookDialect{dialectClaude, dialectJSON} {
			var payload struct {
				HookSpecificOutput struct {
					AdditionalContext string `json:"additionalContext"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(renderPromptContext(d, text).stdout), &payload); err != nil || !strings.Contains(payload.HookSpecificOutput.AdditionalContext, "Socratica (intercepción automática)") {
				t.Errorf("%s: %v %q", d, err, payload.HookSpecificOutput.AdditionalContext)
			}
		}
		if out := renderPromptContext(dialectText, text).stdout; !strings.HasPrefix(out, "Socratica (intercepción automática)") {
			t.Errorf("dialecto neutral: %q", out)
		}
	})
	t.Run("escape prefix", func(t *testing.T) {
		e := newHookSocEnv(t, unverified("codex"))
		c := classifySocraticaPrompt(e.deps, "p", "codex", usecases.PromptSignals{Text: ">" + capturable})
		if c.Capture || !c.Forwarded || c.Forward != capturable || c.Directive != domain.SocraticaForwardDirective {
			t.Fatal(c)
		}
	})
	t.Run("approved dispatch is consumed once", func(t *testing.T) {
		e := newHookSocEnv(t, unverified("claude"))
		ctx := context.Background()
		f := domain.RefinementFlow{ID: "f", SchemaVersion: 1, Project: "p", Client: "claude", Mode: domain.ModeRefine, OriginalPrompt: "o", ProposedPrompt: capturable, Phase: domain.PhaseReview, LastStablePhase: domain.PhaseReview, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := e.store.CreateFlow(ctx, f); err != nil {
			t.Fatal(err)
		}
		approved, _, err := e.store.Approve(ctx, "f", "d-1", capturable)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = e.store.Ack(ctx, "f", approved.DispatchID, "", "n"); err != nil {
			t.Fatal(err)
		}
		sig := usecases.PromptSignals{Text: capturable, DispatchID: "d-1"}
		if c := classifySocraticaPrompt(e.deps, "p", "claude", sig); c.Capture {
			t.Fatal("recapturó el prompt aprobado", c)
		}
		// H2: la variable sigue en el entorno, pero la marca ya se consumió.
		if c := classifySocraticaPrompt(e.deps, "p", "claude", sig); !c.Capture {
			t.Fatal("un segundo prompt con la variable presente debe capturarse", c)
		}
		if c := classifySocraticaPrompt(e.deps, "p", "claude", usecases.PromptSignals{Text: capturable, DispatchID: "desconocido"}); !c.Capture {
			t.Fatal("un dispatch_id desconocido se evalúa normalmente", c)
		}
	})
	t.Run("capability gates interception", func(t *testing.T) {
		on := domain.SocraticaSettings{Interception: map[string]bool{"claude": true}}
		if c := classifySocraticaPrompt(newHookSocEnv(t, on).deps, "p", "claude", usecases.PromptSignals{Text: capturable}); c.Capture {
			t.Fatal("capturó sin capacidad ni opt-in sin verificar (M5)")
		}
		if c := classifySocraticaPrompt(newHookSocEnv(t, unverified("claude")).deps, "p", "claude", usecases.PromptSignals{Text: capturable}); !c.Capture {
			t.Fatal("con interception_unverified debe capturar")
		}
	})
	t.Run("all off touches nothing but the provenance read", func(t *testing.T) {
		e := newHookSocEnv(t, domain.SocraticaSettings{})
		c := classifySocraticaPrompt(e.deps, "p", "claude", usecases.PromptSignals{Text: capturable, DispatchID: "d-1"})
		// FR-026 exige saber si el prompt responde a un flujo pendiente: es la
		// única consulta permitida con la intercepción apagada.
		if c.Capture || c.Directive != "" || e.spy.consume != 0 || e.spy.active > 1 {
			t.Fatalf("%+v active=%d consume=%d", c, e.spy.active, e.spy.consume)
		}
		e.spy.active = 0
		if c = classifySocraticaPrompt(e.deps, "p", "claude", usecases.PromptSignals{Text: "/socratica algo"}); e.spy.active != 0 || !c.SkipProvenance {
			t.Fatal("la invocación explícita no debe abrir la base", e.spy.active)
		}
	})
	t.Run("a prompt that passes leaves no trace", func(t *testing.T) {
		e := newHookSocEnv(t, unverified("claude"))
		for _, p := range []string{capturable, "/plan algo", ">" + capturable, "ok"} {
			classifySocraticaPrompt(e.deps, "p", "claude", usecases.PromptSignals{Text: p})
		}
		if _, found, _ := e.store.ActiveFlow(context.Background(), "p", "claude"); found {
			t.Fatal("la intercepción creó un flujo: solo debe añadir la directiva (G6)")
		}
		if rows, _ := e.store.ListHistory(context.Background(), "p", "", 20); len(rows) != 0 {
			t.Fatal("la intercepción escribió historial", rows)
		}
	})
}

// fitPromptSectionsText une las secciones como lo hace el hook, sin presupuesto.
func fitPromptSectionsText(sections []domain.HookSection) string {
	var parts []string
	for _, s := range sections {
		parts = append(parts, s.Text)
	}
	return strings.Join(parts, "\n\n")
}

// BenchmarkUserPromptSubmit_Off y _On registran el coste de Socratica en el
// hook por turno (objetivo < 20 ms, M8; resultado en evidence/README.md).
func BenchmarkUserPromptSubmit_Off(b *testing.B) {
	e := newHookSocEnv(b, domain.SocraticaSettings{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifySocraticaPrompt(e.deps, "p", "claude", usecases.PromptSignals{Text: capturable})
	}
}

func BenchmarkUserPromptSubmit_On(b *testing.B) {
	e := newHookSocEnv(b, unverified("claude"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifySocraticaPrompt(e.deps, "p", "claude", usecases.PromptSignals{Text: capturable, DispatchID: "d-1"})
	}
}
