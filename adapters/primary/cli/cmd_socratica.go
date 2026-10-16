package cli

import (
	"bufio"
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
}

func (s socraticaIO) printf(format string, args ...any) {
	_, _ = fmt.Fprint(s.out, domain.ProtectText(fmt.Sprintf(format, args...)))
}
func (s socraticaIO) errorf(format string, args ...any) {
	_, _ = fmt.Fprint(s.err, domain.ProtectText(fmt.Sprintf(format, args...)))
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
	sio := socraticaIO{in: bufio.NewReader(os.Stdin), out: os.Stdout, err: os.Stderr, tty: tty}
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
	launcher ports.AgentLauncher
}

func (c *socraticaCmd) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("mem socratica "+name, flag.ContinueOnError)
	fs.SetOutput(c.sio.err)
	return fs
}

func (c *socraticaCmd) refine(args []string) int {
	fs := c.flags("refine")
	mode := fs.String("mode", "refine", "refine | scrutiny")
	client := fs.String("client", "", "agente destino: claude | codex | opencode")
	printOnly := fs.Bool("print", false, "imprime el prompt aprobado en vez de lanzar el agente")
	raw := fs.Bool("raw", false, "con --print, sin el prefijo >")
	if fs.Parse(args) != nil {
		return socraticaExitUsage
	}
	text, err := c.readPrompt(fs.Args())
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	gen := c.u.Generator
	if gen == nil || !gen.Available() {
		c.sio.errorf("No hay ningún agente disponible para generar preguntas (claude o codex en el PATH). Tu prompt queda intacto:\n")
		c.sio.printf("%s\n", text)
		return socraticaExitNoGenerator
	}
	target := *client
	if target == "" {
		target = gen.Name()
	}
	if !domain.IsSocraticaClient(target) || (*mode != string(domain.ModeRefine) && *mode != string(domain.ModeScrutiny)) {
		c.sio.errorf("Error: --client debe ser claude, codex u opencode y --mode refine o scrutiny\n")
		return socraticaExitUsage
	}
	c.launcher = c.deps.SocraticaExec
	if *printOnly {
		c.launcher = c.deps.SocraticaPrint(c.sio.out, *raw)
	}
	// Acknowledge immediately, before the generator runs (FR-017).
	c.sio.printf("Generando preguntas con %s…\n", gen.Name())
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
	if err == nil && resume && (r.Flow.Phase == domain.PhaseCaptured || r.Flow.Phase == domain.PhaseRewriting) {
		r, err = c.u.Resume(c.ctx, id, usecases.OriginLocal)
	}
	for {
		if err != nil {
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
			r, err = c.u.Resume(c.ctx, id, usecases.OriginLocal)
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
	c.sio.printf("\nPregunta %d de %d: %s\n", i+1, len(f.Questions), q.Text)
	for n, o := range q.Options {
		c.sio.printf("  %d) %s\n", n+1, o)
	}
	c.sio.printf("  o) Otra (escríbela)   s) Omitir   S) Omitir el resto y mejorar ya\n> ")
	for {
		line, err := c.sio.readLine()
		if err != nil {
			return c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "skip_rest"}, usecases.OriginLocal)
		}
		switch line = strings.TrimSpace(line); line {
		case "S":
			return c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "skip_rest"}, usecases.OriginLocal)
		case "s":
			return c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "skip"}, usecases.OriginLocal)
		case "o":
			c.sio.printf("Tu respuesta: ")
			custom, err := c.sio.readLine()
			if err == nil && strings.TrimSpace(custom) != "" {
				return c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: i, Custom: custom}}, usecases.OriginLocal)
			}
		default:
			var n int
			if _, err := fmt.Sscanf(line, "%d", &n); err == nil && n >= 1 && n <= len(q.Options) {
				return c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: i, Choice: q.Options[n-1]}}, usecases.OriginLocal)
			}
		}
		c.sio.printf("Elige un número, o, s o S: ")
	}
}

func (c *socraticaCmd) printReview(f domain.RefinementFlow) {
	c.sio.printf("\n── Original ──\n%s\n\n── Propuesta ──\n%s\n\n", f.OriginalPrompt, f.ProposedPrompt)
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
		c.sio.printf("\n[e]jecutar [d]itar [c]opiar [x]cancelar: ")
		line, err := c.sio.readLine()
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
	text = domain.ProtectText(text)
	if c.sio.tty && os.Getenv("TERM") != "dumb" && os.Getenv("TERM") != "" {
		_, _ = fmt.Fprintf(c.sio.out, "\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(text)))
		c.sio.printf("Copiado al portapapeles (OSC 52). Si tu terminal no lo admite, cópialo de aquí:\n")
	}
	c.sio.printf("-----8<-----\n%s\n-----8<-----\n", text)
}

func (c *socraticaCmd) cancel(id string) int {
	r, err := c.u.Apply(c.ctx, id, domain.Event{Kind: "cancel"}, usecases.OriginLocal)
	if err != nil {
		c.sio.errorf("Error: %v\n", err)
		return socraticaExitUsage
	}
	if r.Flow.InFlight {
		c.sio.printf("Envío en curso: otro proceso está despachando este flujo. No se cambió nada.\n")
		return socraticaExitUsage
	}
	c.sio.printf("Cancelado. Tu prompt original:\n%s\n", r.Flow.OriginalPrompt)
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
	c.sio.printf("\nNo se pudo generar: %s\nTu prompt original queda intacto:\n%s\n", f.LastError, f.OriginalPrompt)
	if !c.sio.tty {
		return socraticaExitUsage
	}
	for {
		c.sio.printf("\n[r]eintentar [o]riginal [x]cancelar: ")
		line, err := c.sio.readLine()
		if err != nil {
			return socraticaExitUsage
		}
		switch strings.TrimSpace(line) {
		case "r":
			if _, err := c.u.Apply(c.ctx, f.ID, domain.Event{Kind: "retry"}, usecases.OriginLocal); err != nil {
				current, getErr := c.u.Get(c.ctx, f.ID)
				if getErr != nil {
					c.sio.errorf("Error: %v\n", err)
					return socraticaExitUsage
				}
				f = current.Flow
				c.sio.printf("\nNo se pudo generar: %s\n", f.LastError)
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

// reconcile handles a claim without ack: the person decides, never mem.
func (c *socraticaCmd) reconcile(f domain.RefinementFlow) int {
	c.sio.printf("Puede que ya se haya enviado (%s). No se relanza nada automáticamente.\n", f.DispatchID)
	if !c.sio.tty {
		return socraticaExitUsage
	}
	for {
		c.sio.printf("[m]arcar enviado [r]eenviar [x]cancelar: ")
		line, err := c.sio.readLine()
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
	generatorName := "ninguno"
	if c.u.Generator != nil && c.u.Generator.Available() {
		generatorName = c.u.Generator.Name()
	}
	c.sio.printf("Modo privado: %t · generador: %s\n", settings.Private, generatorName)
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
			c.sio.errorf("Error: no existe el flujo %s\n", *resume)
			return socraticaExitUsage
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
