//go:build windows

package hcdfiles

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// taskkillNotFound es el código de salida de taskkill cuando el proceso ya no
// existe.
const taskkillNotFound = 128

// configureProcessTree termina el árbol completo de la comprobación al vencer
// el plazo (HCD-045). Windows no tiene grupos de procesos POSIX: taskkill /T
// recorre los descendientes antes de retornar.
func configureProcessTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil || cmd.Process.Pid <= 0 {
			return os.ErrProcessDone
		}
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		kill := exec.Command(filepath.Join(root, "System32", "taskkill.exe"), "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		if output, err := kill.CombinedOutput(); err != nil {
			// 128: taskkill no encontró el proceso porque ya terminó (ADM-09);
			// eso no es un fallo de la comprobación. Cualquier otro error sí se
			// propaga: el árbol puede seguir vivo y el plazo no se habría cumplido.
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == taskkillNotFound {
				return os.ErrProcessDone
			}
			return fmt.Errorf("detener el árbol de la comprobación: %w: %s", err, output)
		}
		return nil
	}
}
