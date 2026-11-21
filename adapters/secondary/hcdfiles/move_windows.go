//go:build windows

package hcdfiles

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func renameNoReplace(src, dst string) error {
	from, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	// MoveFileEx sin MOVEFILE_REPLACE_EXISTING falla de forma atómica si dst existe.
	err = windows.MoveFileEx(from, to, 0)
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
		return os.ErrExist
	}
	return &os.LinkError{Op: "rename", Old: src, New: dst, Err: err}
}
