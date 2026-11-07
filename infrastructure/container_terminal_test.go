package main

import (
	"testing"

	"mem/adapters/secondary/hostbind"
)

// HCD-048 (C-001): sin id de hook ni enlace del host, la CLI y la TUI usan la
// sesión común del proyecto, como el MCP y los hooks, tengan TTY o no. Solo
// GOMEMORY_TERMINAL les da una identidad propia, y el MCP y los hooks la
// ignoran.
func TestConversationFor(t *testing.T) {
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	current := func(channel string) string {
		conv, _ := conversationFor(channel, "hcd048-proyecto")
		return conv.Current()
	}

	t.Setenv(hostbind.TerminalEnv, "")
	for _, ch := range []string{"cli", "tui", "mcp", "hook"} {
		if got := current(ch); got != "" {
			t.Errorf("C-02: canal %s sin %s = %q; quería la sesión común (\"\")", ch, hostbind.TerminalEnv, got)
		}
	}

	t.Setenv(hostbind.TerminalEnv, "pane-1")
	for ch, want := range map[string]string{"cli": "terminal-pane-1", "tui": "terminal-pane-1", "mcp": "", "hook": ""} {
		if got := current(ch); got != want {
			t.Errorf("C-03: canal %s con %s=pane-1 = %q; quería %q", ch, hostbind.TerminalEnv, got, want)
		}
	}
}
