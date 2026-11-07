package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// HCD-046: la tool MCP hcd_history devuelve el mismo listado que la CLI.
func TestMCP_HCDHistory(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	history := persistence.NewDecisionHistoryRepository(db)
	for _, d := range []struct {
		m     domain.Memory
		files []string
	}{
		{domain.Memory{Project: "proj", Type: domain.Decision, Title: "HCD-044 · Metadatos privados (cerrada)", Content: "Tipo: fix · Carril: full · Estado: cerrada · Inicio: 2026-11-06\nIntención: rechazar topic_key privado", Filepath: "hcd/HCD-044-x.md", TopicKey: "hcd/HCD-044"}, []string{"adapters/secondary/persistence/memory.go"}},
		{domain.Memory{Project: "proj", Type: domain.Discovery, Title: "Auditoría 2026-10-11 · QA", Content: "Informe: docs/audits/2026-10-11-qa.md", Filepath: "docs/audits/2026-10-11-qa.md", TopicKey: "audit/2026-10-11-qa.md"}, nil},
	} {
		m := d.m
		if _, err := history.UpsertDigest(&m, d.files, true); err != nil {
			t.Fatal(err)
		}
	}
	query := &usecases.HistoryQuery{Reader: history, Project: "proj"}
	cs := connectMCPTestSession(t, &Deps{Root: root, Project: "proj", HistoryQuery: query}, root, "proj")

	for _, args := range []map[string]any{{}, {"text": "topic_key"}, {"file": "adapters/secondary/persistence/memory.go"}, {"file": "otro.go"}} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "hcd_history", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("hcd_history %v: err=%v res=%s", args, err, toolText(res))
		}
		var got []usecases.HistoryItem
		if err := json.Unmarshal([]byte(toolText(res)), &got); err != nil {
			t.Fatalf("hcd_history %v devolvió %q: %v", args, toolText(res), err)
		}
		text, _ := args["text"].(string)
		file, _ := args["file"].(string)
		want, err := query.List(text, file)
		if err != nil {
			t.Fatal(err)
		}
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Fatalf("hcd_history %v = %s, la CLI da %s", args, gotJSON, wantJSON)
		}
	}
}
