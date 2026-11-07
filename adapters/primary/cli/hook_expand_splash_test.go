package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// HCD-047: en Codex el hijo de spawn_agent no dispara hooks de arranque; con
// fork_turns distinto de "none" hereda la conversación del raíz, memoria
// incluida (sondeo real, Codex 0.166.0). El pre-tool del raíz avisa.
func TestExpandPreTool_SplashCodexSinFork(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, persistence.MemDir), 0o700); err != nil {
		t.Fatal(err)
	}
	allow := func(context.Context, domain.ExpandSignals, []domain.SupertoolAction, usecases.ExpandHookDeps) domain.ExpandOutcome {
		return domain.ExpandOutcome{}
	}
	call := func(session string, input map[string]any) string {
		raw, _ := json.Marshal(map[string]any{"session_id": session, "hook_event_name": "PreToolUse", "tool_name": "collaborationspawn_agent", "tool_input": input})
		d := hookExpandDeps{root: root, now: time.Now, run: allow}
		return runHookExpand(context.Background(), d, "pre-tool", "json", raw).stdout
	}

	if out := call("s1", map[string]any{"task_name": "splash", "fork_turns": "all", "message": "x"}); !strings.Contains(out, `fork_turns`) || !strings.Contains(out, "splash") {
		t.Fatalf("un splash con fork_turns=all debe avisar, salió %q", out)
	}
	if out := call("s2", map[string]any{"agent_type": "splash", "message": "x"}); !strings.Contains(out, "fork_turns") {
		t.Fatalf("un splash sin fork_turns (hereda por defecto) debe avisar, salió %q", out)
	}
	if out := call("s3", map[string]any{"task_name": "splash", "fork_turns": "none", "message": "x"}); strings.Contains(out, "fork_turns") {
		t.Fatalf("un splash con fork_turns=none no debe avisar, salió %q", out)
	}
	if out := call("s4", map[string]any{"task_name": "revisor", "fork_turns": "all", "message": "x"}); strings.Contains(out, "fork_turns") {
		t.Fatalf("otro subagente no debe avisar, salió %q", out)
	}
}
