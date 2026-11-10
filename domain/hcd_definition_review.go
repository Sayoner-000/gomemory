package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type HCDReviewOperationID string
type HCDReviewRevision string

type HCDReviewQuestion struct {
	Text            string `json:"text"`
	Answer          string `json:"answer,omitempty"`
	Status          string `json:"status"`
	Blocking        bool   `json:"blocking,omitempty"`
	ClarificationID string `json:"clarification_id,omitempty"`
}

type HCDReviewFinding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // "Blocker", "Major", "Nit"
	Quote    string `json:"quote,omitempty"`
	Problem  string `json:"problem"`
	Question string `json:"question,omitempty"`
}

type HCDDefinitionReviewPayload struct {
	Summary   string              `json:"summary,omitempty"`
	Questions []HCDReviewQuestion `json:"questions,omitempty"`
	Quote     string              `json:"quote,omitempty"`
	AttemptID string              `json:"attempt_id,omitempty"`
	Reason    string              `json:"reason,omitempty"`
	Findings  []HCDReviewFinding  `json:"findings,omitempty"`
	FindingID string              `json:"finding_id,omitempty"`
}

type HCDDefinitionReviewEvent struct {
	OperationID HCDReviewOperationID       `json:"operation_id"`
	UnitID      string                     `json:"unit_id"`
	Revision    HCDReviewRevision          `json:"revision"`
	Kind        string                     `json:"kind"`
	Action      string                     `json:"action"`
	Client      string                     `json:"client"`
	CreatedAt   time.Time                  `json:"created_at"`
	Payload     HCDDefinitionReviewPayload `json:"payload"`
}

var hcdReviewRevisionPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

func (e HCDDefinitionReviewEvent) Validate() error {
	invalid := func(reason string) error { return fmt.Errorf("%w: definition review: %s", ErrValidation, reason) }
	if strings.TrimSpace(string(e.OperationID)) == "" || strings.TrimSpace(e.UnitID) == "" || strings.TrimSpace(e.Client) == "" || e.CreatedAt.IsZero() || !hcdReviewRevisionPattern.MatchString(string(e.Revision)) {
		return invalid("invalid identity, revision or timestamp")
	}
	switch e.Kind {
	case "socratic":
		if e.Action != "complete" {
			return invalid("unsupported socratic action")
		}
		if strings.TrimSpace(e.Payload.Summary) == "" || len(e.Payload.Questions) > 3 {
			return invalid("summary required; at most three questions")
		}
		for _, q := range e.Payload.Questions {
			if strings.TrimSpace(q.Text) == "" {
				return invalid("question text required")
			}
			switch q.Status {
			case "answered":
				if strings.TrimSpace(q.Answer) == "" {
					return invalid("answer required")
				}
			case "omitted":
				if strings.TrimSpace(q.Answer) != "" {
					return invalid("omitted question cannot contain an answer")
				}
			default:
				return invalid("question must be answered or omitted")
			}
			if q.Blocking && strings.TrimSpace(q.ClarificationID) == "" {
				return invalid("blocking question requires clarification reference")
			}
		}
	case "splash":
		switch e.Action {
		case "accept", "skip":
			if strings.TrimSpace(e.Payload.Quote) == "" {
				return invalid("quote required")
			}
		case "start":
			if strings.TrimSpace(e.Payload.AttemptID) == "" {
				return invalid("attempt_id required")
			}
		case "complete":
			if strings.TrimSpace(e.Payload.AttemptID) == "" {
				return invalid("attempt_id required")
			}
			if strings.TrimSpace(e.Payload.Summary) == "" {
				return invalid("summary required")
			}
			for _, f := range e.Payload.Findings {
				if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.Problem) == "" {
					return invalid("finding id and problem required")
				}
				if f.Severity != "Blocker" && f.Severity != "Major" && f.Severity != "Nit" {
					return invalid("finding severity must be Blocker, Major, or Nit")
				}
			}
		case "fail", "unavailable":
			if strings.TrimSpace(e.Payload.AttemptID) == "" {
				return invalid("attempt_id required")
			}
			if strings.TrimSpace(e.Payload.Reason) == "" {
				return invalid("reason required")
			}
		case "dismiss":
			if strings.TrimSpace(e.Payload.FindingID) == "" {
				return invalid("finding_id required")
			}
			if strings.TrimSpace(e.Payload.Quote) == "" {
				return invalid("quote required")
			}
		default:
			return invalid("unsupported splash action")
		}
	default:
		return invalid("unsupported kind")
	}
	return nil
}

// ValidateTransition checks that state machine ordering is respected for review events.
func (e HCDDefinitionReviewEvent) ValidateTransition(history []HCDDefinitionReviewEvent) error {
	invalid := func(reason string) error {
		return fmt.Errorf("%w: definition review transition: %s", ErrValidation, reason)
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if e.Kind != "splash" {
		return nil
	}
	var splashEvents []HCDDefinitionReviewEvent
	for _, h := range history {
		if h.Revision == e.Revision && h.Kind == "splash" {
			splashEvents = append(splashEvents, h)
		}
	}
	lastDecision := ""
	lastDecisionIdx := -1
	for i, h := range splashEvents {
		if h.Action == "accept" || h.Action == "skip" {
			lastDecision = h.Action
			lastDecisionIdx = i
		}
	}
	switch e.Action {
	case "accept", "skip":
		return nil
	case "start":
		if lastDecision != "accept" {
			return invalid("start requires active accept")
		}
		return nil
	case "complete", "fail", "unavailable":
		if lastDecision == "skip" {
			return invalid(e.Action + " cannot be recorded after skip")
		}
		if lastDecision != "accept" {
			return invalid(e.Action + " requires prior start")
		}
		postAccept := splashEvents[lastDecisionIdx+1:]
		var hasStart, completedOrFailed bool
		for _, h := range postAccept {
			if h.Action == "start" && h.Payload.AttemptID == e.Payload.AttemptID {
				hasStart = true
			}
			if (h.Action == "complete" || h.Action == "fail" || h.Action == "unavailable") && h.Payload.AttemptID == e.Payload.AttemptID {
				if h.OperationID != e.OperationID {
					completedOrFailed = true
				}
			}
		}
		if !hasStart {
			return invalid(e.Action + " requires matching start for attempt " + e.Payload.AttemptID)
		}
		if completedOrFailed {
			return invalid("attempt " + e.Payload.AttemptID + " is already finished")
		}
		return nil
	case "dismiss":
		if lastDecision != "accept" {
			return invalid("dismiss requires an active completed splash review")
		}
		postAccept := splashEvents[lastDecisionIdx+1:]
		lastLifecycleIdx := -1
		for i := len(postAccept) - 1; i >= 0; i-- {
			if postAccept[i].Action != "dismiss" {
				lastLifecycleIdx = i
				break
			}
		}
		if lastLifecycleIdx == -1 || postAccept[lastLifecycleIdx].Action != "complete" {
			return invalid("dismiss requires an existing Blocker finding with id " + e.Payload.FindingID)
		}
		var foundBlocker bool
		for _, f := range postAccept[lastLifecycleIdx].Payload.Findings {
			if f.ID == e.Payload.FindingID && f.Severity == "Blocker" {
				foundBlocker = true
				break
			}
		}
		if !foundBlocker {
			return invalid("dismiss requires an existing Blocker finding with id " + e.Payload.FindingID)
		}
		for _, h := range postAccept[lastLifecycleIdx+1:] {
			if h.Action == "dismiss" && h.Payload.FindingID == e.Payload.FindingID && h.OperationID != e.OperationID {
				return invalid("finding " + e.Payload.FindingID + " is already dismissed")
			}
		}
		return nil
	default:
		return invalid("unsupported action")
	}
}

// Protected returns a copy so caller-owned input is not changed during validation.
func (e HCDDefinitionReviewEvent) Protected() HCDDefinitionReviewEvent {
	e.Payload.Summary = ProtectText(e.Payload.Summary)
	e.Payload.Quote = ProtectText(e.Payload.Quote)
	e.Payload.Reason = ProtectText(e.Payload.Reason)
	if e.Payload.Questions != nil {
		e.Payload.Questions = append([]HCDReviewQuestion(nil), e.Payload.Questions...)
		for i := range e.Payload.Questions {
			q := &e.Payload.Questions[i]
			q.Text = ProtectText(q.Text)
			q.Answer = ProtectText(q.Answer)
		}
	}
	if e.Payload.Findings != nil {
		e.Payload.Findings = append([]HCDReviewFinding(nil), e.Payload.Findings...)
		for i := range e.Payload.Findings {
			f := &e.Payload.Findings[i]
			f.Quote = ProtectText(f.Quote)
			f.Problem = ProtectText(f.Problem)
			f.Question = ProtectText(f.Question)
		}
	}
	return e
}
