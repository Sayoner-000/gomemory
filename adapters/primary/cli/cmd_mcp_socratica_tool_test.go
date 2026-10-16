package cli

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type socraticaMCPGenerator struct{ calls int }

func (g *socraticaMCPGenerator) Name() string    { return "fake" }
func (g *socraticaMCPGenerator) Available() bool { return true }
func (g *socraticaMCPGenerator) GenerateQuestions(context.Context, ports.SocraticGenerationRequest) (domain.GenerationResult, error) {
	g.calls++
	return domain.GenerationResult{}, nil
}
func (g *socraticaMCPGenerator) Rewrite(context.Context, ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	g.calls++
	return domain.RewriteResult{Proposed: "x"}, nil
}

type socraticaMCPEnv struct {
	root string
	repo *persistence.SocraticaStore
	u    *usecases.SocraticaFlow
	gen  *socraticaMCPGenerator
	cs   *mcp.ClientSession
	t    *testing.T
}

func newSocraticaMCPEnv(t *testing.T, configure func(*usecases.SocraticaFlow)) *socraticaMCPEnv {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := persistence.NewSocraticaStore(db, time.Second)
	gen := &socraticaMCPGenerator{}
	u := &usecases.SocraticaFlow{Repo: repo, UOW: repo, Lock: persistence.NewSocraticaDispatchLock(root), Generator: gen, Clock: clock.SystemClock{}, NewID: func() string { return uuid.Must(uuid.NewV7()).String() }}
	if configure != nil {
		configure(u)
	}
	deps := &Deps{Root: root, Project: "p", Socratica: u, MemoryRepo: persistence.NewMemoryRepository(db), ReviewRepo: persistence.NewReviewRepository(db), ConsensusRepo: persistence.NewConsensusRepository(db)}
	return &socraticaMCPEnv{root: root, repo: repo, u: u, gen: gen, cs: connectMCPTestSession(t, deps, root, "p"), t: t}
}

func (e *socraticaMCPEnv) raw(args map[string]any) (map[string]any, string) {
	e.t.Helper()
	res, err := e.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "socratica_flow", Arguments: args})
	if err != nil {
		e.t.Fatal(err)
	}
	if res.IsError {
		return nil, toolText(res)
	}
	var out map[string]any
	if err = json.Unmarshal([]byte(toolText(res)), &out); err != nil {
		e.t.Fatal(err)
	}
	return out, ""
}
func (e *socraticaMCPEnv) call(args map[string]any) map[string]any {
	e.t.Helper()
	out, failure := e.raw(args)
	if failure != "" {
		e.t.Fatalf("%v: %s", args, failure)
	}
	return out
}
func (e *socraticaMCPEnv) fails(args map[string]any) string {
	e.t.Helper()
	_, failure := e.raw(args)
	if failure == "" {
		e.t.Fatalf("%v: expected error", args)
	}
	return failure
}

// review creates a flow in review without questions.
func (e *socraticaMCPEnv) review(client string) string {
	e.t.Helper()
	id := e.call(map[string]any{"action": "start", "client": client, "mode": "refine", "prompt": "original"})["flow_id"].(string)
	e.call(map[string]any{"action": "answer", "flow_id": id, "questions": []any{}})
	e.call(map[string]any{"action": "rewrite", "flow_id": id, "proposed": "proposal [name]"})
	return id
}

func TestMCPSocraticaFlow(t *testing.T) {
	t.Run("schema registered", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		tools, err := e.cs.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range tools.Tools {
			if tool.Name == "socratica_flow" {
				raw, _ := json.Marshal(tool.InputSchema)
				if !strings.Contains(string(raw), `"action"`) || !strings.Contains(string(raw), `"ack_nonce"`) {
					t.Fatal(string(raw))
				}
				return
			}
		}
		t.Fatal("socratica_flow not registered")
	})
	t.Run("refine and dispatch protocol", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		id := e.call(map[string]any{"action": "start", "client": "claude", "mode": "refine", "prompt": "original"})["flow_id"]
		q := e.call(map[string]any{"action": "answer", "flow_id": id, "questions": []any{map[string]any{"text": "Q", "options": []string{"A", "B"}}}})
		if q["phase"] != "questions" {
			t.Fatal(q)
		}
		e.call(map[string]any{"action": "answer", "flow_id": id, "answers": []any{map[string]any{"index": 0, "choice": "A"}}})
		e.call(map[string]any{"action": "rewrite", "flow_id": id, "proposed": "proposal [name]"})
		e.call(map[string]any{"action": "edit", "flow_id": id, "prompt": "edited [name]"})
		a := e.call(map[string]any{"action": "approve", "flow_id": id})
		dispatch := a["dispatch_id"]
		if a["next"] != usecases.NextReadyToAck || a["approved_prompt"] != "edited [name]" {
			t.Fatal(a)
		}
		d := e.call(map[string]any{"action": "approve", "flow_id": id})
		if d["duplicate"] != true || d["acked"] != false || d["dispatch_id"] != dispatch || d["next"] != usecases.NextReadyToAck {
			t.Fatal(d)
		}
		e.fails(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": "foreign", "ack_nonce": "n"})
		ack := map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "nonce"}
		if got := e.call(ack); got["acked"] != true || got["next"] != usecases.NextExecute {
			t.Fatal(got)
		}
		if got := e.call(ack); got["acked"] != true {
			t.Fatal("lost response not recoverable", got)
		}
		ack["ack_nonce"] = "other"
		if got := e.call(ack); got["already_dispatched"] != true || got["acked"] == true {
			t.Fatal(got)
		}
		after := e.call(map[string]any{"action": "approve", "flow_id": id})
		if after["already_dispatched"] != true || after["next"] != usecases.NextConsumed {
			t.Fatal(after)
		}
		// A new session after the crash sees the consumed authorization.
		got := e.call(map[string]any{"action": "get", "flow_id": id})
		if got["already_dispatched"] != true || got["next"] != usecases.NextConsumed {
			t.Fatal(got)
		}
		e.call(map[string]any{"action": "force_resend", "flow_id": id, "dispatch_id": dispatch})
		again := e.call(map[string]any{"action": "approve", "flow_id": id})
		if again["duplicate"] != true || again["acked"] != false || again["dispatch_id"] == dispatch {
			t.Fatal(again)
		}
		e.fails(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "late"})
		if got := e.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": again["dispatch_id"], "ack_nonce": "n2"}); got["acked"] != true {
			t.Fatal(got)
		}
	})
	t.Run("concurrent acks", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		id := e.review("claude")
		dispatch := e.call(map[string]any{"action": "approve", "flow_id": id})["dispatch_id"]
		var wg sync.WaitGroup
		var mu sync.Mutex
		acked := 0
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				out, _ := e.raw(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": []string{"a", "b"}[i]})
				if out["acked"] == true {
					mu.Lock()
					acked++
					mu.Unlock()
				}
			}(i)
		}
		wg.Wait()
		if acked != 1 {
			t.Fatal(acked)
		}
	})
	t.Run("uncertain reconciliation", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		id := e.review("claude")
		dispatch := e.call(map[string]any{"action": "approve", "flow_id": id})["dispatch_id"].(string)
		if ok, err := e.repo.ClaimDispatch(context.Background(), id, dispatch, "local", time.Now()); err != nil || !ok {
			t.Fatal(ok, err)
		}
		got := e.call(map[string]any{"action": "get", "flow_id": id})
		if got["uncertain"] != true || got["next"] != usecases.NextUncertain || got["recoverable"] == true {
			t.Fatal(got)
		}
		if r := e.call(map[string]any{"action": "resume", "flow_id": id}); r["phase"] != "approved" || r["uncertain"] != true {
			t.Fatal("resume of a claimed flow must never reach review", r)
		}
		e.fails(map[string]any{"action": "approve", "flow_id": id})
		e.fails(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n"})
		release, ok, err := persistence.NewSocraticaDispatchLock(e.root).TryAcquire(id)
		if err != nil || !ok {
			t.Fatal(ok, err)
		}
		held := e.call(map[string]any{"action": "force_resend", "flow_id": id, "dispatch_id": dispatch})
		if held["in_flight"] != true || held["next"] != usecases.NextInFlight || held["dispatch_id"] != dispatch {
			t.Fatal(held)
		}
		release()
		e.call(map[string]any{"action": "force_resend", "flow_id": id, "dispatch_id": dispatch})
		a := e.call(map[string]any{"action": "approve", "flow_id": id})
		if a["duplicate"] != true || a["acked"] != false || a["dispatch_id"] == dispatch {
			t.Fatal(a)
		}
		e.fails(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "old"})
		if got := e.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": a["dispatch_id"], "ack_nonce": "new"}); got["acked"] != true {
			t.Fatal(got)
		}
	})
	t.Run("next by state", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		id := e.review("claude")
		dispatch := e.call(map[string]any{"action": "approve", "flow_id": id})["dispatch_id"].(string)
		got := e.call(map[string]any{"action": "get", "flow_id": id})
		if got["recoverable"] != true || got["next"] != usecases.NextRecoverable || got["phase"] != "approved" {
			t.Fatal("get must be read-only and recoverable", got)
		}
		r := e.call(map[string]any{"action": "resume", "flow_id": id})
		if r["phase"] != "review" || r["recovered"] != true || r["next"] != usecases.NextRecoverable {
			t.Fatal(r)
		}
		e.call(map[string]any{"action": "approve", "flow_id": id})
		release, _, _ := persistence.NewSocraticaDispatchLock(e.root).TryAcquire(id)
		if _, err := e.repo.ClaimDispatch(context.Background(), id, dispatch, "local", time.Now()); err != nil {
			t.Fatal(err)
		}
		inflight := e.call(map[string]any{"action": "get", "flow_id": id})
		release()
		uncertain := e.call(map[string]any{"action": "get", "flow_id": id})
		for _, out := range []map[string]any{inflight, uncertain} {
			if strings.Contains(out["next"].(string), "Ejecutar") {
				t.Fatal("offered Ejecutar", out)
			}
		}
		if inflight["next"] != usecases.NextInFlight || uncertain["next"] != usecases.NextUncertain {
			t.Fatal(inflight, uncertain)
		}
	})
	t.Run("settings are read-only", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		s := e.call(map[string]any{"action": "settings", "client": "claude"})
		if s["interception"] != false {
			t.Fatal(s)
		}
		for _, action := range []string{"on", "off", "set_interception"} {
			e.fails(map[string]any{"action": action, "client": "claude"})
		}
	})
	t.Run("scrutiny is persisted", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		id := e.call(map[string]any{"action": "start", "client": "codex", "mode": "scrutiny", "prompt": "migrate to X", "questions": []any{map[string]any{"text": "Why?", "options": []string{"A", "B"}}}, "scrutiny": map[string]any{"restatement": "move to X", "assumptions": []any{map[string]any{"text": "faster", "kind": "hypothesis"}}}})["flow_id"]
		got := e.call(map[string]any{"action": "get", "flow_id": id})
		sc, _ := got["scrutiny"].(map[string]any)
		if sc["restatement"] != "move to X" || got["dispatch_id"] != "" {
			t.Fatal(got)
		}
	})
	t.Run("unavailable without memory", func(t *testing.T) {
		root := t.TempDir()
		cs := connectMCPTestSession(t, &Deps{Root: root, Project: "p"}, root, "p")
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "socratica_flow", Arguments: map[string]any{"action": "get", "flow_id": "x"}})
		if err != nil || !strings.Contains(toolText(res), `"unavailable"`) {
			t.Fatal(err, toolText(res))
		}
	})
	t.Run("private text never served", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		res, err := e.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "socratica_flow", Arguments: map[string]any{"action": "start", "client": "claude", "prompt": "deploy <private>s3cr3t-value</private>"}})
		if err != nil || strings.Contains(toolText(res), "s3cr3t-value") {
			t.Fatal(err, toolText(res))
		}
	})
	t.Run("resume of generation phases never calls the generator", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		id := e.call(map[string]any{"action": "start", "client": "claude", "prompt": "original"})["flow_id"]
		r := e.call(map[string]any{"action": "resume", "flow_id": id})
		if r["phase"] != "captured" || !strings.Contains(r["next"].(string), "preguntas") {
			t.Fatal(r)
		}
		e.call(map[string]any{"action": "answer", "flow_id": id, "questions": []any{}})
		r = e.call(map[string]any{"action": "resume", "flow_id": id})
		if r["phase"] != "rewriting" || !strings.Contains(r["next"].(string), "propuesta") || e.gen.calls != 0 {
			t.Fatal(r, e.gen.calls)
		}
	})
	t.Run("private ack redacts the row", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, func(u *usecases.SocraticaFlow) { u.Settings.Private = true })
		id := e.review("claude")
		dispatch := e.call(map[string]any{"action": "approve", "flow_id": id})["dispatch_id"]
		e.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n"})
		got := e.call(map[string]any{"action": "get", "flow_id": id})
		if got["original"] != "" || got["approved_prompt"] != "" || got["dispatch_id"] != dispatch {
			t.Fatal(got)
		}
		if rows := e.call(map[string]any{"action": "history", "client": "claude"})["history"].([]any); len(rows) != 0 {
			t.Fatal("private mode wrote history", rows)
		}
	})
	t.Run("invalid setting is an MCP error", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, func(u *usecases.SocraticaFlow) {
			u.SettingsError = &domain.ErrInvalidSocraticaSettings{Key: "socratica_history_limit", Value: 0}
		})
		if msg := e.fails(map[string]any{"action": "settings", "client": "claude"}); !strings.Contains(msg, "socratica_history_limit") {
			t.Fatal(msg)
		}
	})
	t.Run("model payload is validated", func(t *testing.T) {
		e := newSocraticaMCPEnv(t, nil)
		q := func(text string, n int) map[string]any {
			opts := []string{}
			for i := 0; i < n; i++ {
				opts = append(opts, string(rune('A'+i)))
			}
			return map[string]any{"text": text, "options": opts}
		}
		out := e.call(map[string]any{"action": "start", "client": "claude", "prompt": "original", "questions": []any{q("one", 1), q("a", 5), q("b", 2), q("c", 2), q("d", 2)}})
		qs := out["questions"].([]any)
		if len(qs) != 3 || len(qs[0].(map[string]any)["options"].([]any)) != 4 || qs[0].(map[string]any)["text"] != "a" {
			t.Fatal(qs)
		}
		e2 := newSocraticaMCPEnv(t, nil)
		id := e2.call(map[string]any{"action": "start", "client": "claude", "prompt": "original"})["flow_id"]
		e2.fails(map[string]any{"action": "answer", "flow_id": id, "questions": []any{q("", 2)}})
		got := e2.call(map[string]any{"action": "get", "flow_id": id})
		if got["phase"] != "failed" || got["original"] != "original" {
			t.Fatal(got)
		}
	})
}

// TestSocraticaSkill_SoloHerramientasRegistradas (T022): la habilidad no puede
// nombrar herramientas MCP que el servidor no registra. Los identificadores en
// minúsculas con guion bajo son herramientas o campos de socratica_flow.
func TestSocraticaSkill_SoloHerramientasRegistradas(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "infrastructure", "templates", "socratica", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	e := newSocraticaMCPEnv(t, nil)
	tools, err := e.cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{
		// Campos de entrada y salida de socratica_flow (contracts/mcp-tool.md).
		"approved_prompt": true, "dispatch_id": true, "ack_nonce": true, "already_dispatched": true, "in_flight": true,
		// Octopus solo se registra con Octopus activo; la habilidad lo nombra de forma condicional (G7).
		"octopus_route_task": true,
	}
	for _, tool := range tools.Tools {
		known[tool.Name] = true
	}
	for _, m := range regexp.MustCompile("`([a-z]+(?:_[a-z]+)+)").FindAllStringSubmatch(string(raw), -1) {
		if !known[m[1]] {
			t.Errorf("la habilidad nombra %q, que no es una herramienta registrada ni un campo de socratica_flow", m[1])
		}
	}
	if !strings.Contains(string(raw), "Si Octopus está activo") {
		t.Error("octopus_route_task debe mencionarse solo de forma condicional")
	}
}
