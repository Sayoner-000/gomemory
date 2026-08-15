package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"mem/application/ports"
	"mem/application/usecases"
)

// CmdIndex indexa manualmente el código Go del proyecto (`mem index [--force]`)
// y, salvo --skip-graph, refresca también el grafo de código externo
// (opcional, multi-lenguaje) si hay un proveedor configurado que lo soporte.
// El indexado incremental normal ocurre solo vía el hook turn-end tras cada
// turno del agente; este comando es para correrlo a demanda (primera carga
// de un proyecto grande, o forzar un reindexado completo con --force).
func CmdIndex(deps *Deps, args []string) {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	force := fs.Bool("force", false, "Reindexar todos los archivos aunque no hayan cambiado")
	skipGraph := fs.Bool("skip-graph", false, "Omitir el refresco del grafo de código externo")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}
	project := deps.ProjectRepo.Key(root)

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

	if !*skipGraph {
		indexExternalGraph(deps)
	}
}

// indexExternalGraph refresca el grafo de código externo (opcional,
// multi-lenguaje) tras el indexado nativo Go. El indexado nativo ya se
// completó con éxito y es el resultado principal del comando: cualquier
// problema con el proveedor externo se reporta como línea informativa
// (no instalado) o advertencia (fallo real), pero nunca hace fallar el
// comando — el exit code sigue en 0.
func indexExternalGraph(deps *Deps) {
	if len(deps.CodeProviders) == 0 {
		fmt.Println("  (grafo externo: sin proveedor configurado, omitido)")
		return
	}
	indexer, ok := deps.CodeProviders[0].(ports.CodeGraphIndexer)
	if !ok {
		fmt.Println("  (grafo externo: el proveedor configurado no soporta reindexado, omitido)")
		return
	}
	fmt.Printf("🔗 Indexando grafo externo (%s)...\n", indexer.Name())
	nodes, edges, err := indexer.IndexRepository(context.Background(), "full")
	if err != nil {
		if errors.Is(err, ports.ErrIndexerNotInstalled) {
			fmt.Println("  (grafo externo: codebase-memory-mcp no está instalado en PATH, omitido)")
			return
		}
		fmt.Printf("  ⚠️  grafo externo: %v\n", err)
		return
	}
	fmt.Printf("  Nodos: %d, aristas: %d\n", nodes, edges)
}
