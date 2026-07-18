package persistence

import (
	"strings"
	"testing"

	"mem/domain"
)

// TestInsertMemory_RedactsKnownSecretPatterns cubre la Historia de Usuario 2
// (specs/009-mitigacion-riesgos): un secreto reconocible pegado FUERA de
// <private> también debe quedar redactado al guardar, no solo el que el
// usuario recordó envolver.
func TestInsertMemory_RedactsKnownSecretPatterns(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Decision,
		Title:   "clave de prueba",
		Content: "clave de ejemplo (no envuelta en private): AKIAIOSFODNN7EXAMPLE",
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
		t.Fatalf("esperaba 1 memoria con id %d, got %+v", id, mems)
	}
	if strings.Contains(mems[0].Content, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("secreto no redactado en el contenido persistido: %+v", mems[0])
	}
	if !strings.Contains(mems[0].Content, "[REDACTED:aws-key]") {
		t.Fatalf("esperaba placeholder [REDACTED:aws-key], got %q", mems[0].Content)
	}
}

// TestImportMemory_RedactsKnownSecretPatterns confirma que la misma segunda
// capa de redacción aplica también al restaurar un bundle (import), no solo
// al guardado normal.
func TestImportMemory_RedactsKnownSecretPatterns(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project: "proj",
		Type:    domain.Decision,
		Content: "token de github: ghp_1234567890abcdefghijklmnopqrstuvwxyzABCD",
	}
	id, err := ImportMemory(db, m)
	if err != nil {
		t.Fatalf("import memory: %v", err)
	}

	mems, _ := ListAllMemories(db, "proj")
	var found *domain.Memory
	for i := range mems {
		if mems[i].ID == id {
			found = &mems[i]
		}
	}
	if found == nil {
		t.Fatalf("memoria importada no encontrada: %+v", mems)
	}
	if strings.Contains(found.Content, "ghp_1234567890abcdefghijklmnopqrstuvwxyzABCD") {
		t.Fatalf("secreto no redactado en import: %+v", found)
	}
	if !strings.Contains(found.Content, "[REDACTED:github-token]") {
		t.Fatalf("esperaba placeholder [REDACTED:github-token], got %q", found.Content)
	}
}

// TestSetLastPrompt_RedactsKnownSecretPatterns confirma que el último prompt
// de sesión (usado como provenance heredada, ver InsertMemory) también pasa
// por la misma segunda capa de redacción.
func TestSetLastPrompt_RedactsKnownSecretPatterns(t *testing.T) {
	db := openTestDB(t)

	if _, err := StartSession(db, "proj"); err != nil {
		t.Fatalf("start session: %v", err)
	}
	prompt := "aquí está mi clave xoxb-FAKE-TEST-TOKEN-NOT-REAL para el bot"
	if err := SetSessionLastPrompt(db, "proj", prompt); err != nil {
		t.Fatalf("set last prompt: %v", err)
	}

	stored := activeSessionLastPrompt(db, "proj")
	if strings.Contains(stored, "xoxb-FAKE-TEST-TOKEN-NOT-REAL") {
		t.Fatalf("secreto no redactado en last_prompt: %q", stored)
	}
	if !strings.Contains(stored, "[REDACTED:slack-token]") {
		t.Fatalf("esperaba placeholder [REDACTED:slack-token], got %q", stored)
	}
}
