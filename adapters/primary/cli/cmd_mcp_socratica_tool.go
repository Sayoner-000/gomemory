package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"mem/application/usecases"
	"mem/domain"
)

type socraticaInput struct {
	Action     string                    `json:"action"`
	FlowID     string                    `json:"flow_id,omitempty"`
	DispatchID string                    `json:"dispatch_id,omitempty"`
	AckNonce   string                    `json:"ack_nonce,omitempty"`
	Client     string                    `json:"client,omitempty"`
	Mode       string                    `json:"mode,omitempty"`
	Prompt     string                    `json:"prompt,omitempty"`
	Questions  []domain.Question         `json:"questions,omitempty"`
	Answers    []domain.Answer           `json:"answers,omitempty"`
	SkipRest   bool                      `json:"skip_rest,omitempty"`
	Proposed   string                    `json:"proposed,omitempty"`
	Additions  []string                  `json:"additions,omitempty"`
	Error      string                    `json:"error,omitempty"`
	Scrutiny   *domain.ScrutinySynthesis `json:"scrutiny,omitempty"`
	Limit      int                       `json:"limit,omitempty"`
	// mode=specify: the revision the agent read and the preparation operation.
	ExpectedRevision *int                       `json:"expected_revision,omitempty"`
	Preparation      *socraticaPreparationInput `json:"preparation,omitempty"`
	// report: the result the agent communicates for dispatch_id.
	Outcome         *socraticaOutcomeInput `json:"outcome,omitempty"`
	CorrectPrevious bool                   `json:"correct_previous,omitempty"`
}

// socraticaOutcomeInput is the result of a delivery as the agent saw it after
// opening the generated files. It carries no permission and no verdict of the
// backend about the document.
type socraticaOutcomeInput struct {
	Status        string                     `json:"status"`
	SpecPath      string                     `json:"spec_path,omitempty"`
	ChecklistPath string                     `json:"checklist_path,omitempty"`
	Traceability  []socraticaTraceabilityRow `json:"traceability,omitempty"`
	Message       string                     `json:"message,omitempty"`
}

type socraticaTraceabilityRow struct {
	CriterionID   string   `json:"criterion_id"`
	StoryIDs      []string `json:"story_ids"`
	SpecReference string   `json:"spec_reference"`
}

// socraticaPreparationInput carries prepare(set) and prepare(validate). Value is parsed strictly by
// the domain, so computed data (stage, checks, hashes, permissions) is refused.
type socraticaPreparationInput struct {
	Operation string `json:"operation"`
	Section   string `json:"section,omitempty"`
	Value     any    `json:"value,omitempty"`
	Stage     string `json:"stage,omitempty"`
}

func registerSocraticaTool(server *mcp.Server, deps *Deps, project string) {
	addTool(server, &mcp.Tool{Name: domain.ToolSocraticaFlow, Description: "Gestiona refinamiento, escrutinio y preparación para Spec Kit (mode=specify) con Socratica. Requiere aprobación explícita; ack con dispatch_id y ack_nonce antes de ejecutar. En mode=specify propón secciones con prepare(set) y sigue next, stage y allowed_actions calculados por el servidor. No activa intercepción."}, func(ctx context.Context, req *mcp.CallToolRequest, in socraticaInput) (*mcp.CallToolResult, any, error) {
		if deps.Socratica == nil {
			return reviewToolResult(map[string]any{"phase": "unavailable"})
		}
		u := deps.Socratica
		settings, err := u.ValidatedSettings()
		if err != nil {
			return nil, nil, err
		}
		switch in.Action {
		case "settings":
			if !domain.IsSocraticaClient(in.Client) {
				return nil, nil, fmt.Errorf("invalid client")
			}
			return reviewToolResult(map[string]any{"interception": settings.Interception[in.Client], "private": settings.Private, "capabilities": domain.CapabilitiesForSocratica(in.Client)})
		case "history":
			rows, e := u.History(ctx, project, in.Client)
			if e != nil {
				return nil, nil, e
			}
			if in.Limit > 0 && in.Limit < len(rows) {
				rows = rows[:in.Limit]
			}
			return reviewToolResult(map[string]any{"history": rows})
		}
		var result usecases.SocraticaResult
		if in.Action == "start" {
			mode := domain.SocraticaMode(in.Mode)
			if mode == "" {
				mode = domain.ModeRefine
			}
			result, err = u.Start(ctx, project, in.Client, mode, in.Prompt)
			if err == nil && !result.Flow.Recovered && (in.Questions != nil || in.Scrutiny != nil) {
				if mode == domain.ModeSpecify {
					result, err = applySpecifyQuestions(ctx, u, result.Flow.ID, in.Questions)
				} else {
					result, err = applySocraticaQuestions(ctx, u, result.Flow.ID, in.Questions, in.Scrutiny)
				}
			}
		} else {
			if in.FlowID == "" {
				return nil, nil, fmt.Errorf("flow_id is required")
			}
			// Scope every operation to the server's project before touching state.
			current, e := u.Get(ctx, in.FlowID)
			if e != nil {
				return nil, nil, e
			}
			if current.Flow.Project != project {
				return nil, nil, fmt.Errorf("flow belongs to another project")
			}
			if current.Flow.Mode == domain.ModeSpecify {
				switch in.Action {
				// Free proposals and generic retry bypass the prepared content.
				case "rewrite", "edit", "send_original", "retry", "fail":
					return nil, nil, fmt.Errorf("unsupported_specify_action: %s is not available in mode=specify; edit sections with prepare", in.Action)
				}
			} else if in.Action == "prepare" {
				return nil, nil, fmt.Errorf("unsupported_specify_action: prepare only applies to mode=specify")
			}
			ev := domain.Event{Kind: in.Action, Proposed: in.Proposed, Scrutiny: in.Scrutiny, Additions: in.Additions, DispatchID: in.DispatchID, Nonce: in.AckNonce, Error: in.Error}
			switch in.Action {
			case "get":
				result = current
			case "resume":
				result, err = u.Resume(ctx, in.FlowID, usecases.OriginMCP)
			case "prepare":
				result, err = prepareSpecify(ctx, u, current.Flow, in)
			case "answer":
				if current.Flow.Mode == domain.ModeSpecify && current.Flow.Phase == domain.PhaseCaptured {
					result, err = applySpecifyQuestions(ctx, u, in.FlowID, in.Questions)
				} else if current.Flow.Phase == domain.PhaseCaptured {
					result, err = applySocraticaQuestions(ctx, u, in.FlowID, in.Questions, in.Scrutiny)
				} else if in.SkipRest {
					result, err = u.Apply(ctx, in.FlowID, domain.Event{Kind: "skip_rest"}, usecases.OriginMCP)
				} else {
					if len(in.Answers) == 0 {
						return nil, nil, fmt.Errorf("answer is required")
					}
					for _, answer := range in.Answers {
						result, err = u.Apply(ctx, in.FlowID, domain.Event{Kind: "answer", Answer: answer}, usecases.OriginMCP)
						if err != nil {
							break
						}
					}
				}
			case "rewrite":
				payload, e := json.Marshal(domain.GenerationResult{Proposed: in.Proposed, Additions: in.Additions, Scrutiny: in.Scrutiny})
				if e != nil {
					return nil, nil, e
				}
				g, e := domain.ParseGeneration(payload)
				if e != nil {
					return nil, nil, e
				}
				ev.Kind = "rewrite_ready"
				ev.Scrutiny = g.Scrutiny
				result, err = u.Apply(ctx, in.FlowID, ev, usecases.OriginMCP)
			case "report":
				if current.Flow.Mode != domain.ModeSpecify {
					return nil, nil, fmt.Errorf("unsupported_specify_action: report only applies to mode=specify")
				}
				if in.DispatchID == "" || in.Outcome == nil {
					return nil, nil, fmt.Errorf("dispatch_id and outcome are required")
				}
				outcome := domain.SpecifyOutcome{Status: in.Outcome.Status, SpecPath: in.Outcome.SpecPath, ChecklistPath: in.Outcome.ChecklistPath, Message: in.Outcome.Message}
				for _, row := range in.Outcome.Traceability {
					outcome.Traceability = append(outcome.Traceability, domain.SpecifyTrace{CriterionID: row.CriterionID, StoryIDs: row.StoryIDs, SpecReference: row.SpecReference})
				}
				result, err = u.ReportOutcome(ctx, in.FlowID, in.DispatchID, outcome, in.CorrectPrevious)
			case "ack":
				if in.DispatchID == "" || in.AckNonce == "" {
					return nil, nil, fmt.Errorf("dispatch_id and ack_nonce are required")
				}
				result, err = u.Apply(ctx, in.FlowID, ev, usecases.OriginMCP)
			case "force_resend", "mark_dispatched":
				if in.DispatchID == "" || in.DispatchID != current.Flow.DispatchID {
					return nil, nil, fmt.Errorf("dispatch_id does not match")
				}
				result, err = u.Apply(ctx, in.FlowID, ev, usecases.OriginMCP)
			case "edit":
				ev.Proposed = in.Prompt
				result, err = u.Apply(ctx, in.FlowID, ev, usecases.OriginMCP)
			case "approve":
				if current.Flow.Mode == domain.ModeSpecify {
					// The person approves the version that was shown: the revision is mandatory.
					if in.ExpectedRevision == nil {
						return nil, nil, fmt.Errorf("expected_revision is required")
					}
					result, err = u.ApproveSpecify(ctx, in.FlowID, *in.ExpectedRevision)
					if errors.Is(err, domain.ErrRevisionConflict) {
						err = fmt.Errorf("%w: the current revision is %d; read the flow with get, show it again and retry", err, current.Flow.Revision)
					}
					break
				}
				result, err = u.Apply(ctx, in.FlowID, ev, usecases.OriginMCP)
			case "cancel", "fail", "retry", "send_original":
				result, err = u.Apply(ctx, in.FlowID, ev, usecases.OriginMCP)
			default:
				return nil, nil, fmt.Errorf("unsupported socratica action")
			}
		}
		if err != nil {
			return nil, nil, err
		}
		return reviewToolResult(socraticaOutput(result))
	})
}

// applySocraticaQuestions validates the agent's questions with the same parser
// as the local generators (SC-004); an irreparable payload moves the flow to
// failed with the original intact.
func applySocraticaQuestions(ctx context.Context, u *usecases.SocraticaFlow, id string, questions []domain.Question, scrutiny *domain.ScrutinySynthesis) (usecases.SocraticaResult, error) {
	raw, err := json.Marshal(domain.GenerationResult{Questions: questions, Scrutiny: scrutiny})
	if err != nil {
		return usecases.SocraticaResult{}, err
	}
	g, err := domain.ParseGeneration(raw)
	if err != nil {
		_, _ = u.Apply(ctx, id, domain.Event{Kind: "fail", Error: "invalid generation"}, usecases.OriginMCP)
		return usecases.SocraticaResult{}, err
	}
	return u.Apply(ctx, id, domain.Event{Kind: "questions_generated", Questions: g.Questions, Scrutiny: g.Scrutiny}, usecases.OriginMCP)
}

// applySpecifyQuestions registers questions of a specify flow. A malformed
// payload is rejected without failing the flow: there is no generator to retry,
// and the draft must survive.
func applySpecifyQuestions(ctx context.Context, u *usecases.SocraticaFlow, id string, questions []domain.Question) (usecases.SocraticaResult, error) {
	raw, err := json.Marshal(domain.GenerationResult{Questions: questions})
	if err != nil {
		return usecases.SocraticaResult{}, err
	}
	g, err := domain.ParseGeneration(raw)
	if err != nil {
		return usecases.SocraticaResult{}, err
	}
	return u.Apply(ctx, id, domain.Event{Kind: "questions_generated", Questions: g.Questions}, usecases.OriginMCP)
}

// prepareSpecify runs the prepare operations of a specify flow.
func prepareSpecify(ctx context.Context, u *usecases.SocraticaFlow, current domain.RefinementFlow, in socraticaInput) (usecases.SocraticaResult, error) {
	if in.Preparation == nil {
		return usecases.SocraticaResult{}, fmt.Errorf("preparation is required")
	}
	if in.ExpectedRevision == nil && (in.Preparation.Operation == "set" || in.Preparation.Operation == "validate") {
		return usecases.SocraticaResult{}, fmt.Errorf("expected_revision is required")
	}
	switch in.Preparation.Operation {
	case "validate":
		// Gomemory computes validity and hashes; the request only names the stage.
		if in.Preparation.Section != "" || in.Preparation.Value != nil {
			return usecases.SocraticaResult{}, fmt.Errorf("unsupported_specify_action: validate takes only stage")
		}
		result, err := u.PrepareValidate(ctx, current.ID, *in.ExpectedRevision, domain.SpecifyStage(in.Preparation.Stage))
		if errors.Is(err, domain.ErrRevisionConflict) {
			err = fmt.Errorf("%w: the current revision is %d; read the flow with get and retry from it", err, current.Revision)
		}
		return result, err
	case "set":
		value, err := json.Marshal(in.Preparation.Value)
		if err != nil {
			return usecases.SocraticaResult{}, err
		}
		result, err := u.PrepareSet(ctx, current.ID, *in.ExpectedRevision, domain.SpecifyStage(in.Preparation.Section), value)
		if errors.Is(err, domain.ErrRevisionConflict) {
			err = fmt.Errorf("%w: the current revision is %d; read the flow with get and retry from it", err, current.Revision)
		}
		return result, err
	}
	return usecases.SocraticaResult{}, fmt.Errorf("unsupported_specify_action: preparation operation %q", in.Preparation.Operation)
}

func socraticaOutput(r usecases.SocraticaResult) map[string]any {
	out := socraticaBaseOutput(r)
	if f := r.Flow; f.Mode == domain.ModeSpecify {
		view := usecases.SpecifyView(f)
		var reviewHash, outcome any
		if view.ReviewHash != "" {
			reviewHash = view.ReviewHash
		}
		if f.Preparation != nil && f.Preparation.Outcome != nil {
			outcome = f.Preparation.Outcome
		}
		out["mode"], out["revision"], out["stage"], out["preparation"] = f.Mode, f.Revision, view.Stage, f.Preparation
		out["pending_checks"], out["allowed_actions"], out["questions_used"] = view.PendingChecks, view.AllowedActions, view.QuestionsUsed
		out["review_hash"], out["execution_granted"], out["outcome"], out["private_state_unavailable"] = reviewHash, f.ExecutionGranted, outcome, f.PrivateStateUnavailable
	}
	return out
}
func socraticaBaseOutput(r usecases.SocraticaResult) map[string]any {
	f := r.Flow
	return map[string]any{"flow_id": f.ID, "phase": f.Phase, "original": f.OriginalPrompt, "proposed": f.ProposedPrompt, "approved_prompt": f.ApprovedPrompt, "questions": f.Questions, "answers": f.Answers, "dispatch_id": f.DispatchID, "duplicate": f.Duplicate, "acked": f.Acked, "already_dispatched": f.AlreadyDispatched, "recovered": f.Recovered, "recoverable": usecases.SocraticaRecoverable(f) && !f.InFlight, "uncertain": f.Uncertain, "in_flight": f.InFlight, "summary": map[string]any{"words_before": domain.WordCount(f.OriginalPrompt), "words_after": domain.WordCount(f.ProposedPrompt), "additions": f.Additions}, "scrutiny": f.Scrutiny, "next": r.Next}
}
