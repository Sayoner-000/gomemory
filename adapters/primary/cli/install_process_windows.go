//go:build windows

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
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

// En Windows el cliente termina el árbol completo con taskkill /T; basta con
// las señales que el sistema sí entrega.
func notifyInstallCancel() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

// watchParent abre el proceso padre al arrancar (mientras sigue vivo, así el
// identificador no puede reutilizarse) y espera su fin: Windows no cambia el
// ppid al morir el padre, pero el handle sí se señala.
func watchParent(ctx context.Context, gone func()) {
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(os.Getppid()))
	if err != nil {
		return
	}
	go func() {
		defer syscall.CloseHandle(handle)
		for ctx.Err() == nil {
			if ev, _ := syscall.WaitForSingleObject(handle, 200); ev == syscall.WAIT_OBJECT_0 {
				gone()
				return
			}
		}
	}()
}

// killOwnProcessGroup termina este proceso y su árbol de descendientes.
func killOwnProcessGroup() {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	_ = exec.Command(filepath.Join(root, "System32", "taskkill.exe"), "/PID", strconv.Itoa(os.Getpid()), "/T", "/F").Run()
	exitProcess(1)
}

// En Windows la muerte del cliente se detecta por su handle (watchParent) y
// el cliente termina el árbol con taskkill /T.
func watchOutput(context.Context, int, func()) {}
