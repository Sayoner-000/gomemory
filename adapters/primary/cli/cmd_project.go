package cli

func CmdProject(deps *Deps, args []string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no se pudo determinar el directorio de trabajo: %v", err)
	}

	project := deps.ProjectRepo.Key(root)
	dbPath := deps.ProjectRepo.DbPath(root)

	humanf("Proyecto: %s\n", project)
	humanf("Raíz:     %s\n", root)
	humanf("BD:       %s\n", dbPath)

	count := 0
	if mems, err := deps.MemoryRepo.List(project, 200); err == nil {
		count = len(mems)
	}

	humanf("Memorias:  %d\n", count)

	sess, _ := deps.SessionRepo.Active(project)
	if sess != nil {
		humanf("Sesión:    Activa desde %s\n", sess.CreatedAt)
	} else {
		humanln("Sesión:    Ninguna activa")
	}
}
