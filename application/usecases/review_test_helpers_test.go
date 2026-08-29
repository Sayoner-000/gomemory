package usecases

import (
	"fmt"

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
	findings map[string][]domain.ConsensusFinding
	fixes    map[string][]domain.FixDelta
}

func newMemoryConsensusRepository() *memoryConsensusRepository {
	return &memoryConsensusRepository{
		findings: make(map[string][]domain.ConsensusFinding),
		fixes:    make(map[string][]domain.FixDelta),
	}
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
