package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/persistence"
)

// TestMCPReviewTools_FlujoCompleto recorre por MCP una revisión adversarial
// con un defecto confirmado por ambos revisores, corregido, re-juzgado como
// resuelto, finalizado y promovido a memoria. Comprueba las respuestas de
// cada herramienta, no solo que no fallen.
func TestMCPReviewTools_FlujoCompleto(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	memRepo := persistence.NewMemoryRepository(db)
	deps := &Deps{
		Root: root, Project: "proj",
		MemoryRepo:    memRepo,
		ReviewRepo:    persistence.NewReviewRepository(db),
		ConsensusRepo: persistence.NewConsensusRepository(db),
	}
	cs := connectMCPTestSession(t, deps, root, "proj")
	call := func(tool string, args map[string]any) map[string]any {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v res=%s", tool, err, toolText(res))
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(toolText(res)), &out); err != nil {
			t.Fatalf("%s: respuesta no JSON: %v\n%s", tool, err, toolText(res))
		}
		return out
	}

	start := call("review_start", map[string]any{
		"target_type": "diff", "revision": "working-tree", "digest": "sha256:v0", "scope": []string{"."},
		"reviewer_a":     map[string]any{"provider": "one", "model": "a"},
		"reviewer_b":     map[string]any{"provider": "two", "model": "b"},
		"max_fix_rounds": 2, "auto_fix_severities": []string{"CRITICAL", "HIGH"}, "fix_authorized": true,
	})
	reviewID, _ := start["review_id"].(string)
	if reviewID == "" || start["fix_authorized"] != true {
		t.Fatalf("review_start: %v", start)
	}

	findingIDs := map[string]float64{}
	for _, rv := range []struct{ who, local, provider, model string }{{"A", "A-001", "one", "a"}, {"B", "B-001", "two", "b"}} {
		out := call("review_submit", map[string]any{
			"review_id": reviewID, "reviewer": rv.who, "target_digest": "sha256:v0", "status": "success",
			"provider": rv.provider, "model": rv.model,
			"findings": []map[string]any{{
				"local_id": rv.local, "location": "x.go:1", "severity": "HIGH", "category": "correctness",
				"claim": "defecto reproducible", "evidence_class": "deterministic",
				"evidence": []string{"reproducido"}, "confidence": "high",
			}},
		})
		ids, _ := out["finding_ids"].(map[string]any)
		id, ok := ids[rv.local].(float64)
		if out["stored"] != true || !ok {
			t.Fatalf("review_submit %s: %v", rv.who, out)
		}
		findingIDs[rv.who] = id
		if rv.who == "B" && out["consensus_ready"] != true {
			t.Fatalf("tras ambos revisores el consenso debe estar listo: %v", out)
		}
	}

	cons := call("review_consensus", map[string]any{
		"review_id": reviewID,
		"matches": []map[string]any{{
			"status": "CONFIRMED", "finding_id_a": findingIDs["A"], "finding_id_b": findingIDs["B"],
			"severity": "HIGH", "claim": "defecto reproducible",
		}},
	})
	if found, _ := cons["consensus_findings"].([]any); len(found) != 1 {
		t.Fatalf("review_consensus: %v", cons)
	}

	if st := call("review_status", map[string]any{"review_id": reviewID}); !strings.Contains(mustJSON(t, st), reviewID) {
		t.Fatalf("review_status: %v", st)
	}

	fix := call("review_fix_record", map[string]any{
		"review_id": reviewID, "addressed_consensus_ids": []string{"C-001"},
		"base_target_digest": "sha256:v0", "fixed_target_digest": "sha256:v1",
		"modified_paths": []string{"x.go"}, "verification": []string{"go test ./..."},
	})
	if fix["fix_delta"] == nil {
		t.Fatalf("review_fix_record: %v", fix)
	}

	for _, who := range []string{"A", "B"} {
		rj := call("review_rejudge", map[string]any{
			"review_id": reviewID, "reviewer": who,
			"judgments": map[string]any{"C-001": map[string]any{"state": "RESOLVED", "evidence": []string{"ya no reproduce"}}},
		})
		if got, _ := rj["rejudged"].([]any); len(got) != 1 {
			t.Fatalf("review_rejudge %s: %v", who, rj)
		}
	}

	// Re-revisión del target corregido: la ronda nueva necesita el resultado
	// de ambos revisores sobre el digest vigente.
	for _, rv := range []struct{ who, provider, model string }{{"A", "one", "a"}, {"B", "two", "b"}} {
		out := call("review_submit", map[string]any{
			"review_id": reviewID, "reviewer": rv.who, "target_digest": "sha256:v1", "status": "success",
			"provider": rv.provider, "model": rv.model, "findings": []map[string]any{},
		})
		if out["stored"] != true {
			t.Fatalf("re-revisión %s: %v", rv.who, out)
		}
	}

	final := call("review_finalize", map[string]any{"review_id": reviewID})
	if final["verdict"] != "APPROVED" {
		t.Fatalf("review_finalize: %v", final)
	}
	if prom, _ := final["promotable_findings"].([]any); len(prom) != 1 {
		t.Fatalf("debe haber un hallazgo promovible: %v", final)
	}

	promoted := call("review_promote_memory", map[string]any{
		"review_id": reviewID,
		"learnings": map[string]any{"C-001": map[string]any{
			"category": "correctness", "component": "x", "problem": "defecto reproducible",
			"root_cause": "condición invertida", "resolution": "se corrigió la condición",
			"verification": []string{"go test ./..."}, "confidence": "high",
		}},
	})
	if got, _ := promoted["promoted"].([]any); len(got) != 1 {
		t.Fatalf("review_promote_memory: %v", promoted)
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "review_status", Arguments: map[string]any{"review_id": "rev-inexistente"}})
	if err != nil || !res.IsError {
		t.Fatalf("una revisión inexistente debe ser un error de herramienta: err=%v res=%v", err, res)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
