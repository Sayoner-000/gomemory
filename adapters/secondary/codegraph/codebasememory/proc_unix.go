//go:build !windows

package codebasememory

import (
	"os/exec"
	"syscall"
)

// detach pone al hijo en su propia sesión (setsid) para que sobreviva a la
// salida de un proceso padre corto (hook / CLI) y no reciba su SIGHUP.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
