package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mem/adapters/secondary/hcdfiles"
	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
)

type renameTick struct{ t time.Time }

func (c *renameTick) Now() time.Time { c.t = c.t.Add(time.Second); return c.t }

// HCD-058: la tool MCP hcd_rename es la vía de la IA para renombrar una
// unidad; deja el mismo resultado que la CLI.
func TestHCDRenameMCP(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	tpl, err := os.ReadFile("../../../infrastructure/templates/hcd/unit/unit.md")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	store := persistence.NewHCDStore(db, "proj")
	uc := usecases.NewHCD(store, hcdfiles.Workspace{Root: root}, &renameTick{t: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}, func() string { n++; return fmt.Sprint(n) }, string(tpl))
	uc.ConfigureDecisions(persistence.NewDecisionHistoryRepository(db), "proj")
	ctx := context.Background()
	if _, err := uc.Start(ctx, usecases.HCDStartInput{Kind: "fix", Lane: "fast", Title: "Titulo original", Why: "w"}); err != nil {
		t.Fatal(err)
	}
	cs := connectMCPTestSession(t, &Deps{Root: root, Project: "proj", HCD: uc}, root, "proj")

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "hcd_rename", Arguments: map[string]any{"unit": "HCD-001", "title": "Título por IA"}})
	if err != nil || res.IsError {
		t.Fatalf("hcd_rename: err=%v res=%s", err, toolText(res))
	}
	var got struct {
		UnitID  string `json:"unit_id"`
		Title   string `json:"title"`
		DocPath string `json:"doc_path"`
	}
	if err := json.Unmarshal([]byte(toolText(res)), &got); err != nil {
		t.Fatalf("respuesta %q: %v", toolText(res), err)
	}
	if got.UnitID != "HCD-001" || got.Title != "Título por IA" || got.DocPath != "hcd/HCD-001-titulo-por-ia.md" {
		t.Fatalf("hcd_rename = %+v", got)
	}
	data, err := os.ReadFile(filepath.Join(root, got.DocPath))
	if err != nil || !strings.HasPrefix(string(data), "# Título por IA\n") {
		t.Fatalf("doc = %q %v", data, err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "hcd_rename", Arguments: map[string]any{"title": " "}})
	if err == nil && !res.IsError {
		t.Fatalf("hcd_rename con título vacío debe fallar: %s", toolText(res))
	}
}
