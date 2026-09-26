package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

const ctxDeArranque = "# Memoria del Proyecto\n\n- una memoria\n"

// Contrato hook-session-start: sin avisos, la salida es exactamente la de
// v2.26.4 (el contexto en texto plano) en todos los dialectos.
func TestRenderSessionStart_SinAvisosIdenticoAHoy(t *testing.T) {
	for _, d := range []hookDialect{dialectClaude, dialectJSON, dialectNeutral, dialectText} {
		if got := renderSessionStart(d, ctxDeArranque, nil); got != ctxDeArranque {
			t.Errorf("%s sin avisos cambió la salida: %q", d, got)
		}
	}
}

// Claude: el aviso va en systemMessage (lo ve la persona) y el contexto en
// additionalContext (lo ve el modelo).
func TestRenderSessionStart_ClaudeConAviso(t *testing.T) {
	out := renderSessionStart(dialectClaude, ctxDeArranque, []string{"aviso uno", "aviso dos"})
	var got struct {
		SystemMessage      string `json:"systemMessage"`
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("salida no es JSON: %v\n%s", err, out)
	}
	if got.SystemMessage != "aviso uno\naviso dos" {
		t.Errorf("systemMessage = %q", got.SystemMessage)
	}
	if got.HookSpecificOutput.HookEventName != "SessionStart" || got.HookSpecificOutput.AdditionalContext != ctxDeArranque {
		t.Errorf("hookSpecificOutput = %+v", got.HookSpecificOutput)
	}
}

// Sin contexto (proyecto sin memoria) el aviso sigue llegando a la persona.
func TestRenderSessionStart_ClaudeSoloAviso(t *testing.T) {
	out := renderSessionStart(dialectClaude, "", []string{"aviso"})
	if strings.Contains(out, "hookSpecificOutput") {
		t.Errorf("sin contexto no debe emitirse additionalContext vacío: %s", out)
	}
	if !strings.Contains(out, `"systemMessage":"aviso"`) {
		t.Errorf("falta el aviso: %s", out)
	}
}

func TestRenderSessionStart_JSONConAviso(t *testing.T) {
	out := renderSessionStart(dialectJSON, ctxDeArranque, []string{"aviso"})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("salida no es JSON: %v", err)
	}
	if got["systemMessage"] != "aviso" {
		t.Errorf("systemMessage = %v", got["systemMessage"])
	}
}

// Dialectos de un solo canal: el aviso se antepone como instrucción al agente.
func TestRenderSessionStart_PlanoConAviso(t *testing.T) {
	for _, d := range []hookDialect{dialectNeutral, dialectText} {
		out := renderSessionStart(d, ctxDeArranque, []string{"aviso uno", "aviso dos"})
		want := "Informa a la persona: aviso uno\nInforma a la persona: aviso dos\n\n" + ctxDeArranque
		if out != want {
			t.Errorf("%s = %q; quiero %q", d, out, want)
		}
	}
}

// R3: el payload de SessionStart no distingue a Claude (no trae tool_name);
// la señal es CLAUDE_PROJECT_DIR, que Claude Code exporta a sus hooks. --emit
// manda sobre todo.
func TestSessionStartDialect(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "CLAUDE_PROJECT_DIR" {
				return v
			}
			return ""
		}
	}
	cases := []struct {
		name   string
		args   []string
		claude string
		want   hookDialect
	}{
		{"Claude Code", nil, "/p", dialectClaude},
		{"otro agente", nil, "", dialectNeutral},
		{"--emit manda", []string{"--emit=text"}, "/p", dialectText},
		{"--emit=json", []string{"--emit=json"}, "", dialectJSON},
	}
	for _, c := range cases {
		if got := sessionStartDialect(c.args, env(c.claude)); got != c.want {
			t.Errorf("%s: %s; quiero %s", c.name, got, c.want)
		}
	}
}
