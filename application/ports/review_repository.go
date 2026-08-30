package ports

import "mem/domain"

// StatusTransition es una escritura del estado de una revisión bajo comparación-y-cambio.
//
// Sirve tanto para el cierre —que además fija el veredicto— como para los avances de
// estado durante el envío de resultados. Los dos tienen el mismo problema y la misma
// solución, y darles nombres distintos solo escondía que uno de ellos seguía usando
// la escritura ciega.
//
// El caso de uso deriva el veredicto de una lectura hecha FUERA de toda transacción.
// Entre esa lectura y la escritura cabe una ronda de corrección entera, y la
// finalización llegaba después con un UPDATE ciego de todas las columnas: devolvía
// `round` y `current_target_digest` a los valores obsoletos que había leído y encima
// cerraba la revisión. Los campos Expected* son el estado sobre el que se derivó el
// veredicto; si al abrir la transacción no coincide, la finalización se rechaza y hay
// que rederivarla sobre lo que hay ahora.
type StatusTransition struct {
	ExpectedStatus domain.ReviewStatus
	ExpectedRound  int
	// ExpectedDigest es el target vigente que vio el caso de uso. Un veredicto se
	// emite SOBRE un target concreto; si cambió, ya no es el mismo juicio.
	ExpectedDigest string
	// ExpectedRejudgmentMark identifica el conjunto de re-juicios sobre el que se
	// dedujo el veredicto. Vacío significa "no comparar", para los avances de estado
	// que no dependen de ellos.
	//
	// Comparar estado, ronda y digest NO basta: un re-juicio no toca ninguna de esas
	// tres columnas. Un revisor que se retracta —cambia su RESOLVED por REGRESSED en
	// la ronda vigente— entre la lectura y la escritura dejaba pasar las tres guardas
	// y persistía APPROVED sobre un hallazgo severo que el ledger registra como
	// reaparecido. Reproducido en 7 de cada 300 ejecuciones.
	ExpectedRejudgmentMark string
	// ExpectedReviewerResultsMark identifica los resultados y hallazgos de revisor
	// usados para derivar el veredicto. Un resultado tardío no cambia status, round
	// ni digest, así que necesita su propia guarda.
	ExpectedReviewerResultsMark string
	// Verdict vacío deja el veredicto sin fijar: un avance de estado intermedio no
	// emite juicio.
	Verdict    domain.Verdict
	NextStatus domain.ReviewStatus
}

type ReviewRepository interface {
	CreateReview(review *domain.Review) error
	GetReview(project, reviewID string) (*domain.Review, error)
	UpdateReview(review *domain.Review) error
	ListReviews(project string, limit int) ([]domain.Review, error)
	// SetReviewStatusAtomically escribe el estado (y el veredicto, si lo hay)
	// comparando antes aquello sobre lo que se decidió. Devuelve error si otra
	// operación se adelantó.
	//
	// Escribe SOLO status, verdict y updated_at. Que no pueda tocar round ni
	// current_target_digest no es una omisión: es lo que hace imposible —y no solo
	// improbable— que una escritura tardía restaure el target de una ronda anterior.
	SetReviewStatusAtomically(project, reviewID string, transition StatusTransition) error

	UpsertReviewerResult(project, reviewID string, result *domain.ReviewerResult) error
	// UpsertReviewerResultAtomically persiste un resultado solo si la revisión sigue
	// exactamente en el estado y ronda observados por el caso de uso. La comprobación
	// y la escritura comparten el mismo bloqueo de escritura.
	UpsertReviewerResultAtomically(
		project, reviewID string,
		expectedStatus domain.ReviewStatus,
		expectedRound int,
		result *domain.ReviewerResult,
	) error
	ListReviewerResults(project, reviewID string, round int) ([]domain.ReviewerResult, error)
	GetFinding(project, reviewID string, findingID int64) (*domain.Finding, error)
	ListFindings(project, reviewID string, round int) ([]domain.Finding, error)

	// RejudgmentMark resume el conjunto de re-juicios de una revisión en un valor
	// comparable. Cambia con cualquier alta o modificación de un re-juicio, que es lo
	// que permite detectar que el veredicto se dedujo de un estado ya superado.
	RejudgmentMark(project, reviewID string) (string, error)
	// ReviewerResultsMark resume los resultados y hallazgos de la ronda indicada.
	// Cambia con cualquier escritura que pueda alterar el veredicto.
	ReviewerResultsMark(project, reviewID string, round int) (string, error)

	// CountPromotedMemories devuelve cuántas memorias promovió esta revisión y
	// cuántas de esas promociones reforzaron una memoria existente en vez de crear
	// una nueva. Alimenta memory_promoted y memory_deduplicated del contrato de
	// métricas, derivándolas del ledger en vez de acumular contadores por el
	// camino, que es lo que las dejaría desincronizadas (FR-024).
	CountPromotedMemories(project, reviewID string) (promovidas, deduplicadas int, err error)
}
