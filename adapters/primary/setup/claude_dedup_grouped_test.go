package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// S-001 (acr_715249c3): una entrada que agrupa varios comandos pierde solo los
// duplicados, nunca los demás.
func TestDedupClaudeProjectHooks_EntradaAgrupadaConservaLoNoDuplicado(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	write := func(dir, body string) {
		_ = os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
		if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(home, `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"mem hook turn-end"}]}]}}`)
	write(root, `{"hooks":{"Stop":[{"matcher":"","hooks":[`+
		`{"type":"command","command":"mem hook turn-end"},`+
		`{"type":"command","command":"mem hook tool-output claude"},`+
		`{"type":"command","command":"herdr-agent-state stop"}]}]}}`)

	removed, err := DedupClaudeProjectHooks(home, root)
	if err != nil || strings.Join(removed, ",") != "turn-end" {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	got := string(data)
	if strings.Contains(got, "hook turn-end") {
		t.Error("el comando duplicado debe retirarse")
	}
	if !strings.Contains(got, "hook tool-output claude") || !strings.Contains(got, "herdr-agent-state stop") {
		t.Errorf("los comandos no duplicados de la misma entrada deben conservarse:\n%s", got)
	}
}
