package cli

import (
	"flag"
	"fmt"
)

// CmdMigrate expone explícitamente la migración de un `.memory/mem.db`
// legado (modelo de instalación por proyecto anterior a esta feature) al
// store global. El init perezoso ya migra automáticamente en el primer uso
// (mem save, mem mcp, etc.) sin necesidad de este comando — mem migrate
// sirve para forzarla con --force en el caso "ambos existen", o para
// confirmar explícitamente que ya no queda nada pendiente.
func CmdMigrate(deps *Deps, args []string) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	force := fs.Bool("force", false, "Sobrescribir el store global si ya tenía datos propios")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no se pudo determinar el directorio de trabajo: %v", err)
	}

	migrated, err := deps.ProjectRepo.MigrateLegacy(root, *force)
	if err != nil {
		fail("%v", err)
	}
	if !migrated {
		fmt.Println("Nada que migrar: no hay .memory/mem.db legado en este proyecto (o ya se migró antes).")
		return
	}

	project := deps.ProjectRepo.Key(root)
	count := 0
	if mems, err := deps.MemoryRepo.List(project, 1_000_000); err == nil {
		count = len(mems)
	}
	fmt.Printf("✓ Migración completa: %d memoria(s) movidas al store global\n", count)
	fmt.Printf("  Base de datos: %s\n", deps.ProjectRepo.DbPath(root))
}
