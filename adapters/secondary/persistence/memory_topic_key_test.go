package persistence

import (
	"testing"

	"mem/domain"
)

// S-004 (revisión 031): el dedup busca la topic_key recortada, así que también
// debe guardarse recortada. Si no, una clave con espacios laterales inserta un
// duplicado en cada guardado.
func TestInsert_TopicKeyConEspaciosNoDuplica(t *testing.T) {
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = db.Close() }()

	first, err := InsertMemory(db, &domain.Memory{Project: "p", Type: domain.Decision, Title: "uno", Content: "a", TopicKey: " clave "})
	if err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	second, err := InsertMemory(db, &domain.Memory{Project: "p", Type: domain.Decision, Title: "dos", Content: "b", TopicKey: " clave "})
	if err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	third, err := InsertMemory(db, &domain.Memory{Project: "p", Type: domain.Decision, Title: "tres", Content: "c", TopicKey: "clave"})
	if err != nil {
		t.Fatalf("insert 3: %v", err)
	}
	if first != second || second != third {
		t.Fatalf("la misma clave con o sin espacios debe actualizar la misma memoria: %d, %d, %d", first, second, third)
	}
	var stored string
	if err := db.QueryRow(`SELECT topic_key FROM memories WHERE id = ?`, first).Scan(&stored); err != nil {
		t.Fatalf("select: %v", err)
	}
	if stored != "clave" {
		t.Fatalf("topic_key guardada = %q, want \"clave\"", stored)
	}
}
