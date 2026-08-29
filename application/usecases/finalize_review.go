package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// ReviewMetrics resume una revisión para poder analizar el protocolo (FR-042).
//
// Sin estos números no hay forma de responder si la revisión adversarial vale
// lo que cuesta: cuántos hallazgos confirma frente a los que descarta, cuántas
// rondas gasta, con qué frecuencia acaba escalando. Se derivan de lo persistido
// en el momento de finalizar, no se acumulan por el camino, así que no pueden
// desincronizarse del ledger.
type ReviewMetrics struct {
	Verdict           domain.Verdict
	FindingsTotal     int
	FindingsConfirmed int
	FindingsSuspect   int
	Contradictions    int
	FixRounds         int
	Rounds            int
}

// FinalizeReview deriva el estado terminal. Conserva la firma original porque
// la mayoría de llamadores no necesitan las métricas.
func FinalizeReview(
	reviews ports.ReviewRepository,
	consensus ports.ConsensusRepository,
	project, reviewID string,
) (*domain.Review, error) {
	review, _, err := FinalizeReviewWithMetrics(reviews, consensus, project, reviewID)
	return review, err
}

// FinalizeReviewWithMetrics finaliza y además devuelve el resumen del protocolo.
func FinalizeReviewWithMetrics(
	reviews ports.ReviewRepository,
	consensus ports.ConsensusRepository,
	project, reviewID string,
) (*domain.Review, ReviewMetrics, error) {
	review, err := reviews.GetReview(project, reviewID)
	if err != nil {
		return nil, ReviewMetrics{}, err
	}
	if review == nil {
		return nil, ReviewMetrics{}, fmt.Errorf("review %s not found", reviewID)
	}
	results, err := reviews.ListReviewerResults(project, reviewID, review.Round)
	if err != nil {
		return nil, ReviewMetrics{}, err
	}
	// TODAS las rondas, no la actual. Un hallazgo nace en la ronda del consenso
	// y su resolución llega en una posterior: filtrar por `review.Round` hacía
	// que tras la primera corrección no se viera ninguno y la revisión se
	// aprobara con su defecto severo intacto — sin error, sin aviso y con el
	// ledger diciendo APPROVED.
	findings, err := consensus.ListAllConsensusFindings(project, reviewID)
	if err != nil {
		return nil, ReviewMetrics{}, err
	}
	fixes, err := consensus.ListFixDeltas(project, reviewID)
	if err != nil {
		return nil, ReviewMetrics{}, err
	}
	verdict := domain.DeriveVerdict(*review, results, findings, fixes)
	if !verdict.Valid() {
		return nil, ReviewMetrics{}, fmt.Errorf("review is not ready to finalize")
	}
	review.Verdict = verdict
	switch verdict {
	case domain.VerdictApproved:
		review.Status = domain.ReviewApproved
	case domain.VerdictEscalated:
		review.Status = domain.ReviewEscalated
	case domain.VerdictIncomplete:
		review.Status = domain.ReviewIncomplete
	}
	if err := reviews.UpdateReview(review); err != nil {
		return nil, ReviewMetrics{}, err
	}

	metrics := ReviewMetrics{
		Verdict:       verdict,
		FindingsTotal: len(findings),
		FixRounds:     len(fixes),
		Rounds:        review.Round,
	}
	for _, finding := range findings {
		switch finding.Status {
		case domain.ConsensusConfirmed:
			metrics.FindingsConfirmed++
		case domain.ConsensusSuspect:
			metrics.FindingsSuspect++
		case domain.ConsensusContradiction:
			metrics.Contradictions++
		}
	}
	return review, metrics, nil
}
