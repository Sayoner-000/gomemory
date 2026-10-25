package tui

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

type socTUI struct {
	t        *testing.T
	m        model
	repo     *countingSocraticaRepo
	store    *persistence.SocraticaStore
	gen      *tuiGen
	launcher *tuiLauncher
	root     string
	db       *sql.DB
}

func newSocTUI(t *testing.T) *socTUI {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := persistence.NewSocraticaStore(db, time.Second)
	repo := &countingSocraticaRepo{inner: store}
	gen := &tuiGen{questions: []domain.Question{{Text: "¿Para quién?", Options: []string{"Equipo", "Cliente"}}}}
	l := &tuiLauncher{}
	flow := &usecases.SocraticaFlow{Repo: repo, UOW: store, Lock: persistence.NewSocraticaDispatchLock(root), Generator: gen, Clock: clock.SystemClock{}, NewID: func() string { return uuid.Must(uuid.NewV7()).String() }}
	m := newTestModel(nil, 40)
	m.width = 120
	m.root = root
	m.project = "p"
	m.settingsRepo = persistence.NewSettingsRepository()
	m.soc = newSocraticaState(SocraticaDeps{Flow: flow, Launcher: l})
	return &socTUI{t: t, m: m, repo: repo, store: store, gen: gen, launcher: l, root: root, db: db}
}

// press envía una tecla y ejecuta los tea.Cmd resultantes hasta agotarlos,
// salvo un tea.Exec, que devuelve sin ejecutar.
func (s *socTUI) press(keys ...string) tea.Cmd {
	s.t.Helper()
	var pending tea.Cmd
	for _, k := range keys {
		next, cmd := s.m.Update(keyMsg(k))
		s.m = next.(model)
		pending = s.drain(cmd)
	}
	return pending
}

func (s *socTUI) drain(cmd tea.Cmd) tea.Cmd {
	s.t.Helper()
	for i := 0; cmd != nil && i < 8; i++ {
		msg := cmd()
		if fmt.Sprintf("%T", msg) == "tea.execMsg" {
			return cmd
		}
		if _, ok := msg.(tea.BatchMsg); ok || msg == nil {
			return nil
		}
		next, c := s.m.Update(msg)
		s.m = next.(model)
		cmd = c
	}
	return nil
}

func (s *socTUI) typeText(text string) {
	for _, r := range text {
		next, _ := s.m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		s.m = next.(model)
	}
}

func (s *socTUI) view() string { return ansi.Strip(s.m.renderView()) }

// runExec ejecuta Ejecutar como lo haría tea.Exec y entrega el resultado.
func (s *socTUI) runExec() {
	s.t.Helper()
	c := s.m.socExecCommand(s.m.soc.current.Flow)
	err := c.Run()
	next, cmd := s.m.Update(socraticaExecDoneMsg{id: c.id, result: c.result, err: err})
	s.m = next.(model)
	s.drain(cmd)
}

// refine abre Socratica, escribe un prompt y lo envía.
func (s *socTUI) refine(prompt string) {
	s.t.Helper()
	s.press("p")
	s.press("r")
	s.press("enter") // explicit harness selection, even with a single option
	s.typeText(prompt)
	s.press("enter")
}

func TestSocraticaScreens(t *testing.T) {
	t.Run("acknowledgement before the generator", func(t *testing.T) {
		s := newSocTUI(t)
		s.press("p", "r", "enter")
		s.typeText("envía el informe")
		next, cmd := s.m.Update(keyMsg("enter"))
		s.m = next.(model)
		if !strings.Contains(s.view(), "Generando preguntas con claude…") || cmd == nil || s.gen.calls.Load() != 0 {
			t.Fatalf("el acuse debe pintarse antes del tea.Cmd: calls=%d\n%s", s.gen.calls.Load(), s.view())
		}
		s.drain(cmd)
		if s.gen.calls.Load() != 1 || s.m.screen != screenSocraticaQuestions {
			t.Fatal(s.gen.calls.Load(), s.m.screen)
		}
	})
	t.Run("questions frame, counter and check", func(t *testing.T) {
		s := newSocTUI(t)
		s.gen.questions = append(s.gen.questions, domain.Question{Text: "¿Formato?", Options: []string{"PDF", "Correo"}})
		s.refine("envía el informe")
		v := s.view()
		if !strings.Contains(v, "0/2 respondidas") || !strings.Contains(v, "┏") || !strings.Contains(v, "1 Equipo") {
			t.Fatal(v)
		}
		s.press("2")
		if v = s.view(); !strings.Contains(v, "1/2 respondidas") || !strings.Contains(v, "✓ 1. ¿Para quién?") {
			t.Fatal(v)
		}
		s.press("S")
		if s.m.screen != screenSocraticaDetail || !strings.Contains(s.view(), "Envía el informe al cliente") {
			t.Fatal(s.view())
		}
	})
	t.Run("review actions, layouts and execute", func(t *testing.T) {
		s := newSocTUI(t)
		s.gen.questions = nil
		s.refine("envía el informe")
		v := s.view()
		if !strings.Contains(v, "e Ejecutar · d Editar · c Copiar · x Cancelar") {
			t.Fatal(v)
		}
		lines := strings.Split(v, "\n")
		side := false
		for _, l := range lines {
			if strings.Contains(l, "Original") && strings.Contains(l, "Propuesta") {
				side = true
			}
		}
		if !side {
			t.Fatalf("a 120 columnas las tarjetas van lado a lado:\n%s", v)
		}
		s.m.width = 80
		for _, l := range strings.Split(s.view(), "\n") {
			if strings.Contains(l, "Original") && strings.Contains(l, "Propuesta") {
				t.Fatalf("a 80 columnas se apilan:\n%s", s.view())
			}
		}
		s.m.width = 59
		if v = s.view(); !strings.Contains(v, "Ventana demasiado estrecha: original conservado") || !strings.Contains(v, "envía el informe") {
			t.Fatal(v)
		}
		for _, w := range []int{59, 80, 120} {
			s.m.width = w
			for _, l := range strings.Split(s.m.renderView(), "\n") {
				if ansi.StringWidth(l) > w {
					t.Errorf("ancho %d desbordado: %q", w, l)
				}
			}
		}
		s.m.width = 120
		cmd := s.press("e")
		if cmd == nil || fmt.Sprintf("%T", cmd()) != "tea.execMsg" {
			t.Fatal("Ejecutar debe ceder la terminal con tea.Exec")
		}
		s.runExec()
		if s.launcher.calls.Load() != 1 || s.m.soc.current.Flow.Phase != domain.PhaseDispatched || !strings.Contains(s.view(), "Ya enviado") {
			t.Fatal(s.launcher.calls.Load(), s.view())
		}
		// Reenviar exige elegirlo explícitamente (I10).
		s.press("r")
		if !strings.Contains(s.view(), "y confirmar reenvío") || s.m.soc.current.Flow.Phase != domain.PhaseDispatched {
			t.Fatal(s.view())
		}
		s.press("n")
		if s.m.soc.current.Flow.Phase != domain.PhaseDispatched {
			t.Fatal("reenvió sin confirmación")
		}
	})
	t.Run("uncertain and in-flight states", func(t *testing.T) {
		s := newSocTUI(t)
		s.gen.questions = nil
		s.refine("envía el informe")
		f := s.m.soc.current.Flow
		ctx := context.Background()
		r, err := s.m.soc.deps.Flow.Apply(ctx, f.ID, domain.Event{Kind: "approve"}, usecases.OriginLocal)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.store.ClaimDispatch(ctx, f.ID, r.Flow.DispatchID, "other", time.Now()); err != nil {
			t.Fatal(err)
		}
		s.press("esc", "enter")
		if v := s.view(); !strings.Contains(v, "Puede que ya se haya enviado") || !strings.Contains(v, "m Marcar enviado · r Reenviar · x Cancelar") || strings.Contains(v, "Ejecutar") {
			t.Fatal(v)
		}
		release, ok, err := persistence.NewSocraticaDispatchLock(s.root).TryAcquire(f.ID)
		if err != nil || !ok {
			t.Fatal(ok, err)
		}
		defer release()
		s.press("esc", "enter")
		if v := s.view(); !strings.Contains(v, "Envío en curso") || strings.Contains(v, "Ejecutar") || strings.Contains(v, "Reenviar") {
			t.Fatal(v)
		}
	})
	t.Run("scrutiny shows the synthesis", func(t *testing.T) {
		s := newSocTUI(t)
		s.gen.questions = nil
		s.gen.scrutiny = &domain.ScrutinySynthesis{Restatement: "Migrar a X", StrongestObjection: "Coste", Assumptions: []domain.SocraticaAssumption{{Text: "carga baja", Kind: "hypothesis"}}}
		ctx := context.Background()
		r, err := s.m.soc.deps.Flow.Start(ctx, "p", "claude", domain.ModeScrutiny, "migremos a X")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.m.soc.deps.Flow.Resume(ctx, r.Flow.ID, usecases.OriginLocal); err != nil {
			t.Fatal(err)
		}
		s.press("p", "enter")
		if v := s.view(); !strings.Contains(v, "Síntesis") || !strings.Contains(v, "[hipótesis] carga baja") || !strings.Contains(v, "Objeción más fuerte: Coste") {
			t.Fatal(v)
		}
	})
	t.Run("generator failure", func(t *testing.T) {
		s := newSocTUI(t)
		s.gen.fail = errors.New("timeout")
		s.refine("envía  el informe")
		if v := s.view(); s.m.screen != screenSocraticaDetail || !strings.Contains(v, "Continuar asistido") || !strings.Contains(v, "Cambiar harness") || !strings.Contains(v, "envía  el informe") {
			t.Fatal(v)
		}
	})
	t.Run("history detail survives the purge", func(t *testing.T) {
		s := newSocTUI(t)
		s.gen.questions = nil
		s.refine("envía el informe")
		id := s.m.soc.current.Flow.ID
		s.press("x")
		if err := s.store.DeleteFlow(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		s.press("esc", "p", "enter")
		if v := s.view(); !strings.Contains(v, "envía el informe") || !strings.Contains(v, "Envía el informe al cliente") {
			t.Fatal(v)
		}
	})
	t.Run("list, interception and private", func(t *testing.T) {
		s := newSocTUI(t)
		s.gen.questions = nil
		ctx := context.Background()
		r, err := s.m.soc.deps.Flow.Start(ctx, "p", "codex", domain.ModeRefine, "pendiente de codex")
		if err != nil {
			t.Fatal(err)
		}
		_ = r
		s.press("p")
		v := s.view()
		if !strings.Contains(v, "SOCRATICA") || !strings.Contains(v, "Tu prompt │ Preguntas │ Reescritura │ Revisión") || !strings.Contains(v, "codex · captured · hace ahora") {
			t.Fatal(v)
		}
		s.press("i")
		if s.m.settingsRepo.Read(s.root).Interception["codex"] || !strings.Contains(s.view(), "no soportada en codex") {
			t.Fatal("activó la intercepción sin capacidad", s.view())
		}
		if !strings.Contains(s.m.statusMsg, "mem socratica on --client codex --unverified") {
			t.Fatalf("el rechazo no indica cómo probarla: %q", s.m.statusMsg)
		}
		s.press("p")
		if !s.m.settingsRepo.Read(s.root).Private || !s.m.soc.deps.Flow.Settings.Private {
			t.Fatal("p no alternó el modo privado")
		}
		long := strings.Repeat("palabra ", 40)
		if _, err := s.m.soc.deps.Flow.Start(ctx, "p", "opencode", domain.ModeRefine, long); err != nil {
			t.Fatal(err)
		}
		s.press("esc", "p")
		for _, l := range strings.Split(s.m.renderView(), "\n") {
			if ansi.StringWidth(l) > s.m.width {
				t.Fatalf("fila sin elipsis: %q", l)
			}
		}
		if !strings.Contains(s.view(), "…") {
			t.Fatal("el original largo debe acotarse con elipsis")
		}
	})
	t.Run("no Reduce in the TUI", func(t *testing.T) {
		for _, f := range []string{"socratica.go", "tui.go"} {
			data, err := os.ReadFile(filepath.Join(".", f))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "domain.Reduce") {
				t.Fatalf("%s llama a domain.Reduce (K1)", f)
			}
		}
	})
}

// TestSocraticaView_Pura (T049): View() solo lee el modelo.
func TestSocraticaView_Pura(t *testing.T) {
	s := newSocTUI(t)
	s.gen.questions = nil
	s.refine("envía el informe")
	for _, sc := range []screen{screenSocratica, screenSocraticaDetail, screenSocraticaQuestions} {
		s.m.screen = sc
		repoBefore, genBefore := s.repo.calls.Load(), s.gen.calls.Load()
		first := s.m.renderView()
		for i := 0; i < 100; i++ {
			_ = s.m.View()
			if got := s.m.renderView(); got != first {
				t.Fatalf("pantalla %d: View no es determinista", sc)
			}
		}
		if s.repo.calls.Load() != repoBefore || s.gen.calls.Load() != genBefore || s.launcher.calls.Load() != 0 {
			t.Fatalf("pantalla %d: View tocó repositorio, generador o lanzador", sc)
		}
	}
}
