package cli

import (
	"regexp"
	"strings"
	"testing"
)

// emojiRe reconoce los emojis que HCD-022 retira de la salida humana:
// pictogramas, el selector de variante y los emojis del plano básico en uso.
var emojiRe = regexp.MustCompile("[\U0001F300-\U0001FAFF️✅❌➖ℹ✕♥]")

// HCD-022 C-01/C-02: setup-mcp (global y por proyecto) informa cada resultado
// con los símbolos de console; fuera de TTY salen limpios y sin emojis.
func TestSetupMCPGlobal_SimbolosSinEmoji(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PATH", t.TempDir())
	out := captureStdout(t, func() {
		CmdMCPSetup(nil, []string{"--scope", "global", "--agents", "claude,codex,agy,cursor"})
		CmdMCPSetup(nil, []string{"--scope", "project", "--target", t.TempDir(), "--agents", "codex,desconocido"})
	})
	if loc := emojiRe.FindStringIndex(out); loc != nil {
		t.Fatalf("la salida contiene un emoji (%q):\n%s", out[loc[0]:loc[1]], out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("fuera de TTY no debe haber ANSI:\n%s", out)
	}
	for _, want := range []string{"✓ ", "⚠ ", "○ "} {
		if !strings.Contains(out, want) {
			t.Errorf("falta el símbolo %q en la salida:\n%s", want, out)
		}
	}
}
