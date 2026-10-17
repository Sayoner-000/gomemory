package cli

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/launcher"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// cliGen is a generator double that records what the person had already seen
// when it was called (FR-017).
type cliGen struct {
	name      string
	mu        sync.Mutex
	out       *syncBuffer
	seen      []string
	questions []domain.Question
	proposed  string
	fail      error
	available bool
}

func (g *cliGen) Name() string {
	if g.name != "" {
		return g.name
	}
	return "claude"
}
func (g *cliGen) Available() bool { return g.available }
func (g *cliGen) record() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.out != nil {
		g.seen = append(g.seen, g.out.String())
	}
}
func (g *cliGen) GenerateQuestions(context.Context, ports.SocraticGenerationRequest) (domain.GenerationResult, error) {
	g.record()
	if g.fail != nil {
		return domain.GenerationResult{}, g.fail
	}
	return domain.GenerationResult{Questions: g.questions}, nil
}
func (g *cliGen) Rewrite(context.Context, ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	g.record()
	return domain.RewriteResult{Proposed: g.proposed, Additions: []string{"destinatario"}}, nil
}

// syncBuffer lets a goroutine read what another one is writing.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type countingLauncher struct{ calls int }

func (l *countingLauncher) Launch(context.Context, ports.LaunchRequest, func() error) error {
	l.calls++
	return nil
}

type cliEnv struct {
	t     *testing.T
	root  string
	db    *sql.DB
	repo  *persistence.SocraticaStore
	u     *usecases.SocraticaFlow
	gen   *cliGen
	deps  *Deps
	out   *syncBuffer
	agent string // directorio del PATH con agentes falsos
}

func newCLIEnv(t *testing.T) *cliEnv {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := persistence.NewSocraticaStore(db, time.Second)
	out := &syncBuffer{}
	gen := &cliGen{out: out, available: true, proposed: "Envía el informe al cliente antes de [fecha]", questions: []domain.Question{{Text: "¿Para quién?", Options: []string{"Equipo", "Cliente"}}}}
	u := &usecases.SocraticaFlow{Repo: repo, UOW: repo, Lock: persistence.NewSocraticaDispatchLock(root), Generator: gen, Clock: clock.SystemClock{}, NewID: func() string { return uuid.Must(uuid.NewV7()).String() }}
	agentDir := t.TempDir()
	t.Setenv("PATH", agentDir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	deps := &Deps{Root: root, Project: "p", Socratica: u, SettingsRepo: persistence.NewSettingsRepository(),
		SocraticaExec:  launcher.NewExec(strings.NewReader(""), io.Discard, io.Discard),
		SocraticaPrint: func(w io.Writer, raw bool) ports.AgentLauncher { return launcher.NewPrint(w, raw) }}
	return &cliEnv{t: t, root: root, db: db, repo: repo, u: u, gen: gen, deps: deps, out: out, agent: agentDir}
}

func (e *cliEnv) run(tty bool, input string, args ...string) (int, string, string) {
	e.t.Helper()
	// Existing flow tests specify the harness; picker behavior is tested below
	// through runSocratica with its actual input sequence.
	if len(args) > 0 && args[0] == "refine" {
		explicit := false
		for _, arg := range args {
			if arg == "--client" || strings.HasPrefix(arg, "--client=") {
				explicit = true
			}
		}
		if !explicit {
			args = append([]string{"refine", "--client", "claude"}, args[1:]...)
		}
	}
	before := len(e.out.String())
	var errOut bytes.Buffer
	code := runSocratica(e.deps, args, socraticaIO{in: bufio.NewReader(strings.NewReader(input)), out: e.out, err: &errOut, tty: tty})
	return code, e.out.String()[before:], errOut.String()
}

// onlyFlow returns the single flow of a client, pending or not.
func (e *cliEnv) flow(client string) domain.RefinementFlow {
	e.t.Helper()
	var id string
	if err := e.db.QueryRow(`SELECT id FROM socratica_flows WHERE client=? ORDER BY created_at DESC LIMIT 1`, client).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	f, err := e.repo.GetFlow(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return f
}

// fakeAgent waits for <dir>/release, after writing <dir>/started.
func (e *cliEnv) fakeAgent(name string) {
	e.t.Helper()
	if runtime.GOOS == "windows" {
		e.t.Skip("agentes falsos en shell")
	}
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' \"$GOMEMORY_SOCRATICA_DISPATCH\" > %q\ni=0; while [ ! -f %q ] && [ $i -lt 200 ]; do sleep 0.05; i=$((i+1)); done\n",
		filepath.Join(e.agent, "started"), filepath.Join(e.agent, "release"))
	if err := os.WriteFile(filepath.Join(e.agent, name), []byte(script), 0o755); err != nil {
		e.t.Fatal(err)
	}
}
func (e *cliEnv) waitFile(name string) string {
	e.t.Helper()
	for i := 0; i < 200; i++ {
		if data, err := os.ReadFile(filepath.Join(e.agent, name)); err == nil {
			return string(data)
		}
		time.Sleep(25 * time.Millisecond)
	}
	e.t.Fatalf("%s no apareció", name)
	return ""
}

func TestCmdSocratica(t *testing.T) {
	t.Run("acknowledges before generating", func(t *testing.T) {
		e := newCLIEnv(t)
		e.run(true, "1\nx\n", "refine", "envía el informe")
		if len(e.gen.seen) == 0 || !strings.Contains(e.gen.seen[0], "Generando preguntas con claude…") {
			t.Fatalf("el acuse no precedió a la generación: %q", e.gen.seen)
		}
	})
	t.Run("print emits once", func(t *testing.T) {
		e := newCLIEnv(t)
		code, out, _ := e.run(true, "1\ne\n", "refine", "--print", "envía el informe")
		if code != 0 || strings.Count(out, ">Envía el informe al cliente antes de [fecha]\n") != 1 {
			t.Fatalf("code=%d\n%s", code, out)
		}
		f := e.flow("claude")
		if f.Phase != domain.PhaseDispatched {
			t.Fatal(f.Phase)
		}
		code, out, _ = e.run(true, "", "pending", "--resume", f.ID)
		if code != 0 || !strings.Contains(out, "Ya enviado ("+f.DispatchID+")") || strings.Contains(out, ">Envía") {
			t.Fatalf("segunda ejecución: code=%d\n%s", code, out)
		}
	})
	t.Run("print raw", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		_, out, _ := e.run(true, "e\n", "refine", "--print", "--raw", "envía el informe")
		if !strings.Contains(out, "\nEnvía el informe al cliente antes de [fecha]\n") || strings.Contains(out, ">Envía") {
			t.Fatal(out)
		}
	})
	t.Run("edit then execute", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		_, out, _ := e.run(true, "d\nInforme trimestral\npara el cliente\n.\ne\n", "refine", "--print", "envía el informe")
		if !strings.Contains(out, ">Informe trimestral\npara el cliente\n") {
			t.Fatal(out)
		}
	})
	t.Run("cancel repeats the original", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		_, out, _ := e.run(true, "x\n", "refine", "  envía  el informe ")
		if !strings.HasSuffix(out, "Tu prompt original:\n  envía  el informe \n") {
			t.Fatalf("%q", out)
		}
	})
	t.Run("no generator", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.available = false
		code, out, errOut := e.run(true, "", "refine", "envía el informe")
		if code != 2 || out != "envía el informe\n" || errOut == "" {
			t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
		}
	})
	t.Run("without TTY never executes", func(t *testing.T) {
		e := newCLIEnv(t)
		l := &countingLauncher{}
		e.deps.SocraticaExec = l
		code, out, _ := e.run(false, "e\n", "refine", "envía el informe")
		if code != 0 || l.calls != 0 || !strings.Contains(out, "── Propuesta ──") {
			t.Fatalf("code=%d calls=%d\n%s", code, l.calls, out)
		}
		if f := e.flow("claude"); f.Phase != domain.PhaseReview || len(f.Answers) != 1 || !f.Answers[0].Skipped {
			t.Fatalf("sin TTY la pregunta debe quedar omitida y el flujo en revisión: %+v", f)
		}
	})
	t.Run("execute with a fake agent", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.name = "codex"
		e.gen.questions = nil
		e.fakeAgent("codex")
		done := make(chan int, 1)
		go func() {
			code, _, _ := e.run(true, "e\n", "refine", "--client", "codex", "envía el informe")
			done <- code
		}()
		dispatch := e.waitFile("started")
		// El agente sigue en ejecución y el ack ya está escrito.
		f := e.flow("codex")
		if !f.DispatchAck || f.DispatchID != dispatch || f.Client != "codex" {
			t.Errorf("ack ausente mientras el agente corre: %+v (env=%q)", f, dispatch)
		}
		select {
		case <-done:
			t.Fatal("la CLI no esperó a que el agente terminara")
		case <-time.After(100 * time.Millisecond):
		}
		if err := os.WriteFile(filepath.Join(e.agent, "release"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if code := <-done; code != 0 {
			t.Fatal(code)
		}
	})
	t.Run("second process sees dispatch in flight", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		e.fakeAgent("claude")
		done := make(chan struct{})
		go func() { e.run(true, "e\n", "refine", "--client", "claude", "envía el informe"); close(done) }()
		e.waitFile("started")
		f := e.flow("claude")
		// Reenviar mientras el primer proceso retiene el lock del flujo.
		code, out, _ := e.run(true, "r\nreenviar\n", "history", "--show", f.ID)
		if code == 0 || !strings.Contains(out, "Envío en curso") {
			t.Fatalf("code=%d\n%s", code, out)
		}
		if _, out, _ = e.run(true, "", "pending", "--resume", f.ID); !strings.Contains(out, "Ya enviado") {
			t.Fatal(out)
		}
		_ = os.WriteFile(filepath.Join(e.agent, "release"), nil, 0o600)
		<-done
		if after := e.flow("claude"); after.DispatchID != f.DispatchID || after.Phase != domain.PhaseDispatched {
			t.Fatal("el segundo proceso cambió el flujo", after)
		}
	})
	t.Run("launch failure leaves a recoverable review", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		if err := os.WriteFile(filepath.Join(e.agent, "claude"), []byte("#!/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, errOut := e.run(true, "e\n", "refine", "--client", "claude", "envía el informe")
		f := e.flow("claude")
		if code != 1 || f.DispatchAck || f.DispatchClaimOwner != "" || !strings.Contains(errOut, "No se pudo arrancar claude") {
			t.Fatalf("code=%d %+v %s", code, f, errOut)
		}
		_, out, _ := e.run(true, "", "pending", "--resume", f.ID)
		if !strings.Contains(out, "── Propuesta ──") || !strings.Contains(out, "[e]jecutar") {
			t.Fatal(out)
		}
	})
	t.Run("ack failure is uncertain and never relaunched", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		e.repo = persistence.NewSocraticaStore(e.db, 100*time.Millisecond)
		e.u.Repo, e.u.UOW = e.repo, e.repo
		locking := &lockingLauncher{db: e.db}
		e.deps.SocraticaExec = locking
		code, _, errOut := e.run(true, "e\n", "refine", "--client", "claude", "envía el informe")
		locking.release()
		if code != 1 || !strings.Contains(errOut, "el agente arrancó pero no se pudo confirmar el envío; no reintentes") {
			t.Fatalf("code=%d %s", code, errOut)
		}
		f := e.flow("claude")
		l := &countingLauncher{}
		e.deps.SocraticaExec = l
		_, out, _ := e.run(true, "", "pending", "--resume", f.ID)
		if !strings.Contains(out, "[m]arcar enviado [r]eenviar [x]cancelar") || l.calls != 0 {
			t.Fatalf("calls=%d\n%s", l.calls, out)
		}
	})
	t.Run("history show on a dispatched flow", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		e.run(true, "e\n", "refine", "--print", "envía el informe")
		f := e.flow("claude")
		_, out, _ := e.run(false, "", "history", "--show", f.ID)
		if !strings.Contains(out, "Ya enviado") || strings.Contains(out, "[r]eenviar") {
			t.Fatalf("sin TTY no se ofrece reenviar:\n%s", out)
		}
		_, out, _ = e.run(true, "r\nno\n", "history", "--show", f.ID)
		if !strings.Contains(out, "[r]eenviar") || !strings.Contains(out, "No se reenvió nada") {
			t.Fatal(out)
		}
		if again := e.flow("claude"); again.DispatchID != f.DispatchID {
			t.Fatal("reenvió sin confirmación explícita")
		}
	})
	t.Run("generator failure keeps the original", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.fail = fmt.Errorf("claude: %w (1s)", domain.ErrGenerationTimeout)
		code, out, _ := e.run(true, "o\n", "refine", "--print", "envía  el informe\n")
		if code != 0 || !strings.Contains(out, "Tu prompt original queda intacto:\nenvía  el informe\n") || !strings.Contains(out, "[r]eintentar [o]riginal [x]cancelar") || !strings.Contains(out, ">envía  el informe\n") {
			t.Fatalf("code=%d\n%s", code, out)
		}
		if f := e.flow("claude"); f.ApprovedPrompt != "envía  el informe\n" || f.Phase != domain.PhaseDispatched {
			t.Fatalf("%+v", f)
		}
	})
	t.Run("history, toggles, pending and private", func(t *testing.T) {
		e := newCLIEnv(t)
		e.gen.questions = nil
		for i := 0; i < 22; i++ {
			e.run(true, "x\n", "refine", fmt.Sprintf("prompt %d", i))
		}
		if err := e.u.PurgeStale(context.Background(), "p"); err != nil {
			t.Fatal(err)
		}
		_, out, _ := e.run(false, "", "history")
		if n := strings.Count(out, "\n"); n != 20 {
			t.Fatalf("history listó %d registros:\n%s", n, out)
		}
		last := e.flow("claude")
		// El flujo cancelado ya no existe (purga): el historial sigue sirviéndolo (I6).
		if _, err := e.db.Exec(`DELETE FROM socratica_flows WHERE id=?`, last.ID); err != nil {
			t.Fatal(err)
		}
		if _, out, _ = e.run(false, "", "history", "--show", last.ID); !strings.Contains(out, "prompt 21") || !strings.Contains(out, "Envía el informe") {
			t.Fatal(out)
		}

		if code, _, errOut := e.run(false, "", "on", "--client", "codex"); code != 1 || e.deps.SettingsRepo.Read(e.root).Interception["codex"] {
			t.Fatal("activó una intercepción sin capacidad", code)
		} else if !strings.Contains(errOut, "mem socratica on --client codex --unverified") {
			t.Fatalf("el rechazo no indica cómo probarla: %q", errOut)
		}
		if code, out, _ := e.run(false, "", "on", "--client", "codex", "--unverified"); code != 0 || !strings.Contains(out, "sin verificar") {
			t.Fatal(code, out)
		}
		_, out, _ = e.run(false, "", "status")
		if !strings.Contains(out, "codex     intercepción: sin verificar") || !strings.Contains(out, "claude    intercepción: desactivada") {
			t.Fatal(out)
		}
		e.run(false, "", "off", "--client", "claude")
		if s := e.deps.SettingsRepo.Read(e.root); !s.Interception["codex"] || s.Interception["claude"] {
			t.Fatal("on/off de un agente alteró a otro", s.Interception)
		}

		e.gen.name = "codex"
		e.run(false, "", "refine", "--client", "codex", "pendiente")
		pending := e.flow("codex")
		if _, out, _ = e.run(false, "", "pending"); !strings.Contains(out, pending.ID) {
			t.Fatal(out)
		}
		if _, out, _ = e.run(false, "", "pending", "--cancel", pending.ID); !strings.HasSuffix(out, "pendiente\n") {
			t.Fatal(out)
		}

		e.run(false, "", "private", "on")
		before, _ := e.u.History(context.Background(), "p", "opencode")
		e.gen.name = "opencode"
		e.run(true, "x\n", "refine", "--client", "opencode", "secreto")
		after, _ := e.u.History(context.Background(), "p", "opencode")
		if len(after) != len(before) {
			t.Fatal("el modo privado escribió historial")
		}
	})
	t.Run("storage unavailable and invalid settings", func(t *testing.T) {
		var errOut bytes.Buffer
		if code := runSocratica(&Deps{}, []string{"status"}, socraticaIO{in: bufio.NewReader(strings.NewReader("")), out: io.Discard, err: &errOut}); code != 3 {
			t.Fatal(code)
		}
		e := newCLIEnv(t)
		zero := 0
		e.u.Settings.HistoryLimit = &zero
		if code, _, errOut := e.run(false, "", "status"); code != 1 || !strings.Contains(errOut, "socratica_history_limit") {
			t.Fatal(code, errOut)
		}
	})
}

// lockingLauncher starts "the agent", then holds a write transaction so that
// both the ack in onStarted and its retry after Wait fail.
type lockingLauncher struct {
	db *sql.DB
	tx *sql.Tx
}

func (l *lockingLauncher) Launch(ctx context.Context, _ ports.LaunchRequest, onStarted func() error) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE socratica_flows SET revision=revision`); err != nil {
		return err
	}
	l.tx = tx
	if err := onStarted(); err != nil {
		return errors.Join(ports.ErrConfirmFailed, err)
	}
	return nil
}
func (l *lockingLauncher) release() {
	if l.tx != nil {
		_ = l.tx.Rollback()
	}
}
