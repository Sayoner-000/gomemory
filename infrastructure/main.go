package main

import (
	"embed"
	"fmt"
	"os"

	"mem/adapters/primary/cli"
	"mem/adapters/primary/setup"
	"mem/adapters/secondary/persistence"
)

// bootstrapDeps construye Deps para comandos que no requieren un proyecto
// con memoria ya inicializada. ProjectRepo y SettingsRepo no dependen de una
// conexión a base de datos abierta.
func bootstrapDeps() *cli.Deps {
	return &cli.Deps{
		SettingsRepo: persistence.NewSettingsRepository(),
		ProjectRepo:  persistence.NewProjectRepository(),
	}
}

//go:embed all:plugin
var pluginFS embed.FS

//go:embed all:templates
var templatesFS embed.FS

func init() {
	setup.PluginFS = pluginFS
	cli.TemplatesFS = templatesFS
}

// rootIndependentCommands no requieren un .memory/ preexistente: ellos mismos
// lo crean (init, install) o solo escriben configuración de archivos (setup,
// setup-mcp, mcp-setup, help). Despachar sin pasar por FindRoot()/NewContainer.
var rootIndependentCommands = map[string]bool{
	"init":      true,
	"install":   true,
	"setup":     true,
	"setup-mcp": true,
	"mcp-setup": true,
	"help":      true,
	"-h":        true,
	"--help":    true,
	"update":    true,
	"version":   true,
	"--version": true,
	"-v":        true,
}

func main() {
	if len(os.Args) >= 2 && rootIndependentCommands[os.Args[1]] {
		cli.Run(os.Args[1], os.Args[2:], bootstrapDeps())
		return
	}

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
