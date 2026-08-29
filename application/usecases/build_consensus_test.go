package usecases

import (
	"testing"

	"mem/domain"
)

func TestBuildConsensusValidatesIndependentSourcesAndEvidence(t *testing.T) {
	reviews := newMemoryReviewRepository()
	ledger := newMemoryConsensusRepository()
	target, _ := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", nil)
	review := &domain.Review{ID: "acr_consensus", Project: "proj", Target: target, Status: domain.ReviewConsensusReady}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	a := domain.ReviewerResult{Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{
		{LocalID: "A-001", Severity: domain.SeverityHigh, Claim: "race", EvidenceClass: domain.EvidenceReproduced, Evidence: []string{"go test -race falla"}},
		{LocalID: "A-002", Severity: domain.SeverityMedium, Claim: "único", EvidenceClass: domain.EvidenceStaticAnalysis, Evidence: []string{"ruta alcanzable"}},
		{LocalID: "A-003", Severity: domain.SeverityHigh, Claim: "sin evidencia", EvidenceClass: domain.EvidenceDeterministic},
	}}
	b := domain.ReviewerResult{Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess, Findings: []domain.Finding{
		{LocalID: "B-001", Severity: domain.SeverityHigh, Claim: "race equivalente", EvidenceClass: domain.EvidenceTestFailure, Evidence: []string{"mismo fallo"}},
		{LocalID: "B-003", Severity: domain.SeverityHigh, Claim: "pareja", EvidenceClass: domain.EvidenceDeterministic, Evidence: []string{"traza"}},
	}}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &a); err != nil {
		t.Fatal(err)
	}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &b); err != nil {
		t.Fatal(err)
	}

	findings, err := BuildConsensus(reviews, ledger, BuildConsensusInput{
		Project: "proj", ReviewID: review.ID,
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: b.Findings[0].ID,
			Severity: domain.SeverityHigh, Claim: "race",
		}},
		Unmatched: []ConsensusUnmatched{{Status: domain.ConsensusSuspect, FindingID: a.Findings[1].ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 || findings[0].Status != domain.ConsensusConfirmed || len(findings[0].SourceFindingIDs) != 2 {
		t.Fatalf("consenso confirmado inválido: %#v", findings)
	}
	if findings[1].Status != domain.ConsensusSuspect || len(findings[1].SourceFindingIDs) != 1 {
		t.Fatalf("hallazgo único no quedó SUSPECT: %#v", findings[1])
	}

	_, err = BuildConsensus(reviews, newMemoryConsensusRepository(), BuildConsensusInput{
		Project: "proj", ReviewID: review.ID,
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[0].ID, FindingIDB: a.Findings[1].ID,
			Severity: domain.SeverityHigh,
		}},
	})
	if err == nil {
		t.Fatal("dos fuentes del mismo revisor no pueden confirmar un hallazgo")
	}

	_, err = BuildConsensus(reviews, newMemoryConsensusRepository(), BuildConsensusInput{
		Project: "proj", ReviewID: review.ID,
		Matches: []ConsensusMatch{{
			Status: domain.ConsensusConfirmed, FindingIDA: a.Findings[2].ID, FindingIDB: b.Findings[1].ID,
			Severity: domain.SeverityHigh,
		}},
	})
	if err == nil {
		t.Fatal("un hallazgo sin evidencia no puede quedar CONFIRMED")
	}
}
