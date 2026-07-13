package cli

import (
	"flag"
	"fmt"
	"os"

	"mem/application/usecases"
)

// CmdImport lee un bundle JSON y lo importa al proyecto actual: append con dedup
// por contenido, preservando timestamps y remapeando el proyecto y los ids de
// relación. No forma sinapsis automáticas.
func CmdImport(deps *Deps, args []string) {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fail("uso: mem import <archivo.json>")
	}

	f, err := os.Open(rest[0])
	if err != nil {
		fail("abrir %s: %v", rest[0], err)
	}
	defer f.Close()

	bundle, err := usecases.DecodeBundle(f)
	if err != nil {
		fail("leer bundle: %v", err)
	}

	rep, err := usecases.ImportBundle(deps.MemoryRepo, deps.RelationRepo, deps.Project, bundle)
	if err != nil {
		fail("importar: %v", err)
	}

	fmt.Printf("✓ Import: %d memorias nuevas (%d omitidas), %d relaciones nuevas (%d omitidas)\n",
		rep.MemoriesImported, rep.MemoriesSkipped, rep.RelationsImported, rep.RelationsSkipped)
}
