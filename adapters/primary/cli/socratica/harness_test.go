package socratica

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mem/adapters/primary/console"
	"mem/adapters/secondary/generator"
	"mem/application/ports"
	"mem/domain"
)

type namedCLIHarness struct {
	*cliGen
	id string
}

const openCodeFreeTierFailure = `opencode: exit status 1: {"type":"provider.auth","message":"OpenCode's free tier can only be used from within OpenCode","status":403,"response":{"body":"FreeTierError"}}`

type assistedLauncher struct {
	request ports.LaunchRequest
	calls   int
}

func (l *assistedLauncher) Launch(_ context.Context, r ports.LaunchRequest, onStarted func() error) error {
	l.calls++
	l.request = r
	return onStarted()
}

func TestSocraticaProviderFailureContinuesAssistedWithoutDispatch(t *testing.T) {
	e := newCLIEnv(t)
	e.gen.fail = errors.New(openCodeFreeTierFailure)
	e.u.Generator = generator.NewFirstAvailable([]ports.SocraticGenerator{namedCLIHarness{e.gen, "opencode"}})
	l := &assistedLauncher{}
	e.deps.SocraticaExec = l
	reader := bufio.NewReader(strings.NewReader("1\n")) // first action: continue inside the selected harness
	var out bytes.Buffer
	code := runSocratica(e.deps, []string{"refine", "--client", "opencode", "busca grafos visuales"}, socraticaIO{in: reader, out: &out, err: &out, tty: true, ui: console.NewPlain(reader, &out)})
	if code != 0 || l.calls != 1 || len(e.gen.seen) != 1 {
		t.Fatalf("code=%d launches=%d generations=%d\n%s", code, l.calls, len(e.gen.seen), out.String())
	}
	if strings.Contains(out.String(), `"provider.auth"`) || strings.Contains(out.String(), "Reintentar con opencode") || !strings.Contains(out.String(), "Continuar asistido en opencode") {
		t.Fatal(out.String())
	}
	if l.request.Client != "opencode" || !strings.Contains(l.request.Prompt, "busca grafos visuales") || !strings.Contains(l.request.Prompt, "propio modelo") || strings.Contains(l.request.Prompt, "FreeTierError") {
		t.Fatal(l.request)
	}
	f := e.flow("opencode")
	if f.Phase != domain.PhaseFailed || f.DispatchAck || f.DispatchID != "" || l.request.Env["GOMEMORY_SOCRATICA_DISPATCH"] != "" {
		t.Fatal("assisted opening authorized execution", f)
	}
}

func TestSocraticaProviderFailureCanChangeHarnessAndProduceProposal(t *testing.T) {
	e := newCLIEnv(t)
	bad := &cliGen{available: true, fail: errors.New(openCodeFreeTierFailure)}
	e.u.Generator = generator.NewFirstAvailable([]ports.SocraticGenerator{namedCLIHarness{e.gen, "claude"}, namedCLIHarness{bad, "opencode"}})
	reader := bufio.NewReader(strings.NewReader("2\n1\n1\n4\n")) // change, Claude, answer, cancel review
	var out bytes.Buffer
	code := runSocratica(e.deps, []string{"refine", "--client", "opencode", "busca grafos"}, socraticaIO{in: reader, out: &out, err: &out, tty: true, ui: console.NewPlain(reader, &out)})
	if code != 0 || len(e.gen.seen) != 2 {
		t.Fatalf("code=%d\n%s", code, out.String())
	}
	if e.flow("claude").OriginalPrompt != "busca grafos" || e.flow("opencode").Phase != domain.PhaseCancelled || !strings.Contains(out.String(), "── Propuesta ──") {
		t.Fatal(out.String())
	}
}

func (g namedCLIHarness) Name() string { return g.id }

func TestSocraticaHarnessPicker(t *testing.T) {
	for _, tc := range []struct {
		name, input, client string
		tty                 bool
		code                int
	}{
		{"select codex", "2\n1\n4\n", "codex", true, 0},
		{"explicit even with one option", "1\n1\n4\n", "claude", true, 0},
		{"cancel before creation", "3\n", "", true, 0},
		{"EOF before creation", "", "", true, 0},
		{"non TTY requires client", "1\n", "", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newCLIEnv(t)
			claude, codex := e.gen, &cliGen{available: true, questions: e.gen.questions, proposed: "Busca proyectos de grafos"}
			e.u.Generator = generator.NewFirstAvailable([]ports.SocraticGenerator{namedCLIHarness{claude, "claude"}, namedCLIHarness{codex, "codex"}})
			reader := bufio.NewReader(strings.NewReader(tc.input))
			var out, errs bytes.Buffer
			sio := socraticaIO{in: reader, out: &out, err: &errs, tty: tc.tty}
			if tc.tty {
				sio.ui = console.NewPlain(reader, &out)
			}
			if code := runSocratica(e.deps, []string{"refine", "busca grafos"}, sio); code != tc.code {
				t.Fatalf("code=%d out=%s err=%s", code, out.String(), errs.String())
			}
			var count int
			if err := e.db.QueryRow("SELECT COUNT(*) FROM socratica_flows").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if tc.client == "" {
				if count != 0 {
					t.Fatal("flow created without selection")
				}
				return
			}
			f := e.flow(tc.client)
			if f.Phase != domain.PhaseCancelled || f.Client != tc.client {
				t.Fatal(f)
			}
			if tc.client == "codex" && len(claude.seen) != 0 {
				t.Fatal("Claude generated for Codex")
			}
			if tc.tty && !strings.Contains(out.String(), "OpenCode: deshabilitado") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestSocraticaUnavailableSelectionDoesNotCreateFlow(t *testing.T) {
	e := newCLIEnv(t)
	reader := bufio.NewReader(strings.NewReader(""))
	var out bytes.Buffer
	code := runSocratica(e.deps, []string{"refine", "--client", "unknown", "texto"}, socraticaIO{in: reader, out: &out, err: &out, tty: true})
	if code != 1 {
		t.Fatal(code)
	}
	if pending, err := e.u.Pending(context.Background(), "p"); err != nil || len(pending) != 0 {
		t.Fatal(pending, err)
	}
}

func TestSocraticaPrintKeepsHumanPanelsOffStdout(t *testing.T) {
	e := newCLIEnv(t)
	e.gen.questions = nil
	reader := bufio.NewReader(strings.NewReader("1\n")) // execute from the review selector
	var out, human bytes.Buffer
	env := console.Env{StdinTTY: true, StdoutTTY: true, Width: 65, Getenv: func(key string) string {
		if key == "NO_COLOR" {
			return "1"
		}
		return ""
	}}
	sio := socraticaIO{in: reader, out: &out, err: &human, tty: true, env: &env, ui: console.NewPlain(reader, &human)}
	code := runSocratica(e.deps, []string{"refine", "--client", "claude", "--print", "--raw", "envía informe"}, sio)
	if code != 0 || out.String() != e.gen.proposed+"\n" {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, out.String(), human.String())
	}
	if !strings.Contains(human.String(), "Revisión") || !strings.Contains(human.String(), "Socratica") {
		t.Fatal(human.String())
	}
}
