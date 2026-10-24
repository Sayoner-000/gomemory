//go:build windows

package atomicfs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
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
		err := renameReplace(tmp, dst)
		if err == nil || !transient(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
		if wait < 64*time.Millisecond {
			wait *= 2
		}
	}
}

// renameReplace sustituye dst con semántica POSIX: reemplaza aunque haya
// lectores con dst abierto compartiendo el borrado (como ReadFile). os.Rename
// usa MoveFileEx, que falla mientras dure cualquier lectura, y con lectores
// constantes el reintento se agotaba sin hueco. Los sistemas de archivos sin
// esa semántica (FAT, Windows anteriores a 10 1709) vuelven a os.Rename.
func renameReplace(tmp, dst string) error {
	if err := posixRename(tmp, dst); err == nil {
		return nil
	}
	return os.Rename(tmp, dst)
}

func posixRename(tmp, dst string) error {
	abs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	name, err := windows.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(from, windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(h) }()

	// FILE_RENAME_INFO: Flags (unión con ReplaceIfExists), RootDirectory,
	// FileNameLength en bytes sin el nulo y FileName.
	type renameInfo struct {
		Flags          uint32
		RootDirectory  windows.Handle
		FileNameLength uint32
		FileName       [1]uint16
	}
	nameBytes := (len(name) - 1) * 2
	buf := make([]byte, int(unsafe.Offsetof(renameInfo{}.FileName))+len(name)*2)
	info := (*renameInfo)(unsafe.Pointer(&buf[0]))
	info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	info.FileNameLength = uint32(nameBytes)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	return windows.SetFileInformationByHandle(h, windows.FileRenameInfoEx, &buf[0], uint32(len(buf)))
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
