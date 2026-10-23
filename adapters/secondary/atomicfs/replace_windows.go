//go:build windows

package atomicfs

import (
	"errors"
	"io"
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

// readTimeout es menor que replaceTimeout: un lector solo espera a que
// termine un reemplazo en curso.
const readTimeout = 500 * time.Millisecond

func readFile(path string) ([]byte, error) {
	deadline := time.Now().Add(readTimeout)
	wait := time.Millisecond
	for {
		data, err := readShared(path)
		if err == nil || !transient(err) || time.Now().After(deadline) {
			return data, err
		}
		time.Sleep(wait)
		if wait < 32*time.Millisecond {
			wait *= 2
		}
	}
}

func readShared(path string) ([]byte, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	return io.ReadAll(f)
}
