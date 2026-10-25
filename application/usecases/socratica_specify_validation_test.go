package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/domain"
)

const (
	scopeJSON   = `{"included":["Exportar el historial propio"],"excluded":{"declared":true,"items":["Historiales ajenos"]},"constraints":{"declared":true,"items":[]},"assumptions":{"declared":true,"items":[]},"pending":{"declared":true,"items":[]}}`
	storiesJSON = `{"stories":[{"id":"US1","actor_id":"A1","need":"Exportar mi historial","outcome":"Consultarlo fuera"}],` +
		`"criteria":[{"id":"C1","story_ids":["US1"],"condition":"Tiene registros","expected_result":"El archivo los contiene"}],` +
		`"deliveries":[{"id":"D1","title":"Exportación","outcome":"Historial exportable","story_ids":["US1"],"criterion_ids":["C1"]}]}`
)

func startedSpecify(t *testing.T) (*SocraticaFlow, *socraticaFakeRepo, string) {
	t.Helper()
	u, r := specifyFixture()
	res, err := u.Start(context.Background(), "p", "codex", domain.ModeSpecify, "exportar historial")
	if err != nil {
		t.Fatal(err)
	}
	return u, r, res.Flow.ID
}

func setAll(t *testing.T, u *SocraticaFlow, id string) {
	t.Helper()
	ctx := context.Background()
	for i, s := range []struct {
		section domain.SpecifyStage
		raw     string
	}{{domain.StageIntent, intentJSON}, {domain.StageScope, scopeJSON}, {domain.StageStories, storiesJSON}} {
		if _, err := u.PrepareSet(ctx, id, i, s.section, []byte(s.raw)); err != nil {
			t.Fatalf("%s: %v", s.section, err)
		}
	}
}

func TestSpecifyValidate_FullPathReachesReviewWithoutApproval(t *testing.T) {
	ctx := context.Background()
	u, r, id := startedSpecify(t)
	setAll(t, u, id)
	var res SocraticaResult
	var err error
	for _, s := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope, domain.StageStories, domain.StageValidation} {
		if res, err = u.PrepareValidate(ctx, id, 3, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	f := res.Flow
	if f.Phase != domain.PhaseReview || f.Preparation.Stage != domain.StageReview || f.ProposedPrompt == "" || f.Preparation.Review == nil {
		t.Fatalf("flow = phase %s stage %s", f.Phase, f.Preparation.Stage)
	}
	if f.ApprovedPrompt != "" || f.DispatchID != "" || f.ExecutionGranted || r.flow.ApprovedPrompt != "" {
		t.Fatalf("a checked preparation is not an approval: %+v", f)
	}
	view := SpecifyView(f)
	if view.ReviewHash != domain.SpecifyPromptHash(f.ProposedPrompt) || len(view.PendingChecks) != 0 {
		t.Fatalf("view = %+v", view)
	}
	if !strings.Contains(res.Next, "persona confirma") || !strings.Contains(res.Next, "no ejecutes") {
		t.Fatalf("next = %q", res.Next)
	}
	// Persisted, not just returned.
	if r.flow.Phase != domain.PhaseReview || r.flow.Preparation.Review == nil {
		t.Fatal("the review was not saved with the transition")
	}
}

func TestSpecifyValidate_FailureIsAResultWithMissingItems(t *testing.T) {
	ctx := context.Background()
	u, _, id := startedSpecify(t)
	if _, err := u.PrepareSet(ctx, id, 0, domain.StageIntent, []byte(`{"objective":"Exportar","actors":[{"id":"A1","name":"Titular"}]}`)); err != nil {
		t.Fatal(err)
	}
	res, err := u.PrepareValidate(ctx, id, 1, domain.StageIntent)
	if err != nil {
		t.Fatalf("a failed check is not an error: %v", err)
	}
	var missing []string
	for _, c := range SpecifyView(res.Flow).PendingChecks {
		missing = append(missing, c.Code+"@"+c.Path)
	}
	for _, want := range []string{"missing@intent.expected_result", "section_missing@scope", "section_missing@stories"} {
		if !contains(missing, want) {
			t.Errorf("pending checks %v lack %s", missing, want)
		}
	}
	if res.Flow.Preparation.Stage != domain.StageIntent || res.Flow.Preparation.Content.Intent == nil {
		t.Fatal("the draft must be kept at the failed stage")
	}
	if !strings.Contains(res.Next, "pending_checks") {
		t.Fatalf("next = %q", res.Next)
	}
	if _, err = u.PrepareValidate(ctx, id, 1, domain.StageScope); !errors.Is(err, domain.ErrStageBlocked) {
		t.Fatalf("skipping to scope = %v", err)
	}
}

func TestSpecifyValidate_NextTellsWhatToDo(t *testing.T) {
	ctx := context.Background()
	u, _, id := startedSpecify(t)
	if _, err := u.PrepareSet(ctx, id, 0, domain.StageIntent, []byte(intentJSON)); err != nil {
		t.Fatal(err)
	}
	res, _ := u.Get(ctx, id)
	if !strings.Contains(res.Next, "prepare(validate, stage=intent, expected_revision=1)") {
		t.Fatalf("a complete stage should say to validate it: %q", res.Next)
	}
	res, err := u.PrepareValidate(ctx, id, 1, domain.StageIntent)
	if err != nil || res.Flow.Preparation.Stage != domain.StageScope || !strings.Contains(res.Next, "scope") {
		t.Fatalf("after intent: stage %s next %q, %v", res.Flow.Preparation.Stage, res.Next, err)
	}
}

func TestSpecifyValidate_StaleRevisionAndOtherModes(t *testing.T) {
	ctx := context.Background()
	u, _, id := startedSpecify(t)
	if _, err := u.PrepareValidate(ctx, id, 7, domain.StageIntent); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("stale revision = %v", err)
	}
	refine, _ := socraticaFixture()
	if _, err := refine.PrepareValidate(ctx, "f", 0, domain.StageIntent); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("validate on refine = %v", err)
	}
}

func TestSpecifyValidate_ScopeEditCannotDropABlockingPending(t *testing.T) {
	ctx := context.Background()
	u, r, id := startedSpecify(t)
	blocked := strings.Replace(scopeJSON, `"pending":{"declared":true,"items":[]}`, `"pending":{"declared":true,"items":[{"id":"P1","description":"Formato","blocking":true,"resolution":"open"}]}`, 1)
	if _, err := u.PrepareSet(ctx, id, 0, domain.StageScope, []byte(blocked)); err != nil {
		t.Fatal(err)
	}
	if _, err := u.PrepareSet(ctx, id, 1, domain.StageScope, []byte(scopeJSON)); !errors.Is(err, domain.ErrBlockingPending) {
		t.Fatalf("silent removal = %v, want ErrBlockingPending", err)
	}
	if len(r.flow.Preparation.Content.Scope.Pending.Items) != 1 || r.flow.Revision != 1 {
		t.Fatal("a rejected edit changed the draft")
	}
}
