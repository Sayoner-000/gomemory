package domain

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

type contentMutation func(*SpecifyContent)

func issueAt(issues []SpecifyIssue, code, path string) bool {
	for _, i := range issues {
		if i.Code == code && i.Path == path {
			return true
		}
	}
	return false
}

func TestValidateSpecifyStage_CompleteExamplePassesEveryStage(t *testing.T) {
	c := completeSpecifyContent()
	for _, stage := range []SpecifyStage{StageIntent, StageScope, StageStories, StageValidation} {
		if issues := ValidateSpecifyStage(c, stage); len(issues) != 0 {
			t.Errorf("%s: %+v", stage, issues)
		}
	}
}

func TestValidateSpecifyStage_Minimums(t *testing.T) {
	cases := []struct {
		name   string
		stage  SpecifyStage
		mutate contentMutation
		code   string
		path   string
	}{
		{"empty objective", StageIntent, func(c *SpecifyContent) { c.Intent.Objective = "" }, "missing", "intent.objective"},
		{"spaces are not information", StageIntent, func(c *SpecifyContent) { c.Intent.ExpectedResult = " \t\n " }, "missing", "intent.expected_result"},
		{"punctuation-only filler", StageIntent, func(c *SpecifyContent) { c.Intent.Objective = "..." }, "placeholder", "intent.objective"},
		{"TBD filler", StageIntent, func(c *SpecifyContent) { c.Intent.Objective = " TBD " }, "placeholder", "intent.objective"},
		{"por definir filler", StageIntent, func(c *SpecifyContent) { c.Intent.ExpectedResult = "Por definir" }, "placeholder", "intent.expected_result"},
		{"essential marker", StageIntent, func(c *SpecifyContent) { c.Intent.Objective = "Exportar [formato] del historial" }, "essential_marker", "intent.objective"},
		{"no actors", StageIntent, func(c *SpecifyContent) { c.Intent.Actors = nil }, "missing", "intent.actors"},
		{"actor without name", StageIntent, func(c *SpecifyContent) { c.Intent.Actors[0].Name = "  " }, "missing", "intent.actors[0].name"},
		{"actor without id", StageIntent, func(c *SpecifyContent) { c.Intent.Actors[0].ID = "" }, "missing", "intent.actors[0].id"},
		{"duplicate actor ids", StageIntent, func(c *SpecifyContent) {
			c.Intent.Actors = append(c.Intent.Actors, SpecifyActor{ID: "A1", Name: "Otro"})
		}, "duplicate_id", "intent.actors[1].id"},
		{"no included scope", StageScope, func(c *SpecifyContent) { c.Scope.Included = nil }, "missing", "scope.included"},
		{"blank included item", StageScope, func(c *SpecifyContent) { c.Scope.Included = []string{"  "} }, "missing", "scope.included[0]"},
		{"excluded not declared", StageScope, func(c *SpecifyContent) { c.Scope.Excluded = DeclaredList{} }, "not_declared", "scope.excluded"},
		{"constraints not declared", StageScope, func(c *SpecifyContent) { c.Scope.Constraints = DeclaredList{Items: []string{"x"}} }, "not_declared", "scope.constraints"},
		{"assumptions not declared", StageScope, func(c *SpecifyContent) { c.Scope.Assumptions = DeclaredList{} }, "not_declared", "scope.assumptions"},
		{"pending not declared", StageScope, func(c *SpecifyContent) { c.Scope.Pending = PendingList{} }, "not_declared", "scope.pending"},
		{"blank declared item", StageScope, func(c *SpecifyContent) { c.Scope.Excluded.Items = []string{""} }, "missing", "scope.excluded.items[0]"},
		{"pending without id", StageScope, func(c *SpecifyContent) { c.Scope.Pending.Items[0].ID = "" }, "missing", "scope.pending.items[0].id"},
		{"pending invalid resolution", StageScope, func(c *SpecifyContent) { c.Scope.Pending.Items[0].Resolution = "maybe" }, "invalid_value", "scope.pending.items[0].resolution"},
		{"resolved pending without note", StageScope, func(c *SpecifyContent) { c.Scope.Pending.Items[0].ResolutionNote = " " }, "missing", "scope.pending.items[0].resolution_note"},
		{"duplicate pending ids", StageScope, func(c *SpecifyContent) {
			c.Scope.Pending.Items = append(c.Scope.Pending.Items, c.Scope.Pending.Items[0])
		}, "duplicate_id", "scope.pending.items[1].id"},
		{"no stories", StageStories, func(c *SpecifyContent) { c.Stories.Stories = nil }, "missing", "stories.stories"},
		{"no criteria", StageStories, func(c *SpecifyContent) { c.Stories.Criteria = nil }, "missing", "stories.criteria"},
		{"no deliveries", StageStories, func(c *SpecifyContent) { c.Stories.Deliveries = nil }, "missing", "stories.deliveries"},
		{"story need blank", StageStories, func(c *SpecifyContent) { c.Stories.Stories[0].Need = "" }, "missing", "stories.stories[0].need"},
		{"criterion expected result blank", StageStories, func(c *SpecifyContent) { c.Stories.Criteria[1].ExpectedResult = "" }, "missing", "stories.criteria[1].expected_result"},
		{"delivery title blank", StageStories, func(c *SpecifyContent) { c.Stories.Deliveries[0].Title = "" }, "missing", "stories.deliveries[0].title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := completeSpecifyContent()
			tc.mutate(&c)
			if issues := ValidateSpecifyStage(c, tc.stage); !issueAt(issues, tc.code, tc.path) {
				t.Fatalf("want %s at %s, got %+v", tc.code, tc.path, issues)
			}
		})
	}
}

func TestValidateSpecifyStage_ReferencesAndCoverage(t *testing.T) {
	cases := []struct {
		name   string
		mutate contentMutation
		code   string
		path   string
	}{
		{"story with unknown actor", func(c *SpecifyContent) { c.Stories.Stories[0].ActorID = "A9" }, "invalid_reference", "stories.stories[0].actor_id"},
		{"duplicate story ids", func(c *SpecifyContent) {
			c.Stories.Stories = append(c.Stories.Stories, c.Stories.Stories[0])
		}, "duplicate_id", "stories.stories[1].id"},
		{"duplicate criterion ids", func(c *SpecifyContent) { c.Stories.Criteria[1].ID = "C1" }, "duplicate_id", "stories.criteria[1].id"},
		{"duplicate delivery ids", func(c *SpecifyContent) {
			c.Stories.Deliveries = append(c.Stories.Deliveries, c.Stories.Deliveries[0])
		}, "duplicate_id", "stories.deliveries[1].id"},
		{"criterion without story", func(c *SpecifyContent) { c.Stories.Criteria[0].StoryIDs = nil }, "missing", "stories.criteria[0].story_ids"},
		{"criterion unknown story", func(c *SpecifyContent) { c.Stories.Criteria[0].StoryIDs = []string{"US9"} }, "invalid_reference", "stories.criteria[0].story_ids[0]"},
		{"story without criterion", func(c *SpecifyContent) {
			c.Stories.Stories = append(c.Stories.Stories, SpecifyStory{ID: "US2", ActorID: "A1", Need: "n", Outcome: "o"})
		}, "uncovered_story", "stories.stories[1]"},
		{"delivery unknown story", func(c *SpecifyContent) { c.Stories.Deliveries[0].StoryIDs = []string{"US1", "US7"} }, "invalid_reference", "stories.deliveries[0].story_ids[1]"},
		{"delivery unknown criterion", func(c *SpecifyContent) { c.Stories.Deliveries[0].CriterionIDs = []string{"C1", "C8"} }, "invalid_reference", "stories.deliveries[0].criterion_ids[1]"},
		{"delivery without criteria", func(c *SpecifyContent) { c.Stories.Deliveries[0].CriterionIDs = nil }, "missing", "stories.deliveries[0].criterion_ids"},
		// References are uniqueItems in the schema (C-008, ACR 042 r2).
		{"criterion repeats a story", func(c *SpecifyContent) {
			c.Stories.Criteria[0].StoryIDs = append(c.Stories.Criteria[0].StoryIDs, c.Stories.Criteria[0].StoryIDs[0])
		}, "duplicate_id", "stories.criteria[0].story_ids[1]"},
		{"delivery repeats a criterion", func(c *SpecifyContent) {
			c.Stories.Deliveries[0].CriterionIDs = append(c.Stories.Deliveries[0].CriterionIDs, c.Stories.Deliveries[0].CriterionIDs[0])
		}, "duplicate_id", "stories.deliveries[0].criterion_ids[" + strconv.Itoa(len(completeSpecifyContent().Stories.Deliveries[0].CriterionIDs)) + "]"},
		{"story in no delivery", func(c *SpecifyContent) {
			c.Stories.Stories = append(c.Stories.Stories, SpecifyStory{ID: "US2", ActorID: "A1", Need: "n", Outcome: "o"})
			c.Stories.Criteria = append(c.Stories.Criteria, SpecifyCriterion{ID: "C3", StoryIDs: []string{"US2"}, Condition: "c", ExpectedResult: "r"})
		}, "story_without_delivery", "stories.stories[1]"},
		{"delivery claims a foreign criterion", func(c *SpecifyContent) {
			c.Stories.Stories = append(c.Stories.Stories, SpecifyStory{ID: "US2", ActorID: "A1", Need: "n", Outcome: "o"})
			c.Stories.Criteria = append(c.Stories.Criteria, SpecifyCriterion{ID: "C3", StoryIDs: []string{"US2"}, Condition: "c", ExpectedResult: "r"})
			c.Stories.Deliveries[0].CriterionIDs = append(c.Stories.Deliveries[0].CriterionIDs, "C3")
		}, "foreign_criterion", "stories.deliveries[0].criterion_ids[2]"},
		{"delivery story not checked by its criteria", func(c *SpecifyContent) {
			c.Stories.Stories = append(c.Stories.Stories, SpecifyStory{ID: "US2", ActorID: "A1", Need: "n", Outcome: "o"})
			c.Stories.Criteria = append(c.Stories.Criteria, SpecifyCriterion{ID: "C3", StoryIDs: []string{"US2"}, Condition: "c", ExpectedResult: "r"})
			c.Stories.Deliveries[0].StoryIDs = append(c.Stories.Deliveries[0].StoryIDs, "US2")
		}, "delivery_gap", "stories.deliveries[0].story_ids[1]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := completeSpecifyContent()
			tc.mutate(&c)
			if issues := ValidateSpecifyStage(c, StageStories); !issueAt(issues, tc.code, tc.path) {
				t.Fatalf("want %s at %s, got %+v", tc.code, tc.path, issues)
			}
		})
	}
	c := completeSpecifyContent()
	c.Intent = nil
	if issues := ValidateSpecifyStage(c, StageStories); !issueAt(issues, "section_missing", "intent") {
		t.Fatalf("stories without the intent actors to reference: %+v", issues)
	}
}

func TestValidateSpecifyStage_BlockingPendingsAndMarkers(t *testing.T) {
	open := func(c *SpecifyContent, blocking bool) {
		c.Scope.Pending.Items = append(c.Scope.Pending.Items, SpecifyPending{ID: "P2", Description: "Decidir el formato", Blocking: blocking, Resolution: PendingOpen})
	}
	c := completeSpecifyContent()
	open(&c, true)
	if issues := ValidateSpecifyStage(c, StageScope); len(issues) != 0 {
		t.Fatalf("a blocking pending can stay visible while the scope is prepared: %+v", issues)
	}
	if issues := ValidateSpecifyStage(c, StageValidation); !issueAt(issues, "blocking_pending", "scope.pending.items[1]") {
		t.Fatalf("validation must stop on an open blocking pending: %+v", issues)
	}
	c = completeSpecifyContent()
	open(&c, false)
	if issues := ValidateSpecifyStage(c, StageValidation); len(issues) != 0 {
		t.Fatalf("a non-blocking open pending does not stop validation: %+v", issues)
	}
	c = completeSpecifyContent()
	c.Scope.Pending.Items[0].Blocking = true // resolved with a note
	if issues := ValidateSpecifyStage(c, StageValidation); len(issues) != 0 {
		t.Fatalf("a resolved blocking pending is not open: %+v", issues)
	}
	c = completeSpecifyContent()
	c.Stories.Stories[0].Need = "Exportar en [formato por decidir]"
	if issues := ValidateSpecifyStage(c, StageValidation); !issueAt(issues, "essential_marker", "stories.stories[0].need") {
		t.Fatalf("an unresolved marker must block validation: %+v", issues)
	}
	// Not markers: indexes, links and the privacy redaction label.
	c = completeSpecifyContent()
	c.Stories.Stories[0].Need = "Leer items[0] y [la guía](https://x.test) con [REDACTED:key]"
	if issues := ValidateSpecifyStage(c, StageValidation); len(issues) != 0 {
		t.Fatalf("false marker: %+v", issues)
	}
}

func TestValidateSpecifyStage_IssuesAreDeterministicAndBounded(t *testing.T) {
	c := completeSpecifyContent()
	c.Intent.Objective, c.Stories.Stories[0].ActorID = "", "A9"
	a, b := ValidateSpecifyStage(c, StageValidation), ValidateSpecifyStage(c, StageValidation)
	if len(a) != len(b) || len(a) < 2 {
		t.Fatalf("%+v / %+v", a, b)
	}
	for i := range a {
		if a[i] != b[i] || strings.TrimSpace(a[i].Message) == "" {
			t.Fatalf("issue %d differs or has no message: %+v %+v", i, a[i], b[i])
		}
	}
	if issues := ValidateSpecifyStage(c, "nonsense"); len(issues) != 1 || issues[0].Code != "unsupported_stage" {
		t.Fatalf("unknown stage = %+v", issues)
	}
}

func TestSpecifyInputHash_TracksOnlyItsProjection(t *testing.T) {
	base := completeSpecifyContent()
	hash := func(c SpecifyContent, s SpecifyStage) string { return SpecifyInputHash(c, s) }
	changedStories := completeSpecifyContent()
	changedStories.Stories.Stories[0].Need = "otra necesidad"
	changedIntent := completeSpecifyContent()
	changedIntent.Intent.Objective = "otro objetivo"
	if hash(base, StageIntent) != hash(changedStories, StageIntent) || hash(base, StageScope) != hash(changedStories, StageScope) {
		t.Fatal("editing stories must not change the intent or scope hashes")
	}
	if hash(base, StageStories) == hash(changedStories, StageStories) || hash(base, StageValidation) == hash(changedStories, StageValidation) {
		t.Fatal("editing stories must change the stories and validation hashes")
	}
	for _, s := range []SpecifyStage{StageIntent, StageScope, StageStories, StageValidation} {
		if hash(base, s) == hash(changedIntent, s) {
			t.Errorf("editing the intent must change the %s hash", s)
		}
		if hash(base, s) != hash(completeSpecifyContent(), s) {
			t.Errorf("%s hash is not deterministic", s)
		}
	}
}

func TestCheckPendingContinuity_NoSilentRemovalOrReclassification(t *testing.T) {
	old := &SpecifyScope{Pending: PendingList{Declared: true, Items: []SpecifyPending{
		{ID: "P1", Description: "d", Blocking: true, Resolution: PendingOpen},
		{ID: "P2", Description: "d", Blocking: false, Resolution: PendingOpen},
	}}}
	with := func(items ...SpecifyPending) *SpecifyScope {
		return &SpecifyScope{Pending: PendingList{Declared: true, Items: items}}
	}
	keep := SpecifyPending{ID: "P1", Description: "d", Blocking: true, Resolution: PendingOpen}
	p2 := SpecifyPending{ID: "P2", Description: "d", Resolution: PendingOpen}
	bad := map[string]*SpecifyScope{
		"removed":            with(p2),
		"list emptied":       with(),
		"downgraded no note": with(SpecifyPending{ID: "P1", Description: "d", Blocking: false, Resolution: PendingOpen}, p2),
		"resolved no note":   with(SpecifyPending{ID: "P1", Description: "d", Blocking: true, Resolution: PendingResolved}, p2),
		"renamed id":         with(SpecifyPending{ID: "P1b", Description: "d", Blocking: true, Resolution: PendingOpen}, p2),
		"downgraded blank":   with(SpecifyPending{ID: "P1", Description: "d", Blocking: false, Resolution: PendingOpen, ResolutionNote: "  "}, p2),
		"no pending section": nil,
	}
	for name, next := range bad {
		if err := CheckPendingContinuity(old, next); !errors.Is(err, ErrBlockingPending) {
			t.Errorf("%s = %v, want ErrBlockingPending", name, err)
		}
	}
	good := map[string]*SpecifyScope{
		"unchanged":            with(keep, p2),
		"resolved + note":      with(SpecifyPending{ID: "P1", Description: "d", Blocking: true, Resolution: PendingResolved, ResolutionNote: "decidido con la persona"}, p2),
		"downgraded + why":     with(SpecifyPending{ID: "P1", Description: "d", Blocking: false, Resolution: PendingOpen, ResolutionNote: "ya no afecta el alcance"}, p2),
		"non-blocking dropped": with(keep),
		"new pending added":    with(keep, p2, SpecifyPending{ID: "P3", Description: "n", Blocking: true, Resolution: PendingOpen}),
	}
	for name, next := range good {
		if err := CheckPendingContinuity(old, next); err != nil {
			t.Errorf("%s = %v, want nil", name, err)
		}
	}
	if err := CheckPendingContinuity(nil, with()); err != nil {
		t.Errorf("first scope = %v", err)
	}
}
