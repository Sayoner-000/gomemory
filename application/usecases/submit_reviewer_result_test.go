package usecases

import (
	"strings"
	"testing"

	"mem/domain"
)

func TestSubmitReviewerResultValidatesDigestIdempotencyAndFailure(t *testing.T) {
	repo := newMemoryReviewRepository()
	target, err := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", nil)
	if err != nil {
		t.Fatal(err)
	}
	review := &domain.Review{
		ID:      "acr_submit",
		Project: "proj",
		Target:  target,
		Round:   0,
		Status:  domain.ReviewAwaitingReviewers,
	}
	if err := repo.CreateReview(review); err != nil {
		t.Fatal(err)
	}

	result := domain.ReviewerResult{
		Reviewer: domain.ReviewerA,
		Status:   domain.ReviewerResultSuccess,
		Findings: []domain.Finding{{
			LocalID:       "A-001",
			Severity:      domain.SeverityHigh,
			Claim:         "defecto",
			EvidenceClass: domain.EvidenceDeterministic,
			Evidence:      []string{"evidencia"},
		}},
	}
	_, err = SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project: "proj", ReviewID: review.ID, TargetDigest: "sha256:changed", Result: result,
	})
	if err == nil || !strings.Contains(err.Error(), "target changed") {
		t.Fatalf("digest distinto debe devolver target changed, got %v", err)
	}
	if len(repo.results[reviewKey("proj", review.ID)]) != 0 {
		t.Fatal("se persistió un resultado contra el target equivocado")
	}

	input := SubmitReviewerResultInput{
		Project: "proj", ReviewID: review.ID, TargetDigest: "sha256:frozen", Result: result,
	}
	first, err := SubmitReviewerResult(repo, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.FindingIDs["A-001"] == 0 {
		t.Fatal("el envío debe devolver los IDs persistidos para construir el consenso")
	}
	input.Result.Findings[0].Claim = "defecto actualizado"
	if _, err := SubmitReviewerResult(repo, input); err != nil {
		t.Fatal(err)
	}
	storedResults, _ := repo.ListReviewerResults("proj", review.ID, 0)
	if len(storedResults) != 1 || len(storedResults[0].Findings) != 1 {
		t.Fatalf("reenvío no idempotente: %#v", storedResults)
	}

	if _, err := SubmitReviewerResult(repo, SubmitReviewerResultInput{
		Project:      "proj",
		ReviewID:     review.ID,
		TargetDigest: "sha256:frozen",
		Result: domain.ReviewerResult{
			Reviewer: domain.ReviewerB,
			Status:   domain.ReviewerResultFailure,
		},
	}); err != nil {
		t.Fatal(err)
	}
	storedReview, _ := repo.GetReview("proj", review.ID)
	if storedReview.Status != domain.ReviewIncomplete {
		t.Fatalf("un fallo de revisor debe bloquear APPROVED: status=%s", storedReview.Status)
	}
}
