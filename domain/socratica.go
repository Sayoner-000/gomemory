package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type SocraticaPhase string

const (
	PhaseCaptured   SocraticaPhase = "captured"
	PhaseQuestions  SocraticaPhase = "questions"
	PhaseRewriting  SocraticaPhase = "rewriting"
	PhaseReview     SocraticaPhase = "review"
	PhaseApproved   SocraticaPhase = "approved"
	PhaseDispatched SocraticaPhase = "dispatched"
	PhaseFailed     SocraticaPhase = "failed"
	PhaseCancelled  SocraticaPhase = "cancelled"
)

func (p SocraticaPhase) IsStable() bool {
	return p == PhaseCaptured || p == PhaseQuestions || p == PhaseReview || p == PhaseApproved
}
func (p SocraticaPhase) Terminal() bool { return p == PhaseDispatched || p == PhaseCancelled }

type SocraticaMode string

const (
	ModeRefine   SocraticaMode = "refine"
	ModeScrutiny SocraticaMode = "scrutiny"
)

type Question struct {
	Text    string   `json:"text"`
	Options []string `json:"options"`
}
type Answer struct {
	QuestionIndex int    `json:"index"`
	Choice        string `json:"choice,omitempty"`
	Custom        string `json:"custom,omitempty"`
	Skipped       bool   `json:"skipped,omitempty"`
}
type SocraticaAssumption struct {
	Text string `json:"text"`
	Kind string `json:"kind"`
}

// ScrutinySynthesis fields are all optional, as in generation.schema.json.
type ScrutinySynthesis struct {
	Restatement        string                `json:"restatement,omitempty"`
	Assumptions        []SocraticaAssumption `json:"assumptions,omitempty"`
	StrongestSupport   string                `json:"strongest_support,omitempty"`
	StrongestObjection string                `json:"strongest_objection,omitempty"`
	Alternatives       []string              `json:"alternatives,omitempty"`
	MissingEvidence    []string              `json:"missing_evidence,omitempty"`
	DecisionCriteria   []string              `json:"decision_criteria,omitempty"`
	Hypotheses         []string              `json:"hypotheses,omitempty"`
}
type RefinementFlow struct {
	ID                                                                  string
	SchemaVersion                                                       int
	Project, Client                                                     string
	Mode                                                                SocraticaMode
	OriginalPrompt                                                      string
	Questions                                                           []Question
	Answers                                                             []Answer
	ProposedPrompt, ApprovedPrompt                                      string
	Scrutiny                                                            *ScrutinySynthesis
	Additions                                                           []string
	Phase, LastStablePhase                                              SocraticaPhase
	Revision                                                            int
	DispatchID                                                          string
	DispatchAck                                                         bool
	ApprovedHash, DispatchClaimOwner                                    string
	DispatchClaimedAt                                                   *time.Time
	AckNonce                                                            string
	DispatchConsumed                                                    bool
	LastError                                                           string
	CreatedAt, UpdatedAt                                                time.Time
	Duplicate, AlreadyDispatched, Acked, Recovered, Uncertain, InFlight bool
}
type RefinementHistory struct {
	ID, FlowID, Project, Client                    string
	Mode                                           SocraticaMode
	OriginalPrompt, ProposedPrompt, ApprovedPrompt string
	Scrutiny                                       *ScrutinySynthesis
	Additions                                      []string
	WordsBefore, WordsAfter                        int
	CompletedAt                                    time.Time
}
type Event struct {
	Kind                            string
	Questions                       []Question
	Answer                          Answer
	Proposed                        string
	Scrutiny                        *ScrutinySynthesis
	Additions                       []string
	DispatchID, Owner, Nonce, Error string
	Private, LockHeld               bool
}
type Effect string

const (
	EffectNone              Effect = "none"
	EffectRecordHistory     Effect = "record_history"
	EffectGenerateQuestions Effect = "generate_questions"
	EffectGenerateRewrite   Effect = "generate_rewrite"
	EffectRestoreOriginal   Effect = "restore_original"
	EffectDeleteFlow        Effect = "delete_flow"
	EffectRedactFlow        Effect = "redact_flow"
)

var ErrInvalidTransition = errors.New("invalid socratica transition")

// Reduce is pure: all nondeterministic inputs and host lock state are supplied.
func Reduce(f RefinementFlow, ev Event, now time.Time, newID func() string) (RefinementFlow, Effect, error) {
	before := f
	f.Answers = append([]Answer(nil), f.Answers...)
	f.Duplicate = false
	f.AlreadyDispatched = false
	f.Acked = false
	f.Recovered = false
	f.Uncertain = false
	f.InFlight = false
	effect := EffectNone
	invalid := func() (RefinementFlow, Effect, error) { return before, EffectNone, ErrInvalidTransition }
	if f.DispatchClaimOwner != "" && ev.LockHeld && (ev.Kind == "cancel" || ev.Kind == "force_resend" || ev.Kind == "mark_dispatched") {
		f.InFlight = true
		return f, effect, nil
	}
	approve := func(prompt string) {
		f.Phase = PhaseApproved
		f.ApprovedPrompt = prompt
		if f.DispatchID == "" {
			f.DispatchID = newID()
		}
		f.ApprovedHash = SocraticaPromptHash(prompt)
		f.LastStablePhase = PhaseApproved
		effect = EffectRecordHistory
	}
	switch ev.Kind {
	case "questions_generated":
		if f.Phase != PhaseCaptured || len(ev.Questions) > 3 {
			return invalid()
		}
		for _, q := range ev.Questions {
			if strings.TrimSpace(q.Text) == "" || len(q.Options) < 2 || len(q.Options) > 4 {
				return invalid()
			}
		}
		f.Questions = cloneSocraticaQuestions(ev.Questions)
		if ev.Scrutiny != nil {
			f.Scrutiny = ev.Scrutiny
		}
		if len(f.Questions) == 0 {
			f.Phase = PhaseRewriting
			effect = EffectGenerateRewrite
		} else {
			f.Phase = PhaseQuestions
			f.LastStablePhase = PhaseQuestions
		}
	case "answer", "skip", "skip_rest":
		if f.Phase != PhaseQuestions {
			return invalid()
		}
		index := len(f.Answers)
		if index >= len(f.Questions) {
			return invalid()
		}
		if ev.Kind == "skip_rest" {
			for i := index; i < len(f.Questions); i++ {
				f.Answers = append(f.Answers, Answer{QuestionIndex: i, Skipped: true})
			}
		} else {
			a := ev.Answer
			if ev.Kind == "skip" {
				a = Answer{QuestionIndex: index, Skipped: true}
			}
			if a.QuestionIndex != index {
				return invalid()
			}
			if !a.Skipped && a.Custom == "" {
				valid := false
				for _, o := range f.Questions[index].Options {
					if a.Choice == o {
						valid = true
					}
				}
				if !valid {
					return invalid()
				}
			}
			f.Answers = append(f.Answers, a)
		}
		if len(f.Answers) == len(f.Questions) {
			f.Phase = PhaseRewriting
			effect = EffectGenerateRewrite
		}
	case "rewrite_ready":
		if f.Phase != PhaseRewriting || strings.TrimSpace(ev.Proposed) == "" {
			return invalid()
		}
		f.ProposedPrompt = ev.Proposed
		f.Scrutiny = ev.Scrutiny
		f.Additions = append([]string(nil), ev.Additions...)
		f.Phase = PhaseReview
		f.LastStablePhase = PhaseReview
		effect = EffectRecordHistory
	case "edit":
		if f.Phase != PhaseReview || strings.TrimSpace(ev.Proposed) == "" {
			return invalid()
		}
		f.ProposedPrompt = ev.Proposed
		f.Revision++
		effect = EffectRecordHistory
	case "approve":
		switch f.Phase {
		case PhaseReview:
			if strings.TrimSpace(f.ProposedPrompt) == "" {
				return invalid()
			}
			approve(f.ProposedPrompt)
		case PhaseApproved:
			if f.DispatchClaimOwner != "" {
				return invalid()
			}
			f.Duplicate = true
		case PhaseDispatched:
			f.Duplicate = true
			f.AlreadyDispatched = true
		default:
			return invalid()
		}
	case "ack":
		if ev.DispatchID == "" || ev.DispatchID != f.DispatchID || ev.Owner != f.DispatchClaimOwner {
			return invalid()
		}
		if f.Phase == PhaseDispatched {
			if ev.Nonce != "" && ev.Nonce == f.AckNonce {
				f.Acked = true
			} else {
				f.AlreadyDispatched = true
			}
			return f, effect, nil
		}
		if f.Phase != PhaseApproved {
			return invalid()
		}
		f.Phase = PhaseDispatched
		f.DispatchAck = true
		f.Acked = true
		f.AckNonce = ev.Nonce
		if ev.Private {
			effect = EffectRedactFlow
		}
	case "model_error", "fail":
		if f.Phase != PhaseCaptured && f.Phase != PhaseRewriting {
			return invalid()
		}
		f.Phase = PhaseFailed
		f.LastError = ev.Error
	case "retry":
		if f.Phase != PhaseFailed {
			return invalid()
		}
		switch f.LastStablePhase {
		case PhaseCaptured:
			f.Phase = PhaseCaptured
			effect = EffectGenerateQuestions
		case PhaseQuestions:
			f.Phase = PhaseRewriting
			effect = EffectGenerateRewrite
		default:
			return invalid()
		}
		f.LastError = ""
	case "send_original":
		if f.Phase != PhaseFailed {
			return invalid()
		}
		approve(f.OriginalPrompt)
	case "cancel":
		if f.Phase.Terminal() {
			return invalid()
		}
		f.Phase = PhaseCancelled
		f.ProposedPrompt = ""
		effect = EffectRestoreOriginal
		if ev.Private {
			effect = EffectDeleteFlow
		}
	case "resume":
		switch f.Phase {
		case PhaseApproved:
			if f.DispatchClaimOwner != "" {
				f.InFlight = ev.LockHeld
				f.Uncertain = !ev.LockHeld
			} else {
				f.Phase = PhaseReview
				f.LastStablePhase = PhaseReview
				f.Recovered = true
			}
		case PhaseCaptured:
			effect = EffectGenerateQuestions
		case PhaseRewriting:
			effect = EffectGenerateRewrite
		case PhaseQuestions, PhaseReview:
		default:
			return invalid()
		}
	case "mark_dispatched":
		if f.Phase != PhaseApproved || f.DispatchClaimOwner == "" {
			return invalid()
		}
		f.Phase = PhaseDispatched
		f.DispatchAck = true
		if ev.Private {
			effect = EffectRedactFlow
		}
	case "force_resend":
		if f.Phase != PhaseDispatched && (f.Phase != PhaseApproved || f.DispatchClaimOwner == "") {
			return invalid()
		}
		if strings.TrimSpace(f.ApprovedPrompt) == "" {
			return invalid()
		}
		f.Phase = PhaseApproved
		f.DispatchID = newID()
		f.DispatchAck = false
		f.AckNonce = ""
		f.DispatchClaimOwner = ""
		f.DispatchClaimedAt = nil
		f.DispatchConsumed = false
	default:
		return invalid()
	}
	f.UpdatedAt = now
	return f, effect, nil
}
func SocraticaPromptHash(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}
func cloneSocraticaQuestions(qs []Question) []Question {
	result := make([]Question, len(qs))
	for i, q := range qs {
		result[i] = q
		result[i].Options = append([]string(nil), q.Options...)
	}
	return result
}
