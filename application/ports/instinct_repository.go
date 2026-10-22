package ports

import (
	"context"
	"mem/domain"
	"time"
)

type InstinctRepository interface {
	Fold(ctx context.Context, batch []domain.Observation, now time.Time) error
	PurgeFolded(ctx context.Context, before time.Time) error
	List(ctx context.Context, all bool) ([]domain.Instinct, error)
	SetStatus(ctx context.Context, slug, status string) error
	// Adopt crea la fila local de una propuesta que solo existe en el
	// registro global, ya con su estado revisado (B-001).
	Adopt(ctx context.Context, instinct domain.Instinct, status string, now time.Time) error
	ForContext(ctx context.Context, minConfidence float64) ([]domain.Instinct, error)
}
type InstinctGlobalRepository interface {
	SanitizeCandidates(ctx context.Context, project string) ([]domain.SanitizeCandidate, error)
	RecordProject(ctx context.Context, project string, instinct domain.Instinct, now time.Time) error
	GlobalFor(ctx context.Context, minConfidence float64) ([]domain.Instinct, error)
	Sanitize(ctx context.Context, redact func(string) string) (int, error)
}
