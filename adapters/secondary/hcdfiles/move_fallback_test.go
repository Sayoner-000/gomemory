//go:build !windows

package hcdfiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLinkAndRemoveNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.md")
	dst := filepath.Join(dir, "dst.md")
	if err := os.WriteFile(src, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := linkAndRemove(src, dst); !errors.Is(err, os.ErrExist) {
		t.Fatalf("linkAndRemove over existing = %v, want os.ErrExist", err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "existing" {
		t.Fatalf("existing dst altered: %q %v", got, err)
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != "source" {
		t.Fatalf("src altered: %q %v", got, err)
	}
	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	if err := linkAndRemove(src, dst); err != nil {
		t.Fatalf("linkAndRemove fresh = %v", err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "source" {
		t.Fatalf("moved dst = %q %v", got, err)
	}
	if _, err := os.Lstat(src); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("src still exists: %v", err)
	}
}
