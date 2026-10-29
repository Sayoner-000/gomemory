package hcd

import (
	"os"
	"testing"
)

// TestMain aísla el store global: sin esto, persistence.Init de los tests del
// motor creaba proyectos en el ~/.local/share/gomemory real de quien corre la
// suite (los «001-*» que dejó la feature 045).
func TestMain(m *testing.M) {
	dataHome, err := os.MkdirTemp("", "gomemory-cli-hcd-test-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("GOMEMORY_DATA_HOME", dataHome); err != nil {
		panic(err)
	}

	code := m.Run()
	_ = os.RemoveAll(dataHome)
	os.Exit(code)
}
