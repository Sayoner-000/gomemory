package server

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNew(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := New(db, "test", 0)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}
