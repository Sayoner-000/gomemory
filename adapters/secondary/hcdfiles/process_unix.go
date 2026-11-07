//go:build !windows

package hcdfiles

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureProcessTree ejecuta la comprobación en su propio grupo de procesos
// y, al vencer el plazo, termina el grupo entero: así ningún descendiente se
// queda con la salida heredada ni sigue escribiendo después (HCD-045).
func configureProcessTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil || cmd.Process.Pid <= 0 {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
