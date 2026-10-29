package hcd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mem/adapters/primary/cli/clikit"
	"mem/application/usecases"
	"mem/domain"
	"strings"
)

var (
	commands = map[string]bool{"start": true, "show": true, "next": true, "clarify": true, "approve": true, "verify": true, "analyze": true, "close": true, "cancel": true, "promote": true}
	clients  = map[string]bool{"terminal": true, "claude": true, "codex": true, "opencode": true, "tui": true}
	// Flags que admite cada comando, además de --json y --client.
	allowedFlags = map[string]map[string]bool{
		"start":   {"kind": true, "lane": true, "title": true, "why": true, "intent": true},
		"show":    {},
		"next":    {},
		"clarify": {"ask": true, "id": true, "answer": true},
		"approve": {"quote": true},
		"verify":  {"task": true, "expect-fail": true, "manual": true},
		"analyze": {},
		"close":   {"quote": true},
		"cancel":  {"quote": true},
		"promote": {},
	}
	boolFlags = map[string]bool{"json": true, "expect-fail": true}
)

type parsed struct {
	command, id, client, run string
	flags                    map[string]string
	json                     bool
}

func parse(args []string) (parsed, error) {
	p := parsed{command: args[0], flags: map[string]string{}, client: "terminal"}
	rest := args[1:]
	for i, a := range rest {
		if a == "--" {
			p.run = strings.Join(rest[i+1:], " ")
			rest = rest[:i]
			break
		}
	}
	for _, a := range rest {
		if a == "--json" {
			p.json = true
		}
	}
	if !commands[p.command] {
		return p, fmt.Errorf("comando desconocido: %s", p.command)
	}
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if !strings.HasPrefix(a, "--") {
			if p.id != "" {
				return p, fmt.Errorf("solo se admite una unidad")
			}
			p.id = a
			continue
		}
		key, value, has := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		if key != "json" && key != "client" && !allowedFlags[p.command][key] {
			return p, fmt.Errorf("flag no admitido en %s: --%s", p.command, key)
		}
		if !has && !boolFlags[key] {
			if i+1 >= len(rest) {
				return p, fmt.Errorf("falta valor: --%s", key)
			}
			i++
			value = rest[i]
		}
		if key == "client" {
			if !clients[value] {
				return p, fmt.Errorf("cliente inválido: %s", value)
			}
			p.client = value
			continue
		}
		if boolFlags[key] && !has {
			value = "true"
		}
		p.flags[key] = value
	}
	if p.run != "" && p.command != "verify" {
		return p, fmt.Errorf("solo verify admite un comando tras --")
	}
	return p, nil
}

func execute(deps *clikit.Deps, args []string, out, errOut io.Writer) int {
	p, err := parse(args)
	if err != nil {
		return emitError(out, errOut, p.json, "usage", err.Error())
	}
	if deps == nil || deps.HCD == nil {
		return emitError(out, errOut, p.json, "internal", "HCD no está conectado")
	}
	uc, ctx := deps.HCD, context.Background()
	var view usecases.HCDView
	var findings []domain.HCDFinding
	data := map[string]any{}
	switch p.command {
	case "start":
		view, err = uc.Start(ctx, usecases.HCDStartInput{Kind: p.flags["kind"], Lane: p.flags["lane"], Title: p.flags["title"], Why: p.flags["why"], Intent: p.flags["intent"], Client: p.client})
	case "show", "next":
		view, err = uc.Show(ctx, p.id)
	case "clarify":
		var qid string
		view, qid, err = uc.Clarify(ctx, usecases.HCDClarifyInput{ID: p.id, Ask: p.flags["ask"], Question: p.flags["id"], Answer: p.flags["answer"]})
		data["clarification"] = qid
	case "approve":
		view, err = uc.Approve(ctx, usecases.HCDApproveInput{ID: p.id, Quote: p.flags["quote"], Client: p.client})
	case "analyze":
		view, findings, err = uc.Analyze(ctx, p.id)
	case "verify":
		var ev domain.Evidence
		view, ev, err = uc.Verify(ctx, usecases.HCDVerifyInput{ID: p.id, Tasks: splitList(p.flags["task"]), Command: p.run, Manual: p.flags["manual"], ExpectFail: p.flags["expect-fail"] == "true"})
		if err == nil {
			data["evidence"] = map[string]any{"id": ev.ID, "tasks": ev.Tasks, "kind": ev.Kind, "expect_fail": ev.ExpectFail, "exit_code": ev.ExitCode, "passed": ev.Passed(), "red_observed": ev.RedObserved(), "excerpt": ev.Excerpt}
		}
	case "close":
		view, err = uc.Close(ctx, usecases.HCDCloseInput{ID: p.id, Quote: p.flags["quote"], Client: p.client})
	case "cancel":
		view, err = uc.Cancel(ctx, usecases.HCDCloseInput{ID: p.id, Quote: p.flags["quote"], Client: p.client})
	case "promote":
		view, err = uc.Promote(ctx, p.id)
	}
	if err != nil {
		return emitError(out, errOut, p.json, errorCode(err), err.Error())
	}
	response := toResponse(view, findings, data)
	if p.json {
		if e := emitJSON(out, response); e != nil {
			return 1
		}
		return 0
	}
	printText(out, response)
	return 0
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrInvalidQuote):
		return "invalid_quote"
	case errors.Is(err, domain.ErrNotAllowed):
		return "not_allowed"
	case errors.Is(err, domain.ErrValidation):
		return "validation"
	default:
		return "internal"
	}
}

func toResponse(v usecases.HCDView, findings []domain.HCDFinding, data map[string]any) Response {
	r := Response{ProtocolVersion: ProtocolVersion, OK: true, UnitID: v.Unit.ID, Kind: string(v.Unit.Kind), Lane: string(v.Unit.Lane), State: string(v.Unit.State), Stage: string(v.Stage), DocPath: v.Unit.DocPath, DefinitionRev: v.DefinitionRevision, WorkRev: v.WorkRevision, Approved: v.Approved, Fingerprint: v.State.Fingerprint, RedMissing: v.RedMissing, NextAction: NextAction{Skill: v.Next.Skill, Command: v.Next.Command, Why: v.Next.Why}, Warnings: v.Warnings, Data: data}
	for _, t := range v.Tasks {
		r.Tasks = append(r.Tasks, Task{ID: t.ID, Text: t.Text, Check: t.Check, Status: string(t.Status)})
	}
	for _, f := range findings {
		r.Findings = append(r.Findings, Finding{Severity: string(f.Severity), Code: f.Code, Ref: f.Ref, Message: f.Message})
	}
	for _, c := range v.State.Pending {
		r.PendingChecks = append(r.PendingChecks, PendingCheck{Code: c.Code, Field: c.Field, Message: c.Message})
	}
	return r
}

func printText(w io.Writer, r Response) {
	fmt.Fprintf(w, "%s · %s · %s · paso %s · %s\n", r.UnitID, r.Kind, r.Lane, r.Stage, r.DocPath)
	for _, c := range r.PendingChecks {
		fmt.Fprintf(w, "  falta: %s\n", c.Message)
	}
	for _, t := range r.Tasks {
		fmt.Fprintf(w, "  [%s] %s. %s\n", t.Status, t.ID, t.Text)
	}
	for _, f := range r.Findings {
		fmt.Fprintf(w, "  %s %s %s: %s\n", f.Severity, f.Code, f.Ref, f.Message)
	}
	if len(r.RedMissing) > 0 {
		fmt.Fprintf(w, "  rojo pendiente: %s\n", strings.Join(r.RedMissing, ", "))
	}
	for _, warning := range r.Warnings {
		fmt.Fprintf(w, "  aviso: %s\n", warning)
	}
	fmt.Fprintf(w, "Siguiente: /%s — %s\n", r.NextAction.Skill, r.NextAction.Why)
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
