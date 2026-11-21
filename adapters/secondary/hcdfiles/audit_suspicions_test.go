package hcdfiles

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"mem/domain"
)

// This establishes the OS primitive used by MoveDoc's vulnerable suffix. It
// does NOT claim to schedule another writer inside a real MoveDoc invocation:
// production has no seam between Lstat and Rename, so that race is STATIC_ONLY.
func TestAuditC004POSIXRenameOverwritePrimitive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX overwrite semantics only")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "hcd"), 0700); err != nil {
		t.Fatal(err)
	}
	src, dst := filepath.Join(root, "hcd/source.md"), filepath.Join(root, "hcd/target.md")
	if err := os.WriteFile(src, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("other writer"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (Workspace{Root: root}).MoveDoc("hcd/source.md", "hcd/target.md"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("existing destination protection: %v", err)
	}
	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("destination must be absent at check")
	}
	// Deterministically model another writer winning after MoveDoc's Lstat.
	if err := os.WriteFile(dst, []byte("other writer"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "source" {
		t.Fatalf("overwrite primitive: %q %v", data, err)
	}
	t.Log("STATIC_ONLY for MoveDoc race; runtime-confirmed POSIX Rename overwrites a destination created after Lstat")
}
