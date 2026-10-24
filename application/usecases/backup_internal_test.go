package usecases

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// Con la marca de tiempo repetida (reloj de Windows), cada snapshot conserva
// su archivo y la poda retira el más antiguo, no uno al azar.
func TestCreateSnapshot_RepeatedClockKeepsEverySnapshot(t *testing.T) {
	fixed := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	snapshotNow = func() time.Time { return fixed }
	t.Cleanup(func() { snapshotNow = time.Now })

	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	memRepo := persistence.NewMemoryRepository(db)
	relRepo := persistence.NewRelationRepository(db)
	if _, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Content: "cuerpo"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	dir := t.TempDir()
	var paths []string
	for i := 0; i < 4; i++ {
		p, err := CreateSnapshot(memRepo, relRepo, "proj", dir, 3)
		if err != nil {
			t.Fatalf("snapshot %d: %v", i, err)
		}
		paths = append(paths, p)
		// Misma fecha de modificación en todos: decide el nombre.
		if err := os.Chtimes(p, fixed, fixed); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("esperaba 3 snapshots, hay %d", len(entries))
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("el más antiguo (%s) debía podarse: %v", filepath.Base(paths[0]), err)
	}
	for _, p := range paths[1:] {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s debía sobrevivir: %v", filepath.Base(p), err)
		}
	}
}
