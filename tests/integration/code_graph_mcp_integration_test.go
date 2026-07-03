package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCodeGraphMCPIntegration indexa un proyecto fixture y consulta el grafo
// de código a través del servidor MCP real (stdio), como lo haría un agente
// conectado — no llama a los usecases directamente. Cubre el flujo completo:
// index_project → graph_status → search_code → get_symbol → list_dependencies.
func TestCodeGraphMCPIntegration(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	writeFixtureFile(t, target, "pkg/lib.go", `package pkg

func Helper() int {
	return 1
}

func UseHelper() int {
	return Helper()
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.Command(bin, "mcp", "--root", target)
	transport := &mcp.CommandTransport{Command: cmd}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to mcp server: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	for _, want := range []string{"index_project", "graph_status", "search_code", "get_symbol", "list_dependencies"} {
		if !names[want] {
			t.Errorf("esperaba la tool %q registrada, tools: %v", want, names)
		}
	}

	indexRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "index_project",
		Arguments: map[string]any{"force": true},
	})
	if err != nil {
		t.Fatalf("CallTool index_project: %v", err)
	}
	if indexRes.IsError {
		t.Fatalf("index_project devolvió error: %v", toolResultText(indexRes))
	}

	statusRes, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "graph_status", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool graph_status: %v", err)
	}
	statusText := toolResultText(statusRes)
	if !strings.Contains(statusText, "Archivos: 1") {
		t.Errorf("esperaba 1 archivo indexado en graph_status, got: %s", statusText)
	}

	searchRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_code",
		Arguments: map[string]any{"query": "Helper"},
	})
	if err != nil {
		t.Fatalf("CallTool search_code: %v", err)
	}
	if !strings.Contains(toolResultText(searchRes), "Helper") {
		t.Errorf("esperaba encontrar Helper en search_code, got: %s", toolResultText(searchRes))
	}

	symbolRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_symbol",
		Arguments: map[string]any{"name": "UseHelper"},
	})
	if err != nil {
		t.Fatalf("CallTool get_symbol: %v", err)
	}
	symbolText := toolResultText(symbolRes)
	if !strings.Contains(symbolText, "Llama a:") || !strings.Contains(symbolText, "Helper") {
		t.Errorf("esperaba que get_symbol(UseHelper) muestre que llama a Helper, got: %s", symbolText)
	}

	depsRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_dependencies",
		Arguments: map[string]any{"name": "Helper", "direction": "in", "kind": "calls"},
	})
	if err != nil {
		t.Fatalf("CallTool list_dependencies: %v", err)
	}
	if !strings.Contains(toolResultText(depsRes), "UseHelper") {
		t.Errorf("esperaba que list_dependencies(Helper, in) incluya UseHelper, got: %s", toolResultText(depsRes))
	}
}

func writeFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func toolResultText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
