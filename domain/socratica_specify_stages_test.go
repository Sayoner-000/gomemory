package domain

import (
	"errors"
	"testing"
)

func validateEvent(stage SpecifyStage, rev int) Event {
	return Event{Kind: "prepare_validate", Section: stage, ExpectedRevision: rev}
}

// flowWithContent is a captured specify flow whose content is complete.
func flowWithContent() RefinementFlow {
	f := newSpecifyFlow()
	f.Revision = 3
	f.Preparation.Content = completeSpecifyContent()
	return f
}

func step(t *testing.T, f RefinementFlow, ev Event) RefinementFlow {
	t.Helper()
	got, _, err := specifyReduce(t, f, ev)
	if err != nil {
		t.Fatalf("%s %s: %v", ev.Kind, ev.Section, err)
	}
	return got
}

func checkFor(f RefinementFlow, stage SpecifyStage) *SpecifyCheck {
	for i := range f.Preparation.Checks {
		if f.Preparation.Checks[i].Stage == stage {
			return &f.Preparation.Checks[i]
		}
	}
	return nil
}

func TestValidate_StagesCannotBeSkipped(t *testing.T) {
	f := flowWithContent()
	for _, stage := range []SpecifyStage{StageScope, StageStories, StageValidation} {
		if _, _, err := specifyReduce(t, f, validateEvent(stage, 3)); !errors.Is(err, ErrStageBlocked) {
			t.Errorf("%s before its predecessors = %v, want ErrStageBlocked", stage, err)
		}
	}
	f = step(t, f, validateEvent(StageIntent, 3))
	if _, _, err := specifyReduce(t, f, validateEvent(StageStories, 3)); !errors.Is(err, ErrStageBlocked) {
		t.Errorf("stories without the scope check = %v", err)
	}
	for _, stage := range []SpecifyStage{StageReview, StageApproved, "nonsense"} {
		if _, _, err := specifyReduce(t, f, validateEvent(stage, 3)); !errors.Is(err, ErrStageBlocked) {
			t.Errorf("stage %q is not checkable: %v", stage, err)
		}
	}
}

func TestValidate_PassRecordsHashAndAdvances(t *testing.T) {
	f := flowWithContent()
	f = step(t, f, validateEvent(StageIntent, 3))
	c := checkFor(f, StageIntent)
	if c == nil || !c.Passed || len(c.Missing) != 0 || c.InputHash != SpecifyInputHash(f.Preparation.Content, StageIntent) {
		t.Fatalf("intent check = %+v", c)
	}
	if f.Preparation.Stage != StageScope || f.Phase != PhaseCaptured || f.Revision != 3 {
		t.Fatalf("stage %s phase %s rev %d; want scope, captured, revision untouched", f.Preparation.Stage, f.Phase, f.Revision)
	}
	f = step(t, f, validateEvent(StageScope, 3))
	f = step(t, f, validateEvent(StageStories, 3))
	if f.Preparation.Stage != StageValidation || f.Phase != PhaseCaptured {
		t.Fatalf("after stories: stage %s phase %s", f.Preparation.Stage, f.Phase)
	}
}

func TestValidate_FailureKeepsDraftAndDoesNotUnlockNextStages(t *testing.T) {
	f := flowWithContent()
	f.Preparation.Content.Intent.Objective = ""
	f = step(t, f, validateEvent(StageIntent, 3))
	c := checkFor(f, StageIntent)
	if c == nil || c.Passed || !issueAt(c.Missing, "missing", "intent.objective") {
		t.Fatalf("failed check = %+v", c)
	}
	if f.Preparation.Stage != StageIntent || f.Preparation.Content.Intent == nil {
		t.Fatalf("stage %s; the draft must be kept and stay at intent", f.Preparation.Stage)
	}
	if _, _, err := specifyReduce(t, f, validateEvent(StageScope, 3)); !errors.Is(err, ErrStageBlocked) {
		t.Fatalf("scope after a failed intent = %v, want ErrStageBlocked", err)
	}
	// Fix it through the normal edit: the failed check is dropped and it passes.
	fixed := completeSpecifyContent()
	f = step(t, f, prepareEvent(StageIntent, 3, SpecifyContent{Intent: fixed.Intent}))
	if checkFor(f, StageIntent) != nil {
		t.Fatal("the failed check survived the edit")
	}
	f = step(t, f, validateEvent(StageIntent, 4))
	if c = checkFor(f, StageIntent); c == nil || !c.Passed {
		t.Fatalf("after the fix = %+v", c)
	}
}

func TestValidate_StaleHashDoesNotEnableTheNextStage(t *testing.T) {
	f := flowWithContent()
	f = step(t, f, validateEvent(StageIntent, 3))
	// A check whose inputs changed behind its back (e.g. storage edited) is stale.
	f.Preparation.Content.Intent.Objective = "otro objetivo"
	if _, _, err := specifyReduce(t, f, validateEvent(StageScope, 3)); !errors.Is(err, ErrStageBlocked) {
		t.Fatalf("scope over a stale intent check = %v, want ErrStageBlocked", err)
	}
	// The agent cannot supply validity or hashes.
	forged := flowWithContent()
	forged.Preparation.Checks = []SpecifyCheck{{Stage: StageIntent, Passed: true, InputHash: "inventado"}}
	if _, _, err := specifyReduce(t, forged, validateEvent(StageScope, 3)); !errors.Is(err, ErrStageBlocked) {
		t.Fatalf("scope over a forged check = %v", err)
	}
}

func TestValidate_EditInvalidatesFromTheEditedSection(t *testing.T) {
	f := flowWithContent()
	for _, s := range []SpecifyStage{StageIntent, StageScope, StageStories} {
		f = step(t, f, validateEvent(s, 3))
	}
	scope := completeSpecifyContent().Scope
	scope.Included = []string{"otro alcance"}
	f = step(t, f, prepareEvent(StageScope, 3, SpecifyContent{Scope: scope}))
	if checkFor(f, StageIntent) == nil || checkFor(f, StageScope) != nil || checkFor(f, StageStories) != nil {
		t.Fatalf("checks after a scope edit = %+v", f.Preparation.Checks)
	}
	if _, _, err := specifyReduce(t, f, validateEvent(StageStories, 4)); !errors.Is(err, ErrStageBlocked) {
		t.Fatalf("stories after a scope edit = %v", err)
	}
}

func TestValidate_FinalPassBuildsTheReviewAndOpensItOnly(t *testing.T) {
	f := flowWithContent()
	for _, s := range []SpecifyStage{StageIntent, StageScope, StageStories, StageValidation} {
		f = step(t, f, validateEvent(s, 3))
	}
	want, issues := RenderSpecifyPrompt(f.Preparation.Content)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	if f.Phase != PhaseReview || f.LastStablePhase != PhaseReview || f.Preparation.Stage != StageReview {
		t.Fatalf("phase %s stage %s", f.Phase, f.Preparation.Stage)
	}
	if f.ProposedPrompt != want || f.Preparation.Review == nil || f.Preparation.Review.Revision != 3 || f.Preparation.Review.PromptHash != SpecifyPromptHash(want) {
		t.Fatalf("review = %+v", f.Preparation.Review)
	}
	if f.ApprovedPrompt != "" || f.ApprovedHash != "" || f.DispatchID != "" || f.ExecutionGranted {
		t.Fatalf("a complete preparation is not an approval: %+v", f)
	}
	if c := checkFor(f, StageValidation); c == nil || !c.Passed || c.InputHash != SpecifyInputHash(f.Preparation.Content, StageValidation) {
		t.Fatalf("validation check = %+v", c)
	}
	// The review is gone as soon as the content changes.
	again := step(t, f, prepareEvent(StageStories, 3, SpecifyContent{Stories: completeSpecifyContent().Stories}))
	if again.Preparation.Review == nil || again.Phase != PhaseReview {
		t.Fatalf("an identical edit must not drop the review: %s", again.Phase)
	}
	st := completeSpecifyContent().Stories
	st.Stories[0].Need = "otra necesidad"
	changed := step(t, f, prepareEvent(StageStories, 3, SpecifyContent{Stories: st}))
	if changed.Preparation.Review != nil || changed.ProposedPrompt != "" || changed.Phase != PhaseCaptured {
		t.Fatalf("review survived a content change: %+v", changed.Preparation.Review)
	}
}

func TestValidate_BlockingPendingAndOmissionsKeepTheFinalCheckClosed(t *testing.T) {
	f := flowWithContent()
	f.Preparation.Content.Scope.Pending.Items = append(f.Preparation.Content.Scope.Pending.Items,
		SpecifyPending{ID: "P2", Description: "Formato", Blocking: true, Resolution: PendingOpen})
	f.Questions = specifyQuestions(1)
	f.Phase, f.LastStablePhase = PhaseQuestions, PhaseQuestions
	f = step(t, f, Event{Kind: "skip_rest"}) // omitting never resolves the block
	for _, s := range []SpecifyStage{StageIntent, StageScope, StageStories, StageValidation} {
		f = step(t, f, validateEvent(s, 3))
	}
	c := checkFor(f, StageValidation)
	if c == nil || c.Passed || !issueAt(c.Missing, "blocking_pending", "scope.pending.items[1]") {
		t.Fatalf("validation = %+v", c)
	}
	if f.Phase != PhaseCaptured || f.Preparation.Stage != StageValidation || f.ProposedPrompt != "" || f.Preparation.Review != nil {
		t.Fatalf("a blocked preparation opened a review: phase %s stage %s", f.Phase, f.Preparation.Stage)
	}
	// The cumulative budget stays used after the failure: no new round opens.
	f.Questions = specifyQuestions(3)
	f.Answers = make([]Answer, 3)
	if _, _, err := specifyReduce(t, f, Event{Kind: "questions_generated", Questions: specifyQuestions(1)}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("a fourth question = %v", err)
	}
}

func TestValidate_RevisionPhaseAndClaimGuards(t *testing.T) {
	if _, _, err := specifyReduce(t, flowWithContent(), validateEvent(StageIntent, 2)); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale revision = %v", err)
	}
	q := flowWithContent()
	q.Phase = PhaseQuestions
	claimed := flowWithContent()
	claimed.DispatchClaimOwner = "o"
	refine := RefinementFlow{Mode: ModeRefine, Phase: PhaseCaptured}
	for name, f := range map[string]RefinementFlow{"questions pending": q, "claimed": claimed, "refine": refine} {
		if _, _, err := specifyReduce(t, f, validateEvent(StageIntent, f.Revision)); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s = %v, want ErrInvalidTransition", name, err)
		}
	}
	future := flowWithContent()
	future.Preparation.SchemaVersion = 9
	if _, _, err := specifyReduce(t, future, validateEvent(StageIntent, 3)); !errors.Is(err, ErrUnsupportedPreparation) {
		t.Fatalf("newer preparation = %v", err)
	}
	f := flowWithContent()
	before := f.Preparation.Clone()
	step(t, f, validateEvent(StageIntent, 3))
	if len(f.Preparation.Checks) != len(before.Checks) {
		t.Fatal("Reduce mutated the flow it received")
	}
}

func TestPrepareSet_ScopeEditCannotDropAnOpenBlockingPending(t *testing.T) {
	f := flowWithContent()
	f.Preparation.Content.Scope.Pending.Items = []SpecifyPending{{ID: "P1", Description: "d", Blocking: true, Resolution: PendingOpen}}
	scope := completeSpecifyContent().Scope
	scope.Pending = PendingList{Declared: true, Items: []SpecifyPending{}}
	if _, _, err := specifyReduce(t, f, prepareEvent(StageScope, 3, SpecifyContent{Scope: scope})); !errors.Is(err, ErrBlockingPending) {
		t.Fatalf("silent removal = %v, want ErrBlockingPending", err)
	}
	scope.Pending.Items = []SpecifyPending{{ID: "P1", Description: "d", Blocking: true, Resolution: PendingResolved, ResolutionNote: "decidido: CSV"}}
	got := step(t, f, prepareEvent(StageScope, 3, SpecifyContent{Scope: scope}))
	if got.Preparation.Content.Scope.Pending.Items[0].Resolution != PendingResolved {
		t.Fatalf("a documented resolution must be accepted: %+v", got.Preparation.Content.Scope.Pending)
	}
	if f.Preparation.Content.Scope.Pending.Items[0].Resolution != PendingOpen {
		t.Fatal("the input flow changed")
	}
}

// Content changed outside prepare (a sanitize pass) leaves stale checks; the
// rewind drops them so the suggested validation can pass (C-004, ACR 042).
func TestRewindStaleChecks_BackToTheFirstOutdatedStage(t *testing.T) {
	f := flowWithContent()
	f = step(t, f, validateEvent(StageIntent, 3))
	f = step(t, f, validateEvent(StageScope, 3))
	if f.Preparation.Stage != StageStories {
		t.Fatalf("stage = %s, want stories", f.Preparation.Stage)
	}
	if RewindStaleChecks(f.Preparation) {
		t.Fatal("current checks were rewound")
	}
	f.Preparation.Content.Scope.Included = []string{"[REDACTED:key]"}
	if !RewindStaleChecks(f.Preparation) || f.Preparation.Stage != StageScope || checkFor(f, StageScope) != nil || checkFor(f, StageIntent) == nil {
		t.Fatalf("after rewind: stage %s, checks %+v", f.Preparation.Stage, f.Preparation.Checks)
	}
	step(t, f, validateEvent(StageScope, 3))
}
