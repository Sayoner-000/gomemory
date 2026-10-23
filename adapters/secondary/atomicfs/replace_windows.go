//go:build windows

package atomicfs

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// replaceTimeout acota la espera: más allá, el destino sigue bloqueado por
// algo que no es otra escritura breve y el error se devuelve.
const replaceTimeout = 2 * time.Second

// Errores con los que Windows rechaza un rename mientras el destino está en
// uso; son transitorios cuando el otro uso es una lectura o un reemplazo.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
)

func transient(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errorAccessDenied || errno == errorSharingViolation || errno == errorLockViolation
}

func replace(tmp, dst string) error {
	deadline := time.Now().Add(replaceTimeout)
	wait := time.Millisecond
	for {
		err := os.Rename(tmp, dst)
		if err == nil || !transient(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
		if wait < 64*time.Millisecond {
			wait *= 2
		}
	}
}
