package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestPayloadPolicyFor_OnlyPrivateSpecifyIsEphemeral(t *testing.T) {
	cases := []struct {
		mode    SocraticaMode
		private bool
		want    PayloadPolicy
	}{
		{ModeSpecify, true, PayloadEphemeral},
		{ModeSpecify, false, PayloadPersistent},
		{ModeRefine, true, PayloadPersistent},
		{ModeScrutiny, true, PayloadPersistent},
	}
	for _, c := range cases {
		if got := PayloadPolicyFor(c.mode, c.private); got != c.want {
			t.Errorf("PayloadPolicyFor(%s, %v) = %s, want %s", c.mode, c.private, got, c.want)
		}
	}
	if got := (RefinementFlow{}).EffectivePayloadPolicy(); got != PayloadPersistent {
		t.Errorf("legacy policy = %s, want persistent", got)
	}
}

func samplePreparation() *SpecifyPreparation {
	p := NewSpecifyPreparation()
	p.Content = SpecifyContent{
		Intent: &SpecifyIntent{Objective: "o", ExpectedResult: "r", Actors: []SpecifyActor{{ID: "A1", Name: "n"}}},
		Scope: &SpecifyScope{Included: []string{"i"}, Excluded: DeclaredList{Declared: true, Items: []string{"e"}},
			Pending: PendingList{Declared: true, Items: []SpecifyPending{{ID: "P1", Description: "d", Blocking: true, Resolution: PendingOpen}}}},
		Stories: &SpecifyStories{
			Stories:    []SpecifyStory{{ID: "US1", ActorID: "A1", Need: "n", Outcome: "o"}},
			Criteria:   []SpecifyCriterion{{ID: "C1", StoryIDs: []string{"US1"}, Condition: "c", ExpectedResult: "r"}},
			Deliveries: []SpecifyDelivery{{ID: "D1", Title: "t", Outcome: "o", StoryIDs: []string{"US1"}, CriterionIDs: []string{"C1"}}},
		},
	}
	p.Checks = []SpecifyCheck{{Stage: StageIntent, InputHash: "h", Passed: false, Missing: []SpecifyIssue{{Code: "missing", Path: "intent.objective"}}}}
	p.Review = &SpecifyReview{Revision: 1, PromptHash: "x"}
	p.Outcome = &SpecifyOutcome{DispatchID: "d", Status: OutcomeCompleted, Traceability: []SpecifyTrace{{CriterionID: "C1", StoryIDs: []string{"US1"}}}}
	return p
}

func TestSpecifyPreparationClone_IsDeep(t *testing.T) {
	orig := samplePreparation()
	c := orig.Clone()
	if !reflect.DeepEqual(orig, c) {
		t.Fatalf("clone differs:\n%#v\n%#v", orig, c)
	}
	c.Content.Intent.Actors[0].Name = "x"
	c.Content.Scope.Excluded.Items[0] = "x"
	c.Content.Scope.Pending.Items[0].Resolution = PendingResolved
	c.Content.Stories.Criteria[0].StoryIDs[0] = "x"
	c.Content.Stories.Deliveries[0].CriterionIDs[0] = "x"
	c.Checks[0].Missing[0].Path = "x"
	c.Review.Revision = 9
	c.Outcome.Traceability[0].StoryIDs[0] = "x"
	if !reflect.DeepEqual(orig, samplePreparation()) {
		t.Fatalf("mutating the clone changed the original: %#v", orig)
	}
	if (*SpecifyPreparation)(nil).Clone() != nil {
		t.Fatal("nil clone must stay nil")
	}
}

func TestReduce_DoesNotShareThePreparation(t *testing.T) {
	f := RefinementFlow{Mode: ModeSpecify, Phase: PhaseCaptured, Preparation: samplePreparation()}
	got, _, err := Reduce(f, Event{Kind: "resume"}, time.Unix(1, 0), func() string { return "id" })
	if err != nil {
		t.Fatal(err)
	}
	got.Preparation.Content.Intent.Objective = "changed"
	if f.Preparation.Content.Intent.Objective != "o" {
		t.Fatal("Reduce returned a preparation aliased with its input")
	}
}

func TestParseSpecifySection(t *testing.T) {
	ok := `{"objective":"o","expected_result":"r","actors":[{"id":"A1","name":"n"}]}`
	c, err := ParseSpecifySection(StageIntent, []byte(ok))
	if err != nil || c.Intent == nil || c.Intent.Actors[0].ID != "A1" {
		t.Fatalf("valid intent = %+v, %v", c, err)
	}
	// An incomplete draft is kept: its gaps are reported by the stage checks.
	if c, err = ParseSpecifySection(StageIntent, []byte(`{"objective":"o"}`)); err != nil || c.Intent.Objective != "o" {
		t.Fatalf("draft intent = %+v, %v", c, err)
	}
	for name, raw := range map[string]string{
		"computed stage":      `{"objective":"o","stage":"approved"}`,
		"injected ready mark": `{"objective":"o","ready":true}`,
		"wrong type":          `{"objective":5}`,
		"trailing data":       ok + ` {"x":1}`,
		"not an object":       `"texto"`,
		// A missing or null value must not wipe the section (C-009, ACR 042).
		"null value":    `null`,
		"missing value": ``,
		"array value":   `[]`,
	} {
		if _, err = ParseSpecifySection(StageIntent, []byte(raw)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err = ParseSpecifySection(StageValidation, []byte(`{}`)); err == nil {
		t.Error("section validation accepted")
	}
	if _, err = ParseSpecifySection(StageScope, []byte(`{"included":["a"],"pending":{"declared":true,"items":[{"id":"P1","blocking":true,"resolution":"open","approved":true}]}}`)); err == nil {
		t.Error("unknown field in a nested pending accepted")
	}
}

func TestProtectSpecifyContent(t *testing.T) {
	c := SpecifyContent{Intent: &SpecifyIntent{Objective: "x <private>interno-042</private> y", Actors: []SpecifyActor{{ID: "A1", Name: "clave AKIAIOSFODNN7EXAMPLE"}}}}
	got, err := ProtectSpecifyContent(c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Intent.Objective == c.Intent.Objective || got.Intent.Actors[0].Name == c.Intent.Actors[0].Name {
		t.Fatalf("not protected: %+v", got.Intent)
	}
	if c.Intent.Objective != "x <private>interno-042</private> y" {
		t.Fatal("the input must not be mutated")
	}
}

func prepareEvent(section SpecifyStage, rev int, c SpecifyContent) Event {
	return Event{Kind: "prepare_set", Section: section, ExpectedRevision: rev, Content: c}
}

func TestPrepareSet_RevisionConflictIsAtomic(t *testing.T) {
	f := newSpecifyFlow()
	f.Revision = 2
	got, _, err := specifyReduce(t, f, prepareEvent(StageIntent, 1, SpecifyContent{Intent: &SpecifyIntent{Objective: "o"}}))
	if !errors.Is(err, ErrRevisionConflict) || got.Revision != 2 || got.Preparation.Content.Intent != nil {
		t.Fatalf("stale revision = %v rev %d, want conflict and nothing applied", err, got.Revision)
	}
}

func TestPrepareSet_IdenticalReplacementKeepsRevision(t *testing.T) {
	f := newSpecifyFlow()
	c := SpecifyContent{Intent: &SpecifyIntent{Objective: "o", Actors: []SpecifyActor{{ID: "A1", Name: "n"}}}}
	f, _, err := specifyReduce(t, f, prepareEvent(StageIntent, 0, c))
	if err != nil || f.Revision != 1 {
		t.Fatalf("first set rev %d, %v", f.Revision, err)
	}
	f.Preparation.Checks = []SpecifyCheck{{Stage: StageIntent, Passed: true}}
	again, _, err := specifyReduce(t, f, prepareEvent(StageIntent, 1, SpecifyContent{Intent: &SpecifyIntent{Objective: "o", Actors: []SpecifyActor{{ID: "A1", Name: "n"}}}}))
	if err != nil || again.Revision != 1 || len(again.Preparation.Checks) != 1 {
		t.Fatalf("identical set rev %d checks %d, %v; want a no-op", again.Revision, len(again.Preparation.Checks), err)
	}
}

func TestPrepareSet_MaterialChangeInvalidatesDownstream(t *testing.T) {
	f := newSpecifyFlow()
	f.Revision = 4
	f.Phase, f.LastStablePhase = PhaseReview, PhaseReview
	f.ProposedPrompt, f.ApprovedPrompt, f.ApprovedHash, f.DispatchID = "p", "p", "h", "d1"
	f.Preparation.Stage = StageReview
	f.Preparation.Content = completeSpecifyContent()
	f.Preparation.Review = &SpecifyReview{Revision: 4, PromptHash: "x"}
	f.Preparation.Checks = []SpecifyCheck{{Stage: StageIntent, Passed: true}, {Stage: StageScope, Passed: true}, {Stage: StageStories, Passed: true}, {Stage: StageValidation, Passed: true}}
	scope := completeSpecifyContent().Scope
	scope.Included = []string{"otro alcance"}
	got, _, err := specifyReduce(t, f, prepareEvent(StageScope, 4, SpecifyContent{Scope: scope}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 5 || got.Phase != PhaseCaptured || got.LastStablePhase != PhaseCaptured || got.Preparation.Stage != StageScope {
		t.Fatalf("rev %d phase %s stage %s", got.Revision, got.Phase, got.Preparation.Stage)
	}
	if len(got.Preparation.Checks) != 1 || got.Preparation.Checks[0].Stage != StageIntent {
		t.Fatalf("checks = %+v; only the intent check survives a scope edit", got.Preparation.Checks)
	}
	if got.Preparation.Review != nil || got.ProposedPrompt != "" || got.ApprovedPrompt != "" || got.ApprovedHash != "" || got.DispatchID != "" {
		t.Fatalf("review/proposal/approval survived: %+v", got)
	}
	if f.Preparation.Content.Scope.Included[0] == "otro alcance" || f.Preparation.Review == nil {
		t.Fatal("Reduce mutated the flow it received")
	}
}

func TestPrepareSet_ClosedStatesAndOtherModes(t *testing.T) {
	c := SpecifyContent{Intent: &SpecifyIntent{Objective: "o"}}
	claimed := newSpecifyFlow()
	claimed.DispatchClaimOwner = "owner"
	acked := newSpecifyFlow()
	acked.DispatchAck = true
	rewriting := newSpecifyFlow()
	rewriting.Phase = PhaseRewriting
	refine := RefinementFlow{Mode: ModeRefine, Phase: PhaseCaptured}
	noPrep := newSpecifyFlow()
	noPrep.Preparation = nil
	future := newSpecifyFlow()
	future.Preparation.SchemaVersion = 99
	for name, f := range map[string]RefinementFlow{"claimed": claimed, "consumed": acked, "rewriting": rewriting, "refine": refine, "no preparation": noPrep} {
		if _, _, err := specifyReduce(t, f, prepareEvent(StageIntent, 0, c)); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s = %v, want ErrInvalidTransition", name, err)
		}
	}
	if _, _, err := specifyReduce(t, future, prepareEvent(StageIntent, 0, c)); !errors.Is(err, ErrUnsupportedPreparation) {
		t.Errorf("newer preparation = %v", err)
	}
	if _, _, err := specifyReduce(t, newSpecifyFlow(), prepareEvent(StageScope, 0, c)); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("section without content = %v", err)
	}
	if _, _, err := specifyReduce(t, newSpecifyFlow(), prepareEvent(StageValidation, 0, c)); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("validation is not a settable section: %v", err)
	}
}

func TestEffectivePayloadPolicy_KeepsExplicitPolicy(t *testing.T) {
	if got := (RefinementFlow{PayloadPolicy: PayloadEphemeral}).EffectivePayloadPolicy(); got != PayloadEphemeral {
		t.Fatalf("policy = %s, want ephemeral", got)
	}
}
