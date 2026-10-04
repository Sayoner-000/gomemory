package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// T056 (feature 035, FR-022): varios procesos MCP suman huella a la vez. El
// archivo llegó a contener 5078217904191461789312457.
func TestFootprintAdd_ConcurrenteSiempreEsUnEntero(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				footprintAdd(root, 100)
			}
		}()
	}
	wg.Wait()
	raw, err := os.ReadFile(footprintPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := strconv.Atoi(strings.TrimSpace(string(raw))); err != nil || n <= 0 {
		t.Errorf("la huella debe ser un entero positivo: %q", raw)
	}
}

func TestFootprintRead_ValorCorruptoSeReinicia(t *testing.T) {
	root := t.TempDir()
	p := footprintPath(root)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte("5078217904191461789312457"), 0o644)
	if got := footprintRead(root); got != 0 {
		t.Errorf("un valor corrupto se lee como 0, got %d", got)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("el archivo corrupto debe borrarse para que la huella vuelva a contar")
	}
}
