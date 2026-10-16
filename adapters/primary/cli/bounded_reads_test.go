package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// TestSocraticaRefine_StdinAcotado cierra H-03 (auditoría 2026-10-16): `refine -`
// no acumula un stdin sin límite; por encima del tope falla limpio.
func TestSocraticaRefine_StdinAcotado(t *testing.T) {
	e := newCLIEnv(t)
	big := strings.Repeat("a", socraticaPromptMax+1)
	var errOut bytes.Buffer
	code := runSocratica(e.deps, []string{"refine", "-"}, socraticaIO{in: bufio.NewReader(strings.NewReader(big)), out: io.Discard, err: &errOut})
	if code != socraticaExitUsage || !strings.Contains(errOut.String(), errPayloadTooLarge.Error()) {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
	if e.gen.seen != nil {
		t.Fatal("llamó al generador con una entrada fuera del tope")
	}
}

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
