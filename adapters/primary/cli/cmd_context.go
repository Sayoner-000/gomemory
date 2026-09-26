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
	full := fs.Bool("full", false, "Entregar todo aunque ya se haya enviado en esta sesión")
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
		// --full no anota: es la vía de recuperación (ver deliverContextDocFull).
		if err == nil && deps.DeliveryLog != nil && !*full {
			_ = deps.DeliveryLog.Record(ports.DeliveryContext, usecases.HashDeContenido(output))
		}
		if err != nil {
			fail("generar contexto: %v", err)
		}
		_, _ = os.Stdout.WriteString(deliverContextDocFull(deps, output, *full))
	}
}
