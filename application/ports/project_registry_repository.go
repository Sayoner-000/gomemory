package ports

import (
	"context"

	"mem/domain"
)

// ProjectRegistryRepository recuerda dónde está cada proyecto instalado y los
// encuentra en disco, para que la desinstalación de sistema no deje rastros
// (feature 034, FR-013).
type ProjectRegistryRepository interface {
	// Register anota la ruta del proyecto. Idempotente.
	Register(ctx context.Context, root string) error
	// List devuelve los proyectos registrados, incluidos los huérfanos.
	List(ctx context.Context) ([]domain.ProjectRegistration, error)
	// Scan busca proyectos de gomemory bajo root hasta maxDepth niveles, sin
	// seguir enlaces. skipped son las rutas que no se pudieron leer.
	Scan(ctx context.Context, root string, maxDepth int) (found, skipped []string, err error)
}
