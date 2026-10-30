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

// TestHCD001FullDelivery recorre un destinatario secundario (full=true) y el
// principal en la misma sesión real: full=true no aplica delta ni marca como
// leído lo que el principal todavía no recibió (C-08).
func TestHCD001FullDelivery(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	const literal = "regla de entrega completa á漢字 RS256"
	repo := persistence.NewMemoryRepository(db)
	m := domain.Memory{Project: "own", Type: domain.Learning, Title: "entrega", Content: literal}
	if _, err := repo.Insert(&m); err != nil {
		t.Fatal(err)
	}
	sessions := persistence.NewSessionRepository(db)
	if _, err := sessions.Start("own"); err != nil {
		t.Fatal(err)
	}
	doc := "# Memoria del Proyecto\n\n## Decisiones Técnicas\n\n- **Regla**: " + literal + "\n"
	deps := &Deps{
		Root: root, Project: "own", MemoryRepo: repo, SessionRepo: sessions,
		ContextBuilder: contextBuilderFalso{salida: doc},
		Compressor:     compression.StructuralCompressor{}, TokenCounter: tokens.ApproximateTokenCounter{},
		CompressionLevel: ports.CompressionMax,
		DeliveredBlocks:  persistence.NewDeliveredBlocksRepository(db, "own", clock.SystemClock{}),
	}
	cs := connectMCPTestSession(t, deps, root, "own")
	getContext := func(full bool) string {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_context", Arguments: map[string]any{"full": full}})
		if err != nil || res.IsError {
			t.Fatalf("get_context: %v %q", err, toolText(res))
		}
		return toolText(res)
	}
	complete := func(who, out string) {
		t.Helper()
		if !strings.Contains(out, literal) || strings.Contains(out, domain.DeliveredTag) {
			t.Fatalf("%s debe recibir la evidencia completa: %q", who, out)
		}
	}

	complete("secundario (full=true)", getContext(true))
	complete("principal tras el secundario", getContext(false))
	if out := getContext(false); !strings.Contains(out, domain.DeliveredTag) {
		t.Fatalf("control: el delta del principal debía estar activo: %q", out)
	}
	complete("secundario tras la lectura del principal", getContext(true))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "pack_build", Arguments: map[string]any{"task": "entrega", "max_tokens": 4000, "no_code_graph": true}})
	if err != nil || res.IsError || !strings.Contains(toolText(res), literal) {
		t.Fatalf("pack_build debe entregar la evidencia sin delta: %v %q", err, toolText(res))
	}
}
