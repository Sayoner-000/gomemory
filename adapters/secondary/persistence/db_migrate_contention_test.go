package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrate_UpToDateDBDoesNotWaitForOpenWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.db")
	db, err := openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	w, err := sql.Open("sqlite", sqliteDSN(path, 5000))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	tx, err := w.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT INTO memories(project,content) VALUES ('p','uncommitted')"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		opened, err := openDBFile(path)
		if err == nil {
			err = opened.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		_ = tx.Rollback()
		<-done
		t.Fatal("opening an up-to-date database waited for the open writer")
	}
}

func TestMigrate_NewerUserVersionIsNotDowngraded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.db")
	db, err := openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = " + fmt.Sprint(schemaVersion+5)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion+5 {
		t.Errorf("newer schema version changed: got %d, want %d", version, schemaVersion+5)
	}
}
