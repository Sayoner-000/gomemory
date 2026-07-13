package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/adapters/primary/cli"
	"mem/adapters/primary/setup"
	"mem/adapters/secondary/codegraph/codebasememory"
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
	// code-refresh: proceso de fondo (detached) que refresca el snapshot del
	// grafo externo FUERA del hot path. No abre la DB; resuelve el root y sondea
	// al proveedor con su propio timeout. Best-effort: cualquier fallo → exit 0.
	if len(os.Args) >= 2 && os.Args[1] == "code-refresh" {
		if root, err := persistence.FindRoot(); err == nil {
			if s := persistence.ReadSettings(root); !s.CodeGraphDisabled {
				codebasememory.New(root, filepath.Join(root, persistence.MemDir), s.CodeGraphCommand).Refresh(context.Background())
			}
		}
		os.Exit(0)
	}

	if len(os.Args) >= 2 && rootIndependentCommands[os.Args[1]] {
		cli.Run(os.Args[1], os.Args[2:], bootstrapDeps())
		return
	}

	if len(os.Args) < 2 {
		root, err := persistence.FindRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: no se pudo determinar el directorio de trabajo: %v\n", err)
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

	root, err := resolveRootForCommand(os.Args[1], os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: no se pudo determinar el directorio de trabajo: %v\n", err)
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

// resolveRootForCommand determina la raíz de proyecto a usar antes de
// construir el Container. Por defecto resuelve por FindRoot() (git root o
// cwd — ver adapters/secondary/persistence.FindProjectRoot). El comando
// "mcp" es la única excepción: si trae --root explícito, ese valor manda,
// porque el proceso que lo lanza (un cliente MCP) puede no fijar el cwd al
// proyecto. Sin este caso especial, --root solo afectaría qué "project" ve
// CmdMCP internamente, pero no cambiaría a qué base de datos se conecta el
// Container ya construido — un desajuste real que este fix cierra.
func resolveRootForCommand(command string, args []string) (string, error) {
	if command == "mcp" {
		for i, a := range args {
			if a == "--root" && i+1 < len(args) {
				return filepath.Abs(args[i+1])
			}
			if v, ok := strings.CutPrefix(a, "--root="); ok {
				return filepath.Abs(v)
			}
		}
	}
	return persistence.FindRoot()
}
