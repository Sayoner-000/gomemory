package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// ReviewStatus es el linaje completo de una revisión leído en una sola
// operación: qué se detectó, qué corrección abordó cada hallazgo, cómo lo
// re-juzgó cada revisor y qué entregó cada revisor en cada ronda.
//
// Antes lo armaban por separado la herramienta review_status y `mem review
// show`/`status`, y la vigencia de los re-juicios llegó a divergir entre ambas
// hasta corregirse en los dos sitios (hallazgo H-02 de la evaluación 043).
// Ahora las superficies solo deciden cómo presentarlo.
type ReviewStatus struct {
	Review   *domain.Review
	Findings []ReviewFindingStatus
	Fixes    []domain.FixDelta
	// ResultsByRound[r] son los resultados de revisor de la ronda r, de la 0 a
	// la vigente. Se devuelven todas porque cada superficie elige: el informe
	// humano muestra el historial y review_status, la ronda vigente.
	ResultsByRound [][]domain.ReviewerResult
	Counts         ReviewCounts
}

// ReviewFindingStatus es un hallazgo de consenso con su linaje resuelto.
type ReviewFindingStatus struct {
	domain.ConsensusFinding
	// Vigente es el estado agregado con el MISMO criterio que el veredicto;
	// vacío significa pendiente en la ronda vigente.
	Vigente domain.ReJudgmentState
	// ByReviewer son los re-juicios de la ronda vigente, por revisor.
	ByReviewer map[domain.Reviewer]domain.ReJudgmentState
	// AddressedByRound es la ronda de la última corrección que lo abordó;
	// solo es válida si Addressed es true.
	AddressedByRound int
	Addressed        bool
}

// ReviewCounts resume los hallazgos. Pending cuenta los que no tienen estado
// vigente, que nunca se suman a ByRejudgment.
type ReviewCounts struct {
	ByStatus     map[domain.ConsensusStatus]int
	BySeverity   map[domain.Severity]int
	ByRejudgment map[domain.ReJudgmentState]int
	Pending      int
}

// BuildReviewStatus lee el linaje de una revisión ya resuelta. Quien llama
// decide cómo informar de una revisión inexistente; aquí review nunca es nil.
func BuildReviewStatus(
	reviews ports.ReviewRepository,
	consensus ports.ConsensusRepository,
	project string,
	review *domain.Review,
) (*ReviewStatus, error) {
	findings, err := consensus.ListAllConsensusFindings(project, review.ID)
	if err != nil {
		return nil, fmt.Errorf("listar consenso: %w", err)
	}
	fixes, err := consensus.ListFixDeltas(project, review.ID)
	if err != nil {
		return nil, fmt.Errorf("listar correcciones: %w", err)
	}
	results := make([][]domain.ReviewerResult, review.Round+1)
	for round := range results {
		if results[round], err = reviews.ListReviewerResults(project, review.ID, round); err != nil {
			return nil, fmt.Errorf("listar resultados de revisor: %w", err)
		}
	}

	addressedBy := map[string]int{}
	for _, fix := range fixes {
		for _, localID := range fix.AddressedConsensusIDs {
			addressedBy[localID] = fix.Round
		}
	}

	status := &ReviewStatus{
		Review: review, Fixes: fixes, ResultsByRound: results,
		Findings: make([]ReviewFindingStatus, 0, len(findings)),
		Counts: ReviewCounts{
			ByStatus:     map[domain.ConsensusStatus]int{},
			BySeverity:   map[domain.Severity]int{},
			ByRejudgment: map[domain.ReJudgmentState]int{},
		},
	}
	for _, finding := range findings {
		byReviewer, err := ReJudgmentsByReviewer(consensus, project, review.ID, finding.ConsensusLocalID, review.Round)
		if err != nil {
			return nil, fmt.Errorf("listar re-juicios: %w", err)
		}
		vigente := finding.EstadoVigente(review.Round)
		round, addressed := addressedBy[finding.ConsensusLocalID]
		status.Findings = append(status.Findings, ReviewFindingStatus{
			ConsensusFinding: finding, Vigente: vigente, ByReviewer: byReviewer,
			AddressedByRound: round, Addressed: addressed,
		})
		status.Counts.ByStatus[finding.Status]++
		status.Counts.BySeverity[finding.Severity]++
		if vigente == "" {
			status.Counts.Pending++
		} else {
			status.Counts.ByRejudgment[vigente]++
		}
	}
	return status, nil
}

// PromotableReviewFindings devuelve qué hallazgos puede promover a memoria una
// revisión en la ronda dada; la regla es de dominio.
func PromotableReviewFindings(
	consensus ports.ConsensusRepository, project, reviewID string, round int,
) ([]string, error) {
	findings, err := consensus.ListAllConsensusFindings(project, reviewID)
	if err != nil {
		return nil, err
	}
	return domain.PromotableFindings(findings, round), nil
}
