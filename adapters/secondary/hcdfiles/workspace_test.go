package hcdfiles

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		run(t, root, "git", args...)
	}
	write(t, root, "a.go", "package a\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-qm", "init")
	return root
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintTracksCodeButNotHCDDocs(t *testing.T) {
	root := gitRepo(t)
	w, ctx := Workspace{Root: root}, context.Background()
	base, err := w.Fingerprint(ctx)
	if err != nil || base == "nogit" {
		t.Fatalf("fingerprint = %q, %v", base, err)
	}
	if err := w.WriteDoc("hcd/HCD-001-x.md", []byte("# x\n")); err != nil {
		t.Fatal(err)
	}
	if got, _ := w.Fingerprint(ctx); got != base {
		t.Fatal("writing hcd/ changed the fingerprint")
	}
	write(t, root, "a.go", "package a // cambio\n")
	modified, _ := w.Fingerprint(ctx)
	if modified == base {
		t.Fatal("modifying tracked code did not change the fingerprint")
	}
	write(t, root, "b.go", "package a\n")
	if untracked, _ := w.Fingerprint(ctx); untracked == modified {
		t.Fatal("an untracked file did not change the fingerprint")
	}
	if got, _ := (Workspace{Root: t.TempDir()}).Fingerprint(ctx); got != "nogit" {
		t.Fatalf("outside git = %q", got)
	}
}

func TestPathsStayInsideAndRejectSymlinks(t *testing.T) {
	root := gitRepo(t)
	w := Workspace{Root: root}
	for _, bad := range []string{"../x.md", "hcd/../a.go", "/etc/passwd", "a.go", "hcd//x.md"} {
		if err := w.WriteDoc(bad, []byte("x")); err == nil {
			t.Errorf("WriteDoc(%q) accepted", bad)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "hcd")); err != nil {
		t.Skip("symlinks not supported")
	}
	if err := w.WriteDoc("hcd/x.md", []byte("x")); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlinked hcd/ accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "x.md")); err == nil {
		t.Fatal("wrote through the symlink")
	}
}

func TestRunReportsExitCodeAndTail(t *testing.T) {
	w, ctx := Workspace{Root: t.TempDir()}, context.Background()
	code, out, err := w.Run(ctx, "echo ok")
	if err != nil || code != 0 || out != "ok" {
		t.Fatalf("Run ok = %d %q %v", code, out, err)
	}
	code, out, err = w.Run(ctx, "for i in $(seq 1 50); do echo l$i; done; exit 3")
	if err != nil || code != 3 || !strings.HasSuffix(out, "l50") || strings.Contains(out, "l1\n") {
		t.Fatalf("Run fail = %d %q %v", code, out, err)
	}
}
