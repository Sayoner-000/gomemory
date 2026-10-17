package domain

import "testing"

func TestStripTerminalControls(t *testing.T) {
	for in, want := range map[string]string{
		"texto normal\n\tcon tab · ñ 日本": "texto normal\n\tcon tab · ñ 日本",
		"\x1b]0;PWNED\x07título":         "]0;PWNEDtítulo",
		"\x1b]52;c;aGFjaw==\x07":         "]52;c;aGFjaw==",
		"rojo \x1b[31mX\x1b[0m":          "rojo [31mX[0m",
		"línea\r\notra\rpisa":            "línea\notrapisa",
		"c1 \u009b31m y del \x7f":        "c1 31m y del ",
		"byte crudo \x9b31m":             "byte crudo �31m",
	} {
		if got := StripTerminalControls(in); got != want {
			t.Errorf("StripTerminalControls(%q) = %q, want %q", in, got, want)
		}
	}
}
