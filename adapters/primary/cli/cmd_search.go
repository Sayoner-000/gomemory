package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func CmdSearch(deps *Deps, args []string) {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("n", 20, "Número de resultados")
	fs.Parse(args)

	query := strings.Join(fs.Args(), " ")
	if query == "" {
		fail("la consulta de búsqueda es obligatoria\nEjemplo: mem search \"autenticación\"")
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := deps.ProjectRepo.Key(root)
	mems, err := deps.MemoryRepo.Search(project, query, *limit)
	if err != nil {
		fail("buscar: %v", err)
	}

	if len(mems) == 0 {
		fmt.Println("Sin resultados para:", query)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tTipo\tTítulo\tContenido\n")
	fmt.Fprintf(w, "--\t----\t------\t--------\n")
	for _, m := range mems {
		content := m.Content
		if len(content) > 60 {
			content = content[:57] + "..."
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", m.ID, m.Type, m.Title, content)
	}
	w.Flush()
	fmt.Printf("\n(%d resultados)\n", len(mems))
}
