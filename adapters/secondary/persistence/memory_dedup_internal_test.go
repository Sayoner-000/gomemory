package persistence

import (
	"testing"

	"mem/domain"
)

func TestFindDuplicateTx_ErrorDeConsulta_NoSeTrataComoSinDuplicado(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("ALTER TABLE memories RENAME TO memories_x"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	_, _, err = findDuplicateTx(tx, &domain.Memory{Project: "p", Type: domain.Learning, Title: "t"}, "t", "c")
	if err == nil {
		t.Fatal("expected dedup lookup error")
	}
}
