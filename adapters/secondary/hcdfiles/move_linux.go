//go:build linux

package hcdfiles

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplace(src, dst string) error {
	err := unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EEXIST) {
		return os.ErrExist
	}
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) {
		return linkAndRemove(src, dst)
	}
	return &os.LinkError{Op: "rename", Old: src, New: dst, Err: err}
}
