package hcd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/primary/cli/clikit"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// HCD-046: mem hcd index y mem hcd history.

type memAudits map[string]string

func (a memAudits) ListAudits() ([]string, error) {
	var out []string
	for k := range a {
		out = append(out, k)
	}
	return out, nil
}
func (a memAudits) ReadAudit(rel string) ([]byte, error) { return []byte(a[rel]), nil }

func newHistoryDeps(t *testing.T) *clikit.Deps {
	t.Helper()
	deps, root := newDeps(t)
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	history := persistence.NewDecisionHistoryRepository(db)
	memRepo := persistence.NewMemoryRepository(db)
	deps.HistoryIndex = &usecases.IndexHistory{
		Units:    deps.HCD,
		Audits:   memAudits{"docs/audits/2026-10-11-qa.md": "# Auditoría QA\n\n## 2. Hallazgos\n\n| ID | Severidad | Problema |\n|---|---|---|\n| H-01 | Alta | Fuga en `adapters/secondary/persistence/memory.go` |\n"},
		Memories: memRepo.(ports.HistoryMemories),
		History:  history,
		Project:  "p",
	}
	deps.HistoryQuery = &usecases.HistoryQuery{Reader: history, Project: "p"}
	ctx := context.Background()
	if _, err := deps.HCD.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Metadatos privados", Why: "privacidad", Intent: "Rechazar topic_key privado en `adapters/secondary/persistence/memory.go`"}); err != nil {
		t.Fatal(err)
	}
	step := domain.Memory{Project: "p", Type: domain.Decision, Title: "HCD-001: carril fast", Content: "Carril fast"}
	if _, err := memRepo.Insert(&step); err != nil {
		t.Fatal(err)
	}
	return deps
}

func TestExecuteIndex(t *testing.T) {
	deps := newHistoryDeps(t)

	code, r := call(t, deps, "index")
	if code != 0 || !r.OK || r.Data["applied"] != false || r.Data["created"] != float64(2) || r.Data["deleted"] != float64(1) {
		t.Fatalf("vista previa = %d %+v", code, r)
	}
	code, r = call(t, deps, "index", "--apply")
	if code != 0 || r.Data["applied"] != true || r.Data["created"] != float64(2) || r.Data["deleted"] != float64(1) {
		t.Fatalf("aplicar = %d %+v", code, r)
	}
	code, r = call(t, deps, "index", "--apply")
	if code != 0 || r.Data["created"] != float64(0) || r.Data["unchanged"] != float64(2) || r.Data["deleted"] != float64(0) {
		t.Fatalf("repetir = %d %+v", code, r)
	}

	var out, errOut bytes.Buffer
	if code := run(deps, []string{"index"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "Vista previa") || !strings.Contains(out.String(), "--apply") {
		t.Fatalf("texto = %d %q %q", code, out.String(), errOut.String())
	}
	if code, r := call(t, deps, "index", "--quote", "x"); code == 0 || r.OK {
		t.Fatalf("index admitió un flag ajeno: %+v", r)
	}
}

func TestExecuteHistory(t *testing.T) {
	deps := newHistoryDeps(t)
	if code, _ := call(t, deps, "index", "--apply"); code != 0 {
		t.Fatal("index --apply falló")
	}

	code, r := call(t, deps, "history")
	items := historyItems(t, r)
	if code != 0 || len(items) != 2 || !strings.HasPrefix(items[0]["title"].(string), "Auditoría 2026-10-11") || !strings.HasPrefix(items[1]["title"].(string), "HCD-001 · Metadatos privados") {
		t.Fatalf("history = %d %+v", code, items)
	}
	if _, r = call(t, deps, "history", "topic_key"); len(historyItems(t, r)) != 1 {
		t.Fatalf("filtro por texto = %+v", historyItems(t, r))
	}
	if _, r = call(t, deps, "history", "--file", "adapters/secondary/persistence/memory.go"); len(historyItems(t, r)) != 2 {
		t.Fatalf("filtro por archivo = %+v", historyItems(t, r))
	}
	if _, r = call(t, deps, "history", "--file", "otro.go"); len(historyItems(t, r)) != 0 {
		t.Fatalf("archivo sin fichas = %+v", historyItems(t, r))
	}

	var out, errOut bytes.Buffer
	if code := run(deps, []string{"history", "--file", "adapters/secondary/persistence/memory.go"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "HCD-001 · Metadatos privados (activa)") || !strings.Contains(out.String(), "get_memory") {
		t.Fatalf("texto = %d %q %q", code, out.String(), errOut.String())
	}
}

func historyItems(t *testing.T, r Response) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(r.Data["items"])
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("items = %s: %v", raw, err)
	}
	return items
}
