package persistence

import (
	"path/filepath"
	"sync"
	"testing"
)

// Feature 035: con los hooks duplicados, dos procesos abren a la vez un almacén
// que todavía no existe. Pasar una base nueva a WAL pide un bloqueo exclusivo;
// si busy_timeout no rige aún, el perdedor recibe SQLITE_BUSY al instante y el
// hook termina con código 1 ("migrate: database is locked").
func TestOpenDBFile_AperturaConcurrenteDeUnaBaseNueva(t *testing.T) {
	for ronda := 0; ronda < 5; ronda++ {
		path := filepath.Join(t.TempDir(), "mem.db")
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				db, err := openDBFile(path)
				if err != nil {
					errs <- err
					return
				}
				_ = db.Close()
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("ronda %d: abrir una base nueva en paralelo no debe fallar: %v", ronda, err)
		}
	}
}
