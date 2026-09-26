package ports

import (
	"context"

	"mem/domain"
)

// UpdateCheckRepository guarda la caché de versiones (feature 034, R4).
type UpdateCheckRepository interface {
	// Read devuelve la caché; ok=false si no existe o no se puede leer.
	Read(ctx context.Context) (domain.UpdateCheck, bool)
	Write(ctx context.Context, c domain.UpdateCheck) error
}
