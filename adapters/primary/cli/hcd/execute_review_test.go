package hcd

import (
	"bytes"
	"encoding/json"
	"mem/adapters/primary/cli/clikit"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func callWithStdin(t *testing.T, deps *clikit.Deps, stdin string, args ...string) (int, Response) {
	t.Helper()
	var out, errOut bytes.Buffer
	inReader := strings.NewReader(stdin)
	code := runWithStdin(deps, append([]string{args[0], "--json"}, args[1:]...), inReader, &out, &errOut)
	var r Response
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("decode %q (stderr %q): %v", out.String(), errOut.String(), err)
	}
	return code, r
}

func TestHCD057SocraticCLI(t *testing.T) {
	deps, root := newDeps(t)
	// Start unit in full lane
	code, r := call(t, deps, "start", "--kind", "evolution", "--lane", "full", "--title", "Socratic CLI", "--why", "probar CLI de examen", "--intent", "CLI socratic review", "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("start failed: %d %+v", code, r)
	}
	unitID := r.UnitID

	// Fill required sections
	docPath := filepath.Join(root, r.DocPath)
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	filled := strings.NewReplacer(
		"Incluido: pendiente", "Incluido: guardado",
		"Excluido: pendiente", "Excluido: exportación",
		"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nnada",
		"## Alternativa más barata\npendiente", "## Alternativa más barata\nninguna",
		"### US1. pendiente\npendiente\n- C-01: pendiente", "### US1. Historia\nComo usuario quiero guardar para persistir\n- C-01: Dado x, cuando y, entonces z",
	).Replace(string(data))
	if err := os.WriteFile(docPath, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}

	// Show unit: stage is define, socratic status is missing
	code, r = call(t, deps, "show", unitID)
	if code != 0 || r.Stage != "define" || r.NextAction.Skill != "hcd-define" {
		t.Fatalf("show: code=%d stage=%s skill=%s", code, r.Stage, r.NextAction.Skill)
	}
	rev := r.DefinitionRev
	if rev == "" {
		t.Fatal("expected non-empty definition revision")
	}
	reviewData, ok := r.Data["definition_review"].(map[string]any)
	if !ok || reviewData["socratic"] != "missing" {
		t.Fatalf("expected socratic missing in data.definition_review: %+v", r.Data)
	}

	// 1. Missing flags or unknown flags
	code, _ = call(t, deps, "review", unitID, "--unknown", "val")
	if code != 2 {
		t.Fatalf("expected usage code 2 on unknown flag, got %d", code)
	}

	code, r = call(t, deps, "review", unitID, "--kind", "socratic")
	if code != 2 || r.OK {
		t.Fatalf("expected usage code on missing flags, got %d", code)
	}

	// 2. Invalid input file (nonexistent)
	code, r = call(t, deps, "review", unitID, "--kind", "socratic", "--action", "complete", "--revision", rev, "--operation-id", "op-1", "--input", "nonexistent-file.json")
	if code == 0 || r.OK {
		t.Fatalf("expected failure on nonexistent input file")
	}

	// 3. Malformed JSON input
	badJSON := filepath.Join(root, "bad.json")
	if err := os.WriteFile(badJSON, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, r = call(t, deps, "review", unitID, "--kind", "socratic", "--action", "complete", "--revision", rev, "--operation-id", "op-1", "--input", badJSON)
	if code == 0 || r.OK || r.Error == nil || r.Error.Code != "validation" {
		t.Fatalf("expected validation error on bad json, got %d %+v", code, r)
	}

	// 3b. Trailing data after JSON object rejected (H-03 / C-005)
	trailingJSON := filepath.Join(root, "trailing.json")
	if err := os.WriteFile(trailingJSON, []byte(`{"summary":"ok"} trailing`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, r = call(t, deps, "review", unitID, "--kind", "socratic", "--action", "complete", "--revision", rev, "--operation-id", "op-trailing", "--input", trailingJSON)
	if code != 5 || r.OK || r.Error == nil || r.Error.Code != "validation" {
		t.Fatalf("expected exit code 5 / validation error on trailing json data, got %d %+v", code, r)
	}

	// 4. Valid examination via input file
	validPayload := `{
		"summary": "Examen socratico verificado con supuestos claros",
		"questions": [
			{
				"text": "¿Se requiere validacion de esquema previa?",
				"answer": "Si, la migracion transaccional cubre el esquema.",
				"status": "answered"
			},
			{
				"text": "¿Hay alternativas mas baratas?",
				"status": "omitted"
			}
		]
	}`
	validFile := filepath.Join(root, "exam.json")
	if err := os.WriteFile(validFile, []byte(validPayload), 0o644); err != nil {
		t.Fatal(err)
	}

	code, r = call(t, deps, "review", unitID, "--kind", "socratic", "--action", "complete", "--revision", rev, "--operation-id", "op-socratic-1", "--input", validFile, "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("valid review failed: code=%d err=%+v", code, r.Error)
	}
	if r.Stage != "approve" || r.NextAction.Skill != "hcd-approve" {
		t.Fatalf("expected stage approve and skill hcd-approve, got %s / %s", r.Stage, r.NextAction.Skill)
	}
	revData := r.Data["definition_review"].(map[string]any)
	if revData["socratic"] != "complete" {
		t.Fatalf("expected socratic complete, got %v", revData["socratic"])
	}

	// 5. Replay of same operation succeeds idempotently
	code, r = call(t, deps, "review", unitID, "--kind", "socratic", "--action", "complete", "--revision", rev, "--operation-id", "op-socratic-1", "--input", validFile, "--client", "antigravity")
	if code != 0 || !r.OK {
		t.Fatalf("replay failed: code=%d err=%+v", code, r.Error)
	}

	// 6. Test stdin input via --input -
	stdinPayload := `{
		"summary": "Examen socratico repetido por stdin",
		"questions": []
	}`
	// Different operation on same completed definition without changing revision is rejected
	code, r = callWithStdin(t, deps, stdinPayload, "review", unitID, "--kind", "socratic", "--action", "complete", "--revision", rev, "--operation-id", "op-socratic-2", "--input", "-", "--client", "antigravity")
	if code == 0 || r.OK {
		t.Fatalf("expected error on opening second round for same revision")
	}

	// 7. Editorial change invalidates review (becomes expired)
	data, _ = os.ReadFile(docPath)
	edited := strings.Replace(string(data), "Incluido: guardado", "Incluido: guardado y lectura", 1)
	_ = os.WriteFile(docPath, []byte(edited), 0o644)

	code, r = call(t, deps, "show", unitID)
	if code != 0 || r.Stage != "define" || r.NextAction.Skill != "hcd-define" {
		t.Fatalf("show after edit: stage=%s skill=%s", r.Stage, r.NextAction.Skill)
	}
	revData = r.Data["definition_review"].(map[string]any)
	if revData["socratic"] != "expired" {
		t.Fatalf("expected socratic expired after revision change, got %v", revData["socratic"])
	}
}
