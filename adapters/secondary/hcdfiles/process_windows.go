//go:build windows

package hcdfiles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// configureProcessTree termina el árbol completo de la comprobación al vencer
// el plazo (HCD-045). Windows no tiene grupos de procesos POSIX: taskkill /T
// recorre los descendientes antes de retornar.
func configureProcessTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		kill := exec.Command(filepath.Join(root, "System32", "taskkill.exe"), "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		if output, err := kill.CombinedOutput(); err != nil {
			return fmt.Errorf("detener el árbol de la comprobación: %w: %s", err, output)
		}
		return nil
	}
}
