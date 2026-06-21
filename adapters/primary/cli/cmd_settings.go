package cli

import (
	"flag"
	"fmt"
)

func CmdSettings(deps *Deps, args []string) {
	fs := flag.NewFlagSet("settings", flag.ContinueOnError)
	autoApprove := fs.Bool("auto-approve", false, "Activar auto-approve en MCP")
	show := fs.Bool("show", false, "Mostrar configuración actual")
	fs.Parse(args)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no hay proyecto con memoria: %v", err)
	}

	if *show {
		s := deps.SettingsRepo.Read(root)
		fmt.Printf("Auto-approve: %v\n", s.AutoApprove)
		if s.AutoApprove {
			fmt.Printf("Tools: %v\n", s.AutoApproveTools)
		}
		return
	}

	if fs.NFlag() == 0 {
		s := deps.SettingsRepo.Read(root)
		fmt.Printf("Auto-approve: %v\n", s.AutoApprove)
		fmt.Println("\nUsa --auto-approve=true|false para cambiar")
		return
	}

	settings := deps.SettingsRepo.Read(root)
	settings.AutoApprove = *autoApprove
	if err := deps.SettingsRepo.Write(root, settings); err != nil {
		fail("guardar settings: %v", err)
	}
	deps.SettingsRepo.ApplyAutoApprove(root, settings)
	if settings.AutoApprove {
		fmt.Println("✅ Auto-approve activado — MCP configs actualizados")
	} else {
		fmt.Println("✅ Auto-approve desactivado — MCP configs actualizados")
	}
}
