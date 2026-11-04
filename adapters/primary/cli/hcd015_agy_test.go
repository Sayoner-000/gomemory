package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/primary/setup"
	"mem/domain"
)

func agyFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../tests/fixtures/hcd015/events.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Events map[string]json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Events[name]
}

func TestHCD015AgyInput(t *testing.T) {
	for event, want := range map[string][]string{
		"SessionStart":  {"session-start --emit=agy", "expand session-start --emit=agy"},
		"PreInvocation": {"user-prompt-submit --emit=agy", "expand prompt --emit=agy"},
		"PreToolUse":    {"expand pre-tool --emit=agy"},
		"PostToolUse":   {"expand post-tool --emit=agy"},
		"Stop":          {"turn-end --emit=agy", "expand stop --emit=agy"},
	} {
		plan := planAgyHook(event, agyFixture(t, event))
		if plan.dir != "/workspace/project" {
			t.Errorf("%s: proyecto %q", event, plan.dir)
		}
		if len(plan.calls) != len(want) {
			t.Fatalf("%s: %d llamadas, se esperaban %d", event, len(plan.calls), len(want))
		}
		for i, call := range plan.calls {
			if got := strings.Join(call.args, " "); got != want[i] {
				t.Errorf("%s: llamada %q, se esperaba %q", event, got, want[i])
			}
			var payload map[string]any
			if err := json.Unmarshal(call.stdin, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["session_id"] != "00000000-0000-4000-8000-000000000015" {
				t.Errorf("%s: session_id %v", event, payload["session_id"])
			}
		}
	}

	var pre map[string]any
	_ = json.Unmarshal(planAgyHook("PreToolUse", agyFixture(t, "PreToolUse")).calls[0].stdin, &pre)
	if pre["tool_name"] != "Bash" || pre["tool_input"].(map[string]any)["command"] != "printf HCD015_OK" {
		t.Errorf("herramienta mal traducida: %v", pre)
	}
	var prompt map[string]any
	_ = json.Unmarshal(planAgyHook("PreInvocation", agyFixture(t, "PreInvocation")).calls[0].stdin, &prompt)
	if prompt["prompt"] != "fixture" {
		t.Errorf("prompt mal traducido: %v", prompt)
	}
	// agy repite PreInvocation por cada llamada al modelo; el prompt es uno solo.
	later := strings.Replace(string(agyFixture(t, "PreInvocation")), `"invocationNum": 0`, `"invocationNum": 1`, 1)
	if calls := planAgyHook("PreInvocation", []byte(later)).calls; len(calls) != 0 {
		t.Errorf("una invocación posterior repite el prompt: %v", calls)
	}
}

func TestHCD015AgyStopRecordsTurnActivity(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "transcript_full.jsonl")
	lines := []string{
		`{"step_index":0,"type":"USER_INPUT","content":"turno anterior"}`,
		`{"step_index":1,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"run_command","args":{"CommandLine":"go build"}}]}`,
		`{"step_index":2,"type":"GENERIC","content":"Command exited with code 0."}`,
		`{"step_index":3,"type":"USER_INPUT","content":"turno actual"}`,
		`{"step_index":4,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"run_command","args":{"CommandLine":"go test ./..."}},{"name":"write_file","args":{"Path":"/workspace/a.go"}}]}`,
		`{"step_index":5,"type":"GENERIC","content":"Command exited with code 0."}`,
		`{"step_index":6,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git reset --hard"}}]}`,
		`{"step_index":7,"type":"GENERIC","error":"denied","content":"Encountered error in step execution"}`,
	}
	if err := os.WriteFile(transcript, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(agyFixture(t, "Stop"), &event); err != nil {
		t.Fatal(err)
	}
	event["transcriptPath"] = transcript
	raw, _ := json.Marshal(event)
	var payload struct {
		Files      []string `json:"files"`
		Commands   []string `json:"commands"`
		Transcript string   `json:"transcript_path"`
	}
	if err := json.Unmarshal(planAgyHook("Stop", raw).calls[0].stdin, &payload); err != nil {
		t.Fatal(err)
	}
	if strings.Join(payload.Commands, "|") != "go test ./..." || strings.Join(payload.Files, "|") != "/workspace/a.go" || payload.Transcript != "" {
		t.Errorf("actividad del turno: %+v", payload)
	}
}

func TestAgyCleanArgQuotes(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "transcript_full.jsonl")
	lines := []string{
		`{"step_index":0,"type":"USER_INPUT","content":"turno actual"}`,
		`{"step_index":1,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"run_command","args":{"CommandLine":"\"go test ./...\""}},{"name":"write_file","args":{"Path":"\"/workspace/a.go\""}}]}`,
		`{"step_index":2,"type":"GENERIC","content":"Command exited with code 0."}`,
	}
	if err := os.WriteFile(transcript, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, commands := agyTurnActivity(transcript)
	if len(files) != 1 || files[0] != "/workspace/a.go" {
		t.Fatalf("files esperado [/workspace/a.go], obtenido: %v", files)
	}
	if len(commands) != 1 || commands[0] != "go test ./..." {
		t.Fatalf("commands esperado [go test ./...], obtenido: %v", commands)
	}
	if filepath.Ext(files[0]) != ".go" {
		t.Fatalf("filepath.Ext fallo: %q", filepath.Ext(files[0]))
	}
}

func TestHCD015AgyOutput(t *testing.T) {
	envelope := `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"contexto real"}}`
	got := string(agyHookResponse("PreInvocation", agyFixture(t, "PreInvocation"), func(string, agyCall) (string, error) { return envelope, nil }))
	if strings.Contains(got, "hookSpecificOutput") || !strings.Contains(got, "contexto real") {
		t.Errorf("el sobre de Claude llega al modelo: %s", got)
	}

	reply := func(out string) agyRunner {
		return func(string, agyCall) (string, error) { return out, nil }
	}
	cases := []struct{ event, stdout, want string }{
		{"SessionStart", "contexto", `{"injectSteps":[{"systemMessage":{"systemMessage":"contexto\n\ncontexto"}}]}`},
		{"PreInvocation", "aviso", `{"injectSteps":[{"ephemeralMessage":"aviso\n\naviso"}]}`},
		{"PreInvocation", "", `{}`},
		{"PreToolUse", "", `{"decision":"allow"}`},
		{"PreToolUse", "BLOCK: peligroso", `{"decision":"deny","reason":"peligroso"}`},
		{"PostToolUse", "ignorado", `{}`},
		{"Stop", "ignorado", `{}`},
	}
	for _, c := range cases {
		if got := string(agyHookResponse(c.event, agyFixture(t, c.event), reply(c.stdout))); got != c.want {
			t.Errorf("%s con %q: %s", c.event, c.stdout, got)
		}
	}
}

func TestHCD015AgyFailOpen(t *testing.T) {
	fail := func(string, agyCall) (string, error) { return "", errors.New("mem no disponible") }
	for _, c := range []struct {
		event string
		raw   []byte
		want  string
	}{
		{"SessionStart", agyFixture(t, "SessionStart"), `{}`},
		{"PreToolUse", agyFixture(t, "PreToolUse"), `{"decision":"allow"}`},
		{"PreToolUse", []byte("no es json"), `{"decision":"allow"}`},
		{"Stop", agyFixture(t, "Stop"), `{}`},
		{"EventoNuevo", []byte(`{}`), `{}`},
	} {
		got := agyHookResponse(c.event, c.raw, fail)
		if !json.Valid(got) || string(got) != c.want {
			t.Errorf("%s: %s", c.event, got)
		}
	}
	panics := func(string, agyCall) (string, error) { panic("fallo") }
	if got := string(agyHookResponse("PreToolUse", agyFixture(t, "PreToolUse"), panics)); got != `{"decision":"allow"}` {
		t.Errorf("un pánico bloquea el turno: %s", got)
	}
}

func TestHCD015DoctorAgy(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gemini", "config", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	ref := setup.AgentRef{HookCommand: "mem", MCPCommand: "mem", MCPArgs: []string{"mcp"}}
	if got := setup.InspectAgyEntry(home, "1.4.12"); got.State != domain.StateNotApplicable {
		t.Errorf("sin instalar: %v", got)
	}
	if err := os.WriteFile(path, []byte(`{"gomemory-lifecycle":{"Stop":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := setup.InspectAgyEntry(home, "1.4.12"); got.State != domain.StateOutdated {
		t.Errorf("grupo manual: %v", got)
	}
	if _, err := setup.InstallAgyGlobal(home, ref, time.Now()); err != nil {
		t.Fatal(err)
	}
	ok := setup.InspectAgyEntry(home, "1.4.12")
	if ok.State != domain.StateOK || !strings.Contains(ok.Detail, "sin compresión automática") || !strings.Contains(ok.Detail, "tras compactar") {
		t.Errorf("instalado: %v", ok)
	}
	if got := setup.InspectAgyEntry(home, "9.9.9"); got.State != domain.StateOutdated || !strings.Contains(got.Detail, "9.9.9") {
		t.Errorf("versión no comprobada: %v", got)
	}
	if !strings.Contains(toolOutputHookStates(t.TempDir(), true)["agy"], "no disponible") {
		t.Error("doctor debe declarar que agy no comprime salidas")
	}
}

func TestHCD015UninstallAgy(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".gemini", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), []byte(`{"ajeno":{"Stop":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mcp_config.json"), []byte(`{"mcpServers":{"ajeno":{"command":"otro"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := setup.AgentRef{HookCommand: "mem", MCPCommand: "mem", MCPArgs: []string{"mcp"}}
	if _, err := setup.InstallAgyGlobal(home, ref, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.UninstallAgyGlobal(home, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	hooks, _ := os.ReadFile(filepath.Join(dir, "hooks.json"))
	mcp, _ := os.ReadFile(filepath.Join(dir, "mcp_config.json"))
	if strings.Contains(string(hooks), "gomemory") || strings.Contains(string(mcp), "gomemory") {
		t.Errorf("quedan entradas de gomemory: %s %s", hooks, mcp)
	}
	if !strings.Contains(string(hooks), "ajeno") || !strings.Contains(string(mcp), "ajeno") {
		t.Errorf("se perdió lo ajeno: %s %s", hooks, mcp)
	}
	covered := map[domain.ChannelKind]bool{}
	for _, cell := range domain.CellsForActivity(domain.ActivityUninstallGlobal) {
		if cell.Agent == "agy" {
			covered[cell.Kind] = true
		}
	}
	if !covered[domain.KindServerConfig] || !covered[domain.KindLifecycleHook] {
		t.Errorf("la retirada global no cubre agy: %v", covered)
	}
}
