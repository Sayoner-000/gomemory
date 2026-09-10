package usecases_test

import (
	"os"
	"testing"
)

// TestMain sandboxea el store global de gomemory en un directorio temporal
// para toda la suite de este paquete, evitando que los tests escriban en el
// $HOME real de la máquina que los ejecuta (ver
// adapters/secondary/persistence/globalstore.go, DataHome).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gomemory-test-usecases-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("GOMEMORY_DATA_HOME", dir); err != nil {
		panic(err)
	}

	code := m.Run()

	if err := os.RemoveAll(dir); err != nil {
		panic(err)
	}
	os.Exit(code)
}
