package main

import (
	"fmt"
	"os"
	"path/filepath"

	"mem/store"
	"mem/tui"
)

func main() {
	if len(os.Args) < 2 {
		launchTUI()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "init":
		cmdInit(args)
	case "save":
		cmdSave(args)
	case "capture":
		cmdCapture(args)
	case "compare":
		cmdCompare(args)
	case "project":
		cmdProject(args)
	case "context":
		cmdContext(args)
	case "search":
		cmdSearch(args)
	case "session":
		cmdSession(args)
	case "install":
		cmdInstall(args)
	case "wrap":
		cmdWrap(args)
	case "mcp":
		cmdMCP(args)
	case "setup-mcp", "mcp-setup":
		cmdMCPSetup(args)
	case "list", "log":
		cmdList(args)
	case "settings":
		cmdSettings(args)
	case "tui":
		launchTUI()
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Error: comando desconocido '%s'\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func launchTUI() {
	root, err := store.FindRoot()
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

	db, err := store.Open(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir base de datos: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	project := filepath.Base(root)
	if err := tui.Run(db, root, project); err != nil {
		fmt.Fprintf(os.Stderr, "Error en TUI: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`gomemory — Memoria colectiva para agentes AI

Uso:
  mem                              Abrir interfaz TUI
  mem init [--force]               Inicializar .memory/ en el proyecto
  mem save [flags] <texto>         Guardar un aprendizaje
    -t, --title    Título descriptivo
    -y, --type     Tipo: learning|decision|architecture|bugfix|pattern|discovery
    -f, --filepath Archivo relacionado

  mem capture [flags]              Guardar aprendizaje estructurado (What/Why/Where/Learned)
    -w, --what      ¿Qué se hizo?
    -y, --why       ¿Por qué?
    -f, --where     Archivos afectados
    -l, --learned   ¿Qué se aprendió?
    -t, --type      Tipo (default: learning)
    -i              Modo interactivo

  mem compare [flags] <id1> <id2>  Comparar dos memorias y persistir veredicto
    -r, --relation  related|compatible|scoped|conflicts_with|supersedes|not_conflict
    -c, --confidence  Confianza 0.0-1.0 (default: 1.0)
    -m, --reasoning   Razonamiento del veredicto
  mem compare list [-n N]          Listar relaciones guardadas

  mem project                      Detectar proyecto actual y mostrar información

  mem context [-w|--write]         Mostrar contexto de memoria
  mem search <query>               Buscar en la memoria
  mem install [dir]                Instalar gomemory en un proyecto
  mem session start                Iniciar nueva sesión
  mem session end [-s|--summary]   Finalizar sesión actual
  mem list [-n|--limit N]          Listar memorias recientes
  mem log [-n|--limit N]           Alias de list
  mem wrap <comando> [args...]     Ejecutar comando y preguntar si guardar
  mem mcp [--root <dir>]           Servidor MCP para agentes AI
  mem setup-mcp [--agents a,b,c]   Configurar MCP: opencode, claude, cursor, windsurf, cline, codex, all
  mem settings [--auto-approve=true|false] [--show]
                                    Ver o cambiar auto-approve de las tools MCP
  mem tui                          Abrir interfaz TUI explícitamente
  mem help                         Mostrar esta ayuda

Ejemplos:
  mem                              # Abrir TUI
  mem init                         # Primera vez
  mem save -t "usamos SQLite" -y decision "Base de datos SQLite"
  mem capture -w "implementar auth" -y "seguridad" -f "middleware.go" -l "usar JWT"
  mem capture -i                   # Modo interactivo
  mem compare -r supersedes -c 0.9 -m "la nueva decisión reemplaza a la anterior" 1 2
  mem project
  mem context --write
  mem search "autenticación"
  mem install ~/proyectos/mi-app   # Instalar en otro proyecto
  mem setup-mcp --agents codex,cursor
  mem settings --show
  mem session start
  mem session end -s "Implementado módulo de búsqueda"
`)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
