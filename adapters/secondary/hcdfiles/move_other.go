//go:build !darwin && !linux && !windows

package hcdfiles

func renameNoReplace(src, dst string) error {
	return linkAndRemove(src, dst)
}
