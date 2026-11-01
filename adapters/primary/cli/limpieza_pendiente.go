package cli

import (
	"errors"
	"fmt"
	"os"

	"mem/domain"
)

// avisarLimpiezaPendiente imprime la advertencia y devuelve true si err solo indica
// una limpieza pendiente tras un borrado confirmado: la operación fue un éxito.
func avisarLimpiezaPendiente(err error) bool {
	if !errors.Is(err, domain.ErrLimpiezaPendiente) {
		return false
	}
	_, _ = fmt.Fprintf(os.Stderr, "⚠ %v\n", err)
	return true
}
