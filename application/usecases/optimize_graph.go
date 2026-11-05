package usecases

import (
	"context"
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// OptimizeGraphInput define los parámetros de ejecución del caso de uso.
type OptimizeGraphInput struct {
	Project         string
	DryRun          bool
	Target          string // "all", "internal", "external" (por defecto "all")
	ReindexExternal bool
}

// OptimizeGraphOutput reúne los resultados de optimización del almacén interno y externos.
type OptimizeGraphOutput struct {
	StorageReport   domain.StorageOptimizationReport
	ExternalReports []domain.ExternalProviderOptimizeReport
}

// OptimizeGraph orquesta el mantenimiento y saneamiento de grafos e índices.
type OptimizeGraph struct {
	maintRepo ports.MaintenanceRepository
	providers []ports.CodeGraphProvider
}

// NewOptimizeGraph construye el caso de uso con las dependencias necesarias.
func NewOptimizeGraph(maintRepo ports.MaintenanceRepository, providers []ports.CodeGraphProvider) *OptimizeGraph {
	return &OptimizeGraph{
		maintRepo: maintRepo,
		providers: providers,
	}
}

// Execute ejecuta la optimización de acuerdo al alcance y opciones seleccionadas.
func (u *OptimizeGraph) Execute(ctx context.Context, in OptimizeGraphInput) (OptimizeGraphOutput, error) {
	out := OptimizeGraphOutput{}
	target := in.Target
	if target == "" {
		target = "all"
	}

	switch target {
	case "all", "internal", "external":
	default:
		return out, fmt.Errorf("ámbito de optimización inválido %q: use all, internal o external", target)
	}

	if target == "all" || target == "internal" {
		if u.maintRepo != nil {
			rep, err := u.maintRepo.OptimizeStorage(ctx, in.Project, in.DryRun)
			if err != nil {
				return out, fmt.Errorf("optimizar almacenamiento: %w", err)
			}
			out.StorageReport = rep
		}
	}

	if target == "all" || target == "external" {
		for _, p := range u.providers {
			if maintainer, ok := p.(ports.CodeGraphMaintainer); ok {
				rep, err := maintainer.Optimize(ctx, in.DryRun, in.ReindexExternal)
				if err != nil {
					return out, fmt.Errorf("optimizar proveedor %s: %w", p.Name(), err)
				}
				out.ExternalReports = append(out.ExternalReports, rep)
			}
		}
	}

	return out, nil
}
