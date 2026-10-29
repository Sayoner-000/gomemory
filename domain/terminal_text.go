package domain

import (
	"strings"
	"unicode/utf8"
)

// StripTerminalControls removes C0 controls (except tab and newline), DEL and
// C1 controls, so stored text cannot drive the terminal that shows it: no
// escape sequence can set the window title, write the clipboard (OSC 52) or
// move the cursor (CWE-150). CRLF becomes LF. Printable text, including any
// Unicode, is kept; invalid UTF-8 becomes U+FFFD so a raw 8-bit C1 byte cannot
// slip through either.
func StripTerminalControls(s string) string {
	if utf8.ValidString(s) && !strings.ContainsFunc(s, isTerminalControl) {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		if isTerminalControl(r) {
			return -1
		}
		return r
	}, s)
}

func isTerminalControl(r rune) bool {
	return r != '\t' && r != '\n' && (r < 0x20 || (r >= 0x7f && r <= 0x9f))
}

// ProtectHCDField prepara un campo corto de una unidad HCD (título,
// implementador) para persistirlo o mostrarlo: redacta lo privado y los
// secretos y quita los controles de terminal.
func ProtectHCDField(s string) string {
	return strings.TrimSpace(StripTerminalControls(ProtectText(s)))
}
