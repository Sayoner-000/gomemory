package ports

import (
	"context"
	"mem/domain"
)

// CodeGraphMaintainer es una capacidad opcional de un proveedor de grafo de código
// para verificar su estado de salud, sanear snapshots/bloqueos obsoletos y,
// opcionalmente, disparar un reindexado de mantenimiento.
type CodeGraphMaintainer interface {
	Optimize(ctx context.Context, dryRun bool, reindex bool) (domain.ExternalProviderOptimizeReport, error)
}
