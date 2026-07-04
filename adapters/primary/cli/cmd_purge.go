package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"mem/application/ports"
)

// ParsePurgeFlags interpreta los flags de `mem purge`. defaultProject es el
// proyecto actual (resuelto por ProjectRepo.FindRoot), usado cuando el
// usuario no pasa --project ni --all (FR-003: alcance por defecto = proyecto actual).
func ParsePurgeFlags(args []string, defaultProject string) (ports.PurgeFilter, bool, error) {
	fs := flag.NewFlagSet("purge", flag.ContinueOnError)
	project := fs.String("project", "", "Proyecto objetivo (default: proyecto actual)")
	all := fs.Bool("all", false, "Purgar todos los proyectos del archivo .memory/mem.db")
	memType := fs.String("type", "", "Filtrar por tipo de memoria")
	olderThanDays := fs.Int("older-than-days", 0, "Solo memorias mas viejas que N dias")
	yes := fs.Bool("yes", false, "Omitir el prompt interactivo de confirmacion")

	if err := fs.Parse(args); err != nil {
		return ports.PurgeFilter{}, false, err
	}

	filter := ports.PurgeFilter{
		All:           *all,
		Type:          *memType,
		OlderThanDays: *olderThanDays,
	}
	if !filter.All {
		filter.Project = *project
		if filter.Project == "" {
			filter.Project = defaultProject
		}
	}

	return filter, *yes, nil
}

// ConfirmAction imprime prompt y lee una linea de r; devuelve true solo si
// la respuesta (case-insensitive, sin espacios) es "si" o "sí" (FR-002, FR-013).
func ConfirmAction(r io.Reader, prompt string) bool {
	fmt.Printf("%s [escribe 'si' para confirmar]: ", prompt)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "si" || answer == "sí"
}

func purgeScopeLabel(filter ports.PurgeFilter) string {
	if filter.All {
		return "TODOS los proyectos"
	}
	return fmt.Sprintf("el proyecto %q", filter.Project)
}

func CmdPurge(deps *Deps, args []string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}
	defaultProject := deps.ProjectRepo.Key(root)

	filter, yes, err := ParsePurgeFlags(args, defaultProject)
	if err != nil {
		fail("%v", err)
	}

	if !yes {
		prompt := fmt.Sprintf("Esto eliminara memorias de %s permanentemente. ¿Continuar?", purgeScopeLabel(filter))
		if !ConfirmAction(os.Stdin, prompt) {
			fmt.Println("Purga cancelada. No se eliminó nada.")
			return
		}
	}

	deleted, err := deps.MaintenanceRepo.Purge(filter)
	if err != nil {
		fail("purgar: %v", err)
	}

	if deleted == 0 {
		fmt.Println("No había memorias que purgar en el alcance indicado.")
		return
	}
	fmt.Printf("✅ %d memoria(s) eliminada(s) de %s.\n", deleted, purgeScopeLabel(filter))
}
