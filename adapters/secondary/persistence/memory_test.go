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

func TestSecondsSinceLastSave(t *testing.T) {
	db := openTestDB(t)

	// Proyecto sin memorias: no hay guardado real.
	if _, exists, err := SecondsSinceLastSave(db, "proj"); err != nil || exists {
		t.Fatalf("proyecto vacío debía dar exists=false, got exists=%v err=%v", exists, err)
	}

	// Solo un checkpoint reciente: no cuenta como guardado real.
	if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Checkpoint, Content: "actividad"}); err != nil {
		t.Fatalf("insert checkpoint: %v", err)
	}
	if _, exists, err := SecondsSinceLastSave(db, "proj"); err != nil || exists {
		t.Fatalf("solo checkpoint debía dar exists=false, got exists=%v err=%v", exists, err)
	}

	// Un guardado real retrasado 20 min: existe y el reloj lo refleja.
	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Decision, Content: "elegimos X"})
	if err != nil {
		t.Fatalf("insert decision: %v", err)
	}
	if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now','-5 hours','-20 minutes') WHERE id = ?`, id); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	secs, exists, err := SecondsSinceLastSave(db, "proj")
	if err != nil || !exists {
		t.Fatalf("con guardado real debía dar exists=true, got exists=%v err=%v", exists, err)
	}
	if secs < 900 {
		t.Fatalf("esperaba > 900s desde el guardado real, got %d", secs)
	}

	// Un checkpoint reciente NO debe reiniciar el reloj del guardado real.
	if _, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Checkpoint, Content: "más actividad"}); err != nil {
		t.Fatalf("insert checkpoint 2: %v", err)
	}
	secs2, exists2, err := SecondsSinceLastSave(db, "proj")
	if err != nil || !exists2 {
		t.Fatalf("checkpoint reciente no debía borrar el guardado real, got exists=%v err=%v", exists2, err)
	}
	if secs2 < 900 {
		t.Fatalf("el checkpoint reciente reinició el reloj indebidamente: %d", secs2)
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
