package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

type memoryReviewRepository struct {
	reviews map[string]*domain.Review
	results map[string][]domain.ReviewerResult
}

func newMemoryReviewRepository() *memoryReviewRepository {
	return &memoryReviewRepository{
		reviews: make(map[string]*domain.Review),
		results: make(map[string][]domain.ReviewerResult),
	}
}

func reviewKey(project, reviewID string) string { return project + ":" + reviewID }

func (r *memoryReviewRepository) CreateReview(review *domain.Review) error {
	key := reviewKey(review.Project, review.ID)
	if _, exists := r.reviews[key]; exists {
		return fmt.Errorf("duplicate review")
	}
	copy := *review
	r.reviews[key] = &copy
	return nil
}

func (r *memoryReviewRepository) GetReview(project, reviewID string) (*domain.Review, error) {
	review := r.reviews[reviewKey(project, reviewID)]
	if review == nil {
		return nil, nil
	}
	copy := *review
	return &copy, nil
}

func (r *memoryReviewRepository) UpdateReview(review *domain.Review) error {
	copy := *review
	r.reviews[reviewKey(review.Project, review.ID)] = &copy
	return nil
}

func (r *memoryReviewRepository) ListReviews(project string, limit int) ([]domain.Review, error) {
	var out []domain.Review
	for _, review := range r.reviews {
		if review.Project == project {
			out = append(out, *review)
		}
	}
	return out, nil
}

func (r *memoryReviewRepository) UpsertReviewerResult(project, reviewID string, result *domain.ReviewerResult) error {
	key := reviewKey(project, reviewID)
	items := r.results[key]
	for i := range items {
		if items[i].Reviewer == result.Reviewer && items[i].Round == result.Round {
			for j := range result.Findings {
				if result.Findings[j].ID == 0 {
					result.Findings[j].ID = items[i].Findings[0].ID
				}
			}
			items[i] = *result
			r.results[key] = items
			return nil
		}
	}
	result.ID = int64(len(items) + 1)
	for i := range result.Findings {
		result.Findings[i].ID = int64(len(items)*100 + i + 1)
		result.Findings[i].ReviewerResultID = result.ID
	}
	r.results[key] = append(items, *result)
	return nil
}

func (r *memoryReviewRepository) ListReviewerResults(project, reviewID string, round int) ([]domain.ReviewerResult, error) {
	var out []domain.ReviewerResult
	for _, result := range r.results[reviewKey(project, reviewID)] {
		if result.Round == round {
			out = append(out, result)
		}
	}
	return out, nil
}

func (r *memoryReviewRepository) GetFinding(project, reviewID string, findingID int64) (*domain.Finding, error) {
	for _, result := range r.results[reviewKey(project, reviewID)] {
		for _, finding := range result.Findings {
			if finding.ID == findingID {
				copy := finding
				return &copy, nil
			}
		}
	}
	return nil, nil
}

func (r *memoryReviewRepository) ListFindings(project, reviewID string, round int) ([]domain.Finding, error) {
	var out []domain.Finding
	for _, result := range r.results[reviewKey(project, reviewID)] {
		if result.Round == round {
			out = append(out, result.Findings...)
		}
	}
	return out, nil
}

type memoryConsensusRepository struct {
	findings   map[string][]domain.ConsensusFinding
	fixes      map[string][]domain.FixDelta
	rejudgment map[string][]domain.ReJudgment
	reviews    *memoryReviewRepository
}

func newMemoryConsensusRepository() *memoryConsensusRepository {
	return &memoryConsensusRepository{
		findings:   make(map[string][]domain.ConsensusFinding),
		fixes:      make(map[string][]domain.FixDelta),
		rejudgment: make(map[string][]domain.ReJudgment),
	}
}

// enlazar deja al ledger en memoria hablar con el repositorio de revisiones, que es
// lo que la transición atómica de corrección necesita para avanzar la revisión.
func (r *memoryConsensusRepository) enlazar(reviews *memoryReviewRepository) *memoryConsensusRepository {
	r.reviews = reviews
	return r
}

func (r *memoryConsensusRepository) UpsertConsensusFinding(project, reviewID string, finding *domain.ConsensusFinding) error {
	key := reviewKey(project, reviewID)
	for i := range r.findings[key] {
		if r.findings[key][i].ConsensusLocalID == finding.ConsensusLocalID {
			r.findings[key][i] = *finding
			return nil
		}
	}
	finding.ID = int64(len(r.findings[key]) + 1)
	r.findings[key] = append(r.findings[key], *finding)
	return nil
}

func (r *memoryConsensusRepository) GetConsensusFinding(project, reviewID, localID string) (*domain.ConsensusFinding, error) {
	for _, finding := range r.findings[reviewKey(project, reviewID)] {
		if finding.ConsensusLocalID == localID {
			copy := finding
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *memoryConsensusRepository) ListConsensusFindings(project, reviewID string, round int) ([]domain.ConsensusFinding, error) {
	var out []domain.ConsensusFinding
	for _, finding := range r.findings[reviewKey(project, reviewID)] {
		if finding.Round == round {
			out = append(out, finding)
		}
	}
	return out, nil
}

func (r *memoryConsensusRepository) ListAllConsensusFindings(project, reviewID string) ([]domain.ConsensusFinding, error) {
	return append([]domain.ConsensusFinding(nil), r.findings[reviewKey(project, reviewID)]...), nil
}

func (r *memoryConsensusRepository) UpsertFixDelta(project, reviewID string, delta *domain.FixDelta) error {
	key := reviewKey(project, reviewID)
	for i := range r.fixes[key] {
		if r.fixes[key][i].Round == delta.Round {
			r.fixes[key][i] = *delta
			return nil
		}
	}
	r.fixes[key] = append(r.fixes[key], *delta)
	return nil
}

func (r *memoryConsensusRepository) ListFixDeltas(project, reviewID string) ([]domain.FixDelta, error) {
	return append([]domain.FixDelta(nil), r.fixes[reviewKey(project, reviewID)]...), nil
}

func rejudgmentKey(project, reviewID, localID string) string {
	return reviewKey(project, reviewID) + ":" + localID
}

func (r *memoryConsensusRepository) UpsertReJudgment(project, reviewID string, judgment *domain.ReJudgment) error {
	if err := judgment.Validate(); err != nil {
		return err
	}
	finding, err := r.GetConsensusFinding(project, reviewID, judgment.ConsensusLocalID)
	if err != nil {
		return err
	}
	if finding == nil {
		return fmt.Errorf("el hallazgo de consenso %s no existe en esta revisión", judgment.ConsensusLocalID)
	}
	key := rejudgmentKey(project, reviewID, judgment.ConsensusLocalID)
	reemplazado := false
	for i := range r.rejudgment[key] {
		existente := r.rejudgment[key][i]
		if existente.Reviewer == judgment.Reviewer && existente.Round == judgment.Round {
			r.rejudgment[key][i] = *judgment
			reemplazado = true
			break
		}
	}
	if !reemplazado {
		r.rejudgment[key] = append(r.rejudgment[key], *judgment)
	}
	finding.RejudgmentState = domain.AggregateReJudgment(r.rejudgment[key])
	return r.UpsertConsensusFinding(project, reviewID, finding)
}

func (r *memoryConsensusRepository) ListReJudgments(project, reviewID, localID string) ([]domain.ReJudgment, error) {
	return append([]domain.ReJudgment(nil), r.rejudgment[rejudgmentKey(project, reviewID, localID)]...), nil
}

func (r *memoryConsensusRepository) RecordFixAtomically(
	project, reviewID string, transition ports.FixTransition,
) error {
	key := reviewKey(project, reviewID)
	if len(r.fixes[key]) != transition.ExpectedRounds {
		return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
	}
	if r.reviews != nil && transition.ExpectedBaseDigest != "" {
		actual, err := r.reviews.GetReview(project, reviewID)
		if err != nil {
			return err
		}
		if actual != nil && actual.ActiveTargetDigest() != transition.ExpectedBaseDigest {
			return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
		}
	}
	for _, existente := range r.fixes[key] {
		if existente.Round == transition.Delta.Round {
			return fmt.Errorf("la ronda %d ya fue registrada por otra corrección", transition.NextRound)
		}
	}
	r.fixes[key] = append(r.fixes[key], *transition.Delta)
	if r.reviews == nil {
		return nil
	}
	review, err := r.reviews.GetReview(project, reviewID)
	if err != nil {
		return err
	}
	if review == nil {
		return fmt.Errorf("review %s not found", reviewID)
	}
	review.Round = transition.NextRound
	review.Status = transition.NextStatus
	review.CurrentTargetDigest = transition.CurrentTargetDigest
	return r.reviews.UpdateReview(review)
}

// CountPromotedMemories: el repositorio en memoria no guarda memorias promovidas,
// así que informa cero. Los tests que miden esta métrica usan SQLite real.
func (r *memoryReviewRepository) CountPromotedMemories(project, reviewID string) (int, int, error) {
	return 0, 0, nil
}
