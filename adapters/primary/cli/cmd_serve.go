package cli

import (
	"flag"
	"fmt"
	"path/filepath"

	"mem/adapters/primary/mcp"
)

func CmdServe(deps *Deps, args []string) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 9735, "Puerto del servidor HTTP")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no hay .memory/ en este proyecto. Ejecuta 'mem init' primero")
	}

	project := filepath.Base(root)

	srv := mcp.NewWithRepos(deps.MemoryRepo, deps.SessionRepo, project, *port)
	fmt.Printf("🧠 gomemory server corriendo en 127.0.0.1:%d\n", *port)
	fmt.Printf("   Proyecto: %s\n", project)

	if err := srv.Start(); err != nil {
		fail("servidor HTTP: %v", err)
	}
}
