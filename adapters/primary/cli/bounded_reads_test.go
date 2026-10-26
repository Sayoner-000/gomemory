package cli

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

type oneByteSeeker struct {
	io.Reader
	s io.Seeker
}

func (o oneByteSeeker) Seek(off int64, whence int) (int64, error) { return o.s.Seek(off, whence) }

// TestTailOf_LecturaCorta: un Read que devuelve menos bytes no puede truncar la
// cola en silencio (H-03).
func TestTailOf_LecturaCorta(t *testing.T) {
	content := "primera\nsegunda línea\ntercera línea completa\n"
	base := bytes.NewReader([]byte(content))
	r := oneByteSeeker{Reader: iotest.OneByteReader(base), s: base}
	got, err := tailOf(r, int64(len(content)), 30)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "tercera línea completa\n" {
		t.Fatalf("cola truncada: %q", got)
	}
	if _, err := tailOf(oneByteSeeker{Reader: iotest.ErrReader(errors.New("disco")), s: base}, 10, 30); err == nil {
		t.Fatal("un error de lectura no puede ignorarse")
	}
}
