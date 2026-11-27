package persistence

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"mem/domain"
)

var errUnavailable = errors.New("proveedor no disponible (fake de test)")

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("cerrar base de prueba: %v", err)
		}
	})
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

// HCD-069: guardar con filepath no añade nada al contenido; la anotación de
// impacto se retiró porque CodeGraph no expone hotspots.
func TestInsertMemory_ConFilepathNoTocaElContenido(t *testing.T) {
	db := openTestDB(t)

	id, err := InsertMemory(db, &domain.Memory{
		Project:  "proj",
		Type:     domain.Bugfix,
		Content:  "contenido sin cambios",
		Filepath: "cualquier/archivo.go",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	mems, _ := ListMemories(db, "proj", 10)
	if len(mems) != 1 || mems[0].ID != id {
		t.Fatalf("esperaba 1 memoria con id %d", id)
	}
	if mems[0].Content != "contenido sin cambios" {
		t.Fatalf("el contenido no debería tocarse al guardar: %q", mems[0].Content)
	}
}

func TestGetMemoryByID(t *testing.T) {
	db := openTestDB(t)
	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Learning, Title: "t", Content: "c"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := GetMemoryByID(db, "proj", id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Content != "c" || got.Title != "t" {
		t.Fatalf("memoria inesperada: %+v", got)
	}
}

func TestGetMemoryByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := GetMemoryByID(db, "proj", 999)
	if err != nil {
		t.Fatalf("no debería error, solo nil: %v", err)
	}
	if got != nil {
		t.Fatalf("esperaba nil, hubo %+v", got)
	}
}

func TestGetMemoryByID_WrongProject(t *testing.T) {
	db := openTestDB(t)
	id, _ := InsertMemory(db, &domain.Memory{Project: "proj-a", Type: domain.Learning, Content: "c"})
	got, err := GetMemoryByID(db, "proj-b", id)
	if err != nil {
		t.Fatalf("no debería error: %v", err)
	}
	if got != nil {
		t.Fatalf("no debería devolver una memoria de otro proyecto: %+v", got)
	}
}

func TestUpdateMemoryContent(t *testing.T) {
	db := openTestDB(t)
	id, err := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Title: "original", Content: "v1"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := UpdateMemoryContent(db, "proj", id, "actualizado", "v2"); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := GetMemoryByID(db, "proj", id)
	if err != nil || got == nil {
		t.Fatalf("get after update: %v / %v", got, err)
	}
	if got.Title != "actualizado" || got.Content != "v2" {
		t.Fatalf("no se actualizó: %+v", got)
	}
}

func TestUpdateMemoryContent_RedactsSecrets(t *testing.T) {
	db := openTestDB(t)
	id, _ := InsertMemory(db, &domain.Memory{Project: "proj", Type: domain.Architecture, Content: "v1"})

	if err := UpdateMemoryContent(db, "proj", id, "t", "token <private>sk-abc123</private>"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := GetMemoryByID(db, "proj", id)
	if strings.Contains(got.Content, "sk-abc123") || strings.Contains(got.Content, "<private>") {
		t.Fatalf("no se redactó: %q", got.Content)
	}
}

func TestUpdateMemoryContent_NotFound(t *testing.T) {
	db := openTestDB(t)
	if err := UpdateMemoryContent(db, "proj", 999, "t", "c"); err == nil {
		t.Fatal("esperaba error al actualizar un id inexistente")
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
