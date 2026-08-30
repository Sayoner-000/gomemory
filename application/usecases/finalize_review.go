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
	// La marca de re-juicios se lee ANTES que los datos de los que se deriva el
	// veredicto, no después. El orden es la mitad de la garantía: leída después,
	// una retractación que aterrice entre la lectura de los hallazgos y la de la
	// marca queda DENTRO de la marca, el veredicto se deriva de los hallazgos ya
	// obsoletos y la comparación da igual porque compara lo nuevo con lo nuevo.
	// Leída antes, cualquier cambio posterior la desplaza y el cierre se rechaza.
	marcaResultados, err := reviews.ReviewerResultsMark(project, reviewID, review.Round)
	if err != nil {
		return nil, ReviewMetrics{}, err
	}
	marca, err := reviews.RejudgmentMark(project, reviewID)
	if err != nil {
		return nil, ReviewMetrics{}, err
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
	terminal := map[domain.Verdict]domain.ReviewStatus{
		domain.VerdictApproved:   domain.ReviewApproved,
		domain.VerdictEscalated:  domain.ReviewEscalated,
		domain.VerdictIncomplete: domain.ReviewIncomplete,
	}[verdict]
	// El estado desde el que se finaliza se guarda ANTES de moverlo: es la mitad de
	// la comparación que la escritura hará dentro de su transacción.
	anterior := review.Status
	review.Verdict = verdict
	// Por TransitionTo y no por asignación directa: es el único punto que impide
	// reabrir o mover una revisión ya cerrada (FR-015, FR-016).
	if err := review.TransitionTo(terminal); err != nil {
		return nil, ReviewMetrics{}, err
	}
	review.UpdatedAt = time.Now()
	// Comparación-y-cambio, no UpdateReview. Todo lo anterior —la revisión, los
	// resultados, los hallazgos, las correcciones— se leyó fuera de cualquier
	// transacción, y entre esas lecturas y esta escritura cabe una ronda de
	// corrección entera. UpdateReview reescribía todas las columnas desde el objeto
	// obsoleto: restauraba la ronda y el target de antes y encima cerraba la
	// revisión. Si algo se movió, el veredicto ya no corresponde a lo que hay y hay
	// que rederivarlo.
	if err := reviews.SetReviewStatusAtomically(project, reviewID, ports.StatusTransition{
		ExpectedStatus:              anterior,
		ExpectedRound:               review.Round,
		ExpectedDigest:              review.ActiveTargetDigest(),
		ExpectedRejudgmentMark:      marca,
		ExpectedReviewerResultsMark: marcaResultados,
		Verdict:                     verdict,
		NextStatus:                  terminal,
	}); err != nil {
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
