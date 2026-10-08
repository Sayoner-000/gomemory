package cli

import (
	"fmt"
	"strings"

	"mem/adapters/primary/console"
)

func CmdProject(deps *Deps, args []string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no se pudo determinar el directorio de trabajo: %v", err)
	}

	project := deps.ProjectRepo.Key(root)
	dbPath := deps.ProjectRepo.DbPath(root)

	var b strings.Builder
	fmt.Fprintf(&b, "Proyecto: %s\n", project)
	fmt.Fprintf(&b, "Raíz:     %s\n", root)
	fmt.Fprintf(&b, "BD:       %s\n", dbPath)

	count := 0
	if mems, err := deps.MemoryRepo.List(project, 200); err == nil {
		count = len(mems)
	}

	fmt.Fprintf(&b, "Memorias:  %d\n", count)

	sess, _ := deps.SessionRepo.Active(project)
	if sess != nil {
		fmt.Fprintf(&b, "Sesión:    Activa desde %s\n", sess.CreatedAt)
	} else {
		b.WriteString("Sesión:    Ninguna activa\n")
	}
	if env := console.DetectEnv(); humanTerminal(env) {
		fmt.Print(console.NewLayout(env).Report("Proyecto", "", b.String()))
		return
	}
	humanf("%s", b.String())
}
