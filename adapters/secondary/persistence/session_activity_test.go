package persistence

import (
	"testing"

	"mem/domain"
)

func TestLastSessionActivity_IncluyeCheckpoints(t *testing.T) {
	db := openTestDB(t)
	s, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	real := &domain.Memory{Project: "proj", SessionID: s.ID, Type: domain.Decision, Title: "decisión", Content: "contenido de una decisión real"}
	cp := &domain.Memory{Project: "proj", SessionID: s.ID, Type: domain.Checkpoint, Title: "Checkpoint automático", Content: "Editó: a.go"}
	idReal, err := InsertMemory(db, real)
	if err != nil {
		t.Fatalf("insert real: %v", err)
	}
	idCp, err := InsertMemory(db, cp)
	if err != nil {
		t.Fatalf("insert checkpoint: %v", err)
	}
	_, _ = db.Exec(`UPDATE memories SET created_at = '2026-01-01 10:00:00', updated_at = '2026-01-01 10:00:00' WHERE id = ?`, idReal)
	_, _ = db.Exec(`UPDATE memories SET created_at = '2026-01-01 12:00:00', updated_at = '2026-01-01 12:30:00' WHERE id = ?`, idCp)

	got, ok, err := LastSessionActivity(db, s.ID)
	if err != nil || !ok {
		t.Fatalf("LastSessionActivity: %v %v", ok, err)
	}
	if got != "2026-01-01 12:30:00" {
		t.Errorf("la actividad debe ser la última escritura, checkpoints incluidos: %q", got)
	}
}

func TestLastSessionActivity_SinMemoriasUsaElInicioDeLaSesion(t *testing.T) {
	db := openTestDB(t)
	s, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	got, ok, err := LastSessionActivity(db, s.ID)
	if err != nil || !ok || got != s.CreatedAt {
		t.Errorf("sin memorias debía devolver created_at=%q, got %q %v %v", s.CreatedAt, got, ok, err)
	}
}

func TestLastSessionActivity_SesionInexistente(t *testing.T) {
	db := openTestDB(t)
	if _, ok, err := LastSessionActivity(db, "no-existe"); ok || err != nil {
		t.Errorf("una sesión inexistente devuelve ok=false sin error: %v %v", ok, err)
	}
	r := &SessionRepository{db: db}
	if _, ok, err := r.LastActivity("no-existe"); ok || err != nil {
		t.Errorf("SessionRepository.LastActivity: %v %v", ok, err)
	}
}
