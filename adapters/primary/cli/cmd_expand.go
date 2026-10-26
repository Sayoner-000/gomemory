package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/charmbracelet/x/term"
	"io"
	"mem/adapters/primary/cli/clikit"
	"mem/adapters/primary/console"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"mem/adapters/primary/setup"
	"mem/domain"
)

type expandIO struct {
	in       io.Reader
	out, err io.Writer
	tty      bool
	home     string
	now      func() time.Time
	ui       console.UI
	install  func(string, string) ([]string, error)
}

func CmdExpand(deps *Deps, args []string) {
	home, _ := os.UserHomeDir()
	x := expandIO{in: os.Stdin, out: os.Stdout, err: os.Stderr, home: home, now: time.Now, tty: term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())}
	if x.tty {
		x.ui = console.NewPlain(os.Stdin, os.Stdout)
	}
	os.Exit(runExpand(deps, args, x))
}

func runExpand(d *Deps, args []string, x expandIO) int {
	ctx := context.Background()
	if x.now == nil {
		x.now = time.Now
	}
	if x.in == nil {
		x.in = strings.NewReader("")
	}
	if x.out == nil {
		x.out = io.Discard
	}
	if x.err == nil {
		x.err = io.Discard
	}
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	js := false
	for _, a := range args {
		if a == "--json" {
			js = true
		}
	}
	emit := func(v any) {
		if err := json.NewEncoder(x.out).Encode(v); err != nil {
			fmt.Fprintln(x.err, err)
		}
	}
	switch sub {
	case "instincts":
		return runExpandInstincts(d, args, x)
	case "on", "off":
		if len(args) > 1 {
			return 2
		}
		s := d.SettingsRepo.Read(d.Root)
		if len(args) == 0 {
			s.ExpandDisabled = sub == "off"
			if sub == "on" {
				s.ExpandOff = nil
			}
		} else {
			valid := false
			for _, n := range domain.Supertools {
				if string(n) == args[0] {
					valid = true
				}
			}
			if !valid {
				fmt.Fprintf(x.err, "supertool inválida: %s\n", args[0])
				return 2
			}
			next := []string{}
			for _, n := range s.ExpandOff {
				if n != args[0] {
					next = append(next, n)
				}
			}
			if sub == "off" {
				next = append(next, args[0])
			}
			s.ExpandOff = next
		}
		if err := d.SettingsRepo.Write(d.Root, s); err != nil {
			fmt.Fprintln(x.err, err)
			return 1
		}
		if sub == "off" && len(args) == 0 {
			fmt.Fprintln(x.out, "expand desactivado (emergencia). Reactiva con: mem expand on")
		} else if sub == "on" && len(args) == 0 {
			fmt.Fprintln(x.out, "expand activado: todas las supertools")
		} else {
			fmt.Fprintf(x.out, "%s: %s\n", args[0], sub)
		}
		return 0
	case "status":
		for _, a := range args {
			if a != "--json" {
				return 2
			}
		}
		s := d.SettingsRepo.Read(d.Root)
		rules, errs := d.RuleSource.Load(ctx, d.Root)
		states := map[string]bool{}
		for _, n := range domain.Supertools {
			on := !s.ExpandDisabled
			for _, off := range s.ExpandOff {
				if off == string(n) {
					on = false
				}
			}
			states[string(n)] = on
		}
		events := []domain.GuardCount{}
		if g, ok := d.ChannelActivity.(interface {
			GuardSince(int) ([]domain.GuardCount, error)
		}); ok {
			all, err := g.GuardSince(7)
			if err != nil {
				fmt.Fprintln(x.err, err)
				return 1
			}
			for _, e := range all {
				if strings.HasPrefix(e.Kind, "expand.") {
					events = append(events, e)
				}
			}
		}
		wiring := setup.ExpandWiring(x.home, d.Root)
		var shield any
		if d.ShieldState != nil {
			if r, ok := d.ShieldState.Last(ctx); ok {
				shield = r
			}
		}
		if js {
			emit(map[string]any{"wiring": wiring, "supertools": states, "rules": rules, "errors": errs, "activations": events, "shield": shield})
			return 0
		}
		for _, n := range []string{"claude", "codex", "opencode"} {
			mark := "✗"
			if wiring[n] {
				mark = "✓"
			}
			fmt.Fprintf(x.out, "%s %s\n", n, mark)
		}
		for _, n := range domain.Supertools {
			fmt.Fprintf(x.out, "%s: %t\n", n, states[string(n)])
		}
		for _, r := range rules {
			fmt.Fprintf(x.out, "regla: %s (%s)\n", termSafe(r.Name), r.Origin)
		}
		for _, e := range errs {
			fmt.Fprintf(x.out, "error: %s: %s\n", termSafe(e.Source), termSafe(e.Reason))
		}
		for _, e := range events {
			fmt.Fprintf(x.out, "%s %s: %d\n", termSafe(e.Agent), termSafe(e.Kind), e.Count)
		}
		if r, ok := shield.(domain.ShieldReport); ok {
			fmt.Fprintf(x.out, "Shield: %s\n", r.Summary())
		}
		return 0
	case "rules":
		check := false
		event := domain.RuleEventFile
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--json":
			case "--check":
				check = true
			case "--event":
				i++
				if i >= len(args) {
					return 2
				}
				event = domain.RuleEvent(args[i])
				switch event {
				case domain.RuleEventBash, domain.RuleEventFile, domain.RuleEventPrompt, domain.RuleEventStop:
				default:
					return 2
				}
			default:
				return 2
			}
		}
		rules, errs := d.RuleSource.Load(ctx, d.Root)
		if !check {
			if js {
				emit(map[string]any{"rules": rules, "errors": errs})
			} else {
				for _, r := range rules {
					fmt.Fprintf(x.out, "%s (%s)\n", termSafe(r.Name), r.Origin)
				}
				for _, e := range errs {
					fmt.Fprintf(x.out, "%s: %s\n", termSafe(e.Source), termSafe(e.Reason))
				}
			}
			return 0
		}
		raw, err := io.ReadAll(io.LimitReader(x.in, (1<<20)+1))
		if err != nil || len(raw) > 1<<20 || !utf8.Valid(raw) {
			fmt.Fprintln(x.err, "entrada ilegible o demasiado grande")
			return 3
		}
		text := string(raw)
		paths, content := expandDiff(text)
		input := domain.RuleInput{Command: content, FilePaths: paths, Content: content, Prompt: text, LastMessage: text}
		v := domain.EvaluateRules(event, input, rules)
		if js {
			emit(map[string]any{"decision": v.Action, "rule": v.Rule, "reason": v.Reason})
		} else {
			fmt.Fprintf(x.out, "%s %s %s\n", v.Action, termSafe(v.Rule), termSafe(v.Reason))
		}
		switch v.Action {
		case domain.RuleBlock:
			return 2
		case domain.RuleWarn:
			return 1
		}
		return 0
	default:
		fmt.Fprintln(x.err, "uso: mem expand status|on|off|rules|instincts")
		return 2
	}
}

// termSafe depura todo texto que procede del disco (nombres de archivo de
// .gomemory/rules/, estados cacheados) antes de pintarlo (CWE-150, H-03).
func termSafe(s string) string { return domain.StripTerminalControls(s) }

func runExpandInstincts(d *Deps, args []string, x expandIO) int {
	if len(args) == 0 {
		args = []string{"list"}
	}
	if args[0] == "evolve" {
		return runExpandEvolve(d, args[1:], x)
	}
	if d.Instincts == nil {
		fmt.Fprintln(x.err, "instincts no disponible")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch args[0] {
	case "list":
		all, js := false, false
		for _, a := range args[1:] {
			switch a {
			case "--all":
				all = true
			case "--json":
				js = true
			default:
				return 2
			}
		}
		list, err := d.Instincts.List(ctx, all)
		if err != nil {
			fmt.Fprintln(x.err, err)
			return 1
		}
		if js {
			if err := json.NewEncoder(x.out).Encode(list); err != nil {
				return 1
			}
		} else {
			for _, i := range list {
				fmt.Fprintf(x.out, "%s %s %.1f %s %d %s — %s\n", termSafe(i.Slug), i.Kind, i.Confidence, i.Scope, i.Observations, i.Status, termSafe(i.Description))
			}
		}
		return 0
	case "confirm", "reject":
		if len(args) != 2 {
			return 2
		}
		var err error
		if args[0] == "confirm" {
			err = d.Instincts.Confirm(ctx, args[1])
		} else {
			err = d.Instincts.Reject(ctx, args[1])
		}
		if err != nil {
			fmt.Fprintln(x.err, err)
			return 1
		}
		fmt.Fprintf(x.out, "%s: %s\n", args[1], args[0])
		return 0
	default:
		return 2
	}
}

func runExpandEvolve(d *Deps, args []string, x expandIO) int {
	if !x.tty {
		return 2
	}
	if len(args) != 1 {
		return 2
	}
	if d.Socratica == nil || d.Socratica.Generator == nil || d.Instincts == nil {
		fmt.Fprintln(x.err, "No hay harness disponible")
		return 1
	}
	ui := x.ui
	if ui == nil {
		ui = console.NewPlain(x.in, x.out)
	}
	client, err := clikit.SelectSocraticHarness(d.Socratica.Harnesses(), ui, x.in, x.out, func(label, reason string) {
		fmt.Fprint(x.out, domain.ProtectText(fmt.Sprintf("  − %s: %s\n", label, reason)))
	})
	if err != nil {
		if errors.Is(err, console.ErrCanceled) {
			return 3
		}
		fmt.Fprintln(x.err, err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	name, body, err := d.Instincts.Evolve(ctx, d.Socratica.Generator, client, args[0], embeddedTemplate("expand/evolve-prompt.md"))
	if err != nil {
		fmt.Fprintln(x.err, domain.ProtectText(err.Error()))
		return 1
	}
	fmt.Fprintf(x.out, "\n%s\n", domain.StripTerminalControls(body))
	yes, err := ui.Confirm("¿Instalar esta skill en los harnesses presentes?", false)
	if err != nil || !yes {
		return 3
	}
	install := x.install
	if install == nil {
		install = func(name, body string) ([]string, error) { return setup.InstallAgentSkill(name, body) }
	}
	paths, err := install(name, body)
	if err != nil {
		fmt.Fprintln(x.err, err)
		return 1
	}
	for _, p := range paths {
		fmt.Fprintln(x.out, p)
	}
	return 0
}

func expandDiff(s string) ([]string, string) {
	if !strings.Contains(s, "+++ ") && !strings.HasPrefix(s, "diff --git ") {
		return nil, s
	}
	var paths, lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			p := strings.TrimPrefix(strings.TrimPrefix(line[4:], "a/"), "b/")
			if p != "/dev/null" {
				paths = append(paths, p)
			}
		} else if strings.HasPrefix(line, "+") {
			lines = append(lines, line[1:])
		}
	}
	return paths, strings.Join(lines, "\n")
}
