package persistence

import (
	"context"
	"testing"
	"time"
)

// T047 — el registro de bloques está acotado a la sesión activa.
func TestDeliveredBlocks(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	repo := NewDeliveredBlocksRepository(db, "p", clk)

	// Sin sesión activa: inerte.
	if err := repo.Mark(ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if seen, _ := repo.Seen(ctx, []string{"a"}); seen["a"] {
		t.Fatal("sin sesión activa nada cuenta como entregado")
	}

	sess := NewSessionRepository(db)
	s1, err := sess.Start("p")
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.Mark(ctx, []string{"a", "b"})
	seen, _ := repo.Seen(ctx, []string{"a", "b", "c"})
	if !seen["a"] || !seen["b"] || seen["c"] {
		t.Fatalf("Seen inesperado: %v", seen)
	}

	// Otra sesión no hereda lo entregado.
	_ = sess.End(s1.ID, "")
	if _, err := sess.Start("p"); err != nil {
		t.Fatal(err)
	}
	if seen, _ := repo.Seen(ctx, []string{"a"}); seen["a"] {
		t.Error("una sesión nueva no debe heredar lo entregado en otra")
	}
	_ = repo.Mark(ctx, []string{"z"})

	// Reset borra solo la sesión activa.
	if err := repo.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if seen, _ := repo.Seen(ctx, []string{"z"}); seen["z"] {
		t.Error("Reset debe olvidar lo entregado en la sesión activa")
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM delivered_blocks WHERE session_id = ?`, s1.ID).Scan(&n)
	if n != 2 {
		t.Errorf("Reset no debe tocar otras sesiones: quedan %d de 2", n)
	}
}
