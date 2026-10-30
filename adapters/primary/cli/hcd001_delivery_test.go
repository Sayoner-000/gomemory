package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/domain"
)

// TestHCD001OwnDelivery mide la respuesta final de `mem pack build` (texto y
// JSON) y del texto MCP de pack_build contra el presupuesto pedido (Q3).
func TestHCD001OwnDelivery(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	repo := persistence.NewMemoryRepository(db)
	for _, m := range []domain.Memory{
		{Project: "own", Type: domain.Learning, Title: "limite", Content: strings.Repeat("limite á漢字 verificable ", 70)},
		{Project: "own", Type: domain.Discovery, Title: "limite secundario", Content: strings.Repeat("limite resultado diferente ", 60)},
		{Project: "own", Type: domain.Decision, Title: "regla", Content: strings.Repeat("regla obligatoria sin truncar ", 40)},
	} {
		if _, err := repo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	counter := tokens.ApproximateTokenCounter{}
	deps := &Deps{Root: root, Project: "own", MemoryRepo: repo, Compressor: compression.StructuralCompressor{}, TokenCounter: counter, CompressionLevel: ports.CompressionNone}
	const budget = 1100

	for _, asJSON := range []bool{false, true} {
		req, _, err := ParsePackBuildFlags([]string{"--task", "limite", "--max-tokens", "1100", "--no-speckit", "--no-code-graph", "--no-compress"}, "own")
		if err != nil {
			t.Fatal(err)
		}
		out, err := buildPackOutput(deps, req, asJSON)
		if err != nil {
			t.Fatalf("json=%v: %v", asJSON, err)
		}
		if got := counter.Count(out); got > budget || !strings.Contains(out, "limite") {
			t.Fatalf("json=%v: respuesta completa=%d > presupuesto=%d o sin evidencia", asJSON, got, budget)
		}

		req.Task = "zzsincoincidencia1zz"
		out, err = buildPackOutput(deps, req, asJSON)
		if err != nil || !strings.Contains(out, "no_match") && !strings.Contains(out, "Sin coincidencias") || strings.Contains(out, "verificable") {
			t.Fatalf("json=%v: no-match no explícito o con relleno: %v %q", asJSON, err, out)
		}

		req.Task, req.MaxTokens = "regla", 60
		if _, err := buildPackOutput(deps, req, asJSON); !errors.Is(err, domain.ErrCriticalContextOverflow) {
			t.Fatalf("json=%v: desbordamiento obligatorio silencioso: %v", asJSON, err)
		}
	}

	cs := connectMCPTestSession(t, deps, root, "own")
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "pack_build", Arguments: map[string]any{"task": "limite", "max_tokens": budget, "no_code_graph": true}})
	if err != nil || res.IsError {
		t.Fatalf("pack_build: %v %q", err, toolText(res))
	}
	if got := counter.Count(toolText(res)); got > budget {
		t.Fatalf("MCP respuesta completa=%d > presupuesto=%d", got, budget)
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "pack_build", Arguments: map[string]any{"task": "regla", "max_tokens": 60, "no_code_graph": true}})
	if err != nil || !res.IsError || !strings.Contains(toolText(res), "excede el presupuesto") {
		t.Fatalf("MCP desbordamiento no explícito: %v %q", err, toolText(res))
	}
}
