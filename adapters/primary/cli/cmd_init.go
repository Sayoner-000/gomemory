package cli

import (
	"flag"
)

// CmdInit ya no es un paso obligatorio: el store global se crea de forma
// perezosa en el primer uso real (mem save, mem mcp, etc. — ver
// specs/005-global-mcp-store). Se conserva como comando informativo y como
// disparador explícito de la migración de un .memory/mem.db legado (modelo
// de instalación por proyecto anterior a esta feature), para quien prefiera
// no esperar al primer save/mcp.
func CmdInit(deps *Deps, args []string) {
	fs := newFlagSet("init", flag.ContinueOnError)
	fs.Bool("force", false, "Ya no tiene efecto: el store global no requiere reinicialización manual")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no se pudo determinar el directorio de trabajo: %v", err)
	}

	if migrated, err := deps.ProjectRepo.MigrateLegacy(root, false); err != nil {
		fail("migrar .memory/mem.db legado: %v", err)
	} else if migrated {
		humanln("✓ Se detectó y migró un .memory/mem.db legado (instalación por proyecto anterior) al store global.")
	}

	if err := deps.ProjectRepo.Init(root); err != nil {
		fail("inicializar base de datos: %v", err)
	}

	project := deps.ProjectRepo.Key(root)
	humanf("gomemory ya está listo para el proyecto '%s' — no hace falta ejecutar 'mem init' de nuevo.\n", project)
	humanf("  Base de datos: %s\n", deps.ProjectRepo.DbPath(root))
	humanln()
	humanln("  Próximos pasos:")
	humanln("    mem save -t \"primera entrada\" \"Aprendizaje inicial del proyecto\"")
}
