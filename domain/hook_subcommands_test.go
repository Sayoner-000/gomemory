package domain

import "testing"

func TestIsGomemoryHookSubcommand(t *testing.T) {
	cases := []struct {
		sub  string
		want bool
	}{
		{"session-start", true},
		{"pre-compact", true},
		{"tool-output", true},
		{"expand", true},
		{"", false},
		{"expand-x", false},
		{"whatever", false},
		{"Session-Start", false},
	}
	for _, c := range cases {
		if got := IsGomemoryHookSubcommand(c.sub); got != c.want {
			t.Errorf("IsGomemoryHookSubcommand(%q) = %v, want %v", c.sub, got, c.want)
		}
	}
}

func TestGomemoryHookSubcommandsHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range GomemoryHookSubcommands {
		if seen[s] {
			t.Errorf("subcomando duplicado en el registro: %q", s)
		}
		seen[s] = true
	}
}
