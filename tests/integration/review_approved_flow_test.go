package main

import (
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

func TestReviewApprovedFlow(t *testing.T) {
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviews := persistence.NewReviewRepository(db)
	consensus := persistence.NewConsensusRepository(db)

	review, err := usecases.StartReview(reviews, usecases.StartReviewInput{
		Project: "integration", TargetType: domain.TargetDiff, Revision: "working-tree",
		Digest: "sha256:frozen", Scope: []string{"domain/"},
		ReviewerA: usecases.ReviewerIdentity{Provider: "one", Model: "a"},
		ReviewerB: usecases.ReviewerIdentity{Provider: "two", Model: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{{
		LocalID: "A-001", Severity: domain.SeverityLow, Claim: "detalle menor",
		EvidenceClass: domain.EvidenceStaticAnalysis, Evidence: []string{"ruta alcanzable"},
	}}}
	b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{{
		LocalID: "B-001", Severity: domain.SeverityLow, Claim: "mismo detalle",
		EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"misma ruta"},
	}}}
	for _, result := range []*domain.ReviewerResult{&a, &b} {
		if _, err := usecases.SubmitReviewerResult(reviews, usecases.SubmitReviewerResultInput{
			Project: "integration", ReviewID: review.ID, TargetDigest: "sha256:frozen", Result: *result,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := usecases.BuildConsensus(reviews, consensus, usecases.BuildConsensusInput{
		Project: "integration", ReviewID: review.ID,
		Matches: []usecases.ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: b.Findings[0].ID,
			Severity: domain.SeverityLow, Claim: "detalle menor",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	finalized, err := usecases.FinalizeReview(reviews, consensus, "integration", review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finalized.Verdict != domain.VerdictApproved {
		t.Fatalf("verdict=%s, want APPROVED", finalized.Verdict)
	}
}
