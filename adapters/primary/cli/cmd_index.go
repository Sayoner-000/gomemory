package cli

import (
	"flag"
	"fmt"
	"path/filepath"

	"mem/application/usecases"
)

// CmdIndex indexa manualmente el código Go del proyecto (`mem index [--force]`).
// El indexado incremental normal ocurre solo vía el hook turn-end tras cada
// turno del agente; este comando es para correrlo a demanda (primera carga
// de un proyecto grande, o forzar un reindexado completo con --force).
func CmdIndex(deps *Deps, args []string) {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	force := fs.Bool("force", false, "Reindexar todos los archivos aunque no hayan cambiado")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}
	project := filepath.Base(root)

	ix := usecases.NewIndexer(deps.CodeGraphRepo, root, project)
	fmt.Println("🔍 Indexando código Go...")
	report, err := ix.IndexProject(*force)
	if err != nil {
		fail("indexar: %v", err)
	}

	fmt.Printf("  Escaneados: %d, parseados: %d, omitidos (sin cambios): %d, eliminados: %d\n",
		report.Scanned, report.Parsed, report.Skipped, report.Deleted)
	fmt.Printf("  Nodos: %d, aristas: %d\n", report.Nodes, report.Edges)
	fmt.Printf("  ✅ Listo en %s\n", report.Duration.Round(1e6))
}
