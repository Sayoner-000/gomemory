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

	// CountPromotedMemories devuelve cuántas memorias promovió esta revisión y
	// cuántas de esas promociones reforzaron una memoria existente en vez de crear
	// una nueva. Alimenta memory_promoted y memory_deduplicated del contrato de
	// métricas, derivándolas del ledger en vez de acumular contadores por el
	// camino, que es lo que las dejaría desincronizadas (FR-024).
	CountPromotedMemories(project, reviewID string) (promovidas, deduplicadas int, err error)
}
