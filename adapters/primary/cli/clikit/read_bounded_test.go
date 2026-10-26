package clikit

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReadBounded(t *testing.T) {
	t.Run("accepts limit exactly", func(t *testing.T) {
		got, err := ReadBounded(strings.NewReader("1234"), 4)
		if err != nil || string(got) != "1234" {
			t.Fatalf("got %q, %v", got, err)
		}
	})

	t.Run("rejects excess without returning truncated content", func(t *testing.T) {
		got, err := ReadBounded(strings.NewReader("12345"), 4)
		if len(got) != 0 || !errors.Is(err, ErrPayloadTooLarge) {
			t.Fatalf("got %q, %v", got, err)
		}
	})

	t.Run("returns reader errors", func(t *testing.T) {
		_, err := ReadBounded(iotest.ErrReader(errors.New("read failed")), 4)
		if err == nil || err.Error() != "read failed" {
			t.Fatalf("got %v", err)
		}
	})
}
