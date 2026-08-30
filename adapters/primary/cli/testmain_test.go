package cli

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dataHome, err := os.MkdirTemp("", "gomemory-cli-test-*")
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
