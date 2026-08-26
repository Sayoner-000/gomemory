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

	// Checkpoints de contenido DISTINTO siguen siendo filas distintas: la clave
	// de identidad de un checkpoint es su contenido, y dos turnos distintos no
	// deben fundirse jamás.
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
		t.Fatalf("dos turnos con actividad distinta deben ser dos filas, got %d", cc)
	}
}

// TestInsertMemory_DedupCheckpointPorContenido cubre la causa raíz medida en un
// proyecto real: la actividad de un mismo turno se registraba dos veces —una por
// el agente principal, otra por el subagente— con títulos distintos y cuerpo
// idéntico. Resultado: 178 filas redundantes, 265 KB, y 106 de 120 grupos
// duplicados que diferían SOLO en el título.
func TestInsertMemory_DedupCheckpointPorContenido(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer db.Close()
	repo := persistence.NewMemoryRepository(db)

	actividad := "Editó: main.go. Comandos: go build ./..."
	id1, err := repo.Insert(&domain.Memory{Project: "p", Type: domain.Checkpoint, Title: "Checkpoint automático", Content: actividad})
	if err != nil {
		t.Fatalf("insert principal: %v", err)
	}
	id2, err := repo.Insert(&domain.Memory{Project: "p", Type: domain.Checkpoint, Title: "Checkpoint de subagente", Content: actividad})
	if err != nil {
		t.Fatalf("insert subagente: %v", err)
	}

	if id1 != id2 {
		t.Fatalf("el mismo turno visto dos veces debe consolidar en una fila (%d != %d)", id1, id2)
	}

	mems, _ := repo.List("p", 100)
	cc := 0
	for _, m := range mems {
		if m.Type == domain.Checkpoint {
			cc++
		}
	}
	if cc != 1 {
		t.Fatalf("esperaba 1 checkpoint tras deduplicar por contenido, got %d", cc)
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
