package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

type SubmitReviewerResultInput struct {
	Project      string
	ReviewID     string
	TargetDigest string
	Result       domain.ReviewerResult
}

type SubmitReviewerResultOutput struct {
	ConsensusReady bool
	FindingIDs     map[string]int64
}

func SubmitReviewerResult(repo ports.ReviewRepository, input SubmitReviewerResultInput) (SubmitReviewerResultOutput, error) {
	review, err := repo.GetReview(input.Project, input.ReviewID)
	if err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	if review == nil {
		return SubmitReviewerResultOutput{}, fmt.Errorf("review %s not found", input.ReviewID)
	}
	if err := review.Target.ValidateDigest(input.TargetDigest); err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	if !input.Result.Reviewer.Valid() {
		return SubmitReviewerResultOutput{}, fmt.Errorf("reviewer must be A or B")
	}
	if input.Result.Status != domain.ReviewerResultSuccess && input.Result.Status != domain.ReviewerResultFailure {
		return SubmitReviewerResultOutput{}, fmt.Errorf("invalid reviewer result status")
	}
	existing, err := repo.ListReviewerResults(input.Project, input.ReviewID, review.Round)
	if err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	for _, result := range existing {
		if result.Reviewer == input.Result.Reviewer && result.Status == domain.ReviewerResultFailure {
			return SubmitReviewerResultOutput{}, fmt.Errorf("failed reviewer result is final for this round")
		}
	}
	input.Result.Round = review.Round
	input.Result.ReviewID = review.ID
	if err := repo.UpsertReviewerResult(input.Project, input.ReviewID, &input.Result); err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	findingIDs := make(map[string]int64, len(input.Result.Findings))
	for _, finding := range input.Result.Findings {
		findingIDs[finding.LocalID] = finding.ID
	}
	if input.Result.Status == domain.ReviewerResultFailure {
		review.Status = domain.ReviewIncomplete
		review.Verdict = domain.VerdictIncomplete
		if err := repo.UpdateReview(review); err != nil {
			return SubmitReviewerResultOutput{}, err
		}
		return SubmitReviewerResultOutput{FindingIDs: findingIDs}, nil
	}
	results, err := repo.ListReviewerResults(input.Project, input.ReviewID, review.Round)
	if err != nil {
		return SubmitReviewerResultOutput{}, err
	}
	seen := map[domain.Reviewer]bool{}
	for _, result := range results {
		if result.Status == domain.ReviewerResultSuccess {
			seen[result.Reviewer] = true
		}
	}
	ready := seen[domain.ReviewerA] && seen[domain.ReviewerB]
	if ready {
		review.Status = domain.ReviewConsensusReady
		if err := repo.UpdateReview(review); err != nil {
			return SubmitReviewerResultOutput{}, err
		}
	}
	return SubmitReviewerResultOutput{ConsensusReady: ready, FindingIDs: findingIDs}, nil
}
