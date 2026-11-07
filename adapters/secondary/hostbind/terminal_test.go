package hostbind

import "testing"

// HCD-048 (C-001): la identidad de terminal solo la fija GOMEMORY_TERMINAL.
// El TTY del proceso ya no cuenta: con TTY o sin él, sin la variable no hay
// identidad y la CLI comparte la sesión común del proyecto.
func TestTerminalID(t *testing.T) {
	cases := []struct {
		env, want string
	}{
		{"", ""},
		{"   ", ""},
		{"pane-1", "terminal-pane-1"},
		{" pane-1 ", "terminal-pane-1"},
	}
	for _, c := range cases {
		t.Setenv(TerminalEnv, c.env)
		if got := TerminalID(); got != c.want {
			t.Errorf("TerminalID() con %s=%q = %q; quería %q", TerminalEnv, c.env, got, c.want)
		}
	}
}
