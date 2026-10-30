package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
)

type hcd001Clock struct{ now time.Time }

func (c *hcd001Clock) Now() time.Time { return c.now }

// TestHCD001ReferenceIsolation recorre el límite de recuperación con un
// almacén real: una ref solo resuelve dentro del proyecto que la guardó.
func TestHCD001ReferenceIsolation(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	clk := &hcd001Clock{now: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	store := persistence.NewOriginalStoreRepository(db, clk, 1, 0)
	ctx := context.Background()

	const shared = "contrato compartido á漢字 alfa-beta"
	const alphaOnly = "dato exclusivo de alfa ñandú"
	refShared := map[string]string{}
	for _, project := range []string{"alfa", "beta"} {
		ref, err := store.Put(ctx, project, shared, "context_item", "budget")
		if err != nil {
			t.Fatal(err)
		}
		refShared[project] = ref
	}
	refAlpha, err := store.Put(ctx, "alfa", alphaOnly, "context_item", "budget")
	if err != nil {
		t.Fatal(err)
	}

	for project, ref := range refShared {
		got, _, err := usecases.RetrieveOriginal(ctx, store, project, ref)
		if err != nil || got != shared {
			t.Fatalf("%s no recupera su propio original (ref %s): %v", project, ref, err)
		}
	}
	for name, ref := range map[string]string{"ajena": refAlpha, "inexistente": "ffffffffffff"} {
		got, _, err := usecases.RetrieveOriginal(ctx, store, "beta", ref)
		if !errors.Is(err, usecases.ErrOriginalNotFound) || got != "" || strings.Contains(err.Error(), "exclusivo") {
			t.Fatalf("ref %s debe rechazarse sin contenido ni datos: %q %v", name, got, err)
		}
	}

	clk.now = clk.now.Add(48 * time.Hour)
	if got, _, err := usecases.RetrieveOriginal(ctx, store, "alfa", refAlpha); !errors.Is(err, usecases.ErrOriginalNotFound) || got != "" {
		t.Fatalf("ref caducada debe rechazarse: %q %v", got, err)
	}
}
