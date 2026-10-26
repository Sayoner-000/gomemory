package clikit

import (
	"errors"
	"fmt"
	"io"
)

// ErrPayloadTooLarge identifies an input that exceeds its configured limit.
var ErrPayloadTooLarge = errors.New("payload excede el tope")

// ReadBounded reads at most limit+1 bytes so oversized payloads are rejected,
// never silently returned in truncated form.
func ReadBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w de %d MB", ErrPayloadTooLarge, limit>>20)
	}
	return data, nil
}
