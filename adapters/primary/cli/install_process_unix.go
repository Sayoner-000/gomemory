//go:build !windows

package cli

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Solo el instalador real crea un grupo. init, seed y sus descendientes lo
// heredan; el puente sigue vivo hasta terminar y esperar ese árbol.
func configureInstallProcessTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return stopInstallProcessTree(cmd) }
}

func stopInstallProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// El cliente no mata al puente: le permite cerrar el grupo del instalador.
func requestInstallStop(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return cmd.Process.Signal(syscall.SIGTERM)
}
