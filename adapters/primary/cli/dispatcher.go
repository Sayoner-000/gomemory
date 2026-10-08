package cli

import (
	"fmt"
	"os"
	"strings"

	"mem/adapters/primary/console"
	"mem/version"
)

func Run(cmd string, args []string, deps *Deps) {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		switch cmd {
		case "session", "pack", "review", "docs", "forget", "get", "gc", "compact", "purge", "octopus", "adr-sync", "project", "seed", "uninstall", "mcp", "hook", "update-check", "version":
			if commandHelp(cmd) {
				return
			}
		}
	}
	rejectUnexpectedArgs(cmd, args)
	if commandHasBanner(cmd, args) {
		banner := cmd
		if (cmd == "help" || cmd == "-h" || cmd == "--help") && len(args) > 0 {
			// El logo completo es para el índice; la ayuda de un comando
			// lleva la cabecera compacta.
			banner = "help " + args[0]
		}
		console.PrintBrand(banner)
	}
	switch cmd {
	case "version", "--version", "-v":
		fmt.Println("gomemory " + version.Version)
	case "update":
		CmdUpdate(deps, args)
	case "update-check":
		CmdUpdateCheck(deps, args)
	case "index":
		CmdIndex(deps, args)
	case "init":
		CmdInit(deps, args)
	case "migrate":
		CmdMigrate(deps, args)
	case "save":
		CmdSave(deps, args)
	case "capture":
		CmdCapture(deps, args)
	case "compare", "judge":
		CmdCompare(deps, args)
	case "review":
		CmdReview(deps, args)
	case "forget":
		CmdForget(deps, args)
	case "project":
		CmdProject(deps, args)
	case "context":
		CmdContext(deps, args)
	case "plan-context":
		CmdPlanContext(deps, args)
	case "search":
		CmdSearch(deps, args)
	case "session":
		CmdSession(deps, args)
	case "pack":
		CmdPack(deps, args)
	case "mass":
		CmdMass(deps, args)
	case "octopus":
		CmdOctopus(deps, args)
	case "install":
		CmdInstall(deps, args)
	case "wrap":
		CmdWrap(deps, args)
	case "mcp":
		CmdMCP(deps, args)
	case "hook":
		CmdHook(deps, args)
	case "setup":
		CmdSetup(deps, args)
	case "setup-mcp", "mcp-setup":
		CmdMCPSetup(deps, args)
	case "list", "log":
		CmdList(deps, args)
	case "settings":
		CmdSettings(deps, args)
	case "doctor":
		CmdDoctor(deps, args)
	case "docs":
		CmdDocs(deps, args)
	case "seed":
		CmdSeed(deps, args)
	case "constitution":
		CmdConstitution(deps, args)
	case "rules":
		CmdRules(deps, args)
	case "usage":
		CmdUsage(deps, args)
	case "adr-sync":
		CmdADRSync(deps, args)
	case "purge":
		CmdPurge(deps, args)
	case "compact":
		CmdCompact(deps, args)
	case "gc":
		CmdGC(deps, args)
	case "consolidate":
		CmdConsolidate(deps, args)
	case "get":
		CmdGet(deps, args)
	case "export":
		CmdExport(deps, args)
	case "import":
		CmdImport(deps, args)
	case "uninstall":
		if code := CmdUninstall(deps, args); code != 0 {
			os.Exit(code)
		}
	case "tui":
		LaunchTUI(deps)
	case "help", "-h", "--help":
		if len(args) == 0 || !commandHelp(args[0]) {
			Usage()
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: comando desconocido '%s'\n\n", cmd)
		Usage()
		os.Exit(1)
	}
}

// Solo comandos de lectura humana: las salidas de datos y protocolos se
// mantienen utilizables por agentes y scripts, incluso con una pseudo-TTY.
func commandHasBanner(cmd string, args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		// search usa flag.FlagSet: después del primer posicional todo es
		// consulta, incluso textos que parecen flags. -n consume su valor.
		if cmd == "search" {
			if arg == "-n" || arg == "--n" {
				i++
				continue
			}
			if !strings.HasPrefix(arg, "-") || arg == "-" {
				break
			}
		}
		if strings.HasPrefix(arg, "--json") || strings.HasPrefix(arg, "--format") {
			return false
		}
	}
	switch cmd {
	case "help", "-h", "--help", "doctor", "settings", "setup", "setup-mcp", "mcp-setup", "usage", "list", "log", "search", "project", "init", "migrate", "save", "capture", "session", "forget", "compare", "judge", "seed", "purge", "compact", "gc", "consolidate", "import", "export", "adr-sync":
		return true
	}
	return false
}

// noArgCommands no aceptan opciones ni argumentos (--help se atiende antes).
var noArgCommands = map[string]bool{"seed": true, "compact": true, "project": true, "version": true, "--version": true, "-v": true, "update-check": true}

// rejectUnexpectedArgs termina con código 2 si un comando sin opciones recibe
// argumentos: ignorarlos ejecutaba la acción y declaraba éxito (C-001).
func rejectUnexpectedArgs(cmd string, args []string) {
	if !noArgCommands[cmd] || len(args) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Error: mem %s no acepta argumentos: %s\n", cmd, strings.Join(args, " "))
	exitProcess(2)
}

func fail(format string, args ...any) {
	msg := fmt.Sprintf("Error: "+format+"\n", args...)
	if env := console.DetectEnv(); humanTerminal(env) {
		// Mismo color y ajuste que el resto de la salida; sigue en stderr.
		msg = console.NewLayout(env).Document(msg)
	}
	fmt.Fprint(os.Stderr, msg)
	os.Exit(1)
}
