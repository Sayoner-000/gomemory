package usecases_test

import (
	"context"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHCD057RecordSocraticExam(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	v, err := env.uc.Start(ctx, usecases.HCDStartInput{Kind: "evolution", Lane: "full", Title: "Review", Why: "contract", Intent: "review"})
	if err != nil {
		t.Fatal(err)
	}
	editDoc(t, env, v, append(commonFill, storyFill...)...)
	v, err = env.uc.Show(ctx, v.Unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	in := usecases.HCDDefinitionReviewInput{ID: v.Unit.ID, Revision: domain.HCDReviewRevision(v.DefinitionRevision), OperationID: "exam-1", Kind: "socratic", Action: "complete", Client: "codex", Payload: domain.HCDDefinitionReviewPayload{Summary: "Context covers scope and <private>hidden</private> alternatives."}}
	env.uc.ConfigureDefinitionReviews(nil)
	if _, _, err := env.uc.RecordDefinitionReview(ctx, in); err == nil {
		t.Fatal("missing repository accepted")
	}
	repo := persistence.NewHCDStore(env.db, "p")
	env.uc.ConfigureDefinitionReviews(repo)
	path := filepath.Join(env.root, v.Unit.DocPath)
	before, _ := os.ReadFile(path)
	_, event, err := env.uc.RecordDefinitionReview(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(event.Payload.Summary, "hidden") {
		t.Fatal("private content retained")
	}
	_, replay, err := env.uc.RecordDefinitionReview(ctx, in)
	if err != nil || !event.CreatedAt.Equal(replay.CreatedAt) {
		t.Fatalf("replay %+v %v", replay, err)
	}
	bad := in
	bad.OperationID = "stale"
	bad.Revision = "012345abcdef"
	if _, _, err := env.uc.RecordDefinitionReview(ctx, bad); err == nil {
		t.Fatal("stale revision accepted")
	}
	bad = in
	bad.OperationID = "unknown"
	bad.Payload.Questions = []domain.HCDReviewQuestion{{Text: "Scope?", Status: "omitted", Blocking: true, ClarificationID: "Q999"}}
	if _, _, err := env.uc.RecordDefinitionReview(ctx, bad); err == nil {
		t.Fatal("unknown clarification accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("recording mutated the document")
	}
	_, qid, err := env.uc.Clarify(ctx, usecases.HCDClarifyInput{ID: v.Unit.ID, Ask: "Scope?"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ = env.uc.Show(ctx, v.Unit.ID)
	in.Revision = domain.HCDReviewRevision(v.DefinitionRevision)
	in.OperationID = "exam-2"
	in.Payload.Questions = []domain.HCDReviewQuestion{{Text: "Scope?", Status: "omitted", Blocking: true, ClarificationID: qid}}
	if _, _, err := env.uc.RecordDefinitionReview(ctx, in); err != nil {
		t.Fatal(err)
	}
	v, _ = env.uc.Show(ctx, v.Unit.ID)
	if len(v.State.Doc.OpenClarifications()) != 1 {
		t.Fatal("omission closed a blocking clarification")
	}
	list, err := repo.DefinitionReviews(ctx, v.Unit.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("events %d %v", len(list), err)
	}
}
