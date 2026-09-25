package persistence

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// T023 — almacén de originales: exactitud, dedup, TTL y LRU.
func TestOriginalStore(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	store := NewOriginalStoreRepository(db, clk, 7, 0)

	orig := "línea 1\n\tlínea 2 con ⟦símbolos⟧ y \x00 bytes\n"
	ref, err := store.Put(ctx, "p", orig, "log", "log")
	if err != nil || len(ref) != 12 {
		t.Fatalf("Put: ref=%q err=%v", ref, err)
	}
	got, meta, ok, err := store.Get(ctx, ref)
	if err != nil || !ok || got != orig || meta.ContentType != "log" {
		t.Fatalf("Get debe devolver el original byte a byte: ok=%v err=%v", ok, err)
	}

	// Dedup: el mismo contenido no crea otra fila.
	ref2, _ := store.Put(ctx, "p", orig, "log", "log")
	if _, n, _ := store.Usage(ctx); ref2 != ref || n != 1 {
		t.Errorf("dedup: ref2=%q filas=%d", ref2, n)
	}

	// TTL desde el último acceso: a los 6 días se renueva con Get; a los 8 sin
	// uso, caduca.
	clk.now = clk.now.Add(6 * 24 * time.Hour)
	if _, _, ok, _ := store.Get(ctx, ref); !ok {
		t.Fatal("a los 6 días todavía debe existir")
	}
	clk.now = clk.now.Add(6 * 24 * time.Hour)
	if _, _, ok, _ := store.Get(ctx, ref); !ok {
		t.Fatal("Get renueva la caducidad: a los 6 días del último acceso debe existir")
	}
	clk.now = clk.now.Add(8 * 24 * time.Hour)
	if _, _, ok, _ := store.Get(ctx, ref); ok {
		t.Error("tras 8 días sin uso debe haber caducado")
	}
}

func TestOriginalStoreLRU(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	store := NewOriginalStoreRepository(db, clk, 7, 1) // tope 1 MB
	rng := rand.New(rand.NewSource(1))
	random := func() string { // incompresible: gzip no lo reduce
		b := make([]byte, 300<<10)
		rng.Read(b)
		return string(b)
	}
	var refs []string
	for i := 0; i < 3; i++ {
		ref, err := store.Put(ctx, "p", random(), "prose", "prose")
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
		clk.now = clk.now.Add(time.Minute)
	}
	// El primero se toca: el menos usado pasa a ser el segundo.
	if _, _, ok, _ := store.Get(ctx, refs[0]); !ok {
		t.Fatal("el primero aún cabe")
	}
	clk.now = clk.now.Add(time.Minute)
	if _, err := store.Put(ctx, "p", random(), "prose", "prose"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, _ := store.Get(ctx, refs[1]); ok {
		t.Error("LRU: el de acceso más antiguo debe haberse expulsado")
	}
	if _, _, ok, _ := store.Get(ctx, refs[0]); !ok {
		t.Error("LRU: el recién usado debe seguir")
	}
	if bytes, _, _ := store.Usage(ctx); bytes > 1<<20 {
		t.Errorf("tope superado: %d", bytes)
	}
}

func TestOriginalStoreRejectsOriginalLargerThanCapacity(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	store := NewOriginalStoreRepository(db, clk, 7, 1)

	rng := rand.New(rand.NewSource(2))
	b := make([]byte, 2<<20)
	if _, err := rng.Read(b); err != nil {
		t.Fatal(err)
	}
	if ref, err := store.Put(ctx, "p", string(b), "prose", "prose"); err == nil || ref != "" {
		t.Fatalf("un original que no cabe debe degradar sin ref: ref=%q err=%v", ref, err)
	}
	if bytes, rows, err := store.Usage(ctx); err != nil || bytes != 0 || rows != 0 {
		t.Fatalf("el rechazo no debe dejar filas: bytes=%d rows=%d err=%v", bytes, rows, err)
	}
}
