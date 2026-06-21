package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"mem/adapters/primary/setup"
)

func CmdSetup(deps *Deps, args []string) {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	agent := fs.String("agent", "", "Agente: opencode, claude-code")
	target := fs.String("target", ".", "Directorio del proyecto")
	port := fs.Int("port", 9735, "Puerto del servidor HTTP")
	fs.Parse(args)

	if *agent == "" && fs.NArg() > 0 {
		*agent = fs.Arg(0)
	}

	if *agent == "" {
		fmt.Println("Uso: mem setup <agent> [--target dir] [--port 9735]")
		fmt.Println("Agentes: opencode, claude-code")
		os.Exit(1)
	}

	root := *target
	if root == "." {
		var err error
		root, err = deps.ProjectRepo.FindRoot()
		if err != nil {
			root = "."
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fail("ruta inválida: %v", err)
	}

	binPath := filepath.Join(absRoot, "mem")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		self, err := os.Executable()
		if err == nil {
			binPath = self
		}
	}

	fmt.Printf("🔌 Instalando plugin de gomemory para %s\n\n", *agent)

	switch *agent {
	case "opencode":
		if err := setup.InstallOpenCode(absRoot, binPath, *port); err != nil {
			fail("error instalando plugin opencode: %v", err)
		}
	case "claude-code", "claude":
		if err := setup.InstallClaudeCode(absRoot, binPath, *port); err != nil {
			fail("error instalando plugin claude-code: %v", err)
		}
	default:
		fmt.Printf("Agente desconocido: %s\n", *agent)
		fmt.Println("Agentes disponibles: opencode, claude-code")
		os.Exit(1)
	}

	fmt.Printf("\n✅ Plugin %s instalado. Reinicia el agente para activarlo.\n", *agent)
	_ = port
}
