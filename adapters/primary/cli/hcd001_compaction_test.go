package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/domain"
)

// TestHCD001CompactionReset fija que la composición presupuestada no depende
// del delta de sesión: entrega la evidencia aunque search_memories ya la haya
// marcado, la vuelve a entregar tras el reinicio y no marca lectura que
// suprima otra entrega de la misma u otra sesión (C-08).
func TestHCD001CompactionReset(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	const literal = "compactacion reinicia entregas RS256"
	repo := persistence.NewMemoryRepository(db)
	m := domain.Memory{Project: "own", Type: domain.Learning, Title: "compactacion", Content: literal}
	if _, err := repo.Insert(&m); err != nil {
		t.Fatal(err)
	}
	sessions := persistence.NewSessionRepository(db)
	deps := &Deps{
		Root: root, Project: "own", MemoryRepo: repo, SessionRepo: sessions,
		Compressor: compression.StructuralCompressor{}, TokenCounter: tokens.ApproximateTokenCounter{},
		CompressionLevel: ports.CompressionMax,
		DeliveredBlocks:  persistence.NewDeliveredBlocksRepository(db, "own", clock.SystemClock{}),
	}
	cs := connectMCPTestSession(t, deps, root, "own")
	call := func(name string, args map[string]any) string {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %q", name, err, toolText(res))
		}
		return toolText(res)
	}
	pack := func(step string) {
		t.Helper()
		if out := call("pack_build", map[string]any{"task": "compactacion", "max_tokens": 4000, "no_code_graph": true}); !strings.Contains(out, literal) {
			t.Fatalf("%s: el pack no entrega la evidencia necesaria: %q", step, out)
		}
	}

	for _, step := range []string{"sesión 1", "sesión 2"} {
		sess, err := sessions.Start("own")
		if err != nil {
			t.Fatal(err)
		}
		pack(step + ": antes de buscar")
		if out := call("search_memories", map[string]any{"query": "compactacion"}); !strings.Contains(out, literal) {
			t.Fatalf("%s: pack_build marcó como leída evidencia que la búsqueda no entregó: %q", step, out)
		}
		if out := call("search_memories", map[string]any{"query": "compactacion"}); !strings.Contains(out, domain.DeliveredTag) {
			t.Fatalf("%s: el delta de la búsqueda debía estar activo: %q", step, out)
		}
		pack(step + ": tras el delta de la búsqueda")
		resetDeliveries(deps)
		pack(step + ": tras compactar")
		if err := sessions.End(sess.ID, ""); err != nil {
			t.Fatal(err)
		}
	}
}
