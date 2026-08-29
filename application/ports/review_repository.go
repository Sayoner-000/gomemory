package ports

import "mem/domain"

type ReviewRepository interface {
	CreateReview(review *domain.Review) error
	GetReview(project, reviewID string) (*domain.Review, error)
	UpdateReview(review *domain.Review) error
	ListReviews(project string, limit int) ([]domain.Review, error)

	UpsertReviewerResult(project, reviewID string, result *domain.ReviewerResult) error
	ListReviewerResults(project, reviewID string, round int) ([]domain.ReviewerResult, error)
	GetFinding(project, reviewID string, findingID int64) (*domain.Finding, error)
	ListFindings(project, reviewID string, round int) ([]domain.Finding, error)
}
