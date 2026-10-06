package console

import "testing"

func TestTheme(t *testing.T) {
	for _, tc := range []struct{ mode, bg, want string }{
		{"dark", "0;15", "dark"}, {"light", "15;0", "light"},
		{"auto", "0;15", "light"}, {"", "0;7", "light"},
		{"auto", "15;0", "dark"}, {"", "", "dark"}, {"bad", "invalid", "dark"},
	} {
		p := Theme(func(k string) string {
			if k == "GOMEMORY_THEME" {
				return tc.mode
			}
			return tc.bg
		})
		if p.Name != tc.want {
			t.Errorf("%q/%q: got %s, want %s", tc.mode, tc.bg, p.Name, tc.want)
		}
	}
	if Theme(nil).Name != "dark" {
		t.Fatal("nil environment should use dark")
	}
}
