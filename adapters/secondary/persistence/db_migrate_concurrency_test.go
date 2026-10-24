package persistence

import (
	"fmt"
	"mem/domain"
	"path/filepath"
	"sync"
	"testing"
)

func TestMigrate_ConcurrentPoolsNewDBWithWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.db")
	start := make(chan struct{})
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			db, err := openDBFile(path)
			if err != nil {
				results <- err
				return
			}
			defer db.Close()
			_, err = InsertMemory(db, &domain.Memory{Project: "p", Type: domain.Learning, Title: fmt.Sprintf("memory %d", i), Content: fmt.Sprintf("distinct observation %d", i)})
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
	if t.Failed() {
		return
	}
	db, err := openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count, version int
	if err := db.QueryRow("SELECT COUNT(*) FROM memories WHERE project='p'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 8 {
		t.Errorf("expected 8 memories, got %d", count)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Errorf("schema version = %d, want %d", version, schemaVersion)
	}
}
