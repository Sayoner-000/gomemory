//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func configureInstallProcessTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return stopInstallProcessTree(cmd) }
}

// Windows no entrega SIGTERM como señal capturable. taskkill /T termina el
// árbol completo antes de retornar; el llamador aún espera al proceso raíz.
func stopInstallProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	command := exec.Command(filepath.Join(root, "System32", "taskkill.exe"), "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("detener árbol de instalación: %w: %s", err, output)
	}
	return nil
}

func requestInstallStop(cmd *exec.Cmd) error { return stopInstallProcessTree(cmd) }
