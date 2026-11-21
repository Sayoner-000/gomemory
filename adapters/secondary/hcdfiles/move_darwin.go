//go:build darwin

package hcdfiles

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplace(src, dst string) error {
	err := unix.RenamexNp(src, dst, unix.RENAME_EXCL)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EEXIST) {
		return os.ErrExist
	}
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL) {
		return linkAndRemove(src, dst)
	}
	return &os.LinkError{Op: "rename", Old: src, New: dst, Err: err}
}
