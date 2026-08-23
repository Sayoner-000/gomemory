package cli

import (
	"flag"
	"fmt"
	"mem/application/ports"
	"mem/application/usecases"
	"os"
	"path/filepath"
)

func CmdContext(deps *Deps, args []string) {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	write := fs.Bool("w", false, "Escribir a .memory/context.md")
	fs.BoolVar(write, "write", false, "Escribir a .memory/context.md")
	if err := fs.Parse(args); err != nil {
		return
	}

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
		// Se anota lo entregado para que la operación de contexto para
		// planificar no lo reenvíe en esta misma sesión (feature 023, FR-006).
		if err == nil && deps.DeliveryLog != nil {
			deps.DeliveryLog.Record(ports.DeliveryContext, usecases.HashDeContenido(output))
		}
		if err != nil {
			fail("generar contexto: %v", err)
		}
		os.Stdout.WriteString(output)
	}
}
