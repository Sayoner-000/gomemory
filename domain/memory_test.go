package domain

import "testing"

func TestValidMemoryType(t *testing.T) {
	cases := []struct {
		in   string
		want MemoryType
	}{
		{"learning", Learning},
		{"decision", Decision},
		{"architecture", Architecture},
		{"bugfix", Bugfix},
		{"pattern", Pattern},
		{"discovery", Discovery},
		{"preference", Preference},
		{"checkpoint", Checkpoint},
		{"not-a-type", Learning},
		{"", Learning},
	}

	for _, c := range cases {
		if got := ValidMemoryType(c.in); got != c.want {
			t.Errorf("ValidMemoryType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
