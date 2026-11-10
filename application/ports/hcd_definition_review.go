package ports

import (
	"context"
	"mem/domain"
)

// HCDDefinitionReviewRepository serializes changes for one definition revision.
// expectedLast is the last operation read by the use case (empty for no events).
// Replaying an identical operation succeeds even when expectedLast is now stale.
type HCDDefinitionReviewRepository interface {
	DefinitionReviews(ctx context.Context, unitID string) ([]domain.HCDDefinitionReviewEvent, error)
	AppendReview(ctx context.Context, event domain.HCDDefinitionReviewEvent, expectedLast domain.HCDReviewOperationID) (domain.HCDDefinitionReviewEvent, error)
	AddReviewedApproval(ctx context.Context, approval domain.Approval, expectedLast domain.HCDReviewOperationID) error
}
