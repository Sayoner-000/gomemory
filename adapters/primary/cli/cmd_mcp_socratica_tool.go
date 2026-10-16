package cli

import (
	"context"
	"encoding/json"
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
}

func registerSocraticaTool(server *mcp.Server, deps *Deps, project string) {
	addTool(server, &mcp.Tool{Name: domain.ToolSocraticaFlow, Description: "Gestiona refinamiento y escrutinio Socratica. Requiere aprobación explícita; ack con dispatch_id y ack_nonce antes de ejecutar. No activa intercepción."}, func(ctx context.Context, req *mcp.CallToolRequest, in socraticaInput) (*mcp.CallToolResult, any, error) {
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
				result, err = applySocraticaQuestions(ctx, u, result.Flow.ID, in.Questions, in.Scrutiny)
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
			ev := domain.Event{Kind: in.Action, Proposed: in.Proposed, Scrutiny: in.Scrutiny, Additions: in.Additions, DispatchID: in.DispatchID, Nonce: in.AckNonce, Error: in.Error}
			switch in.Action {
			case "get":
				result = current
			case "resume":
				result, err = u.Resume(ctx, in.FlowID, usecases.OriginMCP)
			case "answer":
				if current.Flow.Phase == domain.PhaseCaptured {
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
			case "approve", "cancel", "fail", "retry", "send_original":
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
func socraticaOutput(r usecases.SocraticaResult) map[string]any {
	f := r.Flow
	return map[string]any{"flow_id": f.ID, "phase": f.Phase, "original": f.OriginalPrompt, "proposed": f.ProposedPrompt, "approved_prompt": f.ApprovedPrompt, "questions": f.Questions, "answers": f.Answers, "dispatch_id": f.DispatchID, "duplicate": f.Duplicate, "acked": f.Acked, "already_dispatched": f.AlreadyDispatched, "recovered": f.Recovered, "recoverable": usecases.SocraticaRecoverable(f) && !f.InFlight, "uncertain": f.Uncertain, "in_flight": f.InFlight, "summary": map[string]any{"words_before": domain.WordCount(f.OriginalPrompt), "words_after": domain.WordCount(f.ProposedPrompt), "additions": f.Additions}, "scrutiny": f.Scrutiny, "next": r.Next}
}
