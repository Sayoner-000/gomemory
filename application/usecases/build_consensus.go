package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

type ConsensusMatch struct {
	Status     domain.ConsensusStatus
	FindingIDA int64
	FindingIDB int64
	Severity   domain.Severity
	Claim      string
}

type ConsensusUnmatched struct {
	Status    domain.ConsensusStatus
	FindingID int64
}

type BuildConsensusInput struct {
	Project   string
	ReviewID  string
	Matches   []ConsensusMatch
	Unmatched []ConsensusUnmatched
}

type findingSource struct {
	finding  domain.Finding
	reviewer domain.Reviewer
}

func BuildConsensus(
	reviews ports.ReviewRepository,
	ledger ports.ConsensusRepository,
	input BuildConsensusInput,
) ([]domain.ConsensusFinding, error) {
	review, err := reviews.GetReview(input.Project, input.ReviewID)
	if err != nil {
		return nil, err
	}
	if review == nil {
		return nil, fmt.Errorf("review %s not found", input.ReviewID)
	}
	results, err := reviews.ListReviewerResults(input.Project, input.ReviewID, review.Round)
	if err != nil {
		return nil, err
	}
	sources := make(map[int64]findingSource)
	for _, result := range results {
		for _, finding := range result.Findings {
			sources[finding.ID] = findingSource{finding: finding, reviewer: result.Reviewer}
		}
	}
	var out []domain.ConsensusFinding
	for _, match := range input.Matches {
		if match.Status != domain.ConsensusConfirmed && match.Status != domain.ConsensusContradiction {
			return nil, fmt.Errorf("paired finding must be CONFIRMED or CONTRADICTION")
		}
		a, okA := sources[match.FindingIDA]
		b, okB := sources[match.FindingIDB]
		if !okA || !okB {
			return nil, fmt.Errorf("source finding does not belong to the active review round")
		}
		if a.reviewer == b.reviewer {
			return nil, fmt.Errorf("consensus sources must come from independent reviewers")
		}
		if match.Status == domain.ConsensusConfirmed && (!a.finding.Confirmable() || !b.finding.Confirmable()) {
			return nil, fmt.Errorf("confirmed finding requires concrete evidence from both reviewers")
		}
		out = append(out, domain.ConsensusFinding{
			ReviewID: input.ReviewID, Round: review.Round, Status: match.Status,
			Severity: match.Severity, Claim: match.Claim,
			SourceFindingIDs: []int64{match.FindingIDA, match.FindingIDB},
		})
	}
	for _, unmatched := range input.Unmatched {
		if unmatched.Status != domain.ConsensusSuspect && unmatched.Status != domain.ConsensusInfo {
			return nil, fmt.Errorf("unmatched finding must be SUSPECT or INFO")
		}
		source, ok := sources[unmatched.FindingID]
		if !ok {
			return nil, fmt.Errorf("source finding does not belong to the active review round")
		}
		out = append(out, domain.ConsensusFinding{
			ReviewID: input.ReviewID, Round: review.Round, Status: unmatched.Status,
			Severity: source.finding.Severity, Claim: source.finding.Claim,
			SourceFindingIDs: []int64{unmatched.FindingID},
		})
	}
	for i := range out {
		out[i].ConsensusLocalID = fmt.Sprintf("C-%03d", i+1)
		if err := ledger.UpsertConsensusFinding(input.Project, input.ReviewID, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
