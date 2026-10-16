//go:build !windows

package persistence

import (
	"errors"
	"os"
	"syscall"
)

// noFollowFlag makes OpenFile fail on a symlink instead of following it.
const noFollowFlag = syscall.O_NOFOLLOW

func tryLockSocraticaFile(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}
	return false, err
}

func unlockSocraticaFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
