package domain

import "testing"

// Las salidas esperadas se contrastaron con el printf y el echo de bash
// (B-001 del ACR acr_1a0a68c0).
func TestPrintfOutput(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"%s%s", "r", "m"}, "rm"},
		{[]string{"%c%c", "rr", "mm"}, "rm"},
		{[]string{`r\x6d`}, "rm"},
		{[]string{`\162\155`}, "rm"},
		{[]string{`A`}, "A"},
		{[]string{"%5s|%-4s|", "a", "b"}, "    a|b   |"},
		{[]string{"%.2s", "rmxx"}, "rm"},
		{[]string{"%*s", "3", "x"}, "  x"},
		{[]string{"%d %i %x %X %o", "10", "0x1f", "255", "255", "8"}, "10 31 ff FF 10"},
		{[]string{"%s\n", "a", "b", "c"}, "a\nb\nc\n"},
		{[]string{"x%sy", "1", "2"}, "x1yx2y"},
		{[]string{"%b", `r\0155`}, "rm"},
		{[]string{"%b-%s", `a\cb`, "z"}, "a"},
		{[]string{"%%%s", "x"}, "%x"},
		{[]string{"%c", ""}, ""},
		{[]string{"%d", "'A"}, "65"},
		{[]string{"%05d|%+d", "42", "7"}, "00042|+7"},
		{[]string{"%.3f %e %g", "3.14159", "1000", "0.5"}, "3.142 1.000000e+03 0.5"},
		{[]string{`a\tb\\c`}, "a\tb\\c"},
		{[]string{"%s"}, ""},
		{[]string{"-v", "R", "rm"}, ""},
		{[]string{"--", "%s", "rm"}, "rm"},
	}
	for _, c := range cases {
		if got, ok := printfOutput(c.args); !ok || got != c.want {
			t.Errorf("printfOutput(%q) = %q, %v; want %q", c.args, got, ok, c.want)
		}
	}
	for _, args := range [][]string{{"%q", "rm"}, {"%d", "rm"}, {"%999999999s", "x"}, {"%*s", "99999", "x"}, {"%"}, {}} {
		if got, ok := printfOutput(args); ok {
			t.Errorf("printfOutput(%q) = %q, debe ser no resoluble", args, got)
		}
	}
}

func TestEchoText(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-e", `r\x6d`}, "rm"},
		{[]string{`r\x6d`}, "rm"}, // dash y zsh interpretan sin -e.
		{[]string{"-E", `r\x6d`}, `r\x6d`},
		{[]string{"-n", "rm"}, "rm"},
		{[]string{"-ne", `a\tb`}, "a\tb"},
		{[]string{"-eE", `a\tb`}, `a\tb`},
		{[]string{"-x", "y"}, "-x y"},
		{[]string{"-e", `a\cb`}, "a"},
		{[]string{"-e", `\0101`}, "A"},
	}
	for _, c := range cases {
		if got := echoText(c.args); got != c.want {
			t.Errorf("echoText(%q) = %q, want %q", c.args, got, c.want)
		}
	}
}
