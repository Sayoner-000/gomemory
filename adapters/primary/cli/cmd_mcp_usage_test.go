package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/adapters/secondary/usage"
	"mem/domain"
)

// TestMCPServer_SearchAndList_RecordUsage es la verificación V3 de
// quickstart.md contra el servidor MCP REAL (no un equivalente por línea de
// comandos): levanta el servidor sobre un transporte en memoria, llama
// search_memories y list_memories como lo haría un cliente MCP de verdad, y
// comprueba que quedan registrados con línea base mayor que lo emitido
// (feature 020, T028).
func TestMCPServer_SearchAndList_RecordUsage(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	long := strings.Repeat("contenido bastante largo para forzar que el extracto de search/list sea menor que el original. ", 5)
	memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "una decisión", Content: long})

	usageRepo := persistence.NewUsageRepository(db)
	recorder := usage.NewRecorder(usageRepo, "proj", "mcp", func() string { return "" })

	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:    memRepo,
		SessionRepo:   persistence.NewSessionRepository(db),
		UsageRepo:     usageRepo,
		UsageRecorder: recorder,
		TokenCounter:  tokens.ApproximateTokenCounter{},
	}

	server := newMCPServer(deps, root, "proj")
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer clientSession.Close()

	if _, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_memories",
		Arguments: map[string]any{"query": "decisión"},
	}); err != nil {
		t.Fatalf("CallTool search_memories: %v", err)
	}
	if _, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_memories",
		Arguments: map[string]any{},
	}); err != nil {
		t.Fatalf("CallTool list_memories: %v", err)
	}
	// save_memory no optimiza: debe caer en el respaldo del middleware con
	// línea base == emitido (FR-005), etiquetado con el canal "mcp".
	if _, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "save_memory",
		Arguments: map[string]any{"title": "otra", "type": "learning", "content": "algo"},
	}); err != nil {
		t.Fatalf("CallTool save_memory: %v", err)
	}

	recs, err := usageRepo.Totals("proj")
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("se esperaban 3 registros de uso (search, list, save), got %d: %+v", len(recs), recs)
	}

	byOp := map[string]domain.UsageRecord{}
	for _, r := range recs {
		byOp[r.Operation] = r
		if r.Channel != "mcp" {
			t.Fatalf("todos los registros de esta sesión deben quedar etiquetados canal=mcp, got %q en %+v", r.Channel, r)
		}
	}

	search, ok := byOp[domain.OpSearchMemories]
	if !ok {
		t.Fatalf("falta el registro de search_memories: %+v", recs)
	}
	if search.BaselineTokens <= search.EmittedTokens {
		t.Fatalf("search_memories debe reducir: baseline=%d emitted=%d", search.BaselineTokens, search.EmittedTokens)
	}

	list, ok := byOp[domain.OpListMemories]
	if !ok {
		t.Fatalf("falta el registro de list_memories: %+v", recs)
	}
	if list.BaselineTokens <= list.EmittedTokens {
		t.Fatalf("list_memories debe reducir: baseline=%d emitted=%d", list.BaselineTokens, list.EmittedTokens)
	}

	// save_memory no optimiza (FR-005): línea base == emitido, vía el
	// respaldo del middleware, no vía auto-reporte.
	save, ok := byOp[domain.OpSaveMemory]
	if !ok {
		t.Fatalf("falta el registro de respaldo de save_memory: %+v", recs)
	}
	if save.BaselineTokens != save.EmittedTokens {
		t.Fatalf("save_memory (no optimiza) debe registrar baseline==emitted, got baseline=%d emitted=%d", save.BaselineTokens, save.EmittedTokens)
	}
}

// TestMCPServer_GetMemory_TranslatesToOwnOperation confirma que el
// middleware de respaldo traduce el nombre de la tool a la operación de
// dominio correspondiente (FR-003) para get_memory, en vez de dejarlo caer
// en "other".
func TestMCPServer_GetMemory_TranslatesToOwnOperation(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	id, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "d", Content: "contenido"})

	usageRepo := persistence.NewUsageRepository(db)
	recorder := usage.NewRecorder(usageRepo, "proj", "mcp", func() string { return "" })
	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:    memRepo,
		SessionRepo:   persistence.NewSessionRepository(db),
		UsageRepo:     usageRepo,
		UsageRecorder: recorder,
		TokenCounter:  tokens.ApproximateTokenCounter{},
	}

	server := newMCPServer(deps, root, "proj")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_memory", Arguments: map[string]any{"id": id}}); err != nil {
		t.Fatalf("CallTool get_memory: %v", err)
	}

	recs, _ := usageRepo.Totals("proj")
	if len(recs) != 1 || recs[0].Operation != domain.OpGetMemory {
		t.Fatalf("get_memory debe registrarse como domain.OpGetMemory, got %+v", recs)
	}
}

