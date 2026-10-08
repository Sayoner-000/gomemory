package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/x/ansi"
	"mem/adapters/primary/console"
)

func CmdList(deps *Deps, args []string) {
	fs := newFlagSet("list", flag.ContinueOnError)
	limit := fs.Int("n", 20, "Número de resultados")
	if err := fs.Parse(args); err != nil {
		return
	}

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	project := deps.ProjectRepo.Key(root)
	mems, err := deps.MemoryRepo.List(project, *limit)
	if err != nil {
		fail("%v", err)
	}

	if len(mems) == 0 {
		humanln("Sin memorias guardadas. Crea una con: mem save \"tu aprendizaje\"")
		return
	}

	if env := console.DetectEnv(); humanTerminal(env) {
		l := console.NewLayout(env)
		fmt.Print(memoryPanel(l, "Memorias recientes", fmt.Sprintf("%d", len(mems)), mems))
		fmt.Println(l.Document("\nUsa mem get <id> para ver el detalle completo"))
		return
	}
	rows := make([][]string, 0, len(mems))
	for _, m := range mems {
		content := m.Content
		content = ansi.Truncate(content, 50, "...")
		date := m.CreatedAt
		if len(date) > 10 {
			date = date[:10]
		}
		rows = append(rows, []string{fmt.Sprint(m.ID), string(m.Type), m.Title, date, content})
	}
	fmt.Fprint(os.Stdout, formatMemoryRows(console.DetectEnv(), []string{"ID", "Tipo", "Título", "Fecha", "Contenido"}, rows))
	humanf("\n(%d memorias)\n", len(mems))
}
