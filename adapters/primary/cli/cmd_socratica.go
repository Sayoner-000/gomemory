package cli

import "mem/adapters/primary/cli/socratica"

// CmdSocratica preserves the CLI dispatcher boundary and existing process exit behavior.
func CmdSocratica(deps *Deps, args []string) {
	if code := socratica.Run(deps, args); code != 0 {
		exitProcess(code)
	}
}
