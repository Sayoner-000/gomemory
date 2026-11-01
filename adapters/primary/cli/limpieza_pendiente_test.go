package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

type memoriasQueFallan struct {
	ports.MemoryRepository
	borrada bool
	err     error
}

func (m memoriasQueFallan) Delete(string, int64) (bool, error) { return m.borrada, m.err }

// C-25: forget_memory informa éxito con advertencia legible por el agente.
func TestLimpiezaPendiente_MCPInformaExitoConAviso(t *testing.T) {
	err := fmt.Errorf("el texto sigue en disco: %w", domain.ErrLimpiezaPendiente)
	texto, fallo := forgetMemoryText(memoriasQueFallan{borrada: true, err: err}, "p", 7)
	if fallo != nil {
		t.Fatalf("no debe ser error: %v", fallo)
	}
	for _, parte := range []string{"Memoria 7 eliminada", "limpieza_pendiente", "sigue en disco"} {
		if !strings.Contains(texto, parte) {
			t.Errorf("falta %q en %q", parte, texto)
		}
	}
}

// C-25: otra causa sigue siendo un error aunque la memoria se borrara.
func TestLimpiezaPendiente_MCPConservaLosFallos(t *testing.T) {
	_, fallo := forgetMemoryText(memoriasQueFallan{borrada: true, err: errors.New("disco lleno")}, "p", 7)
	if fallo == nil || !strings.Contains(fallo.Error(), "disco lleno") {
		t.Fatalf("debe seguir siendo un fallo: %v", fallo)
	}
}
