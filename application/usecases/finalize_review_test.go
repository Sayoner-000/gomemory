package usecases

import (
	"testing"

	"mem/application/ports"
	"mem/domain"
)

func TestFinalizeReviewDerivesVerdictFromPersistedState(t *testing.T) {
	var finalizeSignature func(ports.ReviewRepository, ports.ConsensusRepository, string, string) (*domain.Review, error)
	finalizeSignature = FinalizeReview
	_ = finalizeSignature

	reviews := newMemoryReviewRepository()
	consensus := newMemoryConsensusRepository()
	target, _ := domain.NewTarget(domain.TargetDiff, "working-tree", "sha256:frozen", nil)
	review := &domain.Review{
		ID: "acr_finalize", Project: "proj", Target: target, MaxFixRounds: 2, Status: domain.ReviewConsensusReady,
	}
	if err := reviews.CreateReview(review); err != nil {
		t.Fatal(err)
	}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &domain.ReviewerResult{
		Reviewer: domain.ReviewerA, Status: domain.ReviewerResultSuccess,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reviews.UpsertReviewerResult("proj", review.ID, &domain.ReviewerResult{
		Reviewer: domain.ReviewerB, Status: domain.ReviewerResultSuccess,
	}); err != nil {
		t.Fatal(err)
	}

	finalized, err := FinalizeReview(reviews, consensus, "proj", review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finalized.Verdict != domain.VerdictApproved || finalized.Status != domain.ReviewApproved {
		t.Fatalf("finalización no derivada: status=%s verdict=%s", finalized.Status, finalized.Verdict)
	}
	stored, _ := reviews.GetReview("proj", review.ID)
	if stored.Verdict != domain.VerdictApproved {
		t.Fatalf("veredicto no persistido: %s", stored.Verdict)
	}
}

// TestFinalizeReview_EmiteMetricasDelProtocolo cubre FR-042.
//
// Las métricas no son adorno: sin ellas no se puede responder si el protocolo
// sirve —cuántos hallazgos confirma frente a los que descarta, cuántas rondas
// gasta, con qué frecuencia escala— y una revisión adversarial que no se puede
// evaluar es un ritual caro.
func TestFinalizeReview_EmiteMetricasDelProtocolo(t *testing.T) {
	reviews, ledger := escenarioReRevisable(t)
	for _, reviewer := range []domain.Reviewer{domain.ReviewerA, domain.ReviewerB} {
		ronda, _ := reviews.GetReview("proj", "acr_test")
		r := domain.ReviewerResult{Reviewer: reviewer, Round: ronda.Round, Status: domain.ReviewerResultSuccess}
		if err := reviews.UpsertReviewerResult("proj", "acr_test", &r); err != nil {
			t.Fatalf("UpsertReviewerResult: %v", err)
		}
	}
	reRevisionUnanime(t, reviews, ledger,
		map[string]domain.ReJudgmentState{"C-001": domain.ReJudgmentResolved})

	_, metricas, err := FinalizeReviewWithMetrics(reviews, ledger, "proj", "acr_test")
	if err != nil {
		t.Fatalf("FinalizeReviewWithMetrics: %v", err)
	}
	if metricas.Verdict != domain.VerdictApproved {
		t.Errorf("Verdict = %s", metricas.Verdict)
	}
	if metricas.FindingsConfirmed != 1 {
		t.Errorf("FindingsConfirmed = %d, se esperaba 1", metricas.FindingsConfirmed)
	}
	if metricas.FindingsSuspect != 1 {
		t.Errorf("FindingsSuspect = %d, se esperaba 1", metricas.FindingsSuspect)
	}
	if metricas.FixRounds != 1 {
		t.Errorf("FixRounds = %d, se esperaba 1", metricas.FixRounds)
	}
	if metricas.Contradictions != 0 {
		t.Errorf("Contradictions = %d, se esperaba 0", metricas.Contradictions)
	}
}
