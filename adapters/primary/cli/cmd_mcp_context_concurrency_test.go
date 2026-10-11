package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/adapters/secondary/usage"
	"mem/application/usecases"
	"mem/domain"
)

// TestMCPGetContext_OchoSolapadasSinMezcla cubre US3.2 (spec 037, FR-010)
// con los puertos reales del composition root: un Builder compartido por el
// servidor MCP, el contador de tokens real y el recorder real sobre SQLite.
// Cada emisión registrada debe ser igual a la de una construcción aislada;
// se compara cada fila, nunca el total agregado de uso. Ejecutar con -race.
func TestMCPGetContext_OchoSolapadasSinMezcla(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	memRepo := persistence.NewMemoryRepository(db)
	for i := 0; i < 40; i++ {
		if _, err := memRepo.ImportMemory(&domain.Memory{Project: "proj", Type: domain.Decision,
			Title:     fmt.Sprintf("decisión %d", i),
			Content:   strings.Repeat(fmt.Sprintf("contenido %d ", i), 30),
			CreatedAt: "2026-10-01 00:00:00", UpdatedAt: "2026-10-01 00:00:00"}); err != nil {
			t.Fatal(err)
		}
	}
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)
	newBuilder := func(rec interface{ Record(string, int, int) }) *usecases.Builder {
		b := usecases.New(memRepo, sessRepo, relRepo, root, "proj")
		b.Budget = 3000
		b.Counter = tokens.ApproximateTokenCounter{}
		b.Recorder = rec
		return b
	}

	usageRepo := persistence.NewUsageRepository(db)
	isolated := usage.NewRecorder(usageRepo, "aislado", "test", nil)
	if _, err := newBuilder(isolated).Build(); err != nil {
		t.Fatal(err)
	}
	var wantBase, wantEmit int
	if err := db.QueryRow(`SELECT baseline_tokens, emitted_tokens FROM usage_records WHERE project = 'aislado'`).Scan(&wantBase, &wantEmit); err != nil {
		t.Fatalf("emisión aislada: %v", err)
	}
	if wantBase <= wantEmit {
		t.Fatalf("el caso debe tener descartes: base=%d emitido=%d", wantBase, wantEmit)
	}

	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo: memRepo, SessionRepo: sessRepo, RelationRepo: relRepo,
		ContextBuilder: newBuilder(usage.NewRecorder(usageRepo, "proj", "mcp", nil)),
	}
	cs := connectMCPTestSession(t, deps, root, "proj")

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_context", Arguments: map[string]any{"full": true}})
			if err == nil && res.IsError {
				err = fmt.Errorf("tool error: %s", toolText(res))
			}
			errs[i] = err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("llamada %d: %v", i, err)
		}
	}

	rows, err := db.Query(`SELECT baseline_tokens, emitted_tokens FROM usage_records WHERE project = 'proj' AND operation = ?`, domain.OpBuildContext)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		var base, emit int
		if err := rows.Scan(&base, &emit); err != nil {
			t.Fatal(err)
		}
		n++
		if base != wantBase || emit != wantEmit {
			t.Fatalf("emisión %d mezclada: base=%d emitido=%d, aislada base=%d emitido=%d", n, base, emit, wantBase, wantEmit)
		}
	}
	if n != 8 {
		t.Fatalf("emisiones registradas = %d, quiero 8", n)
	}
}
