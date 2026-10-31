package usecases_test

import (
	"context"
	"fmt"
	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// startUC crea un caso de uso propio por competidor: en producción cada
// `mem hcd start` es un proceso con su reloj y su generador de ids.
func startUC(t *testing.T, env hcdEnv) *usecases.HCD {
	t.Helper()
	tpl, err := os.ReadFile("../../infrastructure/templates/hcd/unit/unit.md")
	if err != nil {
		t.Fatal(err)
	}
	clock := &hcdTickClock{t: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}
	return usecases.NewHCD(persistence.NewHCDStore(env.db, "p"), hcdfiles.Workspace{Root: env.root}, clock, func() string { return "id" }, string(tpl))
}

func TestHCDStartConcurrente(t *testing.T) {
	env := newHCDEnv(t)
	const n = 8
	ucs := make([]*usecases.HCD, n)
	for i := range ucs {
		ucs[i] = startUC(t, env)
	}
	type result struct {
		id, doc string
		err     error
	}
	results := make([]result, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range ucs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			v, err := ucs[i].Start(context.Background(), usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "mismo titulo", Why: "repro", Intent: fmt.Sprintf("intent-%d-fin", i)})
			results[i] = result{id: v.Unit.ID, doc: v.Unit.DocPath, err: err}
		}(i)
	}
	close(start)
	wg.Wait()

	seen := map[string]bool{}
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("competidor %d: %v", i, r.err)
		}
		if seen[r.id] {
			t.Fatalf("id repetido: %s", r.id)
		}
		seen[r.id] = true
		doc, err := os.ReadFile(filepath.Join(env.root, r.doc))
		if err != nil {
			t.Fatal(err)
		}
		if want := fmt.Sprintf("intent-%d-fin", i); !strings.Contains(string(doc), want) {
			t.Fatalf("%s no contiene su intent %q", r.id, want)
		}
	}
	docs, err := filepath.Glob(filepath.Join(env.root, "hcd", "HCD-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != n {
		t.Fatalf("docs = %d, quiero %d", len(docs), n)
	}
}

func TestHCDStartFalloDeDoc(t *testing.T) {
	env := newHCDEnv(t)
	uc := startUC(t, env)
	blocker := filepath.Join(env.root, "hcd")
	if err := os.WriteFile(blocker, []byte("no soy un directorio"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "t", Why: "w", Intent: "i"}
	if _, err := uc.Start(context.Background(), in); err == nil {
		t.Fatal("Start debía fallar si no puede escribir el doc")
	}
	var units int
	if err := env.db.QueryRow("SELECT COUNT(*) FROM hcd_units").Scan(&units); err != nil {
		t.Fatal(err)
	}
	if units != 0 {
		t.Fatalf("quedaron %d unidades sin doc", units)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	v, err := uc.Start(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if v.Unit.ID != "HCD-001" {
		t.Fatalf("id = %s, quiero HCD-001", v.Unit.ID)
	}
}
