package cli

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func CmdSearch(deps *Deps, args []string) {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("n", 20, "Número de resultados")
	if err := fs.Parse(args); err != nil {
		return
	}

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

	// Se arma en memoria para poder pasarlo por el motor nativo con el nivel
	// max (feature 033); con los demás niveles la salida no cambia.
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tTipo\tTítulo\tContenido")
	_, _ = fmt.Fprintln(w, "--\t----\t------\t--------")
	for _, m := range mems {
		content := m.Content
		if len(content) > 60 {
			content = content[:57] + "..."
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", m.ID, m.Type, m.Title, content)
	}
	_ = w.Flush()
	_, _ = os.Stdout.WriteString(compressDeliveredContext(deps, buf.String()))
	fmt.Printf("\n(%d resultados)\n", len(mems))
}
