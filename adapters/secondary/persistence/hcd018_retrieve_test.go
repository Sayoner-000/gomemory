package persistence

import (
	"context"
	"testing"
	"time"
)

// Con la escritura retenida por otra conexión, leer funciona y renovar no:
// el original ya leído debe devolverse y su caducidad queda como estaba.
func TestHCD018GetReturnsOriginalWhenRenewalFails(t *testing.T) {
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)}
	store := NewOriginalStoreRepository(db, clk, 7, 0)
	ref, err := store.Put(t.Context(), "p", "original", "log", "log")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	content, meta, found, err := store.Get(ctx, ref)
	if err != nil || !found || content != "original" || meta.Ref != ref {
		t.Fatalf("Get: %q %+v %v %v", content, meta, found, err)
	}
}
