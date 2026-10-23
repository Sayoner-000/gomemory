package atomicfs

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// Reemplazos concurrentes sobre el mismo destino, con lectores a la vez: en
// Windows es justo el caso en que rename falla sin reintento.
func TestReplace_ConcurrenteConLectores(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "estado")
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				tmp, err := os.CreateTemp(dir, ".tmp-*")
				if err != nil {
					t.Error(err)
					return
				}
				_, _ = tmp.WriteString(strconv.Itoa(g*1000 + i))
				_ = tmp.Close()
				if err := Replace(tmp.Name(), dst); err != nil {
					t.Errorf("Replace: %v", err)
					return
				}
				if raw, err := ReadFile(dst); err == nil {
					if _, err := strconv.Atoi(string(raw)); err != nil {
						t.Errorf("lectura a medias: %q", raw)
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
}
