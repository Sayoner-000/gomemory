package usecases

import (
	"fmt"
	"time"

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
	Verdict domain.Verdict
	// Duration son los segundos transcurridos entre la apertura de la revisión y
	// su finalización. El contrato publicado lo exige y el struct no lo tenía.
	Duration          int
	FindingsTotal     int
	FindingsConfirmed int
	FindingsSuspect   int
	Contradictions    int
	FixRounds         int
	Rounds            int
	// MemoryPromoted y MemoryDeduplicated cierran el contrato de métricas. Se
	// derivan del ledger, no se acumulan por el camino, así que no pueden
	// desincronizarse de él.
	MemoryPromoted     int
	MemoryDeduplicated int
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
	terminal := map[domain.Verdict]domain.ReviewStatus{
		domain.VerdictApproved:   domain.ReviewApproved,
		domain.VerdictEscalated:  domain.ReviewEscalated,
		domain.VerdictIncomplete: domain.ReviewIncomplete,
	}[verdict]
	// Por TransitionTo y no por asignación directa: es el único punto que impide
	// reabrir o mover una revisión ya cerrada (FR-015, FR-016).
	if err := review.TransitionTo(terminal); err != nil {
		return nil, ReviewMetrics{}, err
	}
	review.UpdatedAt = time.Now()
	if err := reviews.UpdateReview(review); err != nil {
		return nil, ReviewMetrics{}, err
	}

	promovidas, deduplicadas, err := reviews.CountPromotedMemories(project, reviewID)
	if err != nil {
		return nil, ReviewMetrics{}, err
	}
	metrics := ReviewMetrics{
		Verdict:            verdict,
		Duration:           duracionEnSegundos(review.CreatedAt, review.UpdatedAt),
		FindingsTotal:      len(findings),
		FixRounds:          len(fixes),
		Rounds:             review.Round,
		MemoryPromoted:     promovidas,
		MemoryDeduplicated: deduplicadas,
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

// duracionEnSegundos mide lo que tardó el protocolo. Nunca devuelve negativo: un
// reloj que retrocede no debe convertir una métrica en un dato absurdo.
func duracionEnSegundos(inicio, fin time.Time) int {
	if inicio.IsZero() || fin.IsZero() || fin.Before(inicio) {
		return 0
	}
	return int(fin.Sub(inicio).Seconds())
}
