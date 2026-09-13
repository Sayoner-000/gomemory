package persistence_test

import (
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func TestInsert_MismoTituloTipoSesion_DevuelveMismoID(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := persistence.NewMemoryRepository(db)

	first, err := repo.Insert(&domain.Memory{Project: "p", Type: domain.Learning, Title: "misma", Content: "uno", SessionID: "s1"})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	second, err := repo.Insert(&domain.Memory{Project: "p", Type: domain.Learning, Title: "misma", Content: "dos", SessionID: "s1"})
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if first != second {
		t.Fatalf("ids differ: %d != %d", first, second)
	}
	mems, err := repo.ListAll("p")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(mems) != 1 {
		t.Fatalf("expected one row, got %d", len(mems))
	}
}
