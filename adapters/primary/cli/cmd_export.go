package cli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"mem/application/usecases"
)

// CmdExport vuelca las memorias y relaciones del proyecto actual a un archivo
// JSON portable (cross-OS), apto para moverlas entre proyectos y máquinas.
func CmdExport(deps *Deps, args []string) {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	out := fs.String("out", "", "archivo de salida (default gomemory-export-<proyecto>-<fecha>.json)")
	if err := fs.Parse(args); err != nil {
		return
	}

	bundle, err := usecases.ExportProject(deps.MemoryRepo, deps.RelationRepo, deps.Project)
	if err != nil {
		fail("exportar: %v", err)
	}

	path := *out
	if path == "" {
		path = fmt.Sprintf("gomemory-export-%s-%s.json", deps.Project, time.Now().Format("20060102"))
	}

	// 0600: la exportación es toda la memoria del proyecto, tan privada como
	// el almacén (feature 034, R12). os.Create la dejaba legible por otros.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		fail("crear %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	_ = f.Chmod(0o600) // un archivo previo conserva su modo al truncarse

	if err := usecases.EncodeBundle(f, bundle); err != nil {
		fail("escribir bundle: %v", err)
	}

	fmt.Printf("✓ Exportadas %d memorias y %d relaciones → %s\n", len(bundle.Memories), len(bundle.Relations), path)
}
