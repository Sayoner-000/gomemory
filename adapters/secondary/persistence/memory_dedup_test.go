package persistence_test

import (
	"fmt"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func TestInsertMemory_DedupPorIdentidad(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()
	repo := persistence.NewMemoryRepository(db)

	// 3 memorias equivalentes (mismo proyecto+tipo+título) ⇒ 1 fila consolidada.
	for i := 0; i < 3; i++ {
		if _, err := repo.Insert(&domain.Memory{Project: "p", Type: domain.Decision, Title: "misma decisión", Content: fmt.Sprintf("versión %d", i)}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	mems, _ := repo.List("p", 100)
	count := 0
	for _, m := range mems {
		if m.Title == "misma decisión" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("esperaba 1 fila consolidada por identidad, got %d", count)
	}

	// Los checkpoints NO se deduplican (su contenido varía por turno).
	for i := 0; i < 2; i++ {
		repo.Insert(&domain.Memory{Project: "p", Type: domain.Checkpoint, Title: "chk", Content: fmt.Sprintf("actividad %d", i)})
	}
	mems, _ = repo.List("p", 100)
	cc := 0
	for _, m := range mems {
		if m.Type == domain.Checkpoint {
			cc++
		}
	}
	if cc != 2 {
		t.Fatalf("los checkpoints no deben deduplicarse, got %d", cc)
	}
}

func TestInsertMemory_UpsertPorTopicKey(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()
	repo := persistence.NewMemoryRepository(db)

	id1, _ := repo.Insert(&domain.Memory{Project: "p", Type: domain.Decision, Title: "t1", Content: "primera", TopicKey: "arq-cache"})
	id2, _ := repo.Insert(&domain.Memory{Project: "p", Type: domain.Learning, Title: "t2", Content: "segunda", TopicKey: "arq-cache"})

	if id1 != id2 {
		t.Fatalf("mismo topic_key debe actualizar la misma fila (%d != %d)", id1, id2)
	}
	mems, _ := repo.List("p", 100)
	if len(mems) != 1 {
		t.Fatalf("esperaba 1 fila por topic_key, got %d", len(mems))
	}
	if mems[0].Content != "segunda" {
		t.Fatalf("el upsert debe actualizar el contenido, got %q", mems[0].Content)
	}
}
