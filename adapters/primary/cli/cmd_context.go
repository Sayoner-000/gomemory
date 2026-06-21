package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func CmdContext(deps *Deps, args []string) {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	write := fs.Bool("w", false, "Escribir a .memory/context.md")
	fs.BoolVar(write, "write", false, "Escribir a .memory/context.md")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("%v", err)
	}

	if *write {
		if err := deps.ContextBuilder.WriteFile(); err != nil {
			fail("escribir context.md: %v", err)
		}
		fmt.Printf("✓ Contexto escrito en %s\n", filepath.Join(root, deps.ProjectRepo.MemDir(), "context.md"))
	} else {
		output, err := deps.ContextBuilder.Build()
		if err != nil {
			fail("generar contexto: %v", err)
		}
		os.Stdout.WriteString(output)
	}
}
