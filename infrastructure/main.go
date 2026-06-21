package main

import (
	"embed"
	"fmt"
	"os"

	"mem/adapters/primary/cli"
	"mem/adapters/primary/setup"
	"mem/adapters/secondary/persistence"
)

//go:embed plugin
var pluginFS embed.FS

func init() {
	setup.PluginFS = pluginFS
}

func main() {
	if len(os.Args) < 2 {
		root, err := persistence.FindRoot()
		if err != nil {
			fmt.Fprintln(os.Stderr, "No hay .memory/ en este proyecto.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  Primero inicializa la memoria:")
			fmt.Fprintln(os.Stderr, "    mem init")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  O consulta la ayuda:")
			fmt.Fprintln(os.Stderr, "    mem help")
			os.Exit(1)
		}

		container, err := NewContainer(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error al abrir TUI: %v\n", err)
			os.Exit(1)
		}
		defer container.Close()

		if err := container.RunTUI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error en TUI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	root, err := persistence.FindRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "No hay .memory/ en este proyecto.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Primero inicializa la memoria:")
		fmt.Fprintln(os.Stderr, "    mem init")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  O consulta la ayuda:")
		fmt.Fprintln(os.Stderr, "    mem help")
		os.Exit(1)
	}

	container, err := NewContainer(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al inicializar: %v\n", err)
		os.Exit(1)
	}
	defer container.Close()

	cli.Run(os.Args[1], os.Args[2:], container.ToDeps())
}
