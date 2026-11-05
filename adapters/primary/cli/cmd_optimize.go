package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"mem/application/usecases"
)

// CmdOptimize ejecuta la optimización y saneamiento de grafos, relaciones e índices.
func CmdOptimize(deps *Deps, args []string) {
	fs := newFlagSet("optimize", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "Simular la optimización sin aplicar cambios en disco")
	yes := fs.Bool("yes", false, "Confirmar y aplicar cambios sin preguntar")
	fs.BoolVar(yes, "y", false, "Alias de --yes")
	target := fs.String("target", "all", "Ámbito de optimización: all, internal, external")
	reindexExternal := fs.Bool("reindex-external", false, "Reindexar proveedores externos si están desactualizados")

	if err := fs.Parse(args); err != nil {
		return
	}
	if fs.NArg() > 0 {
		fail("argumentos inesperados: %v", fs.Args())
	}
	switch *target {
	case "", "all", "internal", "external":
	default:
		fail("optimizar grafos: ámbito de optimización inválido %q: use all, internal o external", *target)
	}

	if deps == nil || deps.ProjectRepo == nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}
	project := deps.ProjectRepo.Key(root)

	if !*yes && !*dryRun {
		prompt := fmt.Sprintf("Esto saneará relaciones huérfanas, duplicados e índices del proyecto %s. ¿Continuar?", project)
		if !ConfirmAction(os.Stdin, prompt) {
			humanln("Optimización cancelada. No se modificó nada.")
			return
		}
	}

	uc := usecases.NewOptimizeGraph(deps.MaintenanceRepo, deps.CodeProviders)
	out, err := uc.Execute(context.Background(), usecases.OptimizeGraphInput{
		Project:         project,
		DryRun:          *dryRun,
		Target:          *target,
		ReindexExternal: *reindexExternal,
	})
	if err != nil {
		fail("optimizar grafos: %v", err)
	}

	sr := out.StorageReport
	if *dryRun {
		humanln("▸ Modo simulación (dry-run): ningún dato fue modificado en disco.")
		humanf("  • Relaciones huérfanas detectadas: %d\n", sr.OrphanRelationsCleaned)
		humanf("  • Relaciones duplicadas a consolidar: %d\n", sr.DuplicateRelationsMerged)
		humanf("  • Auto-relaciones detectadas: %d\n", sr.SelfRelationsCleaned)
		humanf("  • Nodos de código huérfanos detectados: %d\n", sr.OrphanCodeNodesCleaned)
		humanf("  • Aristas de código huérfanas detectadas: %d\n", sr.OrphanCodeEdgesCleaned)
		humanf("  • Paquetes desreferenciados detectados: %d\n", sr.UnreferencedPackagesCleaned)
		for _, er := range out.ExternalReports {
			if !er.Available {
				humanf("  • %s: no disponible o no instalado.\n", er.Provider)
				continue
			}
			if er.SnapshotsCleaned > 0 {
				humanf("  • %s: %d snapshot(s) o lock(s) huérfano(s) a purgar.\n", er.Provider, er.SnapshotsCleaned)
			}
			if er.StatusSummary != "" {
				humanf("  • %s: %s\n", er.Provider, er.StatusSummary)
			}
		}
		return
	}

	total := sr.TotalCleaned()
	var extCleaned int64
	for _, er := range out.ExternalReports {
		extCleaned += int64(er.SnapshotsCleaned)
	}
	grandTotal := total + extCleaned
	if grandTotal == 0 && len(out.ExternalReports) == 0 {
		humanln("✓ El almacén y grafos ya estaban optimizados — nada que sanear.")
	} else {
		humanf("✓ Optimización completada: %d elemento(s) saneado(s).\n", grandTotal)
		if sr.OrphanRelationsCleaned > 0 || sr.DuplicateRelationsMerged > 0 || sr.SelfRelationsCleaned > 0 {
			humanf("  • Relaciones: %d huérfanas purgadas, %d duplicadas consolidadas, %d auto-relaciones eliminadas\n",
				sr.OrphanRelationsCleaned, sr.DuplicateRelationsMerged, sr.SelfRelationsCleaned)
		}
		if sr.OrphanCodeNodesCleaned > 0 || sr.OrphanCodeEdgesCleaned > 0 || sr.UnreferencedPackagesCleaned > 0 {
			humanf("  • Grafo de código: %d nodos huérfanos, %d aristas huérfanas, %d paquetes desreferenciados purgados\n",
				sr.OrphanCodeNodesCleaned, sr.OrphanCodeEdgesCleaned, sr.UnreferencedPackagesCleaned)
		}
	}

	if sr.FTSOptimized {
		humanln("  • Índices de búsqueda FTS5 optimizados.")
	}
	if sr.DatabaseOptimized {
		humanln("  • Base de datos SQLite optimizada (PRAGMA optimize).")
	}

	for _, er := range out.ExternalReports {
		if !er.Available {
			humanf("  • %s: no disponible o no instalado.\n", er.Provider)
			continue
		}
		if er.SnapshotsCleaned > 0 {
			humanf("  • %s: %d snapshot(s) huérfano(s) purgado(s).\n", er.Provider, er.SnapshotsCleaned)
		}
		if er.Reindexed {
			humanf("  • %s: reindexado completo (%d nodos, %d aristas).\n", er.Provider, er.Nodes, er.Edges)
		} else if er.StatusSummary != "" {
			humanf("  • %s: %s\n", er.Provider, er.StatusSummary)
		}
	}
}
