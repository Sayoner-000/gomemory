package main

import (
	"flag"
	"fmt"

	"mem/store"
)

func cmdSettings(args []string) {
	fs := flag.NewFlagSet("settings", flag.ContinueOnError)
	autoApprove := fs.Bool("auto-approve", false, "Activar auto-approve en MCP")
	show := fs.Bool("show", false, "Mostrar configuración actual")
	fs.Parse(args)

	root, err := store.FindRoot()
	if err != nil {
		fail("no hay proyecto con memoria: %v", err)
	}

	if *show {
		s := store.ReadSettings(root)
		fmt.Printf("Auto-approve: %v\n", s.AutoApprove)
		if s.AutoApprove {
			fmt.Printf("Tools: %v\n", s.AutoApproveTools)
		}
		return
	}

	if fs.NFlag() == 0 {
		s := store.ReadSettings(root)
		fmt.Printf("Auto-approve: %v\n", s.AutoApprove)
		fmt.Println("\nUsa --auto-approve=true|false para cambiar")
		return
	}

	settings := store.ReadSettings(root)
	settings.AutoApprove = *autoApprove
	if err := store.WriteSettings(root, settings); err != nil {
		fail("guardar settings: %v", err)
	}
	store.ApplyAutoApprove(root, settings)
	if settings.AutoApprove {
		fmt.Println("✅ Auto-approve activado — MCP configs actualizados")
	} else {
		fmt.Println("✅ Auto-approve desactivado — MCP configs actualizados")
	}
}
