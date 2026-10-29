package domain

import (
	"regexp"
	"testing"
)

func TestContentRevision_NormalizesLineEndingsAndTrailingSpace(t *testing.T) {
	lf := ContentRevision([]byte("uno  \ndos\n"))
	crlf := ContentRevision([]byte("uno\r\ndos\r\n"))
	if lf != crlf {
		t.Fatalf("revisiones LF/CRLF distintas: %q != %q", lf, crlf)
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(lf) {
		t.Fatalf("revisión no tiene 12 hex minúsculos: %q", lf)
	}
}
