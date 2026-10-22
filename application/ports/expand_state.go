package ports

import (
	"context"

	"mem/domain"
)

// ObservationQueueRepository es la cola de observaciones de instincts
// (feature 040, research R16/R18). Es transitoria: cada observación se
// borra al plegarse.
type ObservationQueueRepository interface {
	Append(ctx context.Context, obs domain.Observation) error
	// ReadBatch devuelve como mucho max observaciones, las más antiguas
	// primero. Las líneas ilegibles se ignoran.
	ReadBatch(ctx context.Context, max int) ([]domain.Observation, error)
	// Remove borra de la cola las observaciones con esos IDs.
	Remove(ctx context.Context, ids []string) error
	// Sanitize reescribe Sample con redact y devuelve cuántas cambiaron
	// (mem sanitize, constitución §20).
	Sanitize(ctx context.Context, redact func(string) string) (int, error)
}

// ShieldStateRepository guarda la huella del último escaneo de shield y su
// informe, para escanear una sola vez por huella (contracts/shield.md, S4).
type ShieldStateRepository interface {
	LastHash(ctx context.Context) string
	Save(ctx context.Context, report domain.ShieldReport) error
	Last(ctx context.Context) (domain.ShieldReport, bool)
}
