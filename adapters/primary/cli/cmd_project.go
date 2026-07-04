package cli

import (
	"fmt"
)

func CmdProject(deps *Deps, args []string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no se pudo determinar el directorio de trabajo: %v", err)
	}

	project := deps.ProjectRepo.Key(root)
	dbPath := deps.ProjectRepo.DbPath(root)

	fmt.Printf("Proyecto: %s\n", project)
	fmt.Printf("Raíz:     %s\n", root)
	fmt.Printf("BD:       %s\n", dbPath)

	count := 0
	if mems, err := deps.MemoryRepo.List(project, 200); err == nil {
		count = len(mems)
	}

	fmt.Printf("Memorias:  %d\n", count)

	sess, _ := deps.SessionRepo.Active(project)
	if sess != nil {
		fmt.Printf("Sesión:    Activa desde %s\n", sess.CreatedAt)
	} else {
		fmt.Println("Sesión:    Ninguna activa")
	}
}
