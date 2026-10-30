package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression/native"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// hcd001RawSource simula una fuente que no pasa por Insert: devuelve memorias
// con secretos sin redactar.
type hcd001RawSource struct {
	ports.MemoryRepository
	raw []domain.Memory
}

func (s hcd001RawSource) Search(string, string, int) ([]domain.Memory, error) { return s.raw, nil }

type hcd001JSON struct{}

func (hcd001JSON) Render(p domain.ContextPack) string {
	b, _ := json.MarshalIndent(p, "", "  ")
	return string(b) + "\n"
}

// TestHCD001PrivateDelivery recorre entrega y recuperación con secretos
// sintéticos: ni la salida, ni los recibos, ni los originales recuperables los
// conservan, y ninguna ref los expone (C-09).
func TestHCD001PrivateDelivery(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store := persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	engine := native.NewEngine(store, nil, nil, "p")

	var unmarked, marked, plain strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&unmarked, "2026-10-29 10:00:%02d INFO privado lote %d usa %s contra la API\n", i%60, i, secretoFalso)
		fmt.Fprintf(&marked, "privado: el puente %d del molino guarda <private>clave interna %s</private> en violeta\n", i*7, secretoFalso)
		fmt.Fprintf(&plain, "privado control: tejado naranja %d sin datos sensibles über ñandú\n", i*13)
	}
	source := hcd001RawSource{MemoryRepository: persistence.NewMemoryRepository(db), raw: []domain.Memory{
		{ID: 1, Project: "p", Type: domain.Learning, Title: "privado sin marca", Content: unmarked.String()},
		{ID: 2, Project: "p", Type: domain.Learning, Title: "privado marcado " + secretoFalso, Content: marked.String()},
		{ID: 3, Project: "p", Type: domain.Learning, Title: "privado control", Content: plain.String()},
		{ID: 4, Project: "p", Type: domain.Learning, Title: "privado corto", Content: "usa " + secretoFalso + " y <private>clave interna</private>"},
	}}

	pack, err := usecases.BuildContextPack(source, engine, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{
		Task: "privado", Project: "p", MaxTokens: 3000, Compression: ports.CompressionMax,
		Renderer: hcd001JSON{}, Originals: store, Receipt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	leaks := func(where, s string) {
		t.Helper()
		if strings.Contains(s, secretoFalso) || strings.Contains(s, "clave interna") || strings.Contains(s, "<private>") {
			t.Errorf("%s expone contenido sensible:\n%.300s", where, s)
		}
	}
	leaks("la salida", hcd001JSON{}.Render(pack))
	receipts, _ := json.Marshal(pack.Receipts)
	leaks("los recibos", string(receipts))

	paths := map[string]bool{}
	for _, r := range pack.Receipts {
		for _, ref := range r.Refs {
			paths[r.Status] = true
			got, _, err := usecases.RetrieveOriginal(context.Background(), store, "p", ref)
			if err != nil {
				t.Fatalf("ref %s de %s no recuperable: %v", ref, r.ID, err)
			}
			leaks("la recuperación de "+ref, got)
		}
	}
	originals := originalesGuardados(t, db, store)
	if !paths["selected"] || !paths["omitted"] || len(originals) == 0 {
		t.Fatalf("control: se esperaban refs de compresión y de omisión por presupuesto (%v, originales=%d)", paths, len(originals))
	}
	for _, o := range originals {
		leaks("un original guardado", o)
	}
}
