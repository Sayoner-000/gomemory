package cli

import "mem/adapters/primary/cli/hcd"

func CmdHCD(deps *Deps, args []string) {
	if code := hcd.Run(deps, args); code != 0 {
		exitProcess(code)
	}
}
