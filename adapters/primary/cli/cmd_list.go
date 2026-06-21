package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

func CmdList(deps *Deps, args []string) {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	limit := fs.Int("n", 20, "Número de resultados")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := filepath.Base(root)
	mems, err := deps.MemoryRepo.List(project, *limit)
	if err != nil {
		fail("%v", err)
	}

	if len(mems) == 0 {
		fmt.Println("Sin memorias guardadas. Crea una con: mem save \"tu aprendizaje\"")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tTipo\tTítulo\tFecha\tContenido\n")
	fmt.Fprintf(w, "--\t----\t------\t-----\t--------\n")
	for _, m := range mems {
		content := m.Content
		if len(content) > 50 {
			content = content[:47] + "..."
		}
		date := m.CreatedAt
		if len(date) > 10 {
			date = date[:10]
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", m.ID, m.Type, m.Title, date, content)
	}
	w.Flush()
	fmt.Printf("\n(%d memorias)\n", len(mems))
}
