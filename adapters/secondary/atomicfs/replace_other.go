//go:build !windows

package atomicfs

import "os"

func replace(tmp, dst string) error {
	return os.Rename(tmp, dst)
}
