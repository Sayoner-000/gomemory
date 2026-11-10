package main

import (
	"context"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestHCD057RecordSocraticExam(t *testing.T) {
	c, _ := containerCloseFixture(t, "cli")
	ctx := context.Background()
	v, err := c.HCD.Start(ctx, usecases.HCDStartInput{Kind: "evolution", Lane: "full", Title: "Examine", Why: "contract", Intent: "Review assumptions", Client: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	_, event, err := c.HCD.RecordDefinitionReview(ctx, usecases.HCDDefinitionReviewInput{ID: v.Unit.ID, Revision: domain.HCDReviewRevision(v.DefinitionRevision), OperationID: "wired-exam", Kind: "socratic", Action: "complete", Client: "codex", Payload: domain.HCDDefinitionReviewPayload{Summary: "The request already establishes scope and constraints."}})
	if err != nil {
		t.Fatal(err)
	}
	if event.OperationID != "wired-exam" {
		t.Fatalf("missing event: %+v", event)
	}
	var count int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM hcd_definition_reviews WHERE project = ? AND unit_id = ?", c.Project, v.Unit.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("event not persisted: %d %v", count, err)
	}
}
