//go:build !windows

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
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

// installSignals son las señales que cancelan una instalación: además de
// SIGINT/SIGTERM, el cierre de la terminal (SIGHUP) y un cliente que ya no lee
// los eventos (SIGPIPE). Sin ellas el proceso moría sin cerrar su árbol (C-002).
func installSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE}
}

// notifyInstallCancel devuelve un contexto que se cancela con las señales de
// installSignals.
func notifyInstallCancel() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), installSignals()...)
}

// watchParent llama a gone cuando el proceso padre desaparece (el proceso
// queda adoptado y cambia su ppid). Cubre la muerte del padre por SIGKILL,
// que ninguna señal propia puede anunciar.
func watchParent(ctx context.Context, gone func()) {
	parent := os.Getppid()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if os.Getppid() != parent {
					gone()
					return
				}
			}
		}
	}()
}

// watchOutput llama a gone cuando el lector de fd (la salida de eventos del
// puente) se cierra: el cliente se fue aunque el puente no haya cambiado de
// padre, por ejemplo porque ya arrancó adoptado (ppid 1). Con POLLOUT, una
// pipe sin lector devuelve POLLHUP (macOS) o POLLERR (Linux); una terminal o
// un archivo no lo hacen mientras siguen disponibles.
func watchOutput(ctx context.Context, fd int, gone func()) {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
				if n, err := unix.Poll(fds, 0); err == nil && n > 0 && fds[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0 {
					gone()
					return
				}
			}
		}
	}()
}

// killOwnProcessGroup termina el grupo del instalador real con todos sus
// descendientes. Solo si el proceso es líder de su grupo (el puente lo crea
// con Setpgid); si no, se termina solo a sí mismo y nunca toca el grupo de
// quien lo lanzó.
func killOwnProcessGroup() {
	_ = syscall.Kill(ownGroupTarget(), syscall.SIGKILL)
}

// ownGroupTarget es el destino de kill: -pid (el grupo) para el líder, pid
// (solo este proceso) en cualquier otro caso.
func ownGroupTarget() int {
	pid := os.Getpid()
	if pgid, err := syscall.Getpgid(0); err == nil && pgid == pid {
		return -pid
	}
	return pid
}
