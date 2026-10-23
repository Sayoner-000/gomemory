//go:build !windows

package atomicfs

import "os"

func replace(tmp, dst string) error {
	return os.Rename(tmp, dst)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
