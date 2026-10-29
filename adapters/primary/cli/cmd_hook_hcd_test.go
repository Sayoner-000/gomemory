package cli

import (
	"context"
	"errors"
	"io"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHCDHookRecordsEvenEmptyActivityAndReportsFailure(t *testing.T) {
	calls := 0
	deps := &Deps{HCDHeartbeat: func(ctx context.Context, input usecases.HCDHeartbeatInput) error {
		calls++
		if input.Client != "codex" || input.SessionID != "session" {
			t.Fatalf("input=%+v", input)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("missing deadline")
		}
		return errors.New("temporary failure")
	}}
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	old := os.Stderr
	os.Stderr = w
	recordHCDHeartbeat(deps, t.TempDir(), "project", map[string]any{"client": "codex", "session_id": "session"}, turnActivity{})
	os.Stderr = old
	w.Close()
	data, _ := io.ReadAll(r)
	r.Close()
	if calls != 1 || !strings.Contains(string(data), "mem hook: hcd heartbeat: temporary failure; se ignora") {
		t.Fatalf("calls=%d output=%q", calls, data)
	}
}

type hcdHookProject struct {
	ports.ProjectRepository
	root string
}

func (p hcdHookProject) FindRoot() (string, error) { return p.root, nil }
func (p hcdHookProject) Key(string) string         { return "project" }

type hcdHookClock struct{}

func (hcdHookClock) Now() time.Time { return time.Now() }
func TestHCDSubagentStopHeartbeatWithActiveFocus(t *testing.T) {
	root := t.TempDir()
	db, e := persistence.Init(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	repo := persistence.NewHCDStore(db, "project")
	ctx := context.Background()
	if e = repo.CreateUnit(ctx, domain.Unit{ID: "HCD-001", Approach: "hcd", State: domain.StateDraft, StateRevision: 1}); e != nil {
		t.Fatal(e)
	}
	if e = repo.SetFocus(ctx, domain.Focus{UnitID: "HCD-001"}); e != nil {
		t.Fatal(e)
	}
	uc := usecases.NewHCD(repo, repo, nil, repo, hcdHookClock{}, nil)
	deps := &Deps{ProjectRepo: hcdHookProject{root: root}, HCD: uc}
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	_, e = w.WriteString(`{"client":"codex","session_id":"session"}`)
	if e != nil {
		t.Fatal(e)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	hookSubagentStop(deps, nil)
	os.Stdin = old
	r.Close()
	events, e := repo.ListHeartbeats(ctx, "HCD-001")
	if e != nil || len(events) != 1 || events[0].Client != "codex" {
		t.Fatalf("heartbeat=%+v %v", events, e)
	}
}

func TestWithDialectClientAttributesCodexOnly(t *testing.T) {
	if got := withDialectClient(map[string]any{}, dialectJSON)["client"]; got != "codex" {
		t.Fatalf("json sin client = %v", got)
	}
	if got := withDialectClient(map[string]any{"client": "opencode"}, dialectJSON)["client"]; got != "opencode" {
		t.Fatalf("client explícito pisado: %v", got)
	}
	if _, ok := withDialectClient(map[string]any{}, dialectClaude)["client"]; ok {
		t.Fatal("dialecto claude no debe inventar client")
	}
	if withDialectClient(nil, dialectJSON) != nil {
		t.Fatal("payload nil debe seguir nil")
	}
}
