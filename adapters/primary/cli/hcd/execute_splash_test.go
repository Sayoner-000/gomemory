package hcd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHCD057SplashCLI(t *testing.T) {
	deps, root := newDeps(t)
	// Start unit in full lane
	code, r := call(t, deps, "start", "--kind", "evolution", "--lane", "full", "--title", "Splash CLI", "--why", "probar CLI de splash", "--intent", "CLI splash review", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("start failed: %d %+v", code, r)
	}
	unitID := r.UnitID

	// Fill required sections
	docPath := filepath.Join(root, r.DocPath)
	data, _ := os.ReadFile(docPath)
	filled := strings.NewReplacer(
		"Incluido: pendiente", "Incluido: guardado",
		"Excluido: pendiente", "Excluido: exportación",
		"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nnada",
		"## Alternativa más barata\npendiente", "## Alternativa más barata\nninguna",
		"### US1. pendiente\npendiente\n- C-01: pendiente", "### US1. Historia\nComo usuario quiero guardar para persistir\n- C-01: Dado x, cuando y, entonces z",
	).Replace(string(data))
	_ = os.WriteFile(docPath, []byte(filled), 0o644)

	// Complete socratic review first
	code, r = call(t, deps, "show", unitID)
	rev := r.DefinitionRev
	socraticPayload := `{"summary": "Examen socratico verificado", "questions": []}`
	code, r = callWithStdin(t, deps, socraticPayload, "review", unitID, "--kind", "socratic", "--action", "complete", "--revision", rev, "--operation-id", "op-soc-cli", "--input", "-", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("socratic review failed: %d %+v", code, r)
	}

	// Initial splash status is missing
	code, r = call(t, deps, "show", unitID)
	revData := r.Data["definition_review"].(map[string]any)
	if revData["splash"] != "missing" {
		t.Fatalf("expected splash missing, got %v", revData["splash"])
	}

	// 1. Start without accept is rejected
	code, _ = call(t, deps, "review", unitID, "--kind", "splash", "--action", "start", "--revision", rev, "--operation-id", "op-start-bad", "--attempt", "att-1", "--client", "antigravity")
	if code == 0 {
		t.Fatal("expected failure on start without accept")
	}

	// 2. Accept splash with quote
	code, r = call(t, deps, "review", unitID, "--kind", "splash", "--action", "accept", "--revision", rev, "--operation-id", "op-acc-cli", "--quote", "acepto la revisión splash", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("accept failed: %d %+v", code, r)
	}
	revData = r.Data["definition_review"].(map[string]any)
	if revData["splash"] != "in_progress" {
		t.Fatalf("expected splash in_progress after accept, got %v", revData["splash"])
	}

	// 3. Start attempt att-1
	code, r = call(t, deps, "review", unitID, "--kind", "splash", "--action", "start", "--revision", rev, "--operation-id", "op-start-cli", "--attempt", "att-1", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("start failed: %d %+v", code, r)
	}

	// 4. Fail attempt att-1 with reason
	code, r = call(t, deps, "review", unitID, "--kind", "splash", "--action", "fail", "--revision", rev, "--operation-id", "op-fail-cli", "--attempt", "att-1", "--reason", "timeout de ejecucion", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("fail failed: %d %+v", code, r)
	}
	revData = r.Data["definition_review"].(map[string]any)
	if revData["splash"] != "failed" {
		t.Fatalf("expected splash failed, got %v", revData["splash"])
	}

	// 5. Start attempt att-2 and complete with Blocker B1
	code, r = call(t, deps, "review", unitID, "--kind", "splash", "--action", "start", "--revision", rev, "--operation-id", "op-start-cli-2", "--attempt", "att-2", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("start att-2 failed: %d %+v", code, r)
	}

	splashReport := `{
		"attempt_id": "att-2",
		"summary": "Informe splash con blocker y major",
		"findings": [
			{
				"id": "B1",
				"severity": "Blocker",
				"quote": "Incluido: guardado",
				"problem": "Bloqueo critico detectado"
			},
			{
				"id": "M1",
				"severity": "Major",
				"problem": "Advertencia importante"
			}
		]
	}`
	code, r = callWithStdin(t, deps, splashReport, "review", unitID, "--kind", "splash", "--action", "complete", "--revision", rev, "--operation-id", "op-comp-cli-2", "--input", "-", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("complete failed: %d %+v", code, r)
	}
	revData = r.Data["definition_review"].(map[string]any)
	if revData["splash"] != "pending_blockers" {
		t.Fatalf("expected splash pending_blockers, got %v", revData["splash"])
	}

	// 6. Dismiss Blocker B1 with quote
	code, r = call(t, deps, "review", unitID, "--kind", "splash", "--action", "dismiss", "--revision", rev, "--operation-id", "op-dism-cli", "--finding", "B1", "--quote", "descarto B1 por justificacion documentada", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("dismiss failed: %d %+v", code, r)
	}
	revData = r.Data["definition_review"].(map[string]any)
	if revData["splash"] != "completed" {
		t.Fatalf("expected splash completed after dismissal, got %v", revData["splash"])
	}

	// 7. Skip splash in another attempt/round
	code, r = call(t, deps, "review", unitID, "--kind", "splash", "--action", "skip", "--revision", rev, "--operation-id", "op-skip-cli", "--quote", "omito splash finalmente", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("skip failed: %d %+v", code, r)
	}
	revData = r.Data["definition_review"].(map[string]any)
	if revData["splash"] != "skipped" {
		t.Fatalf("expected splash skipped, got %v", revData["splash"])
	}
}
