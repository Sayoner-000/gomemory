package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// TestHCD002FullAndCompaction fija C-03 con la palanca de Q4 activa: el
// arranque (hook), MCP get_context y `mem context` entregan las entradas con
// el extracto corto; `full=true` y el reinicio tras compactar devuelven la
// evidencia completa, sin delta, aunque la sesión ya la hubiera recibido.
func TestHCD002FullAndCompaction(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	repo := persistence.NewMemoryRepository(db)
	for i := 0; i < 8; i++ {
		content := fmt.Sprintf("DECISION-%d ", i) + strings.Repeat(fmt.Sprintf("razón %d del contrato ", i), 30)
		if _, err := repo.Insert(&domain.Memory{Project: "own", Type: domain.Decision, Title: fmt.Sprintf("decisión %d", i), Content: content}); err != nil {
			t.Fatal(err)
		}
	}
	sessions := persistence.NewSessionRepository(db)
	if _, err := sessions.Start("own"); err != nil {
		t.Fatal(err)
	}
	builder := usecases.New(repo, sessions, persistence.NewRelationRepository(db), root, "own")
	builder.Budget = 24000
	deps := &Deps{
		Root: root, Project: "own", MemoryRepo: repo, SessionRepo: sessions, ContextBuilder: builder,
		Compressor: compression.StructuralCompressor{}, TokenCounter: tokens.ApproximateTokenCounter{},
		CompressionLevel: ports.CompressionMax,
		DeliveredBlocks:  persistence.NewDeliveredBlocksRepository(db, "own", clock.SystemClock{}),
	}
	complete := func(who, out string) {
		t.Helper()
		if strings.Contains(out, domain.DeliveredTag) || strings.Count(out, "→ `get_memory") < 8 {
			t.Fatalf("%s debe entregar las 8 entradas completas, sin delta:\n%s", who, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "- **decisión") && strings.Contains(line, strings.Repeat("razón", 4)) {
				t.Fatalf("%s entrega un extracto largo, no el de 80 caracteres: %q", who, line)
			}
		}
	}

	complete("arranque (hook)", entregaContextoDeArranque(deps))
	cs := connectMCPTestSession(t, deps, root, "own")
	getContext := func(full bool) string {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_context", Arguments: map[string]any{"full": full}})
		if err != nil || res.IsError {
			t.Fatalf("get_context: %v %q", err, toolText(res))
		}
		return toolText(res)
	}
	if out := getContext(false); !strings.Contains(out, domain.DeliveredTag) {
		t.Fatalf("control: tras el arranque, get_context debía aplicar el delta: %q", out)
	}
	complete("MCP get_context(full=true)", getContext(true))
	raw, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	complete("mem context --full", deliverContextDocFull(deps, raw, true))

	resetDeliveries(deps) // compactación
	complete("MCP get_context tras compactar", getContext(false))
	resetDeliveries(deps)
	complete("mem context tras compactar", deliverContextDocFull(deps, raw, false))
}
