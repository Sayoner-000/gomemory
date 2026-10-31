package cli

import (
	"fmt"
	"os"

	"mem/adapters/primary/setup"
)

// retireGlobalBlocks quita los bloques marcados de gomemory de los archivos de
// instrucciones de usuario. Copia cada archivo antes de tocarlo: si la copia
// falla, ese archivo no se modifica. Un archivo sin bloques no se copia, lo que
// hace idempotente la migración. Devuelve las rutas retiradas.
func retireGlobalBlocks(paths []string, stamp string) ([]string, []error) {
	var retired []string
	var errs []error
	for _, path := range paths {
		// Un enlace de dotfiles se migra en su destino: el rename del reemplazo
		// sustituiría el enlace por un archivo normal.
		real := setup.GlobalConfigPath(path)
		data, err := os.ReadFile(real)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if content := string(data); protocolStart(content) == -1 && universalInstructionsStart(content) == -1 {
			continue
		}
		if err := copyBeforeRetire(real, data, stamp); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := removeProtocolBlockFile(real); err != nil {
			errs = append(errs, err)
			continue
		}
		retired = append(retired, path)
	}
	return retired, errs
}

// copyBeforeRetire nunca reemplaza una copia previa: O_EXCL hace fallar una
// colisión, y con ello se deja intacto el original.
func copyBeforeRetire(path string, data []byte, stamp string) error {
	backup := path + ".gomemory-backup-" + stamp
	f, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode(path))
	if err != nil {
		return fmt.Errorf("copiar %s antes de retirar sus bloques: %w", path, err)
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(backup)
		return fmt.Errorf("copiar %s antes de retirar sus bloques: %w", path, err)
	}
	return nil
}
