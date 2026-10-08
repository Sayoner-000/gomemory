package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"mem/adapters/primary/console"
)

func CmdSearch(deps *Deps, args []string) {
	fs := newFlagSet("search", flag.ContinueOnError)
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
		humanln("Sin resultados para:", query)
		return
	}

	if env := console.DetectEnv(); humanTerminal(env) {
		l := console.NewLayout(env)
		fmt.Print(memoryPanel(l, "Resultados · «"+query+"»", fmt.Sprintf("%d", len(mems)), mems))
		fmt.Println(l.Document("\nUsa mem get <id> para ver el detalle completo"))
		return
	}
	// Se arma en memoria para poder pasarlo por el motor nativo con el nivel
	// max (feature 033); con los demás niveles la salida no cambia.
	rows := make([][]string, 0, len(mems))
	for _, m := range mems {
		content := m.Content
		content = ansi.Truncate(content, 60, "...")
		rows = append(rows, []string{fmt.Sprint(m.ID), string(m.Type), m.Title, content})
	}
	text := formatMemoryRows(console.DetectEnv(), []string{"ID", "Tipo", "Título", "Contenido"}, rows)
	_, _ = os.Stdout.WriteString(compressDeliveredContext(deps, text))
	humanf("\n(%d resultados)\n", len(mems))
}
