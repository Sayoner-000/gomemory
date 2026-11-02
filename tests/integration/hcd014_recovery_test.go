package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression/native"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/requestcompression"
	"mem/application/ports"
	"mem/application/usecases"
)

func hcd014Log() string {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "2026-11-02 10:00:%02d INFO worker_%03d heartbeat completed successfully job=%d\n", i%60, i, i)
	}
	return b.String()
}

func hcd014Request(text string) []byte {
	b, _ := json.Marshal(map[string]any{"input": []any{
		map[string]any{"type": "function_call", "call_id": "c1", "name": "exec_command", "arguments": `{"cmd":"go test ./..."}`},
		map[string]any{"type": "function_call_output", "call_id": "c1", "output": text},
	}})
	return b
}

// Persistence already precedes publication in the native engine. This test
// exercises that invariant through the new request adapter and a reopened DB.
func TestHCD014PersistBeforeDelivery(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	store := persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	original := hcd014Log()
	body := hcd014Request(original)
	out := hcd014Capture(t, body, func(b []byte) []byte {
		return requestcompression.Transform(b, "responses", native.NewEngine(store, nil, nil, "p"))
	})
	if bytes.Equal(out, body) {
		t.Fatal("fixture did not compress")
	}
	rows, err := db.Query("SELECT ref FROM compression_originals")
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()
	if len(refs) != 1 || !bytes.Contains(out, []byte(refs[0])) {
		t.Fatalf("delivered references do not match persisted originals: %v", refs)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store = persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	got, _, err := usecases.RetrieveOriginal(context.Background(), store, "p", refs[0])
	if err != nil || sha256.Sum256([]byte(got)) != sha256.Sum256([]byte(original)) {
		t.Fatalf("recovery after reopen differs: %v", err)
	}
}

type hcd014FailedStore struct {
	ports.OriginalStoreRepository
	err error
}

func (s hcd014FailedStore) Put(context.Context, string, string, string, string) (string, error) {
	return "", s.err
}

type hcd014CompressorFunc func(string, ports.CompressionOptions) (ports.CompressionResult, error)

func (f hcd014CompressorFunc) Compress(s string, o ports.CompressionOptions) (ports.CompressionResult, error) {
	return f(s, o)
}

func TestHCD014AutomaticFailurePassthrough(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store := persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	for name, compressor := range map[string]ports.Compressor{
		"write_failure":   native.NewEngine(hcd014FailedStore{store, errors.New("synthetic write failure")}, nil, nil, "p"),
		"storage_timeout": native.NewEngine(hcd014FailedStore{store, context.DeadlineExceeded}, nil, nil, "p"),
		"compressor_failure": hcd014CompressorFunc(func(string, ports.CompressionOptions) (ports.CompressionResult, error) {
			return ports.CompressionResult{}, errors.New("synthetic compression error")
		}),
		"missing_reference": hcd014CompressorFunc(func(string, ports.CompressionOptions) (ports.CompressionResult, error) {
			return ports.CompressionResult{Content: "short", Compressed: true}, nil
		}),
	} {
		t.Run(name, func(t *testing.T) {
			body := hcd014Request(hcd014Log())
			got := requestcompression.Transform(body, "responses", compressor)
			if !bytes.Equal(got, body) {
				t.Fatal("failure published a replacement")
			}
		})
	}
	// Policy-protected material remains available to the host but is never
	// copied into a recoverable original by automatic compression.
	body := hcd014Request(strings.Repeat("<private>synthetic protected text</private>\n", 400))
	if got := requestcompression.Transform(body, "responses", native.NewEngine(store, nil, nil, "p")); !bytes.Equal(got, body) {
		t.Fatal("private fallback altered host content")
	}
	_, count, err := store.Usage(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("failed/private paths persisted originals: count=%d err=%v", count, err)
	}
}
