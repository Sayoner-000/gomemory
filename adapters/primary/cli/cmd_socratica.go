package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"

	"mem/adapters/primary/console"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// socraticaPromptMax acota el prompt leído por stdin con `refine -`.
const socraticaPromptMax = 1 << 20

// Exit codes of `mem socratica` (contracts/cli.md).
const (
	socraticaExitOK          = 0
	socraticaExitUsage       = 1
	socraticaExitNoGenerator = 2
	socraticaExitStorage     = 3
)

// socraticaIO is the terminal of one invocation; tests inject it. Without a
// TTY nothing is ever executed (FR-032a).
type socraticaIO struct {
	in       *bufio.Reader
	out, err io.Writer
	tty      bool
	// outScreen/errScreen: out/err is a terminal. Only then is text painted,
	// and stripped of control sequences; a pipe gets the exact bytes, since
	// the original or the approved prompt may be the deliverable (FR-016).
	outScreen, errScreen bool
	ui                   console.UI
	env                  *console.Env
	richIn               io.Reader // retain the terminal fd so Bubble Tea can enable raw mode
}

func (s socraticaIO) printf(format string, args ...any) {
	_, _ = fmt.Fprint(s.out, domain.ProtectText(fmt.Sprintf(format, displayArgs(s.outScreen, args)...)))
}
func (s socraticaIO) errorf(format string, args ...any) {
	_, _ = fmt.Fprint(s.err, domain.ProtectText(fmt.Sprintf(format, displayArgs(s.errScreen, args)...)))
}

// rendered marks text this command already styled (panels), which printf
// passes through. Every other string or error argument is stored, generated or
// provider text and loses its control sequences before reaching the terminal
// (CWE-150); a panel left unmarked only loses its style.
type rendered string

func displayArgs(screen bool, args []any) []any {
	if !screen {
		return args
	}
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case rendered:
			out[i] = string(v)
		case string:
			out[i] = domain.StripTerminalControls(v)
		case error:
			out[i] = domain.StripTerminalControls(v.Error())
		default:
			out[i] = a
		}
	}
	return out
}

// readLine returns the next line without its line break; io.EOF ends input.
func (s socraticaIO) readLine() (string, error) {
	line, err := s.in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// CmdSocratica implements `mem socratica` (feature 039, US4).
func CmdSocratica(deps *Deps, args []string) {
	tty := term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
	sio := socraticaIO{in: bufio.NewReader(os.Stdin), out: os.Stdout, err: os.Stderr, tty: tty, richIn: os.Stdin,
		outScreen: term.IsTerminal(os.Stdout.Fd()), errScreen: term.IsTerminal(os.Stderr.Fd())}
	env := console.DetectEnv()
	sio.env = &env
	switch console.DetectMode(env, false) {
	case console.ModeRich:
		sio.ui = console.NewRich(os.Stdin, sio.out)
	case console.ModePlain:
		sio.ui = console.NewPlain(sio.in, sio.out)
	}
	if code := runSocratica(deps, args, sio); code != socraticaExitOK {
		exitProcess(code)
	}
}

func runSocratica(deps *Deps, args []string, sio socraticaIO) int {
	if deps.Socratica == nil {
		sio.errorf("Socratica no está disponible: el proyecto no tiene memoria inicializada (mem init).\n")
		return socraticaExitStorage
	}
	// Invalid settings fail before touching the database or a generator.
	if _, err := deps.Socratica.ValidatedSettings(); err != nil {
		sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	if len(args) == 0 {
		sio.errorf("Uso: mem socratica refine|history|on|off|status|pending|private\n")
		return socraticaExitUsage
	}
	c := &socraticaCmd{deps: deps, u: deps.Socratica, sio: sio, ctx: context.Background()}
	// Reuse the report panels for short human-facing actions. Streaming flows
	// and machine output keep their own writers and terminal ownership.
	if sio.tty && sio.env != nil && args[0] != "refine" {
		report := true
		for _, a := range args[1:] {
			if a == "--json" || strings.HasPrefix(a, "--json=") || a == "--resume" || a == "--cancel" || a == "--show" || strings.HasPrefix(a, "--resume=") || strings.HasPrefix(a, "--cancel=") || strings.HasPrefix(a, "--show=") {
				report = false
			}
		}
		if report {
			var body bytes.Buffer
			c.sio.out = &body
			defer func() {
				layout := console.NewLayout(*sio.env)
				// body was written through printf, so its content is already safe.
				sio.printf("%s\n", rendered(layout.Panel("Socratica · "+domain.StripTerminalControls(args[0]), "", "", strings.Split(strings.TrimSpace(body.String()), "\n"))))
			}()
		}
	}
	switch args[0] {
	case "refine":
		return c.refine(args[1:])
	case "history":
		return c.history(args[1:])
	case "on", "off":
		return c.toggle(args[0] == "on", args[1:])
	case "status":
		return c.status()
	case "pending":
		return c.pending(args[1:])
	case "private":
		return c.private(args[1:])
	}
	sio.errorf("Error: subcomando desconocido %q\n", args[0])
	return socraticaExitUsage
}

type socraticaCmd struct {
	deps *Deps
	u    *usecases.SocraticaFlow
	sio  socraticaIO
	ctx  context.Context
	// launcher is ExecLauncher, or PrintLauncher with --print.
	launcher  ports.AgentLauncher
	printOnly bool
}

func (c *socraticaCmd) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("mem socratica "+name, flag.ContinueOnError)
	fs.SetOutput(c.sio.err)
	return fs
}

func (c *socraticaCmd) refine(args []string) int {
	fs := c.flags("refine")
	mode := fs.String("mode", "refine", "refine | scrutiny")
	client := fs.String("client", "", "harness para refinar y ejecutar: claude | codex | opencode")
	printOnly := fs.Bool("print", false, "imprime el prompt aprobado en vez de lanzar el agente")
	raw := fs.Bool("raw", false, "con --print, sin el prefijo >")
	if fs.Parse(args) != nil {
		return socraticaExitUsage
	}
	c.printOnly = *printOnly
	if *mode != string(domain.ModeRefine) && *mode != string(domain.ModeScrutiny) {
		c.sio.errorf("Error: --mode debe ser refine o scrutiny\n")
		return socraticaExitUsage
	}
	promptOut := c.sio.out
	if c.sio.outScreen {
		// On a terminal the printed prompt is painted, not piped.
		promptOut = terminalSafe{promptOut}
	}
	if *printOnly && c.sio.env != nil {
		// Keep stdout exclusively for the approved prompt.
		c.sio.out, c.sio.outScreen = c.sio.err, c.sio.errScreen
		if c.sio.ui != nil {
			if console.DetectMode(*c.sio.env, false) == console.ModeRich {
				in := c.sio.richIn
				if in == nil {
					in = c.sio.in
				}
				c.sio.ui = console.NewRich(in, c.sio.out)
			} else {
				c.sio.ui = console.NewPlain(c.sio.in, c.sio.out)
			}
		}
	}
	text, err := c.readPrompt(fs.Args())
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	target := *client
	if target == "" {
		if !c.sio.tty {
			c.sio.errorf("Sin terminal interactiva, elige el harness con --client claude|codex|opencode. Tu prompt queda intacto.\n")
			c.sio.printf("%s\n", text)
			return socraticaExitNoGenerator
		}
		target, err = c.chooseHarness()
		if errors.Is(err, console.ErrCanceled) {
			c.sio.printf("Cancelado. Tu prompt original:\n%s\n", text)
			return socraticaExitOK
		}
		if err != nil {
			c.sio.errorf("%v\n", err)
			c.sio.printf("%s\n", text)
			return socraticaExitNoGenerator
		}
	}
	if !domain.IsSocraticaClient(target) || (*mode != string(domain.ModeRefine) && *mode != string(domain.ModeScrutiny)) {
		c.sio.errorf("Error: --client debe ser claude, codex u opencode y --mode refine o scrutiny\n")
		return socraticaExitUsage
	}
	if !c.u.HarnessAvailable(target) {
		c.sio.errorf("%s no está disponible para refinar; revisa mem socratica status. Tu prompt queda intacto:\n", target)
		c.sio.printf("%s\n", text)
		return socraticaExitNoGenerator
	}
	c.launcher = c.deps.SocraticaExec
	if *printOnly {
		c.launcher = c.deps.SocraticaPrint(promptOut, *raw)
	}
	// Acknowledge immediately, before the generator runs (FR-017).
	c.sio.printf("Generando preguntas con %s…\n", target)
	r, err := c.u.Start(c.ctx, c.deps.Project, target, domain.SocraticaMode(*mode), text)
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	if r.Flow.Recovered {
		c.sio.printf("Retomando el flujo pendiente %s de %s.\n", r.Flow.ID, r.Flow.Client)
	}
	return c.drive(r.Flow.ID, true)
}

func (c *socraticaCmd) chooseHarness(exclude ...string) (string, error) {
	var opts []console.Option
	for _, h := range c.u.Harnesses() {
		skipped := false
		for _, id := range exclude {
			if h.ID == id {
				skipped = true
			}
		}
		if skipped {
			continue
		}
		if !h.Available {
			c.sio.printf("  − %s: %s\n", h.Label, h.Reason)
			continue
		}
		opts = append(opts, console.Option{Value: h.ID, Label: h.Label, Hint: "preguntas, reescritura y ejecución"})
	}
	if len(opts) == 0 {
		return "", ports.ErrNoGenerator
	}
	opts = append(opts, console.Option{Value: "cancel", Label: "Cancelar", Hint: "conservar el original"})
	ui := c.sio.ui
	if ui == nil {
		ui = console.NewPlain(c.sio.in, c.sio.out)
	}
	choice, err := ui.Select("Socratica · Elige el harness para este flujo", opts)
	if choice == "cancel" {
		return "", console.ErrCanceled
	}
	return choice, err
}

// A progress panel owns the terminal only while the model is running. Stop it
// before any prompt or child process can read/write the same terminal.
func (c *socraticaCmd) generate(id, client string, event domain.Event) (usecases.SocraticaResult, error) {
	label := "Generando preguntas con " + client
	if r, err := c.u.Get(c.ctx, id); err == nil && (r.Flow.Phase == domain.PhaseRewriting || r.Flow.LastStablePhase == domain.PhaseRewriting || event.Kind == "answer" || event.Kind == "skip" || event.Kind == "skip_rest") {
		label = "Reescribiendo con " + client
	}
	if !c.sio.tty || c.sio.env == nil {
		return c.u.Apply(c.ctx, id, event, usecases.OriginLocal)
	}
	flow := console.NewFlow(c.sio.out, *c.sio.env, "Socratica")
	defer flow.Stop()
	flow.Plan(label)
	flow.Progress(label)
	r, err := c.u.Apply(c.ctx, id, event, usecases.OriginLocal)
	step := console.StepResult{Name: label}
	if err != nil {
		step.Status = console.StepFail
		step.Detail = "No se pudo completar la generación"
	}
	flow.Done(step)
	flow.End("")
	return r, err
}

// readPrompt takes the text from the arguments, from stdin with "-", or asks
// for it on a TTY.
func (c *socraticaCmd) readPrompt(rest []string) (string, error) {
	text := strings.Join(rest, " ")
	switch {
	case text == "-":
		// Acotado como todo stdin de mem (H-03, auditoría 2026-10-16).
		data, err := readBounded(c.sio.in, socraticaPromptMax)
		if err != nil {
			return "", err
		}
		text = string(data)
	case text == "" && c.sio.tty:
		c.sio.printf("Tu prompt: ")
		line, err := c.sio.readLine()
		if err != nil {
			return "", fmt.Errorf("no se recibió ningún prompt")
		}
		text = line
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("falta el prompt (texto, - para stdin, o ejecútalo en una terminal)")
	}
	return text, nil
}

// drive runs the flow from its current phase until a terminal answer, an
// explicit exit or the end of the input.
func (c *socraticaCmd) drive(id string, resume bool) int {
	r, err := c.u.Get(c.ctx, id)
	if err == nil && r.Flow.Mode == domain.ModeSpecify {
		return c.showSpecify(r)
	}
	if err == nil && resume && (r.Flow.Phase == domain.PhaseCaptured || r.Flow.Phase == domain.PhaseRewriting) {
		r, err = c.generate(id, r.Flow.Client, domain.Event{Kind: "resume"})
	}
	for {
		if err != nil {
			if errors.Is(err, console.ErrCanceled) {
				c.sio.printf("Flujo pendiente; puedes retomarlo con mem socratica pending --resume %s\n", id)
				return socraticaExitOK
			}
			current, getErr := c.u.Get(c.ctx, id)
			if getErr != nil || current.Flow.Phase != domain.PhaseFailed {
				c.sio.errorf("Error: %v\n", err)
				return socraticaExitUsage
			}
			r, err = current, nil
		}
		f := r.Flow
		switch {
		case f.InFlight:
			c.sio.printf("Envío en curso: otro proceso está despachando este flujo. No se cambió nada.\n")
			return socraticaExitUsage
		case f.Uncertain:
			return c.reconcile(f)
		}
		switch f.Phase {
		case domain.PhaseQuestions:
			r, err = c.ask(f)
		case domain.PhaseCaptured, domain.PhaseRewriting:
			r, err = c.generate(id, f.Client, domain.Event{Kind: "resume"})
		case domain.PhaseReview, domain.PhaseApproved:
			return c.review(f)
		case domain.PhaseFailed:
			return c.failed(f)
		case domain.PhaseDispatched:
			c.sio.printf("Ya enviado (%s).\n", f.DispatchID)
			return socraticaExitOK
		case domain.PhaseCancelled:
			c.sio.printf("%s\n", f.OriginalPrompt)
			return socraticaExitOK
		default:
			return socraticaExitUsage
		}
	}
}

// ask asks the next unanswered question. Without a TTY the remaining ones
// are skipped: an unanswered question counts as omitted.
func (c *socraticaCmd) ask(f domain.RefinementFlow) (usecases.SocraticaResult, error) {
	if !c.sio.tty {
		return c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "skip_rest"}, usecases.OriginLocal)
	}
	i := len(f.Answers)
	q := f.Questions[i]
	if c.sio.ui != nil {
		opts := make([]console.Option, 0, len(q.Options)+3)
		for n, o := range q.Options {
			opts = append(opts, console.Option{Value: fmt.Sprint(n), Label: o})
		}
		opts = append(opts, console.Option{Value: "other", Label: "Otra respuesta"}, console.Option{Value: "skip", Label: "Omitir pregunta"}, console.Option{Value: "skip_rest", Label: "Omitir el resto y mejorar"})
		choice, err := c.sio.ui.Select(fmt.Sprintf("Pregunta %d de %d · %s", i+1, len(f.Questions), q.Text), opts)
		if err != nil {
			return usecases.SocraticaResult{}, console.ErrCanceled
		}
		ev := domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: i}}
		switch choice {
		case "skip", "skip_rest":
			ev.Kind = choice
		case "other":
			c.sio.printf("Tu respuesta: ")
			custom, readErr := c.sio.readLine()
			if readErr != nil || strings.TrimSpace(custom) == "" {
				return usecases.SocraticaResult{}, console.ErrCanceled
			}
			ev.Answer.Custom = custom
		default:
			var n int
			if _, err := fmt.Sscanf(choice, "%d", &n); err != nil || n < 0 || n >= len(q.Options) {
				return c.u.Get(c.ctx, f.ID)
			}
			ev.Answer.Choice = q.Options[n]
		}
		return c.answer(f, ev)
	}
	c.sio.printf("\nPregunta %d de %d: %s\n", i+1, len(f.Questions), q.Text)
	for n, o := range q.Options {
		c.sio.printf("  %d) %s\n", n+1, o)
	}
	c.sio.printf("  o) Otra (escríbela)   s) Omitir   S) Omitir el resto y mejorar ya\n> ")
	for {
		line, err := c.sio.readLine()
		if err != nil {
			return c.answer(f, domain.Event{Kind: "skip_rest"})
		}
		switch line = strings.TrimSpace(line); line {
		case "S":
			return c.answer(f, domain.Event{Kind: "skip_rest"})
		case "s":
			return c.answer(f, domain.Event{Kind: "skip"})
		case "o":
			c.sio.printf("Tu respuesta: ")
			custom, err := c.sio.readLine()
			if err == nil && strings.TrimSpace(custom) != "" {
				return c.answer(f, domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: i, Custom: custom}})
			}
		default:
			var n int
			if _, err := fmt.Sscanf(line, "%d", &n); err == nil && n >= 1 && n <= len(q.Options) {
				return c.answer(f, domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: i, Choice: q.Options[n-1]}})
			}
		}
		c.sio.printf("Elige un número, o, s o S: ")
	}
}

func (c *socraticaCmd) answer(f domain.RefinementFlow, ev domain.Event) (usecases.SocraticaResult, error) {
	if ev.Kind == "skip_rest" || len(f.Answers)+1 == len(f.Questions) {
		return c.generate(f.ID, f.Client, ev)
	}
	return c.u.Apply(c.ctx, f.ID, ev, usecases.OriginLocal)
}

func (c *socraticaCmd) action(title, fallback string, opts []console.Option) (string, error) {
	if c.sio.ui != nil {
		return c.sio.ui.Select(title, opts)
	}
	c.sio.printf("%s", fallback)
	return c.sio.readLine()
}

func (c *socraticaCmd) printReview(f domain.RefinementFlow) {
	if c.sio.env != nil && c.sio.tty {
		layout := console.NewLayout(*c.sio.env)
		c.sio.printf("\n%s\n", rendered(layout.Panel("Revisión · "+f.Client, "Original", "", strings.Split(domain.StripTerminalControls(f.OriginalPrompt), "\n"))))
		c.sio.printf("%s\n", rendered(layout.Panel("Propuesta", "", "", strings.Split(domain.StripTerminalControls(f.ProposedPrompt), "\n"))))
	} else {
		c.sio.printf("\n── Original ──\n%s\n\n── Propuesta ──\n%s\n\n", f.OriginalPrompt, f.ProposedPrompt)
	}
	c.sio.printf("Palabras: %d → %d · adiciones: %d\n", domain.WordCount(f.OriginalPrompt), domain.WordCount(f.ProposedPrompt), len(f.Additions))
	for _, a := range f.Additions {
		c.sio.printf("  + %s\n", a)
	}
	if s := f.Scrutiny; s != nil {
		c.sio.printf("\n── Síntesis ──\n%s\n", s.Restatement)
		for _, a := range s.Assumptions {
			c.sio.printf("  · [%s] %s\n", a.Kind, a.Text)
		}
		if s.StrongestSupport != "" {
			c.sio.printf("A favor: %s\n", s.StrongestSupport)
		}
		if s.StrongestObjection != "" {
			c.sio.printf("Objeción más fuerte: %s\n", s.StrongestObjection)
		}
		for _, h := range s.Hypotheses {
			c.sio.printf("Hipótesis: %s\n", strings.TrimPrefix(h, "Hipótesis: "))
		}
	}
}

// review shows the proposal and its actions. Without a TTY it only prints:
// the flow stays pending for `pending --resume`.
func (c *socraticaCmd) review(f domain.RefinementFlow) int {
	c.printReview(f)
	if !c.sio.tty {
		c.sio.printf("\nSin terminal interactiva no se ejecuta nada. Retómalo con: mem socratica pending --resume %s\n", f.ID)
		return socraticaExitOK
	}
	for {
		line, err := c.action("Socratica · ¿Qué quieres hacer con la propuesta?", "\n[e]jecutar [d]itar [c]opiar [x]cancelar: ", []console.Option{{Value: "e", Label: "Ejecutar en " + f.Client}, {Value: "d", Label: "Editar propuesta"}, {Value: "c", Label: "Copiar propuesta"}, {Value: "x", Label: "Cancelar", Hint: "conservar el original"}})
		if err != nil {
			return socraticaExitOK
		}
		switch strings.TrimSpace(line) {
		case "e":
			return c.execute(f)
		case "d":
			edited, ok := c.readEdit()
			if !ok {
				continue
			}
			r, err := c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "edit", Proposed: edited}, usecases.OriginLocal)
			if err != nil {
				c.sio.errorf("Error: %v\n", err)
				continue
			}
			f = r.Flow
			c.printReview(f)
		case "c":
			c.copy(f.ProposedPrompt)
		case "x":
			return c.cancel(f.ID)
		}
	}
}

// readEdit reads lines until one containing only ".".
func (c *socraticaCmd) readEdit() (string, bool) {
	c.sio.printf("Escribe el prompt editado y termina con una línea que contenga solo «.»:\n")
	var lines []string
	for {
		line, err := c.sio.readLine()
		if err != nil || line == "." {
			break
		}
		lines = append(lines, line)
	}
	text := strings.Join(lines, "\n")
	return text, strings.TrimSpace(text) != ""
}

// copy uses OSC 52 on a terminal that is not dumb; otherwise it prints a
// delimited block to copy by hand. No system utility is invoked.
func (c *socraticaCmd) copy(text string) {
	text = domain.StripTerminalControls(domain.ProtectText(text))
	if c.sio.tty && os.Getenv("TERM") != "dumb" && os.Getenv("TERM") != "" {
		_, _ = fmt.Fprintf(c.sio.out, "\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(text)))
		c.sio.printf("Copiado al portapapeles (OSC 52). Si tu terminal no lo admite, cópialo de aquí:\n")
	}
	c.sio.printf("-----8<-----\n%s\n-----8<-----\n", text)
}

func (c *socraticaCmd) cancel(id string) int {
	r, err := c.u.Apply(c.ctx, id, domain.Event{Kind: "cancel"}, usecases.OriginLocal)
	if err != nil {
		c.flowError(id, err)
		return socraticaExitUsage
	}
	if r.Flow.InFlight {
		c.sio.printf("Envío en curso: otro proceso está despachando este flujo. No se cambió nada.\n")
		return socraticaExitUsage
	}
	if r.Flow.OriginalPrompt == "" {
		c.sio.printf("Cancelado. El original no está disponible en esta instancia.\n")
		return socraticaExitOK
	}
	c.sio.printf("Cancelado. Tu prompt original:\n%s\n", r.Flow.OriginalPrompt)
	return socraticaExitOK
}

// flowError reports a failure to load flow id: an unknown id reads as such,
// and any other failure keeps its own cause instead of posing as "not found".
func (c *socraticaCmd) flowError(id string, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		c.sio.errorf("Error: no existe el flujo %s\n", id)
		return
	}
	c.sio.errorf("Error: %v\n", err)
}

// showSpecify only reports a specify flow: its preparation belongs to the
// agent, so the generic local routes (generate, review, edit, execute) never
// see it. Reading starts nothing and consumes nothing.
func (c *socraticaCmd) showSpecify(r usecases.SocraticaResult) int {
	f := r.Flow
	v := usecases.SpecifyView(f)
	c.sio.printf("Flujo %s · modo specify · %s\n", f.ID, f.Client)
	if f.PrivateStateUnavailable {
		c.sio.printf("Reserva privada: el contenido solo existe en la instancia que lo inició; esta consulta no puede mostrarlo ni recuperarlo.\nPuedes cancelar la reserva y empezar de nuevo: mem socratica pending --cancel %s\n", f.ID)
		return socraticaExitOK
	}
	c.sio.printf("Fase: %s · Etapa: %s · Revisión: %d · Preguntas: %d/%d\n", f.Phase, v.Stage, f.Revision, v.QuestionsUsed, domain.MaxSocraticaQuestions)
	c.sio.printf("Original:\n%s\n", f.OriginalPrompt)
	switch {
	case f.InFlight:
		c.sio.printf("Envío en curso: otro proceso está despachando este flujo.\n")
	case f.Uncertain:
		c.sio.printf("Envío incierto: puede que ya se haya enviado; decídelo desde el agente (reenviar o marcar como enviado).\n")
	case f.Phase == domain.PhaseReview:
		c.sio.printf("Preparación comprobada: pendiente de que la persona la revise y elija «Aprobar y generar especificación» en el agente.\n")
	case f.Phase == domain.PhaseApproved:
		c.sio.printf("Aprobado, sin enviar: el agente confirma y entrega a Spec Kit.\n")
	case f.Phase == domain.PhaseDispatched:
		c.sio.printf("Ya enviado (%s).\n", f.DispatchID)
	}
	for _, check := range v.PendingChecks {
		c.sio.printf("  - %s @ %s: %s\n", check.Code, check.Path, check.Message)
	}
	// A terminal flow can neither be resumed nor cancelled.
	if f.Phase.Terminal() {
		c.sio.printf("Aquí solo se consulta; el resultado o un reenvío se gestionan desde el agente (%s) con socratica_flow.\n", f.Client)
		return socraticaExitOK
	}
	c.sio.printf("Esto se prepara desde el agente: retómalo en %s con socratica_flow (get o resume). Aquí solo se consulta; no se genera ni se ejecuta nada.\nPara descartarlo: mem socratica pending --cancel %s\n", f.Client, f.ID)
	return socraticaExitOK
}

// execute delegates the whole dispatch to the use case (R4).
func (c *socraticaCmd) execute(f domain.RefinementFlow) int {
	if !c.sio.tty {
		c.sio.printf("Sin terminal interactiva no se ejecuta nada.\n")
		return socraticaExitOK
	}
	launcher := c.launcher
	if launcher == nil {
		launcher = c.deps.SocraticaExec
	}
	r, err := c.u.Execute(c.ctx, f.ID, f.Client, launcher)
	switch {
	case errors.Is(err, usecases.ErrDispatchUnconfirmed):
		c.sio.errorf("Error: el agente arrancó pero no se pudo confirmar el envío; no reintentes. Revísalo con: mem socratica pending --resume %s\n", f.ID)
		return socraticaExitUsage
	case errors.Is(err, usecases.ErrDispatchUncertain):
		c.sio.errorf("Puede que ya se haya enviado. Decide con: mem socratica pending --resume %s\n", f.ID)
		return socraticaExitUsage
	case errors.Is(err, ports.ErrLaunchFailed):
		c.sio.errorf("No se pudo arrancar %s: %v\nEl flujo queda aprobado sin confirmar; retómalo con: mem socratica pending --resume %s\n", f.Client, err, f.ID)
		return socraticaExitUsage
	case err != nil:
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	case r.Flow.InFlight:
		c.sio.printf("Envío en curso: otro proceso está despachando este flujo. No se cambió nada.\n")
		return socraticaExitUsage
	case r.Flow.AlreadyDispatched:
		c.sio.printf("Ya enviado (%s).\n", r.Flow.DispatchID)
		return socraticaExitOK
	}
	// stderr: with --print, stdout carries only the prompt.
	c.sio.errorf("Enviado a %s (%s).\n", f.Client, r.Flow.DispatchID)
	return socraticaExitOK
}

// failed keeps the original byte for byte and asks for an explicit choice
// (FR-016); "send my original" goes through Execute with send_original.
func (c *socraticaCmd) failed(f domain.RefinementFlow) int {
	info := c.u.ExplainFailure(f.LastError)
	c.sio.printf("\nNo se pudo generar: %s\nTu prompt original queda intacto:\n%s\n", info.Message, f.OriginalPrompt)
	if !c.sio.tty {
		return socraticaExitUsage
	}
	for {
		opts := []console.Option{{Value: "h", Label: "Cambiar harness", Hint: "conservar el prompt y volver a refinar"}, {Value: "x", Label: "Cancelar", Hint: "conservar el original"}, {Value: "o", Label: "Enviar mi original", Hint: "ejecutar sin refinamiento"}}
		if !c.printOnly {
			opts = append([]console.Option{{Value: "a", Label: "Continuar asistido en " + f.Client, Hint: "preguntas y propuesta dentro del harness; sin ejecutar la tarea"}}, opts...)
		}
		fallback := "\n[h]cambiar harness [o]riginal [x]cancelar: "
		if info.Retryable {
			opts = append(opts, console.Option{Value: "r", Label: "Reintentar con " + f.Client, Hint: "misma configuración"})
			fallback = "\n[r]eintentar [o]riginal [x]cancelar [h]cambiar harness: "
		}
		if !c.printOnly {
			fallback += "[a]continuar asistido: "
		}
		line, err := c.action("Socratica · No se pudo generar con "+f.Client, fallback, opts)
		if err != nil {
			return socraticaExitUsage
		}
		switch strings.TrimSpace(line) {
		case "a":
			if c.printOnly {
				continue
			}
			c.sio.printf("Abriendo Socratica asistida en %s. Tu original se refina en la conversación; todavía no se ejecuta.\n", f.Client)
			if err := c.u.Assist(c.ctx, f.ID, c.deps.SocraticaExec); err != nil {
				c.sio.errorf("No se pudo abrir la sesión asistida: %v\n", err)
				return socraticaExitUsage
			}
			c.sio.printf("Sesión asistida terminada. Consulta el flujo con mem socratica pending.\n")
			return socraticaExitOK
		case "h":
			return c.changeHarness(f)
		case "r":
			if !info.Retryable {
				c.sio.printf("Elige otro harness o corrige su configuración antes de continuar.\n")
				continue
			}
			if _, err := c.generate(f.ID, f.Client, domain.Event{Kind: "retry"}); err != nil {
				current, getErr := c.u.Get(c.ctx, f.ID)
				if getErr != nil {
					c.sio.errorf("Error: %v\n", err)
					return socraticaExitUsage
				}
				f = current.Flow
				info = c.u.ExplainFailure(f.LastError)
				c.sio.printf("\nNo se pudo generar: %s\n", info.Message)
				continue
			}
			return c.drive(f.ID, false)
		case "o":
			return c.execute(f)
		case "x":
			return c.cancel(f.ID)
		}
	}
}

func (c *socraticaCmd) changeHarness(f domain.RefinementFlow) int {
	client, err := c.chooseHarness(f.Client)
	if errors.Is(err, console.ErrCanceled) {
		return socraticaExitOK
	}
	if err != nil {
		c.sio.errorf("No hay otro harness disponible. El original sigue en el flujo %s.\n", f.ID)
		return socraticaExitNoGenerator
	}
	r, err := c.u.ChangeHarness(c.ctx, f.ID, client)
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	c.sio.printf("Generando preguntas con %s…\n", client)
	return c.drive(r.Flow.ID, true)
}

// reconcile handles a claim without ack: the person decides, never mem.
func (c *socraticaCmd) reconcile(f domain.RefinementFlow) int {
	c.sio.printf("Puede que ya se haya enviado (%s). No se relanza nada automáticamente.\n", f.DispatchID)
	if !c.sio.tty {
		return socraticaExitUsage
	}
	for {
		line, err := c.action("Socratica · Conciliar envío", "[m]arcar enviado [r]eenviar [x]cancelar: ", []console.Option{{Value: "m", Label: "Marcar como enviado"}, {Value: "r", Label: "Reenviar", Hint: "puede duplicar una tarea ya iniciada"}, {Value: "x", Label: "Cancelar"}})
		if err != nil {
			return socraticaExitUsage
		}
		kind := map[string]string{"m": "mark_dispatched", "r": "force_resend", "x": "cancel"}[strings.TrimSpace(line)]
		if kind == "" {
			continue
		}
		r, err := c.u.Apply(c.ctx, f.ID, domain.Event{Kind: kind, DispatchID: f.DispatchID}, usecases.OriginLocal)
		if err != nil {
			c.sio.errorf("Error: %v\n", err)
			return socraticaExitUsage
		}
		if r.Flow.InFlight {
			c.sio.printf("Envío en curso: otro proceso está despachando este flujo. No se cambió nada.\n")
			return socraticaExitUsage
		}
		switch kind {
		case "mark_dispatched":
			c.sio.printf("Marcado como enviado.\n")
			return socraticaExitOK
		case "cancel":
			c.sio.printf("Cancelado. Tu prompt original:\n%s\n", r.Flow.OriginalPrompt)
			return socraticaExitOK
		}
		return c.execute(r.Flow)
	}
}

func (c *socraticaCmd) history(args []string) int {
	fs := c.flags("history")
	client := fs.String("client", "", "filtra por agente")
	show := fs.String("show", "", "muestra un registro completo")
	asJSON := fs.Bool("json", false, "salida JSON")
	if fs.Parse(args) != nil {
		return socraticaExitUsage
	}
	rows, err := c.u.History(c.ctx, c.deps.Project, *client)
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	if *show != "" {
		return c.showHistory(rows, *show)
	}
	if *asJSON {
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return socraticaExitUsage
		}
		c.sio.printf("%s\n", data)
		return socraticaExitOK
	}
	if len(rows) == 0 {
		c.sio.printf("Sin historial de Socratica.\n")
		return socraticaExitOK
	}
	for _, h := range rows {
		c.sio.printf("%s  %s  %-8s %-8s %d→%d  %s\n", h.ID, h.CompletedAt.Local().Format("2006-01-02 15:04"), h.Client, h.Mode, h.WordsBefore, h.WordsAfter, truncateRunes(h.OriginalPrompt, 50))
	}
	return socraticaExitOK
}

func (c *socraticaCmd) showHistory(rows []domain.RefinementHistory, id string) int {
	for _, h := range rows {
		if h.ID != id && h.FlowID != id {
			continue
		}
		c.sio.printf("── Original ──\n%s\n\n── Propuesta ──\n%s\n", h.OriginalPrompt, h.ProposedPrompt)
		if h.ApprovedPrompt != "" {
			c.sio.printf("\n── Aprobado ──\n%s\n", h.ApprovedPrompt)
		}
		r, err := c.u.Get(c.ctx, h.FlowID)
		if err != nil || r.Flow.Phase != domain.PhaseDispatched {
			return socraticaExitOK
		}
		// A consumed authorization is never resent automatically (I10).
		c.sio.printf("\nYa enviado (%s).\n", r.Flow.DispatchID)
		// A specify flow is resent by the agent only: the local executor refuses
		// it, and resending here would already have dropped its outcome.
		if r.Flow.Mode == domain.ModeSpecify {
			c.sio.printf("El reenvío de specify se decide desde el agente (%s) con socratica_flow; aquí solo se consulta.\n", r.Flow.Client)
			return socraticaExitOK
		}
		if !c.sio.tty {
			return socraticaExitOK
		}
		c.sio.printf("[r]eenviar (autorización nueva) o Intro para salir: ")
		if line, err := c.sio.readLine(); err != nil || strings.TrimSpace(line) != "r" {
			return socraticaExitOK
		}
		c.sio.printf("¿Seguro? La tarea podría ejecutarse dos veces. Escribe «reenviar» para confirmar: ")
		if line, err := c.sio.readLine(); err != nil || strings.TrimSpace(line) != "reenviar" {
			c.sio.printf("No se reenvió nada.\n")
			return socraticaExitOK
		}
		resent, err := c.u.Apply(c.ctx, r.Flow.ID, domain.Event{Kind: "force_resend", DispatchID: r.Flow.DispatchID}, usecases.OriginLocal)
		if err != nil {
			c.sio.errorf("Error: %v\n", err)
			return socraticaExitUsage
		}
		return c.execute(resent.Flow)
	}
	c.sio.errorf("No existe el registro %s en el historial.\n", id)
	return socraticaExitUsage
}

func (c *socraticaCmd) toggle(on bool, args []string) int {
	name := "off"
	if on {
		name = "on"
	}
	fs := c.flags(name)
	client := fs.String("client", "", "claude | codex | opencode")
	unverified := false
	if on {
		fs.BoolVar(&unverified, "unverified", false, "activa sin evidencia de aceptación (solo para obtenerla)")
	}
	if fs.Parse(args) != nil || !domain.IsSocraticaClient(*client) {
		c.sio.errorf("Uso: mem socratica %s --client claude|codex|opencode\n", name)
		return socraticaExitUsage
	}
	if on && !domain.CapabilitiesForSocratica(*client).Interception && !unverified {
		c.sio.errorf("Intercepción no soportada en %s (sin evidencia de aceptación). No se cambió nada.\n", *client)
		c.sio.errorf("Para probarla y obtener esa evidencia: mem socratica on --client %s --unverified\n", *client)
		return socraticaExitUsage
	}
	s := c.deps.SettingsRepo.Read(c.deps.Root)
	if s.Interception == nil {
		s.Interception = map[string]bool{}
	}
	if s.InterceptionUnverified == nil {
		s.InterceptionUnverified = map[string]bool{}
	}
	s.Interception[*client] = on
	s.InterceptionUnverified[*client] = on && unverified
	if err := c.writeSettings(s); err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	switch {
	case on && unverified:
		c.sio.printf("Aviso: intercepción sin verificar en %s.\n", *client)
	case on:
		c.sio.printf("Intercepción activada en %s.\n", *client)
	default:
		c.sio.printf("Intercepción desactivada en %s.\n", *client)
	}
	return socraticaExitOK
}

// writeSettings persists and keeps the in-process use case in step, so a
// long-lived caller (the TUI) never acts on settings read at startup.
func (c *socraticaCmd) writeSettings(s ports.SettingsData) error {
	if _, err := domain.ValidateSocraticaSettings(s.SocraticaSettings); err != nil {
		return err
	}
	if err := c.deps.SettingsRepo.Write(c.deps.Root, s); err != nil {
		return err
	}
	c.u.Settings = s.SocraticaSettings
	return nil
}

func (c *socraticaCmd) status() int {
	settings, err := domain.ValidateSocraticaSettings(c.deps.SettingsRepo.Read(c.deps.Root).SocraticaSettings)
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	pending, err := c.u.Pending(c.ctx, c.deps.Project)
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitStorage
	}
	byClient := map[string]domain.RefinementFlow{}
	for _, r := range pending {
		byClient[r.Flow.Client] = r.Flow
	}
	for _, client := range []string{"claude", "codex", "opencode"} {
		caps := domain.CapabilitiesForSocratica(client)
		state := "desactivada"
		switch {
		case settings.Interception[client] && caps.Interception:
			state = "activa"
		case settings.Interception[client] && settings.InterceptionUnverified[client]:
			state = "sin verificar"
		}
		c.sio.printf("%-9s intercepción: %-13s capacidades: comando=%t intercepción=%t portapapeles=%t", client, state, caps.NamedCommand, caps.Interception, caps.Clipboard)
		if f, ok := byClient[client]; ok {
			c.sio.printf("  pendiente: %s (%s, hace %s)", f.ID, f.Phase, time.Since(f.UpdatedAt).Round(time.Minute))
		}
		c.sio.printf("\n")
	}
	c.sio.printf("Modo privado: %t · harness: se elige en cada flujo\n", settings.Private)
	for _, h := range c.u.Harnesses() {
		state := "disponible"
		if !h.Available {
			state = h.Reason
		}
		c.sio.printf("  %s: %s\n", h.Label, state)
	}
	return socraticaExitOK
}

func (c *socraticaCmd) pending(args []string) int {
	fs := c.flags("pending")
	resume := fs.String("resume", "", "retoma un flujo")
	cancel := fs.String("cancel", "", "cancela un flujo e imprime el original")
	if fs.Parse(args) != nil {
		return socraticaExitUsage
	}
	switch {
	case *cancel != "":
		return c.cancel(*cancel)
	case *resume != "":
		c.launcher = c.deps.SocraticaExec
		r, err := c.u.Get(c.ctx, *resume)
		if err != nil {
			c.flowError(*resume, err)
			return socraticaExitUsage
		}
		if r.Flow.Mode == domain.ModeSpecify {
			return c.showSpecify(r)
		}
		if r.Flow.Phase == domain.PhaseApproved && !r.Flow.Uncertain && !r.Flow.InFlight {
			// Approved without ack nor claim: nothing ran, back to review.
			r, err = c.u.Resume(c.ctx, *resume, usecases.OriginLocal)
			if err != nil {
				c.sio.errorf("Error: %v\n", err)
				return socraticaExitUsage
			}
		}
		return c.drive(*resume, true)
	}
	rows, err := c.u.Pending(c.ctx, c.deps.Project)
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitStorage
	}
	if len(rows) == 0 {
		c.sio.printf("Sin flujos pendientes.\n")
	}
	for _, r := range rows {
		state := string(r.Flow.Phase)
		if r.Flow.Mode == domain.ModeSpecify {
			state = "specify:" + string(usecases.SpecifyView(r.Flow).Stage)
			if r.Flow.PrivateStateUnavailable {
				state = "specify:reserva"
			}
		}
		switch {
		case r.Flow.InFlight:
			state = "envío en curso"
		case r.Flow.Uncertain:
			state = "envío incierto"
		}
		c.sio.printf("%s  %-8s %-15s hace %s  %s\n", r.Flow.ID, r.Flow.Client, state, time.Since(r.Flow.UpdatedAt).Round(time.Minute), truncateRunes(r.Flow.OriginalPrompt, 50))
	}
	return socraticaExitOK
}

func (c *socraticaCmd) private(args []string) int {
	if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
		c.sio.errorf("Uso: mem socratica private on|off\n")
		return socraticaExitUsage
	}
	s := c.deps.SettingsRepo.Read(c.deps.Root)
	s.Private = args[0] == "on"
	if err := c.writeSettings(s); err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	if s.Private {
		c.sio.printf("Modo privado activado: Socratica no guarda historial.\n")
	} else {
		c.sio.printf("Modo privado desactivado.\n")
	}
	return socraticaExitOK
}

func truncateRunes(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
