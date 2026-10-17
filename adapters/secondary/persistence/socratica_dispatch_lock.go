package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// createSocraticaLockFile opens (creating if needed) a lock file that must be
// a regular file. The no-follow flag makes the check part of the open itself,
// so a symlink swapped in between cannot redirect it (H-06).
func createSocraticaLockFile(path string) (*os.File, error) {
	return openLockWithFlags(path, os.O_CREATE|os.O_RDWR)
}

// openSocraticaLockFile opens an existing lock file with the same guarantees.
func openSocraticaLockFile(path string) (*os.File, error) {
	return openLockWithFlags(path, os.O_RDWR)
}

func openLockWithFlags(path string, flags int) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("invalid socratica lock file")
	}
	f, err := os.OpenFile(path, flags|noFollowFlag, 0o600)
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("invalid socratica lock file")
	}
	return f, nil
}

type SocraticaFileLock struct{ root string }

func NewSocraticaDispatchLock(root string) *SocraticaFileLock { return &SocraticaFileLock{root: root} }
func (l *SocraticaFileLock) path(id string) (string, error) {
	if id == "" || id == "." || strings.ContainsAny(id, "/\\\x00") || len(id) > 128 {
		return "", fmt.Errorf("invalid socratica flow id")
	}
	return filepath.Join(l.root, ".memory", "socratica", "locks", id+".lock"), nil
}

// Remove drops the lock file of a purged flow only after taking the lock
// itself: a lock someone still holds is never removed (H-06). Unix removes it
// while held; Windows cannot delete an open file, so it releases first.
func (l *SocraticaFileLock) Remove(id string) error {
	path, err := l.path(id)
	if err != nil {
		return err
	}
	f, err := openSocraticaLockFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	acquired, err := tryLockSocraticaFile(f)
	if err != nil || !acquired {
		_ = f.Close()
		return err
	}
	if runtime.GOOS != "windows" {
		err = os.Remove(path)
	}
	unlockSocraticaFile(f)
	_ = f.Close()
	if runtime.GOOS == "windows" {
		err = os.Remove(path)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List returns the flow ids with a lock file. Entries that are not a valid
// "<id>.lock" name are ignored, never removed.
func (l *SocraticaFileLock) List() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(l.root, ".memory", "socratica", "locks"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".lock")
		if !ok || !e.Type().IsRegular() {
			continue
		}
		if _, err := l.path(id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (l *SocraticaFileLock) TryAcquire(id string) (func(), bool, error) {
	path, err := l.path(id)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, false, err
	}
	f, err := createSocraticaLockFile(path)
	if err != nil {
		return nil, false, err
	}
	acquired, err := tryLockSocraticaFile(f)
	if err != nil || !acquired {
		_ = f.Close()
		return nil, false, err
	}
	var once sync.Once
	return func() { once.Do(func() { unlockSocraticaFile(f); _ = f.Close() }) }, true, nil
}
