package hcdfiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStore_AtomicWriteReadHashAndList(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	content := []byte("# Definition\n")
	if err := store.Write(ctx, "001-example", "definition.md", content); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read(ctx, "001-example", "definition.md")
	if err != nil || string(got) != string(content) {
		t.Fatalf("Read=%q, %v", got, err)
	}
	hash, err := store.Hash(ctx, "001-example", "definition.md")
	if err != nil || len(hash) != 12 {
		t.Fatalf("Hash=%q, %v", hash, err)
	}
	exists, err := store.Exists(ctx, "001-example", "definition.md")
	if err != nil || !exists {
		t.Fatalf("Exists=%v, %v", exists, err)
	}
	units, err := store.ListUnits(ctx)
	if err != nil || len(units) != 1 || units[0] != "001-example" {
		t.Fatalf("ListUnits=%v, %v", units, err)
	}
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if filepath.Dir(store.Root()) != resolvedRoot {
		t.Fatalf("unexpected root %q", store.Root())
	}
}

func TestStore_RejectsTraversalAndSymlink(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Write(context.Background(), "../outside", "definition.md", []byte("bad")); err == nil {
		t.Fatal("must reject traversal")
	}
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(store.Root(), "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(store.Root(), "features", "001-link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := store.Write(context.Background(), "001-link", "definition.md", []byte("bad")); err == nil {
		t.Fatal("must reject symlinked unit directory")
	}
	if _, err := os.Stat(filepath.Join(outside, "definition.md")); !os.IsNotExist(err) {
		t.Fatal("write escaped hcd/ through symlink")
	}
}
