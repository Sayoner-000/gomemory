package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"corto", 10, "corto"},
		{"aañ", 2, "aa"},
		{"aañ", 3, "aa"},
		{"aañ", 4, "aañ"},
		{strings.Repeat("ñ", 3), 5, "ññ"},
	}
	for _, c := range cases {
		got := TruncateUTF8(c.in, c.max)
		if got != c.want || !utf8.ValidString(got) || len(got) > c.max {
			t.Errorf("TruncateUTF8(%q, %d) = %q, quiero %q", c.in, c.max, got, c.want)
		}
	}
}
