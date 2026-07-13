package cli

import (
	"flag"
	"fmt"
)

func CmdSettings(deps *Deps, args []string) {
	fs := flag.NewFlagSet("settings", flag.ContinueOnError)
	autoApprove := fs.Bool("auto-approve", false, "Activar auto-approve en MCP")
	codeGraph := fs.Bool("code-graph", true, "Activar el grafo de código externo (codebase-memory-mcp)")
	codeGraphCmd := fs.String("code-graph-command", "", "Binario del proveedor de grafo externo (opcional)")
	show := fs.Bool("show", false, "Mostrar configuración actual")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no hay proyecto con memoria: %v", err)
	}

	if *show {
		printSettings(deps, root)
		return
	}

	if fs.NFlag() == 0 {
		printSettings(deps, root)
		fmt.Println("\nUsa --auto-approve=true|false o --code-graph=true|false para cambiar")
		return
	}

	// Solo se tocan los flags realmente pasados (fs.Visit), para no pisar el
	// resto de la configuración con sus valores por defecto.
	settings := deps.SettingsRepo.Read(root)
	autoApproveChanged := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "auto-approve":
			settings.AutoApprove = *autoApprove
			autoApproveChanged = true
		case "code-graph":
			settings.CodeGraphDisabled = !*codeGraph
		case "code-graph-command":
			settings.CodeGraphCommand = *codeGraphCmd
		}
	})

	if err := deps.SettingsRepo.Write(root, settings); err != nil {
		fail("guardar settings: %v", err)
	}
	if autoApproveChanged {
		deps.SettingsRepo.ApplyAutoApprove(root, settings)
	}
	fmt.Println("✅ Settings actualizados")
	printSettings(deps, root)
}

func printSettings(deps *Deps, root string) {
	s := deps.SettingsRepo.Read(root)
	fmt.Printf("Auto-approve: %v\n", s.AutoApprove)
	if s.AutoApprove {
		fmt.Printf("Tools: %v\n", s.AutoApproveTools)
	}
	fmt.Printf("Grafo de código externo: %v\n", !s.CodeGraphDisabled)
	if s.CodeGraphCommand != "" {
		fmt.Printf("Binario del proveedor: %s\n", s.CodeGraphCommand)
	}
}
