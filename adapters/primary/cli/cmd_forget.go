package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// CmdForget borra una memoria puntual del proyecto actual por su ID.
// A diferencia de `mem purge` (borrado masivo por filtro), este comando
// borra exactamente una memoria y no requiere confirmación interactiva:
// el ID explícito ya es la confirmación.
func CmdForget(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("uso: mem forget <id>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fail("ID inválido: %s", args[0])
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}
	project := filepath.Base(root)

	deleted, err := deps.MemoryRepo.Delete(project, id)
	if err != nil {
		fail("borrar memoria: %v", err)
	}
	if !deleted {
		fail("memoria %d no encontrada en el proyecto '%s'", id, project)
	}
	fmt.Printf("✓ Memoria %d eliminada\n", id)
}
