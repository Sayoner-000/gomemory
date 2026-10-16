//go:build windows

package persistence

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// noFollowFlag: Windows has no O_NOFOLLOW; the Lstat and post-open Stat checks
// apply, and creating symlinks there requires elevated rights.
const noFollowFlag = 0

func tryLockSocraticaFile(f *os.File) (bool, error) {
	var overlapped windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return false, err
}

func unlockSocraticaFile(f *os.File) {
	var overlapped windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
}
