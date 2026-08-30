package ports

import "mem/domain"

// FinalizeTransition es la escritura del veredicto terminal, con la comparación que
// impide pisar una corrección concurrente.
//
// El caso de uso deriva el veredicto de una lectura hecha FUERA de toda transacción.
// Entre esa lectura y la escritura cabe una ronda de corrección entera, y la
// finalización llegaba después con un UPDATE ciego de todas las columnas: devolvía
// `round` y `current_target_digest` a los valores obsoletos que había leído y encima
// cerraba la revisión. Los campos Expected* son el estado sobre el que se derivó el
// veredicto; si al abrir la transacción no coincide, la finalización se rechaza y hay
// que rederivarla sobre lo que hay ahora.
type FinalizeTransition struct {
	ExpectedStatus domain.ReviewStatus
	ExpectedRound  int
	// ExpectedDigest es el target vigente que vio el caso de uso. Un veredicto se
	// emite SOBRE un target concreto; si cambió, ya no es el mismo juicio.
	ExpectedDigest string
	Verdict        domain.Verdict
	NextStatus     domain.ReviewStatus
}

type ReviewRepository interface {
	CreateReview(review *domain.Review) error
	GetReview(project, reviewID string) (*domain.Review, error)
	UpdateReview(review *domain.Review) error
	ListReviews(project string, limit int) ([]domain.Review, error)
	// FinalizeReviewAtomically escribe el veredicto terminal comparando antes el
	// estado sobre el que se derivó. Devuelve error si otra operación se adelantó.
	//
	// Escribe SOLO verdict, status y updated_at. Que no pueda tocar round ni
	// current_target_digest no es una omisión: es lo que hace imposible —y no solo
	// improbable— que una finalización tardía restaure el target de una ronda
	// anterior.
	FinalizeReviewAtomically(project, reviewID string, transition FinalizeTransition) error

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
