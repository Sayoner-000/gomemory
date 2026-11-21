//go:build !windows

package hcdfiles

import "os"

// linkAndRemove ofrece semántica atómica no-replace cuando el sistema de
// archivos no soporta RENAME_EXCL / RENAME_NOREPLACE: os.Link falla con
// EEXIST si dst ya existe, sin sobrescribirlo.
func linkAndRemove(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		// Solo revertir dst si src sigue existiendo y dst apunta al mismo inodo,
		// para no borrar un reemplazo concurrente ni la única copia viva.
		if srcInfo, sErr := os.Lstat(src); sErr == nil {
			if dstInfo, dErr := os.Lstat(dst); dErr == nil && os.SameFile(srcInfo, dstInfo) {
				_ = os.Remove(dst)
			}
		}
		return err
	}
	return nil
}
