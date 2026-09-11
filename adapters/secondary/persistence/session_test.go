package persistence

import "testing"

// TestStartSession_DevuelveCreatedAt: la fecha de creación la genera la BD;
// StartSession debe devolverla en vez de una cadena vacía, igual que Active.
func TestStartSession_DevuelveCreatedAt(t *testing.T) {
	db := openTestDB(t)

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if sess.CreatedAt == "" {
		t.Fatal("CreatedAt vacío: StartSession no devuelve la fecha que generó la BD")
	}

	var guardada string
	if err := db.QueryRow(`SELECT created_at FROM sessions WHERE id = ?`, sess.ID).Scan(&guardada); err != nil {
		t.Fatalf("leer sesión: %v", err)
	}
	if sess.CreatedAt != guardada {
		t.Errorf("CreatedAt = %q, la BD guardó %q", sess.CreatedAt, guardada)
	}
}
