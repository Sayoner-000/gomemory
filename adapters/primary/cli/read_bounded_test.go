package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// zeroReader produce bytes sin fin sin reservar memoria: si readBounded no
// acotara la lectura, el test agotaría la memoria en vez de fallar limpio.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { return len(p), nil }

func TestReadBounded_PayloadDesmedidoEsErrorLimpio(t *testing.T) {
	for _, r := range []io.Reader{
		io.LimitReader(zeroReader{}, 16<<20),
		zeroReader{},
	} {
		data, err := readBounded(r, hookStdinMax)
		if !errors.Is(err, errPayloadTooLarge) || data != nil {
			t.Fatalf("quiero errPayloadTooLarge sin datos, obtuve %d bytes, %v", len(data), err)
		}
	}
}

func TestReadBounded_PayloadNormalIntacto(t *testing.T) {
	payload := `{"hook_event_name":"Stop"}`
	data, err := readBounded(strings.NewReader(payload), hookStdinMax)
	if err != nil || string(data) != payload {
		t.Fatalf("readBounded = %q, %v", data, err)
	}
	exacto := bytes.Repeat([]byte("a"), 1024)
	if data, err := readBounded(bytes.NewReader(exacto), 1024); err != nil || len(data) != 1024 {
		t.Fatalf("en el tope exacto = %d bytes, %v", len(data), err)
	}
}
