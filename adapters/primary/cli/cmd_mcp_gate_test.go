package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
)

func TestMCPServer_SaveMemory_AvisaCasiDuplicado(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	deps := &Deps{Root: root, Project: "proj", MemoryRepo: persistence.NewMemoryRepository(db), SessionRepo: persistence.NewSessionRepository(db)}
	server := newMCPServer(deps, root, "proj")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	call := func(title string) (string, error) {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "save_memory", Arguments: map[string]any{"type": "learning", "title": title, "content": "la caché redis compartida reduce latencia de consultas"}})
		if err != nil {
			return "", err
		}
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				return text.Text, nil
			}
		}
		return "", nil
	}
	if _, err := call("Caché Redis compartida para latencia"); err != nil {
		t.Fatal(err)
	}
	second, err := call("Caché Redis compartida para reducir latencia")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(second, "✓ Memoria guardada") || !strings.Contains(second, "Posible duplicado de #") {
		t.Fatalf("respuesta=%s", second)
	}
}
