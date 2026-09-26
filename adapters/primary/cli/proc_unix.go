//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// detach pone al hijo en su propia sesión (setsid) para que sobreviva a la
// salida del hook que lo lanza y no reciba su SIGHUP. Mismo criterio que el
// refresco del grafo (codebasememory).
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
