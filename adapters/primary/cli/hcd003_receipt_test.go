package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/domain"
)

// TestHCD003ReceiptOptIn fija que el recibo de procedencia y la contabilidad
// ampliada solo viajan cuando se piden (--receipt / receipt), y que en ambos
// casos la respuesta servida cabe en el presupuesto.
func TestHCD003ReceiptOptIn(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	repo := persistence.NewMemoryRepository(db)
	for _, m := range []domain.Memory{
		{Project: "own", Type: domain.Learning, Title: "limite", Content: strings.Repeat("limite á漢字 verificable ", 40)},
		{Project: "own", Type: domain.Discovery, Title: "limite secundario", Content: strings.Repeat("limite resultado diferente ", 30)},
	} {
		if _, err := repo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	counter := tokens.ApproximateTokenCounter{}
	deps := &Deps{Root: root, Project: "own", MemoryRepo: repo, Compressor: compression.StructuralCompressor{}, TokenCounter: counter, CompressionLevel: ports.CompressionNone}
	extras := []string{"Procedencia", "Receipts", "OriginalSHA256", "SelectionSavedTokens", "Ámbito:", "contador local"}

	for _, receipt := range []bool{false, true} {
		for _, asJSON := range []bool{false, true} {
			args := []string{"--task", "limite", "--max-tokens", "2000", "--no-speckit", "--no-code-graph", "--no-compress"}
			if receipt {
				args = append(args, "--receipt")
			}
			req, _, err := ParsePackBuildFlags(args, "own")
			if err != nil {
				t.Fatal(err)
			}
			out, err := buildPackOutput(deps, req, asJSON)
			if err != nil {
				t.Fatal(err)
			}
			if got := counter.Count(out); got > 2000 || !strings.Contains(out, "limite") {
				t.Fatalf("receipt=%v json=%v: %d tokens > 2000 o sin evidencia", receipt, asJSON, got)
			}
			found := false
			for _, e := range extras {
				found = found || strings.Contains(out, e)
			}
			if found != receipt {
				t.Fatalf("receipt=%v json=%v: recibo o contabilidad presentes=%v:\n%s", receipt, asJSON, found, out)
			}
		}
	}

	cs := connectMCPTestSession(t, deps, root, "own")
	for _, receipt := range []bool{false, true} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "pack_build", Arguments: map[string]any{"task": "limite", "max_tokens": 2000, "no_code_graph": true, "receipt": receipt}})
		if err != nil || res.IsError {
			t.Fatalf("pack_build: %v %q", err, toolText(res))
		}
		if got := strings.Contains(toolText(res), "Procedencia"); got != receipt {
			t.Fatalf("MCP receipt=%v: recibo presente=%v", receipt, got)
		}
	}
}
