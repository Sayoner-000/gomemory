package domain

import (
	"errors"
	"strings"
	"testing"
)

// reviewReadyFlow is a specify flow whose four checks passed: phase review.
func reviewReadyFlow(t *testing.T) RefinementFlow {
	t.Helper()
	f := flowWithContent()
	f.OriginalPrompt = "necesidad original"
	for _, s := range []SpecifyStage{StageIntent, StageScope, StageStories, StageValidation} {
		f = step(t, f, validateEvent(s, 3))
	}
	if f.Phase != PhaseReview {
		t.Fatalf("setup: phase %s", f.Phase)
	}
	return f
}

func approveEvent(rev int) Event { return Event{Kind: "approve", ExpectedRevision: rev} }

func approvedFlow(t *testing.T) RefinementFlow {
	t.Helper()
	return step(t, reviewReadyFlow(t), approveEvent(3))
}

func TestApproveSpecify_BindsToTheExactReviewedText(t *testing.T) {
	f := reviewReadyFlow(t)
	got, effect, err := specifyReduce(t, f, approveEvent(3))
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != PhaseApproved || got.LastStablePhase != PhaseApproved || got.Preparation.Stage != StageApproved {
		t.Fatalf("phase %s stage %s", got.Phase, got.Preparation.Stage)
	}
	if got.ApprovedPrompt != f.ProposedPrompt || SpecifyPromptHash(got.ApprovedPrompt) != f.Preparation.Review.PromptHash {
		t.Fatal("the approved text is not byte for byte the reviewed one")
	}
	if got.DispatchID != "new-id" || got.Revision != 3 || effect != EffectRecordHistory || got.ExecutionGranted || got.Acked {
		t.Fatalf("dispatch %q rev %d effect %s granted %v", got.DispatchID, got.Revision, effect, got.ExecutionGranted)
	}
	// The hook provenance hash keeps its own, trimmed meaning.
	if got.ApprovedHash != SocraticaPromptHash(got.ApprovedPrompt) || got.ApprovedHash == SpecifyPromptHash(got.ApprovedPrompt) {
		t.Fatalf("approved_hash = %q; provenance must stay the normalised hash", got.ApprovedHash)
	}
	if !strings.HasSuffix(got.ApprovedPrompt, "\n") {
		t.Fatal("the render ends in a newline: that is what makes the two hashes differ")
	}
}

func TestApproveSpecify_RefusesAnythingButTheCurrentReview(t *testing.T) {
	if _, _, err := specifyReduce(t, reviewReadyFlow(t), approveEvent(2)); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision = %v", err)
	}
	if _, _, err := specifyReduce(t, flowWithContent(), approveEvent(3)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approve before the review = %v", err)
	}
	tamper := map[string]func(*RefinementFlow){
		"no review record":           func(f *RefinementFlow) { f.Preparation.Review = nil },
		"review of another revision": func(f *RefinementFlow) { f.Preparation.Review.Revision = 1 },
		"proposal edited":            func(f *RefinementFlow) { f.ProposedPrompt += "\nextra" },
		"hash does not match":        func(f *RefinementFlow) { f.Preparation.Review.PromptHash = "x" },
		"content changed behind":     func(f *RefinementFlow) { f.Preparation.Content.Intent.Objective = "otro" },
		"sanitised content": func(f *RefinementFlow) {
			f.Preparation.Content.Stories.Stories[0].Need = "[REDACTED:key]"
		},
		"a check went stale": func(f *RefinementFlow) { f.Preparation.Checks[2].InputHash = "viejo" },
		"a check is missing": func(f *RefinementFlow) { f.Preparation.Checks = f.Preparation.Checks[:3] },
	}
	for name, mutate := range tamper {
		f := reviewReadyFlow(t)
		mutate(&f)
		if _, _, err := specifyReduce(t, f, approveEvent(3)); !errors.Is(err, ErrReviewStale) {
			t.Errorf("%s = %v, want ErrReviewStale", name, err)
		}
	}
	for _, phase := range []SocraticaPhase{PhaseRewriting, PhaseFailed, PhaseCancelled} {
		f := reviewReadyFlow(t)
		f.Phase = phase
		if _, _, err := specifyReduce(t, f, approveEvent(3)); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("approve in %s = %v", phase, err)
		}
	}
}

func TestApproveSpecify_RepeatedApprovalKeepsTheDispatch(t *testing.T) {
	f := approvedFlow(t)
	again, effect, err := specifyReduce(t, f, approveEvent(3))
	if err != nil || !again.Duplicate || again.DispatchID != f.DispatchID || again.ApprovedPrompt != f.ApprovedPrompt || again.ExecutionGranted || effect != EffectNone {
		t.Fatalf("repeat = %+v effect %s, %v", again.Duplicate, effect, err)
	}
	if _, _, err = specifyReduce(t, f, approveEvent(2)); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("repeat with a stale revision = %v", err)
	}
	claimed := f
	claimed.DispatchClaimOwner = "o"
	if _, _, err = specifyReduce(t, claimed, approveEvent(3)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approve over a claimed dispatch = %v", err)
	}
	sent := f
	sent.Phase, sent.DispatchAck = PhaseDispatched, true
	got, _, err := specifyReduce(t, sent, approveEvent(3))
	if err != nil || !got.Duplicate || !got.AlreadyDispatched {
		t.Fatalf("approve after dispatch = %+v, %v", got, err)
	}
}

func TestEditAfterApproval_RemovesTheUnconsumedAuthorization(t *testing.T) {
	f := approvedFlow(t)
	st := completeSpecifyContent().Stories
	st.Criteria[0].ExpectedResult = "otro resultado"
	got := step(t, f, prepareEvent(StageStories, 3, SpecifyContent{Stories: st}))
	if got.Phase != PhaseCaptured || got.Revision != 4 || got.ApprovedPrompt != "" || got.ApprovedHash != "" || got.DispatchID != "" || got.ProposedPrompt != "" || got.Preparation.Review != nil {
		t.Fatalf("after editing an approved flow: %+v", got)
	}
	// Intent and scope checks stay current; stories and validation must be redone.
	if len(got.Preparation.Checks) != 2 {
		t.Fatalf("checks = %+v", got.Preparation.Checks)
	}
	if _, _, err := specifyReduce(t, got, approveEvent(4)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("approving the edited draft without review = %v", err)
	}
	for name, mutate := range map[string]func(*RefinementFlow){
		"claimed":  func(f *RefinementFlow) { f.DispatchClaimOwner = "o" },
		"consumed": func(f *RefinementFlow) { f.DispatchAck, f.Phase = true, PhaseDispatched },
	} {
		g := f
		mutate(&g)
		if _, _, err := specifyReduce(t, g, prepareEvent(StageStories, 3, SpecifyContent{Stories: st})); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("editing a %s dispatch = %v, want ErrInvalidTransition", name, err)
		}
	}
}

func TestAckSpecify_ChecksTheIdentityStillMatches(t *testing.T) {
	f := approvedFlow(t)
	ack := Event{Kind: "ack", DispatchID: f.DispatchID, Nonce: "n"}
	got, _, err := specifyReduce(t, f, ack)
	if err != nil || got.Phase != PhaseDispatched || !got.Acked {
		t.Fatalf("ack = %s, %v", got.Phase, err)
	}
	for name, mutate := range map[string]func(*RefinementFlow){
		"content sanitised after approval": func(f *RefinementFlow) { f.Preparation.Content.Intent.Objective = "[REDACTED:key]" },
		"approved text differs":            func(f *RefinementFlow) { f.ApprovedPrompt += " " },
		"review dropped":                   func(f *RefinementFlow) { f.Preparation.Review = nil },
	} {
		g := f
		g.Preparation = f.Preparation.Clone()
		mutate(&g)
		if _, _, err := specifyReduce(t, g, ack); !errors.Is(err, ErrReviewStale) {
			t.Errorf("%s = %v, want ErrReviewStale", name, err)
		}
	}
	// A replay of an already-consumed dispatch does not re-check the content.
	sent := got
	sent.Preparation = got.Preparation.Clone()
	sent.Preparation.Content.Intent.Objective = "[REDACTED:key]"
	if replay, _, err := specifyReduce(t, sent, ack); err != nil || !replay.Acked {
		t.Fatalf("replay after sanitisation = %+v, %v", replay.Acked, err)
	}
}

func TestResumeSpecify_KeepsAValidApprovalAndDropsAStaleOne(t *testing.T) {
	f := approvedFlow(t)
	got, effect, err := specifyReduce(t, f, Event{Kind: "resume"})
	if err != nil || got.Phase != PhaseApproved || !got.Recovered || got.ApprovedPrompt != f.ApprovedPrompt || got.DispatchID != f.DispatchID || effect != EffectNone {
		t.Fatalf("resume = phase %s recovered %v, %v", got.Phase, got.Recovered, err)
	}
	f.Preparation.Content.Intent.Objective = "[REDACTED:key]"
	got, _, err = specifyReduce(t, f, Event{Kind: "resume"})
	if err != nil || got.Phase != PhaseCaptured || got.ApprovedPrompt != "" || got.DispatchID != "" || got.Preparation.Review != nil {
		t.Fatalf("resume over stale content = phase %s approved %q, %v", got.Phase, got.ApprovedPrompt, err)
	}
	// Previous modes keep dropping an unclaimed approval back to review.
	r := RefinementFlow{Mode: ModeRefine, Phase: PhaseApproved, ProposedPrompt: "p", ApprovedPrompt: "p"}
	if got, _, _ = specifyReduce(t, r, Event{Kind: "resume"}); got.Phase != PhaseReview {
		t.Fatalf("refine resume = %s", got.Phase)
	}
}

func TestCancelSpecify_ReturnsTheProtectedOriginalAndWithdrawsThePreparation(t *testing.T) {
	for _, from := range []string{"captured", "review", "approved"} {
		f := reviewReadyFlow(t)
		f.OriginalPrompt = "original protegido"
		if from == "approved" {
			f = step(t, f, approveEvent(3))
		}
		if from == "captured" {
			f = flowWithContent()
			f.OriginalPrompt = "original protegido"
		}
		got, effect, err := specifyReduce(t, f, Event{Kind: "cancel"})
		if err != nil || got.Phase != PhaseCancelled || effect != EffectRestoreOriginal {
			t.Fatalf("%s: phase %s effect %s, %v", from, got.Phase, effect, err)
		}
		if got.OriginalPrompt != "original protegido" || got.Preparation != nil || got.ProposedPrompt != "" || got.ApprovedPrompt != "" || got.DispatchID != "" || got.ApprovedHash != "" {
			t.Fatalf("%s: cancel left the draft or the authorization: %+v", from, got)
		}
		if priv, effect, _ := specifyReduce(t, f, Event{Kind: "cancel", Private: true}); effect != EffectDeleteFlow || priv.Phase != PhaseCancelled {
			t.Fatalf("%s: private cancel effect %s", from, effect)
		}
	}
	// A preparation of a newer version is never destroyed, even by a cancel.
	future := flowWithContent()
	future.Preparation.SchemaVersion = 99
	got, _, err := specifyReduce(t, future, Event{Kind: "cancel"})
	if err != nil || got.Phase != PhaseCancelled || got.Preparation == nil || got.Preparation.SchemaVersion != 99 {
		t.Fatalf("cancel of a newer preparation = %+v, %v", got.Preparation, err)
	}
	// Previous modes keep their cancel behaviour.
	r := RefinementFlow{Mode: ModeRefine, Phase: PhaseReview, ProposedPrompt: "p", OriginalPrompt: "o"}
	if got, _, _ = specifyReduce(t, r, Event{Kind: "cancel"}); got.OriginalPrompt != "o" || got.ProposedPrompt != "" {
		t.Fatalf("refine cancel = %+v", got)
	}
}
