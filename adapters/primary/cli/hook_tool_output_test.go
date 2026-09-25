package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fakeCompress(s string) (string, bool) {
	if len(s) < 50 {
		return "", false
	}
	return "COMPRIMIDO ⟦mem⟧ · ref=abcdefabcdef", true
}

func TestRewriteToolOutputClaudeBashShape(t *testing.T) {
	in := `{"tool_name":"Bash","tool_response":{"stdout":"` + strings.Repeat("x", 80) + `","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`
	out := rewriteToolOutput("claude", []byte(in), fakeCompress)
	var got struct {
		HSO struct {
			Event string         `json:"hookEventName"`
			Upd   map[string]any `json:"updatedToolOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("salida inválida: %s", out)
	}
	if got.HSO.Event != "PostToolUse" || got.HSO.Upd["stdout"] != "COMPRIMIDO ⟦mem⟧ · ref=abcdefabcdef" {
		t.Errorf("salida inesperada: %s", out)
	}
	for _, k := range []string{"stderr", "interrupted", "isImage", "noOutputExpected"} {
		if _, ok := got.HSO.Upd[k]; !ok {
			t.Errorf("H2: se perdió la clave %s", k)
		}
	}
	if got.HSO.Upd["interrupted"] != false {
		t.Error("H2: los tipos no string no deben cambiar")
	}
}

func TestRewriteToolOutputExclusionsAndCodex(t *testing.T) {
	big := strings.Repeat("y", 80)
	for _, name := range []string{"Read", "Edit", "Write", "MultiEdit", "NotebookEdit", "mcp__gomemory__get_context"} {
		in := `{"tool_name":"` + name + `","tool_response":{"output":"` + big + `"}}`
		if out := rewriteToolOutput("claude", []byte(in), fakeCompress); out != nil {
			t.Errorf("H3: %s no debe reescribirse", name)
		}
	}
	shell := `{"tool_name":"shell","tool_response":{"output":"` + big + `"}}`
	if out := rewriteToolOutput("codex", []byte(shell), fakeCompress); out != nil {
		t.Error("Codex solo admite reescribir herramientas MCP")
	}
	mcp := `{"tool_name":"mcp__otro__x","tool_response":{"content":[{"type":"text","text":"` + big + `"}],"isError":false}}`
	out := rewriteToolOutput("codex", []byte(mcp), fakeCompress)
	if !strings.Contains(string(out), `"updatedMCPToolOutput"`) || !strings.Contains(string(out), "COMPRIMIDO") {
		t.Errorf("Codex MCP: %s", out)
	}
	if out := rewriteToolOutput("claude", []byte(`no es json`), fakeCompress); out != nil {
		t.Error("H1: una entrada inválida no produce salida")
	}
	small := `{"tool_name":"Bash","tool_response":{"stdout":"poco"}}`
	if out := rewriteToolOutput("claude", []byte(small), fakeCompress); out != nil {
		t.Error("sin compresión no hay salida")
	}
}

func TestRewriteToolOutputOpenCode(t *testing.T) {
	out := rewriteToolOutput("opencode", []byte(`{"tool":"bash","output":"`+strings.Repeat("z", 80)+`"}`), fakeCompress)
	if string(out) != `{"output":"COMPRIMIDO ⟦mem⟧ · ref=abcdefabcdef"}` {
		t.Errorf("OpenCode: %s", out)
	}
	if out := rewriteToolOutput("opencode", []byte(`{"tool":"read","output":"`+strings.Repeat("z", 80)+`"}`), fakeCompress); out != nil {
		t.Error("read de OpenCode excluido")
	}
}

func TestWithBudget(t *testing.T) {
	start := time.Now()
	if r := withBudget(20*time.Millisecond, func() []byte { time.Sleep(time.Second); return []byte("tarde") }); r != nil {
		t.Error("H1: si vence el plazo, no hay salida")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Error("el plazo debe respetarse")
	}
	if r := withBudget(time.Second, func() []byte { panic("x") }); r != nil {
		t.Error("un pánico no produce salida")
	}
}
