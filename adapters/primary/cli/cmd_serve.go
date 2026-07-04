package cli

import (
	"flag"
	"fmt"

	"mem/adapters/primary/mcp"
)

func CmdServe(deps *Deps, args []string) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 9735, "Puerto del servidor HTTP")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no se pudo determinar el directorio de trabajo: %v", err)
	}

	project := deps.ProjectRepo.Key(root)

	srv := mcp.NewWithRepos(deps.MemoryRepo, deps.SessionRepo, project, *port)
	fmt.Printf("🧠 gomemory server corriendo en 127.0.0.1:%d\n", *port)
	fmt.Printf("   Proyecto: %s\n", project)

	if err := srv.Start(); err != nil {
		fail("servidor HTTP: %v", err)
	}
}
