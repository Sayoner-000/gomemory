package domain

import (
	"errors"
	"testing"
	"time"
)

var specifyNow = time.Unix(1000, 0).UTC()

func newSpecifyFlow() RefinementFlow {
	return RefinementFlow{ID: "f", Mode: ModeSpecify, Phase: PhaseCaptured, LastStablePhase: PhaseCaptured, Preparation: NewSpecifyPreparation(), PayloadPolicy: PayloadPersistent}
}

func specifyReduce(t *testing.T, f RefinementFlow, ev Event) (RefinementFlow, Effect, error) {
	t.Helper()
	return Reduce(f, ev, specifyNow, func() string { return "new-id" })
}

func specifyQuestions(n int) []Question {
	qs := make([]Question, n)
	for i := range qs {
		qs[i] = Question{Text: "¿Pregunta?", Options: []string{"a", "b"}}
	}
	return qs
}

func TestSpecifyQuestions_ZeroQuestionsStayInPreparation(t *testing.T) {
	got, effect, err := specifyReduce(t, newSpecifyFlow(), Event{Kind: "questions_generated"})
	if err != nil || got.Phase != PhaseCaptured || effect != EffectNone || len(got.Questions) != 0 {
		t.Fatalf("zero questions = %s %s %v; want captured, no generation", got.Phase, effect, err)
	}
}

func TestSpecifyQuestions_AccumulateUpToThreeAndReturnToPreparation(t *testing.T) {
	f, effect, err := specifyReduce(t, newSpecifyFlow(), Event{Kind: "questions_generated", Questions: specifyQuestions(2)})
	if err != nil || f.Phase != PhaseQuestions || len(f.Questions) != 2 || effect != EffectNone {
		t.Fatalf("first batch = %s %d %s %v", f.Phase, len(f.Questions), effect, err)
	}
	// Answering all of them goes back to preparation, never to a free rewrite.
	f, _, err = specifyReduce(t, f, Event{Kind: "answer", Answer: Answer{QuestionIndex: 0, Choice: "a"}})
	if err != nil || f.Phase != PhaseQuestions {
		t.Fatalf("partial answer = %s, %v", f.Phase, err)
	}
	f, effect, err = specifyReduce(t, f, Event{Kind: "skip_rest"})
	if err != nil || f.Phase != PhaseCaptured || f.LastStablePhase != PhaseCaptured || effect != EffectNone {
		t.Fatalf("after last answer = %s %s %s %v; want captured without EffectGenerateRewrite", f.Phase, f.LastStablePhase, effect, err)
	}
	// One more question fits; a fourth never does, and the list is not reset.
	f, _, err = specifyReduce(t, f, Event{Kind: "questions_generated", Questions: specifyQuestions(1)})
	if err != nil || len(f.Questions) != 3 || len(f.Answers) != 2 || f.Phase != PhaseQuestions {
		t.Fatalf("second batch = %d questions %d answers %s, %v", len(f.Questions), len(f.Answers), f.Phase, err)
	}
	f, _, err = specifyReduce(t, f, Event{Kind: "skip"})
	if err != nil || f.Phase != PhaseCaptured {
		t.Fatalf("last skip = %s, %v", f.Phase, err)
	}
	if _, _, err = specifyReduce(t, f, Event{Kind: "questions_generated", Questions: specifyQuestions(1)}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("fourth question = %v, want ErrInvalidTransition", err)
	}
}

func TestSpecifyQuestions_BatchOverTheLimitIsRejectedWhole(t *testing.T) {
	f, _, _ := specifyReduce(t, newSpecifyFlow(), Event{Kind: "questions_generated", Questions: specifyQuestions(2)})
	f, _, _ = specifyReduce(t, f, Event{Kind: "skip_rest"})
	if _, _, err := specifyReduce(t, f, Event{Kind: "questions_generated", Questions: specifyQuestions(2)}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("2+2 questions = %v, want rejected without partial application", err)
	}
}

func TestSpecifyQuestions_NoNewQuestionsWhileOneIsPending(t *testing.T) {
	f, _, _ := specifyReduce(t, newSpecifyFlow(), Event{Kind: "questions_generated", Questions: specifyQuestions(1)})
	if _, _, err := specifyReduce(t, f, Event{Kind: "questions_generated", Questions: specifyQuestions(1)}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("questions while one is unanswered = %v", err)
	}
}

func TestSpecifyQuestions_EditAndResumeDoNotResetTheCounter(t *testing.T) {
	f, _, _ := specifyReduce(t, newSpecifyFlow(), Event{Kind: "questions_generated", Questions: specifyQuestions(3)})
	f, _, _ = specifyReduce(t, f, Event{Kind: "skip_rest"})
	f, effect, err := specifyReduce(t, f, Event{Kind: "resume"})
	if err != nil || effect != EffectNone || len(f.Questions) != 3 || len(f.Answers) != 3 {
		t.Fatalf("resume = %d questions, %s, %v; want counter kept and no generation", len(f.Questions), effect, err)
	}
	f, _, err = specifyReduce(t, f, Event{Kind: "prepare_set", Section: "intent", ExpectedRevision: 0, Content: SpecifyContent{Intent: &SpecifyIntent{Objective: "o"}}})
	if err != nil || len(f.Questions) != 3 || len(f.Answers) != 3 {
		t.Fatalf("section edit = %d questions %d answers, %v; the budget must survive edits", len(f.Questions), len(f.Answers), err)
	}
}

func TestSpecifyMode_ClosesGenericRoutes(t *testing.T) {
	f := newSpecifyFlow()
	f.Phase, f.ProposedPrompt = PhaseReview, "texto libre del agente"
	// approve is not here: it is specific to specify (review-bound) since US3.
	for _, kind := range []string{"rewrite_ready", "edit", "send_original", "retry", "model_error", "fail"} {
		ev := Event{Kind: kind, Proposed: "texto libre"}
		for _, phase := range []SocraticaPhase{PhaseCaptured, PhaseRewriting, PhaseReview, PhaseFailed} {
			g := f
			g.Phase = phase
			if _, _, err := specifyReduce(t, g, ev); !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("%s in %s = %v; the generic route must stay closed for specify", kind, phase, err)
			}
		}
	}
	// Previous modes keep their behaviour.
	r := RefinementFlow{Mode: ModeRefine, Phase: PhaseReview, ProposedPrompt: "p"}
	if got, _, err := specifyReduce(t, r, Event{Kind: "approve"}); err != nil || got.Phase != PhaseApproved {
		t.Fatalf("refine approve = %s, %v", got.Phase, err)
	}
	// resume of a captured specify flow never asks for generation.
	if _, effect, err := specifyReduce(t, newSpecifyFlow(), Event{Kind: "resume"}); err != nil || effect != EffectNone {
		t.Fatalf("specify resume = %s, %v", effect, err)
	}
	if _, effect, _ := specifyReduce(t, RefinementFlow{Mode: ModeRefine, Phase: PhaseCaptured}, Event{Kind: "resume"}); effect != EffectGenerateQuestions {
		t.Fatalf("refine resume effect = %s", effect)
	}
}
