//go:build !windows

package hostbind

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolverWithDeniedPS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte("#!/bin/sh\necho 'operation not permitted: ps' >&2\nexit 126\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv(ConversationEnv, "sandbox-A")
	store := New(t.TempDir())
	if _, _, _, err := psInfo(os.Getpid()); err == nil {
		t.Fatal("ps denegado debe informar el fallo")
	}
	if _, ok := store.Lookup(); ok {
		t.Fatal("no inventar un enlace sin verificar el proceso")
	}
	if err := store.Publish("hook"); err == nil {
		t.Fatal("no publicar un PID sin identidad verificada")
	}
	r := NewResolver(store, nil)
	if got := r.Current(); got != "sandbox-A" {
		t.Fatalf("perdió afinidad sin ps: %q", got)
	}
	t.Setenv(ConversationEnv, "sandbox-B")
	if got := r.Current(); got != "sandbox-B" {
		t.Fatalf("fallback debe seguir el contexto explícito: %q", got)
	}
	r.Adopt("explicit-hook")
	if got := r.Current(); got != "explicit-hook" {
		t.Fatalf("hook explícito debe prevalecer: %q", got)
	}
}
