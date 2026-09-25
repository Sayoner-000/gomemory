package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postToolUseCommands(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, g := range s.Hooks["PostToolUse"] {
		for _, h := range g.Hooks {
			out = append(out, g.Matcher+" => "+h.Command)
		}
	}
	return out
}

// T059 — el hook de salidas solo se registra con el ajuste activo, y se retira
// al apagarlo sin tocar los hooks de terceros.
func TestClaudeToolOutputHookRegistration(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".memory"), 0o755)
	write := func(v string) {
		_ = os.WriteFile(filepath.Join(root, ".memory", "settings.json"), []byte(v), 0o644)
	}
	has := func() bool {
		for _, c := range postToolUseCommands(t, root) {
			if strings.Contains(c, "hook tool-output claude") {
				if !strings.HasPrefix(c, claudeToolOutputMatcher+" =>") {
					t.Errorf("matcher inesperado: %s", c)
				}
				return true
			}
		}
		return false
	}
	ref := AgentRef{HookCommand: "mem"}

	write(`{}`)
	if err := writeClaudeHooks(root, ref); err != nil {
		t.Fatal(err)
	}
	if has() {
		t.Fatal("con el ajuste apagado no debe registrarse (huella cero)")
	}

	write(`{"tool_output_compression": true}`)
	_ = writeClaudeHooks(root, ref)
	if !has() {
		t.Fatal("con el ajuste activo debe registrarse")
	}

	// Un hook de terceros en PostToolUse sobrevive al apagado.
	path := filepath.Join(root, ".claude", "settings.json")
	data, _ := os.ReadFile(path)
	data = []byte(strings.Replace(string(data), `"PostToolUse": [`, `"PostToolUse": [{"matcher":"Bash","hooks":[{"type":"command","command":"otro-hook"}]},`, 1))
	_ = os.WriteFile(path, data, 0o644)

	write(`{}`)
	_ = writeClaudeHooks(root, ref)
	if has() {
		t.Error("al apagarlo, el hook debe retirarse")
	}
	third := false
	for _, c := range postToolUseCommands(t, root) {
		if strings.Contains(c, "otro-hook") {
			third = true
		}
	}
	if !third {
		t.Error("los hooks de terceros deben conservarse")
	}
}
