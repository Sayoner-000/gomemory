package cli

import (
	"fmt"
	"os"
	"strings"

	"mem/application/ports"
)

const gcDefaultOlderThanDays = 90

// ParseGCFlags interpreta los flags de `mem gc`. Reutiliza ParsePurgeFlags
// (mismo PurgeFilter, mismo MaintenanceRepository.Purge) y solo cambia el
// default de OlderThanDays a 90 (FR-009) — el GC nunca se dispara solo.
func ParseGCFlags(args []string, defaultProject string) (ports.PurgeFilter, bool, error) {
	withDefault := append([]string{}, args...)
	hasOlderThanDays := false
	for _, a := range args {
		if a == "--older-than-days" || strings.HasPrefix(a, "--older-than-days=") {
			hasOlderThanDays = true
			break
		}
	}
	if !hasOlderThanDays {
		withDefault = append(withDefault, "--older-than-days", fmt.Sprintf("%d", gcDefaultOlderThanDays))
	}
	return ParsePurgeFlags(withDefault, defaultProject)
}

func CmdGC(deps *Deps, args []string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}
	defaultProject := deps.ProjectRepo.Key(root)

	filter, yes, err := ParseGCFlags(args, defaultProject)
	if err != nil {
		fail("%v", err)
	}

	if !yes {
		prompt := fmt.Sprintf("Esto eliminará memorias de %s con más de %d días de antigüedad. ¿Continuar?",
			purgeScopeLabel(filter), filter.OlderThanDays)
		if !ConfirmAction(os.Stdin, prompt) {
			fmt.Println("Garbage collection cancelado. No se eliminó nada.")
			return
		}
	}

	deleted, err := deps.MaintenanceRepo.Purge(filter)
	if err != nil {
		fail("garbage collection: %v", err)
	}

	if deleted == 0 {
		fmt.Println("No había memorias más viejas que el umbral indicado.")
		return
	}
	fmt.Printf("✅ %d memoria(s) eliminada(s) por garbage collection de %s.\n", deleted, purgeScopeLabel(filter))
}
