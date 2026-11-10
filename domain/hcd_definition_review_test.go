package domain

import (
	"strings"
	"testing"
	"time"
)

func socraticEvent() HCDDefinitionReviewEvent {
	return HCDDefinitionReviewEvent{OperationID: "op-1", UnitID: "HCD-001", Revision: "012345abcdef", Kind: "socratic", Action: "complete", Client: "codex", CreatedAt: time.Unix(1, 0).UTC(), Payload: HCDDefinitionReviewPayload{Summary: "Context already establishes intent, alternatives and observable criteria."}}
}

func TestHCD057SocraticEvent(t *testing.T) {
	if err := socraticEvent().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*HCDDefinitionReviewEvent){
		"operation": func(e *HCDDefinitionReviewEvent) { e.OperationID = "" },
		"revision":  func(e *HCDDefinitionReviewEvent) { e.Revision = "wrong" },
		"unit":      func(e *HCDDefinitionReviewEvent) { e.UnitID = "" },
		"client":    func(e *HCDDefinitionReviewEvent) { e.Client = "" },
		"timestamp": func(e *HCDDefinitionReviewEvent) { e.CreatedAt = time.Time{} },
		"summary":   func(e *HCDDefinitionReviewEvent) { e.Payload.Summary = "  " },
		"action":    func(e *HCDDefinitionReviewEvent) { e.Action = "skip" },
		"four questions": func(e *HCDDefinitionReviewEvent) {
			for range 4 {
				e.Payload.Questions = append(e.Payload.Questions, HCDReviewQuestion{Text: "Goal?", Answer: "Observable", Status: "answered"})
			}
		},
		"empty question": func(e *HCDDefinitionReviewEvent) { e.Payload.Questions = []HCDReviewQuestion{{Status: "omitted"}} },
		"unanswered": func(e *HCDDefinitionReviewEvent) {
			e.Payload.Questions = []HCDReviewQuestion{{Text: "Goal?", Status: "answered"}}
		},
		"invented answer": func(e *HCDDefinitionReviewEvent) {
			e.Payload.Questions = []HCDReviewQuestion{{Text: "Goal?", Status: "omitted", Answer: "Assumed"}}
		},
		"blocking without clarification": func(e *HCDDefinitionReviewEvent) {
			e.Payload.Questions = []HCDReviewQuestion{{Text: "Scope?", Status: "omitted", Blocking: true}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := socraticEvent()
			mutate(&e)
			if e.Validate() == nil {
				t.Fatal("invalid event accepted")
			}
		})
	}
	e := socraticEvent()
	e.Payload.Questions = []HCDReviewQuestion{{Text: "Goal?", Answer: "Observable", Status: "answered"}, {Text: "Scope?", Status: "omitted", Blocking: true, ClarificationID: "Q1"}}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	e.Payload.Questions[0].Text = "<private>hidden</private> goal?"
	e = e.Protected()
	if strings.Contains(e.Payload.Questions[0].Text, "hidden") {
		t.Fatal("private text retained")
	}
}
