package persistence

import (
	"database/sql"
	"strings"
	"testing"

	"mem/domain"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInsertMemory_RedactsPrivateBlocks(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Learning,
		Title:   "token <private>sk-abc123</private> guardado",
		Content: "usa la key <private>sk-abc123</private> para autenticar",
	}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	mems, err := ListMemories(db, "proj", 10)
	if err != nil {
		t.Fatalf("list memories: %v", err)
	}
	if len(mems) != 1 || mems[0].ID != id {
		t.Fatalf("expected 1 memory with id %d, got %+v", id, mems)
	}
	if strings.Contains(mems[0].Title, "sk-abc123") || strings.Contains(mems[0].Content, "sk-abc123") {
		t.Fatalf("private content leaked into stored memory: %+v", mems[0])
	}
	if strings.Contains(mems[0].Title, "<private>") || strings.Contains(mems[0].Content, "<private>") {
		t.Fatalf("private tags not stripped: %+v", mems[0])
	}
}

func TestInsertMemory_EmptyAfterRedactionFails(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Learning,
		Content: "<private>todo el contenido es secreto</private>",
	}
	if _, err := InsertMemory(db, m); err == nil {
		t.Fatal("expected error when content is empty after redaction, got nil")
	}
}

func TestInsertMemory_NoPrivateBlocksUnaffected(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Decision,
		Title:   "decisión normal",
		Content: "contenido sin secretos",
	}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 1 || mems[0].ID != id || mems[0].Content != "contenido sin secretos" {
		t.Fatalf("unexpected content: %+v", mems)
	}
}

func TestDeleteMemory(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{Project: "proj", Type: domain.Learning, Content: "borrar esto"}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	deleted, err := DeleteMemory(db, "proj", id)
	if err != nil {
		t.Fatalf("delete memory: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 0 {
		t.Fatalf("expected memory to be gone, got %+v", mems)
	}

	deletedAgain, err := DeleteMemory(db, "proj", id)
	if err != nil {
		t.Fatalf("delete memory again: %v", err)
	}
	if deletedAgain {
		t.Fatal("expected deleted=false on second delete")
	}
}

func TestDeleteMemory_ScopedToProject(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{Project: "proj-a", Type: domain.Learning, Content: "de proj-a"}
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	deleted, err := DeleteMemory(db, "proj-b", id)
	if err != nil {
		t.Fatalf("delete memory: %v", err)
	}
	if deleted {
		t.Fatal("delete should not cross project scope")
	}

	mems, _ := ListMemories(db, "proj-a", 10)
	if len(mems) != 1 {
		t.Fatalf("memory should still exist in proj-a, got %+v", mems)
	}
}
