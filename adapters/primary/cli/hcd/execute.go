package hcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mem/adapters/primary/cli/clikit"
	"mem/application/usecases"
	"mem/domain"
	"strconv"
	"strings"
)

func execute(deps *clikit.Deps, args []string, out, errOut io.Writer) int {
	playbookAction := "show"
	if len(args) > 1 && args[0] == "playbook" && !strings.HasPrefix(args[1], "--") {
		playbookAction = args[1]
		args = append([]string{"playbook"}, args[2:]...)
		if playbookAction != "show" && playbookAction != "validate" && playbookAction != "adopt" {
			return emitError(out, errOut, true, "usage", "acción playbook inválida")
		}
	}
	if len(args) > 1 && args[0] == "memory" && args[1] == "sync" {
		args = append([]string{"memory"}, args[2:]...)
	}
	jsonMode := false
	for _, a := range args {
		if a == "--json" {
			jsonMode = true
		}
	}
	flags := map[string]string{}
	id := ""
	bools := map[string]bool{"json": true, "open": true, "blocking": true, "request": true, "clear": true, "critical": true, "reject": true, "preview": true, "activate": true}
	allowed := map[string]bool{}
	for _, s := range strings.Fields("json client expected-revision operation-id kind title implementer session file open blocking target question options recommendation answer choice revision quote object prompt-id request clear slice task run resolve reason resolution critical reject superseded-by timeout accept supersede by files approach doc actor preview activate") {
		allowed[s] = true
	}
	for i := 1; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			if id != "" {
				return emitError(out, errOut, jsonMode, "usage", "solo se admite una unidad")
			}
			id = a
			continue
		}
		key, value, has := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		if !allowed[key] {
			return emitError(out, errOut, jsonMode, "usage", "flag desconocido: "+key)
		}
		if !has {
			if bools[key] {
				value = "true"
			} else {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
					return emitError(out, errOut, jsonMode, "usage", "falta valor: "+key)
				}
				i++
				value = args[i]
			}
		}
		flags[key] = value
	}
	command := args[0]
	if !strings.Contains(" start show resume next define validate clarify approve focus slice tasks check run report block list verify status close cancel memory heartbeat decision classify exception playbook migrate subject ", " "+command+" ") {
		return emitError(out, errOut, jsonMode, "usage", "comando aún no disponible: "+command)
	}
	if deps == nil || deps.HCD == nil {
		return emitError(out, errOut, jsonMode, "internal", "HCD no está conectado")
	}
	uc := deps.HCD
	ctx := context.Background()
	client := flags["client"]
	if client == "" {
		client = "terminal"
	}
	if !strings.Contains(" terminal claude codex opencode tui ", " "+client+" ") {
		return emitError(out, errOut, jsonMode, "usage", "cliente desconocido")
	}
	fail := func(err error) int {
		code := hcdErrorCode(err)
		var fields *domain.HCDValidationError
		if jsonMode && errors.As(err, &fields) {
			r := Response{ProtocolVersion: 1, OK: false, Error: &Error{Code: code, Message: fields.Error()}}
			for _, c := range fields.Checks {
				r.PendingChecks = append(r.PendingChecks, PendingCheck{Code: c.Code, Ref: c.Field, Detail: c.Message})
			}
			if emitJSON(out, r) != nil {
				return 1
			}
			return ExitCode(code)
		}
		return emitError(out, errOut, jsonMode, code, code)
	}

	var data any
	var err error
	if command == "heartbeat" {
		var files []string
		if flags["files"] != "" {
			files = strings.Split(flags["files"], ",")
		}
		if e := uc.Heartbeat(ctx, usecases.HCDHeartbeatInput{Client: client, SessionID: flags["session"], Files: files}); e != nil {
			return fail(e)
		}
		if jsonMode {
			if emitJSON(out, Response{ProtocolVersion: 1, OK: true}) != nil {
				return 1
			}
		}
		return 0
	}
	if command == "list" {
		units, e := uc.List(ctx)
		if e != nil {
			return fail(e)
		}
		if jsonMode {
			if emitJSON(out, Response{ProtocolVersion: 1, OK: true, Data: units}) != nil {
				return 1
			}
		} else {
			for _, u := range units {
				if _, e = fmt.Fprintf(out, "%s · %s · %s\n", u.ID, domain.StripTerminalControls(u.Title), u.State); e != nil {
					return 1
				}
			}
		}
		return 0
	}
	if command == "subject" {
		var files []string
		for _, f := range strings.Split(flags["files"], ",") {
			if f = strings.TrimSpace(f); f != "" {
				files = append(files, f)
			}
		}
		if len(files) == 0 {
			return emitError(out, errOut, jsonMode, "usage", "uso: subject --files a.go,b_test.go")
		}
		ref, e := uc.SubjectRef(ctx, files)
		if e != nil {
			return fail(e)
		}
		if jsonMode {
			if emitJSON(out, Response{ProtocolVersion: 1, OK: true, Data: map[string]string{"subject_ref": ref}}) != nil {
				return 1
			}
		} else if _, e = fmt.Fprintln(out, ref); e != nil {
			return 1
		}
		return 0
	}
	if command == "focus" {
		data, err = uc.Focus(ctx, id, client, flags["session"], flags["clear"] == "true")
		if err != nil {
			return fail(err)
		}
		if jsonMode {
			if emitJSON(out, Response{ProtocolVersion: 1, OK: true, Data: data}) != nil {
				return 1
			}
		} else {
			_, err = fmt.Fprintln(out, data)
		}
		if err != nil {
			return 1
		}
		return 0
	}
	if command == "migrate" {
		if (flags["preview"] == "true") == (flags["activate"] == "true") || id == "" {
			return emitError(out, errOut, jsonMode, "usage", "uso: migrate --preview <carpeta specs> | --activate <M-NN> --revision <rev> --quote <texto>")
		}
		if flags["preview"] == "true" {
			m, e := uc.MigratePreview(ctx, id)
			if e != nil {
				return fail(e)
			}
			if jsonMode {
				if emitJSON(out, Response{ProtocolVersion: 1, OK: true, Approach: "speckit", NextAction: NextAction{Skill: "hcd-migrate", Why: "revisar la propuesta " + m.ID + " y pedir aprobación citada de la revisión " + m.Revision}, Data: m}) != nil {
					return 1
				}
			} else if _, e = fmt.Fprintf(out, "%s · %s · revisión %s · %v\n", m.ID, m.SourceDir, m.Revision, m.Counts); e != nil {
				return 1
			}
			return 0
		}
		u, e := uc.MigrateActivate(ctx, usecases.HCDMigrateActivateInput{MigrationID: id, Revision: flags["revision"], Quote: flags["quote"], Client: client, Implementer: flags["implementer"], Kind: domain.Kind(flags["kind"])})
		if e != nil {
			return fail(e)
		}
		id = u.ID
		data = u
	} else if command == "classify" {
		u, e := uc.Classify(ctx, id, flags["approach"], flags["quote"], client)
		if e != nil {
			return fail(e)
		}
		id = u.ID
		data = u
	} else if command == "start" {
		u, e := uc.Start(ctx, usecases.HCDStartInput{Kind: domain.Kind(flags["kind"]), Title: flags["title"], Implementer: flags["implementer"], Client: client, SessionID: flags["session"]})
		if e != nil {
			return fail(e)
		}
		id = u.ID
		data = u
	} else if id == "" {
		id, err = uc.ActiveUnitID(ctx)
		if err != nil {
			return emitError(out, errOut, jsonMode, "usage", "no hay unidad activa")
		}
	}
	expected := int64(0)
	if command == "define" || command == "clarify" || (command == "approve" || command == "slice" || command == "tasks" || command == "run" || command == "report" || command == "block" || command == "verify" || command == "close" || command == "cancel" || command == "decision" || command == "exception" || (command == "playbook" && playbookAction == "adopt")) {
		expected, err = strconv.ParseInt(flags["expected-revision"], 10, 64)
		if err != nil || expected < 1 {
			return emitError(out, errOut, jsonMode, "usage", "se requiere --expected-revision positivo")
		}
	}
	switch command {
	case "memory":
		err = uc.MemorySync(ctx, id)
	case "decision":
		input := usecases.HCDDecisionInput{UnitID: id, ExpectedRevision: expected, OperationID: flags["operation-id"], Quote: flags["quote"], Client: client}
		if flags["accept"] != "" {
			input.Action = "accept"
			input.ID = flags["accept"]
		} else if flags["supersede"] != "" {
			input.Action = "supersede"
			input.ID = flags["supersede"]
			input.By = flags["by"]
		} else {
			content, e := readFileFlag(flags["file"])
			if e != nil {
				return emitError(out, errOut, jsonMode, "usage", "no se pudo leer --file")
			}
			var decision domain.HCDDecision
			if json.Unmarshal(content, &decision) != nil {
				for _, line := range strings.Split(string(content), "\n") {
					k, v, ok := strings.Cut(line, ":")
					if !ok {
						continue
					}
					v = strings.TrimSpace(v)
					switch strings.TrimSpace(k) {
					case "Tipo":
						decision.Kind = v
					case "Resumen":
						decision.Summary = v
					case "Referencias":
						for _, ref := range strings.Split(v, ",") {
							if ref = strings.TrimSpace(ref); ref != "" {
								decision.Refs = append(decision.Refs, ref)
							}
						}
					}
				}
			}
			input.Action = "propose"
			input.Kind = decision.Kind
			input.Summary = decision.Summary
			input.Refs = decision.Refs
		}
		data, err = uc.Decision(ctx, input)
	case "status":
		data, err = uc.Status(ctx, id)
	case "exception":
		content, e := readFileFlag(flags["file"])
		if e != nil {
			return emitError(out, errOut, jsonMode, "usage", "no se pudo leer --file")
		}
		var x domain.Exception
		if e = json.Unmarshal(content, &x); e != nil {
			return emitError(out, errOut, jsonMode, "usage", "excepción JSON inválida")
		}
		data, err = uc.Exception(ctx, usecases.HCDExceptionInput{UnitID: id, ExpectedRevision: expected, OperationID: flags["operation-id"], Quote: flags["quote"], Client: client, Exception: x})
	case "playbook":
		doc := flags["doc"]
		if doc == "" {
			doc = "playbook"
		}
		if playbookAction == "adopt" {
			data, err = uc.PlaybookAdopt(ctx, usecases.HCDAdoptInput{UnitID: id, Doc: doc, ExpectedRevision: expected, OperationID: flags["operation-id"], Quote: flags["quote"], Client: client, Revision: flags["revision"], Reason: flags["reason"], Actor: flags["actor"]})
		} else if playbookAction == "validate" {
			data, err = uc.PlaybookValidate(ctx, id)
		} else {
			data, err = uc.PlaybookShow(ctx, id, doc)
		}
	case "verify":
		data, err = uc.Verify(ctx, id, expected, flags["operation-id"], flags["slice"])
	case "close", "cancel":
		input := usecases.HCDCloseInput{UnitID: id, ExpectedRevision: expected, OperationID: flags["operation-id"], Revision: flags["revision"], Quote: flags["quote"], Client: client, Reason: flags["reason"], Reject: flags["reject"] == "true", SupersededBy: flags["superseded-by"]}
		if command == "close" {
			data, err = uc.Close(ctx, input)
		} else {
			data, err = uc.Cancel(ctx, input)
		}
	case "slice", "tasks":
		content, e := readFileFlag(flags["file"])
		if e != nil {
			return emitError(out, errOut, jsonMode, "usage", "no se pudo leer --file")
		}
		d, e := uc.ParseDelivery(content)
		if e != nil {
			return fail(e)
		}
		if command == "slice" {
			data, err = uc.Slice(ctx, id, expected, flags["operation-id"], d.Slices)
		} else {
			data, err = uc.Tasks(ctx, id, expected, flags["operation-id"], flags["slice"], d.Tasks)
		}
	case "check":
		admission, e := uc.Check(ctx, id, flags["task"])
		if e != nil {
			return fail(e)
		}
		data = admission
		if len(admission.Checks) > 0 {
			v, e := uc.Show(ctx, id)
			if e != nil {
				return fail(e)
			}
			r := viewResponse(v)
			r.OK = false
			r.Error = &Error{Code: "atomicity", Message: "la tarea no está admitida"}
			r.PendingChecks = nil
			for _, c := range admission.Checks {
				r.PendingChecks = append(r.PendingChecks, PendingCheck{Code: c.Code, Ref: c.Field, Detail: c.Message})
			}
			r.Data = admission
			if jsonMode {
				if emitJSON(out, r) != nil {
					return 1
				}
			} else {
				_, _ = fmt.Fprintln(errOut, "la tarea no está admitida")
			}
			return 5
		}
	case "run":
		var timeout []int
		if flags["timeout"] != "" {
			hours, e := strconv.Atoi(flags["timeout"])
			if e != nil || hours < 1 {
				return emitError(out, errOut, jsonMode, "usage", "timeout requiere horas positivas")
			}
			timeout = []int{hours}
		}
		data, err = uc.Run(ctx, id, expected, flags["operation-id"], flags["task"], client, timeout...)
	case "report":
		content, e := readFileFlag(flags["file"])
		if e != nil {
			return emitError(out, errOut, jsonMode, "usage", "no se pudo leer --file")
		}
		ev, e := uc.ParseEvidence(content)
		if e != nil {
			return fail(e)
		}
		data, err = uc.Report(ctx, id, expected, flags["operation-id"], flags["run"], ev)
	case "block":
		target, reason, resolution, critical := flags["target"], flags["reason"], flags["resolution"], flags["critical"] == "true"
		if flags["file"] != "" {
			content, e := readFileFlag(flags["file"])
			if e != nil {
				return emitError(out, errOut, jsonMode, "usage", "no se pudo leer --file")
			}
			for _, line := range strings.Split(string(content), "\n") {
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				v = strings.TrimSpace(v)
				switch strings.TrimSpace(k) {
				case "Objetivo":
					target = v
				case "Motivo":
					reason = v
				case "Resolución":
					resolution = v
				case "Crítico":
					critical = v == "true"
				}
			}
		}
		data, err = uc.Block(ctx, usecases.HCDBlockInput{UnitID: id, ExpectedRevision: expected, OperationID: flags["operation-id"], ResolveID: flags["resolve"], TargetRef: target, Reason: reason, Resolution: resolution, Critical: critical})
	case "define":
		var content []byte
		content, err = readFileFlag(flags["file"])
		if err != nil {
			return emitError(out, errOut, jsonMode, "usage", "no se pudo leer --file")
		}
		var d domain.Definition
		d, err = uc.ParseDefinition(content)
		if err == nil {
			data, err = uc.Define(ctx, id, expected, flags["operation-id"], d, content)
		}
	case "clarify":
		action := "open"
		if flags["answer"] != "" {
			action = "answer"
		}
		data, err = uc.Clarify(ctx, usecases.HCDClarifyInput{UnitID: id, ExpectedRevision: expected, OperationID: flags["operation-id"], Action: action, TargetRef: flags["target"], Question: flags["question"], Options: strings.Split(flags["options"], "|"), Recommendation: flags["recommendation"], Blocking: flags["blocking"] == "true", ClarificationID: flags["answer"], Choice: flags["choice"]})
	case "approve":
		if client == "tui" {
			return fail(domain.ErrNotAllowed)
		}
		var d domain.Definition
		d, err = uc.ReadDefinition(ctx, id)
		if err == nil {
			object := flags["object"]
			if object == "" {
				object = "definition"
			}
			if flags["request"] == "true" {
				revision := flags["revision"]
				if revision == "" {
					view, e := uc.Show(ctx, id)
					if e != nil {
						return fail(e)
					}
					revision = view.Unit.DefinitionRev
				}
				var opts []string
				if flags["options"] != "" {
					opts = strings.Split(flags["options"], "|")
				}
				var prompt domain.ApprovalPrompt
				prompt, err = uc.RequestApproval(ctx, id, expected, object, revision, d, opts...)
				data = map[string]any{"prompt_id": prompt.ID, "revision": prompt.Revision, "options": prompt.Options}
			} else {
				data, err = uc.Approve(ctx, usecases.HCDApproveInput{UnitID: id, ExpectedRevision: expected, OperationID: flags["operation-id"], Object: object, Revision: flags["revision"], Quote: flags["quote"], Client: client, PromptID: flags["prompt-id"], Definition: d})
			}
		}
	}
	if err != nil {
		return fail(err)
	}
	var view usecases.HCDView
	if command == "resume" {
		view, err = uc.Resume(ctx, id, client, flags["session"])
	} else {
		view, err = uc.Show(ctx, id)
	}
	if err != nil {
		return fail(err)
	}
	response := viewResponse(view)
	response.OperationID = flags["operation-id"]
	if r, ok := data.(usecases.HCDRunResult); ok {
		response.ExecutionID = r.Execution.ID
	}
	if data != nil {
		response.Data = data
	} else {
		response.Data = view
	}
	if jsonMode {
		err = emitJSON(out, response)
	} else {
		_, err = fmt.Fprintf(out, "%s · %s · %s · revisión %d\nSiguiente: %s (%s)\n", id, response.State, response.Stage, response.Revision.State, response.NextAction.Skill, response.NextAction.Why)
	}
	if err != nil {
		return 1
	}
	return 0
}
func viewResponse(v usecases.HCDView) Response {
	u := v.Unit
	r := Response{ProtocolVersion: 1, OK: true, UnitID: u.ID, Approach: u.Approach, State: string(u.State), Stage: string(v.Stage), Revision: Revision{State: u.StateRevision, Definition: u.DefinitionRev, Delivery: u.DeliveryRev, Approved: u.ApprovedRev}, PlaybookRevision: u.PlaybookRev, AllowedActions: v.AllowedActions, NextAction: NextAction{Skill: v.NextAction.Skill, Command: v.NextAction.Command, Why: v.NextAction.Why}, MemorySyncStatus: "synced"}
	for _, d := range v.Decisions {
		if d.MemorySync == "pending" {
			r.MemorySyncStatus = "pending"
		}
	}
	for _, c := range v.PendingChecks {
		r.PendingChecks = append(r.PendingChecks, PendingCheck{Code: c.Code, Ref: c.Field, Detail: c.Message})
	}
	return r
}
func hcdErrorCode(err error) string {
	for _, c := range []struct {
		err  error
		code string
	}{{domain.ErrRevisionConflict, "revision_conflict"}, {domain.ErrOperationConflict, "operation_conflict"}, {domain.ErrNoExecutor, "no_executor"}, {domain.ErrAtomicity, "atomicity"}, {domain.ErrUncertain, "uncertain"}, {domain.ErrNotFound, "not_found"}, {domain.ErrValidation, "usage"}, {domain.ErrNotAllowed, "not_allowed"}, {domain.ErrBlocked, "blocked"}, {domain.ErrInvalidQuote, "invalid_quote"}, {domain.ErrAlreadyExists, "already_approved"}, {domain.ErrApproachMismatch, "approach_mismatch"}, {domain.ErrConstitutionConflict, "constitution_conflict"}} {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return "internal"
}
