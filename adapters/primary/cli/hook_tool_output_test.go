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

// HCD-053: la nota de recuperación solo se da por mostrada si la salida se
// entrega. Una ejecución que vence el presupuesto no la consume, no concede
// referencias y la siguiente la vuelve a llevar. Con stdout y stderr
// comprimibles, la nota va en un solo campo y siempre en el mismo.
func TestToolOutputHint(t *testing.T) {
	root := t.TempDir()
	const budget = 50 * time.Millisecond

	// Vence el presupuesto después de pedir la nota: la salida se descarta.
	expiredSeen := make(chan bool, 1)
	out := runToolOutputBudget(root, budget, func(hint func() string, expired func() bool) []byte {
		h := hint()
		time.Sleep(3 * budget)
		expiredSeen <- expired()
		return []byte("salida" + h)
	})
	if out != nil {
		t.Fatalf("una salida fuera de presupuesto se descarta: %q", out)
	}
	select {
	case seen := <-expiredSeen:
		if !seen {
			t.Error("expired() debe ser cierto tras el plazo: sin eso se concederían referencias que nadie ve")
		}
	case <-time.After(time.Second):
		t.Fatal("la ejecución descartada no terminó")
	}

	// La siguiente entrega sí la lleva, y solo entonces cuenta como mostrada.
	out = runToolOutputBudget(root, time.Second, func(hint func() string, _ func() bool) []byte {
		return []byte("salida" + hint())
	})
	if !strings.Contains(string(out), RetrieveHint) {
		t.Fatalf("la nota perdida por el plazo debe llegar en la siguiente entrega: %q", out)
	}
	out = runToolOutputBudget(root, time.Second, func(hint func() string, _ func() bool) []byte {
		return []byte("salida" + hint())
	})
	if strings.Contains(string(out), RetrieveHint) {
		t.Fatalf("una nota ya entregada no se repite: %q", out)
	}

	// Un solo campo lleva la nota, y siempre el mismo (orden alfabético).
	in := `{"tool_name":"Bash","tool_response":{"stdout":"` + strings.Repeat("o", 80) + `","stderr":"` + strings.Repeat("e", 80) + `"}}`
	for i := 0; i < 20; i++ {
		fresh := t.TempDir()
		out = runToolOutputBudget(fresh, time.Second, func(hint func() string, _ func() bool) []byte {
			return rewriteToolOutput("claude", []byte(in), func(s string) (string, bool) {
				c, ok := fakeCompress(s)
				return c + hint(), ok
			})
		})
		var got struct {
			HSO struct {
				Updated map[string]any `json:"updatedToolOutput"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("salida no JSON: %s", out)
		}
		stderr, _ := got.HSO.Updated["stderr"].(string)
		stdout, _ := got.HSO.Updated["stdout"].(string)
		if !strings.Contains(stderr, RetrieveHint) || strings.Contains(stdout, RetrieveHint) {
			t.Fatalf("la nota debe ir solo en stderr, el primer campo en orden alfabético: stderr=%q stdout=%q", stderr, stdout)
		}
	}
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
	// Codex rechaza siempre updatedMCPToolOutput («PostToolUse hook returned
	// unsupported updatedMCPToolOutput»): no se emite nada, ni en shell ni en MCP.
	shell := `{"tool_name":"shell","tool_response":{"output":"` + big + `"}}`
	mcp := `{"tool_name":"mcp__otro__x","tool_response":{"content":[{"type":"text","text":"` + big + `"}],"isError":false}}`
	for _, in := range []string{shell, mcp} {
		if out := rewriteToolOutput("codex", []byte(in), fakeCompress); out != nil {
			t.Errorf("Codex no admite reescribir salidas: %s", out)
		}
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
