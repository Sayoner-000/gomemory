package cli

import (
	"fmt"
	"os"

	"mem/version"
)

func Run(cmd string, args []string, deps *Deps) {
	switch cmd {
	case "version", "--version", "-v":
		fmt.Println("gomemory " + version.Version)
	case "update":
		CmdUpdate(deps, args)
	case "index":
		CmdIndex(deps, args)
	case "init":
		CmdInit(deps, args)
	case "save":
		CmdSave(deps, args)
	case "capture":
		CmdCapture(deps, args)
	case "compare", "judge":
		CmdCompare(deps, args)
	case "forget":
		CmdForget(deps, args)
	case "project":
		CmdProject(deps, args)
	case "context":
		CmdContext(deps, args)
	case "search":
		CmdSearch(deps, args)
	case "session":
		CmdSession(deps, args)
	case "install":
		CmdInstall(deps, args)
	case "wrap":
		CmdWrap(deps, args)
	case "mcp":
		CmdMCP(deps, args)
	case "serve":
		CmdServe(deps, args)
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
	case "purge":
		CmdPurge(deps, args)
	case "compact":
		CmdCompact(deps, args)
	case "gc":
		CmdGC(deps, args)
	case "uninstall":
		CmdUninstall(deps, args)
	case "tui":
		LaunchTUI(deps)
	case "help", "-h", "--help":
		Usage()
	default:
		fmt.Fprintf(os.Stderr, "Error: comando desconocido '%s'\n\n", cmd)
		Usage()
		os.Exit(1)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
