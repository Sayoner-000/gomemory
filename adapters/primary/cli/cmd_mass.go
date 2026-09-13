package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"mem/application/usecases"
	"mem/domain"
)

// CmdMass imprime el ranking de masa de las memorias (contracts/mass.md).
func CmdMass(deps *Deps, args []string) {
	task, top, err := ParseMassFlags(args)
	if err != nil {
		fail("mass: %v", err)
	}
	if err := runMass(deps, task, top, os.Stdout); err != nil {
		fail("mass: %v", err)
	}
}

// ParseMassFlags parsea `mem mass [--task T] [--top N]`, separado de CmdMass
// para probarlo sin que un error dispare os.Exit.
func ParseMassFlags(args []string) (string, int, error) {
	fs := flag.NewFlagSet("mass", flag.ContinueOnError)
	task := fs.String("task", "", "Tarea cuyos resultados de búsqueda siembran la masa")
	top := fs.Int("top", 15, "Número máximo de memorias a listar")
	if err := fs.Parse(args); err != nil {
		return "", 0, err
	}
	if *top <= 0 {
		return "", 0, fmt.Errorf("--top debe ser positivo")
	}
	return *task, *top, nil
}

func runMass(deps *Deps, task string, top int, w io.Writer) error {
	mems, err := deps.MemoryRepo.ListAll(deps.Project)
	if err != nil {
		return err
	}
	rels, err := deps.RelationRepo.ListAll(deps.Project)
	if err != nil {
		return err
	}
	seeds, label, err := massSeedsFor(deps, mems, task)
	if err != nil {
		return err
	}
	if task != "" && len(seeds) == 0 {
		_, err := fmt.Fprintf(w, "Sin memorias que coincidan con %q: masa no calculada\n", task)
		return err
	}

	byID := make(map[int64]domain.Memory, len(mems))
	for _, m := range mems {
		byID[m.ID] = m
	}
	if _, err := fmt.Fprintf(w, "Masa de memorias — semillas: %s\n\n", label); err != nil {
		return err
	}
	for i, entry := range usecases.RankMass(mems, rels, seeds) {
		if i >= top {
			break
		}
		m := byID[entry.ID]
		if _, err := fmt.Fprintf(w, "%3d. [%d] (%s) %s — masa %.4f\n", i+1, entry.ID, m.Type, m.Title, entry.Mass); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "\n%s\n", fmt.Sprintf(usecases.MassDisclaimer, label))
	return err
}

// massSeedsFor siembra en los resultados de la tarea, con peso decreciente
// según su posición, o en el contexto vigente (sesión y hotspots) sin tarea.
func massSeedsFor(deps *Deps, mems []domain.Memory, task string) (map[int64]float64, string, error) {
	if task == "" {
		var sess *domain.Session
		if deps.SessionRepo != nil {
			sess, _ = deps.SessionRepo.Active(deps.Project)
		}
		seeds, label := usecases.ContextSeeds(mems, sess, deps.CodeProviders)
		return seeds, label, nil
	}
	found, err := deps.MemoryRepo.Search(deps.Project, task, 20)
	if err != nil {
		return nil, "", err
	}
	seeds := make(map[int64]float64)
	for _, m := range found {
		if m.Type == domain.Checkpoint {
			continue
		}
		seeds[m.ID] = 1 / float64(len(seeds)+1)
	}
	return seeds, fmt.Sprintf("tarea %q (%d)", task, len(seeds)), nil
}
