package persistence

import (
	"database/sql"
	"strings"
	"testing"

	"mem/domain"
)

// insertAndGet inserta una memoria y devuelve la versión persistida (con las
// columnas ya materializadas, p. ej. origin_prompt).
func insertAndGet(t *testing.T, db *sql.DB, m *domain.Memory) domain.Memory {
	t.Helper()
	id, err := InsertMemory(db, m)
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}
	mems, err := ListMemories(db, m.Project, 10)
	if err != nil {
		t.Fatalf("list memories: %v", err)
	}
	for _, got := range mems {
		if got.ID == id {
			return got
		}
	}
	t.Fatalf("memoria %d no encontrada tras insertar", id)
	return domain.Memory{}
}

func TestInsertMemoryBackfillsOriginPromptFromActiveSession(t *testing.T) {
	db := openTestDB(t)
	if _, err := StartSession(db, "proj"); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := SetSessionLastPrompt(db, "proj", "arregla el bug del login"); err != nil {
		t.Fatalf("set last prompt: %v", err)
	}

	got := insertAndGet(t, db, &domain.Memory{
		Project: "proj", Type: domain.Bugfix, Title: "fix login", Content: "causa raíz X",
	})
	if got.OriginPrompt != "arregla el bug del login" {
		t.Errorf("esperaba que el prompt de la sesión se adjuntara como provenance, got %q", got.OriginPrompt)
	}
}

func TestInsertMemoryKeepsExplicitOriginPrompt(t *testing.T) {
	db := openTestDB(t)
	StartSession(db, "proj")
	SetSessionLastPrompt(db, "proj", "prompt de la sesión")

	got := insertAndGet(t, db, &domain.Memory{
		Project: "proj", Type: domain.Learning, Title: "t", Content: "c",
		OriginPrompt: "prompt explícito del llamador",
	})
	if got.OriginPrompt != "prompt explícito del llamador" {
		t.Errorf("un origin_prompt explícito no debe pisarse con el de la sesión, got %q", got.OriginPrompt)
	}
}

func TestInsertMemoryNoActiveSessionLeavesOriginEmpty(t *testing.T) {
	db := openTestDB(t)
	got := insertAndGet(t, db, &domain.Memory{
		Project: "proj", Type: domain.Learning, Title: "t", Content: "c",
	})
	if got.OriginPrompt != "" {
		t.Errorf("sin sesión activa no debe haber provenance, got %q", got.OriginPrompt)
	}
}

func TestSetSessionLastPromptRedactsPrivate(t *testing.T) {
	db := openTestDB(t)
	StartSession(db, "proj")
	if err := SetSessionLastPrompt(db, "proj", "usa el token <private>sk-secreto</private> aquí"); err != nil {
		t.Fatalf("set last prompt: %v", err)
	}
	got := insertAndGet(t, db, &domain.Memory{
		Project: "proj", Type: domain.Learning, Title: "t", Content: "c",
	})
	if strings.Contains(got.OriginPrompt, "sk-secreto") || strings.Contains(got.OriginPrompt, "<private>") {
		t.Errorf("el prompt originante no debe filtrar contenido <private>, got %q", got.OriginPrompt)
	}
}
